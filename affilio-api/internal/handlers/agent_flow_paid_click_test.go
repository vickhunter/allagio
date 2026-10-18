package handlers

import (
	"context"
	"net/http"
	"net/http/httptest"
	"net/url"
	"testing"
)

func TestAgentFlowPaysABrowserClick(t *testing.T) {
	db, cleanup := setupTestDB(t)
	defer cleanup()
	_, r := newD063Router(t, db)

	created, body := postJSON(t, r, http.MethodPost, "/v1/agent-accounts", map[string]any{
		"role": "publisher", "name": "Paid Click Blog", "email": "paid-click@example.com", "domain": "paid-click.example", "source": "cli",
	}, nil)
	if created.Code != http.StatusCreated || body["plugin_key"] == "" || body["api_key"] == "" {
		t.Fatalf("signup: %d %s", created.Code, created.Body.String())
	}
	siteID := body["site_id"].(string)
	siteKey := map[string]string{"X-API-Key": body["api_key"].(string)}
	pluginKey := map[string]string{"X-Plugin-Key": body["plugin_key"].(string)}

	_, offerID := seedListedOffer(t, db, "paid-click-maker", "paid-click-maker@example.com", "Northstar")
	if _, err := db.Pool.Exec(context.Background(), `UPDATE offers SET landing_url='https://example.com/buy' WHERE id=$1`, offerID); err != nil {
		t.Fatal(err)
	}
	joined, joinedBody := postJSON(t, r, http.MethodPost, "/v1/marketplace/offers/"+offerID+"/join", map[string]any{}, siteKey)
	if joined.Code != http.StatusOK || joinedBody["campaign_id"] == "" || joinedBody["tracked_link"] == "" {
		t.Fatalf("join: %d %s", joined.Code, joined.Body.String())
	}

	contextText := "Northstar devtools reviews every pull request. Northstar finds bugs before merge. This devtools note is about Northstar."
	found, foundBody := postJSON(t, r, http.MethodPost, "/v1/revenue/opportunities", map[string]any{"context": contextText}, siteKey)
	opportunities, _ := foundBody["opportunities"].([]any)
	if found.Code != http.StatusOK || foundBody["status"] != "matched" || len(opportunities) == 0 || opportunities[0].(map[string]any)["offer_id"] != offerID {
		t.Fatalf("opportunity: %d %s", found.Code, found.Body.String())
	}

	surface := "https://paid-click.example/posts/review"
	copy := "We use Northstar to catch bugs before merge."
	prepared, preparedBody := postJSON(t, r, http.MethodPost, "/v1/placements", map[string]any{
		"offer_id": offerID, "surface_url": surface, "context": contextText, "copy": copy, "post_id": 42, "post_title": "Review notes",
	}, siteKey)
	if prepared.Code != http.StatusCreated {
		t.Fatalf("placement: %d %s", prepared.Code, prepared.Body.String())
	}
	placement, _ := preparedBody["placement"].(map[string]any)
	placementID, _ := placement["id"].(string)
	disclosure, _ := placement["disclosure"].(string)
	if placementID == "" || disclosure == "" {
		t.Fatalf("placement body: %s", prepared.Body.String())
	}

	early, earlyBody := postJSON(t, r, http.MethodPost, "/v1/placements/"+placementID+"/plugin-ack", map[string]any{"action": "placed", "rel_sponsored": true}, pluginKey)
	if early.Code != http.StatusConflict || earlyBody["reason"] != "not_committed" {
		t.Fatalf("ack before approval: %d %s", early.Code, early.Body.String())
	}
	var wait, status string
	if err := db.Pool.QueryRow(context.Background(), `SELECT status, plugin_wait FROM placement_proposals WHERE id=$1`, placementID).Scan(&status, &wait); err != nil || status != "pending" || wait != "" {
		t.Fatalf("early ack wrote state %s %s %v", status, wait, err)
	}

	approved, approvedBody := postJSON(t, r, http.MethodPost, "/v1/offer-review/placements/"+placementID+"/approve", map[string]any{
		"site_id": siteID, "owner_email": "paid-click@example.com", "evidence": "Operator approval for the agent owner.",
	}, map[string]string{"X-Admin-Key": "test-admin"})
	if approved.Code != http.StatusOK || approvedBody["status"] != "approved" {
		t.Fatalf("approve: %d %s", approved.Code, approved.Body.String())
	}
	var kind, rule string
	if err := db.Pool.QueryRow(context.Background(), `SELECT approved_by_kind, COALESCE(matched_rule,'') FROM placement_proposals WHERE id=$1`, placementID).Scan(&kind, &rule); err != nil || kind != "operator" || rule != "" {
		t.Fatalf("approved_by %s rule %s %v", kind, rule, err)
	}

	committed, _ := postJSON(t, r, http.MethodPost, "/v1/placements/"+placementID+"/commit", map[string]any{}, siteKey)
	if committed.Code != http.StatusOK {
		t.Fatalf("commit: %d %s", committed.Code, committed.Body.String())
	}
	published := "<p>" + copy + ` <a href="https://example.com/v1/go/` + placementID + `">Northstar</a></p><p>` + disclosure + "</p>"
	receipt, _ := postJSON(t, r, http.MethodPost, "/v1/placements/"+placementID+"/receipt", map[string]any{
		"surface_url": surface, "published_text": published, "evidence": "Agent published the approved copy.", "post_title": "Review notes", "post_id": 42,
	}, siteKey)
	if receipt.Code != http.StatusOK {
		t.Fatalf("receipt: %d %s", receipt.Code, receipt.Body.String())
	}
	acked, ackBody := postJSON(t, r, http.MethodPost, "/v1/placements/"+placementID+"/plugin-ack", map[string]any{"action": "placed", "rel_sponsored": true}, pluginKey)
	if acked.Code != http.StatusOK || ackBody["plugin_state"] != "placed" {
		t.Fatalf("ack: %d %s", acked.Code, acked.Body.String())
	}

	live := hitGo(t, r, http.MethodGet, "/v1/go/"+placementID, testBrowserUA)
	parsed, err := url.Parse(live.Header().Get("Location"))
	if live.Code != http.StatusFound || err != nil || parsed.Query().Get("via") == "" {
		t.Fatalf("go: %d %s", live.Code, live.Header().Get("Location"))
	}
	var clickID, class string
	if err := db.Pool.QueryRow(context.Background(), `SELECT id::text, click_class FROM clicks WHERE placement_id=$1`, placementID).Scan(&clickID, &class); err != nil || clickID != parsed.Query().Get("via") || class != ClickClassBrowser {
		t.Fatalf("via %s click %s class %s err %v", parsed.Query().Get("via"), clickID, class, err)
	}

	digest, digestBody := postJSON(t, r, http.MethodGet, "/v1/digest", nil, siteKey)
	if digest.Code != http.StatusOK {
		t.Fatalf("digest: %d %s", digest.Code, digest.Body.String())
	}
	var item map[string]any
	for _, raw := range digestBody["items"].([]any) {
		row, _ := raw.(map[string]any)
		if row["id"] == placementID {
			item = row
		}
	}
	if item == nil || item["clicks"] != float64(1) || item["paid_clicks"] != float64(1) || item["approved_by"] != "operator" || item["live_now"] != true {
		t.Fatalf("digest item: %v", item)
	}
}

