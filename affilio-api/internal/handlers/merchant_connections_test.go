package handlers

import (
	"bytes"
	"context"
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"html/template"
	"io"
	"log"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"net/url"
	"regexp"
	"strings"
	"testing"
	"time"

	"affilio-api/internal/database"
	"affilio-api/internal/services"

	"github.com/gin-gonic/gin"
)

type fixtureStripeVerifier struct {
	check       services.StripeConnectionCheck
	calls       int
	expectedURL string
}

func (f *fixtureStripeVerifier) Verify(_ context.Context, key, price, endpoint, webhook string) (services.StripeConnectionCheck, error) {
	f.calls++
	if key != "rk_test_verify_once" || price != "price_pilot" || endpoint != "we_pilot" || webhook != f.expectedURL {
		return services.StripeConnectionCheck{}, context.Canceled
	}
	return f.check, nil
}

func TestOwnerStripeConnectionAndRotation(t *testing.T) {
	db, _, _, offerID, _ := testPaidCampaign(t)
	h, r, _ := hostedTestServer(t, db)
	o, cookie := testOwnerSession(t, h, "merchant-owner", "merchant@example.com")
	var productID, makerID string
	if err := db.Pool.QueryRow(context.Background(), `SELECT p.id,p.maker_id FROM products p JOIN offers o ON o.product_id=p.id WHERE o.id=$1`, offerID).Scan(&productID, &makerID); err != nil {
		t.Fatal(err)
	}
	if ok, err := db.BindOwnerAccount(context.Background(), o.ID, "maker", makerID); err != nil || !ok {
		t.Fatal(err)
	}
	verify := &fixtureStripeVerifier{check: services.StripeConnectionCheck{AccountID: "acct_pilot", PriceID: "price_pilot", EndpointID: "we_pilot", Currency: "USD"}, expectedURL: h.BaseURL + "/v1/webhooks/stripe/" + productID}
	merchant := &MerchantConnectionsHandler{DB: db, Cipher: testConnectionCipher(), Verifier: verify, BaseURL: h.BaseURL}
	group := r.Group("/owner", h.ownerSession)
	group.GET("/products/:id/connection", merchant.Form)
	group.POST("/products/:id/connection", merchant.Connect)
	values := url.Values{"csrf": {o.CSRF}, "stripe_key": {"rk_test_verify_once"}, "price_id": {"price_pilot"}, "endpoint_id": {"we_pilot"}, "signing_secret": {"whsec_verified_pilot_secret"}}
	foreign, foreignCookie := testOwnerSession(t, h, "foreign-merchant", "other@example.com")
	foreignValues, _ := url.ParseQuery(values.Encode())
	foreignValues.Set("csrf", foreign.CSRF)
	if rec := ownerForm(t, h, r, foreignCookie, "/owner/products/"+productID+"/connection", foreignValues); rec.Code != 404 || verify.calls != 0 {
		t.Fatal("foreign merchant verified another product")
	}
	if rec := ownerForm(t, h, r, nil, "/owner/products/"+productID+"/connection", values); rec.Code != 401 || verify.calls != 0 {
		t.Fatal("agent credential could connect payments")
	}
	connected := ownerForm(t, h, r, cookie, "/owner/products/"+productID+"/connection", values)
	if connected.Code != 201 || !strings.Contains(connected.Body.String(), "Stripe configuration verified") {
		t.Fatalf("connection: %d %s", connected.Code, connected.Body.String())
	}
	connection, err := db.ProductConnection(context.Background(), productID)
	if err != nil {
		t.Fatal(err)
	}
	decrypted, err := testConnectionCipher().Decrypt(productID, connection.SecretCiphertext)
	if err != nil || decrypted != values.Get("signing_secret") {
		t.Fatal("signing secret not encrypted with product binding")
	}
	var accountID, priceID, endpointID string
	var verified bool
	if err = db.Pool.QueryRow(context.Background(), `SELECT stripe_account_id,stripe_price_id,stripe_endpoint_id,configuration_verified_at IS NOT NULL FROM product_connections WHERE product_id=$1`, productID).Scan(&accountID, &priceID, &endpointID, &verified); err != nil || accountID != "acct_pilot" || priceID != "price_pilot" || endpointID != "we_pilot" || !verified {
		t.Fatalf("verification not recorded: %v", err)
	}
	values.Set("signing_secret", "whsec_rotated_pilot_secret")
	if rec := ownerForm(t, h, r, cookie, "/owner/products/"+productID+"/connection", values); rec.Code != 201 {
		t.Fatalf("secret rotation: %d", rec.Code)
	}
	verify.check.Currency = "EUR"
	if rec := ownerForm(t, h, r, cookie, "/owner/products/"+productID+"/connection", values); rec.Code != 409 {
		t.Fatal("rotation changed currency")
	}
	verify.check.Currency = "USD"
	verify.check.AccountID = "acct_foreign"
	if rec := ownerForm(t, h, r, cookie, "/owner/products/"+productID+"/connection", values); rec.Code != 409 {
		t.Fatal("rotation changed merchant account")
	}
}

