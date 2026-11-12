package handlers

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"affilio-api/internal/database"
)

func TestDemoPagesArePublicWithoutData(t *testing.T) {
	r := ginEngine()
	h := &HostedHandler{BaseURL: "http://127.0.0.1"}
	h.Mount(r)
	redirects := &RedirectsHandler{HomeURL: "https://offerrails.example/"}
	r.GET("/v1/go/:code", redirects.Go)

	for _, path := range []string{"/demo/operator", "/demo/digest", "/demo/post"} {
		rec := httptest.NewRecorder()
		r.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, path, nil))
		if rec.Code != http.StatusOK {
			t.Fatalf("%s: %d %s", path, rec.Code, rec.Body.String())
		}
		if rec.Header().Get("X-Robots-Tag") != "noindex" {
			t.Fatalf("%s robots %q", path, rec.Header().Get("X-Robots-Tag"))
		}
	}

	operator := httptest.NewRecorder()
	r.ServeHTTP(operator, httptest.NewRequest(http.MethodGet, "/demo/operator", nil))
	opBody := operator.Body.String()
	if !strings.Contains(opBody, "Pending placement") || !strings.Contains(opBody, "approved_by operator") || !strings.Contains(opBody, `action="/demo/operator"`) {
		t.Fatalf("operator sample: %s", opBody)
	}

	digest := httptest.NewRecorder()
	r.ServeHTTP(digest, httptest.NewRequest(http.MethodGet, "/demo/digest", nil))
	digestBody := digest.Body.String()
	if !strings.Contains(digestBody, "clicks") || !strings.Contains(digestBody, "paid_clicks") || !strings.Contains(digestBody, "approved_by operator") || !strings.Contains(digestBody, demoSampleDomain) {
		t.Fatalf("digest sample: %s", digestBody)
	}

	post := httptest.NewRecorder()
	r.ServeHTTP(post, httptest.NewRequest(http.MethodGet, "/demo/post", nil))
	if !strings.Contains(post.Body.String(), `href="/v1/go/demo"`) || !strings.Contains(post.Body.String(), "Disclosure:") {
		t.Fatalf("demo post: %s", post.Body.String())
	}

	posted := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/demo/operator", strings.NewReader("placement_id=not-a-placement"))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	r.ServeHTTP(posted, req)
	if posted.Code != http.StatusOK || !strings.Contains(posted.Body.String(), "Nothing was approved.") {
		t.Fatalf("demo approve: %d %s", posted.Code, posted.Body.String())
	}

	gone := httptest.NewRecorder()
	goReq := httptest.NewRequest(http.MethodGet, "/v1/go/demo", nil)
	goReq.RemoteAddr = "192.0.2.40:4000"
	goReq.Header.Set("User-Agent", testBrowserUA)
	r.ServeHTTP(gone, goReq)
	if gone.Code != http.StatusFound || gone.Header().Get("Location") != "https://offerrails.example/" {
		t.Fatalf("sample go link: %d %s", gone.Code, gone.Header().Get("Location"))
	}
}

func TestDemoMigrationReruns(t *testing.T) {
	db, cleanup := setupTestDB(t)
	defer cleanup()
	raw, err := os.ReadFile(filepath.Join("..", "migrations", "043_demo_test_clicks.sql"))
	if err != nil {
		t.Fatal(err)
	}
	if _, err = db.Pool.Exec(context.Background(), string(raw)); err != nil {
		t.Fatal(err)
	}
	if _, err = db.Pool.Exec(context.Background(), string(raw)); err != nil {
		t.Fatal(err)
	}
}

