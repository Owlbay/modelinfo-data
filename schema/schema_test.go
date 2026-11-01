package schema

import (
	"os"
	"path/filepath"
	"reflect"
	"slices"
	"strings"
	"testing"

	"github.com/Owlbay/modelinfo-data/internal/jsonschema"
	"github.com/Owlbay/modelinfo-data/jsonx"
	"github.com/Owlbay/modelinfo-data/model"
)

func load(t *testing.T, k model.Kind) *jsonschema.Schema {
	t.Helper()
	s, err := jsonschema.Parse(For(k))
	if err != nil {
		t.Fatalf("%s.schema.json：%v", k, err)
	}
	return s
}

// 仓库中的每个文件都要通过对应的 schema（Go 校验的结论由 model 包的 TestRepoData 保证）。
func TestRepoDataPassesSchema(t *testing.T) {
	for _, k := range model.Kinds {
		s := load(t, k)
		files, _ := filepath.Glob(filepath.Join("..", k.Dir(), "*.json"))
		if len(files) == 0 {
			t.Fatalf("%s/ 下没有文件", k.Dir())
		}
		for _, f := range files {
			raw, _ := os.ReadFile(f)
			n, err := jsonx.Parse(raw)
			if err != nil {
				t.Errorf("%s：%v", f, err)
				continue
			}
			for _, e := range s.Validate(n) {
				t.Errorf("%s：%s", f, e)
			}
		}
	}
}

// 改坏的文档应同时被 schema 与 Go 校验拒绝。
func TestBrokenDocsRejectedByBoth(t *testing.T) {
	raw, err := os.ReadFile(filepath.Join("..", "models", "qwen3.8-max.json"))
	if err != nil {
		t.Fatal(err)
	}
	s := load(t, model.KindModel)
	cases := []struct{ name, from, to string }{
		{"缺少字段", "  \"name\": \"Qwen3.8-Max\",\n", ""},
		{"未知字段", `"object": "model",`, `"object": "model", "foo": 1,`},
		{"object 取值", `"object": "model"`, `"object": "models"`},
		{"ID 大写", `"id": "qwen3.8-max"`, `"id": "Qwen3.8-max"`},
		{"未知接口类型", `"chat",`, `"chatty",`},
		{"上下文为 0", `"context": 1000000`, `"context": 0`},
		{"上下文为小数", `"context": 1000000`, `"context": 1.5`},
		{"价格为负", `"input": 2,`, `"input": -2,`},
		{"币种小写", `"currency": "USD"`, `"currency": "usd"`},
		{"未知单位", `"unit": "1M_tokens"`, `"unit": "tokens"`},
		{"未知状态", `"status": "active"`, `"status": "live"`},
		{"日期格式", `"verified_at": "2026-10-11"`, `"verified_at": "2026/10/11"`},
		{"数组写成 null", `"features": [`, `"features": null, "x": [`},
		{"端点格式", `"POST /compatible-mode/v1/chat/completions"`, `"/compatible-mode/v1/chat/completions"`},
		{"未知译文语言", `"zh": {`, `"fr": {`},
		{"来源不是 https", `"url": "https://docs.modelstudio`, `"url": "http://docs.modelstudio`},
		{"别名重复", `"qwen3.8-max-2026-09-02"`, `"qwen3.8-max-0902"`},
	}
	for _, c := range cases {
		doc := strings.Replace(string(raw), c.from, c.to, 1)
		if doc == string(raw) {
			t.Fatalf("%s：替换没有生效", c.name)
		}
		n, err := jsonx.Parse([]byte(doc))
		if err != nil {
			t.Fatalf("%s：%v", c.name, err)
		}
		if errs := s.Validate(n); len(errs) == 0 {
			t.Errorf("%s：schema 应拒绝", c.name)
		}
		if _, _, err := model.CheckModel([]byte(doc), nil); err == nil {
			t.Errorf("%s：Go 校验应拒绝", c.name)
		}
	}
}

// schema 的字段、必填项与 Go 结构体一一对应：没有 omitempty 的字段必填，所有对象不允许额外字段。
func TestSchemaMatchesGoTypes(t *testing.T) {
	for _, c := range []struct {
		k model.Kind
		t reflect.Type
	}{
		{model.KindModel, reflect.TypeFor[model.Model]()},
		{model.KindVendor, reflect.TypeFor[model.Vendor]()},
		{model.KindProvider, reflect.TypeFor[model.ProviderInfo]()},
	} {
		s := load(t, c.k)
		compare(t, s, s.Root(), c.t, string(c.k))
	}
}

func compare(t *testing.T, s *jsonschema.Schema, sc jsonx.Node, ty reflect.Type, at string) {
	t.Helper()
	if ref, ok := sc.Get("$ref"); ok {
		sc, _ = s.Resolve(ref.S)
	}
	if ty.Kind() == reflect.Pointer {
		ty = ty.Elem()
	}
	switch ty.Kind() {
	case reflect.Struct:
		props, _ := sc.Get("properties")
		req, _ := sc.Get("required")
		extra, ok := sc.Get("additionalProperties")
		if !ok || extra.Kind != jsonx.Bool || extra.B {
			t.Errorf("%s：应设 additionalProperties: false", at)
		}
		var fields, required, reqSchema []string
		for i := range ty.NumField() {
			f := ty.Field(i)
			name, opts, _ := strings.Cut(f.Tag.Get("json"), ",")
			fields = append(fields, name)
			if !strings.Contains(opts, "omitempty") {
				required = append(required, name)
			}
			ps, ok := props.Get(name)
			if !ok {
				t.Errorf("%s：schema 缺少字段 %s", at, name)
				continue
			}
			compare(t, s, ps, f.Type, at+"."+name)
		}
		for _, r := range req.Items {
			reqSchema = append(reqSchema, r.S)
		}
		if !slices.Equal(props.Keys, fields) {
			t.Errorf("%s：schema 字段 %v，Go 字段 %v（顺序也应一致，便于对照）", at, props.Keys, fields)
		}
		if !slices.Equal(reqSchema, required) {
			t.Errorf("%s：schema 必填 %v，Go 必填 %v", at, reqSchema, required)
		}
	case reflect.Slice:
		if items, ok := sc.Get("items"); ok {
			compare(t, s, items, ty.Elem(), at+"[]")
		}
	case reflect.Map:
		if extra, ok := sc.Get("additionalProperties"); ok && extra.Kind == jsonx.Object {
			compare(t, s, extra, ty.Elem(), at+".*")
		}
	}
}

// schema 中的枚举与 Go 的取值列表相同。
func TestSchemaEnums(t *testing.T) {
	s := load(t, model.KindModel)
	enum := func(path string) []string {
		n, err := s.Resolve(path)
		if err != nil {
			t.Fatal(err)
		}
		e, _ := n.Get("enum")
		var out []string
		for _, it := range e.Items {
			out = append(out, it.S)
		}
		return out
	}
	for _, c := range []struct {
		path string
		want []string
	}{
		{"#/definitions/apiType", model.APITypes},
		{"#/definitions/modality", model.ModalityIn},
		{"#/definitions/feature", model.Features},
		{"#/definitions/price/properties/unit", model.Units},
		{"#/properties/info/properties/status", model.Statuses},
		{"#/properties/translations/propertyNames", model.Langs},
	} {
		if got := enum(c.path); !slices.Equal(got, c.want) {
			t.Errorf("%s：schema %v，Go %v", c.path, got, c.want)
		}
	}
}
