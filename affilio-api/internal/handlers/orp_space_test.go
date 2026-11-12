package handlers

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"net/url"
	"testing"
	"time"

	"affilio-api/internal/orp"
)

func putSpace(t *testing.T, r http.Handler, token, from, id, spaceID string) (int, map[string]any) {
	t.Helper()
	msg := orpEnvelope("space", id, from, map[string]any{
		"action": "put",
		"space": map[string]any{
			"id":       spaceID,
			"kind":     "newsletter.issue",
			"platform": "newsletter",
			"title":    "Indie Stack Weekly",
			"locator":  "https://indiestack.example/issues",
			"topics":   []any{"software.productivity"},
		},
	})
	rec, body := postJSON(t, r, http.MethodPost, "/orp", msg, bearerHeader(token))
	return rec.Code, body
}

func discoverSpaces(t *testing.T, r http.Handler, token, from, id string) []any {
	t.Helper()
	rec, body := postJSON(t, r, http.MethodPost, "/orp", orpEnvelope("discover", id, from, map[string]any{"want": "spaces"}), bearerHeader(token))
	if rec.Code != http.StatusOK {
		t.Fatalf("discover %d %s", rec.Code, rec.Body.String())
	}
	inner, _ := body["body"].(map[string]any)
	items, _ := inner["items"].([]any)
	return items
}

// A fresh party's list step is ask: put stores the space waiting, and it is
// not discoverable until the owner lists it.
func TestORPSpacePutAskWaiting(t *testing.T) {
	_, r := trustHost(t)
	pubToken, publisher := joinParty(t, r)
	makerToken, maker := joinParty(t, r)

	code, body := putSpace(t, r, pubToken, publisher, "m_space01", "sp_news_1")
	inner, _ := body["body"].(map[string]any)
	waiting, _ := inner["waiting"].(map[string]any)
	if code != http.StatusOK || inner["status"] != "waiting" || waiting["step"] != "list" {
		t.Fatalf("space put %d %v", code, body)
	}
	if items := discoverSpaces(t, r, makerToken, maker, "m_spdcd1"); len(items) != 0 {
		t.Fatalf("waiting space discoverable %v", items)
	}
}

// The owner approves the space; a maker then finds it without the locator.
func TestORPSpaceListedAndDiscovered(t *testing.T) {
	entry, r := trustHost(t)
	pubToken, publisher := joinParty(t, r)
	makerToken, maker := joinParty(t, r)

	putSpace(t, r, pubToken, publisher, "m_space02", "sp_news_1")
	listed, err := entry.DB.ListORPSpaceOwner(context.Background(), ownerOfParty(publisher), "sp_news_1")
	if err != nil || !listed {
		t.Fatalf("owner list %v %v", listed, err)
	}

	items := discoverSpaces(t, r, makerToken, maker, "m_spdcd2")
	if len(items) != 1 {
		t.Fatalf("items %v", items)
	}
	item, _ := items[0].(map[string]any)
	space, _ := item["space"].(map[string]any)
	if item["party"] != publisher || space["id"] != "sp_news_1" {
		t.Fatalf("item %v", item)
	}
	if _, hasLocator := space["locator"]; hasLocator {
		t.Fatal("locator must be stripped for another party")
	}
}

// A request lands 202 and the request message in the space owner's inbox.
func TestORPRequestDelivers(t *testing.T) {
	entry, r := trustHost(t)
	pubToken, publisher := joinParty(t, r)
	makerToken, maker := joinParty(t, r)

	putSpace(t, r, pubToken, publisher, "m_space03", "sp_news_1")
	if listed, _ := entry.DB.ListORPSpaceOwner(context.Background(), ownerOfParty(publisher), "sp_news_1"); !listed {
		t.Fatal("space not listed")
	}

	req := orpEnvelope("request", "m_requ01", maker, map[string]any{"space": "sp_news_1", "kind": "sponsored"})
	req["to"] = publisher
	rec, _ := postJSON(t, r, http.MethodPost, "/orp", req, bearerHeader(makerToken))
	if rec.Code != http.StatusAccepted {
		t.Fatalf("request %d %s", rec.Code, rec.Body.String())
	}

	messages, _, err := entry.DB.ListORPInbox(context.Background(), ownerOfParty(publisher), 0, 10)
	if err != nil {
		t.Fatal(err)
	}
	found := false
	for _, raw := range messages {
		var doc map[string]any
		if err := json.Unmarshal(raw, &doc); err == nil && doc["type"] == "request" && doc["from"] == maker {
			found = true
		}
	}
	if !found {
		t.Fatal("request not in owner inbox")
	}
}

