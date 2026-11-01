# 贡献指南

感谢你帮忙维护 ModelInfo 的数据！本仓库是 [modelinfo.dev](https://modelinfo.dev) 与 `https://modelinfo.dev/v1/models` 的**唯一数据来源**：PR 合并到 `main` 后会自动导入线上。

> **English summary** — This repo is the single source of truth for modelinfo.dev. One JSON file per model in `models/`, vendors in `vendors/`, API providers in `providers/`. Every value must be found on an **official vendor/provider page** listed in `sources`; third-party aggregators (OpenRouter, Artificial Analysis, Wikipedia, …) don't count. If the official page doesn't say, write `null` (or `[]`) — never guess, never fill in 0. Prices are listed in every currency the vendor officially publishes, primary currency first, never converted. Before opening a PR run `go run ./cmd/check --fix` (Go 1.23+) and keep one kind of change per PR. Field reference: [docs/fields.md](docs/fields.md). Issues and PRs in English are welcome.

## 目录

- [基本原则](#基本原则)
- [常见改动](#常见改动)：新增模型 · 更新价格 · 标记弃用或下线 · 新增厂商或供应商
- [来源规则](#来源规则)
- [数据约定](#数据约定)
- [提交前检查](#提交前检查)
- [PR 审核标准](#pr-审核标准)

## 基本原则

1. **每个数值都能在 `sources` 列出的官方页面找到。** 来源必须是模型厂商或供应商的官方页面，第三方聚合站不算。
2. **未知写 `null` 或 `[]`，不推算、不填 0。** 不从模型名、相近模型、SDK 用法推断数值；官方只给计算规则时写 `null`。
3. **价格只列官方币种，不换算汇率。** 主币种在前；分时段、阶梯价只记标准价（取最低一档）。
4. **一个 PR 一类改动。** 例如「新增 3 个 Mistral 模型」或「按 OpenAI 价格页更新 5 个模型价格」，不要把新增、调价、改格式混在一起。

## 常见改动

字段的完整含义与取值见 [docs/fields.md](docs/fields.md)。在 VS Code 中打开本仓库会自动获得字段补全与提示（见 [README](README.md#编辑器支持)）。

### 新增模型

1. 确认厂商已在 `vendors/`、供应商已在 `providers/` 中；没有就先新建（见[下文](#新增厂商或供应商)）。
2. 复制同一厂商的一个相近模型文件为 `models/<id>.json`，**文件名与 `id` 一致**。`id` 用厂商的稳定模型 ID；带日期的快照 ID、`-latest` 等放进 `aliases`，不另建文件。
3. 逐个字段对照官方页面填写；查不到的写 `null` 或 `[]`，不要保留复制来的旧值。
4. `sources` 列出你用到的每个官方页面，`retrieved_at` 与 `info.verified_at` 写今天的日期。
5. 运行 `go run ./cmd/check --fix`。

收录范围：在模型厂商自家官方 API 上可调用（或曾经可调用、已下线）的模型，所有类型都收（对话、推理、视觉、代码、向量、重排、OCR、审核、图像、视频、音频）。只有开源权重而没有官方 API、或只开放给审批机构的模型不收录。

### 更新价格

1. 打开厂商官方价格页，按页面修改 `prices`：每个币种一项，主币种在前；单价单位与 `unit` 一致（Token 计价用每百万 Token：每千 Token 的价格 × 1000）。
2. 官方新增了币种就加一项，取消了就删掉；**不要**用汇率换算出其他币种。
3. 促销价、夜间折扣、套餐价不用；按长度或分辨率分档时取最低一档的标准价。
4. 把价格页加入或更新到 `sources`（`retrieved_at` 为今天），更新 `info.verified_at`。
5. 若某个供应商（如云平台）的价格与模型不同，写在该 `providers[]` 项的 `prices` 中。

### 标记弃用或下线

- **已宣布下线但仍可调用**：`info.status` 改为 `deprecated`，把写明下线计划的官方公告或弃用页加入 `sources`。
- **已下线**：`info.status` 改为 `retired`，把写明下线的官方页面加入 `sources`。**不要删除文件**，其余字段保留下线前的官方值。
- 官方页面已撤下时，可用该网址在 [web.archive.org](https://web.archive.org) 的存档快照作来源，`title` 注明「存档」与原网址。

### 新增厂商或供应商

- 厂商：`vendors/<id>.json`，`{id, name, url, icon?}`，`url` 为官网。
- 供应商：`providers/<id>.json`，`{id, name, url, owned_by}`，`owned_by` 为所属厂商 ID（没有写 `null`）。
- ID 只用小写字母、数字与 `-`，与文件名一致。

## 来源规则

**算作官方来源：** 模型厂商或供应商的官方文档、模型页、价格页、更新日志、官方公告与博客、官方 GitHub / Hugging Face 组织下的模型卡、上述页面在 web.archive.org 的存档。

**不算：** OpenRouter、Artificial Analysis、LLM Stats、LMArena 等聚合站或榜单，维基百科，新闻报道，个人博客与社交媒体转述，第三方 API 网关的价格页。它们可以作为线索，但数值必须回到官方页面确认。`go run ./cmd/check` 会拒绝常见的聚合站域名。

唯一例外：`api.sdk` 可参考 models.dev 的 SDK 包映射，来源标题写明 `api.sdk only`；这个例外不适用于价格、上下文等任何其他字段。

**没有官方来源的值写 `null`。** 宁缺毋滥：一个 `null` 比一个来路不明的数字有用。

## 数据约定

- **发布日期**：只用官方明确写出的日期（`info.release_date` 与 `created` 必须一致）；只有月份时都写 `null`。
- **上下文**：文档写了精确数字就照录；只写 "128K"、"1M" 这类缩写时，按该厂商的口径换算（例如 OpenAI、Anthropic 1K = 1000；Google、DeepSeek、Mistral 1K = 1024），依据页面写进 `sources`。拿不准就写 `null` 并在 PR 中说明。
- **推理**：`reasoning.supported` 只有官方明确说明时才写 `true` / `false`；`reasoning.efforts` 只写 API 可传的确切取值。
- **功能与协议**：`features` 与 `api.types` 只列文档明确支持的；不能从同系列其他模型外推。
- **接口地址**：`providers[].endpoints` 只写文档给出的路径；HTTP 用 `https://`，WebSocket 用 `wss://` 与 `WS`，不同协议拆成两个 provider。
- **官方信息自相矛盾**（如输入上限大于上下文）：冲突值写 `null`，在 PR 中说明，不自行修正。
- **简介与译文**：`info.description` 照录官方原文；译文写在 `translations.zh` / `translations.en`，只翻译不增删。

## 提交前检查

需要 [Go 1.23+](https://go.dev/dl/)，不需要安装其他依赖：

```sh
go run ./cmd/check --fix   # 改写为标准格式（字段顺序、缩进、数值写法）并校验全部数据
go run ./cmd/check         # 只校验，不改文件（CI 跑的就是这个）
go test ./...              # 修改了 Go 代码或 schema 时
```

`check` 会检查：JSON 语法、缺少或多余的字段、字段顺序、取值（枚举、日期格式、正整数、价格非负……）、`created` 与发布日期一致、文件名等于 `id`、引用的厂商和供应商存在、模型 ID 与别名不冲突、来源不是聚合站、文件为标准格式。错误会指出文件、行号与字段，例如：

```
models/grok-3.json:5: owned_by：厂商 x-ai 不存在
models/qwen3.8-max.json:1: 缺少字段 name
```

PR 上的 CI 会把同样的错误标注到对应文件的对应行。

没有 Go 环境也可以提 PR：CI 会告诉你哪里不对；格式问题维护者可以帮忙修。

## PR 审核标准

维护者会逐项确认：

- [ ] 每个新增或修改的数值都能在 PR 中给出的官方来源页面找到（审核者会打开链接核对）。
- [ ] 没有推算值：查不到的是 `null` / `[]`，而不是估计值或 0。
- [ ] 价格币种与官方一致，没有汇率换算；取的是标准价。
- [ ] `sources[].retrieved_at`、`info.verified_at` 已更新。
- [ ] CI 通过（`go run ./cmd/check`）。
- [ ] 一个 PR 只包含一类改动，标题说明改了什么，例如 `数据：收录 Mistral Medium 4` 或 `data: update GPT-5 prices`。

合并后几分钟内线上自动更新。发现线上数据有误但不方便改？请[提 Issue](https://github.com/Owlbay/modelinfo-data/issues/new/choose)。

## 许可

提交到本仓库的贡献以 [CC BY 4.0](LICENSE) 发布。

## 维护者：Go 包与工具

`model`、`jsonx` 两个包（`github.com/Owlbay/modelinfo-data/model`）是数据格式的权威定义，modelinfo 应用直接引用。修改格式时：

1. 改 `model/model.go`（字段顺序即标准顺序）与 `model/validate.go`；
2. 同步 `schema/*.schema.json` 与 `docs/fields.md`；`go test ./schema` 会检查 schema 的字段、必填项与枚举是否与 Go 一致，并用 schema 校验全部数据；
3. 公开接口 `/v1/models` 的字段只增不改。
