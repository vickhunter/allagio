package handlers

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"affilio-api/internal/database"
	"affilio-api/internal/middleware"
	"affilio-api/internal/services"

	"github.com/gin-gonic/gin"
)

func newMakerRouter(db *database.DB) *gin.Engine {
	return newMakerRouterWithClaims(db, "")
}

func newMakerRouterWithClaims(db *database.DB, claims string) *gin.Engine {
	return newMakerRouterFull(db, &http.Client{Timeout: 5 * time.Second}, claims)
}

func newMakerRouterWithReadinessClient(db *database.DB, client *http.Client) *gin.Engine {
	return newMakerRouterFull(db, client, "")
}

func newMakerRouterFull(db *database.DB, client *http.Client, claims string) *gin.Engine {
	gin.SetMode(gin.TestMode)
	r := gin.New()
	products := &ProductsHandler{DB: db}
	offers := &OffersHandler{DB: db, ClaimsCheck: claims}
	assessments := &AssessmentsHandler{
		DB:     db,
		Engine: &services.ReadinessEngine{AllowPrivateHosts: true, Client: client},
	}
	maker := r.Group("/v1/maker")
	maker.Use(middleware.MakerKeyAuth(db))
	{
		maker.GET("/getting-started", products.GettingStarted)
		maker.POST("/products", products.Create)
		maker.GET("/products", products.List)
		maker.GET("/products/:id", products.Get)
		maker.PATCH("/products/:id", products.Patch)
		maker.POST("/products/:id/assess", assessments.Assess)
		maker.GET("/products/:id/assessments", assessments.List)
		maker.POST("/offers", offers.Create)
		maker.GET("/offers", offers.List)
		maker.GET("/offers/:id", offers.Get)
		maker.PATCH("/offers/:id", offers.Patch)
		maker.POST("/offers/:id/submit", offers.Submit)
		maker.GET("/offers/:id/matches", offers.Matches)
		reports := &RefundReportHandler{DB: db}
		maker.POST("/sales", reports.ReportSale)
		maker.POST("/refunds", reports.ReportRefund)
	}
	listings := r.Group("/v1/listings")
	listings.Use(middleware.MakerKeyAuth(db))
	listings.GET("", offers.Listings)
	review := &OfferReviewHandler{DB: db, ClaimsCheck: claims}
	admin := r.Group("/v1/offer-review")
	admin.Use(middleware.AdminKeyAuth("admin-secret", services.NewAuditLogger(db)))
	{
		admin.GET("/queue", review.Queue)
		admin.POST("/:id/approve", review.Approve)
		admin.POST("/:id/reject", review.Reject)
		admin.POST("/makers/:id/trust", review.SetMakerTrust)
		admin.GET("/flagged-conversions", review.FlaggedConversions)
		admin.POST("/conversions/:id/clear-flag", review.ClearConversionFlag)
	}
	return r
}

func createTestMaker(t *testing.T, db *database.DB) (makerID, apiKey string) {
	t.Helper()
	apiKey = "maker-key-test"
	id, err := db.CreateMaker(context.Background(), services.HashAPIKey(apiKey), services.KeyPrefix(apiKey), "maker@example.com", "Test Maker")
	if err != nil {
		t.Fatalf("create maker: %v", err)
	}
	return id, apiKey
}

func markProductReady(t *testing.T, db *database.DB, productID string) {
	t.Helper()
	_, err := db.Pool.Exec(context.Background(), `UPDATE products SET readiness_status='ready', readiness_score=90 WHERE id=$1`, productID)
	if err != nil {
		t.Fatalf("mark product ready: %v", err)
	}
}

