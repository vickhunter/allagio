package dcp

import (
	"os"
	"strings"
)

// ServerName is the MCP server name an outside agent sees. Discovery, stdio,
// and both initialize responses use this one value.
const ServerName = "allagio-mcp"

// ServerTitle is the human-readable serverInfo title.
const ServerTitle = "Allagio"

// Environment names the stdio server reads. ALLAGIO_* is preferred.
// OFFERRAILS_* and DISTRIBUTOOR_* remain so a host that still sets an old
// variable keeps working. Discovery advertises only ALLAGIO_*.
const (
	EnvAPIURL  = "ALLAGIO_API_URL"
	EnvAPIKey  = "ALLAGIO_API_KEY"
	EnvSiteKey = "ALLAGIO_SITE_KEY"

	LegacyEnvAPIURL  = "OFFERRAILS_API_URL"
	LegacyEnvAPIKey  = "OFFERRAILS_API_KEY"
	LegacyEnvSiteKey = "OFFERRAILS_SITE_KEY"

	OlderEnvAPIURL  = "DISTRIBUTOOR_API_URL"
	OlderEnvAPIKey  = "DISTRIBUTOOR_API_KEY"
	OlderEnvSiteKey = "DISTRIBUTOOR_SITE_KEY"
)

// Getenv returns the first non-blank trimmed value.
func Getenv(names ...string) string {
	for _, name := range names {
		if value := strings.TrimSpace(os.Getenv(name)); value != "" {
			return value
		}
	}
	return ""
}
