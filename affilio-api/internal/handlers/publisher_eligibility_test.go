package handlers

import (
	"context"
	"net/http"
	"testing"

	"affilio-api/internal/database"
	"affilio-api/internal/middleware"

	"github.com/gin-gonic/gin"
)

func TestEligibleOffersDoNotRequireAPaymentProvider(t *testing.T) {
	db, cleanup := setupTestDB(t)
	defer cleanup()
	ctx := context.Background()
	makerID := insertPublisherMaker(t, db)

	noneProduct, noneOffer := insertPublisherOffer(t, db, makerID, "none", "percentage", "USD", "InboxPilot", []string{"email", "inbox", "productivity"})
	_, emptyOffer := insertPublisherOffer(t, db, makerID, "", "percentage", "USD", "Empty Provider", []string{"email"})
	stripeProduct, stripeOffer := insertPublisherOffer(t, db, makerID, "stripe", "percentage", "USD", "Stripe Tool", []string{"email"})
	insertPublisherConnection(t, db, stripeProduct, "USD", true)
	unverifiedProduct, unverifiedOffer := insertPublisherOffer(t, db, makerID, "stripe", "percentage", "USD", "Unverified Tool", []string{"email"})
	insertPublisherConnection(t, db, unverifiedProduct, "USD", false)
	wrongCurrencyProduct, wrongCurrencyOffer := insertPublisherOffer(t, db, makerID, "stripe", "percentage", "USD", "Wrong Currency", []string{"email"})
	insertPublisherConnection(t, db, wrongCurrencyProduct, "EUR", true)
	_, stripeBareOffer := insertPublisherOffer(t, db, makerID, "stripe", "percentage", "USD", "No Connection", []string{"email"})
	_, bountyOffer := insertPublisherOffer(t, db, makerID, "none", "bounty", "USD", "Bounty Tool", []string{"email"})

	verified, err := db.EligibleRevenueOfferIDs(ctx, true)
	if err != nil {
		t.Fatal(err)
	}
	open, err := db.EligibleRevenueOfferIDs(ctx, false)
	if err != nil {
		t.Fatal(err)
	}

	t.Run("provider none is eligible without a connection", func(t *testing.T) {
		if !verified[noneOffer] || !open[noneOffer] {
			t.Fatalf("listed none-provider offer %s (product %s) was not eligible: verified=%v open=%v", noneOffer, noneProduct, verified[noneOffer], open[noneOffer])
		}
	})
	t.Run("empty provider is treated as none", func(t *testing.T) {
		if !verified[emptyOffer] {
			t.Fatal("empty payment_provider was not eligible")
		}
	})
	t.Run("stripe needs an enabled connection in the offer currency", func(t *testing.T) {
		if !verified[stripeOffer] {
			t.Fatal("verified stripe connection in USD was not eligible")
		}
		if verified[stripeBareOffer] || open[stripeBareOffer] {
			t.Fatal("stripe offer without a connection was eligible")
		}
		if verified[wrongCurrencyOffer] || open[wrongCurrencyOffer] {
			t.Fatal("stripe offer with a different connection currency was eligible")
		}
	})
	t.Run("stripe must be verified when required", func(t *testing.T) {
		if verified[unverifiedOffer] {
			t.Fatal("unverified stripe connection was eligible while verification is required")
		}
		if !open[unverifiedOffer] {
			t.Fatal("enabled matching stripe connection was excluded when verification is not required")
		}
	})
	t.Run("commission type stays restricted", func(t *testing.T) {
		if verified[bountyOffer] || open[bountyOffer] {
			t.Fatal("non percentage/flat commission was eligible")
		}
	})
}

