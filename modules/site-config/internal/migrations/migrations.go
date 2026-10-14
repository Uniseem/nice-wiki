// Package migrations 提供 site-config 控制库的只追加 SQL 迁移。
//
// 迁移文件发布后不得修改或删除；已应用版本记录在 control.db 的
// schema_migrations 表中，由存储装配层按编号顺序执行。
package migrations

import (
	"embed"
	"fmt"
	"regexp"
	"sort"
	"strconv"
)

//go:embed *.sql
var files embed.FS

var namePattern = regexp.MustCompile(`^([0-9]{4})_[a-z0-9_]+\.sql$`)

// Migration 是一份内嵌的控制库迁移。
type Migration struct {
	Version int64
	Name    string
	SQL     string
}

// All 按编号升序返回迁移，并拒绝缺号、重复号和不符合命名约定的文件。
func All() ([]Migration, error) {
	entries, err := files.ReadDir(".")
	if err != nil {
		return nil, fmt.Errorf("读取迁移目录失败: %w", err)
	}
	all := make([]Migration, 0, len(entries))
	for _, entry := range entries {
		match := namePattern.FindStringSubmatch(entry.Name())
		if match == nil {
			return nil, fmt.Errorf("迁移文件名不合法: %s", entry.Name())
		}
		version, err := strconv.ParseInt(match[1], 10, 64)
		if err != nil {
			return nil, fmt.Errorf("迁移编号不合法: %s", entry.Name())
		}
		raw, err := files.ReadFile(entry.Name())
		if err != nil {
			return nil, fmt.Errorf("读取迁移文件失败: %s", entry.Name())
		}
		all = append(all, Migration{Version: version, Name: entry.Name(), SQL: string(raw)})
	}
	sort.Slice(all, func(i, j int) bool { return all[i].Version < all[j].Version })
	for i, item := range all {
		if item.Version != int64(i+1) {
			return nil, fmt.Errorf("迁移编号必须从1起连续: %s", item.Name)
		}
	}
	return all, nil
}
