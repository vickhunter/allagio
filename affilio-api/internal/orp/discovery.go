package orp

import (
	"crypto/sha256"
	"encoding/hex"
	"net/url"
	"regexp"
	"strings"

	"affilio-api/internal/chainpay"
	"affilio-api/internal/config"
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
func discoveryDescription(origin string) string {
	text := "In testing. A cold agent reads " + DocsPath + ", then POST /orp. Message schema: {{PUBLIC_BASE_URL}}/orp/schemas/envelope.schema.json. Protocol: {{PUBLIC_BASE_URL}}/orp/spec. Public proof page template: " + ProofPageTemplate + ". POST /orp/deals/<deal>/activate moves an accepted deal to active. Execute claims an active deal. Allagio does not hold money. L0, L1, L2, and L3 are unclaimed. DCP v0.1 remains the live affiliate path. Many-party, leave, seat."
	return config.ExpandPublicBase(text, origin)
}

// DiscoveryDocument is GET /.well-known/orp.json. conformance is omitted:
// this host does not claim L0, L1, L2, or L3.
func DiscoveryDocument(origin, dns, claimsMode string, hostKeys ...map[string]any) map[string]any {
	doc := map[string]any{
		"orp":         []any{Version04, Version},
		"host":        HostParty(dns),
		"name":        TestingName,
		"description": discoveryDescription(origin),
		"docs":        strings.TrimRight(origin, "/") + DocsPath,
		"endpoint":    origin + "/orp",
		"suggest":     origin + "/orp/suggest",
		"join":        origin + "/orp/join",
		"auth": map[string]any{
			"bearer": true,
		},
		"capabilities": hostCapabilities(claimsMode),
		"legacy": map[string]any{
			"dcp": map[string]any{
				"version":   "0.1",
				"discovery": "/.well-known/dcp.json",
				"http_base": "/v1",
			},
		},
	}
	if len(hostKeys) > 0 {
		keys := make([]any, 0, len(hostKeys))
		for _, jwk := range hostKeys {
			keys = append(keys, jwk)
		}
		doc["keys"] = keys
	}
	return doc
}

// Capabilities is what a 0.3 hello lists. It does not claim a conformance
// level. checks.claims only reports the configured mode. leave and seat are
// not in this list.
func Capabilities(claimsMode string) map[string]any {
	return map[string]any{
		"messages": []any{"hello", "discover", "results", "space", "request", "offer", "counter", "accept", "decline", "deal", "execute", "receipt", "payout", "revoke", "dispute", "reputation", "error"},
		"proofs":   []any{"proof_url"},
		"inbox":    map[string]any{"poll": true},
		"checks": map[string]any{
			"claims": ClaimsMode(claimsMode),
		},
	}
}

// CapabilitiesFor advertises leave and seat only on the 0.4 host document
// and a 0.4 hello. A 0.3 hello keeps Capabilities.
func CapabilitiesFor(version, claimsMode string) map[string]any {
	caps := Capabilities(claimsMode)
	if version != Version04 {
		return caps
	}
	messages := append([]any{}, caps["messages"].([]any)...)
	caps["messages"] = append(messages, "leave", "seat")
	return caps
}

// hostCapabilities is the discovery document's capability set: the 0.4 set
// plus the extensions this host serves.
func hostCapabilities(claimsMode string) map[string]any {
	caps := CapabilitiesFor(Version04, claimsMode)
	caps["ext"] = []any{chainpay.Profile}
	return caps
}
