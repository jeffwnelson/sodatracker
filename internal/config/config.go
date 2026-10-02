package config

import (
	"os"
	"path/filepath"
	"strconv"
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

	// BackupDir holds local backup snapshots (default: backups/ beside the
	// database). The rest of the backup settings are edited in the UI.
	BackupDir string
}

func Load() Config {
	dbPath := getEnv("DB_PATH", "sodatracker.db")
	return Config{
		Port:          getEnvInt("PORT", 3000),
		DBPath:        dbPath,
		APIKey:        getEnv("API_KEY", "dev-api-key"),
		JWTSecret:     getEnv("JWT_SECRET", "dev-jwt-secret"),
		StaticDir:     getEnv("STATIC_DIR", ""),
		AdminPassword: getEnv("ADMIN_PASSWORD", "dev-admin-password"),
		LogFormat:     getEnv("LOG_FORMAT", "text"),

		SlackWebhookURL:    getEnv("SLACK_WEBHOOK_URL", ""),
		SlackAppToken:      getEnv("SLACK_APP_TOKEN", ""),
		SlackSigningSecret: getEnv("SLACK_SIGNING_SECRET", ""),

		BackupDir: getEnv("BACKUP_DIR", filepath.Join(filepath.Dir(dbPath), "backups")),
	}
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
