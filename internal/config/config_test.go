package config

import (
	"strings"
	"testing"
)

func TestValidate(t *testing.T) {
	strong := Config{StaticDir: "/app/static", APIKey: "a-real-key", JWTSecret: strings.Repeat("x", 32), AdminPassword: "pw"}
	if err := strong.Validate(); err != nil {
		t.Errorf("strong secrets: %v", err)
	}

	dev := Config{StaticDir: "/app/static", APIKey: devAPIKey, JWTSecret: devJWTSecret, AdminPassword: devAdminPassword}
	err := dev.Validate()
	if err == nil {
		t.Fatal("dev secrets in a deployment were accepted")
	}
	for _, name := range []string{"API_KEY", "JWT_SECRET", "ADMIN_PASSWORD"} {
		if !strings.Contains(err.Error(), name) {
			t.Errorf("error doesn't name %s: %v", name, err)
		}
	}

	short := strong
	short.JWTSecret = "too-short"
	if err := short.Validate(); err == nil || !strings.Contains(err.Error(), "shorter") {
		t.Errorf("short JWT secret: %v", err)
	}

	// Local development (no STATIC_DIR) and the explicit override still run.
	local := dev
	local.StaticDir = ""
	if err := local.Validate(); err != nil {
		t.Errorf("local dev: %v", err)
	}
	dev.AllowDevSecrets = true
	if err := dev.Validate(); err != nil {
		t.Errorf("ALLOW_DEV_SECRETS: %v", err)
	}
}