// TestPrematurePlacedStillPays reproduces the hosted row: plugin-ack stored
// placed before approval. Commit and the receipt leave that flag in place, so
// a later ack is nothing_to_ack. A browser hit still has to carry via and
// count as paid. An unknown client is stored once, without via, and is not paid.
func TestPrematurePlacedStillPays(t *testing.T) {
	db, cleanup := setupTestDB(t)
	defer cleanup()
	_, r := newD063Router(t, db)

	created, body := postJSON(t, r, http.MethodPost, "/v1/agent-accounts", map[string]any{
		"role": "publisher", "name": "Early Place Blog", "email": "early-place@example.com", "domain": "early-place.example", "source": "cli",
	}, nil)
	if created.Code != http.StatusCreated {
		t.Fatalf("signup: %d %s", created.Code, created.Body.String())
	}
	siteID := body["site_id"].(string)
	siteKey := map[string]string{"X-API-Key": body["api_key"].(string)}
	pluginKey := map[string]string{"X-Plugin-Key": body["plugin_key"].(string)}
	_, offerID := seedListedOffer(t, db, "early-place-maker", "early-place-maker@example.com", "Northstar")
	if _, err := db.Pool.Exec(context.Background(), `UPDATE offers SET landing_url='https://example.com/buy' WHERE id=$1`, offerID); err != nil {
		t.Fatal(err)
	}
	contextText := "Northstar devtools reviews every pull request. Northstar finds bugs before merge. This devtools note is about Northstar."
	surface := "https://early-place.example/posts/review"
	copy := "We use Northstar to catch bugs before merge."
	prepared, preparedBody := postJSON(t, r, http.MethodPost, "/v1/placements", map[string]any{
		"offer_id": offerID, "surface_url": surface, "context": contextText, "copy": copy, "post_id": 7, "post_title": "Review notes",
	}, siteKey)
	if prepared.Code != http.StatusCreated {
		t.Fatalf("placement: %d %s", prepared.Code, prepared.Body.String())
	}
	placementID := preparedBody["placement"].(map[string]any)["id"].(string)
	disclosure := preparedBody["placement"].(map[string]any)["disclosure"].(string)
	// 252aa64 stored placed from plugin-ack while the row was still pending.
	if _, err := db.Pool.Exec(context.Background(), `UPDATE placement_proposals SET plugin_wait='placed', plugin_wait_at=NOW() WHERE id=$1`, placementID); err != nil {
		t.Fatal(err)
	}
	approved, _ := postJSON(t, r, http.MethodPost, "/v1/offer-review/placements/"+placementID+"/approve", map[string]any{
		"site_id": siteID, "owner_email": "early-place@example.com", "evidence": "Operator approval for the agent owner.",
	}, map[string]string{"X-Admin-Key": "test-admin"})
	if approved.Code != http.StatusOK {
		t.Fatalf("approve: %d %s", approved.Code, approved.Body.String())
	}
	committed, _ := postJSON(t, r, http.MethodPost, "/v1/placements/"+placementID+"/commit", map[string]any{}, siteKey)
	if committed.Code != http.StatusOK {
		t.Fatalf("commit: %d %s", committed.Code, committed.Body.String())
	}
	published := "<p>" + copy + ` <a href="https://example.com/v1/go/` + placementID + `">Northstar</a></p><p>` + disclosure + "</p>"
	receipt, _ := postJSON(t, r, http.MethodPost, "/v1/placements/"+placementID+"/receipt", map[string]any{
		"surface_url": surface, "published_text": published, "evidence": "Agent published the approved copy.",
	}, siteKey)
	if receipt.Code != http.StatusOK {
		t.Fatalf("receipt: %d %s", receipt.Code, receipt.Body.String())
	}
	again, againBody := postJSON(t, r, http.MethodPost, "/v1/placements/"+placementID+"/plugin-ack", map[string]any{"action": "placed", "rel_sponsored": true}, pluginKey)
	if again.Code != http.StatusConflict || againBody["reason"] != "nothing_to_ack" {
		t.Fatalf("second ack: %d %s", again.Code, again.Body.String())
	}

	var clicks int
	count := func() int {
		t.Helper()
		if err := db.Pool.QueryRow(context.Background(), `SELECT COUNT(*) FROM clicks WHERE placement_id=$1`, placementID).Scan(&clicks); err != nil {
			t.Fatal(err)
		}
		return clicks
	}
	for i := 0; i < 4; i++ {
		hit := hitGo(t, r, http.MethodGet, "/v1/go/"+placementID, "curl/8.0")
		if hit.Code != http.StatusFound || hit.Header().Get("Location") != "https://example.com/buy" {
			t.Fatalf("unknown hit %d: %d %s", i, hit.Code, hit.Header().Get("Location"))
		}
	}
	if count() != 1 {
		t.Fatalf("unknown hits stored %d clicks", clicks)
	}
	var paid int
	if err := db.Pool.QueryRow(context.Background(), `SELECT COUNT(*) FROM clicks WHERE placement_id=$1 AND click_class='browser'`, placementID).Scan(&paid); err != nil || paid != 0 {
		t.Fatalf("unknown hits paid %d %v", paid, err)
	}

	live := hitGo(t, r, http.MethodGet, "/v1/go/"+placementID, testBrowserUA)
	parsed, err := url.Parse(live.Header().Get("Location"))
	if live.Code != http.StatusFound || err != nil || parsed.Host != "example.com" || parsed.Query().Get("via") == "" || count() != 2 {
		t.Fatalf("browser go: %d %s clicks %d", live.Code, live.Header().Get("Location"), clicks)
	}
	if err := db.Pool.QueryRow(context.Background(), `SELECT COUNT(*) FROM clicks WHERE placement_id=$1 AND click_class='browser'`, placementID).Scan(&paid); err != nil || paid != 1 {
		t.Fatalf("paid clicks %d %v", paid, err)
	}
}

