package handlers

import (
	"net/http"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"
)

func TestNoMatchExplainsEligibilityAndRelevance(t *testing.T) {
	db, cleanup := setupTestDB(t)
	defer cleanup()
	gin.SetMode(gin.TestMode)
	makerID := insertPublisherMaker(t, db)
	siteKey := "site-key-nomatch"
	siteID := createTestSite(t, db, siteKey, "publisher.example.com", "Tutorials")
	linkPublisherOwner(t, db, siteID)
	r := publisherLegRouter(db, true)

	find := func(contextText string) map[string]interface{} {
		t.Helper()
		rec, body := doSiteJSON(t, r, http.MethodPost, "/v1/revenue/opportunities", siteKey, map[string]interface{}{"context": contextText})
		if rec.Code != http.StatusOK {
			t.Fatalf("find: %d %v", rec.Code, body)
		}
		return body
	}

	t.Run("no eligible offers", func(t *testing.T) {
		_, bare := insertPublisherOffer(t, db, makerID, "stripe", "percentage", "USD", "Offer Rails Demo", []string{"rails", "offer"})
		_ = bare
		body := find("WordPress post about Offer Rails Demo pricing and a thirty percent referral.")
		if body["status"] != "no_match" || body["reason"] != "no_eligible_offers" {
			t.Fatalf("status=%v reason=%v body=%v", body["status"], body["reason"], body)
		}
		if _, ok := body["best_score"]; ok {
			t.Fatalf("no_eligible_offers included best_score: %v", body["best_score"])
		}
	})

	t.Run("eligible offers score under 25", func(t *testing.T) {
		_, _ = insertPublisherOffer(t, db, makerID, "none", "percentage", "USD", "InboxPilot", []string{"email", "inbox", "productivity"})
		body := find("A tutorial on gardening with tomatoes and soil")
		if body["status"] != "no_match" || body["reason"] != "below_relevance" {
			t.Fatalf("status=%v reason=%v body=%v", body["status"], body["reason"], body)
		}
		score, ok := body["best_score"].(float64)
		threshold, tok := body["threshold"].(float64)
		if !ok || !tok || threshold != 25 || score < 0 || score >= 25 {
			t.Fatalf("best_score=%v threshold=%v", body["best_score"], body["threshold"])
		}
	})

	t.Run("a relevant offer stays matched without a reason", func(t *testing.T) {
		body := find("InboxPilot email inbox productivity tutorial for developers")
		if body["status"] != "matched" {
			t.Fatalf("expected a match: %v", body)
		}
		if _, ok := body["reason"]; ok {
			t.Fatalf("matched response included reason %v", body["reason"])
		}
	})
}

func TestStagingPost2615StaysRelevant(t *testing.T) {
	db, cleanup := setupTestDB(t)
	defer cleanup()
	gin.SetMode(gin.TestMode)
	makerID := insertPublisherMaker(t, db)
	_, _ = insertPublisherOffer(t, db, makerID, "none", "percentage", "USD", "InboxPilot", []string{"email", "inbox", "productivity", "il", "lo", "la", "gli", "le", "una", "per", "con", "che", "non", "come", "del", "della", "questo", "quello", "sono", "tuo", "tua", "nel", "nella", "anche", "Vedi&nbsp;il&nbsp;prodotto"})
	siteKey := "site-key-2615"
	siteID := createTestSite(t, db, siteKey, "publisher.example.com", "Tutorials")
	linkPublisherOwner(t, db, siteID)
	r := publisherLegRouter(db, true)
	const stagingPost2615 = "InboxPilot email inbox productivity tool. Staging post 2615."

	rec, body := doSiteJSON(t, r, http.MethodPost, "/v1/revenue/opportunities", siteKey, map[string]interface{}{"context": stagingPost2615})
	if rec.Code != http.StatusOK || body["status"] != "matched" {
		t.Fatalf("staging post 2615: %d %v", rec.Code, body)
	}
	rec, garden := doSiteJSON(t, r, http.MethodPost, "/v1/revenue/opportunities", siteKey, map[string]interface{}{"context": "A tutorial on gardening with tomatoes and soil"})
	if rec.Code != http.StatusOK || garden["status"] != "no_match" || garden["reason"] != "below_relevance" {
		t.Fatalf("gardening: %d %v", rec.Code, garden)
	}
	threshold, _ := garden["threshold"].(float64)
	score, _ := garden["best_score"].(float64)
	if threshold != 25 || score >= 25 {
		t.Fatalf("cutoff moved: best_score=%v threshold=%v", garden["best_score"], garden["threshold"])
	}
}

func TestSelfReferralStaysBlocked(t *testing.T) {
	db, cleanup := setupTestDB(t)
	defer cleanup()
	gin.SetMode(gin.TestMode)
	makerID := insertPublisherMaker(t, db)
	_, offerID := insertPublisherOffer(t, db, makerID, "none", "percentage", "USD", "InboxPilot", []string{"email", "inbox", "productivity"})
	siteKey := "site-key-self"
	siteID := createTestSite(t, db, siteKey, "publisher.example.com", "Tutorials")
	linkPublisherOwner(t, db, siteID)
	r := publisherLegRouter(db, true)
	rec, prepared := doSiteJSON(t, r, http.MethodPost, "/v1/placements", siteKey, map[string]interface{}{
		"offer_id": offerID, "surface_url": "https://publisher.example.com/tutorial", "context": "InboxPilot email inbox productivity tutorial for developers",
		"copy": "Consider InboxPilot for an email inbox productivity workflow.",
	})
	if rec.Code != http.StatusCreated {
		t.Fatalf("prepare: %d %s", rec.Code, rec.Body.String())
	}
	placementID := prepared["placement"].(map[string]interface{})["id"].(string)
	rec, _ = adminJSON(t, r, http.MethodPost, "/v1/offer-review/placements/"+placementID+"/approve", map[string]interface{}{
		"site_id": siteID, "owner_email": "maker-elig@example.com", "evidence": "The maker tried to approve their own placement.",
	})
	if rec.Code != http.StatusConflict || !strings.Contains(rec.Body.String(), "self referral") {
		t.Fatalf("self referral: %d %s", rec.Code, rec.Body.String())
	}
}
