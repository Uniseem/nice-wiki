package httpx

import "net/http"

func Security(dev bool, next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("X-Content-Type-Options", "nosniff")
		w.Header().Set("Referrer-Policy", "strict-origin-when-cross-origin")
		w.Header().Set("X-Frame-Options", "DENY")
		w.Header().Set("Permissions-Policy", "camera=(), microphone=(), geolocation=()")
		policy := "default-src 'self'; script-src 'self'; style-src 'self' 'unsafe-inline'; img-src 'self' data:; font-src 'self'; connect-src 'self'; frame-ancestors 'none'; base-uri 'self'; form-action 'self'"
		if dev {
			policy = "default-src 'self'; script-src 'self' http://localhost:5173 http://127.0.0.1:5173; style-src 'self' 'unsafe-inline' http://localhost:5173 http://127.0.0.1:5173; img-src 'self' data:; connect-src 'self' ws://localhost:5173 ws://127.0.0.1:5173 http://localhost:5173 http://127.0.0.1:5173; frame-ancestors 'none'; base-uri 'self'; form-action 'self'"
		}
		w.Header().Set("Content-Security-Policy", policy)
		if !dev {
			w.Header().Set("Strict-Transport-Security", "max-age=31536000")
		}
		next.ServeHTTP(w, r)
	})
}
