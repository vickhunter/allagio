package handlers

import (
	"context"
	"crypto/ed25519"
	"encoding/base64"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"affilio-api/internal/dcp"
	"affilio-api/internal/middleware"
	"affilio-api/internal/orp"

	"github.com/gin-gonic/gin"
)

func TestJoinAgentProblemCodes(t *testing.T) {
	gin.SetMode(gin.TestMode)
	r := gin.New()
	(&ORPHost{}).Mount(r)
	pub, _, err := ed25519.GenerateKey(nil)
	if err != nil {
		t.Fatal(err)
	}
	goodX := base64.RawURLEncoding.EncodeToString(pub)
	validKey := func(extra string) string {
		return `{"kty":"OKP","crv":"Ed25519","x":"` + goodX + `","kid":"abc"` + extra + `}`
	}
	longVersion := strings.Repeat("v", 33)
	cases := []struct {
		name   string
		body   string
		codes  []string
		fields []string
		orp    string
	}{
		{name: "not object", body: `[]`, codes: []string{"join_not_object"}, fields: []string{"(body)"}},
		{name: "null", body: `null`, codes: []string{"join_not_object"}, fields: []string{"(body)"}},
		{name: "not json", body: `{`, codes: []string{"join_body_json"}, fields: []string{"(body)"}},
		{name: "too big", body: `{"pad":"` + strings.Repeat("a", orp.MaxBodyBytes) + `"}`, codes: []string{"join_body_size"}, fields: []string{"(body)"}},
		{name: "dirty", body: `{"agent":{"name":"Bad <script>"}}`, codes: []string{"join_disallowed_chars", "join_agent_name"}, fields: []string{"/agent/name"}, orp: "0.4"},
		{name: "email", body: `{"email":"a@b.c"}`, codes: []string{"join_personal"}, fields: []string{"/email"}},
		{name: "phone", body: `{"phone":"555"}`, codes: []string{"join_personal"}, fields: []string{"/phone"}},
		{name: "password", body: `{"password":"secret"}`, codes: []string{"join_personal"}, fields: []string{"/password"}},
		{name: "domain", body: `{"domain":"example.com"}`, codes: []string{"join_personal"}, fields: []string{"/domain"}},
		{name: "website", body: `{"website":"https://example.com"}`, codes: []string{"join_personal"}, fields: []string{"/website"}},
		{name: "personal name", body: `{"name":"Ada"}`, codes: []string{"join_personal"}, fields: []string{"/name"}},
		{name: "orp version", body: `{"orp":"0.3"}`, codes: []string{"join_orp_version"}, fields: []string{"/orp"}},
		{name: "unknown field", body: `{"nope":true}`, codes: []string{"join_unknown_field"}, fields: []string{"/nope"}},
		{name: "key shape", body: `{"key":[]}`, codes: []string{"join_key_shape"}, fields: []string{"/key"}},
		{name: "key private", body: `{"key":` + validKey(`,"d":"abc"`) + `}`, codes: []string{"join_key_private"}, fields: []string{"/key/d"}},
		{name: "key field", body: `{"key":` + validKey(`,"extra":1`) + `}`, codes: []string{"join_key_field"}, fields: []string{"/key/extra"}},
		{name: "key kty", body: `{"key":{"kty":"RSA","crv":"Ed25519","x":"` + goodX + `","kid":"abc"}}`, codes: []string{"join_key_kty"}, fields: []string{"/key/kty"}},
		{name: "key crv", body: `{"key":{"kty":"OKP","crv":"P-256","x":"` + goodX + `","kid":"abc"}}`, codes: []string{"join_key_crv"}, fields: []string{"/key/crv"}},
		{name: "key x", body: `{"key":{"kty":"OKP","crv":"Ed25519","x":"abc","kid":"abc"}}`, codes: []string{"join_key_x"}, fields: []string{"/key/x"}},
		{name: "key kid", body: `{"key":{"kty":"OKP","crv":"Ed25519","x":"` + goodX + `","kid":"bad kid"}}`, codes: []string{"join_key_kid"}, fields: []string{"/key/kid"}},
		{name: "key alg", body: `{"key":` + validKey(`,"alg":"RS256"`) + `}`, codes: []string{"join_key_alg"}, fields: []string{"/key/alg"}},
		{name: "key use", body: `{"key":` + validKey(`,"use":"enc"`) + `}`, codes: []string{"join_key_use"}, fields: []string{"/key/use"}},
		{name: "agent shape", body: `{"agent":[]}`, codes: []string{"join_agent_shape"}, fields: []string{"/agent"}, orp: "0.4"},
		{name: "agent field", body: `{"agent":{"name":"Ada","rank":1}}`, codes: []string{"join_agent_field"}, fields: []string{"/agent/rank"}, orp: "0.4"},
		{name: "agent name", body: `{"agent":{"name":"Cold Agent (walk)"}}`, codes: []string{"join_agent_name"}, fields: []string{"/agent/name"}, orp: "0.4"},
		{name: "agent version", body: `{"agent":{"name":"Ada","version":"` + longVersion + `"}}`, codes: []string{"join_agent_version"}, fields: []string{"/agent/version"}, orp: "0.4"},
		{name: "several rules", body: `{"email":"a@b.c","agent":{"name":"Cold Agent (walk)"}}`, codes: []string{"join_personal", "join_agent_name"}, fields: []string{"/email", "/agent/name"}, orp: "0.4"},
	}
	seen := map[string]bool{}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			rec := postRaw(t, r, "/orp/join", tc.body, nil)
			doc, codes := problemCodes(t, rec)
			if !sameCodes(codes, tc.codes) {
				t.Fatalf("codes %v want %v body %s", codes, tc.codes, rec.Body.String())
			}
			for _, field := range tc.fields {
				if !problemHasField(doc, field) {
					t.Fatalf("missing field %s in %s", field, rec.Body.String())
				}
			}
			gotORP, _ := doc["orp"].(string)
			if gotORP != tc.orp {
				t.Fatalf("orp %q want %q body %s", gotORP, tc.orp, rec.Body.String())
			}
			if tc.name == "agent name" {
				detail, _ := doc["detail"].(string)
				if !strings.Contains(detail, orp.AgentNameRule) || strings.Contains(detail, "ORP 0.3.") || !strings.Contains(detail, "field /agent/name failed rule join_agent_name") {
					t.Fatalf("detail %q", detail)
				}
			}
			if tc.name == "unknown field" && strings.Contains(rec.Body.String(), "ORP 0.4.") {
				t.Fatalf("unknown field labeled ORP 0.4: %s", rec.Body.String())
			}
			for _, code := range codes {
				seen[code] = true
			}
		})
	}
	for _, code := range orp.JoinErrorCodes {
		if code == "join_bearer_unknown" || code == "join_key_held" {
			continue
		}
		if !seen[code] {
			t.Errorf("no case produced %s", code)
		}
	}
}

