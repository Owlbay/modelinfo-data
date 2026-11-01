package jsonschema

import (
	"testing"

	"github.com/Owlbay/modelinfo-data/jsonx"
)

func TestUnknownKeywordRejected(t *testing.T) {
	if _, err := Parse([]byte(`{"type":"object","oneOf":[]}`)); err == nil {
		t.Fatal("未实现的关键字应报错")
	}
}

func TestValidate(t *testing.T) {
	s, err := Parse([]byte(`{"type":"object","additionalProperties":false,"required":["a"],
		"properties":{"a":{"type":["integer","null"],"exclusiveMinimum":0},"b":{"$ref":"#/definitions/e"}},
		"definitions":{"e":{"enum":["x","y"]}}}`))
	if err != nil {
		t.Fatal(err)
	}
	for doc, bad := range map[string]bool{
		`{"a":1}`: false, `{"a":null,"b":"y"}`: false,
		`{}`: true, `{"a":0}`: true, `{"a":1.5}`: true, `{"a":1,"b":"z"}`: true, `{"a":1,"c":1}`: true,
	} {
		n, _ := jsonx.Parse([]byte(doc))
		if errs := s.Validate(n); (len(errs) > 0) != bad {
			t.Errorf("%s：%v", doc, errs)
		}
	}
}
