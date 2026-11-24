package handlers

import (
	"context"
	"crypto/ed25519"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"sync"
	"testing"
	"time"

	"affilio-api/internal/middleware"
)

func TestProductListingToggleHidesExactName(t *testing.T) {
	db, cleanup := setupTestDB(t)
	defer cleanup()
	entry := &AgentEntry{
		DB: db, BaseURL: "http://127.0.0.1:8080", AddressKey: testAddressKey,
		ClaimsCheck: "stub", TokenLimiter: NewIPRateLimiter(30, time.Minute),
	}
	host := &ORPHost{Entry: entry}
	r := mountORP(entry, host)
	maker := r.Group("/v1/maker", middleware.MakerKeyAuth(db, entry.mcpResource(nil)))
	maker.POST("/products", (&ProductsHandler{DB: db}).Create)
	h := &HostedHandler{DB: db, BaseURL: entry.BaseURL}
	owner := r.Group("/owner", h.ownerSession)
	owner.GET("", h.Dashboard)
	owner.POST("/products/:id/listed", h.SetProductListed)

	rec, joined := postJSON(t, r, http.MethodPost, "/orp/join", map[string]any{"agent": map[string]any{"name": "Toggle Agent"}}, nil)
	if rec.Code != http.StatusCreated {
		t.Fatalf("join %d %s", rec.Code, rec.Body.String())
	}
	token, _ := joined["token"].(string)
	party, _ := joined["party"].(string)
	ownerID := party[strings.LastIndex(party, "/")+1:]
	created, product := postJSON(t, r, http.MethodPost, "/v1/maker/products", map[string]any{
		"name": "Toggle Exact Name", "website_url": "https://toggle.example",
	}, bearerHeader(token))
	if created.Code != http.StatusOK {
		t.Fatalf("product %d %s", created.Code, created.Body.String())
	}
	productID, _ := product["id"].(string)
	if productID == "" {
		t.Fatalf("product body %s", created.Body.String())
	}
	if names := suggestProductNames(t, r, "toggle exact name"); len(names) != 1 || names[0] != "Toggle Exact Name" {
		t.Fatalf("default listed search %#v", names)
	}

	cookie, csrf := sessionForOwner(t, db, ownerID, entry.BaseURL)
	page := httptest.NewRequest(http.MethodGet, "/owner", nil)
	page.AddCookie(cookie)
	pageRec := httptest.NewRecorder()
	r.ServeHTTP(pageRec, page)
	if pageRec.Code != http.StatusOK || !strings.Contains(pageRec.Body.String(), `action="/owner/products/`+productID+`/listed"`) || !strings.Contains(pageRec.Body.String(), `name="listed" value="on" checked`) {
		t.Fatalf("owner page %d %s", pageRec.Code, pageRec.Body.String())
	}

	off := ownerForm(t, h, r, cookie, "/owner/products/"+productID+"/listed", url.Values{"csrf": {csrf}})
	if off.Code != http.StatusSeeOther {
		t.Fatalf("unlist %d %s", off.Code, off.Body.String())
	}
	if names := suggestProductNames(t, r, "Toggle Exact Name"); len(names) != 0 {
		t.Fatalf("unlisted exact search %#v", names)
	}

	on := ownerForm(t, h, r, cookie, "/owner/products/"+productID+"/listed", url.Values{"csrf": {csrf}, "listed": {"on"}})
	if on.Code != http.StatusSeeOther {
		t.Fatalf("relist %d %s", on.Code, on.Body.String())
	}
	if names := suggestProductNames(t, r, "Toggle Exact Name"); len(names) != 1 || names[0] != "Toggle Exact Name" {
		t.Fatalf("relisted exact search %#v", names)
	}

	second, secondBody := postJSON(t, r, http.MethodPost, "/orp/join", map[string]any{"agent": map[string]any{"name": "Other Toggle Agent"}}, nil)
	if second.Code != http.StatusCreated {
		t.Fatalf("other join %d %s", second.Code, second.Body.String())
	}
	secondToken, _ := secondBody["token"].(string)
	secondParty, _ := secondBody["party"].(string)
	secondOwner := secondParty[strings.LastIndex(secondParty, "/")+1:]
	secondProduct, _ := postJSON(t, r, http.MethodPost, "/v1/maker/products", map[string]any{
		"name": "Other Product", "website_url": "https://other.example",
	}, bearerHeader(secondToken))
	if secondProduct.Code != http.StatusOK {
		t.Fatalf("other product %d %s", secondProduct.Code, secondProduct.Body.String())
	}
	otherCookie, otherCSRF := sessionForOwner(t, db, secondOwner, entry.BaseURL)
	foreign := ownerForm(t, h, r, otherCookie, "/owner/products/"+productID+"/listed", url.Values{"csrf": {otherCSRF}, "listed": {"on"}})
	if foreign.Code != http.StatusNotFound && foreign.Code != http.StatusForbidden {
		t.Fatalf("other owner %d %s", foreign.Code, foreign.Body.String())
	}
	if countRows(t, db, `SELECT COUNT(*) FROM products WHERE id=$1 AND listed`, productID) != 1 {
		t.Fatal("other owner changed listing")
	}
}

