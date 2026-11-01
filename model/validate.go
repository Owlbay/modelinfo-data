package model

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"net/url"
	"regexp"
	"slices"
	"strings"
	"time"

	"github.com/Owlbay/modelinfo-data/jsonx"
)

// 枚举字段的取值范围。新增取值时同时更新 schema/*.schema.json 与 docs/fields.md。
var (
	APITypes   = []string{"chat", "messages", "responses", "interactions", "completions", "generate", "batch", "realtime", "embeddings", "images", "video", "audio", "3d", "rerank", "ocr", "moderation"}
	ModalityIn = []string{"text", "image", "audio", "video", "document", "embedding", "score", "3d"}
	Features   = []string{"tools", "structured_output", "streaming", "caching", "batch"}
	Units      = []string{"1M_tokens", "1M_characters", "image", "second", "minute", "hour", "request", "page"}
	Statuses   = []string{"preview", "active", "deprecated", "retired"}
	// Langs 是译文可用的语言（原文多为英文，少数为中文）。
	Langs = []string{"zh", "en"}
)

var (
	reID       = regexp.MustCompile(`^[a-z0-9][a-z0-9._-]*$`)
	reOwner    = regexp.MustCompile(`^[a-z0-9-]+$`)
	reSDK      = regexp.MustCompile(`^(?:@[a-z0-9._-]+/)?[a-z0-9._-]+(?:/[a-z0-9._-]+)*$`)
	reDate     = regexp.MustCompile(`^\d{4}-\d{2}-\d{2}$`)
	reCurrency = regexp.MustCompile(`^[A-Z]{3}$`)
	reBaseURL  = regexp.MustCompile(`^(https|wss)://`)
	reEndpoint = regexp.MustCompile(`^(GET|POST|WS) /`)
	reChat     = regexp.MustCompile(`/chat/completions\b`)
)

// Decode 严格解析一个模型文档：未知字段、重复键、多余内容都报错。同时返回保留键顺序的树。
func Decode(data []byte) (Model, jsonx.Node, error) {
	var m Model
	dec := json.NewDecoder(bytes.NewReader(data))
	dec.DisallowUnknownFields()
	if err := dec.Decode(&m); err != nil {
		return Model{}, jsonx.Node{}, err
	}
	n, err := jsonx.Parse(data)
	if err != nil {
		return Model{}, jsonx.Node{}, err
	}
	return m, n, nil
}

// Registry 是校验引用所需的目录：模型厂商与供应商的 ID，以及（可选的）模型 ID 与别名的占用情况。
type Registry struct {
	Vendors   map[string]bool
	Providers map[string]bool
	// Names 是模型 ID 与别名共用的命名空间：名字 → 用它作 ID 或别名的模型 ID。
	// 为 nil 时 Validate 不检查别名冲突；用 NewNames 从全部模型构建。
	Names map[string][]string
}

// NewNames 从全部模型构建 Registry.Names。
func NewNames(models []Model) map[string][]string {
	names := map[string][]string{}
	claim := func(name, id string) {
		if !slices.Contains(names[name], id) {
			names[name] = append(names[name], id)
		}
	}
	for _, m := range models {
		claim(m.ID, m.ID)
		for _, a := range m.Aliases {
			claim(a, m.ID)
		}
	}
	return names
}

// ThirdPartyHosts 是不能作为来源的第三方聚合站（含其子域名）。来源必须是模型厂商或供应商的官方页面。
var ThirdPartyHosts = []string{
	"openrouter.ai", "artificialanalysis.ai", "llm-stats.com", "lmarena.ai",
	"llmpricecheck.com", "pricepertoken.com", "docsbot.ai", "helicone.ai", "wikipedia.org",
}

// Problems 是校验失败的字段列表（路径：说明）。
type Problems []string

func (p Problems) Error() string { return strings.Join(p, "；") }

