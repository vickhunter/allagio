package orp

import (
	"crypto/rand"
	"encoding/base64"
	"regexp"
	"strings"
	"time"
)

var ownerIDPattern = regexp.MustCompile(`^[0-9a-f]{8}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{12}$`)

// LocalID is a host-scoped id with a short prefix, such as deal_ or ofr_.
func LocalID(prefix string) string {
	var buf [9]byte
	if _, err := rand.Read(buf[:]); err != nil {
		return prefix + "fallbackid"
	}
	return prefix + base64.RawURLEncoding.EncodeToString(buf[:])
}

// ProofURL is this host's public test proof page for a deal.
func ProofURL(origin, dealID string) string {
	return strings.TrimRight(origin, "/") + "/orp/proof/" + dealID
}

// ApproveURL is where the waiting party is sent. Opening it does not record
// an approval. The signed-in owner records approved_by owner by submitting
// the form there, and only when that owner opted in.
func ApproveURL(origin, escalation string) string {
	return strings.TrimRight(origin, "/") + "/orp/approve/" + escalation
}

// OwnerFromParty returns the agent-account id carried in a member party id.
func OwnerFromParty(dns, party string) (string, bool) {
	prefix := "orp:" + dns + "/"
	if !strings.HasPrefix(party, prefix) {
		return "", false
	}
	id := strings.TrimPrefix(party, prefix)
	if !ownerIDPattern.MatchString(id) {
		return "", false
	}
	return id, true
}

// Stamp is the UTC RFC3339 form this host writes on messages.
func Stamp(now time.Time) string {
	return now.UTC().Format(time.RFC3339)
}

// ListedOffer is the host reply that names a standing offer.
func ListedOffer(dns, to, re string, body map[string]any, now time.Time) map[string]any {
	return envelope(dns, to, re, "", "offer", body, now)
}

// DealMessage is one party's deal snapshot.
func DealMessage(dns, to, re, dealID string, body map[string]any, now time.Time) map[string]any {
	return envelope(dns, to, re, dealID, "deal", body, now)
}

// ReceiptMessage is a host proof_url receipt.
func ReceiptMessage(dns, to, re, dealID string, body map[string]any, now time.Time) map[string]any {
	return envelope(dns, to, re, dealID, "receipt", body, now)
}

func envelope(dns, to, re, dealID, typ string, body map[string]any, now time.Time) map[string]any {
	doc := map[string]any{
		"orp":  Version,
		"type": typ,
		"id":   NewMessageID(),
		"ts":   Stamp(now),
		"from": HostParty(dns),
		"to":   to,
		"body": body,
	}
	if re != "" {
		doc["re"] = re
	}
	if dealID != "" {
		doc["deal"] = dealID
	}
	return doc
}

// WaitingView hides escalation reasons and the approval URL from everyone
// except the party that is waiting.
func WaitingView(waiting map[string]any, viewer string) map[string]any {
	if waiting == nil {
		return nil
	}
	party, _ := waiting["party"].(string)
	since, _ := waiting["since"].(string)
	if viewer == party {
		out := map[string]any{}
		for key, value := range waiting {
			out[key] = value
		}
		return out
	}
	return map[string]any{"party": party, "since": since}
}

// DealView is the snapshot body for one viewer.
func DealView(body map[string]any, viewer string) map[string]any {
	out := map[string]any{}
	for key, value := range body {
		if key == "waiting" {
			continue
		}
		out[key] = value
	}
	if waiting, ok := body["waiting"].(map[string]any); ok && waiting != nil {
		out["waiting"] = WaitingView(waiting, viewer)
	}
	return out
}

// SpaceReply answers a space put or withdraw with the listed form.
func SpaceReply(dns, to, re string, body map[string]any, now time.Time) map[string]any {
	return envelope(dns, to, re, "", "space", body, now)
}