func doJSON(t *testing.T, r *gin.Engine, method, path, makerKey string, payload interface{}) (*httptest.ResponseRecorder, map[string]interface{}) {
	t.Helper()
	var req *http.Request
	if payload != nil {
		req = httptest.NewRequest(method, path, newJSONRequest(t, method, path, payload))
	} else {
		req = httptest.NewRequest(method, path, nil)
	}
	if makerKey != "" {
		req.Header.Set("X-Maker-Key", makerKey)
	}
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, req)
	body := map[string]interface{}{}
	_ = json.Unmarshal(rec.Body.Bytes(), &body)
	return rec, body
}

func TestMakerRegisterValidatesAndConflicts(t *testing.T) {
	db, cleanup := setupTestDB(t)
	defer cleanup()

	handler := &MakersHandler{DB: db}
	gin.SetMode(gin.TestMode)
	r := gin.New()
	r.POST("/v1/makers/register", handler.Register)

	rec, body := doJSON(t, r, http.MethodPost, "/v1/makers/register", "", map[string]interface{}{
		"email": "vibe@example.com",
		"name":  "Vibe Coder",
	})
	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %v", rec.Code, body)
	}
	if body["api_key"] == "" || body["maker_id"] == "" {
		t.Fatalf("expected api_key and maker_id, got %v", body)
	}

	rec, _ = doJSON(t, r, http.MethodPost, "/v1/makers/register", "", map[string]interface{}{
		"email": "vibe@example.com",
	})
	if rec.Code != http.StatusConflict {
		t.Fatalf("expected 409 for duplicate email, got %d", rec.Code)
	}

	rec, _ = doJSON(t, r, http.MethodPost, "/v1/makers/register", "", map[string]interface{}{
		"email": "not-an-email",
	})
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("expected 400 for invalid email, got %d", rec.Code)
	}
}

func TestMakerAuthRejectsUnknownKey(t *testing.T) {
	db, cleanup := setupTestDB(t)
	defer cleanup()

	r := newMakerRouter(db)
	rec, _ := doJSON(t, r, http.MethodGet, "/v1/maker/products", "unknown-key", nil)
	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("expected 401 for unknown maker key, got %d", rec.Code)
	}
	rec, _ = doJSON(t, r, http.MethodGet, "/v1/maker/products", "", nil)
	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("expected 401 for missing maker key, got %d", rec.Code)
	}
}

