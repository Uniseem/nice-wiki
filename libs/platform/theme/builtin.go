package theme

import (
	"embed"
	"io/fs"
	"path"
	"strings"
)

//go:embed builtin
var builtinFiles embed.FS

// builtinOrder 是内置外观的展示顺序，第一个是默认外观。
var builtinOrder = []string{"classic", "paper", "forest", "ocean", "midnight"}

var builtins = loadBuiltins()

func loadBuiltins() []*Package {
	out := make([]*Package, 0, len(builtinOrder))
	for _, id := range builtinOrder {
		files := map[string][]byte{}
		root := path.Join("builtin", id)
		err := fs.WalkDir(builtinFiles, root, func(name string, entry fs.DirEntry, err error) error {
			if err != nil || entry.IsDir() {
				return err
			}
			data, err := builtinFiles.ReadFile(name)
			files[strings.TrimPrefix(name, root+"/")] = data
			return err
		})
		if err != nil {
			panic("内置外观 " + id + " 读取失败：" + err.Error())
		}
		pkg, err := FromFiles(files)
		if err != nil {
			panic("内置外观 " + id + " 不合法：" + err.Error())
		}
		pkg.Builtin = true
		out = append(out, pkg)
	}
	return out
}

// Builtins 返回内置外观（第一个是默认外观 classic）。
func Builtins() []*Package { return builtins }

// Default 是站点没选外观时使用的外观。
func Default() *Package { return builtins[0] }

// Builtin 按摘要找内置外观。
func Builtin(digest string) *Package {
	for _, pkg := range builtins {
		if pkg.Digest == digest {
			return pkg
		}
	}
	return nil
}

// Reserved 判断 id 是否被内置外观占用（市场里不能用这些 id 发布）。
func Reserved(id string) bool {
	for _, pkg := range builtins {
		if pkg.Manifest.ID == id {
			return true
		}
	}
	return false
}
