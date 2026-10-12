package api

import (
	"errors"
	"net/http"
)

func (s *Server) UploadAsset(w http.ResponseWriter, r *http.Request) {
	r.Body = http.MaxBytesReader(w, r.Body, (128<<20)+(1<<20))
	if err := r.ParseMultipartForm(8 << 20); err != nil {
		badRequest(w, errors.New("上传表单无效或超过大小限制"))
		return
	}
	if r.MultipartForm != nil {
		defer func() {
			if err := r.MultipartForm.RemoveAll(); err != nil {
				s.Logger.Error("上传临时文件清理失败")
			}
		}()
	}
	file, header, err := r.FormFile("file")
	if err != nil {
		badRequest(w, errors.New("请选择一个文件"))
		return
	}
	defer func() {
		if err := file.Close(); err != nil {
			s.Logger.Error("上传文件关闭失败")
		}
	}()
	value, err := s.Assets.Upload(r.Context(), file, header.Filename, r.FormValue("kind"))
	if err != nil {
		s.validationOrFailure(w, err)
		return
	}
	writeJSON(w, value)
}
func (s *Server) ListAssets(w http.ResponseWriter, r *http.Request) {
	values, err := s.Assets.List(r.Context())
	if err != nil {
		s.fail(w, err)
		return
	}
	writeJSON(w, values)
}
func (s *Server) DeleteAsset(w http.ResponseWriter, r *http.Request, id string) {
	if err := s.Assets.Delete(r.Context(), id); err != nil {
		badRequest(w, err)
		return
	}
	if !s.audit(w, r, "delete_asset", id) {
		return
	}
	writeJSON(w, struct{}{})
}
