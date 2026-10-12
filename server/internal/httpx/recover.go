package httpx

import (
	"log/slog"
	"net/http"
)

func Recover(logger *slog.Logger, next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		defer func() {
			if recovered := recover(); recovered != nil {
				logger.Error("HTTP 处理异常")
				http.Error(w, "服务暂时不可用", http.StatusInternalServerError)
			}
		}()
		next.ServeHTTP(w, r)
	})
}
