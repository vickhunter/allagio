package entry

import (
	"net/url"
	"strings"
)

// ClickSource is the click's source. Clicks store a referrer and no separate
// source column, so the host of the referrer is the source. An empty referrer,
// or one with no host, counts as direct.
func ClickSource(referrer string) string {
	referrer = strings.TrimSpace(referrer)
	if referrer == "" {
		return "direct"
	}
	parsed, err := url.Parse(referrer)
	if err != nil || parsed.Host == "" {
		return "direct"
	}
	return strings.ToLower(parsed.Hostname())
}