// TestApprovedPlacementGoLinkPays is the hosted Site Keeper case. After
// approval, and before commit or plugin-ack, the tracked go link carried no
// via and paid_clicks stayed 0. The click writer required a stored campaign
// and status committed, so the redirect sent the reader to the plain URL.
func TestApprovedPlacementGoLinkPays(t *testing.T) {
	db, cleanup := setupTestDB(t)
	defer cleanup()
	_, r := newD063Router(t, db)

	created, body := postJSON(t, r, http.MethodPost, "/v1/agent-accounts", map[string]any{
		"role": "publisher", "name": "Approved Go Blog", "email": "approved-go@example.com", "domain": "approved-go.example", "source": "cli",
	}, nil)
	if created.Code != http.StatusCreated {
		t.Fatalf("signup: %d %s", created.Code, created.Body.String())
	}
	siteID := body["site_id"].(string)
	siteKey := map[string]string{"X-API-Key": body["api_key"].(string)}
	_, offerID := seedListedOffer(t, db, "approved-go-maker", "approved-go-maker@example.com", "Northstar")
	if _, err := db.Pool.Exec(context.Background(), `UPDATE offers SET landing_url='https://example.com/buy' WHERE id=$1`, offerID); err != nil {
		t.Fatal(err)
	}
	joined, joinedBody := postJSON(t, r, http.MethodPost, "/v1/marketplace/offers/"+offerID+"/join", map[string]any{}, siteKey)
	if joined.Code != http.StatusOK || joinedBody["campaign_id"] == "" {
		t.Fatalf("join: %d %s", joined.Code, joined.Body.String())
	}
	contextText := "Northstar devtools reviews every pull request. Northstar finds bugs before merge. This devtools note is about Northstar."
	prepared, preparedBody := postJSON(t, r, http.MethodPost, "/v1/placements", map[string]any{
		"offer_id": offerID, "surface_url": "https://approved-go.example/posts/review", "context": contextText,
		"copy": "We use Northstar to catch bugs before merge.", "post_id": 9, "post_title": "Review notes",
	}, siteKey)
	if prepared.Code != http.StatusCreated {
		t.Fatalf("placement: %d %s", prepared.Code, prepared.Body.String())
	}
	placementID := preparedBody["placement"].(map[string]any)["id"].(string)

	pending := hitGo(t, r, http.MethodGet, "/v1/go/"+placementID, testBrowserUA)
	if pending.Code != http.StatusFound || pending.Header().Get("Location") != "https://example.com/buy" {
		t.Fatalf("pending go: %d %s", pending.Code, pending.Header().Get("Location"))
	}
	var clicks int
	if err := db.Pool.QueryRow(context.Background(), `SELECT COUNT(*) FROM clicks WHERE placement_id=$1`, placementID).Scan(&clicks); err != nil || clicks != 0 {
		t.Fatalf("pending clicks %d %v", clicks, err)
	}

	approved, approvedBody := postJSON(t, r, http.MethodPost, "/v1/offer-review/placements/"+placementID+"/approve", map[string]any{
		"site_id": siteID, "owner_email": "approved-go@example.com", "evidence": "Operator approval for the agent owner.",
	}, map[string]string{"X-Admin-Key": "test-admin"})
	if approved.Code != http.StatusOK || approvedBody["status"] != "approved" {
		t.Fatalf("approve: %d %s", approved.Code, approved.Body.String())
	}
	var campaignMissing bool
	if err := db.Pool.QueryRow(context.Background(), `SELECT campaign_id IS NULL FROM placement_proposals WHERE id=$1 AND status='approved'`, placementID).Scan(&campaignMissing); err != nil || !campaignMissing {
		t.Fatalf("approval stored a campaign: null=%v %v", campaignMissing, err)
	}

	live := hitGo(t, r, http.MethodGet, "/v1/go/"+placementID, testBrowserUA)
	parsed, err := url.Parse(live.Header().Get("Location"))
	if live.Code != http.StatusFound || err != nil || parsed.Host != "example.com" || parsed.Query().Get("via") == "" {
		t.Fatalf("approved go: %d %s", live.Code, live.Header().Get("Location"))
	}
	var clickID, class string
	if err := db.Pool.QueryRow(context.Background(), `SELECT id::text, click_class FROM clicks WHERE placement_id=$1`, placementID).Scan(&clickID, &class); err != nil || clickID != parsed.Query().Get("via") || class != ClickClassBrowser {
		t.Fatalf("via %s click %s class %s err %v", parsed.Query().Get("via"), clickID, class, err)
	}

	digest, digestBody := postJSON(t, r, http.MethodGet, "/v1/digest", nil, siteKey)
	if digest.Code != http.StatusOK {
		t.Fatalf("digest: %d %s", digest.Code, digest.Body.String())
	}
	var item map[string]any
	for _, raw := range digestBody["items"].([]any) {
		row, _ := raw.(map[string]any)
		if row["id"] == placementID {
			item = row
		}
	}
	if item == nil || item["clicks"] != float64(1) || item["paid_clicks"] != float64(1) || item["live_now"] != false {
		t.Fatalf("digest item: %v", item)
	}
}

