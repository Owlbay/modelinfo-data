package main

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// fixture 把仓库中的一组真实数据复制到临时目录，返回目录与模型文件路径。
func fixture(t *testing.T) (string, string) {
	t.Helper()
	dir := t.TempDir()
	for _, f := range []string{"vendors/alibaba.json", "providers/alibaba.json", "models/qwen3.8-max.json"} {
		raw, err := os.ReadFile(filepath.Join("..", "..", f))
		if err != nil {
			t.Fatal(err)
		}
		dst := filepath.Join(dir, f)
		if err := os.MkdirAll(filepath.Dir(dst), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(dst, raw, 0o644); err != nil {
			t.Fatal(err)
		}
	}
	return dir, filepath.Join(dir, "models", "qwen3.8-max.json")
}

func runCheck(t *testing.T, env map[string]string, args ...string) (int, string, string) {
	t.Helper()
	var out, errb bytes.Buffer
	code := run(args, &out, &errb, func(k string) string { return env[k] })
	return code, out.String(), errb.String()
}

func edit(t *testing.T, path, from, to string) {
	t.Helper()
	raw, _ := os.ReadFile(path)
	s := strings.Replace(string(raw), from, to, 1)
	if s == string(raw) {
		t.Fatalf("替换没有生效：%q", from)
	}
	os.WriteFile(path, []byte(s), 0o644)
}

func TestCheckPasses(t *testing.T) {
	dir, _ := fixture(t)
	code, out, errs := runCheck(t, nil, "--dir", dir)
	if code != 0 || !strings.Contains(out, "校验通过：1 个模型、1 个厂商、1 个供应商") {
		t.Fatalf("退出码 %d\n%s%s", code, out, errs)
	}
}

func TestCheckReportsAndAnnotates(t *testing.T) {
	dir, m := fixture(t)
	edit(t, m, `"owned_by": "alibaba"`, `"owned_by": "nobody"`)
	edit(t, m, "  \"name\": \"Qwen3.8-Max\",\n", "")
	code, out, errs := runCheck(t, map[string]string{"GITHUB_ACTIONS": "true"}, "--dir", dir)
	if code != 1 {
		t.Fatalf("退出码 %d，应为 1", code)
	}
	if !strings.Contains(errs, "models/qwen3.8-max.json:1: 缺少字段 name") {
		t.Errorf("应报告缺少字段：\n%s", errs)
	}
	if !strings.Contains(out, "::error file=models/qwen3.8-max.json,line=1,title=数据校验::缺少字段 name") {
		t.Errorf("应输出 GitHub 注解：\n%s", out)
	}
	// 结构有误时先只报结构问题；补上 name 后再报引用问题
	edit(t, m, `"owned_by": "nobody",`, `"owned_by": "nobody",
  "name": "Qwen3.8-Max",`)
	_, out, errs = runCheck(t, map[string]string{"GITHUB_ACTIONS": "true"}, "--dir", dir)
	if !strings.Contains(errs, "models/qwen3.8-max.json:5: owned_by：厂商 nobody 不存在") ||
		!strings.Contains(out, "::error file=models/qwen3.8-max.json,line=5,title=数据校验：owned_by::owned_by：厂商 nobody 不存在") {
		t.Errorf("应报告并标注不存在的厂商：\n%s\n%s", out, errs)
	}
}

func TestCheckFix(t *testing.T) {
	dir, m := fixture(t)
	want, _ := os.ReadFile(m)
	edit(t, m, `"object": "model",
  "created": 1785715200,`, `"created": 1785715200, "object": "model",`)
	edit(t, m, `"input": 2,`, `"input": 2.00,`)
	code, _, errs := runCheck(t, nil, "--dir", dir)
	if code != 1 || !strings.Contains(errs, "字段顺序应为") || !strings.Contains(errs, "格式不标准") || !strings.Contains(errs, "--fix") {
		t.Fatalf("应报告格式问题（退出码 %d）：\n%s", code, errs)
	}
	code, out, errs := runCheck(t, nil, "--fix", "--dir", dir)
	if code != 0 || !strings.Contains(out, "已改写 models/qwen3.8-max.json") {
		t.Fatalf("--fix 后应通过（退出码 %d）：\n%s%s", code, out, errs)
	}
	if got, _ := os.ReadFile(m); !bytes.Equal(got, want) {
		t.Fatalf("--fix 应还原为标准格式：\n%s", got)
	}
}

func TestCheckOnlyFiles(t *testing.T) {
	dir, m := fixture(t)
	edit(t, m, `"owned_by": "alibaba"`, `"owned_by": "nobody"`)
	code, _, _ := runCheck(t, nil, "--dir", dir, filepath.Join(dir, "vendors", "alibaba.json"))
	if code != 0 {
		t.Fatalf("只看 vendors/alibaba.json 时应通过，退出码 %d", code)
	}
}

func TestEscape(t *testing.T) {
	if got := escapeProp("a:b,c%\n"); got != "a%3Ab%2Cc%25%0A" {
		t.Fatal(got)
	}
}
