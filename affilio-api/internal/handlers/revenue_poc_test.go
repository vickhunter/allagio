package handlers

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"

	"affilio-api/internal/database"
	"affilio-api/internal/middleware"
	"github.com/gin-gonic/gin"
)

func newRevenueRouter(db *database.DB) *gin.Engine {
	r := newPayoutsRouter(db)
	h := &PlacementsHandler{DB: db}
	r.POST("/v1/revenue/preview", h.Find)
	site := r.Group("/v1")
	site.Use(middleware.APIKeyAuth(db, nil))
	site.POST("/revenue/opportunities", h.Find)
	site.POST("/placements", h.Prepare)
	site.GET("/placements", h.List)
	site.GET("/placements/:id", h.Get)
	site.POST("/placements/:id/commit", h.Commit)
	site.POST("/placements/:id/receipt", h.RecordReceipt)
	site.POST("/placements/:id/revoke", h.Revoke)
	admin := r.Group("/v1/offer-review")
	admin.Use(middleware.AdminKeyAuth("admin-secret", nil))
	admin.POST("/placements/:id/approve", h.Approve)
	admin.POST("/agent-access/sites/:site_id", h.CreateAgentAccess)
	admin.POST("/agent-access/:id/revoke", h.RevokeAgentAccess)
	connections := &PaymentConnectionsHandler{DB: db, Cipher: testConnectionCipher()}
	admin.POST("/products/:id/connection", connections.Create)
	return r
}

func testPaidCampaign(t *testing.T) (*database.DB, *gin.Engine, string, string, string) {
	t.Helper()
	db, cleanup := setupTestDB(t)
	t.Cleanup(cleanup)
	_, makerKey := createTestMaker(t, db)
	r := newRevenueRouter(db)
	offerID := listedOffer(t, db, r, makerKey)
	siteKey := "site-key-revenue-test"
	createTestSite(t, db, siteKey, "publisher.example.com", "Developer tutorials")
	_, joined := doSiteJSON(t, r, http.MethodPost, "/v1/marketplace/offers/"+offerID+"/join", siteKey, nil)
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/v1/go/"+joined["tracking_code"].(string), nil))
	u, err := url.Parse(rec.Header().Get("Location"))
	if err != nil || rec.Code != http.StatusFound {
		t.Fatalf("click failed: %d %v", rec.Code, err)
	}
	return db, r, siteKey, offerID, u.Query().Get("via")
}

func TestStripePaymentGatesAndBusinessDedupe(t *testing.T) {
	db, r, _, _, clickID := testPaidCampaign(t)
	unpaid := stripeSalePayload("evt_unpaid", "pi_gate", clickID, 10000)
	unpaid["data"].(map[string]interface{})["object"].(map[string]interface{})["payment_status"] = "unpaid"
	rec, body := sendStripeEvent(t, db, r, unpaid)
	if rec.Code != 200 || body["status"] != "ignored" {
		t.Fatalf("unpaid was not ignored: %d %v", rec.Code, body)
	}
	paid := stripeSalePayload("evt_async", "pi_gate", clickID, 10000)
	paid["type"] = "checkout.session.async_payment_succeeded"
	rec, body = sendStripeEvent(t, db, r, paid)
	if rec.Code != 200 || body["status"] != "recorded" {
		t.Fatalf("async failed: %d %v", rec.Code, body)
	}
	rec, body = sendStripeEvent(t, db, r, stripeSalePayload("evt_completed_other", "pi_gate", clickID, 10000))
	if rec.Code != 200 || body["status"] != "duplicate" {
		t.Fatalf("payment counted twice: %d %v", rec.Code, body)
	}
	wrongMode := stripeSalePayload("evt_wrong_livemode", "pi_wrong_livemode", clickID, 10000)
	wrongMode["livemode"] = true
	rec, _ = sendStripeEvent(t, db, r, wrongMode)
	if rec.Code != 400 {
		t.Fatalf("accepted wrong livemode: %d", rec.Code)
	}
	wrongProduct := stripeSalePayload("evt_wrong_product", "pi_wrong_product", clickID, 10000)
	wrongProduct["data"].(map[string]interface{})["object"].(map[string]interface{})["metadata"] = map[string]interface{}{"dtr_product_id": "other-product"}
	rec, body = sendStripeEvent(t, db, r, wrongProduct)
	if rec.Code != 200 || body["status"] != "ignored" {
		t.Fatalf("wrong product was not ignored: %d %v", rec.Code, body)
	}
	var count int
	if err := db.Pool.QueryRow(context.Background(), `SELECT COUNT(*) FROM conversions WHERE kind='sale'`).Scan(&count); err != nil || count != 1 {
		t.Fatalf("sales=%d err=%v", count, err)
	}
}