func TestProductOfferLifecycle(t *testing.T) {
	db, cleanup := setupTestDB(t)
	defer cleanup()

	_, makerKey := createTestMaker(t, db)
	r := newMakerRouter(db)

	rec, product := doJSON(t, r, http.MethodPost, "/v1/maker/products", makerKey, map[string]interface{}{
		"name":             "PromptForge",
		"tagline":          "Prompt engineering toolkit for indie hackers",
		"website_url":      "https://promptforge.example.com",
		"product_type":     "saas",
		"payment_provider": "stripe",
		"primary_category": "software",
		"keywords":         []string{"prompt", "ai", "toolkit"},
	})
	if rec.Code != http.StatusOK {
		t.Fatalf("create product: expected 200, got %d: %v", rec.Code, product)
	}
	productID := product["id"].(string)
	if product["readiness_status"] != "unassessed" {
		t.Fatalf("expected unassessed readiness, got %v", product["readiness_status"])
	}

	rec, offer := doJSON(t, r, http.MethodPost, "/v1/maker/offers", makerKey, map[string]interface{}{
		"product_id":       productID,
		"commission_type":  "percentage",
		"commission_value": 30,
	})
	if rec.Code != http.StatusOK {
		t.Fatalf("create offer: expected 200, got %d: %v", rec.Code, offer)
	}
	offerID := offer["id"].(string)
	if offer["status"] != "draft" {
		t.Fatalf("expected draft offer, got %v", offer["status"])
	}
	if offer["landing_url"] != "https://promptforge.example.com" {
		t.Fatalf("expected landing_url to default to product website, got %v", offer["landing_url"])
	}

	// Maker cannot self-list: draft -> listed must be rejected.
	rec, _ = doJSON(t, r, http.MethodPatch, "/v1/maker/offers/"+offerID, makerKey, map[string]interface{}{
		"status": "listed",
	})
	if rec.Code != http.StatusConflict {
		t.Fatalf("expected 409 for draft->listed, got %d", rec.Code)
	}

	// Submit is gated on readiness: unassessed products cannot list.
	rec, _ = doJSON(t, r, http.MethodPost, "/v1/maker/offers/"+offerID+"/submit", makerKey, nil)
	if rec.Code != http.StatusConflict {
		t.Fatalf("expected 409 for unassessed product submit, got %d", rec.Code)
	}
	markProductReady(t, db, productID)

	rec, _ = doJSON(t, r, http.MethodPost, "/v1/maker/offers/"+offerID+"/submit", makerKey, nil)
	if rec.Code != http.StatusOK {
		t.Fatalf("submit offer: expected 200, got %d", rec.Code)
	}

	// Second submit must fail: only draft offers can be submitted.
	rec, _ = doJSON(t, r, http.MethodPost, "/v1/maker/offers/"+offerID+"/submit", makerKey, nil)
	if rec.Code != http.StatusConflict {
		t.Fatalf("expected 409 for double submit, got %d", rec.Code)
	}

	// Admin queue shows the submitted offer.
	req := httptest.NewRequest(http.MethodGet, "/v1/offer-review/queue", nil)
	req.Header.Set("X-Admin-Key", "admin-secret")
	qrec := httptest.NewRecorder()
	r.ServeHTTP(qrec, req)
	if qrec.Code != http.StatusOK {
		t.Fatalf("queue: expected 200, got %d", qrec.Code)
	}
	var queueBody struct {
		Queue []map[string]interface{} `json:"queue"`
	}
	if err := json.Unmarshal(qrec.Body.Bytes(), &queueBody); err != nil {
		t.Fatalf("queue decode: %v", err)
	}
	if len(queueBody.Queue) != 1 || queueBody.Queue[0]["id"] != offerID {
		t.Fatalf("expected offer in review queue, got %v", queueBody.Queue)
	}

	// Approve promotes into affiliate_programs with source=distributoor.
	req = httptest.NewRequest(http.MethodPost, "/v1/offer-review/"+offerID+"/approve", nil)
	req.Header.Set("X-Admin-Key", "admin-secret")
	arec := httptest.NewRecorder()
	r.ServeHTTP(arec, req)
	if arec.Code != http.StatusOK {
		t.Fatalf("approve: expected 200, got %d: %s", arec.Code, arec.Body.String())
	}
	var approveBody map[string]interface{}
	_ = json.Unmarshal(arec.Body.Bytes(), &approveBody)
	programID, _ := approveBody["program_id"].(string)
	if programID == "" {
		t.Fatalf("expected program_id in approve response, got %v", approveBody)
	}

	var source, name string
	var linkedOfferID *string
	err := db.Pool.QueryRow(context.Background(), `
SELECT source, name, offer_id::text FROM affiliate_programs WHERE id=$1`, programID).Scan(&source, &name, &linkedOfferID)
	if err != nil {
		t.Fatalf("load promoted program: %v", err)
	}
	if source != "distributoor" || name != "PromptForge" || linkedOfferID == nil || *linkedOfferID != offerID {
		t.Fatalf("unexpected promoted program: source=%s name=%s offer_id=%v", source, name, linkedOfferID)
	}

	rec, offer = doJSON(t, r, http.MethodGet, "/v1/maker/offers/"+offerID, makerKey, nil)
	if rec.Code != http.StatusOK || offer["status"] != "listed" {
		t.Fatalf("expected listed offer after approve, got %d %v", rec.Code, offer["status"])
	}

	// Listed offers can pause and resume but never return to draft.
	rec, _ = doJSON(t, r, http.MethodPatch, "/v1/maker/offers/"+offerID, makerKey, map[string]interface{}{"status": "paused"})
	if rec.Code != http.StatusOK {
		t.Fatalf("pause: expected 200, got %d", rec.Code)
	}
	rec, _ = doJSON(t, r, http.MethodPatch, "/v1/maker/offers/"+offerID, makerKey, map[string]interface{}{"status": "draft"})
	if rec.Code != http.StatusConflict {
		t.Fatalf("expected 409 for paused->draft, got %d", rec.Code)
	}
	rec, _ = doJSON(t, r, http.MethodPatch, "/v1/maker/offers/"+offerID, makerKey, map[string]interface{}{"status": "retired"})
	if rec.Code != http.StatusOK {
		t.Fatalf("retire: expected 200, got %d", rec.Code)
	}
}