// Validate 检查模型的全部规则；reg 为 nil 时跳过对厂商、供应商目录的引用检查。
func Validate(m Model, reg *Registry) error {
	var p Problems
	add := func(path, msg string, args ...any) { p = append(p, path+"："+fmt.Sprintf(msg, args...)) }

	if !reID.MatchString(m.ID) {
		add("id", "只能用小写字母、数字与 . _ -")
	}
	if m.Object != "model" {
		add("object", "必须是 model")
	}
	if m.Created != nil && *m.Created <= 0 {
		add("created", "必须为正整数")
	}
	if !reOwner.MatchString(m.OwnedBy) {
		add("owned_by", "只能用小写字母、数字与 -")
	} else if reg != nil && !reg.Vendors[m.OwnedBy] {
		add("owned_by", "厂商 %s 不存在", m.OwnedBy)
	}
	if strings.TrimSpace(m.Name) == "" {
		add("name", "不能为空")
	}
	if len(m.API.Types) == 0 {
		add("api.types", "至少一项")
	}
	enum(add, "api.types", m.API.Types, APITypes)
	if m.API.SDK == nil {
		add("api.sdk", "不能为 null，没有时写空数组")
	}
	for i, s := range m.API.SDK {
		if !reSDK.MatchString(s) {
			add(fmt.Sprintf("api.sdk[%d]", i), "包名格式不对")
		}
	}
	unique(add, "api.sdk", m.API.SDK)
	limits(add, "limits", m.Limits)
	modalities(add, "modalities", m.Modalities)
	if m.Reasoning.Efforts == nil {
		add("reasoning.efforts", "不能为 null，没有时写空数组")
	}
	if m.Features == nil {
		add("features", "不能为 null，没有时写空数组")
	}
	enum(add, "features", m.Features, Features)
	if !slices.Contains(Statuses, m.Info.Status) {
		add("info.status", "只能是 %s", strings.Join(Statuses, "、"))
	}
	date(add, "info.release_date", m.Info.ReleaseDate)
	date(add, "info.verified_at", m.Info.VerifiedAt)
	if m.Info.Docs != nil && !isURL(*m.Info.Docs, "http://", "https://") {
		add("info.docs", "必须是网址")
	}
	switch {
	case m.Created != nil && (m.Info.ReleaseDate == nil || *m.Info.ReleaseDate != time.Unix(*m.Created, 0).UTC().Format(time.DateOnly)):
		add("info.release_date", "必须与 created 一致")
	case m.Created == nil && m.Info.ReleaseDate != nil:
		add("created", "有 release_date 时 created 不能为空")
	}

	// prices：官方公布的全部币种，主币种在前；没有公开价时为空数组
	if m.Prices == nil {
		add("prices", "不能为 null，没有时写空数组")
	}
	prices(add, "prices", m.Prices)

	if m.Providers == nil {
		add("providers", "不能为 null，没有时写空数组")
	}
	for i, pv := range m.Providers {
		provider(add, fmt.Sprintf("providers[%d]", i), pv, m.API.Types, reg)
	}
	if m.Aliases == nil {
		add("aliases", "不能为 null，没有时写空数组")
	}
	unique(add, "aliases", m.Aliases)
	for i, a := range m.Aliases {
		at := fmt.Sprintf("aliases[%d]", i)
		switch {
		case a == m.ID:
			add(at, "别名不能与自身 ID 相同")
		case strings.TrimSpace(a) == "":
			add(at, "不能为空")
		case reg != nil && reg.Names != nil:
			if others := othersThan(reg.Names[a], m.ID); len(others) > 0 {
				add(at, "别名 %s 已被模型 %s 使用（ID 与别名共用一个命名空间）", a, strings.Join(others, "、"))
			}
		}
	}
	if reg != nil && reg.Names != nil {
		if others := othersThan(reg.Names[m.ID], m.ID); len(others) > 0 {
			add("id", "%s 已是模型 %s 的别名", m.ID, strings.Join(others, "、"))
		}
	}
	if m.Info.Description == nil && len(m.Translations) > 0 {
		add("translations", "info.description 为 null 时不能有译文")
	}
	for _, lang := range sortedKeys(m.Translations) {
		t := m.Translations[lang]
		if !slices.Contains(Langs, lang) {
			add("translations", "语言 %s 不支持", lang)
		}
		if strings.TrimSpace(t.Description) == "" {
			add("translations."+lang+".description", "不能为空")
		}
	}
	if len(m.Sources) == 0 {
		add("sources", "至少一条官方来源")
	}
	for i, s := range m.Sources {
		path := fmt.Sprintf("sources[%d]", i)
		if !isURL(s.URL, "https://") {
			add(path+".url", "必须是 https 网址")
		} else if h := thirdParty(s.URL); h != "" {
			add(path+".url", "%s 是第三方聚合站，来源必须是厂商官方页面", h)
		}
		if !reDate.MatchString(s.RetrievedAt) {
			add(path+".retrieved_at", "日期格式为 YYYY-MM-DD")
		}
	}
	if len(p) > 0 {
		return p
	}
	return nil
}

