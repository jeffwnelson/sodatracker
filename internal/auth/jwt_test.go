package auth

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/golang-jwt/jwt/v5"
)

const testSecret = "test-secret-key"

func TestGenerateAndValidateToken(t *testing.T) {
	token, err := GenerateToken(testSecret, "u1", "admin", "admin")
	if err != nil {
		t.Fatalf("generate: %v", err)
	}
	if token == "" {
		t.Fatal("expected non-empty token")
	}

	// Parse it back
	req := httptest.NewRequest("GET", "/", nil)
	req.Header.Set("Authorization", "Bearer "+token)

	called := false
	handler := JWTMiddleware(testSecret)(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		called = true
		claims := ClaimsFromContext(r.Context())
		if claims == nil {
			t.Fatal("expected claims in context")
		}
		if claims.UserID != "u1" {
			t.Errorf("expected userId=u1, got %s", claims.UserID)
		}
		if claims.Username != "admin" {
			t.Errorf("expected username=admin, got %s", claims.Username)
		}
		if claims.Role != "admin" {
			t.Errorf("expected role=admin, got %s", claims.Role)
		}
	}))

	w := httptest.NewRecorder()
	handler.ServeHTTP(w, req)
	if !called {
		t.Error("handler was not called")
	}
}

func TestJWTMiddleware_MissingToken(t *testing.T) {
	req := httptest.NewRequest("GET", "/", nil)
	w := httptest.NewRecorder()

	handler := JWTMiddleware(testSecret)(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		t.Error("handler should not be called")
	}))

	handler.ServeHTTP(w, req)
	if w.Code != http.StatusUnauthorized {
		t.Errorf("expected 401, got %d", w.Code)
	}
}

func TestJWTMiddleware_InvalidToken(t *testing.T) {
	req := httptest.NewRequest("GET", "/", nil)
	req.Header.Set("Authorization", "Bearer invalid-token")
	w := httptest.NewRecorder()

	handler := JWTMiddleware(testSecret)(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		t.Error("handler should not be called")
	}))

	handler.ServeHTTP(w, req)
	if w.Code != http.StatusUnauthorized {
		t.Errorf("expected 401, got %d", w.Code)
	}
}

func TestJWTMiddleware_ValidToken(t *testing.T) {
	token, _ := GenerateToken(testSecret, "u1", "admin", "admin")
	req := httptest.NewRequest("GET", "/", nil)
	req.Header.Set("Authorization", "Bearer "+token)
	w := httptest.NewRecorder()

	called := false
	handler := JWTMiddleware(testSecret)(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		called = true
		claims := ClaimsFromContext(r.Context())
		if claims == nil {
			t.Fatal("expected claims")
		}
	}))

	handler.ServeHTTP(w, req)
	if !called {
		t.Error("handler was not called")
	}
	if w.Code != http.StatusOK {
		t.Errorf("expected 200, got %d", w.Code)
	}
}

// Tokens that never expire, or that name another algorithm, are refused even
// when signed with the right secret.
func TestJWTRejectsNoExpiryAndOtherAlgorithms(t *testing.T) {
	sign := func(m jwt.SigningMethod, c Claims) string {
		tok, err := jwt.NewWithClaims(m, c).SignedString([]byte(testSecret))
		if err != nil {
			t.Fatal(err)
		}
		return tok
	}
	valid, _ := GenerateToken(testSecret, "u1", "admin", "admin")
	noExpiry := sign(jwt.SigningMethodHS256, Claims{UserID: "u1", Username: "admin", Role: "admin"})
	var withExp Claims
	jwt.ParseWithClaims(valid, &withExp, func(*jwt.Token) (any, error) { return []byte(testSecret), nil })
	hs512 := sign(jwt.SigningMethodHS512, withExp)

	for name, tc := range map[string]struct {
		token string
		want  int
	}{
		"valid":     {valid, http.StatusOK},
		"no expiry": {noExpiry, http.StatusUnauthorized},
		"HS512":     {hs512, http.StatusUnauthorized},
	} {
		req := httptest.NewRequest("GET", "/", nil)
		req.Header.Set("Authorization", "Bearer "+tc.token)
		w := httptest.NewRecorder()
		JWTMiddleware(testSecret)(http.HandlerFunc(func(http.ResponseWriter, *http.Request) {})).ServeHTTP(w, req)
		if w.Code != tc.want {
			t.Errorf("%s: got %d, want %d", name, w.Code, tc.want)
		}
	}
}

func TestAPIKeyMiddlewares(t *testing.T) {
	ok := http.HandlerFunc(func(http.ResponseWriter, *http.Request) {})
	for name, tc := range map[string]struct {
		header, value string
		want          int
	}{
		"X-API-Key":       {"X-API-Key", "k", http.StatusOK},
		"Bearer":          {"Authorization", "Bearer k", http.StatusOK},
		"raw auth header": {"Authorization", "k", http.StatusUnauthorized},
		"wrong key":       {"X-API-Key", "nope", http.StatusUnauthorized},
		"none":            {"X-Other", "k", http.StatusUnauthorized},
	} {
		for mw, h := range map[string]http.Handler{
			"APIKey":      APIKeyMiddleware("k")(ok),
			"APIKeyOrJWT": APIKeyOrJWTMiddleware("k", testSecret)(ok),
		} {
			req := httptest.NewRequest("GET", "/", nil)
			req.Header.Set(tc.header, tc.value)
			w := httptest.NewRecorder()
			h.ServeHTTP(w, req)
			if w.Code != tc.want {
				t.Errorf("%s via %s: got %d, want %d", mw, name, w.Code, tc.want)
			}
		}
	}
}
