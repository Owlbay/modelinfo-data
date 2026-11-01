// Package jsonx 是保持键顺序的 JSON：解析为有序树，按 JavaScript JSON.stringify 的规则输出。
//
// 数据文件的标准写法（两空格缩进、数值与字符串按 JSON.stringify、末尾换行）是 Indent(Parse(raw))；
// model.Format 在此基础上再把字段排成格式定义的顺序。只依赖标准库。
package jsonx

import (
	"math"
	"strconv"
	"strings"
)

// 与 JavaScript JSON.stringify 逐字节一致的数值与字符串编码。encoding/json 在三处不同：
// 浮点数的指数形式（Go 1e-07 / JS 1e-7）、U+2028/U+2029（Go 转义 / JS 原样）、HTML 字符（Go 默认转义）。

// FormatNumber 按 ECMAScript Number::toString 编码有限数值；NaN 与 ±Inf 按 JSON.stringify 写 null。
func FormatNumber(f float64) string {
	if math.IsNaN(f) || math.IsInf(f, 0) {
		return "null"
	}
	if f == 0 {
		return "0" // 含 -0
	}
	sign := ""
	if f < 0 {
		sign, f = "-", -f
	}
	// 最短往返的十进制数字串 digits 与指数 n：值 = 0.digits × 10^n
	e := strconv.FormatFloat(f, 'e', -1, 64) // d.ddde±XX
	mant, exp, _ := strings.Cut(e, "e")
	digits := strings.Replace(mant, ".", "", 1)
	x, _ := strconv.Atoi(exp)
	k, n := len(digits), x+1
	switch {
	case k <= n && n <= 21:
		return sign + digits + strings.Repeat("0", n-k)
	case 0 < n && n <= 21:
		return sign + digits[:n] + "." + digits[n:]
	case -6 < n && n <= 0:
		return sign + "0." + strings.Repeat("0", -n) + digits
	}
	s := digits[:1]
	if k > 1 {
		s += "." + digits[1:]
	}
	if n-1 >= 0 {
		return sign + s + "e+" + strconv.Itoa(n-1)
	}
	return sign + s + "e-" + strconv.Itoa(1-n)
}

// appendString 按 JSON.stringify（ES2019 起的 well-formed 版本）编码字符串。
func appendString(b []byte, s string) []byte {
	const hex = "0123456789abcdef"
	b = append(b, '"')
	for _, r := range s {
		switch r {
		case '"':
			b = append(b, '\\', '"')
		case '\\':
			b = append(b, '\\', '\\')
		case '\b':
			b = append(b, '\\', 'b')
		case '\f':
			b = append(b, '\\', 'f')
		case '\n':
			b = append(b, '\\', 'n')
		case '\r':
			b = append(b, '\\', 'r')
		case '\t':
			b = append(b, '\\', 't')
		default:
			if r < 0x20 {
				b = append(b, '\\', 'u', '0', '0', hex[r>>4], hex[r&0xf])
			} else {
				b = append(b, string(r)...)
			}
		}
	}
	return append(b, '"')
}
