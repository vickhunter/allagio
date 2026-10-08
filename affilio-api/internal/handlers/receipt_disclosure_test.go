package handlers

import (
	"net/http"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"
)

func TestReceiptAcceptsADisclosureThePostAlreadyContains(t *testing.T) {
	db, cleanup := setupTestDB(t)
	defer cleanup()
	gin.SetMode(gin.TestMode)
	makerID := insertPublisherMaker(t, db)
	_, offerID := insertPublisherOffer(t, db, makerID, "none", "percentage", "USD", "InboxPilot", []string{"email", "inbox", "productivity"})
	siteKey := "site-key-receipt-disclosure"
	siteID := createTestSite(t, db, siteKey, "publisher.example.com", "Tutorials")
	linkPublisherOwner(t, db, siteID)
	r := publisherLegRouter(db, true)
	contextText := "InboxPilot email inbox productivity tutorial for developers"
	copy := "Consider InboxPilot for an email inbox productivity workflow."
	rec, prepared := doSiteJSON(t, r, http.MethodPost, "/v1/placements", siteKey, map[string]interface{}{
		"offer_id": offerID, "surface_url": "https://publisher.example.com/tutorial", "context": contextText, "copy": copy,
	})
	if rec.Code != http.StatusCreated {
		t.Fatalf("prepare: %d %v", rec.Code, prepared)
	}
	placement := prepared["placement"].(map[string]interface{})
	placementID := placement["id"].(string)
	disclosure := placement["disclosure"].(string)
	rec, _ = adminJSON(t, r, http.MethodPost, "/v1/offer-review/placements/"+placementID+"/approve", map[string]interface{}{
		"site_id": siteID, "owner_email": "publisher@example.com", "evidence": "Publisher approved this exact providerless placement.",
	})
	if rec.Code != http.StatusOK {
		t.Fatalf("approve: %d %s", rec.Code, rec.Body.String())
	}
	rec, committed := doSiteJSON(t, r, http.MethodPost, "/v1/placements/"+placementID+"/commit", siteKey, nil)
	if rec.Code != http.StatusOK {
		t.Fatalf("commit: %d %v", rec.Code, committed)
	}
	tracked := committed["placement"].(map[string]interface{})["receipt"].(map[string]interface{})["tracked_link"].(string)
	fixture := `<p><em>Nota: alcuni link in questa pagina sono affiliati. Se acquisti tramite questi link, posso ricevere una commissione, senza costi extra per te.</em></p>`
	if strings.Contains(fixture, disclosure) {
		t.Fatal("fixture contains the API disclosure, so it does not prove a post disclosure is enough")
	}
	post := func(published string) (int, map[string]interface{}) {
		t.Helper()
		response, body := doSiteJSON(t, r, http.MethodPost, "/v1/placements/"+placementID+"/receipt", siteKey, map[string]interface{}{
			"surface_url": "https://publisher.example.com/tutorial", "published_text": published, "evidence": "Published on the tutorial with the post disclosure.",
		})
		return response.Code, body
	}

	code, body := post(copy + "\n" + tracked)
	if code != http.StatusConflict {
		t.Fatalf("copy and tracked link with no disclosure: %d %v", code, body)
	}
	code, body = post(copy + "\n" + fixture)
	if code != http.StatusConflict {
		t.Fatalf("disclosure without the tracked link: %d %v", code, body)
	}
	code, body = post(fixture + "\n" + copy + "\n" + tracked)
	if code != http.StatusOK {
		t.Fatalf("post disclosure without the API disclosure: %d %v", code, body)
	}
	if body["status"] != "publisher_reported" {
		t.Fatalf("receipt status = %v", body["status"])
	}
	if _, present := body["error"]; present {
		t.Fatalf("accepted receipt returned an error: %v", body)
	}
}