func TestPrepareAndCommitProviderlessOffer(t *testing.T) {
	db, cleanup := setupTestDB(t)
	defer cleanup()
	gin.SetMode(gin.TestMode)
	makerID := insertPublisherMaker(t, db)
	_, offerID := insertPublisherOffer(t, db, makerID, "none", "percentage", "USD", "InboxPilot", []string{"email", "inbox", "productivity"})
	siteKey := "site-key-providerless"
	siteID := createTestSite(t, db, siteKey, "publisher.example.com", "Tutorials")
	linkPublisherOwner(t, db, siteID)
	r := publisherLegRouter(db, true)
	contextText := "InboxPilot email inbox productivity tutorial for developers"
	rec, body := doSiteJSON(t, r, http.MethodPost, "/v1/revenue/opportunities", siteKey, map[string]interface{}{"context": contextText})
	if rec.Code != http.StatusOK || body["status"] != "matched" {
		t.Fatalf("providerless offer did not match: %d %v", rec.Code, body)
	}
	opportunities, _ := body["opportunities"].([]interface{})
	if len(opportunities) != 1 || opportunities[0].(map[string]interface{})["offer_id"] != offerID {
		t.Fatalf("matched offers = %v, want %s", body["opportunities"], offerID)
	}
	rec, prepared := doSiteJSON(t, r, http.MethodPost, "/v1/placements", siteKey, map[string]interface{}{
		"offer_id": offerID, "surface_url": "https://publisher.example.com/tutorial", "context": contextText,
		"copy": "Consider InboxPilot for an email inbox productivity workflow.",
	})
	if rec.Code != http.StatusCreated {
		t.Fatalf("prepare providerless offer: %d %v", rec.Code, prepared)
	}
	placementID := prepared["placement"].(map[string]interface{})["id"].(string)
	rec, _ = adminJSON(t, r, http.MethodPost, "/v1/offer-review/placements/"+placementID+"/approve", map[string]interface{}{
		"site_id": siteID, "owner_email": "publisher@example.com", "evidence": "Publisher approved this exact providerless placement.",
	})
	if rec.Code != http.StatusOK {
		t.Fatalf("approve: %d %s", rec.Code, rec.Body.String())
	}
	rec, committed := doSiteJSON(t, r, http.MethodPost, "/v1/placements/"+placementID+"/commit", siteKey, nil)
	if rec.Code != http.StatusOK {
		t.Fatalf("commit providerless offer: %d %v", rec.Code, committed)
	}
	receipt := committed["placement"].(map[string]interface{})["receipt"].(map[string]interface{})
	link, _ := receipt["tracked_link"].(string)
	if len(link) < len("/v1/go/") || link[:len("/v1/go/")] != "/v1/go/" {
		t.Fatalf("tracked link = %q", link)
	}
}

func publisherLegRouter(db *database.DB, verified bool) *gin.Engine {
	r := gin.New()
	h := &PlacementsHandler{DB: db, RequireVerifiedConnection: verified}
	site := r.Group("/v1")
	site.Use(middleware.APIKeyAuth(db, nil))
	site.POST("/revenue/opportunities", h.Find)
	site.POST("/placements", h.Prepare)
	site.POST("/placements/:id/commit", h.Commit)
	site.POST("/placements/:id/receipt", h.RecordReceipt)
	admin := r.Group("/v1/offer-review")
	admin.Use(middleware.AdminKeyAuth("admin-secret", nil))
	admin.POST("/placements/:id/approve", h.Approve)
	return r
}

func insertPublisherMaker(t *testing.T, db *database.DB) string {
	t.Helper()
	var id string
	err := db.Pool.QueryRow(context.Background(), `INSERT INTO makers(email, name, api_key) VALUES('maker-elig@example.com','Eligibility Maker','maker-elig-key') RETURNING id`).Scan(&id)
	if err != nil {
		t.Fatal(err)
	}
	return id
}

func insertPublisherOffer(t *testing.T, db *database.DB, makerID, provider, commissionType, currency, name string, keywords []string) (string, string) {
	t.Helper()
	var productID, offerID string
	err := db.Pool.QueryRow(context.Background(), `INSERT INTO products(maker_id, name, website_url, product_type, payment_provider, keywords, tagline, primary_category)
VALUES ($1,$2,'https://inboxpilot.example.com','saas', NULLIF($3,''), $4, 'email inbox tool', 'productivity') RETURNING id`, makerID, name, provider, keywords).Scan(&productID)
	if err != nil {
		t.Fatal(err)
	}
	err = db.Pool.QueryRow(context.Background(), `INSERT INTO offers(product_id, status, commission_type, commission_value, commission_currency, landing_url, refund_window_days)
VALUES ($1,'listed',$2,30,$3,'https://inboxpilot.example.com',30) RETURNING id`, productID, commissionType, currency).Scan(&offerID)
	if err != nil {
		t.Fatal(err)
	}
	return productID, offerID
}

func insertPublisherConnection(t *testing.T, db *database.DB, productID, currency string, verified bool) {
	t.Helper()
	_, err := db.Pool.Exec(context.Background(), `INSERT INTO product_connections(product_id, secret_ciphertext, live_mode, currency, enabled, configuration_verified_at)
VALUES ($1, '\x00', false, $2, true, CASE WHEN $3 THEN NOW() ELSE NULL END)`, productID, currency, verified)
	if err != nil {
		t.Fatal(err)
	}
}

func linkPublisherOwner(t *testing.T, db *database.DB, siteID string) {
	t.Helper()
	_, err := db.Pool.Exec(context.Background(), `INSERT INTO owners(issuer, subject, email, name, site_id)
VALUES ('https://issuer.test', $1, 'publisher@example.com', 'Publisher', $2)`, "subject-"+siteID, siteID)
	if err != nil {
		t.Fatal(err)
	}
}