func othersThan(ids []string, self string) []string {
	var out []string
	for _, id := range ids {
		if id != self {
			out = append(out, id)
		}
	}
	return out
}

// thirdParty 返回网址所属的第三方聚合站域名；不是时返回空串。
func thirdParty(raw string) string {
	u, err := url.Parse(raw)
	if err != nil {
		return ""
	}
	host := strings.ToLower(u.Hostname())
	for _, h := range ThirdPartyHosts {
		if host == h || strings.HasSuffix(host, "."+h) {
			return h
		}
	}
	return ""
}

func provider(add func(string, string, ...any), path string, p Provider, types []string, reg *Registry) {
	if reg != nil && !reg.Providers[p.ID] {
		add(path+".id", "供应商 %s 不存在", p.ID)
	}
	if strings.TrimSpace(p.Model) == "" {
		add(path+".model", "不能为空")
	}
	if !reBaseURL.MatchString(p.BaseURL) {
		add(path+".base_url", "必须以 https:// 或 wss:// 开头")
	}
	for _, t := range sortedKeys(p.Endpoints) {
		e := p.Endpoints[t]
		ep := path + ".endpoints." + t
		if !slices.Contains(APITypes, t) {
			add(ep, "未知接口类型")
		}
		if !reEndpoint.MatchString(e) {
			add(ep, "格式为「GET|POST|WS /路径」")
		}
		if strings.HasPrefix(e, "WS ") != strings.HasPrefix(p.BaseURL, "wss://") {
			add(ep, "WS 端点使用 wss://；HTTP 端点使用 https://，不同协议拆分 provider")
		}
		if !slices.Contains(types, t) {
			add(ep, "接口类型必须包含在 api.types 中")
		}
		if t == "messages" && reChat.MatchString(e) {
			add(ep, "Chat Completions 不能标为 messages")
		}
	}
	for _, t := range sortedKeys(p.ReasoningParams) {
		if _, ok := p.Endpoints[t]; !ok {
			add(path+".reasoning_params."+t, "必须对应已登记的端点")
		}
	}
	if p.Limits != nil {
		limits(add, path+".limits", *p.Limits)
	}
	if p.Modalities != nil {
		modalities(add, path+".modalities", *p.Modalities)
	}
	enum(add, path+".features", p.Features, Features)
	if p.Prices != nil {
		prices(add, path+".prices", *p.Prices)
	}
}

// prices 检查每个币种的价格，同一币种只能出现一次。
func prices(add func(string, string, ...any), path string, ps []Price) {
	seen := map[string]bool{}
	for i, pr := range ps {
		at := fmt.Sprintf("%s[%d]", path, i)
		price(add, at, pr)
		if seen[pr.Currency] {
			add(at, "币种 %s 重复", pr.Currency)
		}
		seen[pr.Currency] = true
	}
}

func limits(add func(string, string, ...any), path string, l Limits) {
	for _, f := range []struct {
		name string
		v    *int64
	}{{"context", l.Context}, {"input", l.Input}, {"output", l.Output}} {
		if f.v != nil && *f.v <= 0 {
			add(path+"."+f.name, "必须为正整数")
		}
	}
	if l.Context != nil {
		if l.Input != nil && *l.Input > *l.Context {
			add(path+".input", "不能大于 context")
		}
		if l.Output != nil && *l.Output > *l.Context {
			add(path+".output", "不能大于 context")
		}
	}
}

