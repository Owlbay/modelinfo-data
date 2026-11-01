// Package jsonschema 是一个只覆盖 schema/*.schema.json 所用关键字的 JSON Schema（draft-07 子集）校验器，
// 供测试确认编辑器用的 schema 与 Go 校验结论一致。遇到未实现的关键字直接报错，避免静默放过。
package jsonschema

import (
	"fmt"
	"math"
	"regexp"
	"slices"
	"strings"

	"github.com/Owlbay/modelinfo-data/jsonx"
)

// Schema 是解析后的 schema 文档。
type Schema struct {
	root jsonx.Node
	re   map[string]*regexp.Regexp
}

// 只作说明、不参与校验的关键字。
var annotations = []string{"$schema", "$id", "title", "description", "markdownDescription", "definitions", "examples", "default"}

// 已实现的校验关键字。
var keywords = []string{"$ref", "type", "properties", "required", "additionalProperties", "propertyNames",
	"items", "enum", "const", "pattern", "minimum", "exclusiveMinimum", "minItems", "uniqueItems"}

// Parse 解析 schema，并检查其中只用了已实现的关键字。
func Parse(data []byte) (*Schema, error) {
	n, err := jsonx.Parse(data)
	if err != nil {
		return nil, err
	}
	s := &Schema{root: n, re: map[string]*regexp.Regexp{}}
	if err := s.check(n, "#"); err != nil {
		return nil, err
	}
	return s, nil
}

// Root 返回 schema 的根节点。
func (s *Schema) Root() jsonx.Node { return s.root }

// Resolve 解析 "#/definitions/x" 形式的引用。
func (s *Schema) Resolve(ref string) (jsonx.Node, error) {
	path, ok := strings.CutPrefix(ref, "#/")
	if !ok {
		return jsonx.Node{}, fmt.Errorf("只支持文档内引用：%s", ref)
	}
	n := s.root
	for _, seg := range strings.Split(path, "/") {
		var found bool
		if n, found = n.Get(seg); !found {
			return jsonx.Node{}, fmt.Errorf("引用不存在：%s", ref)
		}
	}
	return n, nil
}

func (s *Schema) check(n jsonx.Node, at string) error {
	if n.Kind != jsonx.Object {
		return fmt.Errorf("%s：schema 应为对象", at)
	}
	for i, k := range n.Keys {
		v := n.Vals[i]
		switch {
		case k == "definitions" || k == "properties":
			for j, name := range v.Keys {
				if err := s.check(v.Vals[j], at+"/"+k+"/"+name); err != nil {
					return err
				}
			}
		case k == "items" || k == "propertyNames" || (k == "additionalProperties" && v.Kind == jsonx.Object):
			if err := s.check(v, at+"/"+k); err != nil {
				return err
			}
		case k == "$ref":
			if _, err := s.Resolve(v.S); err != nil {
				return err
			}
		case k == "pattern":
			re, err := regexp.Compile(v.S)
			if err != nil {
				return fmt.Errorf("%s：%w", at, err)
			}
			s.re[v.S] = re
		case slices.Contains(annotations, k) || slices.Contains(keywords, k):
		default:
			return fmt.Errorf("%s：未实现的关键字 %s", at, k)
		}
	}
	return nil
}

// Validate 校验文档，返回全部错误（路径：说明）；通过时为 nil。
func (s *Schema) Validate(doc jsonx.Node) []string {
	var errs []string
	s.validate(s.root, doc, "", &errs)
	return errs
}

func (s *Schema) validate(sc, n jsonx.Node, at string, errs *[]string) {
	fail := func(msg string, args ...any) { *errs = append(*errs, at+"："+fmt.Sprintf(msg, args...)) }
	if ref, ok := sc.Get("$ref"); ok {
		target, _ := s.Resolve(ref.S)
		s.validate(target, n, at, errs)
	}
	if t, ok := sc.Get("type"); ok {
		var types []string
		if t.Kind == jsonx.String {
			types = []string{t.S}
		}
		for _, it := range t.Items {
			types = append(types, it.S)
		}
		if !slices.ContainsFunc(types, func(ty string) bool { return is(ty, n) }) {
			fail("类型应为 %s", strings.Join(types, "|"))
			return
		}
	}
	if e, ok := sc.Get("enum"); ok && !slices.ContainsFunc(e.Items, func(v jsonx.Node) bool { return equal(v, n) }) {
		fail("取值 %s 不在枚举中", jsonx.Encode(n))
	}
	if c, ok := sc.Get("const"); ok && !equal(c, n) {
		fail("应为 %s", jsonx.Encode(c))
	}
	switch n.Kind {
	case jsonx.String:
		if p, ok := sc.Get("pattern"); ok && !s.re[p.S].MatchString(n.S) {
			fail("不匹配 %s", p.S)
		}
	case jsonx.Number:
		if m, ok := sc.Get("minimum"); ok && n.N < m.N {
			fail("不能小于 %v", m.N)
		}
		if m, ok := sc.Get("exclusiveMinimum"); ok && n.N <= m.N {
			fail("必须大于 %v", m.N)
		}
	case jsonx.Array:
		if m, ok := sc.Get("minItems"); ok && float64(len(n.Items)) < m.N {
			fail("至少 %v 项", m.N)
		}
		if u, ok := sc.Get("uniqueItems"); ok && u.B {
			for i := range n.Items {
				for j := range i {
					if equal(n.Items[i], n.Items[j]) {
						fail("第 %d 项与第 %d 项重复", j, i)
					}
				}
			}
		}
		if items, ok := sc.Get("items"); ok {
			for i, it := range n.Items {
				s.validate(items, it, fmt.Sprintf("%s[%d]", at, i), errs)
			}
		}
	case jsonx.Object:
		props, _ := sc.Get("properties")
		if req, ok := sc.Get("required"); ok {
			for _, r := range req.Items {
				if _, ok := n.Get(r.S); !ok {
					fail("缺少 %s", r.S)
				}
			}
		}
		names, hasNames := sc.Get("propertyNames")
		extra, hasExtra := sc.Get("additionalProperties")
		for i, k := range n.Keys {
			p := k
			if at != "" {
				p = at + "." + k
			}
			if hasNames {
				s.validate(names, jsonx.Str(k), p, errs)
			}
			if ps, ok := props.Get(k); ok {
				s.validate(ps, n.Vals[i], p, errs)
				continue
			}
			switch {
			case hasExtra && extra.Kind == jsonx.Bool && !extra.B:
				fail("不允许的字段 %s", k)
			case hasExtra && extra.Kind == jsonx.Object:
				s.validate(extra, n.Vals[i], p, errs)
			}
		}
	}
}

func is(ty string, n jsonx.Node) bool {
	switch ty {
	case "null":
		return n.Kind == jsonx.Null
	case "boolean":
		return n.Kind == jsonx.Bool
	case "number":
		return n.Kind == jsonx.Number
	case "integer":
		return n.Kind == jsonx.Number && n.N == math.Trunc(n.N)
	case "string":
		return n.Kind == jsonx.String
	case "array":
		return n.Kind == jsonx.Array
	case "object":
		return n.Kind == jsonx.Object
	}
	return false
}

func equal(a, b jsonx.Node) bool { return string(jsonx.Encode(a)) == string(jsonx.Encode(b)) }