func TestStripeConnectionPageIsTestModeOnly(t *testing.T) {
	db, _, _, offerID, _ := testPaidCampaign(t)
	h, r, _ := hostedTestServer(t, db)
	o, cookie := testOwnerSession(t, h, "test-mode-owner", "test-mode@example.com")
	var productID, makerID string
	if err := db.Pool.QueryRow(context.Background(), `SELECT p.id,p.maker_id FROM products p JOIN offers o ON o.product_id=p.id WHERE o.id=$1`, offerID).Scan(&productID, &makerID); err != nil {
		t.Fatal(err)
	}
	if ok, err := db.BindOwnerAccount(context.Background(), o.ID, "maker", makerID); err != nil || !ok {
		t.Fatal(err)
	}
	verify := &fixtureStripeVerifier{expectedURL: h.BaseURL + "/v1/webhooks/stripe/" + productID}
	merchant := &MerchantConnectionsHandler{DB: db, Cipher: testConnectionCipher(), Verifier: verify, BaseURL: h.BaseURL, Page: h.Page}
	group := r.Group("/owner", h.ownerSession)
	group.GET("/products/:id/connection", merchant.Form)
	group.POST("/products/:id/connection", merchant.Connect)
	get := httptest.NewRequest("GET", "/owner/products/"+productID+"/connection", nil)
	get.AddCookie(cookie)
	page := httptest.NewRecorder()
	r.ServeHTTP(page, get)
	if page.Code != 200 || !strings.Contains(strings.ToLower(page.Body.String()), "test mode only") {
		t.Fatalf("connection page must say test mode only: %d %s", page.Code, page.Body.String())
	}
	before := verify.calls
	for _, key := range []string{"sk_live_rejected", "rk_live_rejected"} {
		values := url.Values{"csrf": {o.CSRF}, "stripe_key": {key}, "price_id": {"price_pilot"}, "endpoint_id": {"we_pilot"}, "signing_secret": {"whsec_verified_pilot_secret"}}
		rec := ownerForm(t, h, r, cookie, "/owner/products/"+productID+"/connection", values)
		if rec.Code != 400 || !strings.Contains(strings.ToLower(rec.Body.String()), "test mode") {
			t.Fatalf("%s: expected a test-mode refusal, got %d %s", key, rec.Code, rec.Body.String())
		}
	}
	if verify.calls != before {
		t.Fatal("live keys were sent to Stripe")
	}
}

