package jsonx

import "testing"

func TestLocate(t *testing.T) {
	doc := []byte(`{
  "id": "x",
  "api": {
    "types": [
      "chat",
      "messages"
    ]
  },
  "providers": [
    {
      "id": "a",
      "endpoints": {
        "chat": "POST /v1/chat/completions"
      }
    }
  ]
}
`)
	cases := []struct {
		path string
		want int
	}{
		{"id", 2},
		{"api", 3},
		{"api.types", 4},
		{"api.types[1]", 6},
		{"providers[0]", 10},
		{"providers[0].endpoints.chat", 13},
		{"providers[0].endpoints.messages", 12}, // 不存在：退到 endpoints
		{"providers[3].id", 9},                  // 不存在：退到 providers
		{"nope", 1},
		{"", 1},
	}
	for _, c := range cases {
		if got := Locate(doc, c.path); got != c.want {
			t.Errorf("Locate(%q) = %d，期望 %d", c.path, got, c.want)
		}
	}
}

func TestSyntaxLine(t *testing.T) {
	doc := []byte("{\n  \"a\": 1,\n  \"b\": ,\n}\n")
	_, err := Parse(doc)
	if err == nil {
		t.Fatal("应报语法错误")
	}
	if got := SyntaxLine(doc, err); got != 3 {
		t.Fatalf("SyntaxLine = %d，期望 3（%v）", got, err)
	}
}
