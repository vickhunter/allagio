package config

import (
	"os"
	"strconv"
	"time"
)

type Config struct {
	Port                  string
	DatabaseURL           string
	ClaudeAPIKey          string
	ClaudeModel           string
	ClaudeAPIURL          string
	ClaudeAPIVersion      string
	AllowedOrigins        string
	APIKeySecret          string
	AdminAPIKey           string
	KeyRotationGracePeriod time.Duration
}

func Load() Config {
	graceHours, _ := strconv.Atoi(getEnvValue("KEY_ROTATION_GRACE_HOURS", "24"))
	if graceHours <= 0 {
		graceHours = 24
	}
	return Config{
		Port:                   getEnvValue("PORT", "8080"),
		DatabaseURL:            getEnvValue("DATABASE_URL", "postgresql://affilio:password@localhost:5432/affilio"),
		ClaudeAPIKey:           os.Getenv("CLAUDE_API_KEY"),
		ClaudeModel:            getEnvValue("CLAUDE_MODEL", "claude-sonnet-4-5-20250929"),
		ClaudeAPIURL:           getEnvValue("CLAUDE_API_URL", ""),
		ClaudeAPIVersion:       getEnvValue("CLAUDE_API_VERSION", ""),
		AllowedOrigins:         getEnvValue("ALLOWED_ORIGINS", "*"),
		APIKeySecret:           getEnvValue("API_KEY_SECRET", "dev-secret"),
		AdminAPIKey:            getEnvValue("ADMIN_API_KEY", ""),
		KeyRotationGracePeriod: time.Duration(graceHours) * time.Hour,
	}
}

func getEnvValue(key, defaultValue string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return defaultValue
}
