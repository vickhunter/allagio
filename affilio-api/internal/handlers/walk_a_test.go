package handlers

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"
)

func TestSurfaceURLProblem(t *testing.T) {
	if problem := surfaceURLProblem("http://[::1]/post", "::1"); problem != "" {
		t.Fatal(problem)
	}
	if problem := surfaceURLProblem("http://localhost/post", "localhost"); problem != "" {
		t.Fatal(problem)
	}
	if problem := surfaceURLProblem("http://blog.localhost/post", "blog.localhost"); problem != "" {
		t.Fatal(problem)
	}
	if problem := surfaceURLProblem("https://publisher.example.com/post", "publisher.example.com"); problem != "" {
		t.Fatal(problem)
	}
	httpPublic := surfaceURLProblem("http://publisher.example.com/post", "publisher.example.com")
	if !strings.Contains(httpPublic, "HTTPS") || !strings.Contains(httpPublic, "127.0.0.1") {
		t.Fatalf("public http error = %q", httpPublic)
	}
	mismatch := surfaceURLProblem("https://other.example/post", "publisher.example.com")
	if !strings.Contains(mismatch, "registered publisher domain") {
		t.Fatalf("hostname mismatch = %q", mismatch)
	}
	if surfaceURLProblem("http://evil.example/post", "127.0.0.1") == "" {
		t.Fatal("hostname mismatch was accepted")
	}
}

func TestPrepareSurfaceURLAllowsLocalHTTP(t *testing.T) {
	db, cleanup := setupTestDB(t)
	defer cleanup()
	gin.SetMode(gin.TestMode)
	makerID := insertPublisherMaker(t, db)
	_, offerID := insertPublisherOffer(t, db, makerID, "none", "percentage", "USD", "InboxPilot", []string{"email", "inbox", "productivity"})
	r := publisherLegRouter(db, true)
	contextText := "InboxPilot email inbox productivity tutorial for developers"
	copy := "Consider InboxPilot for an email inbox productivity workflow."

	localID := createTestSite(t, db, "local-publisher-key", "127.0.0.1", "Local")
	linkPublisherOwner(t, db, localID)
	rec, prepared := doSiteJSON(t, r, http.MethodPost, "/v1/placements", "local-publisher-key", map[string]interface{}{
		"offer_id": offerID, "surface_url": "http://127.0.0.1:8088/tutorial", "context": contextText, "copy": copy,
	})
	if rec.Code != http.StatusCreated {
		t.Fatalf("local http prepare: %d %v", rec.Code, prepared)
	}

	publicID := createTestSite(t, db, "public-publisher-key", "publisher.example.com", "Public")
	linkPublisherOwner(t, db, publicID)
	rec, rejected := doSiteJSON(t, r, http.MethodPost, "/v1/placements", "public-publisher-key", map[string]interface{}{
		"offer_id": offerID, "surface_url": "http://publisher.example.com/tutorial", "context": contextText, "copy": copy,
	})
	errText, _ := rejected["error"].(string)
	if rec.Code != http.StatusBadRequest || !strings.Contains(errText, "HTTPS") || !strings.Contains(errText, "127.0.0.1") {
		t.Fatalf("public http: %d %q", rec.Code, errText)
	}
	rec, mismatch := doSiteJSON(t, r, http.MethodPost, "/v1/placements", "public-publisher-key", map[string]interface{}{
		"offer_id": offerID, "surface_url": "https://other.example/tutorial", "context": contextText, "copy": copy,
	})
	errText, _ = mismatch["error"].(string)
	if rec.Code != http.StatusBadRequest || !strings.Contains(errText, "registered publisher domain") {
		t.Fatalf("hostname mismatch: %d %q", rec.Code, errText)
	}
}

func TestOwnerFindURLOptionalAndShortTopic(t *testing.T) {
	db, cleanup := setupTestDB(t)
	defer cleanup()
	h, r, _ := hostedTestServer(t, db)
	o, cookie := testOwnerSession(t, h, "find-owner", "find-owner@example.com")
	siteID := createTestSite(t, db, "find-owner-key", "publisher.example.com", "Tutorials")
	if _, err := db.Pool.Exec(context.Background(), `UPDATE owners SET site_id=$2 WHERE id=$1`, o.ID, siteID); err != nil {
		t.Fatal(err)
	}
	makerID := insertPublisherMaker(t, db)
	insertPublisherOffer(t, db, makerID, "none", "percentage", "USD", "InboxPilot", []string{"email", "inbox", "productivity"})

	filled := "Gardening tips for tomatoes in small gardens"
	for _, payload := range []map[string]any{{"context": filled}, {"context": filled, "surface_url": ""}} {
		rec := ownerJSON(t, h, r, cookie, o.CSRF, http.MethodPost, "/owner/api/opportunities", payload)
		if rec.Code != http.StatusOK {
			t.Fatalf("find without article URL: %d %s", rec.Code, rec.Body.String())
		}
		var body map[string]any
		if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
			t.Fatal(err)
		}
		if body["reason"] != "below_relevance" {
			t.Fatalf("short topic reason = %v", body["reason"])
		}
		if body["best_score"] == nil || body["threshold"] != float64(relevanceThreshold) {
			t.Fatalf("short topic did not return a score: %s", rec.Body.String())
		}
	}
	short := ownerJSON(t, h, r, cookie, o.CSRF, http.MethodPost, "/owner/api/opportunities", map[string]any{
		"context": "hi", "surface_url": "https://publisher.example.com/post",
	})
	if short.Code != http.StatusBadRequest || !strings.Contains(short.Body.String(), "context") {
		t.Fatalf("short context: %d %s", short.Code, short.Body.String())
	}
}

func TestOwnerArticleTextFetch(t *testing.T) {
	db, cleanup := setupTestDB(t)
	defer cleanup()
	article := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		io.WriteString(w, `<html><script>alert(1)</script><style>body{color:red}</style><p>InboxPilot email inbox productivity tutorial for developers and writers.</p></html>`)
	}))
	defer article.Close()
	h, r, _ := hostedTestServer(t, db)
	o, cookie := testOwnerSession(t, h, "fetch-owner", "fetch-owner@example.com")
	siteID := createTestSite(t, db, "fetch-owner-key", "127.0.0.1", "Local")
	if _, err := db.Pool.Exec(context.Background(), `UPDATE owners SET site_id=$2 WHERE id=$1`, o.ID, siteID); err != nil {
		t.Fatal(err)
	}
	rec := ownerJSON(t, h, r, cookie, o.CSRF, http.MethodPost, "/owner/api/article-text", map[string]any{"surface_url": article.URL + "/post"})
	if rec.Code != http.StatusOK {
		t.Fatalf("article text: %d %s", rec.Code, rec.Body.String())
	}
	var body map[string]string
	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(body["text"], "InboxPilot") || strings.Contains(body["text"], "alert") {
		t.Fatalf("fetched text = %q", body["text"])
	}
}
