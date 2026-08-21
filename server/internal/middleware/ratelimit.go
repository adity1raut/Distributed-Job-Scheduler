package middleware

import "net/http"

// RateLimit enforces a per-project request budget backed by Redis.
func RateLimit(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// TODO: check/increment counter in Redis, return 429 when exceeded
		next.ServeHTTP(w, r)
	})
}
