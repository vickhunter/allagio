package orp

import (
	"net/url"
	"strings"
)

// TestDeal reports a deal the host may verify from its own test fixture.
// The title is the opt-in: terms reject unknown keys, and this host's
// discover pool is already named test. "TEST" and "TEST shout-out swap"
// match. "Testing" does not.
func TestDeal(terms map[string]any) bool {
	title, _ := terms["title"].(string)
	title = strings.TrimSpace(title)
	return title == "TEST" || strings.HasPrefix(title, "TEST ")
}

// TestFixtureURL is the owner-only page that renders one obligation.
func TestFixtureURL(origin, dealID, obligationID string) string {
	return strings.TrimRight(origin, "/") + "/orp/proof/" + dealID + "/test/" + obligationID
}

// SplitTestFixture reads /orp/proof/<deal>/test/<obligation> off an absolute URL.
func SplitTestFixture(raw string) (host, dealID, obligationID string, ok bool) {
	u, err := url.Parse(strings.TrimSpace(raw))
	if err != nil || u.Host == "" {
		return "", "", "", false
	}
	parts := strings.Split(strings.Trim(u.Path, "/"), "/")
	if len(parts) != 5 || parts[0] != "orp" || parts[1] != "proof" || parts[3] != "test" {
		return "", "", "", false
	}
	if parts[2] == "" || parts[4] == "" {
		return "", "", "", false
	}
	return u.Host, parts[2], parts[4], true
}

// OwnTestFixture reports a proof_url that is this host's fixture for this
// deal and obligation. A different scheme or host is someone else's page.
func OwnTestFixture(origin, raw, dealID, obligationID string) bool {
	if !sameOrigin(origin, raw) {
		return false
	}
	_, id, ob, ok := SplitTestFixture(raw)
	return ok && id == dealID && ob == obligationID
}

func sameOrigin(origin, raw string) bool {
	a, err1 := url.Parse(strings.TrimSpace(origin))
	b, err2 := url.Parse(strings.TrimSpace(raw))
	if err1 != nil || err2 != nil || a.Host == "" || b.Host == "" {
		return false
	}
	return strings.EqualFold(a.Scheme, b.Scheme) && strings.EqualFold(a.Host, b.Host)
}
