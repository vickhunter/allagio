package handlers

import (
	"context"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"

	"affilio-api/internal/services"
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