// TestTrailingSlashGoLinkPays keeps the paid click when a reader opens the
// placement link with one extra slash. RedirectTrailingSlash is off, so the
// unmatched route has to 301 /v1/go/<id>/ onto /v1/go/<id>. The slash itself
// records nothing. The followed request carries via and increments paid_clicks.
func TestTrailingSlashGoLinkPays(t *testing.T) {
	db, cleanup := setupTestDB(t)
	defer cleanup()
	_, r := newD063Router(t, db)

	created, body := postJSON(t, r, http.MethodPost, "/v1/agent-accounts", map[string]any{
		"role": "publisher", "name": "Slash Go Blog", "email": "slash-go@example.com", "domain": "slash-go.example", "source": "cli",
	}, nil)
	if created.Code != http.StatusCreated {
		t.Fatalf("signup: %d %s", created.Code, created.Body.String())
	}
	siteID := body["site_id"].(string)
	siteKey := map[string]string{"X-API-Key": body["api_key"].(string)}
	_, offerID := seedListedOffer(t, db, "slash-go-maker", "slash-go-maker@example.com", "Northstar")
	if _, err := db.Pool.Exec(context.Background(), `UPDATE offers SET landing_url='https://example.com/buy' WHERE id=$1`, offerID); err != nil {
		t.Fatal(err)
	}
	joined, joinedBody := postJSON(t, r, http.MethodPost, "/v1/marketplace/offers/"+offerID+"/join", map[string]any{}, siteKey)
	if joined.Code != http.StatusOK || joinedBody["campaign_id"] == "" {
		t.Fatalf("join: %d %s", joined.Code, joined.Body.String())
	}
	contextText := "Northstar devtools reviews every pull request. Northstar finds bugs before merge. This devtools note is about Northstar."
	prepared, preparedBody := postJSON(t, r, http.MethodPost, "/v1/placements", map[string]any{
		"offer_id": offerID, "surface_url": "https://slash-go.example/posts/review", "context": contextText,
		"copy": "We use Northstar to catch bugs before merge.", "post_id": 11, "post_title": "Review notes",
	}, siteKey)
	if prepared.Code != http.StatusCreated {
		t.Fatalf("placement: %d %s", prepared.Code, prepared.Body.String())
	}
	placementID := preparedBody["placement"].(map[string]any)["id"].(string)
	approved, approvedBody := postJSON(t, r, http.MethodPost, "/v1/offer-review/placements/"+placementID+"/approve", map[string]any{
		"site_id": siteID, "owner_email": "slash-go@example.com", "evidence": "Operator approval for the agent owner.",
	}, map[string]string{"X-Admin-Key": "test-admin"})
	if approved.Code != http.StatusOK || approvedBody["status"] != "approved" {
		t.Fatalf("approve: %d %s", approved.Code, approved.Body.String())
	}

	miss := httptest.NewRecorder()
	r.ServeHTTP(miss, httptest.NewRequest(http.MethodGet, "/v1/not-a-route/", nil))
	if miss.Code != http.StatusNotFound {
		t.Fatalf("unknown slash: %d %s", miss.Code, miss.Body.String())
	}

	slash := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/v1/go/"+placementID+"/", nil)
	req.Header.Set("User-Agent", testBrowserUA)
	r.ServeHTTP(slash, req)
	loc, err := url.Parse(slash.Header().Get("Location"))
	if slash.Code != http.StatusMovedPermanently || err != nil || loc.Path != "/v1/go/"+placementID {
		t.Fatalf("slash go: %d %s", slash.Code, slash.Header().Get("Location"))
	}
	var clicks int
	if err := db.Pool.QueryRow(context.Background(), `SELECT COUNT(*) FROM clicks WHERE placement_id=$1`, placementID).Scan(&clicks); err != nil || clicks != 0 {
		t.Fatalf("slash recorded a click: %d %v", clicks, err)
	}

	live := hitGo(t, r, http.MethodGet, loc.RequestURI(), testBrowserUA)
	parsed, err := url.Parse(live.Header().Get("Location"))
	if live.Code != http.StatusFound || err != nil || parsed.Host != "example.com" || parsed.Query().Get("via") == "" {
		t.Fatalf("followed go: %d %s", live.Code, live.Header().Get("Location"))
	}
	var clickID, class string
	if err := db.Pool.QueryRow(context.Background(), `SELECT id::text, click_class FROM clicks WHERE placement_id=$1`, placementID).Scan(&clickID, &class); err != nil || clickID != parsed.Query().Get("via") || class != ClickClassBrowser {
		t.Fatalf("via %s click %s class %s err %v", parsed.Query().Get("via"), clickID, class, err)
	}
	digest, digestBody := postJSON(t, r, http.MethodGet, "/v1/digest", nil, siteKey)
	if digest.Code != http.StatusOK {
		t.Fatalf("digest: %d %s", digest.Code, digest.Body.String())
	}
	var item map[string]any
	for _, raw := range digestBody["items"].([]any) {
		row, _ := raw.(map[string]any)
		if row["id"] == placementID {
			item = row
		}
	}
	if item == nil || item["paid_clicks"] != float64(1) {
		t.Fatalf("digest item: %v", item)
	}
}
