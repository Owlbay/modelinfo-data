package model

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// root 是数据仓库根目录（含 vendors/、providers/、models/）。
const root = ".."

// 仓库中的全部数据必须通过校验（语法、结构、取值、引用、别名、文件名、标准格式）。
func TestRepoData(t *testing.T) {
	ds, issues := LoadDir(root)
	for _, i := range issues {
		t.Error(i)
	}
	if len(ds.Models) < 1000 || len(ds.Vendors) == 0 || len(ds.Providers) == 0 {
		t.Fatalf("数据过少：%d 个模型、%d 个厂商、%d 个供应商", len(ds.Models), len(ds.Vendors), len(ds.Providers))
	}
	// 标准格式是 Format 的不动点
	for _, f := range ds.Models {
		out, err := Format(KindModel, f.Raw)
		if err != nil || !bytes.Equal(out, f.Raw) {
			t.Errorf("%s：不是标准格式", f.Path)
		}
	}
}

func sample(t *testing.T) []byte {
	t.Helper()
	raw, err := os.ReadFile(filepath.Join(root, "models", "qwen3.8-max.json"))
	if err != nil {
		t.Fatal(err)
	}
	return raw
}

func TestValidateRules(t *testing.T) {
	raw := sample(t)
	base, _, err := Decode(raw)
	if err != nil || Validate(base, nil) != nil {
		t.Fatalf("样例应通过校验：%v %v", err, Validate(base, nil))
	}
	f := func(v float64) *float64 { return &v }
	reg := func(names map[string][]string) *Registry {
		return &Registry{Vendors: map[string]bool{"alibaba": true}, Providers: map[string]bool{"alibaba": true}, Names: names}
	}
	cases := []struct {
		name string
		edit func(m *Model)
		reg  *Registry
		want string
	}{
		{"prices 为 null", func(m *Model) { m.Prices = nil }, nil, "prices"},
		{"价格为负", func(m *Model) { m.Prices[0].Input = f(-1) }, nil, "prices[0]"},
		{"币种重复", func(m *Model) { m.Prices[1].Currency = "USD" }, nil, "重复"},
		{"接口类型不在 api.types", func(m *Model) { m.API.Types = []string{"chat"} }, nil, "api.types"},
		{"发布日与 created 不一致", func(m *Model) { m.Info.ReleaseDate = ptr("2020-01-01") }, nil, "created"},
		{"输出大于上下文", func(m *Model) { m.Limits.Output = ptr(int64(1 << 40)) }, nil, "context"},
		{"厂商不存在", func(m *Model) { m.OwnedBy = "nobody" }, reg(nil), "厂商 nobody 不存在"},
		{"供应商不存在", func(m *Model) { m.Providers[0].ID = "nobody" }, reg(nil), "供应商 nobody 不存在"},
		{"简介为 null 却有译文", func(m *Model) { m.Info.Description = nil }, nil, "info.description 为 null"},
		{"别名等于自身", func(m *Model) { m.Aliases = append(m.Aliases, m.ID) }, nil, "自身 ID"},
		{"别名重复", func(m *Model) { m.Aliases = append(m.Aliases, m.Aliases[0]) }, nil, "aliases"},
		{"别名被其他模型使用", func(m *Model) {}, reg(map[string][]string{"qwen3.8-max-0902": {"qwen3.8-max", "other"}}), "已被模型 other 使用"},
		{"ID 是其他模型的别名", func(m *Model) {}, reg(map[string][]string{"qwen3.8-max": {"qwen3.8-max", "other"}}), "已是模型 other 的别名"},
		{"第三方聚合站来源", func(m *Model) { m.Sources[0].URL = "https://openrouter.ai/qwen/qwen3.8-max" }, nil, "第三方聚合站"},
	}
	for _, c := range cases {
		m, _, _ := Decode(raw)
		c.edit(&m)
		err := Validate(m, c.reg)
		if err == nil || !strings.Contains(err.Error(), c.want) {
			t.Errorf("%s：得到 %v，应包含 %q", c.name, err, c.want)
		}
	}
}

func ptr[T any](v T) *T { return &v }

func TestShape(t *testing.T) {
	raw := string(sample(t))
	cases := []struct {
		name, from, to, want string
	}{
		{"缺少字段", `"created": 1785715200,`, ``, "缺少字段 created"},
		{"未知字段", `"object": "model",`, `"object": "model", "foo": 1,`, "foo：未知字段"},
		{"字段顺序", `"object": "model",
  "created": 1785715200,`, `"created": 1785715200,
  "object": "model",`, "字段顺序应为 id、object、created"},
		{"数组写成 null", `"efforts": []`, `"efforts": null`, "reasoning.efforts：应为数组"},
		{"对象写成 null", `"limits": {`, `"limits": null, "x": {`, "limits：应为对象"},
		{"整数写成小数", `"context": 1000000,`, `"context": 1000000.5,`, "limits.context：应为整数"},
		{"provider 缺 reasoning_param", `"reasoning_param": "enable_thinking",`, ``, "providers[0]：缺少字段 reasoning_param"},
	}
	for _, c := range cases {
		doc := strings.Replace(raw, c.from, c.to, 1)
		if doc == raw {
			t.Fatalf("%s：替换没有生效", c.name)
		}
		_, _, err := CheckModel([]byte(doc), nil)
		if err == nil || !strings.Contains(err.Error(), c.want) {
			t.Errorf("%s：得到 %v，应包含 %q", c.name, err, c.want)
		}
	}
}

func TestFormat(t *testing.T) {
	raw := sample(t)
	// 打乱顺序、压缩、改数值写法后，Format 应还原为原文
	messy := strings.Replace(string(raw), `"object": "model",
  "created": 1785715200,`, `"created": 1785715200,
  "object": "model",`, 1)
	messy = strings.Replace(messy, `"input": 2,`, `"input": 2.0,`, 1)
	messy = strings.ReplaceAll(messy, "\n  ", "\n\t")
	out, err := Format(KindModel, []byte(messy))
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(out, raw) {
		t.Fatalf("Format 应还原标准格式，得到：\n%s", out)
	}
	// endpoints 等 map 的键保持原顺序
	v, err := Format(KindVendor, []byte(`{"icon":"x","url":"https://x.ai","name":"X","id":"x"}`))
	if err != nil || string(v) != "{\n  \"id\": \"x\",\n  \"name\": \"X\",\n  \"url\": \"https://x.ai\",\n  \"icon\": \"x\"\n}\n" {
		t.Fatalf("Format(vendor) = %s %v", v, err)
	}
}

func TestFields(t *testing.T) {
	want := "id,object,created,owned_by,name,api,limits,modalities,reasoning,prices,features,info,providers,aliases,translations,sources"
	if got := strings.Join(Fields(KindModel), ","); got != want {
		t.Fatalf("模型字段顺序：%s", got)
	}
}