func TestJoinAgentToolSurfacesStableCode(t *testing.T) {
	gin.SetMode(gin.TestMode)
	r := gin.New()
	(&ORPHost{}).Mount(r)
	server := httptest.NewServer(r)
	defer server.Close()
	client := dcp.NewClient(server.URL, "")
	_, err := dcp.CallTool(client, "join_agent", map[string]interface{}{"name": "Cold Agent (walk)"})
	if err == nil {
		t.Fatal("expected join_agent error")
	}
	var doc map[string]any
	if json.Unmarshal([]byte(err.Error()), &doc) != nil {
		t.Fatalf("tool error is not problem JSON: %s", err.Error())
	}
	if !problemHasCode(doc, "join_agent_name") || problemHasCode(doc, "join_personal") {
		t.Fatalf("tool error %s", err.Error())
	}
	_, err = dcp.CallTool(client, "join_agent", map[string]interface{}{"email": "a@b.c"})
	if err == nil || !strings.Contains(err.Error(), "join_personal") {
		t.Fatalf("email tool error %v", err)
	}
}

func TestJoinBearerReusesParty(t *testing.T) {
	db, cleanup := setupTestDB(t)
	defer cleanup()
	entry := &AgentEntry{
		DB: db, BaseURL: "http://127.0.0.1:8080", AddressKey: testAddressKey,
		ClaimsCheck: "stub", TokenLimiter: NewIPRateLimiter(1, time.Minute),
	}
	host := &ORPHost{Entry: entry}
	r := mountORP(entry, host)
	owners := countRows(t, db, `SELECT COUNT(*) FROM owners`)
	rec, body := postJSON(t, r, http.MethodPost, "/orp/join", map[string]any{"agent": map[string]any{"name": "Ada Lovelace"}}, nil)
	if rec.Code != http.StatusCreated {
		t.Fatalf("join %d %s", rec.Code, rec.Body.String())
	}
	token, _ := body["token"].(string)
	party, _ := body["party"].(string)
	if token == "" || party == "" {
		t.Fatalf("join body %s", rec.Body.String())
	}
	ownerID := party[strings.LastIndex(party, "/")+1:]
	again, againBody := postJSON(t, r, http.MethodPost, "/orp/join", map[string]any{"agent": map[string]any{"name": "Other Name"}}, bearerHeader(token))
	if again.Code != http.StatusOK {
		t.Fatalf("reuse %d %s", again.Code, again.Body.String())
	}
	if againBody["token"] != token || againBody["party"] != party {
		t.Fatalf("reuse body %s", again.Body.String())
	}
	if _, ok := againBody["access_token"]; ok {
		t.Fatalf("reuse added a field %s", again.Body.String())
	}
	var stored string
	if err := db.Pool.QueryRow(context.Background(), `SELECT name FROM owners WHERE id=$1`, ownerID).Scan(&stored); err != nil {
		t.Fatal(err)
	}
	if stored != "Ada Lovelace" {
		t.Fatalf("owner renamed to %q", stored)
	}
	if countRows(t, db, `SELECT COUNT(*) FROM owners`) != owners+1 {
		t.Fatal("reuse minted a second owner")
	}
	if countRows(t, db, `SELECT COUNT(*) FROM agent_join_tokens WHERE owner_id=$1`, ownerID) != 1 {
		t.Fatal("reuse minted a second token")
	}
	blocked, _ := postJSON(t, r, http.MethodPost, "/orp/join", map[string]any{}, nil)
	if blocked.Code != http.StatusTooManyRequests {
		t.Fatalf("fresh join after the cap %d %s", blocked.Code, blocked.Body.String())
	}
	if countRows(t, db, `SELECT COUNT(*) FROM agent_join_tokens WHERE owner_id=$1`, ownerID) != 1 {
		t.Fatal("rate limited join wrote a token")
	}

	bad := postRaw(t, r, "/orp/join", `{}`, map[string]string{"Authorization": "Bearer not-a-real-token"})
	_, codes := problemCodes(t, bad)
	if !sameCodes(codes, []string{"join_bearer_unknown"}) || !strings.Contains(bad.Body.String(), "The bearer wins") {
		t.Fatalf("bad bearer %s", bad.Body.String())
	}
	if countRows(t, db, `SELECT COUNT(*) FROM owners`) != owners+1 {
		t.Fatal("unknown bearer minted a party")
	}

	pubA, _, err := ed25519.GenerateKey(nil)
	if err != nil {
		t.Fatal(err)
	}
	pubB, _, err := ed25519.GenerateKey(nil)
	if err != nil {
		t.Fatal(err)
	}
	keyA := testJWK(pubA, "key-a")
	keyB := testJWK(pubB, "key-b")
	held := postRaw(t, r, "/orp/join", d153JSON(t, map[string]any{"key": keyB}), bearerHeader(token))
	if held.Code != http.StatusOK {
		t.Fatalf("first key bind %d %s", held.Code, held.Body.String())
	}
	if countRows(t, db, `SELECT COUNT(*) FROM party_keys WHERE owner_id=$1`, ownerID) != 1 {
		t.Fatal("first key was not stored")
	}
	clash := postRaw(t, r, "/orp/join", d153JSON(t, map[string]any{"key": keyA}), bearerHeader(token))
	_, clashCodes := problemCodes(t, clash)
	if !sameCodes(clashCodes, []string{"join_key_held"}) || !strings.Contains(clash.Body.String(), "that key wins") {
		t.Fatalf("key clash %s", clash.Body.String())
	}
	if countRows(t, db, `SELECT COUNT(*) FROM party_keys WHERE owner_id=$1`, ownerID) != 1 {
		t.Fatal("clashing key replaced the registered key")
	}
	same := postRaw(t, r, "/orp/join", d153JSON(t, map[string]any{"key": keyB}), bearerHeader(token))
	if same.Code != http.StatusOK {
		t.Fatalf("same key %d %s", same.Code, same.Body.String())
	}
	if countRows(t, db, `SELECT COUNT(*) FROM owners`) != owners+1 || countRows(t, db, `SELECT COUNT(*) FROM party_keys WHERE owner_id=$1`, ownerID) != 1 {
		t.Fatal("same key minted or stored a second key")
	}
}

