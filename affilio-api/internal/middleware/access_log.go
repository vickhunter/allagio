package middleware

import (
	"fmt"
	"net"
	"regexp"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
)

// ipCandidate matches dotted IPv4 and colon-separated tokens that might be IPv6.
// A match is redacted only when net.ParseIP accepts it, so timestamps stay.
var ipCandidate = regexp.MustCompile(`(?:\d{1,3}\.){3}\d{1,3}|[0-9A-Fa-f:]{2,}`)

// AccessLogFormatter is the request log line. It records the path without the
// query and never the client address. Go-link lines are scrubbed again so an
// error message cannot carry a raw address either.
func AccessLogFormatter(p gin.LogFormatterParams) string {
	path := ""
	method := p.Method
	if p.Request != nil {
		if method == "" {
			method = p.Request.Method
		}
		if p.Request.URL != nil {
			path = p.Request.URL.Path
		}
	}
	logged := path
	scrub := isGoLinkPath(path)
	if redacted, ok := approvalLogPath(path); ok {
		logged = redacted
		scrub = true
	}
	line := fmt.Sprintf("%s %d %s %s %s\n", p.TimeStamp.Format(time.RFC3339), p.StatusCode, method, logged, p.Latency)
	if !scrub {
		return line
	}
	if p.ErrorMessage != "" {
		line = strings.TrimRight(line, "\n") + " " + p.ErrorMessage + "\n"
	}
	return ScrubLoggedIPs(line)
}

func isGoLinkPath(path string) bool {
	return strings.Contains(path, "/v1/go/") || strings.HasSuffix(path, "/v1/go")
}

// approvalLogPath redacts the escalation id on /orp/approve/<id>. The id is a
// capability. Those lines are scrubbed the same way as go-link lines.
func approvalLogPath(path string) (string, bool) {
	const marker = "/orp/approve/"
	i := strings.Index(path, marker)
	if i < 0 {
		return path, false
	}
	if path[i+len(marker):] == "" {
		return path, false
	}
	return path[:i+len(marker)] + "[redacted]", true
}

// ScrubLoggedIPs replaces raw IP addresses in a log line.
func ScrubLoggedIPs(line string) string {
	return ipCandidate.ReplaceAllStringFunc(line, func(token string) string {
		if net.ParseIP(token) == nil {
			return token
		}
		return "[redacted]"
	})
}
