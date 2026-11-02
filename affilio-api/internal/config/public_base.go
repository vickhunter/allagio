package config

import (
	"net/url"
	"strings"
)

// DefaultPublicBaseURL is the only hosted origin stored in Go.
// PUBLIC_BASE_URL overrides it. Buying a domain changes this one string.
// ORP 0.3 schema $id values keep today's host as frozen data. They are not a second setting.
const DefaultPublicBaseURL = "https://offer-rails.onrender.com"

// PublicBasePlaceholder is the token in static copy. Serve time replaces it
// with the document origin so the host is not copied into each document.
const PublicBasePlaceholder = "{{PUBLIC_BASE_URL}}"

// PublicBaseURL returns the origin with no trailing slash.
// An empty value uses DefaultPublicBaseURL.
func PublicBaseURL(raw string) string {
	base := strings.TrimRight(strings.TrimSpace(raw), "/")
	if base == "" {
		return DefaultPublicBaseURL
	}
	return base
}

// DocumentOrigin is the origin written into public documents.
// A loopback base is the local server, not the public host, so documents
// keep the default until PUBLIC_BASE_URL is a public origin.
func DocumentOrigin(raw string) string {
	base := PublicBaseURL(raw)
	parsed, err := url.Parse(base)
	if err != nil || parsed.Hostname() == "" {
		return DefaultPublicBaseURL
	}
	switch strings.ToLower(parsed.Hostname()) {
	case "localhost", "127.0.0.1", "::1":
		return DefaultPublicBaseURL
	default:
		return base
	}
}

// ExpandPublicBase replaces PublicBasePlaceholder with the document origin.
func ExpandPublicBase(text, raw string) string {
	return strings.ReplaceAll(text, PublicBasePlaceholder, DocumentOrigin(raw))
}
