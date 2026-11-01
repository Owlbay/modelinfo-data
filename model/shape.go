package model

import (
	"fmt"
	"math"
	"reflect"
	"strings"

	"github.com/Owlbay/modelinfo-data/jsonx"
)

// Kind 是数据实体的种类，决定格式定义与所在目录。
type Kind string

const (
	KindModel    Kind = "model"
	KindVendor   Kind = "vendor"
	KindProvider Kind = "provider"
)

// Kinds 是全部实体种类，按校验顺序（被引用的在前）。
var Kinds = []Kind{KindVendor, KindProvider, KindModel}

// Dir 返回该种类数据文件所在的目录名：models、vendors、providers。
func (k Kind) Dir() string { return string(k) + "s" }

// goType 返回该种类对应的 Go 结构体类型。
func (k Kind) goType() reflect.Type {
	switch k {
	case KindVendor:
		return reflect.TypeFor[Vendor]()
	case KindProvider:
		return reflect.TypeFor[ProviderInfo]()
	}
	return reflect.TypeFor[Model]()
}

// field 是结构体中一个 JSON 字段。
type field struct {
	name     string
	optional bool // omitempty：可以省略
	typ      reflect.Type
}

// fieldsOf 按声明顺序返回结构体的 JSON 字段（即标准键顺序）。
func fieldsOf(t reflect.Type) []field {
	var out []field
	for i := range t.NumField() {
		f := t.Field(i)
		tag := f.Tag.Get("json")
		name, opts, _ := strings.Cut(tag, ",")
		if name == "-" || !f.IsExported() {
			continue
		}
		if name == "" {
			name = f.Name
		}
		out = append(out, field{name: name, optional: strings.Contains(opts, "omitempty"), typ: f.Type})
	}
	return out
}

// Fields 返回某个种类顶层字段的标准顺序（含可省略字段）。
func Fields(k Kind) []string {
	var out []string
	for _, f := range fieldsOf(k.goType()) {
		out = append(out, f.name)
	}
	return out
}

// Shape 检查文档的结构：每一层都不缺字段、没有未知字段、字段顺序符合格式定义、每个值的 JSON 类型正确
// （例如必填的数组不能是 null）。只看结构，不看取值规则（取值由 Validate 检查）。
// 问题按「路径：说明」给出，与 Validate 一致。
func Shape(k Kind, n jsonx.Node) Problems {
	var p Problems
	shape(&p, "", n, k.goType())
	if len(p) == 0 {
		return nil
	}
	return p
}

func shape(p *Problems, path string, n jsonx.Node, t reflect.Type) {
	add := func(msg string, args ...any) {
		at := path
		if at == "" {
			at = "(根)"
		}
		*p = append(*p, at+"："+fmt.Sprintf(msg, args...))
	}
	if t.Kind() == reflect.Pointer {
		if n.Kind == jsonx.Null {
			return
		}
		t = t.Elem()
	}
	switch t.Kind() {
	case reflect.Struct:
		if n.Kind != jsonx.Object {
			add("应为对象，实际为%s", kindName(n.Kind))
			return
		}
		fields := fieldsOf(t)
		known := map[string]field{}
		for _, f := range fields {
			known[f.name] = f
		}
		var order []string // 文档中已知字段的出现顺序
		for i, k := range n.Keys {
			f, ok := known[k]
			if !ok {
				*p = append(*p, join(path, k)+"："+fmt.Sprintf("未知字段（可用字段：%s）", strings.Join(names(fields), "、")))
				continue
			}
			order = append(order, k)
			shape(p, join(path, k), n.Vals[i], f.typ)
		}
		for _, f := range fields {
			if _, ok := n.Get(f.name); !ok && !f.optional {
				add("缺少字段 %s%s", f.name, emptyHint(f.typ))
			}
		}
		var want []string
		for _, f := range fields {
			if _, ok := n.Get(f.name); ok {
				want = append(want, f.name)
			}
		}
		if strings.Join(order, ",") != strings.Join(want, ",") {
			add("字段顺序应为 %s（运行 go run ./cmd/check --fix 自动调整）", strings.Join(want, "、"))
		}
	case reflect.Slice:
		if n.Kind != jsonx.Array {
			add("应为数组，实际为%s（没有时写 []）", kindName(n.Kind))
			return
		}
		for i, it := range n.Items {
			shape(p, fmt.Sprintf("%s[%d]", path, i), it, t.Elem())
		}
	case reflect.Map:
		if n.Kind != jsonx.Object {
			add("应为对象，实际为%s", kindName(n.Kind))
			return
		}
		for i, k := range n.Keys {
			shape(p, join(path, k), n.Vals[i], t.Elem())
		}
	case reflect.String:
		if n.Kind != jsonx.String {
			add("应为字符串，实际为%s", kindName(n.Kind))
		}
	case reflect.Bool:
		if n.Kind != jsonx.Bool {
			add("应为 true 或 false，实际为%s", kindName(n.Kind))
		}
	case reflect.Int64:
		if n.Kind != jsonx.Number || n.N != math.Trunc(n.N) || math.Abs(n.N) > 1<<53 {
			add("应为整数，实际为%s", kindName(n.Kind))
		}
	case reflect.Float64:
		if n.Kind != jsonx.Number {
			add("应为数值，实际为%s", kindName(n.Kind))
		}
	}
}