func refundPayload(id, pi string, amount int) map[string]interface{} {
	return map[string]interface{}{"id": id, "type": "charge.refunded", "created": time.Now().Unix(),
		"data": map[string]interface{}{"object": map[string]interface{}{"id": "ch_" + pi, "payment_intent": pi, "amount_refunded": amount, "currency": "usd"}}}
}

func TestRefundBeforeSaleAndRepeatedPartialRefunds(t *testing.T) {
	db, r, siteKey, _, clickID := testPaidCampaign(t)
	rec, body := sendStripeEvent(t, db, r, refundPayload("evt_early", "pi_refund", 2500))
	if rec.Code != 202 || body["status"] != "pending" {
		t.Fatalf("early refund lost: %d %v", rec.Code, body)
	}
	sendStripeEvent(t, db, r, stripeSalePayload("evt_sale", "pi_refund", clickID, 10000))
	if _, err := db.Pool.Exec(context.Background(), `UPDATE webhook_events SET next_attempt_at=NOW()`); err != nil {
		t.Fatal(err)
	}
	h := &WebhooksHandler{DB: db, Cipher: testConnectionCipher()}
	if err := h.ReconcilePending(context.Background()); err != nil {
		t.Fatal(err)
	}
	_, balance := doSiteJSON(t, r, http.MethodGet, "/v1/payouts/balance", siteKey, nil)
	usd := balance["balances"].([]interface{})[0].(map[string]interface{})
	if usd["pending"] != float64(15) {
		t.Fatalf("partial refund balance: %v", usd)
	}
	rec, body = sendStripeEvent(t, db, r, refundPayload("evt_same_amount_other_id", "pi_refund", 2500))
	if rec.Code != 200 || body["status"] != "duplicate" {
		t.Fatalf("repeated cumulative refund counted: %v", body)
	}
	sendStripeEvent(t, db, r, refundPayload("evt_later_partial", "pi_refund", 6000))
	_, balance = doSiteJSON(t, r, http.MethodGet, "/v1/payouts/balance", siteKey, nil)
	usd = balance["balances"].([]interface{})[0].(map[string]interface{})
	if usd["pending"] != float64(8) {
		t.Fatalf("second partial refund balance: %v", usd)
	}
	sendStripeEvent(t, db, r, refundPayload("evt_full", "pi_refund", 10000))
	_, balance = doSiteJSON(t, r, http.MethodGet, "/v1/payouts/balance", siteKey, nil)
	usd = balance["balances"].([]interface{})[0].(map[string]interface{})
	if usd["pending"] != float64(0) {
		t.Fatalf("full refund balance: %v", usd)
	}
}

func TestPaymentRecoveryAfterBookingBeforeAcknowledgement(t *testing.T) {
	db, r, _, _, clickID := testPaidCampaign(t)
	sendStripeEvent(t, db, r, stripeSalePayload("evt_crash", "pi_crash", clickID, 10000))
	if _, err := db.Pool.Exec(context.Background(), `UPDATE webhook_events SET processed_at=NULL,next_attempt_at=NOW() WHERE provider_event_id='evt_crash'`); err != nil {
		t.Fatal(err)
	}
	if err := (&WebhooksHandler{DB: db}).ReconcilePending(context.Background()); err != nil {
		t.Fatal(err)
	}
	var count int
	var processed bool
	if err := db.Pool.QueryRow(context.Background(), `SELECT COUNT(*) FROM ledger_entries`).Scan(&count); err != nil || count != 1 {
		t.Fatalf("duplicate commission: %d %v", count, err)
	}
	if err := db.Pool.QueryRow(context.Background(), `SELECT processed_at IS NOT NULL FROM webhook_events WHERE provider_event_id='evt_crash'`).Scan(&processed); err != nil || !processed {
		t.Fatalf("pending event not recovered: %v %v", processed, err)
	}
}

