package auth

import (
	"context"
	"net/http"
	"strings"
	"time"

	"github.com/golang-jwt/jwt/v5"
)

type Claims struct {
	UserID   string `json:"userId"`
	Username string `json:"username"`
	Role     string `json:"role"`
	jwt.RegisteredClaims
}

type ctxKey struct{}

func GenerateToken(secret, userID, username, role string) (string, error) {
	claims := Claims{
		UserID:   userID,
		Username: username,
		Role:     role,
		RegisteredClaims: jwt.RegisteredClaims{
			ExpiresAt: jwt.NewNumericDate(time.Now().Add(24 * time.Hour)),
			IssuedAt:  jwt.NewNumericDate(time.Now()),
		},
	}
	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	return token.SignedString([]byte(secret))
}

func JWTMiddleware(secret string) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			tokenStr := extractToken(r)
			if tokenStr == "" {
				http.Error(w, `{"error":"unauthorized"}`, http.StatusUnauthorized)
				return
			}
			claims := &Claims{}
			token, err := jwt.ParseWithClaims(tokenStr, claims, func(t *jwt.Token) (any, error) {
				return []byte(secret), nil
			})
			if err != nil || !token.Valid {
				http.Error(w, `{"error":"unauthorized"}`, http.StatusUnauthorized)
				return
			}
			ctx := context.WithValue(r.Context(), ctxKey{}, claims)
			next.ServeHTTP(w, r.WithContext(ctx))
		})
	}
}

func APIKeyOrJWTMiddleware(apiKey, jwtSecret string) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			// Try API key first
			key := r.Header.Get("X-API-Key")
			if key == apiKey {
				next.ServeHTTP(w, r)
				return
			}

			// Try JWT from Authorization header
			tokenStr := extractToken(r)
			if tokenStr == apiKey {
				// Also accept API key via Bearer
				next.ServeHTTP(w, r)
				return
			}
			if tokenStr != "" {
				claims := &Claims{}
				token, err := jwt.ParseWithClaims(tokenStr, claims, func(t *jwt.Token) (any, error) {
					return []byte(jwtSecret), nil
				})
				if err == nil && token.Valid {
					ctx := context.WithValue(r.Context(), ctxKey{}, claims)
					next.ServeHTTP(w, r.WithContext(ctx))
					return
				}
			}

			http.Error(w, `{"error":"unauthorized"}`, http.StatusUnauthorized)
		})
	}
}

func ClaimsFromContext(ctx context.Context) *Claims {
	c, _ := ctx.Value(ctxKey{}).(*Claims)
	return c
}

func extractToken(r *http.Request) string {
	auth := r.Header.Get("Authorization")
	if strings.HasPrefix(auth, "Bearer ") {
		return strings.TrimPrefix(auth, "Bearer ")
	}
	return ""
}
