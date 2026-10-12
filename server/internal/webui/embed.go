package webui

import (
	"embed"
	"io/fs"
)

//go:embed all:dist
var bundle embed.FS

func Files() (fs.FS, error) { return fs.Sub(bundle, "dist") }