func TestConcurrentPayoutsAndRefundCancellation(t *testing.T) {
	db, r, siteKey, _, clickID := testPaidCampaign(t)
	sendStripeEvent(t, db, r, stripeSalePayload("evt_batch", "pi_batch", clickID, 20000))
	if _, err := db.Pool.Exec(context.Background(), `UPDATE ledger_entries SET clears_at=NOW()-INTERVAL '1 day'`); err != nil {
		t.Fatal(err)
	}
	var wg sync.WaitGroup
	errors := make(chan error, 2)
	for i := 0; i < 2; i++ {
		wg.Add(1)
		go func() { defer wg.Done(); _, err := db.RunPayoutBatch(context.Background(), 25, true); errors <- err }()
	}
	wg.Wait()
	close(errors)
	for err := range errors {
		if err != nil {
			t.Fatal(err)
		}
	}
	var count int
	if err := db.Pool.QueryRow(context.Background(), `SELECT COUNT(*) FROM payouts WHERE status='pending'`).Scan(&count); err != nil || count != 1 {
		t.Fatalf("concurrent payouts=%d err=%v", count, err)
	}
	_, balance := doSiteJSON(t, r, http.MethodGet, "/v1/payouts/balance", siteKey, nil)
	usd := balance["balances"].([]interface{})[0].(map[string]interface{})
	if usd["paid"] != float64(0) || usd["reserved"] != float64(40) {
		t.Fatalf("reserved shown paid: %v", usd)
	}
	sendStripeEvent(t, db, r, refundPayload("evt_batch_refund", "pi_batch", 10000))
	_, balance = doSiteJSON(t, r, http.MethodGet, "/v1/payouts/balance", siteKey, nil)
	usd = balance["balances"].([]interface{})[0].(map[string]interface{})
	if usd["reserved"] != float64(0) || usd["cleared_unpaid"] != float64(20) {
		t.Fatalf("pending batch not cancelled after refund: %v", usd)
	}
}

func TestTestPayoutIsolationAndRefundAfterConfirmedReceipt(t *testing.T) {
	db, r, siteKey, _, clickID := testPaidCampaign(t)
	sendStripeEvent(t, db, r, stripeSalePayload("evt_late", "pi_late", clickID, 20000))
	if _, err := db.Pool.Exec(context.Background(), `UPDATE ledger_entries SET clears_at=NOW()-INTERVAL '1 day'`); err != nil {
		t.Fatal(err)
	}
	liveBatch, err := db.RunPayoutBatch(context.Background(), 25, false)
	if err != nil || len(liveBatch) != 0 {
		t.Fatalf("test commissions entered ordinary payout: %v %v", liveBatch, err)
	}
	testBatch, err := db.RunPayoutBatch(context.Background(), 25, true)
	if err != nil || len(testBatch) != 1 || testBatch[0].Mode != "test" {
		t.Fatalf("test batch mode: %v %v", testBatch, err)
	}
	payout := testBatch[0]
	if marked, err := db.MarkPayoutPaid(context.Background(), payout.ID, "SIMULATED-BANK-REFERENCE"); err != nil || !marked {
		t.Fatalf("mark fixture payment: %v %v", marked, err)
	}
	rec, _ := doSiteJSON(t, r, http.MethodPost, "/v1/payouts/"+payout.ID+"/confirm-received", siteKey, nil)
	if rec.Code != 200 {
		t.Fatalf("receipt confirmation failed: %d", rec.Code)
	}
	sendStripeEvent(t, db, r, refundPayload("evt_late_refund", "pi_late", 10000))
	_, balance := doSiteJSON(t, r, http.MethodGet, "/v1/payouts/balance", siteKey, nil)
	usd := balance["balances"].([]interface{})[0].(map[string]interface{})
	if usd["mode"] != "test" || usd["paid"] != float64(40) || usd["cleared_unpaid"] != float64(-20) {
		t.Fatalf("late refund hid payment or debt: %v", usd)
	}
}

