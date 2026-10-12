package httpx

import (
	"crypto/rand"
	"encoding/hex"
	"log/slog"
	"net/http"
	"time"
)

type responseWriter struct {
	http.ResponseWriter
	status int
}

func (w *responseWriter) WriteHeader(status int) {
	if w.status != 0 {
		return
	}
	w.status = status
	w.ResponseWriter.WriteHeader(status)
}
func (w *responseWriter) Write(data []byte) (int, error) {
	if w.status == 0 {
		w.WriteHeader(200)
	}
	return w.ResponseWriter.Write(data)
}
func (w *responseWriter) Unwrap() http.ResponseWriter { return w.ResponseWriter }
func Logging(logger *slog.Logger, next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		started := time.Now()
		bytes := make([]byte, 12)
		if _, err := rand.Read(bytes); err == nil {
			w.Header().Set("X-Request-ID", hex.EncodeToString(bytes))
		}
		wrapped := &responseWriter{ResponseWriter: w}
		next.ServeHTTP(wrapped, r)
		status := wrapped.status
		if status == 0 {
			status = 200
		}
		// 不记录 URL 查询、正文、Origin、cookie、IP 或认证信息。
		logger.Info("HTTP 请求", "method", r.Method, "status", status, "durationMs", time.Since(started).Milliseconds())
	})
}
