package handlers

import (
	"context"
	"net/http"
	"testing"

	"github.com/gin-gonic/gin"
)

func TestPrepareAndCommitRequireALinkedPublisher(t *testing.T) {
	db, cleanup := setupTestDB(t)
	defer cleanup()
	gin.SetMode(gin.TestMode)
	makerID := insertPublisherMaker(t, db)
	_, offerID := insertPublisherOffer(t, db, makerID, "none", "percentage", "USD", "InboxPilot", []string{"email", "inbox", "productivity"})
	siteKey := "site-key-unlinked"
	siteID := createTestSite(t, db, siteKey, "publisher.example.com", "Tutorials")
	r := publisherLegRouter(db, true)
	payload := map[string]interface{}{
		"offer_id": offerID, "surface_url": "https://publisher.example.com/tutorial",
		"context": "InboxPilot email inbox productivity tutorial for developers",
		"copy":    "Consider InboxPilot for an email inbox productivity workflow.",
	}

	rec, body := doSiteJSON(t, r, http.MethodPost, "/v1/placements", siteKey, payload)
	if rec.Code != http.StatusForbidden {
		t.Fatalf("unlinked prepare: %d %v", rec.Code, body)
	}

	linkPublisherOwner(t, db, siteID)
	rec, prepared := doSiteJSON(t, r, http.MethodPost, "/v1/placements", siteKey, payload)
	if rec.Code != http.StatusCreated {
		t.Fatalf("linked prepare: %d %v", rec.Code, prepared)
	}
	placementID := prepared["placement"].(map[string]interface{})["id"].(string)
	rec, _ = adminJSON(t, r, http.MethodPost, "/v1/offer-review/placements/"+placementID+"/approve", map[string]interface{}{
		"site_id": siteID, "owner_email": "publisher@example.com", "evidence": "Publisher approved this exact placement.",
	})
	if rec.Code != http.StatusOK {
		t.Fatalf("approve: %d %s", rec.Code, rec.Body.String())
	}

	if _, err := db.Pool.Exec(context.Background(), `DELETE FROM owners WHERE site_id=$1`, siteID); err != nil {
		t.Fatal(err)
	}
	rec, body = doSiteJSON(t, r, http.MethodPost, "/v1/placements/"+placementID+"/commit", siteKey, nil)
	if rec.Code != http.StatusForbidden {
		t.Fatalf("unlinked commit: %d %v", rec.Code, body)
	}

	linkPublisherOwner(t, db, siteID)
	rec, committed := doSiteJSON(t, r, http.MethodPost, "/v1/placements/"+placementID+"/commit", siteKey, nil)
	if rec.Code != http.StatusOK {
		t.Fatalf("linked commit: %d %v", rec.Code, committed)
	}
}
