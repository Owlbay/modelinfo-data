package jsonx

import (
	"bytes"
	"encoding/json"
	"errors"
	"strconv"
	"strings"
)

// Locate 返回 data 中路径 path 所在的行号（从 1 开始），用于把校验问题标到具体行。
// 路径写法与校验问题一致：键用 . 连接，数组下标用 [n]，例如 providers[0].endpoints.chat。
// 对象成员定位到键所在行，数组元素定位到元素开头。路径不存在时退到最长的存在前缀；
// 都不存在（或 path 为空、JSON 无法解析）时返回 1。
func Locate(data []byte, path string) int {
	offs := map[string]int{}
	dec := json.NewDecoder(bytes.NewReader(data))
	_ = walk(dec, data, "", offs)
	for p := path; p != ""; p = parent(p) {
		if o, ok := offs[p]; ok {
			return LineAt(data, o)
		}
	}
	return 1
}

// LineAt 返回字节偏移 off 所在的行号（从 1 开始）。
func LineAt(data []byte, off int) int {
	if off > len(data) {
		off = len(data)
	}
	if off < 0 {
		off = 0
	}
	return bytes.Count(data[:off], []byte{'\n'}) + 1
}

// SyntaxLine 返回 JSON 语法错误所在的行号；err 不是语法错误时返回 0。
func SyntaxLine(data []byte, err error) int {
	var se *json.SyntaxError
	if errors.As(err, &se) {
		return LineAt(data, int(se.Offset))
	}
	var ue *json.UnmarshalTypeError
	if errors.As(err, &ue) {
		return LineAt(data, int(ue.Offset))
	}
	return 0
}

// next 返回 off 之后跳过空白与分隔符（, :）的第一个字节位置，即下一个记号的开头。
func next(data []byte, off int) int {
	for off < len(data) && strings.IndexByte(" \t\r\n,:", data[off]) >= 0 {
		off++
	}
	return off
}

// walk 读取一个值，记录其中每个成员的起始偏移。
func walk(dec *json.Decoder, data []byte, path string, offs map[string]int) error {
	tok, err := dec.Token()
	if err != nil {
		return err
	}
	d, ok := tok.(json.Delim)
	if !ok {
		return nil
	}
	switch d {
	case '{':
		for dec.More() {
			start := next(data, int(dec.InputOffset()))
			kt, err := dec.Token()
			if err != nil {
				return err
			}
			k, _ := kt.(string)
			p := k
			if path != "" {
				p = path + "." + k
			}
			offs[p] = start
			if err := walk(dec, data, p, offs); err != nil {
				return err
			}
		}
	case '[':
		for i := 0; dec.More(); i++ {
			p := path + "[" + strconv.Itoa(i) + "]"
			offs[p] = next(data, int(dec.InputOffset()))
			if err := walk(dec, data, p, offs); err != nil {
				return err
			}
		}
	}
	_, err = dec.Token() // 结束符
	return err
}

// parent 去掉路径的最后一段：a.b[0].c → a.b[0] → a.b → a → ""。
func parent(p string) string {
	i := strings.LastIndexAny(p, ".[")
	if i < 0 {
		return ""
	}
	return p[:i]
}
