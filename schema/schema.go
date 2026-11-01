// Package schema 内嵌编辑器用的 JSON Schema（model、vendor、provider），与 model 包的 Go 校验保持一致：
// 测试保证仓库中的全部数据同时通过两者，且 schema 的字段、必填项与枚举和 Go 定义相同。
// JSON Schema 表达不了的规则（字段顺序、跨字段与跨文件规则）只由 go run ./cmd/check 检查。
package schema

import (
	"embed"

	"github.com/Owlbay/modelinfo-data/model"
)

//go:embed *.schema.json
var FS embed.FS

// For 返回某个种类的 JSON Schema 原文（model.schema.json、vendor.schema.json、provider.schema.json）。
func For(k model.Kind) []byte {
	b, err := FS.ReadFile(string(k) + ".schema.json")
	if err != nil {
		panic(err) // 内嵌文件，构建时已确定存在
	}
	return b
}
