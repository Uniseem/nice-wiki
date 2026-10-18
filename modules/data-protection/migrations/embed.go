// Package migrations 保存本域初始可恢复目录与永不替换的控制库迁移。
package migrations

import "embed"

// Files 只包含已登记迁移，执行器按各库user_version事务应用。
//go:embed 0001_domain.sql 0002_control.sql 0003_vault_catalog.sql 0004_vault_layout.sql 0005_backup_intent.sql
var Files embed.FS