func modalities(add func(string, string, ...any), path string, m Modalities) {
	if len(m.Input) == 0 {
		add(path+".input", "至少一项")
	}
	if len(m.Output) == 0 {
		add(path+".output", "至少一项")
	}
	enum(add, path+".input", m.Input, ModalityIn)
	enum(add, path+".output", m.Output, ModalityIn)
}

func price(add func(string, string, ...any), path string, p Price) {
	if !reCurrency.MatchString(p.Currency) {
		add(path+".currency", "三位大写币种代码")
	}
	if !slices.Contains(Units, p.Unit) {
		add(path+".unit", "只能是 %s", strings.Join(Units, "、"))
	}
	for _, f := range []struct {
		name string
		v    *float64
	}{{"input", p.Input}, {"output", p.Output}, {"cache_read", p.CacheRead}, {"cache_write", p.CacheWrite}} {
		if f.v != nil && *f.v < 0 {
			add(path+"."+f.name, "不能为负")
		}
	}
}

func enum(add func(string, string, ...any), path string, vs, allowed []string) {
	for i, v := range vs {
		if !slices.Contains(allowed, v) {
			add(fmt.Sprintf("%s[%d]", path, i), "未知取值 %s", v)
		}
	}
}

func unique(add func(string, string, ...any), path string, vs []string) {
	seen := map[string]bool{}
	for _, v := range vs {
		if seen[v] {
			add(path, "%s 重复", v)
		}
		seen[v] = true
	}
}

func date(add func(string, string, ...any), path string, v *string) {
	if v != nil && !reDate.MatchString(*v) {
		add(path, "日期格式为 YYYY-MM-DD")
	}
}

func isURL(s string, schemes ...string) bool {
	u, err := url.Parse(s)
	if err != nil || u.Host == "" {
		return false
	}
	for _, sc := range schemes {
		if strings.HasPrefix(s, sc) {
			return true
		}
	}
	return false
}

func sortedKeys[V any](m map[string]V) []string {
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	slices.Sort(keys)
	return keys
}

// DecodeVendor、DecodeProvider 严格解析厂商与供应商记录并校验。
func DecodeVendor(data []byte) (Vendor, error) {
	var v Vendor
	if err := strict(data, &v); err != nil {
		return v, err
	}
	var p Problems
	if !reOwner.MatchString(v.ID) {
		p = append(p, "id：只能用小写字母、数字与 -")
	}
	if strings.TrimSpace(v.Name) == "" {
		p = append(p, "name：不能为空")
	}
	if !isURL(v.URL, "https://") {
		p = append(p, "url：必须是 https 网址")
	}
	if v.Icon != nil && !reOwner.MatchString(*v.Icon) {
		p = append(p, "icon：只能用小写字母、数字与 -")
	}
	if len(p) > 0 {
		return v, p
	}
	return v, nil
}

func DecodeProvider(data []byte) (ProviderInfo, error) {
	var v ProviderInfo
	if err := strict(data, &v); err != nil {
		return v, err
	}
	var p Problems
	if !reOwner.MatchString(v.ID) {
		p = append(p, "id：只能用小写字母、数字与 -")
	}
	if strings.TrimSpace(v.Name) == "" {
		p = append(p, "name：不能为空")
	}
	if v.URL != nil && !isURL(*v.URL, "https://") {
		p = append(p, "url：必须是 https 网址")
	}
	if len(p) > 0 {
		return v, p
	}
	return v, nil
}

func strict(data []byte, v any) error {
	dec := json.NewDecoder(bytes.NewReader(data))
	dec.DisallowUnknownFields()
	if err := dec.Decode(v); err != nil {
		return err
	}
	if dec.More() {
		return errors.New("JSON 之后还有多余内容")
	}
	return nil
}
