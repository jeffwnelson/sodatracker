package config

import (
	"os"
	"strconv"
)

type Config struct {
	Port          int
	DBPath        string
	APIKey        string
	JWTSecret     string
	StaticDir     string
	AdminPassword string
	TableName     string
	SNSTopicARN   string
}

func Load() Config {
	return Config{
		Port:          getEnvInt("PORT", 3000),
		DBPath:        getEnv("DB_PATH", "sodatracker.db"),
		APIKey:        getEnv("API_KEY", "dev-api-key"),
		JWTSecret:     getEnv("JWT_SECRET", "dev-jwt-secret"),
		StaticDir:     getEnv("STATIC_DIR", ""),
		AdminPassword: getEnv("ADMIN_PASSWORD", "dev-admin-password"),
		TableName:     getEnv("TABLE_NAME", ""),
		SNSTopicARN:   getEnv("SNS_TOPIC_ARN", ""),
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