func TestOfferRejectReturnsToDraft(t *testing.T) {
	db, cleanup := setupTestDB(t)
	defer cleanup()

	_, makerKey := createTestMaker(t, db)
	r := newMakerRouter(db)

	_, product := doJSON(t, r, http.MethodPost, "/v1/maker/products", makerKey, map[string]interface{}{
		"name":         "PixelQuest",
		"website_url":  "https://pixelquest.example.com",
		"product_type": "game",
	})
	_, offer := doJSON(t, r, http.MethodPost, "/v1/maker/offers", makerKey, map[string]interface{}{
		"product_id":       product["id"],
		"commission_type":  "flat",
		"commission_value": 5,
	})
	offerID := offer["id"].(string)
	markProductReady(t, db, product["id"].(string))
	doJSON(t, r, http.MethodPost, "/v1/maker/offers/"+offerID+"/submit", makerKey, nil)

	req := httptest.NewRequest(http.MethodPost, "/v1/offer-review/"+offerID+"/reject", newJSONRequest(t, http.MethodPost, "", map[string]interface{}{
		"note": "landing page is broken",
	}))
	req.Header.Set("X-Admin-Key", "admin-secret")
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("reject: expected 200, got %d", rec.Code)
	}

	grec, body := doJSON(t, r, http.MethodGet, "/v1/maker/offers/"+offerID, makerKey, nil)
	if grec.Code != http.StatusOK || body["status"] != "draft" {
		t.Fatalf("expected draft after reject, got %d %v", grec.Code, body["status"])
	}
}

func TestAssessGatesOfferSubmitEndToEnd(t *testing.T) {
	db, cleanup := setupTestDB(t)
	defer cleanup()

	_, makerKey := createTestMaker(t, db)
	mux := http.NewServeMux()
	mux.HandleFunc("/", func(w http.ResponseWriter, req *http.Request) {
		if req.URL.Path == "/docs" {
			w.Write([]byte("<html><body>" + strings.Repeat("ShipFast documents the product for publishers. ", 20) + "</body></html>"))
			return
		}
		w.Write([]byte(`<html><body><a href="/signup">Sign up</a> <a href="/privacy">Privacy</a> <a href="/terms">Terms</a> for $9/month <script src="https://js.stripe.com/v3/"></script></body></html>`))
	})
	site := httptest.NewTLSServer(mux)
	defer site.Close()
	probe := site.Client()
	probe.Timeout = 5 * time.Second
	r := newMakerRouterWithReadinessClient(db, probe)

	_, product := doJSON(t, r, http.MethodPost, "/v1/maker/products", makerKey, map[string]interface{}{
		"name":             "ShipFast",
		"website_url":      site.URL,
		"pricing_url":      site.URL,
		"docs_url":         site.URL + "/docs",
		"product_type":     "saas",
		"payment_provider": "stripe",
	})
	productID := product["id"].(string)

	_, offer := doJSON(t, r, http.MethodPost, "/v1/maker/offers", makerKey, map[string]interface{}{
		"product_id":       productID,
		"commission_type":  "recurring",
		"commission_value": 25,
	})
	offerID := offer["id"].(string)

	rec, body := doJSON(t, r, http.MethodPost, "/v1/maker/offers/"+offerID+"/submit", makerKey, nil)
	if rec.Code != http.StatusConflict {
		t.Fatalf("expected 409 before assessment, got %d: %v", rec.Code, body)
	}

	rec, assessment := doJSON(t, r, http.MethodPost, "/v1/maker/products/"+productID+"/assess", makerKey, nil)
	if rec.Code != http.StatusOK {
		t.Fatalf("assess: expected 200, got %d: %v", rec.Code, assessment)
	}
	if assessment["readiness_status"] != "ready" {
		t.Fatalf("expected ready product, got %v (checks: %v)", assessment["readiness_status"], assessment["checks"])
	}

	rec, _ = doJSON(t, r, http.MethodPost, "/v1/maker/offers/"+offerID+"/submit", makerKey, nil)
	if rec.Code != http.StatusOK {
		t.Fatalf("submit after assess: expected 200, got %d", rec.Code)
	}

	rec, listBody := doJSON(t, r, http.MethodGet, "/v1/maker/products/"+productID+"/assessments", makerKey, nil)
	if rec.Code != http.StatusOK {
		t.Fatalf("list assessments: expected 200, got %d", rec.Code)
	}
	assessments, _ := listBody["assessments"].([]interface{})
	if len(assessments) != 1 {
		t.Fatalf("expected 1 stored assessment, got %d", len(assessments))
	}
}

