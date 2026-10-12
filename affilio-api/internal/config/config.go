package config

import (
	"errors"
	"os"
	"strconv"
	"strings"
	"testing"
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

	// ClaimsCheck is the resolved CLAIMS_CHECK mode: stub, real, or off-local.
	ClaimsCheck    string
	claimsCheckErr error
}

const (
	// ClaimsStub holds every untrusted offer in review. It is the hosted default.
	ClaimsStub = "stub"
	// ClaimsReal is the page checker. It is refused on a hosted server until that
	// checker is installed, and until then it does not auto-list.
	ClaimsReal = "real"
	// ClaimsOffLocal keeps today's four listing checks. Local and dev only.
	ClaimsOffLocal = "off-local"
)

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
	claims, claimsErr := ResolveClaimsCheck(os.Getenv("GIN_MODE"), os.Getenv("CLAIMS_CHECK"))
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
		APIKeySecret:           os.Getenv("API_KEY_SECRET"),
		AdminAPIKey:            getEnvValue("ADMIN_API_KEY", ""),
		KeyRotationGracePeriod: time.Duration(graceHours) * time.Hour,
		PluginOfflineAfter:     offlineAfter,

		ConnectionEncryptionKey:   os.Getenv("CONNECTION_ENCRYPTION_KEY"),
		StripeWebhookSecret:       os.Getenv("STRIPE_WEBHOOK_SECRET"),
		LemonSqueezyWebhookSecret: os.Getenv("LEMONSQUEEZY_WEBHOOK_SECRET"),
		PolarWebhookSecret:        os.Getenv("POLAR_WEBHOOK_SECRET"),
		ClaimsCheck:               claims,
		claimsCheckErr:            claimsErr,
	}
}

// Hosted reports whether this process is the hosted server. The signal is
// GIN_MODE=release, the same check ValidateRuntime uses. Empty, debug, and
// test are local or dev.
func Hosted(ginMode string) bool {
	return strings.EqualFold(strings.TrimSpace(ginMode), "release")
}

// ResolveClaimsCheck applies CLAIMS_CHECK. An empty value is stub on a hosted
// server and off-local everywhere else. off-local and real are refused when
// GIN_MODE=release. real is accepted locally, and the listing gate still holds
// the offer until the page checker replaces that branch.
func ResolveClaimsCheck(ginMode, raw string) (string, error) {
	mode := strings.ToLower(strings.TrimSpace(raw))
	hosted := Hosted(ginMode)
	switch mode {
	case "":
		if hosted {
			return ClaimsStub, nil
		}
		return ClaimsOffLocal, nil
	case ClaimsStub:
		return ClaimsStub, nil
	case ClaimsOffLocal:
		if hosted {
			return "", errors.New("CLAIMS_CHECK=off-local is only for local and dev. A hosted server (GIN_MODE=release) stays on stub")
		}
		return ClaimsOffLocal, nil
	case ClaimsReal:
		if hosted {
			return "", errors.New("CLAIMS_CHECK=real is not allowed while GIN_MODE=release. The hosted default stays stub until the page checker is installed")
		}
		return ClaimsReal, nil
	default:
		return "", errors.New("CLAIMS_CHECK must be stub, real, or off-local")
	}
}

// ValidateClaims refuses to boot when CLAIMS_CHECK is unknown or when a hosted
// server asks for real or off-local.
func (c Config) ValidateClaims() error {
	return c.claimsCheckErr
}

// SecretRefused reports an API key secret the server must not boot with.
func SecretRefused(secret string) error {
	secret = strings.TrimSpace(secret)
	if secret == "" || secret == "dev-secret" {
		return errors.New("API_KEY_SECRET must be set to a non-default value")
	}
	return nil
}

// ValidateSecrets refuses to boot outside tests when the API key secret is
// missing or still the development value.
func (c Config) ValidateSecrets() error {
	if testing.Testing() {
		return nil
	}
	return SecretRefused(c.APIKeySecret)
}

// ValidateRuntime refuses to boot a release process that still has the
// development API key secret or a wildcard CORS allowlist.
func (c Config) ValidateRuntime() error {
	if !Hosted(c.GinMode) {
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
