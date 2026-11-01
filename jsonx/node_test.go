package jsonx

import "testing"

func TestParseEncodeKeepsOrder(t *testing.T) {
	in := `{"b":1,"a":[0.0000001,"x ",true,null],"c":{"z":{},"y":[]}}`
	n, err := Parse([]byte(in))
	if err != nil {
		t.Fatal(err)
	}
	want := "{\"b\":1,\"a\":[1e-7,\"x \",true,null],\"c\":{\"z\":{},\"y\":[]}}"
	if got := string(Encode(n)); got != want {
		t.Fatalf("Encode = %s\n期望 %s", got, want)
	}
}

func TestParseErrors(t *testing.T) {
	for _, in := range []string{`{"a":1,"a":2}`, `{"a":1} x`, `{`} {
		if _, err := Parse([]byte(in)); err == nil {
			t.Errorf("Parse(%s) 应报错", in)
		}
	}
}

func TestPickSet(t *testing.T) {
	n, _ := Parse([]byte(`{"a":1,"b":2,"c":3}`))
	if got := string(Encode(n.Pick("c", "a"))); got != `{"a":1,"c":3}` {
		t.Fatalf("Pick = %s", got)
	}
	if got := string(Encode(n.Set("b", Str("x")).Set("d", Str("y")))); got != `{"a":1,"b":"x","c":3,"d":"y"}` {
		t.Fatalf("Set = %s", got)
	}
	if got := string(Encode(n)); got != `{"a":1,"b":2,"c":3}` {
		t.Fatalf("Set 不应修改原节点：%s", got)
	}
}
