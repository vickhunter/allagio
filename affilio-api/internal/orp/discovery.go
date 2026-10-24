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
	// DocsPath is the /llms.txt section a cold agent follows from discovery.
	DocsPath = "/llms.txt#orp-0-3-test-deal"
	// ProofPageTemplate is the test proof URL with the origin and deal id open.
	ProofPageTemplate = "{origin}/orp/proof/<deal>"
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
// login. Accepting the pitch opens a test deal that stays awaiting.
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

// discoveryDescription names the test proof page and the llms.txt walkthrough.
func discoveryDescription() string {
	return "In testing. A cold agent reads " + DocsPath + ", then POST /orp with message format 0.3. Test proof page template: " + ProofPageTemplate + ". The deal stays awaiting. Offer Rails does not hold money. L0, L1, L2, and L3 are unclaimed. DCP v0.1 remains the live affiliate path."
}

// DiscoveryDocument is GET /.well-known/orp.json. conformance is omitted:
// this host does not claim L0, L1, L2, or L3.
func DiscoveryDocument(origin, dns, claimsMode string) map[string]any {
	return map[string]any{
		"orp":         []any{Version},
		"host":        HostParty(dns),
		"name":        TestingName,
		"description": discoveryDescription(),
		"docs":        strings.TrimRight(origin, "/") + DocsPath,
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

// Capabilities is what this testing host handles. It does not claim a
// conformance level. checks.claims only reports the configured mode.
func Capabilities(claimsMode string) map[string]any {
	return map[string]any{
		"messages": []any{"hello", "discover", "results", "offer", "accept", "deal", "execute", "receipt", "error"},
		"proofs":   []any{"proof_url"},
		"inbox":    map[string]any{"poll": true},
		"checks": map[string]any{
			"claims": ClaimsMode(claimsMode),
		},
	}
}