// Withdrawing removes the space from discovery.
func TestORPSpaceWithdraw(t *testing.T) {
	entry, r := trustHost(t)
	pubToken, publisher := joinParty(t, r)
	makerToken, maker := joinParty(t, r)
	putSpace(t, r, pubToken, publisher, "m_space04", "sp_news_1")
	entry.DB.ListORPSpaceOwner(context.Background(), ownerOfParty(publisher), "sp_news_1")

	msg := orpEnvelope("space", "m_spacwd", publisher, map[string]any{"action": "withdraw", "id": "sp_news_1"})
	rec, body := postJSON(t, r, http.MethodPost, "/orp", msg, bearerHeader(pubToken))
	inner, _ := body["body"].(map[string]any)
	if rec.Code != http.StatusOK || inner["status"] != "withdrawn" {
		t.Fatalf("withdraw %d %v", rec.Code, body)
	}
	if items := discoverSpaces(t, r, makerToken, maker, "m_spdcd3"); len(items) != 0 {
		t.Fatalf("withdrawn space discoverable %v", items)
	}
}

// A standing offer under list:ask waits unlisted; the owner's list step
// (the same approval the /owner/offers form posts) flips it discoverable.
func TestORPOfferOwnerListApproval(t *testing.T) {
	entry, r := trustHost(t)
	makerToken, maker := joinParty(t, r)
	pubToken, publisher := joinParty(t, r)

	rec, body := postJSON(t, r, http.MethodPost, "/orp", orpEnvelope("offer", "m_oa_001", maker, standingOffer()), bearerHeader(makerToken))
	if rec.Code != http.StatusOK {
		t.Fatalf("offer %d %s", rec.Code, rec.Body.String())
	}
	inner, _ := body["body"].(map[string]any)
	waiting, _ := inner["waiting"].(map[string]any)
	offerID, _ := inner["id"].(string)
	if waiting["step"] != "list" || offerID == "" {
		t.Fatalf("offer reply %s", rec.Body.String())
	}

	miss, missBody := postJSON(t, r, http.MethodPost, "/orp", orpEnvelope("discover", "m_oa_002", publisher, map[string]any{
		"want": "offers", "query": "zqtestmention", "limit": 5,
	}), bearerHeader(pubToken))
	if miss.Code != http.StatusOK || missBody["body"].(map[string]any)["empty_reason"] != "no_match" {
		t.Fatalf("waiting offer discoverable %s", miss.Body.String())
	}

	listed, err := entry.DB.ListORPOfferOwner(context.Background(), ownerOfParty(maker), offerID)
	if err != nil || !listed {
		t.Fatalf("owner list %v %v", listed, err)
	}
	found, foundBody := postJSON(t, r, http.MethodPost, "/orp", orpEnvelope("discover", "m_oa_003", publisher, map[string]any{
		"want": "offers", "query": "zqtestmention", "limit": 5,
	}), bearerHeader(pubToken))
	if found.Code != http.StatusOK {
		t.Fatalf("discover %d %s", found.Code, found.Body.String())
	}
	items, _ := foundBody["body"].(map[string]any)["items"].([]any)
	if len(items) != 1 {
		t.Fatalf("items %s", found.Body.String())
	}
}

