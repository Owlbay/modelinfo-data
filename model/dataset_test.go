package model

import (
	"strings"
	"testing"
	"testing/fstest"
)

const vendorX = "{\n  \"id\": \"x\",\n  \"name\": \"X\",\n  \"url\": \"https://x.example\"\n}\n"

func modelDoc(id, owner, aliases string) string {
	return `{
  "id": "` + id + `",
  "object": "model",
  "created": null,
  "owned_by": "` + owner + `",
  "name": "M",
  "api": {
    "types": [
      "chat"
    ],
    "sdk": []
  },
  "limits": {
    "context": null,
    "input": null,
    "output": null
  },
  "modalities": {
    "input": [
      "text"
    ],
    "output": [
      "text"
    ]
  },
  "reasoning": {
    "supported": null,
    "efforts": []
  },
  "prices": [],
  "features": [],
  "info": {
    "status": "active",
    "release_date": null,
    "knowledge_cutoff": null,
    "description": null,
    "docs": null,
    "verified_at": null
  },
  "providers": [],
  "aliases": [` + aliases + `],
  "sources": [
    {
      "title": "X",
      "url": "https://x.example/m",
      "retrieved_at": "2026-11-01"
    }
  ]
}
`
}

func TestLoad(t *testing.T) {
	fsys := fstest.MapFS{
		"vendors/x.json":   {Data: []byte(vendorX)},
		"providers/p.json": {Data: []byte("{\n  \"id\": \"p\",\n  \"name\": \"P\",\n  \"url\": null,\n  \"owned_by\": \"ghost\"\n}\n")},
		"models/a.json":    {Data: []byte(modelDoc("a", "x", ""))},
		"models/b.json":    {Data: []byte(modelDoc("b", "nobody", ""))},
		"models/c.json":    {Data: []byte(modelDoc("wrong", "x", ""))},
		"models/d.json":    {Data: []byte(strings.Replace(modelDoc("d", "x", ""), "  ", "    ", -1))},
		"models/e.json":    {Data: []byte(modelDoc("e", "x", `"a"`))},
		"models/f.json":    {Data: []byte("{\n  \"id\": \"f\",\n}\n")},
		"models/g.json":    {Data: []byte(strings.Replace(modelDoc("g", "x", ""), "  \"name\": \"M\",\n", "", 1))},
		"models/README.md": {Data: []byte("x")},
		"models/.DS_Store": {Data: []byte("x")},
	}
	ds, issues := Load(fsys)
	want := []string{
		"providers/p.json:5: owned_by：厂商 ghost 不存在",
		"models/b.json:5: owned_by：厂商 nobody 不存在",
		"models/c.json:2: id：文件名应为 wrong.json",
		"models/d.json:2: 格式不标准：第 2 行应为 `  \"id\": \"d\",`",
		"models/e.json:41: aliases[0]：别名 a 已被模型 a 使用",
		"models/a.json:2: id：a 已是模型 e 的别名",
		"models/f.json:2: 第 2 行：JSON 语法错误",
		"models/g.json:1: 缺少字段 name",
		"models/README.md: models/ 下只能放 .json 文件",
	}
	got := issues.Error()
	for _, w := range want {
		if !strings.Contains(got, w) {
			t.Errorf("缺少问题 %q\n全部问题：\n%s", w, got)
		}
	}
	if strings.Contains(got, ".DS_Store") {
		t.Errorf("隐藏文件应忽略：\n%s", got)
	}
	// 只有 d 通过（格式不标准不影响内容校验）；a 与 e 的别名冲突双方都报
	if len(ds.Models) != 1 || ds.Models[0].Model.ID != "d" {
		t.Errorf("通过校验的模型：%v", ds.Models)
	}
}