func TestJoinKeyScopeAndReuseWithoutKey(t *testing.T) {
	db, cleanup := setupTestDB(t)
	defer cleanup()
	entry := &AgentEntry{
		DB: db, BaseURL: "http://127.0.0.1:8080", AddressKey: testAddressKey,
		ClaimsCheck: "stub", TokenLimiter: NewIPRateLimiter(30, time.Minute),
	}
	r := mountORP(entry, &ORPHost{Entry: entry})

	account, accountBody := postJSON(t, r, http.MethodPost, "/v1/agent-accounts", map[string]any{"role": "maker", "name": "Scope Maker", "email": "scope-maker@example.com"}, nil)
	if account.Code != http.StatusCreated {
		t.Fatalf("account %d %s", account.Code, account.Body.String())
	}
	apiKey, _ := accountBody["api_key"].(string)
	if apiKey == "" {
		t.Fatalf("account body %s", account.Body.String())
	}
	ownersBefore := countRows(t, db, `SELECT COUNT(*) FROM owners`)
	keysBefore := countRows(t, db, `SELECT COUNT(*) FROM party_keys`)
	pub, _, err := ed25519.GenerateKey(nil)
	if err != nil {
		t.Fatal(err)
	}
	body := joinKeyBody(t, pub, "scopekey")
	rec := postRaw(t, r, "/orp/join", body, bearerHeader(apiKey))
	if rec.Code != http.StatusForbidden || rec.Header().Get("Content-Type") != "application/problem+json" {
		t.Fatalf("scope %d %s %s", rec.Code, rec.Header().Get("Content-Type"), rec.Body.String())
	}
	var doc map[string]any
	if err := json.Unmarshal(rec.Body.Bytes(), &doc); err != nil {
		t.Fatal(err)
	}
	if doc["status"] != float64(http.StatusForbidden) || !problemHasCode(doc, "join_key_scope") || !problemHasField(doc, "Authorization") {
		t.Fatalf("problem %#v", doc)
	}
	detail, _ := doc["detail"].(string)
	if !strings.Contains(detail, "Authorization") || !strings.Contains(detail, "join_key_scope") {
		t.Fatalf("detail %q", detail)
	}
	if countRows(t, db, `SELECT COUNT(*) FROM owners`) != ownersBefore || countRows(t, db, `SELECT COUNT(*) FROM party_keys`) != keysBefore {
		t.Fatal("other scope minted a party or stored a key")
	}

	joined, joinBody := postJSON(t, r, http.MethodPost, "/orp/join", map[string]any{"agent": map[string]any{"name": "Scope Agent"}}, nil)
	if joined.Code != http.StatusCreated {
		t.Fatalf("join %d %s", joined.Code, joined.Body.String())
	}
	token, _ := joinBody["token"].(string)
	party, _ := joinBody["party"].(string)
	again, againBody := postJSON(t, r, http.MethodPost, "/orp/join", map[string]any{"agent": map[string]any{"name": "Scope Agent"}}, bearerHeader(token))
	if again.Code != http.StatusOK || againBody["token"] != token || againBody["party"] != party {
		t.Fatalf("reuse without key %d %#v", again.Code, againBody)
	}
}

