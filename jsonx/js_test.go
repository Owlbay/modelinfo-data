package jsonx

import (
	"math"
	"testing"
)

// 期望值取自 Node.js：String(x) 与 JSON.stringify(s)。
func TestFormatNumber(t *testing.T) {
	cases := []struct {
		in   float64
		want string
	}{
		{0, "0"},
		{math.Copysign(0, -1), "0"},
		{5, "5"},
		{-3, "-3"},
		{0.5, "0.5"},
		{0.15, "0.15"},
		{6.25, "6.25"},
		{0.000035, "0.000035"},
		{0.000001, "0.000001"},
		{0.0000001, "1e-7"},
		{1.5e-7, "1.5e-7"},
		{123456789012, "123456789012"},
		{1e21, "1e+21"},
		{1.25e21, "1.25e+21"},
		{100000000000000000000, "100000000000000000000"},
		{0.30000000000000004, "0.30000000000000004"},
		{math.NaN(), "null"},
		{math.Inf(1), "null"},
	}
	for _, c := range cases {
		if got := FormatNumber(c.in); got != c.want {
			t.Errorf("FormatNumber(%v) = %s，期望 %s", c.in, got, c.want)
		}
	}
}

func TestAppendString(t *testing.T) {
	cases := []struct{ in, want string }{
		{"plain", `"plain"`},
		{`a"b\c`, `"a\"b\\c"`},
		{"<a href='x'>&</a>", `"<a href='x'>&</a>"`},
		{"line\nbreak\ttab\r\b\f", `"line\nbreak\ttab\r\b\f"`},
		{"\x00\x1f", `"\u0000\u001f"`},
		{"  ", "\"  \""},
		{"中文 — ’", `"中文 — ’"`},
	}
	for _, c := range cases {
		if got := string(appendString(nil, c.in)); got != c.want {
			t.Errorf("appendString(%q) = %s，期望 %s", c.in, got, c.want)
		}
	}
}