func TestFindPartnersExactProductHidesPrivate(t *testing.T) {
	db, cleanup := setupTestDB(t)
	defer cleanup()
	entry := &AgentEntry{
		DB: db, BaseURL: "http://127.0.0.1:8080", AddressKey: testAddressKey,
		ClaimsCheck: "stub", TokenLimiter: NewIPRateLimiter(8, time.Minute),
	}
	host := &ORPHost{Entry: entry}
	r := mountORP(entry, host)
	maker := r.Group("/v1/maker", middleware.MakerKeyAuth(db, entry.mcpResource(nil)))
	maker.POST("/products", (&ProductsHandler{DB: db}).Create)

	rec, joined := postJSON(t, r, http.MethodPost, "/orp/join", map[string]any{"agent": map[string]any{"name": "Product Agent"}}, nil)
	if rec.Code != http.StatusCreated {
		t.Fatalf("join %d %s", rec.Code, rec.Body.String())
	}
	token, _ := joined["token"].(string)
	party, _ := joined["party"].(string)
	ownerID := party[strings.LastIndex(party, "/")+1:]
	created, product := postJSON(t, r, http.MethodPost, "/v1/maker/products", map[string]any{
		"name": "TEST Zqtesticle", "website_url": "https://zqtesticle.example",
		"tagline": "write owner@agents.invalid", "primary_category": "Tools",
		"docs_url": "https://docs.zqtesticle.example/secret", "pricing_url": "https://zqtesticle.example/pricing",
		"payment_provider": "stripe", "keywords": []string{"zqtesticle", "private-keyword"},
	}, bearerHeader(token))
	if created.Code != http.StatusOK {
		t.Fatalf("product %d %s", created.Code, created.Body.String())
	}
	productID, _ := product["id"].(string)
	if productID == "" {
		t.Fatalf("product body %s", created.Body.String())
	}
	insertProgram(t, db, "AAA TEST Zqtesticle", "Decoy", "aaa-test-zqtesticle")
	if _, err := db.Pool.Exec(context.Background(), `
INSERT INTO offers(product_id, status, commission_type, commission_value, landing_url, terms)
VALUES($1, 'draft', 'percentage', 10, 'https://leakdraft9.example', 'LEAKDRAFT9')`, productID); err != nil {
		t.Fatal(err)
	}
	if _, err := db.Pool.Exec(context.Background(), `
INSERT INTO orp_offers(id, owner_id, party, terms, terms_hash, created_at, listed)
VALUES('off_leak', $1, $2, '{"title":"LEAKOFFER9"}', 'sha256-leak', NOW(), false)`, ownerID, party); err != nil {
		t.Fatal(err)
	}
	if _, err := db.Pool.Exec(context.Background(), `
INSERT INTO orp_spaces(id, owner_id, party, space, listed)
VALUES('sp_leak', $1, $2, '{"title":"LEAKSPACE9","locator":"https://leakspace9.example"}', false)`, ownerID, party); err != nil {
		t.Fatal(err)
	}

	server := httptest.NewServer(r)
	defer server.Close()
	out, err := dcp.CallTool(dcp.NewClient(server.URL, ""), "find_partners", map[string]interface{}{"text": "TEST Zqtesticle"})
	if err != nil {
		t.Fatalf("find_partners %v", err)
	}
	items, _ := out["items"].([]any)
	if len(items) == 0 {
		t.Fatal("find_partners returned no items")
	}
	first, _ := items[0].(map[string]any)
	card, _ := first["product"].(map[string]any)
	if card["name"] != "TEST Zqtesticle" {
		t.Fatalf("first item %#v", first)
	}
	raw, err := json.Marshal(out)
	if err != nil {
		t.Fatal(err)
	}
	if err := orp.ValidateAgainst(filepath.Join(orpRepoRoot(t), "docs", "protocol", "schema", "0.4", "http", "suggest-response.schema.json"), out); err != nil {
		t.Fatalf("suggest schema: %v\n%s", err, raw)
	}
	var email, ownerEmail string
	if err := db.Pool.QueryRow(context.Background(), `SELECT m.email, o.email FROM makers m JOIN owners o ON o.maker_id = m.id WHERE o.id=$1`, ownerID).Scan(&email, &ownerEmail); err != nil {
		t.Fatal(err)
	}
	body := string(raw)
	for _, leak := range []string{email, ownerEmail, "owner@", "LEAKDRAFT9", "LEAKOFFER9", "LEAKSPACE9", "private-keyword", "docs.zqtesticle.example", "payment_provider", "docs_url", "pricing_url"} {
		if strings.Contains(body, leak) {
			t.Fatalf("find_partners leaked %s", leak)
		}
	}
	if _, ok := first["offer"]; ok || first["space"] != nil {
		t.Fatalf("product card carried an offer or space: %#v", first)
	}
	if countRows(t, db, `SELECT COUNT(*) FROM offers WHERE product_id=$1 AND status='listed'`, productID) != 0 {
		t.Fatal("draft offer became listed")
	}
	if countRows(t, db, `SELECT COUNT(*) FROM orp_offers WHERE owner_id=$1 AND listed`, ownerID) != 0 {
		t.Fatal("orp offer became listed")
	}
	if countRows(t, db, `SELECT COUNT(*) FROM orp_spaces WHERE owner_id=$1 AND listed`, ownerID) != 0 {
		t.Fatal("space became listed")
	}
}