func TestConnectionCannotCreditOrRefundAnotherProduct(t *testing.T) {
	db, r, _, _, clickID := testPaidCampaign(t)
	var otherID string
	if err := db.Pool.QueryRow(context.Background(), `INSERT INTO products(maker_id,name,website_url,product_type,payment_provider)
SELECT id,'Other product','https://other.example.com','saas','stripe' FROM makers LIMIT 1 RETURNING id`).Scan(&otherID); err != nil {
		t.Fatal(err)
	}
	const otherSecret = "whsec_other_product_secret"
	encrypted, err := testConnectionCipher().Encrypt(otherID, otherSecret)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := db.CreateProductConnection(context.Background(), database.ProductConnection{ProductID: otherID, SecretCiphertext: encrypted, Currency: "USD"}); err != nil {
		t.Fatal(err)
	}
	payload := stripeSalePayload("evt_cross_product", "pi_cross", clickID, 10000)
	payload["data"].(map[string]interface{})["object"].(map[string]interface{})["metadata"] = map[string]interface{}{"dtr_product_id": otherID}
	rec, _ := sendScopedStripeEvent(t, r, otherID, payload, testStripeSecret)
	if rec.Code != 400 {
		t.Fatalf("another connection secret accepted: %d", rec.Code)
	}
	rec, body := sendScopedStripeEvent(t, r, otherID, payload, otherSecret)
	if rec.Code != 200 || body["status"] != "unattributed" {
		t.Fatalf("another product credited: %d %v", rec.Code, body)
	}
	var originalID string
	if err := db.Pool.QueryRow(context.Background(), `SELECT o.product_id FROM clicks c JOIN campaigns cam ON cam.id=c.campaign_id JOIN offers o ON o.id=cam.offer_id WHERE c.id=$1`, clickID).Scan(&originalID); err != nil {
		t.Fatal(err)
	}
	valid := stripeSalePayload("evt_original", "pi_original", clickID, 10000)
	valid["data"].(map[string]interface{})["object"].(map[string]interface{})["metadata"] = map[string]interface{}{"dtr_product_id": originalID}
	sendScopedStripeEvent(t, r, originalID, valid, testStripeSecret)
	rec, body = sendScopedStripeEvent(t, r, otherID, refundPayload("evt_cross_refund", "pi_original", 10000), otherSecret)
	if rec.Code != 200 || body["status"] != "rejected" {
		t.Fatalf("another product refunded: %d %v", rec.Code, body)
	}
}

func TestPendingWebhookKeepsFirstAcceptedPayload(t *testing.T) {
	db, r, siteKey, _, clickID := testPaidCampaign(t)
	sendStripeEvent(t, db, r, refundPayload("evt_canonical", "pi_canonical", 2500))
	sendStripeEvent(t, db, r, refundPayload("evt_canonical", "pi_canonical", 7500))
	sendStripeEvent(t, db, r, stripeSalePayload("evt_canonical_sale", "pi_canonical", clickID, 10000))
	if _, err := db.Pool.Exec(context.Background(), `UPDATE webhook_events SET next_attempt_at=NOW()`); err != nil {
		t.Fatal(err)
	}
	if err := (&WebhooksHandler{DB: db}).ReconcilePending(context.Background()); err != nil {
		t.Fatal(err)
	}
	_, balance := doSiteJSON(t, r, http.MethodGet, "/v1/payouts/balance", siteKey, nil)
	usd := balance["balances"].([]interface{})[0].(map[string]interface{})
	if usd["pending"] != float64(15) {
		t.Fatalf("redelivery replaced accepted payload: %v", usd)
	}
}