func TestUnspecifiedProviderStaysEmptyAndLegacyStripeStillReads(t *testing.T) {
	db, cleanup := setupTestDB(t)
	defer cleanup()

	demo := ginEngine()
	hosted := &HostedHandler{BaseURL: "http://127.0.0.1"}
	hosted.Mount(demo)
	site := httptest.NewServer(demo)
	defer site.Close()

	makerID, makerKey := createTestMaker(t, db)
	client := site.Client()
	client.Timeout = 5 * time.Second
	r := newMakerRouterWithReadinessClient(db, client)
	rec, product := doJSON(t, r, http.MethodPost, "/v1/maker/products", makerKey, map[string]interface{}{
		"name":        "Neutral",
		"website_url": "https://neutral.example",
	})
	if rec.Code != http.StatusOK {
		t.Fatalf("create: %d %v", rec.Code, product)
	}
	productID := product["id"].(string)
	if product["payment_provider"] != "" {
		t.Fatalf("omitted provider must be stored empty, got %v", product["payment_provider"])
	}
	var stored *string
	if err := db.Pool.QueryRow(context.Background(), `SELECT payment_provider FROM products WHERE id=$1`, productID).Scan(&stored); err != nil {
		t.Fatal(err)
	}
	if stored == nil || *stored != "" {
		t.Fatalf("empty provider must stay an empty string, got %v", stored)
	}

	var legacyID, legacyProvider string
	if err := db.Pool.QueryRow(context.Background(), `INSERT INTO products(maker_id, name, website_url, product_type, payment_provider)
VALUES ($1,'Legacy','https://legacy.example','saas','stripe') RETURNING id`, makerID).Scan(&legacyID); err != nil {
		t.Fatal(err)
	}
	if err := db.Pool.QueryRow(context.Background(), `SELECT payment_provider FROM products WHERE id=$1`, legacyID).Scan(&legacyProvider); err != nil {
		t.Fatal(err)
	}
	if legacyProvider != "stripe" {
		t.Fatalf("legacy stripe insert must still read stripe, got %q", legacyProvider)
	}
	var absentID string
	var absent *string
	if err := db.Pool.QueryRow(context.Background(), `INSERT INTO products(maker_id, name, website_url, product_type, payment_provider)
VALUES ($1,'Absent','https://absent.example','saas',NULL) RETURNING id`, makerID).Scan(&absentID); err != nil {
		t.Fatal(err)
	}
	if err := db.Pool.QueryRow(context.Background(), `SELECT payment_provider FROM products WHERE id=$1`, absentID).Scan(&absent); err != nil {
		t.Fatal(err)
	}
	if absent != nil {
		t.Fatalf("NULL provider must stay NULL, got %q", *absent)
	}

	base := site.URL
	if _, err := db.Pool.Exec(context.Background(), `UPDATE products SET website_url=$2, pricing_url=$3, docs_url=$4 WHERE id=$1`,
		productID, base+"/demo", base+"/demo/pricing", base+"/demo/docs"); err != nil {
		t.Fatal(err)
	}
	rec, assessment := doJSON(t, r, http.MethodPost, "/v1/maker/products/"+productID+"/assess", makerKey, nil)
	if rec.Code != http.StatusOK || assessment["readiness_status"] != "ready" {
		t.Fatalf("empty provider must still pass readiness, got %d %v", rec.Code, assessment)
	}
	_, offer := doJSON(t, r, http.MethodPost, "/v1/maker/offers", makerKey, map[string]interface{}{
		"product_id":       productID,
		"commission_type":  "flat",
		"commission_value": 8,
	})
	submitted, body := doJSON(t, r, http.MethodPost, "/v1/maker/offers/"+offer["id"].(string)+"/submit", makerKey, nil)
	if submitted.Code != http.StatusOK || body["status"] != "submitted" {
		t.Fatalf("empty provider must submit, got %d %v", submitted.Code, body)
	}
}