func TestJoinErrorCodesAreDocumented(t *testing.T) {
	ref, err := os.ReadFile(filepath.Join(orpRepoRoot(t), "affilio-wp", "admin", "build", "landing-v9", "reference.md"))
	if err != nil {
		t.Fatal(err)
	}
	docs := []struct {
		name string
		body string
	}{
		{"mcp.md", mcpDoc},
		{"reference.md", string(ref)},
		{"llms-full", llmsFullDocument()},
	}
	for _, code := range orp.JoinErrorCodes {
		for _, doc := range docs {
			if !strings.Contains(doc.body, code) {
				t.Errorf("%s missing %s", doc.name, code)
			}
		}
	}
	phrase := "An empty User-Agent or a library-default User-Agent is blocked at the edge with Cloudflare error 1010"
	for _, body := range []string{llmsBody(), llmsFullDocument(), mcpDoc, string(ref)} {
		if !strings.Contains(body, phrase) {
			t.Fatal("agent docs omit the User-Agent rule")
		}
	}
	for _, rel := range []string{
		"ui/landing/public/llms.txt",
		"affilio-wp/admin/build/landing-v9/llms.txt",
	} {
		disk, err := os.ReadFile(filepath.Join(orpRepoRoot(t), rel))
		if err != nil {
			t.Fatal(err)
		}
		if string(disk) != llmsBody() {
			t.Fatalf("%s drifted from llmsBody", rel)
		}
	}
}

