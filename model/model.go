// Package model 定义 ModelInfo 数据仓库（github.com/Owlbay/modelinfo-data）的数据格式与校验：
// 模型（models/*.json）、模型厂商（vendors/*.json）、供应商（providers/*.json）。
//
// 结构体字段的声明顺序就是数据文件中键的标准顺序；带 omitempty 的字段可以省略，其余字段必须出现
// （未知值写 null 或空数组）。字段含义见仓库的 docs/fields.md。
//
// 单个文档：Check（= 语法 + Shape + Decode + Validate）；整个目录：LoadDir / Load（另查文件名、
// 跨文件引用、别名冲突与标准格式）；标准格式：Format。只依赖标准库。
package model

// Price 是一种币种的官方标价。
type Price struct {
	Currency   string   `json:"currency"`
	Unit       string   `json:"unit"`
	Input      *float64 `json:"input"`
	Output     *float64 `json:"output"`
	CacheRead  *float64 `json:"cache_read"`
	CacheWrite *float64 `json:"cache_write"`
}

// Limits 是 Token 上限。
type Limits struct {
	Context *int64 `json:"context"`
	Input   *int64 `json:"input"`
	Output  *int64 `json:"output"`
}

// Modalities 是输入、输出模态。
type Modalities struct {
	Input  []string `json:"input"`
	Output []string `json:"output"`
}

// Provider 是能调用该模型的一个供应商接入点。
type Provider struct {
	ID               string             `json:"id"`
	Model            string             `json:"model"`
	BaseURL          string             `json:"base_url"`
	Endpoints        map[string]string  `json:"endpoints"`
	ReasoningParam   *string            `json:"reasoning_param"`
	ReasoningParams  map[string]*string `json:"reasoning_params,omitempty"`
	ReasoningEfforts []string           `json:"reasoning_efforts,omitempty"`
	Limits           *Limits            `json:"limits,omitempty"`
	Modalities       *Modalities        `json:"modalities,omitempty"`
	Features         []string           `json:"features,omitempty"`
	Prices           *[]Price           `json:"prices,omitempty"`
}

// Source 是一条官方来源。
type Source struct {
	Title       string `json:"title"`
	URL         string `json:"url"`
	RetrievedAt string `json:"retrieved_at"`
}

// Model 是一个模型的完整数据（数据文件与库中保存的文档）。
type Model struct {
	ID      string `json:"id"`
	Object  string `json:"object"`
	Created *int64 `json:"created"`
	OwnedBy string `json:"owned_by"`
	Name    string `json:"name"`
	API     struct {
		Types []string `json:"types"`
		SDK   []string `json:"sdk"`
	} `json:"api"`
	Limits     Limits     `json:"limits"`
	Modalities Modalities `json:"modalities"`
	Reasoning  struct {
		Supported *bool    `json:"supported"`
		Efforts   []string `json:"efforts"`
	} `json:"reasoning"`
	Prices   []Price  `json:"prices"`
	Features []string `json:"features"`
	Info     struct {
		Status          string  `json:"status"`
		ReleaseDate     *string `json:"release_date"`
		KnowledgeCutoff *string `json:"knowledge_cutoff"`
		Description     *string `json:"description"`
		Docs            *string `json:"docs"`
		VerifiedAt      *string `json:"verified_at"`
	} `json:"info"`
	Providers    []Provider             `json:"providers"`
	Aliases      []string               `json:"aliases"`
	Translations map[string]Translation `json:"translations,omitempty"`
	Sources      []Source               `json:"sources"`
}

// Translation 是简介的译文。
type Translation struct {
	Description string `json:"description"`
}

// Vendor 是模型厂商（owned_by）。
type Vendor struct {
	ID   string  `json:"id"`
	Name string  `json:"name"`
	URL  string  `json:"url"`
	Icon *string `json:"icon,omitempty"`
}

// ProviderInfo 是供应商目录中的一项（模型 providers[].id 引用它）。
type ProviderInfo struct {
	ID      string  `json:"id"`
	Name    string  `json:"name"`
	URL     *string `json:"url"`
	OwnedBy *string `json:"owned_by"`
}
