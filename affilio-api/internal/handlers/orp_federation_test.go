package handlers

import (
	"context"
	"crypto/ed25519"
	"crypto/rand"
	"encoding/json"
	"net/http"
	"testing"
	"time"
)

type fedStub struct {
	pages map[string]string
	posts []struct {
		url  string
		body string
	}
	reply map[string]any
}

func (f *fedStub) fetch(ctx context.Context, method, rawURL string, payload []byte) (string, int, bool) {
	if method == http.MethodPost {
		f.posts = append(f.posts, struct {
			url  string
			body string
		}{rawURL, string(payload)})
		if f.reply != nil {
			raw, _ := json.Marshal(f.reply)
			return string(raw), 200, true
		}
		return `{"error":"not_found"}`, 404, true
	}
	body, ok := f.pages[rawURL]
	if !ok {
		return "", 0, false
	}
	return body, 200, true
}

func fedHost(t *testing.T, stub *fedStub) (*AgentEntry, http.Handler) {
	t.Helper()
	db, cleanup := setupTestDB(t)
	t.Cleanup(cleanup)
	entry := &AgentEntry{
		DB:           db,
		BaseURL:      "http://127.0.0.1:8080",
		AddressKey:   testAddressKey,
		ClaimsCheck:  "off-local",
		TokenLimiter: NewIPRateLimiter(8, time.Minute),
	}
	host := &ORPHost{Entry: entry}
	if stub != nil {
		host.FederationFetch = stub.fetch
	}
	return entry, mountORP(entry, host)
}

// A signed offer addressed to a foreign party relays through that host's
// discovery endpoint; the remote reply comes back verbatim.
func TestORPFederationOutbound(t *testing.T) {
	stub := &fedStub{
		pages: map[string]string{
			"https://other.example/.well-known/orp.json": `{"endpoint":"https://other.example/orp"}`,
		},
		reply: map[string]any{"orp": "0.3", "type": "deal", "deal": "deal_remote_1", "body": map[string]any{"state": "proposed"}},
	}
	_, r := fedHost(t, stub)
	pub, priv, _ := ed25519.GenerateKey(rand.Reader)
	token, party := joinWithKey(t, r, testJWK(pub, "fed-key-1"))

	offer := orpEnvelope("offer", "m_fed_out_1", party, standingOffer())
	offer["to"] = "orp:other.example/aaaaaaaa-bbbb-cccc-dddd-eeeeeeeeeeee"
	offer = signEnvelope(t, offer, priv, "fed-key-1")
	rec, body := postJSON(t, r, http.MethodPost, "/orp", offer, bearerHeader(token))
	if rec.Code != http.StatusOK {
		t.Fatalf("relay %d %s", rec.Code, rec.Body.String())
	}
	if body["deal"] != "deal_remote_1" {
		t.Fatalf("relay body %s", rec.Body.String())
	}
	if len(stub.posts) != 1 || stub.posts[0].url != "https://other.example/orp" {
		t.Fatalf("posts %+v", stub.posts)
	}
	var sent map[string]any
	if err := json.Unmarshal([]byte(stub.posts[0].body), &sent); err != nil || sent["sig"] == nil {
		t.Fatalf("relayed body %s", stub.posts[0].body)
	}
}

// Unsigned messages to a foreign party are refused before any fetch.
func TestORPFederationUnsigned(t *testing.T) {
	stub := &fedStub{}
	_, r := fedHost(t, stub)
	token, party := joinParty(t, r)
	offer := orpEnvelope("offer", "m_fed_no_1", party, standingOffer())
	offer["to"] = "orp:other.example/aaaaaaaa-bbbb-cccc-dddd-eeeeeeeeeeee"
	rec, _ := postJSON(t, r, http.MethodPost, "/orp", offer, bearerHeader(token))
	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("unsigned foreign %d %s", rec.Code, rec.Body.String())
	}
	if len(stub.posts) != 0 {
		t.Fatal("unsigned message fetched")
	}
}

// A signed envelope from a foreign party needs no bearer: the home host's
// keys endpoint authenticates it, and it reaches local dispatch.
func TestORPFederationInbound(t *testing.T) {
	pub, priv, _ := ed25519.GenerateKey(rand.Reader)
	stub := &fedStub{
		pages: map[string]string{
			"https://remote.example/orp/keys/fed-party-1": mustJSONString(t, map[string]any{"keys": []any{testJWK(pub, "remote-key-1")}}),
		},
	}
	_, r := fedHost(t, stub)

	hello := signEnvelope(t, orpEnvelope("hello", "m_fed_in_1", "orp:remote.example/fed-party-1", map[string]any{"versions": []any{"0.3"}}), priv, "remote-key-1")
	rec, body := postJSON(t, r, http.MethodPost, "/orp", hello, nil)
	if rec.Code != http.StatusOK || body["type"] != "hello" {
		t.Fatalf("foreign hello %d %s", rec.Code, rec.Body.String())
	}

	unsigned := orpEnvelope("hello", "m_fed_in_2", "orp:remote.example/fed-party-1", map[string]any{"versions": []any{"0.3"}})
	rec, _ = postJSON(t, r, http.MethodPost, "/orp", unsigned, nil)
	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("unsigned foreign %d %s", rec.Code, rec.Body.String())
	}

	bad := signEnvelope(t, orpEnvelope("hello", "m_fed_in_3", "orp:remote.example/fed-party-1", map[string]any{"versions": []any{"0.3"}}), priv, "remote-key-1")
	bad["id"] = "m_fed_in_4"
	rec, _ = postJSON(t, r, http.MethodPost, "/orp", bad, nil)
	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("bad foreign sig %d %s", rec.Code, rec.Body.String())
	}
}

// A foreign offer to a local party creates a real deal: the federated party
// gets a synthetic owner and the local party's inbox gets the deal.
func TestORPFederationOfferToLocal(t *testing.T) {
	pub, priv, _ := ed25519.GenerateKey(rand.Reader)
	foreignParty := "orp:remote.example/fed-party-9"
	stub := &fedStub{
		pages: map[string]string{
			"https://remote.example/orp/keys/fed-party-9": mustJSONString(t, map[string]any{"keys": []any{testJWK(pub, "remote-key-9")}}),
		},
	}
	entry, r := fedHost(t, stub)
	_, localParty := joinParty(t, r)

	offer := orpEnvelope("offer", "m_fed_offer_1", foreignParty, standingOffer())
	offer["to"] = localParty
	offer = signEnvelope(t, offer, priv, "remote-key-9")
	rec, body := postJSON(t, r, http.MethodPost, "/orp", offer, nil)
	if rec.Code != http.StatusOK {
		t.Fatalf("foreign offer %d %s", rec.Code, rec.Body.String())
	}
	inner, _ := body["body"].(map[string]any)
	if inner["state"] != "proposed" {
		t.Fatalf("deal %s", rec.Body.String())
	}
	messages, _, err := entry.DB.ListORPInbox(context.Background(), ownerOfParty(localParty), 0, 10)
	if err != nil {
		t.Fatal(err)
	}
	found := false
	for _, raw := range messages {
		var doc map[string]any
		if err := json.Unmarshal(raw, &doc); err == nil && doc["type"] == "deal" {
			found = true
		}
	}
	if !found {
		t.Fatal("local party inbox has no deal")
	}
}

func mustJSONString(t *testing.T, v map[string]any) string {
	t.Helper()
	raw, err := json.Marshal(v)
	if err != nil {
		t.Fatal(err)
	}
	return string(raw)
}