func TestFirstKeyBindIsAtomic(t *testing.T) {
	db, cleanup := setupTestDB(t)
	defer cleanup()
	entry := &AgentEntry{
		DB: db, BaseURL: "http://127.0.0.1:8080", AddressKey: testAddressKey,
		ClaimsCheck: "stub", TokenLimiter: NewIPRateLimiter(30, time.Minute),
	}
	r := mountORP(entry, &ORPHost{Entry: entry})
	joined, joinBody := postJSON(t, r, http.MethodPost, "/orp/join", map[string]any{"agent": map[string]any{"name": "Race Agent"}}, nil)
	if joined.Code != http.StatusCreated {
		t.Fatalf("join %d %s", joined.Code, joined.Body.String())
	}
	token, _ := joinBody["token"].(string)
	party, _ := joinBody["party"].(string)
	ownerID := party[strings.LastIndex(party, "/")+1:]

	const n = 8
	start := make(chan struct{})
	var wg sync.WaitGroup
	codes := make([]int, n)
	bodies := make([]string, n)
	for i := 0; i < n; i++ {
		pub, _, err := ed25519.GenerateKey(nil)
		if err != nil {
			t.Fatal(err)
		}
		raw := []byte(joinKeyBody(t, pub, fmt.Sprintf("racekey%d", i)))
		wg.Add(1)
		go func(i int, raw []byte) {
			defer wg.Done()
			<-start
			req := httptest.NewRequest(http.MethodPost, "/orp/join", strings.NewReader(string(raw)))
			req.Header.Set("Content-Type", "application/json")
			req.Header.Set("Authorization", "Bearer "+token)
			rec := httptest.NewRecorder()
			r.ServeHTTP(rec, req)
			codes[i] = rec.Code
			bodies[i] = rec.Body.String()
		}(i, raw)
	}
	close(start)
	wg.Wait()

	wins := 0
	for i := 0; i < n; i++ {
		switch codes[i] {
		case http.StatusOK:
			wins++
		case http.StatusBadRequest:
			var doc map[string]any
			if err := json.Unmarshal([]byte(bodies[i]), &doc); err != nil || !problemHasCode(doc, "join_key_held") {
				t.Fatalf("loser %d %s", codes[i], bodies[i])
			}
		default:
			t.Fatalf("racer %d %s", codes[i], bodies[i])
		}
	}
	if wins != 1 {
		t.Fatalf("wins %d codes %v", wins, codes)
	}
	if countRows(t, db, `SELECT COUNT(*) FROM party_keys WHERE owner_id=$1 AND revoked_at IS NULL`, ownerID) != 1 {
		t.Fatal("party_keys does not have one active row")
	}
}