func postRaw(t *testing.T, r http.Handler, path, body string, headers map[string]string) *httptest.ResponseRecorder {
	t.Helper()
	req := httptest.NewRequest(http.MethodPost, path, strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	for key, value := range headers {
		req.Header.Set(key, value)
	}
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, req)
	return rec
}

func problemCodes(t *testing.T, rec *httptest.ResponseRecorder) (map[string]any, []string) {
	t.Helper()
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("status %d %s", rec.Code, rec.Body.String())
	}
	if !strings.Contains(rec.Header().Get("Content-Type"), "application/problem+json") {
		t.Fatalf("content-type %q body %s", rec.Header().Get("Content-Type"), rec.Body.String())
	}
	var doc map[string]any
	if err := json.Unmarshal(rec.Body.Bytes(), &doc); err != nil {
		t.Fatal(err)
	}
	if doc["type"] != "https://allagio.dev/problems/join" || doc["status"] != float64(400) {
		t.Fatalf("problem %s", rec.Body.String())
	}
	raw, _ := doc["errors"].([]any)
	codes := make([]string, 0, len(raw))
	for _, item := range raw {
		obj, _ := item.(map[string]any)
		code, _ := obj["code"].(string)
		field, _ := obj["field"].(string)
		detail, _ := obj["detail"].(string)
		if code == "" || field == "" || !strings.Contains(detail, "field "+field+" failed rule "+code) {
			t.Fatalf("error entry %#v", obj)
		}
		codes = append(codes, code)
	}
	if len(codes) == 0 {
		t.Fatalf("no errors in %s", rec.Body.String())
	}
	return doc, codes
}

func problemHasField(doc map[string]any, field string) bool {
	raw, _ := doc["errors"].([]any)
	for _, item := range raw {
		obj, _ := item.(map[string]any)
		if obj["field"] == field {
			return true
		}
	}
	return false
}

func problemHasCode(doc map[string]any, code string) bool {
	raw, _ := doc["errors"].([]any)
	for _, item := range raw {
		obj, _ := item.(map[string]any)
		if obj["code"] == code {
			return true
		}
	}
	return false
}

func sameCodes(got, want []string) bool {
	if len(got) != len(want) {
		return false
	}
	seen := map[string]int{}
	for _, code := range got {
		seen[code]++
	}
	for _, code := range want {
		seen[code]--
		if seen[code] < 0 {
			return false
		}
	}
	for _, n := range seen {
		if n != 0 {
			return false
		}
	}
	return true
}

func d153JSON(t *testing.T, value any) string {
	t.Helper()
	raw, err := json.Marshal(value)
	if err != nil {
		t.Fatal(err)
	}
	return string(raw)
}