func TestLiveKeyRefusalStaysOnTheFormAndOutOfLogs(t *testing.T) {
	db, _, _, offerID, _ := testPaidCampaign(t)
	h, _, _ := hostedTestServer(t, db)
	o, cookie := testOwnerSession(t, h, "live-key-owner", "live-key@example.com")
	var productID, makerID string
	if err := db.Pool.QueryRow(context.Background(), `SELECT p.id,p.maker_id FROM products p JOIN offers o ON o.product_id=p.id WHERE o.id=$1`, offerID).Scan(&productID, &makerID); err != nil {
		t.Fatal(err)
	}
	if ok, err := db.BindOwnerAccount(context.Background(), o.ID, "maker", makerID); err != nil || !ok {
		t.Fatal(err)
	}
	verify := &fixtureStripeVerifier{expectedURL: h.BaseURL + "/v1/webhooks/stripe/" + productID}
	merchant := &MerchantConnectionsHandler{DB: db, Cipher: testConnectionCipher(), Verifier: verify, BaseURL: h.BaseURL, Page: h.Page}
	logs := &bytes.Buffer{}
	prevLog := log.Writer()
	log.SetOutput(logs)
	prevSlog := slog.Default()
	slog.SetDefault(slog.New(slog.NewTextHandler(logs, nil)))
	prevGin, prevGinErr := gin.DefaultWriter, gin.DefaultErrorWriter
	gin.DefaultWriter = logs
	gin.DefaultErrorWriter = logs
	t.Cleanup(func() {
		log.SetOutput(prevLog)
		slog.SetDefault(prevSlog)
		gin.DefaultWriter = prevGin
		gin.DefaultErrorWriter = prevGinErr
	})
	logged := gin.New()
	logged.Use(gin.LoggerWithFormatter(func(p gin.LogFormatterParams) string {
		return fmt.Sprintf("%s %d %s %s %s\n", p.TimeStamp.Format(time.RFC3339), p.StatusCode, p.Method, p.Request.URL.Path, p.Latency)
	}), gin.CustomRecoveryWithWriter(io.Discard, func(c *gin.Context, _ any) {
		log.Print("request panic recovered")
		c.AbortWithStatus(500)
	}))
	group := logged.Group("/owner", h.ownerSession)
	group.GET("/products/:id/connection", merchant.Form)
	group.POST("/products/:id/connection", merchant.Connect)

	suffix := make([]byte, 8)
	if _, err := rand.Read(suffix); err != nil {
		t.Fatal(err)
	}
	secret := "sk_live_TESTSECRET" + hex.EncodeToString(suffix)
	var connectionsBefore int
	if err := db.Pool.QueryRow(context.Background(), `SELECT COUNT(*) FROM product_connections WHERE product_id=$1`, productID).Scan(&connectionsBefore); err != nil {
		t.Fatal(err)
	}
	values := url.Values{"csrf": {o.CSRF}, "stripe_key": {secret}, "price_id": {"price_pilot"}, "endpoint_id": {"we_pilot"}, "signing_secret": {"whsec_verified_pilot_secret"}}
	rec := ownerForm(t, h, logged, cookie, "/owner/products/"+productID+"/connection", values)
	body := rec.Body.String()
	if rec.Code < 400 || rec.Code >= 500 {
		t.Fatalf("expected 4xx, got %d %s", rec.Code, body)
	}
	if strings.HasPrefix(strings.TrimSpace(body), "{") {
		t.Fatalf("live key refusal must render the form, not JSON: %s", body)
	}
	if !strings.Contains(rec.Header().Get("Content-Type"), "text/html") {
		t.Fatalf("content type %s", rec.Header().Get("Content-Type"))
	}
	button := strings.Index(body, "Verify and connect Stripe")
	errAt := strings.Index(body, `id="stripe-live-key"`)
	if button < 0 || errAt < button || !strings.Contains(body, "Live keys are not accepted") {
		t.Fatalf("inline error must sit with the button: %s", body)
	}
	if !strings.Contains(body, `name="stripe_key"`) || strings.Contains(body, "disabled") {
		t.Fatalf("form must stay usable: %s", body)
	}
	if verify.calls != 0 {
		t.Fatal("live key was sent to Stripe")
	}
	if strings.Contains(logs.String(), secret) || strings.Contains(body, secret) {
		t.Fatal("full live key appeared in logs or the response")
	}
	assertSecretAbsent(t, db, secret)
	var connectionsAfter int
	if err := db.Pool.QueryRow(context.Background(), `SELECT COUNT(*) FROM product_connections WHERE product_id=$1`, productID).Scan(&connectionsAfter); err != nil {
		t.Fatal(err)
	}
	if connectionsAfter != connectionsBefore {
		t.Fatalf("live key wrote a connection row: %d -> %d", connectionsBefore, connectionsAfter)
	}
	for _, key := range []string{"rk_live_rejected", "pk_live_rejected"} {
		values.Set("stripe_key", key)
		again := ownerForm(t, h, logged, cookie, "/owner/products/"+productID+"/connection", values)
		if again.Code != http.StatusBadRequest || strings.HasPrefix(strings.TrimSpace(again.Body.String()), "{") || !strings.Contains(again.Body.String(), `id="stripe-live-key"`) {
			t.Fatalf("%s: expected the inline form refusal, got %d %s", key, again.Code, again.Body.String())
		}
	}
	logged.POST("/panic-secret", func(c *gin.Context) { panic(secret) })
	panicRec := httptest.NewRecorder()
	logged.ServeHTTP(panicRec, httptest.NewRequest(http.MethodPost, "/panic-secret", nil))
	if panicRec.Code != http.StatusInternalServerError || !strings.Contains(logs.String(), "request panic recovered") || strings.Contains(logs.String(), secret) {
		t.Fatalf("a panic must not record the live key: %d %s", panicRec.Code, logs.String())
	}
}