// A space whose locator page carries the party id earns a verified entry:
// challenge_post on most kinds, bio_token on a social.profile, and an X
// status locator is checked through the oEmbed endpoint. The domain method
// still wins for same-domain locators; a page without the party id leaves
// verified unset but the space still lists.
func TestORPSpaceVerification(t *testing.T) {
	db, cleanup := setupTestDB(t)
	t.Cleanup(cleanup)
	entry := &AgentEntry{
		DB:           db,
		BaseURL:      "http://127.0.0.1:8080",
		AddressKey:   testAddressKey,
		ClaimsCheck:  "off-local",
		TokenLimiter: NewIPRateLimiter(8, time.Minute),
	}
	var pages map[string]string
	var fetched []string
	host := &ORPHost{
		Entry: entry,
		FetchPage: func(ctx context.Context, rawURL string) (string, bool) {
			fetched = append(fetched, rawURL)
			body, ok := pages[rawURL]
			return body, ok
		},
	}
	r := mountORP(entry, host)
	pubToken, publisher := joinParty(t, r)
	statusEmbed := "https://publish.twitter.com/oembed?url=" + url.QueryEscape("https://x.com/indie/status/1234567890") + "&omit_script=true"
	pages = map[string]string{
		"https://indiestack.example/issues": "<html><body>Weekly by " + publisher + ".</body></html>",
		"https://other.example/page":        "<html><body>no party id here</body></html>",
		"https://links.example/indie":       "<html><body>Bio " + publisher + "</body></html>",
		statusEmbed:                         "<html><body>Post by " + publisher + "</body></html>",
	}
	verifiedMethod := func(t *testing.T, body map[string]any) string {
		t.Helper()
		inner, _ := body["body"].(map[string]any)
		space, _ := inner["space"].(map[string]any)
		verified, _ := space["verified"].([]any)
		if len(verified) != 1 {
			t.Fatalf("verified %v", verified)
		}
		method, _ := verified[0].(map[string]any)["method"].(string)
		return method
	}
	put := func(id, spaceID, kind, locator string) (*httptest.ResponseRecorder, map[string]any) {
		t.Helper()
		msg := orpEnvelope("space", id, publisher, map[string]any{
			"action": "put",
			"space": map[string]any{
				"id": spaceID, "kind": kind, "platform": "newsletter",
				"title": "Surface", "locator": locator,
			},
		})
		return postJSON(t, r, http.MethodPost, "/orp", msg, bearerHeader(pubToken))
	}

	rec, body := put("m_verif01", "sp_mark_1", "newsletter.issue", "https://indiestack.example/issues")
	if rec.Code != http.StatusOK || verifiedMethod(t, body) != "challenge_post" {
		t.Fatalf("challenge_post space %d %s", rec.Code, rec.Body.String())
	}
	if err := orp.ValidateAgainst(schemaPath(t, "messages/space.schema.json"), body); err != nil {
		t.Fatalf("space reply schema: %v", err)
	}

	rec, body = put("m_verif02", "sp_bio__1", "social.profile", "https://links.example/indie")
	if rec.Code != http.StatusOK || verifiedMethod(t, body) != "bio_token" {
		t.Fatalf("bio_token space %d %s", rec.Code, rec.Body.String())
	}
	if err := orp.ValidateAgainst(schemaPath(t, "messages/space.schema.json"), body); err != nil {
		t.Fatalf("bio_token reply schema: %v", err)
	}

	rec, body = put("m_verif03", "sp_xstat1", "social.post", "https://x.com/indie/status/1234567890")
	if rec.Code != http.StatusOK || verifiedMethod(t, body) != "challenge_post" {
		t.Fatalf("x status space %d %s", rec.Code, rec.Body.String())
	}
	if got := fetched[len(fetched)-1]; got != statusEmbed {
		t.Fatalf("x status fetched %q want %q", got, statusEmbed)
	}
	if err := orp.ValidateAgainst(schemaPath(t, "messages/space.schema.json"), body); err != nil {
		t.Fatalf("x status reply schema: %v", err)
	}

	before := len(fetched)
	rec, body = put("m_verif04", "sp_xprof1", "social.profile", "https://x.com/indie")
	inner, _ := body["body"].(map[string]any)
	space, _ := inner["space"].(map[string]any)
	if rec.Code != http.StatusOK || space["verified"] != nil || len(fetched) != before {
		t.Fatalf("x profile space %d %s fetched %v", rec.Code, rec.Body.String(), fetched)
	}

	rec, body = put("m_verif05", "sp_nomark", "web.article", "https://other.example/page")
	inner, _ = body["body"].(map[string]any)
	space, _ = inner["space"].(map[string]any)
	if rec.Code != http.StatusOK || space["verified"] != nil {
		t.Fatalf("unmarked space %d %s", rec.Code, rec.Body.String())
	}

	rec, body = put("m_verif06", "sp_down_1", "web.article", "https://down.example/x")
	inner, _ = body["body"].(map[string]any)
	space, _ = inner["space"].(map[string]any)
	if rec.Code != http.StatusOK || space["verified"] != nil {
		t.Fatalf("unreachable locator %d %s", rec.Code, rec.Body.String())
	}
}

// xStatusEmbedURL sends only status locators to the oEmbed endpoint. Other X
// URLs report the host but no fetch URL; non-X hosts report neither.
func TestXStatusEmbedURL(t *testing.T) {
	escape := func(raw string) string {
		return "https://publish.twitter.com/oembed?url=" + url.QueryEscape(raw) + "&omit_script=true"
	}
	for _, row := range []struct {
		name    string
		locator string
		embed   string
		xHost   bool
	}{
		{"x status", "https://x.com/indie/status/1234567890", escape("https://x.com/indie/status/1234567890"), true},
		{"x status query", "https://x.com/indie/status/123?s=20", escape("https://x.com/indie/status/123?s=20"), true},
		{"twitter status", "https://twitter.com/indie/status/42", escape("https://twitter.com/indie/status/42"), true},
		{"www twitter status", "https://www.twitter.com/indie/status/42", escape("https://www.twitter.com/indie/status/42"), true},
		{"mobile x status", "https://mobile.x.com/indie/status/42", escape("https://mobile.x.com/indie/status/42"), true},
		{"x profile", "https://x.com/indie", "", true},
		{"x home", "https://x.com/home", "", true},
		{"x status letters", "https://x.com/indie/status/abc", "", true},
		{"x status extra segment", "https://x.com/indie/status/42/photo/1", "", true},
		{"other host", "https://indiestack.example/issues", "", false},
		{"x lookalike", "https://x.com.evil.example/indie/status/42", "", false},
		{"not a url", "://bad", "", false},
	} {
		t.Run(row.name, func(t *testing.T) {
			embed, xHost := xStatusEmbedURL(row.locator)
			if embed != row.embed || xHost != row.xHost {
				t.Fatalf("xStatusEmbedURL(%q) = (%q, %v) want (%q, %v)", row.locator, embed, xHost, row.embed, row.xHost)
			}
		})
	}
}