func TestSubmitDoesNotRequireAPaymentConnection(t *testing.T) {
	db, cleanup := setupTestDB(t)
	defer cleanup()

	makerID, makerKey := createTestMaker(t, db)
	r := newMakerRouter(db)
	_, product := doJSON(t, r, http.MethodPost, "/v1/maker/products", makerKey, map[string]interface{}{
		"name":             "Agnostic",
		"website_url":      "https://agnostic.example",
		"payment_provider": "none",
	})
	productID := product["id"].(string)
	var connections int
	if err := db.Pool.QueryRow(context.Background(), `SELECT COUNT(*) FROM product_connections WHERE product_id=$1`, productID).Scan(&connections); err != nil {
		t.Fatal(err)
	}
	if connections != 0 {
		t.Fatalf("expected no payment connection, got %d", connections)
	}
	markProductReady(t, db, productID)
	_, blocked := doJSON(t, r, http.MethodPost, "/v1/maker/offers", makerKey, map[string]interface{}{
		"product_id":       productID,
		"commission_type":  "flat",
		"commission_value": 8,
	})
	// A second offer is created after the first submit so the trusted path can list.
	firstID := blocked["id"].(string)
	early, earlyBody := doJSON(t, r, http.MethodPost, "/v1/maker/offers/"+firstID+"/submit", makerKey, nil)
	if early.Code != http.StatusOK || earlyBody["status"] != "submitted" {
		t.Fatalf("ready product with no provider connection must submit, got %d %v", early.Code, earlyBody)
	}
	if hint, _ := earlyBody["hint"].(string); strings.Contains(hint, "/v1/") || strings.Contains(fmt.Sprint(earlyBody["error"]), "/v1/") {
		t.Fatalf("submit response must not name an API path: %v", earlyBody)
	}

	req := httptest.NewRequest(http.MethodPost, "/v1/offer-review/makers/"+makerID+"/trust", newJSONRequest(t, http.MethodPost, "", map[string]interface{}{"trusted": true}))
	req.Header.Set("X-Admin-Key", "admin-secret")
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("trust: %d %s", rec.Code, rec.Body.String())
	}
	_, listedOffer := doJSON(t, r, http.MethodPost, "/v1/maker/offers", makerKey, map[string]interface{}{
		"product_id":       productID,
		"commission_type":  "flat",
		"commission_value": 9,
	})
	listed, listedBody := doJSON(t, r, http.MethodPost, "/v1/maker/offers/"+listedOffer["id"].(string)+"/submit", makerKey, nil)
	if listed.Code != http.StatusOK || listedBody["status"] != "listed" || listedBody["program_id"] == nil || listedBody["program_id"] == "" {
		t.Fatalf("trusted maker with no provider connection must list, got %d %v", listed.Code, listedBody)
	}
}

