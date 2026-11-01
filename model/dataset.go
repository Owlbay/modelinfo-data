package model

import (
	"bytes"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path"
	"slices"
	"strings"

	"github.com/Owlbay/modelinfo-data/jsonx"
)

// Issue 是数据目录中的一个问题，定位到文件与行。
type Issue struct {
	File string // 相对数据目录的路径，如 models/gpt-5.json
	Line int    // 从 1 开始；0 表示整个文件
	Path string // 字段路径，如 providers[0].id；空表示整个文件
	Msg  string // 说明（中文）
}

// String 形如「models/x.json:12: providers[0].id：供应商 foo 不存在」。
func (i Issue) String() string {
	loc := i.File
	if i.Line > 0 {
		loc += fmt.Sprintf(":%d", i.Line)
	}
	if i.Path != "" {
		return loc + ": " + i.Path + "：" + i.Msg
	}
	return loc + ": " + i.Msg
}

// Issues 是问题列表，可作为 error 使用。
type Issues []Issue

func (is Issues) Error() string {
	s := make([]string, len(is))
	for i, x := range is {
		s[i] = x.String()
	}
	return strings.Join(s, "\n")
}

// File 是数据目录中的一个文件：相对路径、原文与保留键顺序的树。
type File struct {
	Path string
	Raw  []byte
	Node jsonx.Node
}

// ModelFile、VendorFile、ProviderFile 是通过校验的文件及其解析结果。
type (
	ModelFile struct {
		File
		Model Model
	}
	VendorFile struct {
		File
		Vendor Vendor
	}
	ProviderFile struct {
		File
		Provider ProviderInfo
	}
)

// Dataset 是整个数据目录中通过校验的数据（按文件名排序）与引用目录。
type Dataset struct {
	Vendors   []VendorFile
	Providers []ProviderFile
	Models    []ModelFile
	Registry  *Registry
}

// LoadDir 读取并校验数据目录 dir（含 vendors/、providers/、models/），见 Load。
func LoadDir(dir string) (*Dataset, Issues) { return Load(os.DirFS(dir)) }

// Load 读取并校验数据目录中的全部文件：JSON 语法、结构与字段顺序、取值规则、文件名等于 id、
// 引用的厂商与供应商存在、模型 ID 与别名不冲突、文件为标准格式（Format 的输出）。
// 返回通过校验的数据与全部问题；没有问题时 Issues 为 nil。
func Load(fsys fs.FS) (*Dataset, Issues) {
	ds := &Dataset{Registry: &Registry{Vendors: map[string]bool{}, Providers: map[string]bool{}}}
	var issues Issues
	report := func(f File, err error) {
		issues = append(issues, toIssues(f, err)...)
	}
	files := map[Kind][]File{}
	for _, k := range Kinds {
		list, is := readDir(fsys, k.Dir())
		issues = append(issues, is...)
		files[k] = list
		// 先按文件名登记 ID，避免一个厂商文件有误时引用它的模型连带报错（该文件自身仍会报错）
		for _, f := range list {
			id := strings.TrimSuffix(path.Base(f.Path), ".json")
			switch k {
			case KindVendor:
				ds.Registry.Vendors[id] = true
			case KindProvider:
				ds.Registry.Providers[id] = true
			}
		}
	}
	if len(files[KindVendor]) == 0 || len(files[KindModel]) == 0 {
		issues = append(issues, Issue{Msg: "没有找到 vendors/*.json 或 models/*.json，请在数据仓库根目录运行"})
	}

	// 先解析全部模型，构建 ID 与别名的命名空间
	var all []Model
	for _, f := range files[KindModel] {
		if m, _, err := Decode(f.Raw); err == nil {
			all = append(all, m)
		}
	}
	ds.Registry.Names = NewNames(all)

	for _, k := range Kinds {
		for _, f := range files[k] {
			checkFile(ds, k, &f, report)
			if msg, line := formatProblem(f.Raw); msg != "" {
				issues = append(issues, Issue{File: f.Path, Line: line, Msg: msg})
			}
		}
	}
	slices.SortStableFunc(issues, func(a, b Issue) int {
		if c := strings.Compare(a.File, b.File); c != 0 {
			return c
		}
		return a.Line - b.Line
	})
	if len(issues) == 0 {
		issues = nil
	}
	return ds, issues
}

