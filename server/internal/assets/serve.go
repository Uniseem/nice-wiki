package assets

import (
	"mime"
	"net/http"
	"os"
	"path/filepath"
	"strings"
)

func (s *Store) Handler() http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != "GET" && r.Method != "HEAD" {
			w.Header().Set("Allow", "GET, HEAD")
			http.Error(w, "请求方法不允许", http.StatusMethodNotAllowed)
			return
		}
		parts := strings.Split(strings.TrimPrefix(r.URL.Path, "/assets/"), "/")
		if len(parts) != 2 {
			http.NotFound(w, r)
			return
		}
		hash := strings.TrimSuffix(parts[1], filepath.Ext(parts[1]))
		value, err := s.Get(r.Context(), hash)
		if err != nil || parts[0] != value.Kind || parts[1] != hash+value.Extension {
			http.NotFound(w, r)
			return
		}
		path, err := s.path(value)
		if err != nil {
			http.NotFound(w, r)
			return
		}
		file, err := os.Open(path)
		if err != nil {
			http.NotFound(w, r)
			return
		}
		defer func() {
			if err := file.Close(); err != nil {
				s.logger.Error("资源文件关闭失败")
			}
		}()
		info, err := file.Stat()
		if err != nil || !info.Mode().IsRegular() {
			http.NotFound(w, r)
			return
		}
		w.Header().Set("Content-Type", value.MIME)
		w.Header().Set("X-Content-Type-Options", "nosniff")
		w.Header().Set("Cache-Control", "public, max-age=31536000, immutable")
		w.Header().Set("ETag", `"`+value.Hash+`"`)
		if value.Kind == "files" || value.Extension == ".excalidraw" {
			w.Header().Set("Content-Disposition", mime.FormatMediaType("attachment", map[string]string{"filename": value.Name}))
		}
		if value.Extension == ".svg" {
			w.Header().Set("Content-Security-Policy", "sandbox; default-src 'none'; style-src 'unsafe-inline'")
		}
		http.ServeContent(w, r, value.Name, info.ModTime(), file)
	})
}
