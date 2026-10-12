package httpx

import "net/http"

func NotFound(w http.ResponseWriter) { http.Error(w, "页面不存在", http.StatusNotFound) }
