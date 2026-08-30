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

	StripeWebhookSecret       string
	LemonSqueezyWebhookSecret string
	PolarWebhookSecret        string
}

func Load() Config {
	return Config{
		Port:             getEnvValue("PORT", "8080"),
		DatabaseURL:      getEnvValue("DATABASE_URL", "postgresql://affilio:password@localhost:5432/affilio"),
		ClaudeAPIKey:     os.Getenv("CLAUDE_API_KEY"),
		ClaudeModel:      getEnvValue("CLAUDE_MODEL", "claude-sonnet-4-5-20250929"),
		ClaudeAPIURL:     getEnvValue("CLAUDE_API_URL", ""),
		ClaudeAPIVersion: getEnvValue("CLAUDE_API_VERSION", ""),
		AllowedOrigins:   getEnvValue("ALLOWED_ORIGINS", "*"),
		APIKeySecret:     getEnvValue("API_KEY_SECRET", "dev-secret"),
		AdminAPIKey:      getEnvValue("ADMIN_API_KEY", ""),

		StripeWebhookSecret:       os.Getenv("STRIPE_WEBHOOK_SECRET"),
		LemonSqueezyWebhookSecret: os.Getenv("LEMONSQUEEZY_WEBHOOK_SECRET"),
		PolarWebhookSecret:        os.Getenv("POLAR_WEBHOOK_SECRET"),
	}
}

func getEnvValue(key, defaultValue string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return defaultValue
}
