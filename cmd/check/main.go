// check 校验数据仓库中的全部数据，可选把文件改写为标准格式。
//
//	go run ./cmd/check                 校验全部数据（含跨文件引用与标准格式），有问题时退出码为 1
//	go run ./cmd/check --fix           先把全部文件改写为标准格式（字段顺序、缩进、数值写法），再校验
//	go run ./cmd/check models/x.json   仍校验全部数据，只显示这些文件的问题
//
// 在 GitHub Actions 中（环境变量 GITHUB_ACTIONS=true）另输出 ::error 注解，问题会标在 PR 的文件与行上。
package main

import (
	"bytes"
	"flag"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"slices"
	"strings"

	"github.com/Owlbay/modelinfo-data/model"
)

func main() {
	os.Exit(run(os.Args[1:], os.Stdout, os.Stderr, os.Getenv))
}

const usage = `用法：go run ./cmd/check [--fix] [--dir 数据目录] [文件...]

校验 vendors/、providers/、models/ 下的全部 JSON：语法、字段与顺序、取值规则、
文件名等于 id、引用的厂商与供应商存在、ID 与别名不冲突、标准格式。

参数：
`

func run(args []string, stdout, stderr io.Writer, getenv func(string) string) int {
	fs := flag.NewFlagSet("check", flag.ContinueOnError)
	fs.SetOutput(stderr)
	fs.Usage = func() {
		fmt.Fprint(stderr, usage)
		fs.PrintDefaults()
	}
	fix := fs.Bool("fix", false, "先把文件改写为标准格式再校验")
	dir := fs.String("dir", ".", "数据目录（含 vendors/、providers/、models/）")
	if err := fs.Parse(args); err != nil {
		return 2
	}
	github := getenv("GITHUB_ACTIONS") == "true"

	only := map[string]bool{}
	for _, f := range fs.Args() {
		rel, err := filepath.Rel(*dir, f)
		if err != nil {
			rel = f
		}
		only[filepath.ToSlash(rel)] = true
	}

	if *fix {
		n, err := fixDir(*dir, stdout)
		if err != nil {
			fmt.Fprintln(stderr, "改写失败：", err)
			return 1
		}
		fmt.Fprintf(stdout, "已改写 %d 个文件为标准格式。\n", n)
	}

	ds, issues := model.LoadDir(*dir)
	if len(only) > 0 {
		issues = slices.DeleteFunc(issues, func(i model.Issue) bool { return !only[i.File] })
	}
	counts := fmt.Sprintf("%d 个模型、%d 个厂商、%d 个供应商", len(ds.Models), len(ds.Vendors), len(ds.Providers))
	if len(issues) == 0 {
		fmt.Fprintf(stdout, "校验通过：%s。\n", counts)
		return 0
	}
	files := map[string]bool{}
	format := false
	for _, i := range issues {
		files[i.File] = true
		format = format || strings.HasPrefix(i.Msg, "格式不标准") || strings.HasPrefix(i.Msg, "字段顺序")
		fmt.Fprintln(stderr, i)
		if github {
			fmt.Fprintln(stdout, annotation(i))
		}
	}
	fmt.Fprintf(stderr, "\n校验未通过：%d 个文件共 %d 个问题（通过校验的有 %s）。\n", len(files), len(issues), counts)
	if format {
		fmt.Fprintln(stderr, "格式与字段顺序问题可运行 go run ./cmd/check --fix 自动修复。")
	}
	fmt.Fprintln(stderr, "字段说明见 docs/fields.md，规则见 CONTRIBUTING.md。")
	return 1
}

// fixDir 把数据目录下全部 JSON 改写为标准格式，返回改动的文件数；语法错误的文件跳过（校验时报告）。
func fixDir(dir string, out io.Writer) (int, error) {
	changed := 0
	for _, k := range model.Kinds {
		files, err := filepath.Glob(filepath.Join(dir, k.Dir(), "*.json"))
		if err != nil {
			return changed, err
		}
		for _, f := range files {
			raw, err := os.ReadFile(f)
			if err != nil {
				return changed, err
			}
			formatted, err := model.Format(k, raw)
			if err != nil || bytes.Equal(formatted, raw) {
				continue
			}
			if err := os.WriteFile(f, formatted, 0o644); err != nil {
				return changed, err
			}
			rel, _ := filepath.Rel(dir, f)
			fmt.Fprintln(out, "已改写", filepath.ToSlash(rel))
			changed++
		}
	}
	return changed, nil
}

// annotation 生成 GitHub Actions 的错误注解（workflow command）。
func annotation(i model.Issue) string {
	props := "file=" + escapeProp(i.File)
	if i.Line > 0 {
		props += fmt.Sprintf(",line=%d", i.Line)
	}
	title := "数据校验"
	if i.Path != "" {
		title += "：" + i.Path
	}
	props += ",title=" + escapeProp(title)
	msg := i.Msg
	if i.Path != "" {
		msg = i.Path + "：" + msg
	}
	return "::error " + props + "::" + escapeData(msg)
}

// 转义规则见 https://github.com/actions/toolkit/blob/main/packages/core/src/command.ts
func escapeData(s string) string {
	return strings.NewReplacer("%", "%25", "\r", "%0D", "\n", "%0A").Replace(s)
}

func escapeProp(s string) string {
	return strings.NewReplacer("%", "%25", "\r", "%0D", "\n", "%0A", ":", "%3A", ",", "%2C").Replace(s)
}
