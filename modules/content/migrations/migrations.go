// Package migrations 内嵌content只追加SQL迁移；已交付文件不能修改或删除。
package migrations

import (
	"embed"
	"fmt"
	"io/fs"
	"regexp"
	"sort"
	"strconv"

	"nice-wiki/libs/platform"
)

//go:embed *.sql
var files embed.FS

var filename = regexp.MustCompile(`^([0-9]{4})_[a-z0-9_]+\.sql$`)

type Migration struct {
	Version int64
	Name    string
	SQL     string
	SHA256  platform.Hash
}

// All 只返回内嵌SQL，编号必须从1起连续；hash阻止修改已应用迁移。
func All() ([]Migration, error) {
	names, err := fs.Glob(files, "*.sql")
	if err != nil {
		return nil, err
	}
	sort.Strings(names)
	all := make([]Migration, 0, len(names))
	for i, name := range names {
		match := filename.FindStringSubmatch(name)
		if match == nil {
			return nil, fmt.Errorf("迁移文件名不合法")
		}
		version, err := strconv.ParseInt(match[1], 10, 64)
		if err != nil || version != int64(i+1) {
			return nil, fmt.Errorf("迁移编号必须从1起连续")
		}
		raw, err := files.ReadFile(name)
		if err != nil {
			return nil, fmt.Errorf("迁移不可读")
		}
		all = append(all, Migration{Version: version, Name: name, SQL: string(raw), SHA256: platform.HashBytes(raw)})
	}
	return all, nil
}