// checkFile 检查一个文件（含文件名等于 id），通过时加入数据集。
func checkFile(ds *Dataset, k Kind, f *File, report func(File, error)) {
	var (
		err error
		id  string
		add func()
	)
	switch k {
	case KindVendor:
		var v Vendor
		v, f.Node, err = CheckVendor(f.Raw)
		id, add = v.ID, func() { ds.Vendors = append(ds.Vendors, VendorFile{*f, v}) }
	case KindProvider:
		var p ProviderInfo
		p, f.Node, err = CheckProvider(f.Raw, ds.Registry)
		id, add = p.ID, func() { ds.Providers = append(ds.Providers, ProviderFile{*f, p}) }
	default:
		var m Model
		m, f.Node, err = CheckModel(f.Raw, ds.Registry)
		id, add = m.ID, func() { ds.Models = append(ds.Models, ModelFile{*f, m}) }
	}
	if err == nil && id+".json" != path.Base(f.Path) {
		err = Problems{fmt.Sprintf("id：文件名应为 %s.json（与 id 一致）", id)}
	}
	if err != nil {
		report(*f, err)
		return
	}
	add()
}

// readDir 读取 dir 下的 .json 文件（按文件名排序）；其他文件报为问题，以 . 开头的隐藏文件忽略。
func readDir(fsys fs.FS, dir string) ([]File, Issues) {
	entries, err := fs.ReadDir(fsys, dir)
	if errors.Is(err, fs.ErrNotExist) {
		return nil, nil
	}
	if err != nil {
		return nil, Issues{{File: dir, Msg: err.Error()}}
	}
	var out []File
	var issues Issues
	for _, e := range entries {
		name := e.Name()
		p := dir + "/" + name
		switch {
		case strings.HasPrefix(name, "."):
		case e.IsDir() || !strings.HasSuffix(name, ".json"):
			issues = append(issues, Issue{File: p, Msg: fmt.Sprintf("%s/ 下只能放 .json 文件，每个实体一个", dir)})
		default:
			raw, err := fs.ReadFile(fsys, p)
			if err != nil {
				issues = append(issues, Issue{File: p, Msg: err.Error()})
				continue
			}
			out = append(out, File{Path: p, Raw: raw})
		}
	}
	return out, issues
}

// toIssues 把 CheckXxx 的错误转为带行号的问题。
func toIssues(f File, err error) Issues {
	var se *SyntaxError
	if errors.As(err, &se) {
		return Issues{{File: f.Path, Line: se.Line, Msg: se.Error()}}
	}
	var p Problems
	if !errors.As(err, &p) {
		return Issues{{File: f.Path, Line: 1, Msg: err.Error()}}
	}
	out := make(Issues, 0, len(p))
	for _, s := range p {
		at, msg, ok := strings.Cut(s, "：")
		if !ok {
			at, msg = "", s
		}
		if at == "(根)" {
			at = ""
		}
		out = append(out, Issue{File: f.Path, Line: jsonx.Locate(f.Raw, at), Path: at, Msg: msg})
	}
	return out
}

// formatProblem 检查原文是否为标准写法（缩进、数值写法、换行）；字段顺序由 Shape 报告。
// 是标准写法或无法解析时返回空串；否则返回说明与第一处不同所在的行。
func formatProblem(raw []byte) (string, int) {
	n, err := jsonx.Parse(raw)
	if err != nil {
		return "", 0
	}
	want := jsonx.Indent(n)
	if bytes.Equal(want, raw) {
		return "", 0
	}
	const fix = "（运行 go run ./cmd/check --fix 自动修复）"
	if i := bytes.Index(raw, []byte("\r\n")); i >= 0 {
		return "格式不标准：换行应为 LF，不用 CRLF" + fix, jsonx.LineAt(raw, i)
	}
	if bytes.Equal(want[:len(want)-1], raw) {
		return "格式不标准：文件末尾应有一个换行" + fix, bytes.Count(raw, []byte{'\n'}) + 1
	}
	got, exp := strings.Split(string(raw), "\n"), strings.Split(string(want), "\n")
	for i := range exp {
		g := ""
		if i < len(got) {
			g = got[i]
		}
		if g != exp[i] {
			return fmt.Sprintf("格式不标准：第 %d 行应为 %s，实际为 %s%s", i+1, quote(exp[i]), quote(g), fix), i + 1
		}
	}
	return "格式不标准：文件末尾有多余内容" + fix, len(exp)
}

func quote(s string) string {
	r := []rune(s)
	if len(r) > 80 {
		s = string(r[:80]) + "…"
	}
	return "`" + s + "`"
}
