package config

import "os"

type Config struct {
	Port             string
	DatabaseURL      string
	ClaudeAPIKey     string
	ClaudeModel      string
	ClaudeAPIURL     string
	ClaudeAPIVersion string
	AllowedOrigins   string
	APIKeySecret     string
	AdminAPIKey      string
}

func Load() Config {
	return Config{
		Port:             envOr("PORT", "8080"),
		DatabaseURL:      envOr("DATABASE_URL", "postgresql://affilio:password@localhost:5432/affilio"),
		ClaudeAPIKey:     os.Getenv("CLAUDE_API_KEY"),
		ClaudeModel:      envOr("CLAUDE_MODEL", "claude-sonnet-4-5-20250929"),
		ClaudeAPIURL:     envOr("CLAUDE_API_URL", ""),
		ClaudeAPIVersion: envOr("CLAUDE_API_VERSION", ""),
		AllowedOrigins:   envOr("ALLOWED_ORIGINS", "*"),
		APIKeySecret:     envOr("API_KEY_SECRET", "dev-secret"),
		AdminAPIKey:      envOr("ADMIN_API_KEY", ""),
	}
}

func envOr(key, def string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return def
}
