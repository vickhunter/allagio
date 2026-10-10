package config

import (
	"errors"
	"os"
	"strconv"
	"strings"
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
	GinMode                string
	APIKeySecret           string
	AdminAPIKey            string
	KeyRotationGracePeriod time.Duration
	PluginOfflineAfter     time.Duration

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
	offlineAfter := 10 * time.Minute
	if raw := strings.TrimSpace(os.Getenv("PLUGIN_OFFLINE_AFTER")); raw != "" {
		if parsed, err := time.ParseDuration(raw); err == nil && parsed > 0 {
			offlineAfter = parsed
		}
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
		GinMode:                os.Getenv("GIN_MODE"),
		APIKeySecret:           getEnvValue("API_KEY_SECRET", "dev-secret"),
		AdminAPIKey:            getEnvValue("ADMIN_API_KEY", ""),
		KeyRotationGracePeriod: time.Duration(graceHours) * time.Hour,
		PluginOfflineAfter:     offlineAfter,

		ConnectionEncryptionKey:   os.Getenv("CONNECTION_ENCRYPTION_KEY"),
		StripeWebhookSecret:       os.Getenv("STRIPE_WEBHOOK_SECRET"),
		LemonSqueezyWebhookSecret: os.Getenv("LEMONSQUEEZY_WEBHOOK_SECRET"),
		PolarWebhookSecret:        os.Getenv("POLAR_WEBHOOK_SECRET"),
	}
}

// ValidateRuntime refuses to boot a release process that still has the
// development API key secret or a wildcard CORS allowlist.
func (c Config) ValidateRuntime() error {
	if !strings.EqualFold(strings.TrimSpace(c.GinMode), "release") {
		return nil
	}
	secret := strings.TrimSpace(c.APIKeySecret)
	if secret == "" || secret == "dev-secret" {
		return errors.New("API_KEY_SECRET must be set to a non-default value when GIN_MODE=release")
	}
	origins := strings.TrimSpace(c.AllowedOrigins)
	if origins == "" || origins == "*" {
		return errors.New("ALLOWED_ORIGINS must list explicit origins when GIN_MODE=release")
	}
	return nil
}

func getEnvValue(key, defaultValue string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return defaultValue
}