func TestExactNamePastProductCap(t *testing.T) {
	db, cleanup := setupTestDB(t)
	defer cleanup()
	entry := &AgentEntry{
		DB: db, BaseURL: "http://127.0.0.1:8080", AddressKey: testAddressKey,
		ClaimsCheck: "stub", TokenLimiter: NewIPRateLimiter(30, time.Minute),
	}
	host := &ORPHost{Entry: entry}
	r := mountORP(entry, host)
	maker := r.Group("/v1/maker", middleware.MakerKeyAuth(db, entry.mcpResource(nil)))
	maker.POST("/products", (&ProductsHandler{DB: db}).Create)

	rec, joined := postJSON(t, r, http.MethodPost, "/orp/join", map[string]any{"agent": map[string]any{"name": "Catalog Agent"}}, nil)
	if rec.Code != http.StatusCreated {
		t.Fatalf("join %d %s", rec.Code, rec.Body.String())
	}
	token, _ := joined["token"].(string)
	created, product := postJSON(t, r, http.MethodPost, "/v1/maker/products", map[string]any{
		"name": "Catalog Head", "website_url": "https://catalog.example",
	}, bearerHeader(token))
	if created.Code != http.StatusOK {
		t.Fatalf("product %d %s", created.Code, created.Body.String())
	}
	if _, ok := product["id"].(string); !ok {
		t.Fatalf("product body %s", created.Body.String())
	}
	var makerID string
	if err := db.Pool.QueryRow(context.Background(), `SELECT maker_id::text FROM products WHERE name='Catalog Head'`).Scan(&makerID); err != nil {
		t.Fatal(err)
	}
	const extra = 201
	for i := 0; i < extra; i++ {
		name := fmt.Sprintf("Filler Product %03d", i)
		if i == extra-1 {
			name = "Catalog Tail Exact"
		}
		if _, err := db.Pool.Exec(context.Background(), `
INSERT INTO products(maker_id, name, website_url, product_type)
VALUES($1, $2, 'https://catalog.example', 'software')`, makerID, name); err != nil {
			t.Fatal(err)
		}
	}
	var tailID string
	if err := db.Pool.QueryRow(context.Background(), `SELECT id::text FROM products WHERE name='Catalog Tail Exact'`).Scan(&tailID); err != nil {
		t.Fatal(err)
	}
	if _, err := db.Pool.Exec(context.Background(), `
INSERT INTO products(maker_id, name, website_url, product_type, listed)
VALUES($1, 'Catalog Tail Exact', 'https://hidden.example', 'software', false)`, makerID); err != nil {
		t.Fatal(err)
	}
	if countRows(t, db, `SELECT COUNT(*) FROM products`) <= 200 {
		t.Fatal("catalog is not past the old cap")
	}
	names := suggestProductNames(t, r, "catalog tail exact")
	if len(names) != 1 || names[0] != "Catalog Tail Exact" {
		t.Fatalf("exact search %#v", names)
	}
	suggestRec, body := postJSON(t, r, http.MethodPost, "/orp/suggest", map[string]any{"text": "catalog tail exact"}, nil)
	if suggestRec.Code != http.StatusOK {
		t.Fatalf("suggest %d %s", suggestRec.Code, suggestRec.Body.String())
	}
	items, _ := body["items"].([]any)
	first, _ := items[0].(map[string]any)
	card, _ := first["product"].(map[string]any)
	if card["id"] != tailID {
		t.Fatalf("found %#v want %s", card, tailID)
	}
}

func suggestProductNames(t *testing.T, r http.Handler, text string) []string {
	t.Helper()
	rec, body := postJSON(t, r, http.MethodPost, "/orp/suggest", map[string]any{"text": text}, nil)
	if rec.Code != http.StatusOK {
		t.Fatalf("suggest %d %s", rec.Code, rec.Body.String())
	}
	items, _ := body["items"].([]any)
	var names []string
	for _, item := range items {
		obj, _ := item.(map[string]any)
		card, _ := obj["product"].(map[string]any)
		name, _ := card["name"].(string)
		if name != "" {
			names = append(names, name)
		}
	}
	return names
}

func joinKeyBody(t *testing.T, pub ed25519.PublicKey, kid string) string {
	t.Helper()
	raw, err := json.Marshal(map[string]any{
		"agent": map[string]any{"name": "Key Agent"},
		"key": map[string]any{
			"kty": "OKP", "crv": "Ed25519", "kid": kid,
			"x": base64.RawURLEncoding.EncodeToString(pub),
		},
	})
	if err != nil {
		t.Fatal(err)
	}
	return string(raw)
}
