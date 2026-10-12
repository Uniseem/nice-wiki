package api

import (
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
