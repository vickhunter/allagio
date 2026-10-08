package config

import (
	"os"
	"strconv"
	"time"
)

type Config struct {
	Port                   string
	Host                   string
	POCReportPath          string
	PublicBaseURL          string
	UIAssetsPath           string
	DownloadsPath          string
	OperatorOwnerID        string
	AutoMigrate            bool
	OIDCIssuer             string
	OIDCClientID           string
	OIDCClientSecret       string
	DatabaseURL            string
	ClaudeAPIKey           string
	ClaudeModel            string
	ClaudeAPIURL           string
	ClaudeAPIVersion       string
	AllowedOrigins         string
	TrustedProxies         string
	APIKeySecret           string
	AdminAPIKey            string
	KeyRotationGracePeriod time.Duration

	ConnectionEncryptionKey   string
	StripeWebhookSecret       string
	LemonSqueezyWebhookSecret string
	PolarWebhookSecret        string
}

func Load() Config {
	graceHours, _ := strconv.Atoi(getEnvValue("KEY_ROTATION_GRACE_HOURS", "24"))
	if graceHours <= 0 {
		graceHours = 24
	}
	return Config{
		Port:                   getEnvValue("PORT", "8080"),
		Host:                   os.Getenv("HOST"),
		POCReportPath:          os.Getenv("POC_REPORT_PATH"),
		PublicBaseURL:          os.Getenv("PUBLIC_BASE_URL"),
		UIAssetsPath:           os.Getenv("UI_ASSETS_PATH"),
		DownloadsPath:          os.Getenv("DOWNLOADS_PATH"),
		OperatorOwnerID:        os.Getenv("OPERATOR_OWNER_ID"),
		AutoMigrate:            os.Getenv("AUTO_MIGRATE") == "true",
		OIDCIssuer:             os.Getenv("OIDC_ISSUER"),
		OIDCClientID:           os.Getenv("OIDC_CLIENT_ID"),
		OIDCClientSecret:       os.Getenv("OIDC_CLIENT_SECRET"),
		DatabaseURL:            getEnvValue("DATABASE_URL", "postgresql://affilio:password@localhost:5432/affilio"),
		ClaudeAPIKey:           os.Getenv("CLAUDE_API_KEY"),
		ClaudeModel:            getEnvValue("CLAUDE_MODEL", "claude-sonnet-4-5-20250929"),
		ClaudeAPIURL:           getEnvValue("CLAUDE_API_URL", ""),
		ClaudeAPIVersion:       getEnvValue("CLAUDE_API_VERSION", ""),
		AllowedOrigins:         getEnvValue("ALLOWED_ORIGINS", "*"),
		TrustedProxies:         os.Getenv("TRUSTED_PROXIES"),
		APIKeySecret:           getEnvValue("API_KEY_SECRET", "dev-secret"),
		AdminAPIKey:            getEnvValue("ADMIN_API_KEY", ""),
		KeyRotationGracePeriod: time.Duration(graceHours) * time.Hour,

		ConnectionEncryptionKey:   os.Getenv("CONNECTION_ENCRYPTION_KEY"),
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
