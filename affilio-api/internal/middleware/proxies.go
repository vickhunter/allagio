package middleware

import (
	"strings"

	"github.com/gin-gonic/gin"
)

// ConfigureTrustedProxies trusts only the comma-separated IPs and CIDRs in raw
// when reading X-Forwarded-For and X-Real-IP. An empty value trusts nothing:
// gin otherwise trusts every address, which lets a client spoof its IP.
func ConfigureTrustedProxies(engine *gin.Engine, raw string) error {
	var proxies []string
	for _, part := range strings.Split(raw, ",") {
		part = strings.TrimSpace(part)
		if part != "" {
			proxies = append(proxies, part)
		}
	}
	if len(proxies) == 0 {
		return engine.SetTrustedProxies(nil)
	}
	return engine.SetTrustedProxies(proxies)
}