func TestPlacementAuthorityAndScopedAgent(t *testing.T) {
	db, r, siteKey, offerID, _ := testPaidCampaign(t)
	var siteID string
	if err := db.Pool.QueryRow(context.Background(), `SELECT id FROM sites WHERE domain='publisher.example.com'`).Scan(&siteID); err != nil {
		t.Fatal(err)
	}
	_, access := adminJSON(t, r, http.MethodPost, "/v1/offer-review/agent-access/sites/"+siteID, nil)
	agentKey := access["api_key"].(string)
	for _, path := range []string{"/v1/marketplace/offers/" + offerID + "/join", "/v1/payouts/11111111-1111-1111-1111-111111111111/confirm-received"} {
		rec, _ := doSiteJSON(t, r, http.MethodPost, path, agentKey, nil)
		if rec.Code != 403 {
			t.Fatalf("agent bypassed approval: %s %d", path, rec.Code)
		}
	}
	payload := map[string]interface{}{"offer_id": offerID, "surface_url": "https://publisher.example.com/email", "context": "InboxPilot email inbox productivity tutorial", "copy": "InboxPilot is a tool to consider for your email workflow."}
	rec, prepared := doSiteJSON(t, r, http.MethodPost, "/v1/placements", agentKey, payload)
	if rec.Code != 201 {
		t.Fatalf("prepare failed: %d %v", rec.Code, prepared)
	}
	p := prepared["placement"].(map[string]interface{})
	id := p["id"].(string)
	rec, _ = doSiteJSON(t, r, http.MethodPost, "/v1/placements/"+id+"/commit", agentKey, map[string]interface{}{"confirm": true})
	if rec.Code != 403 {
		t.Fatalf("confirm=true replaced owner consent: %d", rec.Code)
	}
	otherKey := "site-key-other-owner"
	createTestSite(t, db, otherKey, "other.example.com", "Other")
	rec, _ = doSiteJSON(t, r, http.MethodGet, "/v1/placements/"+id, otherKey, nil)
	if rec.Code != 404 {
		t.Fatalf("cross-owner proposal exposed: %d", rec.Code)
	}
	rec, _ = adminJSON(t, r, http.MethodPost, "/v1/offer-review/placements/"+id+"/approve", map[string]interface{}{"site_id": siteID, "owner_email": "publisher@example.com", "evidence": "Publisher approved exact copy, surface, disclosure and terms in the local test."})
	if rec.Code != 200 {
		t.Fatalf("approval failed: %d", rec.Code)
	}
	rec, committed := doSiteJSON(t, r, http.MethodPost, "/v1/placements/"+id+"/commit", agentKey, nil)
	if rec.Code != 200 {
		t.Fatalf("commit failed: %d %v", rec.Code, committed)
	}
	receipt := committed["placement"].(map[string]interface{})["receipt"].(map[string]interface{})
	_, again := doSiteJSON(t, r, http.MethodPost, "/v1/placements/"+id+"/commit", agentKey, nil)
	if again["placement"].(map[string]interface{})["receipt"].(map[string]interface{})["tracked_link"] != receipt["tracked_link"] {
		t.Fatal("commit changed tracked link")
	}
	tracked := receipt["tracked_link"].(string)
	req := httptest.NewRequest(http.MethodGet, strings.Split(tracked, "?")[0], nil)
	unscoped := httptest.NewRecorder()
	r.ServeHTTP(unscoped, req)
	if strings.Contains(unscoped.Header().Get("Location"), "via=") {
		t.Fatal("removing placement bypassed grant")
	}
	rec, _ = doSiteJSON(t, r, http.MethodPost, "/v1/placements/"+id+"/revoke", siteKey, nil)
	if rec.Code != 200 {
		t.Fatalf("revocation failed: %d", rec.Code)
	}
	rec, _ = doSiteJSON(t, r, http.MethodPost, "/v1/placements/"+id+"/commit", agentKey, nil)
	if rec.Code != 403 {
		t.Fatalf("revoked placement committed: %d", rec.Code)
	}
	revoked := httptest.NewRecorder()
	r.ServeHTTP(revoked, httptest.NewRequest(http.MethodGet, tracked, nil))
	if strings.Contains(revoked.Header().Get("Location"), "via=") {
		t.Fatal("revoked grant still attributed new traffic")
	}
	_, _ = adminJSON(t, r, http.MethodPost, "/v1/offer-review/agent-access/"+access["id"].(string)+"/revoke", nil)
	rec, _ = doSiteJSON(t, r, http.MethodGet, "/v1/placements", agentKey, nil)
	if rec.Code != 401 {
		t.Fatalf("revoked agent key usable: %d", rec.Code)
	}
}

