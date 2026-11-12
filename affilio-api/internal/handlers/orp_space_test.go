package handlers

import (
	"context"
	"encoding/json"
	"net/http"
	"testing"
	"time"
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

// A space whose locator page carries the party id earns verified:marker. The
// domain method still wins for same-domain locators; a page without the
// marker leaves verified unset but the space still lists.
func TestORPSpaceMarkerVerification(t *testing.T) {
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
	host := &ORPHost{
		Entry: entry,
		FetchPage: func(ctx context.Context, rawURL string) (string, bool) {
			body, ok := pages[rawURL]
			return body, ok
		},
	}
	r := mountORP(entry, host)
	pubToken, publisher := joinParty(t, r)
	pages = map[string]string{
		"https://indiestack.example/issues": "<html><body>Weekly by " + publisher + ".</body></html>",
		"https://other.example/page":        "<html><body>no marker here</body></html>",
	}

	msg := orpEnvelope("space", "m_marker1", publisher, map[string]any{
		"action": "put",
		"space": map[string]any{
			"id": "sp_mark_1", "kind": "newsletter.issue", "platform": "newsletter",
			"title": "Indie Stack Weekly", "locator": "https://indiestack.example/issues",
		},
	})
	rec, body := postJSON(t, r, http.MethodPost, "/orp", msg, bearerHeader(pubToken))
	inner, _ := body["body"].(map[string]any)
	space, _ := inner["space"].(map[string]any)
	verified, _ := space["verified"].([]any)
	if rec.Code != http.StatusOK || len(verified) != 1 || verified[0].(map[string]any)["method"] != "marker" {
		t.Fatalf("marker space %d %s", rec.Code, rec.Body.String())
	}

	msg = orpEnvelope("space", "m_marker2", publisher, map[string]any{
		"action": "put",
		"space": map[string]any{
			"id": "sp_nomark", "kind": "blog.post", "platform": "blog",
			"title": "No marker", "locator": "https://other.example/page",
		},
	})
	rec, body = postJSON(t, r, http.MethodPost, "/orp", msg, bearerHeader(pubToken))
	inner, _ = body["body"].(map[string]any)
	space, _ = inner["space"].(map[string]any)
	if rec.Code != http.StatusOK || space["verified"] != nil {
		t.Fatalf("unmarked space %d %s", rec.Code, rec.Body.String())
	}

	msg = orpEnvelope("space", "m_marker3", publisher, map[string]any{
		"action": "put",
		"space": map[string]any{
			"id": "sp_down", "kind": "blog.post", "platform": "blog",
			"title": "Down page", "locator": "https://down.example/x",
		},
	})
	rec, body = postJSON(t, r, http.MethodPost, "/orp", msg, bearerHeader(pubToken))
	inner, _ = body["body"].(map[string]any)
	space, _ = inner["space"].(map[string]any)
	if rec.Code != http.StatusOK || space["verified"] != nil {
		t.Fatalf("unreachable locator %d %s", rec.Code, rec.Body.String())
	}
}