func TestStripeConnectionNavIncludesOfferReview(t *testing.T) {
	db, _, _, offerID, _ := testPaidCampaign(t)
	h, r, _ := hostedTestServer(t, db)
	o, cookie := testOwnerSession(t, h, "operator-owner", "operator@example.com")
	var productID, makerID string
	if err := db.Pool.QueryRow(context.Background(), `SELECT p.id,p.maker_id FROM products p JOIN offers o ON o.product_id=p.id WHERE o.id=$1`, offerID).Scan(&productID, &makerID); err != nil {
		t.Fatal(err)
	}
	if ok, err := db.BindOwnerAccount(context.Background(), o.ID, "maker", makerID); err != nil || !ok {
		t.Fatal(err)
	}
	page, err := template.ParseFiles("../../../affilio-wp/admin/build/workspace.html")
	if err != nil {
		t.Fatal(err)
	}
	verify := &fixtureStripeVerifier{}
	merchant := &MerchantConnectionsHandler{DB: db, Cipher: testConnectionCipher(), Verifier: verify, BaseURL: h.BaseURL, Page: page, OperatorID: o.ID}
	h.OperatorID = o.ID
	group := r.Group("/owner", h.ownerSession)
	group.GET("/products/:id/connection", merchant.Form)
	get := httptest.NewRequest(http.MethodGet, "/owner/products/"+productID+"/connection", nil)
	get.AddCookie(cookie)
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, get)
	if rec.Code != http.StatusOK || !strings.Contains(rec.Body.String(), `href="/owner/review"`) || !strings.Contains(rec.Body.String(), "Offer review") {
		t.Fatalf("operator stripe page must include offer review: %d %s", rec.Code, rec.Body.String())
	}
	merchant.OperatorID = ""
	plain := httptest.NewRecorder()
	r.ServeHTTP(plain, get)
	if strings.Contains(plain.Body.String(), "Offer review") {
		t.Fatal("offer review must follow the operator flag, not the page body")
	}
}