func TestDemoOperatorDigestAndUnbilledGoLink(t *testing.T) {
	db, cleanup := setupTestDB(t)
	defer cleanup()
	_, r := newD063Router(t, db)

	demoSite := createTestSite(t, db, "demo-d084a-key", "demo-d084a.staging.invalid", "Demo site")
	realSite := createTestSite(t, db, "real-d084a-key", "real-d084a.example", "Real site")
	if _, err := db.Pool.Exec(context.Background(), `UPDATE sites SET demo=true WHERE id=$1`, demoSite); err != nil {
		t.Fatal(err)
	}
	if _, err := db.Pool.Exec(context.Background(), `UPDATE agent_step_modes SET payout='auto' WHERE site_id=$1::uuid`, realSite); err != nil {
		t.Fatal(err)
	}
	_, offerID := seedListedOffer(t, db, "d084a-maker", "d084a-maker@example.com", "Northstar")
	if _, err := db.Pool.Exec(context.Background(), `UPDATE offers SET landing_url='https://example.com/buy', status='listed' WHERE id=$1`, offerID); err != nil {
		t.Fatal(err)
	}
	makerID := makerForOffer(t, db, offerID)

	demoPending := insertPlacement(t, db, demoSite, offerID, "https://demo-d084a.staging.invalid/pending", "DEMO-PENDING-COPY")
	demoApproved := approveDemoPlacement(t, db, demoSite, offerID, "https://demo-d084a.staging.invalid/approved", "DEMO-APPROVED-COPY", "operator", "d084a-demo-camp")
	realPending := insertPlacement(t, db, realSite, offerID, "https://real-d084a.example/pending", "REAL-PENDING-COPY")
	realApproved := approveDemoPlacement(t, db, realSite, offerID, "https://real-d084a.example/approved", "REAL-APPROVED-COPY", "owner", "d084a-real-camp")

	operator := httptest.NewRecorder()
	r.ServeHTTP(operator, httptest.NewRequest(http.MethodGet, "/demo/operator", nil))
	if operator.Code != http.StatusOK {
		t.Fatalf("operator: %d %s", operator.Code, operator.Body.String())
	}
	opBody := operator.Body.String()
	for _, want := range []string{"DEMO-PENDING-COPY", "DEMO-APPROVED-COPY", "approved_by operator", "demo-d084a.staging.invalid", `action="/demo/operator"`} {
		if !strings.Contains(opBody, want) {
			t.Fatalf("operator missing %s: %s", want, opBody)
		}
	}
	for _, secret := range []string{"REAL-PENDING-COPY", "REAL-APPROVED-COPY", "real-d084a.example"} {
		if strings.Contains(opBody, secret) {
			t.Fatalf("operator exposed %s", secret)
		}
	}

	form := strings.NewReader("placement_id=" + realPending + "&placement_id=" + demoPending)
	postReq := httptest.NewRequest(http.MethodPost, "/demo/operator", form)
	postReq.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	posted := httptest.NewRecorder()
	r.ServeHTTP(posted, postReq)
	if posted.Code != http.StatusOK || !strings.Contains(posted.Body.String(), "Nothing was approved.") {
		t.Fatalf("demo post approve: %d %s", posted.Code, posted.Body.String())
	}
	assertPlacementStatus(t, db, realPending, "pending")
	assertPlacementStatus(t, db, demoPending, "pending")

	sneak := httptest.NewRecorder()
	sneakReq := httptest.NewRequest(http.MethodPost, "/v1/offer-review/placements/"+realPending+"/approve", strings.NewReader(`{"site_id":"`+realSite+`","owner_email":"owner@example.com","evidence":"trying the real route"}`))
	sneakReq.Header.Set("Content-Type", "application/json")
	r.ServeHTTP(sneak, sneakReq)
	if sneak.Code == http.StatusOK {
		t.Fatalf("approve without admin key: %d %s", sneak.Code, sneak.Body.String())
	}
	assertPlacementStatus(t, db, realPending, "pending")

	ownerSneak := httptest.NewRecorder()
	r.ServeHTTP(ownerSneak, httptest.NewRequest(http.MethodPost, "/owner/placements/"+realPending+"/approve", nil))
	if ownerSneak.Code == http.StatusOK {
		t.Fatalf("owner approve without a session: %d", ownerSneak.Code)
	}
	assertPlacementStatus(t, db, realPending, "pending")

	digest := httptest.NewRecorder()
	r.ServeHTTP(digest, httptest.NewRequest(http.MethodGet, "/demo/digest", nil))
	if digest.Code != http.StatusOK {
		t.Fatalf("digest: %d %s", digest.Code, digest.Body.String())
	}
	digestBody := digest.Body.String()
	card := articleAround(digestBody, "DEMO-APPROVED-COPY")
	if card == "" || !strings.Contains(card, "demo-d084a.staging.invalid") || !strings.Contains(card, "approved_by operator") {
		t.Fatalf("digest card: %s", digestBody)
	}
	for _, secret := range []string{"REAL-PENDING-COPY", "REAL-APPROVED-COPY", "real-d084a.example"} {
		if strings.Contains(digestBody, secret) {
			t.Fatalf("digest exposed %s", secret)
		}
	}

	postPage := httptest.NewRecorder()
	r.ServeHTTP(postPage, httptest.NewRequest(http.MethodGet, "/demo/post", nil))
	if postPage.Code != http.StatusOK || !strings.Contains(postPage.Body.String(), `/v1/go/`+demoApproved) || !strings.Contains(postPage.Body.String(), "Disclosure:") {
		t.Fatalf("post: %d %s", postPage.Code, postPage.Body.String())
	}

	demoHit := goBrowser(r, "/v1/go/"+demoApproved, "192.0.2.41:4100")
	if demoHit.Code != http.StatusFound {
		t.Fatalf("demo go: %d %s", demoHit.Code, demoHit.Body.String())
	}
	demoLoc, err := url.Parse(demoHit.Header().Get("Location"))
	if err != nil || demoLoc.String() != "https://example.com/buy" || demoLoc.RawQuery != "" {
		t.Fatalf("demo go location %s", demoHit.Header().Get("Location"))
	}
	demoClass := clickClassFor(t, db, demoApproved)
	if demoClass != ClickClassTest {
		t.Fatalf("demo click class %s", demoClass)
	}
	if countRows(t, db, `SELECT COUNT(*) FROM clicks WHERE site_id=$1 AND click_class='browser'`, demoSite) != 0 {
		t.Fatal("demo site recorded a paid click")
	}

	realHit := goBrowser(r, "/v1/go/"+realApproved, "192.0.2.42:4200")
	if realHit.Code != http.StatusFound || !strings.Contains(realHit.Header().Get("Location"), "via=") {
		t.Fatalf("real go: %d %s", realHit.Code, realHit.Header().Get("Location"))
	}
	if clickClassFor(t, db, realApproved) != ClickClassBrowser {
		t.Fatalf("real click class %s", clickClassFor(t, db, realApproved))
	}
	if countRows(t, db, `SELECT COUNT(*) FROM clicks WHERE site_id=$1 AND click_class='browser'`, realSite) != 1 {
		t.Fatal("real paid click was not recorded")
	}

	digestAgain := httptest.NewRecorder()
	r.ServeHTTP(digestAgain, httptest.NewRequest(http.MethodGet, "/demo/digest", nil))
	approvedCard := articleAround(digestAgain.Body.String(), "DEMO-APPROVED-COPY")
	if !strings.Contains(approvedCard, `<span data-clicks>1</span>`) || !strings.Contains(approvedCard, `<span data-paid-clicks>0</span>`) || !strings.Contains(approvedCard, "approved_by operator") {
		t.Fatalf("digest after click: %s", approvedCard)
	}

	floor := time.Unix(0, 0).UTC()
	realItems, err := db.DigestPlacements(context.Background(), realSite, floor)
	if err != nil {
		t.Fatal(err)
	}
	realItem := digestItemByID(realItems, realApproved)
	if realItem == nil || realItem.Clicks != 1 || realItem.PaidClicks != 1 {
		t.Fatalf("real digest totals %+v", realItem)
	}
	demoItems, err := db.DigestPlacements(context.Background(), demoSite, floor)
	if err != nil {
		t.Fatal(err)
	}
	demoItem := digestItemByID(demoItems, demoApproved)
	if demoItem == nil || demoItem.Clicks != 0 || demoItem.PaidClicks != 0 || demoItem.ApprovedBy != "operator" {
		t.Fatalf("demo digest totals %+v", demoItem)
	}
	realSummary, err := db.GetClickSummary(context.Background(), realSite, 0)
	if err != nil || realSummary.PaidClicks != 1 {
		t.Fatalf("real summary %+v %v", realSummary, err)
	}
	demoSummary, err := db.GetClickSummary(context.Background(), demoSite, 0)
	if err != nil || demoSummary.PaidClicks != 0 || demoSummary.TotalClicks != 0 {
		t.Fatalf("demo summary %+v %v", demoSummary, err)
	}

	var demoClickID string
	if err = db.Pool.QueryRow(context.Background(), `SELECT id::text FROM clicks WHERE placement_id=$1`, demoApproved).Scan(&demoClickID); err != nil {
		t.Fatal(err)
	}
	_, err = db.RecordSaleConversion(context.Background(), demoClickID, "stripe", "evt-d084a-demo", "obj-d084a-demo", "sale", 20, "USD", time.Now().UTC())
	if !errors.Is(err, database.ErrClickNotFound) {
		t.Fatalf("test click sale: %v", err)
	}
	if countRows(t, db, `SELECT COUNT(*) FROM conversions WHERE click_id=$1`, demoClickID) != 0 {
		t.Fatal("test click was billed")
	}

	insertClearedLedger(t, db, realSite, offerID, "d084a-real-live")
	insertClearedLedger(t, db, demoSite, offerID, "d084a-demo-live")
	var testClickID, testConversion string
	if err = db.Pool.QueryRow(context.Background(), `INSERT INTO clicks(site_id, click_class) VALUES($1,'test') RETURNING id::text`, realSite).Scan(&testClickID); err != nil {
		t.Fatal(err)
	}
	if err = db.Pool.QueryRow(context.Background(), `INSERT INTO conversions(offer_id, provider, provider_event_id, kind, amount, currency, commission_amount, occurred_at, payment_mode, click_id)
VALUES($1,'stripe','d084a-real-test','sale',200,'USD',40,NOW(),'live',$2) RETURNING id`, offerID, testClickID).Scan(&testConversion); err != nil {
		t.Fatal(err)
	}
	if _, err = db.Pool.Exec(context.Background(), `INSERT INTO ledger_entries(site_id, conversion_id, kind, amount, currency, status) VALUES($1,$2,'commission',40,'USD','cleared')`, realSite, testConversion); err != nil {
		t.Fatal(err)
	}

	if _, paid, err := db.FulfillBatchPayout(context.Background(), demoSite, "USD", makerID, "live"); err != nil || paid {
		t.Fatalf("demo fulfill paid=%v err=%v", paid, err)
	}
	payouts, err := db.RunPayoutBatch(context.Background(), 10, true)
	if err != nil {
		t.Fatal(err)
	}
	var realAmount float64
	var sawReal, sawDemo bool
	for _, payout := range payouts {
		if payout.SiteID == demoSite {
			sawDemo = true
		}
		if payout.SiteID == realSite {
			sawReal = true
			realAmount = payout.Amount
		}
	}
	if sawDemo || !sawReal || realAmount != 40 {
		t.Fatalf("payouts demo=%v real=%v amount=%v rows=%d", sawDemo, sawReal, realAmount, len(payouts))
	}
	if countRows(t, db, `SELECT COUNT(*) FROM ledger_entries WHERE conversion_id=$1 AND payout_id IS NULL`, testConversion) != 1 {
		t.Fatal("test click ledger was paid")
	}
	if countRows(t, db, `SELECT COUNT(*) FROM payouts WHERE site_id=$1`, demoSite) != 0 {
		t.Fatal("demo site was paid")
	}
}