type stdioAgent struct {
	in  io.WriteCloser
	out *bufio.Scanner
	id  int
}

func startStdioAgent(t *testing.T, binary, apiURL, makerKey, siteKey string) *stdioAgent {
	t.Helper()
	cmd := exec.Command(binary)
	for _, value := range os.Environ() {
		if !strings.HasPrefix(value, "DISTRIBUTOOR_") {
			cmd.Env = append(cmd.Env, value)
		}
	}
	cmd.Env = append(cmd.Env, "DISTRIBUTOOR_API_URL="+apiURL, "DISTRIBUTOOR_API_KEY="+makerKey, "DISTRIBUTOOR_SITE_KEY="+siteKey)
	in, err := cmd.StdinPipe()
	if err != nil {
		t.Fatal(err)
	}
	out, err := cmd.StdoutPipe()
	if err != nil {
		t.Fatal(err)
	}
	var stderr bytes.Buffer
	cmd.Stderr = &stderr
	if err := cmd.Start(); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		_ = in.Close()
		if err := cmd.Wait(); err != nil {
			t.Errorf("MCP process failed: %v %s", err, stderr.String())
		}
	})
	return &stdioAgent{in: in, out: bufio.NewScanner(out)}
}

func (a *stdioAgent) call(t *testing.T, name string, args map[string]interface{}) map[string]interface{} {
	t.Helper()
	a.id++
	raw, err := json.Marshal(map[string]interface{}{"jsonrpc": "2.0", "id": a.id, "method": "tools/call", "params": map[string]interface{}{"name": name, "arguments": args}})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := a.in.Write(append(raw, '\n')); err != nil {
		t.Fatal(err)
	}
	if !a.out.Scan() {
		t.Fatalf("MCP process produced no response: %v", a.out.Err())
	}
	var response struct {
		Result struct {
			IsError bool `json:"isError"`
			Content []struct {
				Text string `json:"text"`
			} `json:"content"`
		} `json:"result"`
	}
	if err := json.Unmarshal(a.out.Bytes(), &response); err != nil {
		t.Fatal(err)
	}
	if response.Result.IsError || len(response.Result.Content) == 0 {
		t.Fatalf("tool %s failed: %s", name, a.out.Text())
	}
	var result map[string]interface{}
	if err := json.Unmarshal([]byte(response.Result.Content[0].Text), &result); err != nil {
		t.Fatal(err)
	}
	return result
}

