package dcp

import (
	"os"
	"strings"
)

// ServerName is the MCP server name an outside agent sees. Discovery, stdio,
// and both initialize responses use this one value.
const ServerName = "offer-rails-mcp"

// ServerTitle is the human-readable serverInfo title.
const ServerTitle = "Offer Rails"

// Environment names the stdio server reads. The OFFERRAILS_* name is preferred.
// The DISTRIBUTOOR_* name remains so a host that still sets the old variable
// keeps working.
const (
	EnvAPIURL  = "OFFERRAILS_API_URL"
	EnvAPIKey  = "OFFERRAILS_API_KEY"
	EnvSiteKey = "OFFERRAILS_SITE_KEY"

	LegacyEnvAPIURL  = "DISTRIBUTOOR_API_URL"
	LegacyEnvAPIKey  = "DISTRIBUTOOR_API_KEY"
	LegacyEnvSiteKey = "DISTRIBUTOOR_SITE_KEY"
)

// Getenv returns the trimmed primary value, or the legacy value when the
// primary is unset or blank.
func Getenv(primary, legacy string) string {
	if value := strings.TrimSpace(os.Getenv(primary)); value != "" {
		return value
	}
	return strings.TrimSpace(os.Getenv(legacy))
}
