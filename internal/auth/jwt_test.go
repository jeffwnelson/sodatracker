package auth

import (
	"net/http"
	"net/http/httptest"
	"testing"
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