func TestUnreadySubmitErrorHasNoAPIPath(t *testing.T) {
	db, cleanup := setupTestDB(t)
	defer cleanup()
	_, makerKey := createTestMaker(t, db)
	r := newMakerRouter(db)
	_, product := doJSON(t, r, http.MethodPost, "/v1/maker/products", makerKey, map[string]interface{}{
		"name":        "NotReady",
		"website_url": "https://notready.example",
	})
	_, offer := doJSON(t, r, http.MethodPost, "/v1/maker/offers", makerKey, map[string]interface{}{
		"product_id":       product["id"],
		"commission_type":  "flat",
		"commission_value": 4,
	})
	rec, body := doJSON(t, r, http.MethodPost, "/v1/maker/offers/"+offer["id"].(string)+"/submit", makerKey, nil)
	if rec.Code != http.StatusConflict {
		t.Fatalf("expected 409, got %d", rec.Code)
	}
	encoded := fmt.Sprint(body)
	if strings.Contains(encoded, "/v1/") || strings.Contains(encoded, "POST ") {
		t.Fatalf("readiness refusal must be plain language, got %s", encoded)
	}
}

func TestAssessFailingProductBlocksSubmit(t *testing.T) {
	db, cleanup := setupTestDB(t)
	defer cleanup()

	_, makerKey := createTestMaker(t, db)
	// No checkout marker anywhere: the checkout_present gate must fail.
	site := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
		w.Write([]byte(`<html><body>Just a landing page</body></html>`))
	}))
	defer site.Close()
	probe := site.Client()
	probe.Timeout = 5 * time.Second
	r := newMakerRouterWithReadinessClient(db, probe)

	_, product := doJSON(t, r, http.MethodPost, "/v1/maker/products", makerKey, map[string]interface{}{
		"name":        "NoCheckout",
		"website_url": site.URL,
	})
	productID := product["id"].(string)

	rec, assessment := doJSON(t, r, http.MethodPost, "/v1/maker/products/"+productID+"/assess", makerKey, nil)
	if rec.Code != http.StatusOK {
		t.Fatalf("assess: expected 200, got %d", rec.Code)
	}
	if assessment["readiness_status"] != "failing" {
		t.Fatalf("expected failing product, got %v", assessment["readiness_status"])
	}

	_, offer := doJSON(t, r, http.MethodPost, "/v1/maker/offers", makerKey, map[string]interface{}{
		"product_id":       productID,
		"commission_type":  "flat",
		"commission_value": 10,
	})
	rec, _ = doJSON(t, r, http.MethodPost, "/v1/maker/offers/"+offer["id"].(string)+"/submit", makerKey, nil)
	if rec.Code != http.StatusConflict {
		t.Fatalf("expected 409 for failing product, got %d", rec.Code)
	}
}

func TestProductIsolationBetweenMakers(t *testing.T) {
	db, cleanup := setupTestDB(t)
	defer cleanup()

	_, keyA := createTestMaker(t, db)
	if _, err := db.CreateMaker(context.Background(), services.HashAPIKey("maker-key-b"), services.KeyPrefix("maker-key-b"), "other@example.com", "Other"); err != nil {
		t.Fatalf("create second maker: %v", err)
	}
	r := newMakerRouter(db)

	_, product := doJSON(t, r, http.MethodPost, "/v1/maker/products", keyA, map[string]interface{}{
		"name":        "SoloTool",
		"website_url": "https://solotool.example.com",
	})
	productID := product["id"].(string)

	rec, _ := doJSON(t, r, http.MethodGet, "/v1/maker/products/"+productID, "maker-key-b", nil)
	if rec.Code != http.StatusNotFound {
		t.Fatalf("expected 404 for cross-maker product access, got %d", rec.Code)
	}
}