func approveDemoPlacement(t *testing.T, db *database.DB, siteID, offerID, surface, copy, approvedBy, code string) string {
	t.Helper()
	id := insertPlacement(t, db, siteID, offerID, surface, copy)
	if _, err := db.Pool.Exec(context.Background(), `INSERT INTO campaigns(offer_id, site_id, tracking_code, requires_placement, status) VALUES($1,$2,$3,true,'active')`, offerID, siteID, code); err != nil {
		t.Fatal(err)
	}
	if _, err := db.Pool.Exec(context.Background(), `UPDATE placement_proposals
SET status='approved', approved_by_kind=$2, approved_at=NOW(), expires_at=NOW()+INTERVAL '2 days'
WHERE id=$1`, id, approvedBy); err != nil {
		t.Fatal(err)
	}
	return id
}

func makerForOffer(t *testing.T, db *database.DB, offerID string) string {
	t.Helper()
	var id string
	if err := db.Pool.QueryRow(context.Background(), `SELECT p.maker_id::text FROM offers o JOIN products p ON p.id=o.product_id WHERE o.id=$1`, offerID).Scan(&id); err != nil {
		t.Fatal(err)
	}
	return id
}

func assertPlacementStatus(t *testing.T, db *database.DB, id, want string) {
	t.Helper()
	var status string
	if err := db.Pool.QueryRow(context.Background(), `SELECT status FROM placement_proposals WHERE id=$1`, id).Scan(&status); err != nil || status != want {
		t.Fatalf("placement %s status %s err %v", id, status, err)
	}
}

func clickClassFor(t *testing.T, db *database.DB, placementID string) string {
	t.Helper()
	var class string
	if err := db.Pool.QueryRow(context.Background(), `SELECT click_class FROM clicks WHERE placement_id=$1`, placementID).Scan(&class); err != nil {
		t.Fatal(err)
	}
	return class
}

func goBrowser(r http.Handler, path, remote string) *httptest.ResponseRecorder {
	req := httptest.NewRequest(http.MethodGet, path, nil)
	req.RemoteAddr = remote
	req.Header.Set("User-Agent", testBrowserUA)
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, req)
	return rec
}

func articleAround(body, marker string) string {
	i := strings.Index(body, marker)
	if i < 0 {
		return ""
	}
	start := strings.LastIndex(body[:i], "<article")
	if start < 0 {
		start = i
	}
	rest := body[start:]
	end := strings.Index(rest, "</article>")
	if end < 0 {
		return rest
	}
	return rest[:end]
}

func digestItemByID(items []database.DigestItem, id string) *database.DigestItem {
	for i := range items {
		if items[i].ID == id {
			return &items[i]
		}
	}
	return nil
}
