package webui

import (
	"io/fs"
	"net/http"
	"strings"
)

func Admin(files fs.FS, dev bool) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Cache-Control", "no-store")
		w.Header().Set("X-Robots-Tag", "noindex")
		if dev {
			http.Redirect(w, r, "http://localhost:5173/admin/"+strings.TrimPrefix(r.URL.Path, "/admin/"), http.StatusTemporaryRedirect)
			return
		}
		data, err := fs.ReadFile(files, "admin/index.html")
		if err != nil {
			http.Error(w, "请先构建前端", http.StatusServiceUnavailable)
			return
		}
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		_, _ = w.Write(data)
	})
}
