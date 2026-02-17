package auth

import (
	"net/http"
	"strings"
)

// APIKeyMiddleware validates the X-API-Key header against the expected key.
func APIKeyMiddleware(apiKey string) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			key := r.Header.Get("X-API-Key")
			if key == "" {
				key = strings.TrimPrefix(r.Header.Get("Authorization"), "Bearer ")
			}
			if key != apiKey {
				http.Error(w, `{"error":"unauthorized"}`, http.StatusUnauthorized)
				return
			}
			next.ServeHTTP(w, r)
		})
	}
}
