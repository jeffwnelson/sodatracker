package config

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
)

type Config struct {
	Port          int
	DBPath        string
	APIKey        string
	JWTSecret     string
	StaticDir     string
	AdminPassword string
	LogFormat     string

	// Slack integration. Each part is optional and turns on independently:
	// the webhook posts alerts, the app token receives /soda over Socket
	// Mode, and the signing secret enables the public HTTP command endpoint.
	SlackWebhookURL    string
	SlackAppToken      string
	SlackSigningSecret string

	// AllowDevSecrets lets a deployment (STATIC_DIR set) start with the
	// public dev-* defaults, for trying the container locally.
	AllowDevSecrets bool

	// BackupDir holds local backup snapshots (default: backups/ beside the
	// database). The rest of the backup settings are edited in the UI.
	BackupDir string
}

func Load() Config {
	dbPath := getEnv("DB_PATH", "sodatracker.db")
	return Config{
		Port:          getEnvInt("PORT", 3000),
		DBPath:        dbPath,
		APIKey:        getEnv("API_KEY", devAPIKey),
		JWTSecret:     getEnv("JWT_SECRET", devJWTSecret),
		StaticDir:     getEnv("STATIC_DIR", ""),
		AdminPassword: getEnv("ADMIN_PASSWORD", devAdminPassword),
		LogFormat:     getEnv("LOG_FORMAT", "text"),

		SlackWebhookURL:    getEnv("SLACK_WEBHOOK_URL", ""),
		SlackAppToken:      getEnv("SLACK_APP_TOKEN", ""),
		SlackSigningSecret: getEnv("SLACK_SIGNING_SECRET", ""),

		BackupDir: getEnv("BACKUP_DIR", filepath.Join(filepath.Dir(dbPath), "backups")),

		AllowDevSecrets: os.Getenv("ALLOW_DEV_SECRETS") == "true",
	}
}

// Development defaults. They are in this repository, so anyone could use
// them to forge an admin token or call the API.
const (
	devAPIKey        = "dev-api-key"
	devJWTSecret     = "dev-jwt-secret"
	devAdminPassword = "dev-admin-password"
)

// minJWTSecret is the shortest JWT secret a deployment accepts; HS256 wants
// at least 32 bytes of key.
const minJWTSecret = 32

// Validate refuses a deployment that would run with guessable secrets.
// Deployments are told apart from local development by STATIC_DIR, which only
// the container sets (`make run` and the Vite dev server leave it empty).
func (c Config) Validate() error {
	if c.StaticDir == "" || c.AllowDevSecrets {
		return nil
	}
	var problems []string
	for name, v := range map[string]struct{ got, dev string }{
		"API_KEY":        {c.APIKey, devAPIKey},
		"JWT_SECRET":     {c.JWTSecret, devJWTSecret},
		"ADMIN_PASSWORD": {c.AdminPassword, devAdminPassword},
	} {
		if v.got == v.dev {
			problems = append(problems, name+" is the public development default")
		}
	}
	if c.JWTSecret != devJWTSecret && len(c.JWTSecret) < minJWTSecret {
		problems = append(problems, fmt.Sprintf("JWT_SECRET is shorter than %d characters", minJWTSecret))
	}
	if len(problems) == 0 {
		return nil
	}
	sort.Strings(problems)
	return errors.New(strings.Join(problems, "; ") +
		" (set them in .env, e.g. with `openssl rand -base64 32`; ALLOW_DEV_SECRETS=true to try the container locally)")
}

func getEnv(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}

func getEnvInt(key string, fallback int) int {
	if v := os.Getenv(key); v != "" {
		if i, err := strconv.Atoi(v); err == nil {
			return i
		}
	}
	return fallback
}
