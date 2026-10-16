package handlers

import (
	"net/url"
	"regexp"
	"strings"

	"github.com/gin-gonic/gin"
)

// landingSourcePattern is a campaign tag from /from/<tag>, ?source=, or the
// or_source cookie. It is wider than the owner attribution allow-list so a
// tag such as "test" survives onto the landing links. Owner signup still
// stores only the owner allow-list.
var landingSourcePattern = regexp.MustCompile(`^[a-z0-9][a-z0-9_-]{0,63}$`)

var landingHref = regexp.MustCompile(`href="([^"]*)"`)

func landingSourceTag(raw string) string {
	value := strings.ToLower(strings.TrimSpace(raw))
	if value == "" || value == "direct" || !landingSourcePattern.MatchString(value) {
		return ""
	}
	return value
}

// landingSourceFrom reads the tag from ?source= or the or_source cookie.
// Referrer-Policy is strict-origin, so the Referer path is not a source.
func landingSourceFrom(c *gin.Context) string {
	if tag := landingSourceTag(c.Query("source")); tag != "" {
		return tag
	}
	cookie, err := c.Cookie("or_source")
	if err != nil {
		return ""
	}
	return landingSourceTag(cookie)
}

func landingLinkTracksSource(path string) bool {
	switch path {
	case "/", "/makers", "/publishers", "/protocol", "/owner", "/auth/login":
		return true
	}
	return strings.HasPrefix(path, "/makers/") || strings.HasPrefix(path, "/publishers/") || strings.HasPrefix(path, "/protocol/") || strings.HasPrefix(path, "/owner/")
}

func withLandingSource(href, source string) string {
	if source == "" || href == "" || strings.HasPrefix(href, "#") {
		return href
	}
	raw := strings.ReplaceAll(href, "&amp;", "&")
	parsed, err := url.Parse(raw)
	if err != nil || parsed.IsAbs() || parsed.Host != "" || !landingLinkTracksSource(parsed.Path) {
		return href
	}
	query := parsed.Query()
	if parsed.Path == "/auth/login" {
		if ret := query.Get("return_to"); ret != "" {
			query.Set("return_to", withLandingSource(ret, source))
		}
	}
	query.Set("source", source)
	parsed.RawQuery = query.Encode()
	return parsed.RequestURI()
}

func stampLandingLinks(html []byte, source string) []byte {
	if source == "" {
		return html
	}
	return landingHref.ReplaceAllFunc(html, func(match []byte) []byte {
		parts := landingHref.FindSubmatch(match)
		if len(parts) != 2 {
			return match
		}
		next := withLandingSource(string(parts[1]), source)
		if next == string(parts[1]) {
			return match
		}
		return []byte(`href="` + strings.ReplaceAll(next, "&", "&amp;") + `"`)
	})
}
