package middleware

import "net/http"

// Auth verifies the JWT on the request and attaches the user ID to context.
func Auth(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// TODO: parse and verify JWT from Authorization header
		next.ServeHTTP(w, r)
	})
}
