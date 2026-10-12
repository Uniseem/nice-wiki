package site

import "net/http"

func (s *Site) ask(w http.ResponseWriter, r *http.Request, lang string) {
	if !s.Settings.AI().Enabled {
		s.errorPage(w, r, http.StatusNotFound, "页面不存在")
		return
	}
	data := s.page(lang, "询问文档")
	data.Description = "根据已发布文档回答问题"
	s.render(w, r, http.StatusOK, "ask", data)
}
