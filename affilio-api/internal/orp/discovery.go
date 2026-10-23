package orp

import (
	"crypto/sha256"
	"encoding/hex"
	"net/url"
	"regexp"
	"strings"
)

const (
	// Version is the only ORP version this thin layer speaks.
	Version = "0.3"
	// TestingName is the discovery name. The host is in testing and claims
	// no conformance level.
	TestingName = "In testing"
)

var partyHostPattern = regexp.MustCompile(`^[A-Za-z0-9.\-]{1,253}(:[0-9]{1,5})?$`)

// Origin is the absolute origin and the party-id host (DNS name, optional port).
// publicBase wins when it is set, so a party id stays stable behind a proxy.
func Origin(publicBase, requestScheme, requestHost string) (origin, dns string) {
	base := strings.TrimRight(strings.TrimSpace(publicBase), "/")
	if base == "" {
		host := strings.TrimSpace(requestHost)
		if host == "" {
			host = "localhost"
		}
		scheme := requestScheme
		if scheme != "https" {
			scheme = "http"
		}
		base = scheme + "://" + host
	}
	parsed, err := url.Parse(base)
	if err != nil || parsed.Host == "" || parsed.Scheme != "http" && parsed.Scheme != "https" {
		return "http://localhost", "localhost"
	}
	dns = parsed.Host
	if !partyHostPattern.MatchString(dns) {
		dns = "localhost"
		return parsed.Scheme + "://localhost", dns
	}
	return parsed.Scheme + "://" + parsed.Host, dns
}

// HostParty is the host's own party id, orp:<dns>/host.
func HostParty(dns string) string {
	return "orp:" + dns + "/host"
}

// MemberParty maps an existing agent-join owner id onto an ORP party id.
// The owner id is the opaque part. It is not a second account.
func MemberParty(dns, ownerID string) string {
	return "orp:" + dns + "/" + ownerID
}

// CatalogParty is a stable pseudonymous id for a catalog row. It is not a
// login. Accepting the pitch does not open a deal on this thin layer.
func CatalogParty(dns, source, id string) string {
	sum := sha256.Sum256([]byte(source + ":" + id))
	return "orp:" + dns + "/c_" + hex.EncodeToString(sum[:8])
}

// ClaimsMode maps the process CLAIMS_CHECK value onto the ORP checks.claims
// enum. off-local is not an ORP mode, so it is reported as none. This does
// not change CLAIMS_CHECK.
func ClaimsMode(mode string) string {
	switch strings.ToLower(strings.TrimSpace(mode)) {
	case "real":
		return "real"
	case "stub":
		return "stub"
	default:
		return "none"
	}
}

// DiscoveryDocument is GET /.well-known/orp.json. conformance is omitted:
// this host does not claim L0, L1, L2, or L3.
func DiscoveryDocument(origin, dns, claimsMode string) map[string]any {
	return map[string]any{
		"orp":         []any{Version},
		"host":        HostParty(dns),
		"name":        TestingName,
		"description": "In testing. Hosted ORP 0.3 discovery, suggest, and join for a cold agent. Conformance levels L0, L1, L2, and L3 are unclaimed. DCP v0.1 remains the live affiliate path.",
		"endpoint":    origin + "/orp",
		"suggest":     origin + "/orp/suggest",
		"join":        origin + "/orp/join",
		"auth": map[string]any{
			"bearer": true,
		},
		"capabilities": Capabilities(claimsMode),
		"legacy": map[string]any{
			"dcp": map[string]any{
				"version":   "0.1",
				"discovery": "/.well-known/dcp.json",
				"http_base": "/v1",
			},
		},
	}
}

// Capabilities is what this thin layer actually handles. Deal-opening types
// are absent, and checks.claims reports the configured mode.
func Capabilities(claimsMode string) map[string]any {
	return map[string]any{
		"messages": []any{"hello", "discover", "results", "error"},
		"proofs":   []any{"link"},
		"checks": map[string]any{
			"claims": ClaimsMode(claimsMode),
		},
	}
}
