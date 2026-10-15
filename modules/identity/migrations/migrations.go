// Package migrations 内嵌本域只追加SQL迁移（ADR0005）。
//
// 迁移文件按 NNNN_name.sql 编号放在本目录，发布后不得修改或删除；
// 已应用记录写入数据库 schema_migrations 表，由装配层按编号顺序
// 在事务中执行未应用的部分。
package migrations

import (
	"embed"
	"fmt"
	"regexp"
	"sort"
	"strconv"
)

//go:embed *.sql control/*.sql
var files embed.FS

// namePattern 迁移文件名格式：四位编号_小写字母数字下划线.sql。
var namePattern = regexp.MustCompile(`^([0-9]{4})_[a-z0-9_]+\.sql$`)

// Migration 一份编号迁移。
type Migration struct {
	Version int64
	Name    string
	SQL     string
}

// All 按编号升序返回全部内嵌迁移，并校验编号从1起连续且不重复；
// 目录中出现无法解析的文件名视为打包错误，直接返回错误。
func All() ([]Migration, error) {
	entries, err := files.ReadDir(".")
	if err != nil {
		return nil, fmt.Errorf("读取迁移目录失败: %w", err)
	}
	all := make([]Migration, 0, len(entries))
	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}
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

// ControlSchema 返回独立control库的初始schema；未来变更须单独升级版本。
func ControlSchema() (string, error) {
	raw, err := files.ReadFile("control/0001_bootstrap.sql")
	if err != nil {
		return "", fmt.Errorf("读取控制库迁移失败")
	}
	return string(raw), nil
}
