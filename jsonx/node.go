package jsonx

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"strconv"
)

// Kind 是节点类型。
type Kind uint8

const (
	Null Kind = iota
	Bool
	Number
	String
	Array
	Object
)

// Node 是一个 JSON 值。对象保留键的原始顺序；数值保存为 float64，输出按 JavaScript 规则。
type Node struct {
	Kind  Kind
	B     bool
	N     float64
	S     string
	Items []Node   // Array
	Keys  []string // Object：键顺序
	Vals  []Node   // Object：与 Keys 一一对应
}

// Parse 解析一个完整的 JSON 文本；对象中重复的键报错。
func Parse(data []byte) (Node, error) {
	dec := json.NewDecoder(bytes.NewReader(data))
	dec.UseNumber()
	n, err := parse(dec)
	if err != nil {
		return Node{}, err
	}
	if _, err := dec.Token(); !errors.Is(err, io.EOF) {
		return Node{}, errors.New("JSON 之后还有多余内容")
	}
	return n, nil
}

func parse(dec *json.Decoder) (Node, error) {
	tok, err := dec.Token()
	if err != nil {
		return Node{}, err
	}
	switch v := tok.(type) {
	case nil:
		return Node{Kind: Null}, nil
	case bool:
		return Node{Kind: Bool, B: v}, nil
	case json.Number:
		f, err := strconv.ParseFloat(string(v), 64)
		if err != nil {
			return Node{}, fmt.Errorf("数值 %s：%w", v, err)
		}
		return Node{Kind: Number, N: f}, nil
	case string:
		return Node{Kind: String, S: v}, nil
	case json.Delim:
		if v == '[' {
			n := Node{Kind: Array, Items: []Node{}}
			for dec.More() {
				item, err := parse(dec)
				if err != nil {
					return Node{}, err
				}
				n.Items = append(n.Items, item)
			}
			_, err := dec.Token()
			return n, err
		}
		n := Node{Kind: Object}
		seen := map[string]bool{}
		for dec.More() {
			kt, err := dec.Token()
			if err != nil {
				return Node{}, err
			}
			k := kt.(string)
			if seen[k] {
				return Node{}, fmt.Errorf("键 %q 重复", k)
			}
			seen[k] = true
			val, err := parse(dec)
			if err != nil {
				return Node{}, err
			}
			n.Keys = append(n.Keys, k)
			n.Vals = append(n.Vals, val)
		}
		_, err := dec.Token()
		return n, err
	}
	return Node{}, fmt.Errorf("未知的 JSON 记号 %v", tok)
}

// Get 返回对象中键 k 的值；不是对象或没有该键时 ok 为 false。
func (n Node) Get(k string) (Node, bool) {
	for i, key := range n.Keys {
		if key == k {
			return n.Vals[i], true
		}
	}
	return Node{}, false
}

// Set 设置对象中键 k 的值：已有的原位替换，没有的追加到末尾。返回新节点，不修改 n。
func (n Node) Set(k string, v Node) Node {
	out := n.clone()
	for i, key := range out.Keys {
		if key == k {
			out.Vals[i] = v
			return out
		}
	}
	out.Keys = append(out.Keys, k)
	out.Vals = append(out.Vals, v)
	return out
}

// Pick 只保留给定的键，按 n 中的原有顺序。返回新节点。
func (n Node) Pick(keys ...string) Node {
	want := map[string]bool{}
	for _, k := range keys {
		want[k] = true
	}
	out := Node{Kind: Object}
	for i, k := range n.Keys {
		if want[k] {
			out.Keys = append(out.Keys, k)
			out.Vals = append(out.Vals, n.Vals[i])
		}
	}
	return out
}

func (n Node) clone() Node {
	out := n
	out.Keys = append([]string(nil), n.Keys...)
	out.Vals = append([]Node(nil), n.Vals...)
	return out
}

// Str 构造字符串节点。
func Str(s string) Node { return Node{Kind: String, S: s} }

// Encode 按 JSON.stringify 的规则输出（无缩进）。
func Encode(n Node) []byte { return Append(nil, n) }

// Append 把 n 的编码追加到 b。
func Append(b []byte, n Node) []byte {
	switch n.Kind {
	case Null:
		return append(b, "null"...)
	case Bool:
		if n.B {
			return append(b, "true"...)
		}
		return append(b, "false"...)
	case Number:
		return append(b, FormatNumber(n.N)...)
	case String:
		return appendString(b, n.S)
	case Array:
		b = append(b, '[')
		for i, it := range n.Items {
			if i > 0 {
				b = append(b, ',')
			}
			b = Append(b, it)
		}
		return append(b, ']')
	}
	b = append(b, '{')
	for i, k := range n.Keys {
		if i > 0 {
			b = append(b, ',')
		}
		b = appendString(b, k)
		b = append(b, ':')
		b = Append(b, n.Vals[i])
	}
	return append(b, '}')
}

// Indent 输出两空格缩进、末尾换行的文本（数据文件的标准写法）。
func Indent(n Node) []byte {
	var buf bytes.Buffer
	_ = json.Indent(&buf, Encode(n), "", "  ")
	buf.WriteByte('\n')
	return buf.Bytes()
}