func join(path, k string) string {
	if path == "" {
		return k
	}
	return path + "." + k
}

func names(fs []field) []string {
	out := make([]string, len(fs))
	for i, f := range fs {
		out[i] = f.name
	}
	return out
}

func emptyHint(t reflect.Type) string {
	switch t.Kind() {
	case reflect.Pointer:
		return "（未知时写 null）"
	case reflect.Slice:
		return "（没有时写 []）"
	}
	return ""
}

func kindName(k jsonx.Kind) string {
	switch k {
	case jsonx.Null:
		return " null"
	case jsonx.Bool:
		return "布尔值"
	case jsonx.Number:
		return "数值"
	case jsonx.String:
		return "字符串"
	case jsonx.Array:
		return "数组"
	}
	return "对象"
}

// Format 把一个文档改写为标准格式：按格式定义排好字段顺序（未知字段保留在末尾）、两空格缩进、
// 数值与字符串按 JavaScript JSON.stringify 写法、末尾换行。只改写法与顺序，不校验取值；
// JSON 语法错误时返回错误。对已是标准格式的文档返回与输入逐字节相同的内容。
func Format(k Kind, raw []byte) ([]byte, error) {
	n, err := jsonx.Parse(raw)
	if err != nil {
		return nil, err
	}
	return jsonx.Indent(reorder(n, k.goType())), nil
}

// reorder 按结构体字段顺序重排对象的键；map 与未知字段保持原有顺序。
func reorder(n jsonx.Node, t reflect.Type) jsonx.Node {
	if t.Kind() == reflect.Pointer {
		t = t.Elem()
	}
	switch {
	case t.Kind() == reflect.Struct && n.Kind == jsonx.Object:
		out := jsonx.Node{Kind: jsonx.Object}
		used := map[string]bool{}
		for _, f := range fieldsOf(t) {
			if v, ok := n.Get(f.name); ok {
				out.Keys = append(out.Keys, f.name)
				out.Vals = append(out.Vals, reorder(v, f.typ))
				used[f.name] = true
			}
		}
		for i, k := range n.Keys {
			if !used[k] {
				out.Keys = append(out.Keys, k)
				out.Vals = append(out.Vals, n.Vals[i])
			}
		}
		return out
	case t.Kind() == reflect.Slice && n.Kind == jsonx.Array:
		out := n
		out.Items = make([]jsonx.Node, len(n.Items))
		for i, it := range n.Items {
			out.Items[i] = reorder(it, t.Elem())
		}
		return out
	case t.Kind() == reflect.Map && n.Kind == jsonx.Object:
		out := n
		out.Vals = make([]jsonx.Node, len(n.Vals))
		for i, v := range n.Vals {
			out.Vals[i] = reorder(v, t.Elem())
		}
		return out
	}
	return n
}
