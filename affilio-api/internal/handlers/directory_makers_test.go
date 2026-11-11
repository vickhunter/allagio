package handlers

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
)

func TestDirectoryMakersEmptyWhenDatabaseAnswers(t *testing.T) {
	db, cleanup := setupTestDB(t)
	defer cleanup()
	gin.SetMode(gin.TestMode)
	r := gin.New()
	(&HostedHandler{DB: db}).Mount(r)

	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/v1/directory/makers", nil))
	if rec.Code != http.StatusOK {
		t.Fatalf("status %d %s", rec.Code, rec.Body.String())
	}
	var body map[string]any
	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
		t.Fatal(err)
	}
	items, ok := body["items"].([]any)
	if !ok || len(items) != 0 || body["next_cursor"] != nil || body["total"] != float64(0) {
		t.Fatalf("empty list %#v", body)
	}
	if _, listed := body["error"]; listed {
		t.Fatalf("healthy read named an error %#v", body)
	}
}

func TestDirectoryMakersListsListedParties(t *testing.T) {
	db, cleanup := setupTestDB(t)
	defer cleanup()
	gin.SetMode(gin.TestMode)
	r := gin.New()
	(&HostedHandler{DB: db}).Mount(r)

	var ownerID string
	if err := db.Pool.QueryRow(context.Background(),
		`INSERT INTO owners(issuer,subject,email,name,source) VALUES('agent','subj-a','a@example.com','Agent','direct') RETURNING id`).Scan(&ownerID); err != nil {
		t.Fatal(err)
	}
	terms := `{"title":"Shoes deal","disclosure":{"label":"Affiliate link"}}`
	if _, err := db.Pool.Exec(context.Background(), `
INSERT INTO orp_offers(id, owner_id, party, terms, terms_hash, deal_limit, created_at, listed)
VALUES('off_dir1', $1::uuid, 'orp:example.test/' || $2, $3, 'sha256-x', 3, NOW(), true)`,
		ownerID, ownerID, terms); err != nil {
		t.Fatal(err)
	}
	if _, err := db.Pool.Exec(context.Background(), `
INSERT INTO orp_offers(id, owner_id, party, terms, terms_hash, deal_limit, created_at, listed)
VALUES('off_dir2', $1::uuid, 'orp:example.test/' || $2, $3, 'sha256-y', 3, NOW(), false)`,
		ownerID, ownerID, `{"title":"Unlisted deal"}`); err != nil {
		t.Fatal(err)
	}

	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/v1/directory/makers", nil))
	if rec.Code != http.StatusOK {
		t.Fatalf("status %d %s", rec.Code, rec.Body.String())
	}
	var body map[string]any
	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
		t.Fatal(err)
	}
	items, _ := body["items"].([]any)
	if len(items) != 1 || body["total"] != float64(1) {
		t.Fatalf("items %#v total %v", items, body["total"])
	}
	entry, _ := items[0].(map[string]any)
	if entry["party"] != "orp:example.test/"+ownerID || entry["offer_count"] != float64(1) {
		t.Fatalf("entry %#v", entry)
	}
	titles, _ := entry["titles"].([]any)
	if len(titles) != 1 || titles[0] != "Shoes deal" {
		t.Fatalf("titles %#v", titles)
	}
}

func TestDirectoryMakersClosedPoolIsUnavailable(t *testing.T) {
	db, cleanup := setupTestDB(t)
	cleanup()
	gin.SetMode(gin.TestMode)
	r := gin.New()
	(&HostedHandler{DB: db}).Mount(r)

	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/v1/directory/makers", nil))
	if rec.Code != http.StatusServiceUnavailable || rec.Body.String() != `{"error":"directory unavailable"}` {
		t.Fatalf("closed pool %d %s", rec.Code, rec.Body.String())
	}
}