func TestEmptyProviderCanBeConnectedAsStripe(t *testing.T) {
	db, _, _, offerID, _ := testPaidCampaign(t)
	h, r, _ := hostedTestServer(t, db)
	o, cookie := testOwnerSession(t, h, "neutral-owner", "neutral@example.com")
	var productID, makerID string
	if err := db.Pool.QueryRow(context.Background(), `SELECT p.id,p.maker_id FROM products p JOIN offers o ON o.product_id=p.id WHERE o.id=$1`, offerID).Scan(&productID, &makerID); err != nil {
		t.Fatal(err)
	}
	if ok, err := db.BindOwnerAccount(context.Background(), o.ID, "maker", makerID); err != nil || !ok {
		t.Fatal(err)
	}
	if _, err := db.Pool.Exec(context.Background(), `UPDATE products SET payment_provider=NULL WHERE id=$1`, productID); err != nil {
		t.Fatal(err)
	}
	verify := &fixtureStripeVerifier{check: services.StripeConnectionCheck{AccountID: "acct_pilot", PriceID: "price_pilot", EndpointID: "we_pilot", Currency: "USD"}, expectedURL: h.BaseURL + "/v1/webhooks/stripe/" + productID}
	merchant := &MerchantConnectionsHandler{DB: db, Cipher: testConnectionCipher(), Verifier: verify, BaseURL: h.BaseURL, Page: h.Page}
	group := r.Group("/owner", h.ownerSession)
	group.POST("/products/:id/connection", merchant.Connect)
	values := url.Values{"csrf": {o.CSRF}, "stripe_key": {"rk_test_verify_once"}, "price_id": {"price_pilot"}, "endpoint_id": {"we_pilot"}, "signing_secret": {"whsec_verified_pilot_secret"}}
	rec := ownerForm(t, h, r, cookie, "/owner/products/"+productID+"/connection", values)
	if rec.Code != http.StatusCreated {
		t.Fatalf("empty provider must still connect when Stripe is chosen: %d %s", rec.Code, rec.Body.String())
	}
	var provider string
	if err := db.Pool.QueryRow(context.Background(), `SELECT COALESCE(payment_provider,'') FROM products WHERE id=$1`, productID).Scan(&provider); err != nil {
		t.Fatal(err)
	}
	if provider != "stripe" {
		t.Fatalf("choosing Stripe must record stripe, got %q", provider)
	}
}

var identPattern = regexp.MustCompile(`^[A-Za-z_][A-Za-z0-9_]*$`)

func assertSecretAbsent(t *testing.T, db *database.DB, secret string) {
	t.Helper()
	rows, err := db.Pool.Query(context.Background(), `
SELECT c.table_name, c.column_name
FROM information_schema.columns c
JOIN information_schema.tables tb ON tb.table_schema=c.table_schema AND tb.table_name=c.table_name
WHERE c.table_schema='public' AND tb.table_type='BASE TABLE'
AND c.data_type IN ('text','character varying','character','json','jsonb')`)
	if err != nil {
		t.Fatal(err)
	}
	defer rows.Close()
	type col struct{ table, column string }
	var cols []col
	for rows.Next() {
		var item col
		if err = rows.Scan(&item.table, &item.column); err != nil {
			t.Fatal(err)
		}
		if !identPattern.MatchString(item.table) || !identPattern.MatchString(item.column) {
			t.Fatalf("unexpected identifier %s.%s", item.table, item.column)
		}
		cols = append(cols, item)
	}
	if err = rows.Err(); err != nil {
		t.Fatal(err)
	}
	for _, item := range cols {
		var found bool
		q := fmt.Sprintf(`SELECT EXISTS (SELECT 1 FROM "%s" WHERE position($1 in COALESCE("%s"::text, '')) > 0)`, item.table, item.column)
		if err = db.Pool.QueryRow(context.Background(), q, secret).Scan(&found); err != nil {
			t.Fatalf("scan %s.%s: %v", item.table, item.column, err)
		}
		if found {
			t.Fatalf("live key stored in %s.%s", item.table, item.column)
		}
	}
}
