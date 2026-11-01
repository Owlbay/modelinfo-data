package model

import (
	"errors"
	"fmt"
	"strings"

	"github.com/Owlbay/modelinfo-data/jsonx"
)

// SyntaxError 是 JSON 语法错误（含重复的键、多余内容），带出错的行号。
type SyntaxError struct {
	Line int // 从 1 开始；0 表示无法定位
	Err  error
}

func (e *SyntaxError) Error() string {
	msg := "JSON 语法错误：" + explain(e.Err)
	if e.Line > 0 {
		return fmt.Sprintf("第 %d 行：%s", e.Line, msg)
	}
	return msg
}

func (e *SyntaxError) Unwrap() error { return e.Err }

// explain 把 encoding/json 的常见英文报错换成中文，并保留原文。
func explain(err error) string {
	s := err.Error()
	for _, r := range []struct{ prefix, zh string }{
		{"unexpected end of JSON input", "文件不完整（缺少右括号或引号）"},
		{"unexpected EOF", "文件不完整（缺少右括号或引号）"},
		{"invalid character '}' after object key:value pair", "多余的逗号或缺少值"},
		{"invalid character '}' looking for beginning of object key string", "对象最后一项后面不能有逗号"},
		{"invalid character ']' looking for beginning of value", "数组最后一项后面不能有逗号"},
		{"invalid character", "有非法字符（检查逗号、引号与括号）"},
	} {
		if strings.HasPrefix(s, r.prefix) {
			return r.zh + "（" + s + "）"
		}
	}
	return s
}

// parse 解析 JSON 并把语法错误包装为 *SyntaxError。
func parse(raw []byte) (jsonx.Node, error) {
	n, err := jsonx.Parse(raw)
	if err != nil {
		return n, &SyntaxError{Line: jsonx.SyntaxLine(raw, err), Err: err}
	}
	return n, nil
}

// CheckModel 完整检查一个模型文档：JSON 语法（*SyntaxError）、结构（Shape）、取值规则（Validate）。
// 通过时返回解析结果与保留键顺序的树；否则 error 为 *SyntaxError 或 Problems。
// reg 为 nil 时跳过引用检查；文件名、标准格式由 Load 检查。
func CheckModel(raw []byte, reg *Registry) (Model, jsonx.Node, error) {
	n, err := parse(raw)
	if err != nil {
		return Model{}, n, err
	}
	if p := Shape(KindModel, n); p != nil {
		return Model{}, n, p
	}
	m, _, err := Decode(raw)
	if err != nil {
		return Model{}, n, Problems{"(根)：" + err.Error()}
	}
	if err := Validate(m, reg); err != nil {
		return m, n, err
	}
	return m, n, nil
}

// CheckVendor 完整检查一个厂商文档（语法、结构、取值）。
func CheckVendor(raw []byte) (Vendor, jsonx.Node, error) {
	n, err := parse(raw)
	if err != nil {
		return Vendor{}, n, err
	}
	if p := Shape(KindVendor, n); p != nil {
		return Vendor{}, n, p
	}
	v, err := DecodeVendor(raw)
	return v, n, err
}

// CheckProvider 完整检查一个供应商文档（语法、结构、取值）；reg 不为 nil 时另查 owned_by 引用的厂商存在。
func CheckProvider(raw []byte, reg *Registry) (ProviderInfo, jsonx.Node, error) {
	n, err := parse(raw)
	if err != nil {
		return ProviderInfo{}, n, err
	}
	if p := Shape(KindProvider, n); p != nil {
		return ProviderInfo{}, n, p
	}
	v, err := DecodeProvider(raw)
	var p Problems
	if !errors.As(err, &p) && err != nil {
		return v, n, err
	}
	if v.OwnedBy != nil {
		if !reOwner.MatchString(*v.OwnedBy) {
			p = append(p, "owned_by：只能用小写字母、数字与 -")
		} else if reg != nil && !reg.Vendors[*v.OwnedBy] {
			p = append(p, fmt.Sprintf("owned_by：厂商 %s 不存在", *v.OwnedBy))
		}
	}
	if len(p) > 0 {
		return v, n, p
	}
	return v, n, nil
}
