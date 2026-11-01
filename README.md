# ModelInfo Data

[![check](https://github.com/Owlbay/modelinfo-data/actions/workflows/check.yml/badge.svg)](https://github.com/Owlbay/modelinfo-data/actions/workflows/check.yml)
[![License: CC BY 4.0](https://img.shields.io/badge/License-CC_BY_4.0-lightgrey.svg)](LICENSE)

[modelinfo.dev](https://modelinfo.dev) 的模型数据：模型规格、价格与供应商，每个数值都来自模型厂商的官方页面。

本仓库是**唯一数据来源**：合并到 `main` 的改动会在几秒内自动同步到 modelinfo.dev 与 `https://modelinfo.dev/v1/models`。欢迎提 PR 补充新模型、更新价格或修正错误，规则见 [CONTRIBUTING.md](CONTRIBUTING.md)；发现错误但不方便改，请[提 Issue](https://github.com/Owlbay/modelinfo-data/issues/new/choose)。

> Model specs, prices and providers behind modelinfo.dev, every value sourced from official vendor pages. This repo is the single source of truth; merged PRs are synced to the site automatically. See [CONTRIBUTING.md](CONTRIBUTING.md) (English summary at the top).

## 目录

| 路径 | 内容 |
|---|---|
| `models/` | 每个模型一个 JSON 文件，文件名即模型 ID |
| `vendors/` | 模型厂商（模型的 `owned_by`） |
| `providers/` | API 供应商（模型的 `providers[].id`） |
| `docs/fields.md` | 全部字段的含义与取值 |
| `schema/` | 编辑器用的 JSON Schema |
| `cmd/check/` | 校验与格式化命令 |
| `model/`、`jsonx/` | 数据格式的 Go 包（定义、校验、标准格式），modelinfo 应用直接引用 |

## 快速开始

```sh
git clone https://github.com/Owlbay/modelinfo-data.git
cd modelinfo-data
# 编辑 models/<id>.json 后：
go run ./cmd/check --fix   # 改写为标准格式并校验全部数据（需要 Go 1.23+，无其他依赖）
```

校验通过时输出 `校验通过：N 个模型、…`；否则逐条列出 `文件:行号: 字段：原因`。

一个模型文件的样子（节选，完整字段见 [docs/fields.md](docs/fields.md)）：

```jsonc
{
  "id": "qwen3.8-max",
  "object": "model",
  "created": 1785715200,
  "owned_by": "alibaba",
  "name": "Qwen3.8-Max",
  "api": { "types": ["chat", "messages", "responses"], "sdk": ["openai"] },
  "limits": { "context": 1000000, "input": 991808, "output": 131072 },
  "modalities": { "input": ["text", "image", "video"], "output": ["text"] },
  "reasoning": { "supported": true, "efforts": [] },
  "prices": [
    { "currency": "USD", "unit": "1M_tokens", "input": 2, "output": 6, "cache_read": 0.17, "cache_write": 2.5 },
    { "currency": "CNY", "unit": "1M_tokens", "input": 12, "output": 36, "cache_read": null, "cache_write": null }
  ],
  "features": ["tools", "structured_output", "streaming", "caching", "batch"],
  "info": { "status": "active", "release_date": "2026-08-03", "knowledge_cutoff": null, "description": "…", "docs": "https://…", "verified_at": "2026-10-11" },
  "providers": [ … ],
  "aliases": ["qwen3.8-max-0902"],
  "sources": [ { "title": "…", "url": "https://…", "retrieved_at": "2026-10-11" } ]
}
```

## 编辑器支持

仓库自带 [`.vscode/settings.json`](.vscode/settings.json)，用 VS Code 打开本仓库即可在 `models/`、`vendors/`、`providers/` 下的文件中获得字段补全、取值下拉与悬停说明，写错时会出现波浪线。

其他编辑器或想手动配置时，把下面的映射加到用户设置（JetBrains 系列在 *Settings → Languages & Frameworks → Schemas and DTDs → JSON Schema Mappings* 中添加同样的映射）：

```json
{
  "json.schemas": [
    { "fileMatch": ["/models/*.json"], "url": "./schema/model.schema.json" },
    { "fileMatch": ["/vendors/*.json"], "url": "./schema/vendor.schema.json" },
    { "fileMatch": ["/providers/*.json"], "url": "./schema/provider.schema.json" }
  ]
}
```

JSON Schema 表达不了字段顺序、跨字段与跨文件的规则（如 `created` 与发布日期一致、引用的厂商存在、别名不冲突），这些以 `go run ./cmd/check` 为准。

## 在代码中使用

```go
import "github.com/Owlbay/modelinfo-data/model"

ds, issues := model.LoadDir(".")   // 读取并校验整个数据目录
for _, m := range ds.Models {
	fmt.Println(m.Model.ID, m.Model.Prices)
}
```

## 许可

数据以 [CC BY 4.0](LICENSE) 许可发布：可以自由使用、修改和商用，须注明来源「ModelInfo（modelinfo.dev）」。
