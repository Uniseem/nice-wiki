package site

import (
	"bytes"
	"html/template"
	"net/http"
	"nice-wiki/internal/doc"
)

// Preview 仅通过已完成登录与权限检查的 API 路由调用。
func (s *Site) Preview(w http.ResponseWriter, r *http.Request, id string) {
	draft, err := s.Content.Draft(id)
	if err != nil {
		http.Error(w, "草稿不存在", http.StatusNotFound)
		return
	}
	file, err := s.Content.Get(id)
	if err != nil {
		http.Error(w, "文档不存在", http.StatusNotFound)
		return
	}
	lang := s.Settings.Languages().Base
	body, err := doc.RenderHTML(draft.Document, doc.RenderOptions{Language: lang, Links: s})
	if err != nil {
		http.Error(w, "草稿格式无效", http.StatusBadRequest)
		return
	}
	data := s.page(lang, file.Meta.Title+" · 草稿预览")
	data.Body = template.HTML(body)
	data.TOC = tableOfContents(draft.Document)
	markdown, err := doc.RenderMarkdown(draft.Document, doc.RenderOptions{Language: lang, Links: s})
	if err != nil {
		http.Error(w, "草稿格式无效", http.StatusBadRequest)
		return
	}
	data.Markdown = "# " + file.Meta.Title + "\n\n" + markdown
	var output bytes.Buffer
	if err := s.templates.ExecuteTemplate(&output, "doc", data); err != nil {
		http.Error(w, "草稿渲染失败", http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.Header().Set("Cache-Control", "private, no-store")
	w.Header().Set("X-Robots-Tag", "noindex, nofollow")
	// 页面可嵌入同源后台预览，不能被其他站点嵌入。
	w.Header().Set("X-Frame-Options", "SAMEORIGIN")
	w.Header().Set("Content-Security-Policy", "default-src 'self'; script-src 'self'; style-src 'self' 'unsafe-inline'; img-src 'self' data:; frame-ancestors 'self'; base-uri 'self'")
	if s.Dev {
		w.Header().Set("Content-Security-Policy", "default-src 'self'; script-src 'self' http://localhost:5173; style-src 'self' 'unsafe-inline' http://localhost:5173; img-src 'self' data:; connect-src 'self' http://localhost:5173 ws://localhost:5173; frame-ancestors 'self'; base-uri 'self'")
	}
	if r.Method != http.MethodHead {
		_, _ = w.Write(output.Bytes())
	}
}
