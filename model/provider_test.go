package model

import (
	"strings"
	"testing"
	"testing/fstest"
)

func TestDecodeProviderEntryPoints(t *testing.T) {
	cases := []struct{ name, doc, want string }{
		{"合法", `{"id":"p","name":"P","url":null,"owned_by":null,"api_key_url":"https://p.example/keys","docs_url":"https://p.example/docs","sites":[{"hosts":["api.p.cn"],"api_key_url":"https://p.cn/keys","docs_url":null}]}`, ""},
		{"api_key_url 非 https", `{"id":"p","name":"P","url":null,"owned_by":null,"api_key_url":"http://p.example/keys"}`, "api_key_url：必须是 https 网址"},
		{"docs_url 非 https", `{"id":"p","name":"P","url":null,"owned_by":null,"docs_url":"p.example"}`, "docs_url：必须是 https 网址"},
		{"sites 空数组", `{"id":"p","name":"P","url":null,"owned_by":null,"sites":[]}`, "sites：没有时省略"},
		{"hosts 为空", `{"id":"p","name":"P","url":null,"owned_by":null,"sites":[{"hosts":[],"api_key_url":"https://p.cn/keys","docs_url":null}]}`, "sites[0].hosts：至少一项"},
		{"主机带协议", `{"id":"p","name":"P","url":null,"owned_by":null,"sites":[{"hosts":["https://api.p.cn"],"api_key_url":"https://p.cn/keys","docs_url":null}]}`, "只写主机名"},
		{"主机大写", `{"id":"p","name":"P","url":null,"owned_by":null,"sites":[{"hosts":["API.p.cn"],"api_key_url":"https://p.cn/keys","docs_url":null}]}`, "只写主机名"},
		{"主机重复", `{"id":"p","name":"P","url":null,"owned_by":null,"sites":[{"hosts":["api.p.cn"],"api_key_url":"https://p.cn/keys","docs_url":null},{"hosts":["api.p.cn"],"api_key_url":"https://p.cn/k2","docs_url":null}]}`, "主机 api.p.cn 重复"},
		{"站点两项都为 null", `{"id":"p","name":"P","url":null,"owned_by":null,"sites":[{"hosts":["api.p.cn"],"api_key_url":null,"docs_url":null}]}`, "至少写一项"},
	}
	for _, c := range cases {
		_, err := DecodeProvider([]byte(c.doc))
		switch {
		case c.want == "" && err != nil:
			t.Errorf("%s：不应报错，得到 %v", c.name, err)
		case c.want != "" && (err == nil || !strings.Contains(err.Error(), c.want)):
			t.Errorf("%s：应报 %q，得到 %v", c.name, c.want, err)
		}
	}
}

func TestSiteFor(t *testing.T) {
	s := func(v string) *string { return &v }
	p := ProviderInfo{
		APIKeyURL: s("https://intl.example/keys"),
		DocsURL:   s("https://intl.example/docs"),
		Sites:     []ProviderSite{{Hosts: []string{"api.p.cn", "{workspaceid}.cn-beijing.p.com"}, APIKeyURL: s("https://p.cn/keys"), DocsURL: nil}},
	}
	for _, c := range []struct{ base, key string }{
		{"https://api.p.cn/v1", "https://p.cn/keys"},
		{"https://{WorkspaceId}.cn-beijing.p.com/compatible-mode", "https://p.cn/keys"},
		{"wss://api.p.cn/realtime", "https://p.cn/keys"},
		{"https://api.p.ai/v1", "https://intl.example/keys"},
	} {
		key, docs := p.SiteFor(c.base)
		if key == nil || *key != c.key {
			t.Errorf("%s：api_key_url = %v，期望 %s", c.base, key, c.key)
		}
		if c.key == "https://p.cn/keys" && docs != nil {
			t.Errorf("%s：站点 docs_url 为 null 时应返回 null，得到 %s", c.base, *docs)
		}
	}
	if got := BaseURLHost("https://YOUR-RESOURCE.openai.azure.com/openai/v1"); got != "your-resource.openai.azure.com" {
		t.Errorf("BaseURLHost = %s", got)
	}
}

// sites 中的主机必须有模型以它为 base_url 调用该供应商。
func TestLoadSiteHosts(t *testing.T) {
	m := strings.Replace(modelDoc("a", "x", ""), `"providers": [],`, `"providers": [
    {
      "id": "p",
      "model": "a",
      "base_url": "https://api.p.cn/v1",
      "endpoints": {
        "chat": "POST /chat/completions"
      },
      "reasoning_param": null
    }
  ],`, 1)
	prov := func(host string) string {
		return "{\n  \"id\": \"p\",\n  \"name\": \"P\",\n  \"url\": null,\n  \"owned_by\": null,\n  \"api_key_url\": \"https://p.example/keys\",\n  \"sites\": [\n    {\n      \"hosts\": [\n        \"" + host + "\"\n      ],\n      \"api_key_url\": \"https://p.cn/keys\",\n      \"docs_url\": null\n    }\n  ]\n}\n"
	}
	for _, c := range []struct{ host, want string }{
		{"api.p.cn", ""},
		{"api.p.com", "providers/p.json: sites[0].hosts[0]：没有模型以 api.p.com 为 base_url 主机调用供应商 p"},
	} {
		_, issues := Load(fstest.MapFS{
			"vendors/x.json":   {Data: []byte(vendorX)},
			"providers/p.json": {Data: []byte(prov(c.host))},
			"models/a.json":    {Data: []byte(m)},
		})
		got := ""
		if issues != nil {
			got = issues.Error()
		}
		if c.want == "" && got != "" || c.want != "" && !strings.Contains(got, c.want) {
			t.Errorf("hosts=%s：期望 %q，得到：\n%s", c.host, c.want, got)
		}
	}
}
