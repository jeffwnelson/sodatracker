package auth

import (
	"crypto/subtle"
	"net/http"
)

// APIKeyMiddleware accepts the API key in X-API-Key or as a Bearer token.
func APIKeyMiddleware(apiKey string) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			key := r.Header.Get("X-API-Key")
			if key == "" {
				key = extractToken(r)
			}
			if !keyMatches(key, apiKey) {
				http.Error(w, `{"error":"unauthorized"}`, http.StatusUnauthorized)
				return
			}
			next.ServeHTTP(w, r)
		})
	}
}

// keyMatches compares in constant time, so response timing doesn't reveal
// how much of a guessed key is right.
func keyMatches(got, want string) bool {
	return got != "" && subtle.ConstantTimeCompare([]byte(got), []byte(want)) == 1
}
