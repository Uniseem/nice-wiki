// Package migrations 内嵌本域只追加迁移，运行时不读取仓库路径。
package migrations

import "embed"

// Files 是唯一迁移来源。
//
//go:embed *.sql
var Files embed.FS
