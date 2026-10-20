package handlers

import (
	"net"
	"net/http"
	"net/http/httptest"
	"sync"
	"testing"
)

// TestDomainVerificationTokenServedThenConflict is the local proof of D-063b.
// The token is served by an httptest server. The first verify is 200.
// The second verify is 409 domain_verified and does not fetch again.
func TestDomainVerificationTokenServedThenConflict(t *testing.T) {
	db, cleanup := setupTestDB(t)
	defer cleanup()

	var mu sync.Mutex
	var token string
	page := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		mu.Lock()
		defer mu.Unlock()
		if r.URL.Path != "/.well-known/offer-rails.txt" {
			http.NotFound(w, r)
			return
		}
		w.Header().Set("Content-Type", "text/plain; charset=utf-8")
		_, _ = w.Write([]byte(token + "\n"))
	}))
	t.Cleanup(page.Close)

	var dialed []string
	r := newVerifyRouter(db, verificationFetcher(page, &dialed, func(host string) ([]net.IP, error) {
		if host == "prove.example" {
			return []net.IP{net.ParseIP("203.0.113.50")}, nil
		}
		return []net.IP{net.ParseIP("127.0.0.1")}, nil
	}))

	created, body := postJSON(t, r, http.MethodPost, "/v1/agent-accounts", map[string]any{
		"role": "publisher", "name": "Prove Blog", "email": "prove-blog@example.com", "domain": "prove.example",
	}, nil)
	gotToken, _ := body["verification_token"].(string)
	siteID, _ := body["site_id"].(string)
	siteKey, _ := body["api_key"].(string)
	mu.Lock()
	token = gotToken
	mu.Unlock()
	if created.Code != http.StatusCreated || body["verified"] != false || len(token) != 48 || siteID == "" || siteKey == "" {
		t.Fatalf("signup: %d verified=%v token_len=%d", created.Code, body["verified"], len(token))
	}

	verified, verifiedBody := postJSON(t, r, http.MethodPost, "/v1/sites/"+siteID+"/verify", map[string]any{}, map[string]string{"X-API-Key": siteKey})
	if verified.Code != http.StatusOK || verifiedBody["verified"] != true || verifiedBody["verification_token"] != token || verifiedBody["verified_at"] == "" {
		t.Fatalf("verify: %d verified=%v", verified.Code, verifiedBody["verified"])
	}
	if len(dialed) != 1 || dialed[0] != "203.0.113.50:443" {
		t.Fatalf("file proof dialed %v", dialed)
	}
	dialsAfter := len(dialed)
	again, againBody := postJSON(t, r, http.MethodPost, "/v1/sites/"+siteID+"/verify", map[string]any{}, map[string]string{"X-API-Key": siteKey})
	if again.Code != http.StatusConflict || againBody["reason"] != "domain_verified" || len(dialed) != dialsAfter {
		t.Fatalf("reverify: %d %s dials %d", again.Code, again.Body.String(), len(dialed))
	}
}