func TestTwoIndependentStdioAgentsRevenueLoop(t *testing.T) {
	db, cleanup := setupTestDB(t)
	defer cleanup()
	r := newRevenueRouter(db)
	server := httptest.NewServer(r)
	defer server.Close()
	_, makerKey := createTestMaker(t, db)
	siteKey := "site-key-independent-publisher"
	siteID := createTestSite(t, db, siteKey, "publisher.example.com", "Developer tutorials")
	_, access := adminJSON(t, r, http.MethodPost, "/v1/offer-review/agent-access/sites/"+siteID, nil)
	binary := filepath.Join(t.TempDir(), "distributoor-mcp")
	build := exec.Command("go", "build", "-o", binary, "./cmd/distributoor-mcp")
	build.Dir = filepath.Join("..", "..")
	if output, err := build.CombinedOutput(); err != nil {
		t.Fatalf("build MCP: %v %s", err, output)
	}
	maker := startStdioAgent(t, binary, server.URL, makerKey, "")
	publisher := startStdioAgent(t, binary, server.URL, "", access["api_key"].(string))
	product := maker.call(t, "register_product", map[string]interface{}{"name": "InboxPilot", "website_url": "https://inboxpilot.example.com", "product_type": "saas", "payment_provider": "stripe", "keywords": []string{"email", "inbox", "productivity"}})
	productID := product["id"].(string)
	markProductReady(t, db, productID)
	adminJSON(t, r, http.MethodPost, "/v1/offer-review/products/"+productID+"/connection", map[string]interface{}{"signing_secret": testStripeSecret, "currency": "USD", "live_mode": false})
	offer := maker.call(t, "create_offer", map[string]interface{}{"product_id": productID, "commission_type": "percentage", "commission_value": 20, "refund_window_days": 30})
	offerID := offer["id"].(string)
	maker.call(t, "submit_offer", map[string]interface{}{"offer_id": offerID})
	adminJSON(t, r, http.MethodPost, "/v1/offer-review/"+offerID+"/approve", nil)
	content := "Email inbox productivity tutorial: evaluate InboxPilot for organizing email."
	matched := publisher.call(t, "find_opportunities", map[string]interface{}{"context": content})
	if matched["status"] != "matched" {
		t.Fatalf("no relevant offer: %v", matched)
	}
	noMatch := publisher.call(t, "find_opportunities", map[string]interface{}{"context": "A tutorial on gardening with tomatoes and soil"})
	if noMatch["status"] != "no_match" {
		t.Fatal("irrelevant products recommended")
	}
	prepared := publisher.call(t, "prepare_placement", map[string]interface{}{"offer_id": offerID, "surface_url": "https://publisher.example.com/tutorial", "context": content, "copy": "Consider InboxPilot for an email inbox productivity workflow."})
	p := prepared["placement"].(map[string]interface{})
	id := p["id"].(string)
	adminJSON(t, r, http.MethodPost, "/v1/offer-review/placements/"+id+"/approve", map[string]interface{}{"site_id": siteID, "owner_email": "publisher@example.com", "evidence": "Independent publisher approved this exact proposal in the local proof."})
	committed := publisher.call(t, "commit_placement", map[string]interface{}{"placement_id": id})
	receipt := committed["placement"].(map[string]interface{})["receipt"].(map[string]interface{})
	tracked := receipt["tracked_link"].(string)
	text := p["copy"].(string) + "\n\n" + server.URL + tracked + "\n\n" + p["disclosure"].(string)
	artifact := filepath.Join(t.TempDir(), "approved-placement.md")
	if err := os.WriteFile(artifact, []byte(text), 0600); err != nil {
		t.Fatal(err)
	}
	publisher.call(t, "record_placement_receipt", map[string]interface{}{"placement_id": id, "surface_url": p["surface_url"], "published_text": text, "evidence": "Local artifact only, no public publication: " + artifact})
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, tracked, nil))
	u, err := url.Parse(rec.Header().Get("Location"))
	if err != nil {
		t.Fatal(err)
	}
	sendStripeEvent(t, db, r, stripeSalePayload("evt_two_agents", "pi_two_agents", u.Query().Get("via"), 4900))
	balances := publisher.call(t, "get_balance", nil)["balances"].([]interface{})
	if balances[0].(map[string]interface{})["pending"] != float64(9.8) {
		t.Fatalf("wrong commission: %v", balances)
	}
	earnings := maker.call(t, "get_earnings", nil)
	if earnings["totals"].([]interface{})[0].(map[string]interface{})["commission"] != float64(9.8) {
		t.Fatalf("maker ledger differs: %v", earnings)
	}
	sendStripeEvent(t, db, r, refundPayload("evt_two_agents_refund", "pi_two_agents", 4900))
	balances = publisher.call(t, "get_balance", nil)["balances"].([]interface{})
	if balances[0].(map[string]interface{})["pending"] != float64(0) {
		t.Fatalf("refund did not reverse: %v", balances)
	}
	t.Log("Two separately keyed MCP processes completed discovery → owner-approved local placement → signed paid-event fixture → matching $9.80 commission → full refund. No live Stripe checkout or public publication was performed.")
}
