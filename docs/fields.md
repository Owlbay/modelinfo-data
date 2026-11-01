# 字段参考 / Field reference

每个文件是一个 JSON 对象，**字段顺序固定**（按下表顺序；`go run ./cmd/check --fix` 会自动排好）。除标注「可省略」的字段外都必须出现；官方没有给出的值写 `null` 或 `[]`，不推算、不填 0。

格式的权威定义是 Go 包 [`model`](../model/model.go)（结构体字段顺序即标准顺序）与 [`model/validate.go`](../model/validate.go)（取值规则）；编辑器提示用的 [JSON Schema](../schema/) 由测试保证与之一致。

- [模型 `models/<id>.json`](#模型-modelsidjson)
- [模型厂商 `vendors/<id>.json`](#模型厂商-vendorsidjson)
- [供应商 `providers/<id>.json`](#供应商-providersidjson)
- [跨文件规则](#跨文件规则)
- [标准格式](#标准格式)

## 模型 `models/<id>.json`

前 12 个字段原样进入公开接口 [`/v1/models`](https://modelinfo.dev/v1/models)（前四个与 OpenAI Models API 一致）；`providers`、`aliases`、`translations`、`sources` 是维护字段，不进入 `/v1/models`。

| 字段 | 类型 | 含义与规则 |
|---|---|---|
| `id` | string | 调用接口时传的模型 ID，用模型厂商的稳定 ID。`[a-z0-9][a-z0-9._-]*`，**必须与文件名一致** |
| `object` | `"model"` | 固定值 |
| `created` | integer \| null | 发布时间，Unix 秒（发布日 UTC 0 点）。必须与 `info.release_date` 一致；未知为 `null` |
| `owned_by` | string | 模型厂商 ID，必须存在于 `vendors/`。转售平台、云托管的第三方模型不算该平台的模型 |
| `name` | string | 显示名称，按官方写法，不能为空 |
| `api.types` | string[] | 支持的接口类型，至少一项，取值见下表 |
| `api.sdk` | string[] | 可调用该模型的 SDK 包名（npm 或 PyPI，可含子路径），不重复；没有时 `[]` |
| `limits.context` | integer \| null | 上下文窗口（Token），正整数 |
| `limits.input` | integer \| null | 最大输入 Token，不能大于 `context` |
| `limits.output` | integer \| null | 最大输出 Token，不能大于 `context` |
| `modalities.input` | string[] | 输入模态，至少一项，取值见下表 |
| `modalities.output` | string[] | 输出模态，至少一项 |
| `reasoning.supported` | boolean \| null | 官方明确说明支持 / 不支持时才写 `true` / `false`，否则 `null` |
| `reasoning.efforts` | string[] | API 可传的推理强度取值（按官方写法）；只有开关或 Token 预算时 `[]` |
| `prices` | Price[] | 官方标价，见 [Price](#price)；没有公开价时 `[]` |
| `features` | string[] | 文档明确支持的功能，取值见下表；没有时 `[]` |
| `info.status` | string | `preview` 预览 · `active` 可用 · `deprecated` 已宣布下线但仍可调用 · `retired` 已下线 |
| `info.release_date` | string \| null | 发布日期 `YYYY-MM-DD`，只用官方明确写出的日期；只有月份时写 `null` |
| `info.knowledge_cutoff` | string \| null | 训练数据截止，按官方写法（如 `2026-06`） |
| `info.description` | string \| null | 一句话简介，照录官方原文（多为英文），不翻译 |
| `info.docs` | string \| null | 官方文档页网址 |
| `info.verified_at` | string \| null | 最近一次对照官方页面核实的日期 `YYYY-MM-DD` |
| `providers` | Provider[] | 能调用该模型的官方接入点，见 [Provider](#provider)；没有时 `[]` |
| `aliases` | string[] | 旧 ID、带日期的快照 ID、`-latest` 等其他叫法；没有时 `[]` |
| `translations` | object | **可省略**。简介的译文：`{"zh": {"description": "…"}}`，语言只能是 `zh`、`en`；原文是哪种语言就不写该语言；`info.description` 为 `null` 时不能有 |
| `sources` | Source[] | 官方来源，至少一条，见 [Source](#source) |

### 枚举取值

| 字段 | 取值 |
|---|---|
| `api.types`、`providers[].endpoints` 的键 | `chat`（OpenAI Chat Completions 兼容）· `messages`（Anthropic Messages 兼容）· `responses`（OpenAI Responses 兼容）· `interactions`（Google Interactions）· `completions` · `generate` · `batch` · `realtime` · `embeddings` · `images` · `video` · `audio` · `3d` · `rerank` · `ocr` · `moderation` |
| `modalities.*` | `text` · `image` · `audio` · `video` · `document` · `embedding` · `score` · `3d` |
| `features` | `tools` · `structured_output` · `streaming` · `caching` · `batch` |
| `prices[].unit` | `1M_tokens` · `1M_characters` · `image` · `second` · `minute` · `hour` · `request` · `page` |

### Price

`prices` 列出模型厂商官方公布的**全部币种**，每个币种一项，**主币种（厂商主推的币种）在前**，同一币种只出现一次。**不换算汇率**。

| 字段 | 类型 | 含义 |
|---|---|---|
| `currency` | string | ISO 4217 币种代码，三位大写，如 `USD`、`CNY` |
| `unit` | string | 计价单位（见上表） |
| `input` | number \| null | 输入单价；未公布为 `null`，官方写明免费为 `0` |
| `output` | number \| null | 输出单价 |
| `cache_read` | number \| null | 缓存命中单价 |
| `cache_write` | number \| null | 缓存写入单价 |

- 只记标准价：按输入长度、分辨率、时长分档时取**最低一档**；促销价、夜间折扣、套餐价不用。
- 单位只做精确换算（每千 Token × 1000 得每百万），不跨单位推算（不把按秒折算成按次）。
- 多种单位混合计费、无法用一个单位表达时，写 `[]` 并在 PR 中说明。

### Provider

| 字段 | 类型 | 含义 |
|---|---|---|
| `id` | string | 供应商 ID，必须存在于 `providers/` |
| `model` | string | 在该供应商处调用时传的模型 ID |
| `base_url` | string | `https://…`（HTTP）或 `wss://…`（WebSocket）；路径中的占位符（如 `{WorkspaceId}`）照官方原样 |
| `endpoints` | object | 接口类型 → `"GET\|POST\|WS /路径"`。键必须在 `api.types` 中；HTTP 端点配 `https://`，WS 端点配 `wss://`，协议不同就拆成两个 provider；Chat Completions 路径不能标成 `messages`；文档没给路径时写 `{}` |
| `reasoning_param` | string \| null | 开启或调节推理的请求参数名（如 `reasoning.effort`、`enable_thinking`）；没有为 `null` |
| `reasoning_params` | object | **可省略**。按接口类型区分的推理参数（键必须是已登记的端点），值为参数名或 `null` |
| `reasoning_efforts` | string[] | **可省略**。与模型不同时才写 |
| `limits` | Limits | **可省略**。与模型不同时才写 |
| `modalities` | Modalities | **可省略**。与模型不同时才写 |
| `features` | string[] | **可省略**。与模型不同时才写 |
| `prices` | Price[] | **可省略**。与模型不同时才写；`[]` 表示该供应商没有公开价 |

### Source

| 字段 | 类型 | 含义 |
|---|---|---|
| `title` | string | 页面标题 |
| `url` | string | `https://` 网址，必须是模型厂商或供应商的**官方页面**（文档、模型页、价格页、官方公告、官方博客、官方 GitHub / Hugging Face 组织）。官方页面已撤下时可用该网址在 web.archive.org 的存档，`title` 注明「存档」 |
| `retrieved_at` | string | 查阅日期 `YYYY-MM-DD` |

第三方聚合站（OpenRouter、Artificial Analysis、LLM Stats、LMArena、维基百科等）不能作来源，`go run ./cmd/check` 会拒绝其中常见的域名。唯一例外：`api.sdk` 可参考 models.dev 的 SDK 映射，来源标题写明 `api.sdk only`。

## 模型厂商 `vendors/<id>.json`

| 字段 | 类型 | 含义 |
|---|---|---|
| `id` | string | 厂商 ID，`[a-z0-9-]+`，与文件名一致；模型的 `owned_by` 引用它 |
| `name` | string | 显示名称 |
| `url` | string | 官网 `https://` 网址 |
| `icon` | string | **可省略**。[@lobehub/icons](https://github.com/lobehub/lobe-icons) 的单色图标名 |

## 供应商 `providers/<id>.json`

| 字段 | 类型 | 含义 |
|---|---|---|
| `id` | string | 供应商 ID，`[a-z0-9-]+`，与文件名一致；模型的 `providers[].id` 引用它 |
| `name` | string | 显示名称 |
| `url` | string \| null | 官网 `https://` 网址 |
| `owned_by` | string \| null | 所属模型厂商 ID，必须存在于 `vendors/` |

## 跨文件规则

- 文件名等于 `id`：`models/gpt-5.json` 的 `id` 必须是 `gpt-5`。
- 引用必须存在：模型的 `owned_by` → `vendors/`，`providers[].id` → `providers/`，供应商的 `owned_by` → `vendors/`。新厂商、新供应商先建文件再加模型（可在同一个 PR 里）。
- 模型 ID 与别名共用一个命名空间：别名不能等于其他模型的 ID，也不能同时挂在两个模型下（`-latest` 这类会漂移的别名只挂在当前指向的模型上）。

## 标准格式

两空格缩进、LF 换行、文件末尾一个换行；字段按上表顺序；数值与字符串按 JavaScript `JSON.stringify` 的写法（`2` 而不是 `2.0`，`1e-7` 而不是 `0.0000001`，中文与 `<>&` 原样不转义）。手写时不必在意，提交前运行 `go run ./cmd/check --fix` 即可。
