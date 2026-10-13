package api

import (
	"fmt"
	"io"
	"mime"
	"net/http"
	"nice-wiki/internal/api/gen"
)

func (s *Server) ListBackups(w http.ResponseWriter, r *http.Request) {
	if s.ListBackupFiles == nil {
		writeJSON(w, []BackupResult{})
		return
	}
	values, err := s.ListBackupFiles(r.Context())
	if err != nil {
		s.fail(w, err)
		return
	}
	writeJSON(w, values)
}
func (s *Server) CreateBackup(w http.ResponseWriter, r *http.Request) {
	body, err := decode[gen.BackupInput](w, r)
	if err != nil {
		badRequest(w, err)
		return
	}
	if s.CreateBackupFile == nil {
		writeError(w, 503, "backup_unavailable", "备份服务尚未配置")
		return
	}
	value, err := s.CreateBackupFile(r.Context(), body.IncludeSecrets)
	if err != nil {
		s.fail(w, err)
		return
	}
	if !s.audit(w, r, "create_backup", value.Name) {
		return
	}
	writeJSON(w, value)
}

func (s *Server) DownloadBackup(w http.ResponseWriter, r *http.Request, name string) {
	if s.OpenBackupFile == nil {
		writeError(w, http.StatusServiceUnavailable, "backup_unavailable", "备份服务尚未配置")
		return
	}
	file, size, err := s.OpenBackupFile(r.Context(), name)
	if err != nil {
		s.fail(w, err)
		return
	}
	defer func() { _ = file.Close() }()
	disposition := mime.FormatMediaType("attachment", map[string]string{"filename": name})
	w.Header().Set("Content-Type", "application/gzip")
	w.Header().Set("Content-Disposition", disposition)
	w.Header().Set("Content-Length", fmt.Sprintf("%d", size))
	w.Header().Set("Cache-Control", "no-store")
	if _, err := io.Copy(w, file); err != nil {
		s.Logger.Warn("备份下载中断")
	}
}
