package handlers

import (
	"context"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"

	"affilio-api/internal/database"
	"affilio-api/internal/services"
)

func mintClaim(t *testing.T, r http.Handler, token string) string {
	t.Helper()
	rec, body := postJSON(t, r, http.MethodPost, "/v1/party-claims", map[string]any{}, bearerHeader(token))
	if rec.Code != http.StatusCreated {
		t.Fatalf("claim mint %d %s", rec.Code, rec.Body.String())
	}
	link, _ := body["claim_url"].(string)
	idx := strings.LastIndex(link, "/claim/")
	if idx < 0 {
		t.Fatalf("claim_url %q", link)
	}
	return link[idx+len("/claim/"):]
}

func personOwner(t *testing.T, db *database.DB, subject string) (ownerID, token, csrf string) {
	t.Helper()
	token, err := randomToken()
	if err != nil {
		t.Fatal(err)
	}
	csrf, err = randomToken()
	if err != nil {
		t.Fatal(err)
	}
	err = db.Pool.QueryRow(context.Background(),
		`INSERT INTO owners(issuer,subject,email,name,source) VALUES('https://identity.example',$1,$2,'Person','direct') RETURNING id`,
		subject, subject+"@example.com").Scan(&ownerID)
	if err != nil {
		t.Fatal(err)
	}
	if _, err = db.Pool.Exec(context.Background(), `
INSERT INTO owner_sessions(token_hash, owner_id, csrf, expires_at)
VALUES($1, $2::uuid, $3, NOW()+INTERVAL '8 hours')`, services.HashAPIKey(token), ownerID, csrf); err != nil {
		t.Fatal(err)
	}
	return ownerID, token, csrf
}

func claimPost(t *testing.T, r http.Handler, base, code, cookie, csrf string) *httptest.ResponseRecorder {
	t.Helper()
	form := url.Values{"csrf": {csrf}}
	req := httptest.NewRequest(http.MethodPost, "/claim/"+code, strings.NewReader(form.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req.Header.Set("Origin", base)
	if cookie != "" {
		req.AddCookie(&http.Cookie{Name: ownerCookieName(base), Value: cookie})
	}
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, req)
	return rec
}

func TestORPPartyClaim(t *testing.T) {
	entry, r := trustHost(t)
	db := entry.DB
	base := entry.BaseURL
	token, party := joinParty(t, r)
	agentOwnerID := ownerOfParty(party)

	anon, _ := postJSON(t, r, http.MethodPost, "/v1/party-claims", map[string]any{}, nil)
	if anon.Code != http.StatusUnauthorized {
		t.Fatalf("mint without bearer %d", anon.Code)
	}
	code := mintClaim(t, r, token)

	page := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/claim/"+code, nil)
	r.ServeHTTP(page, req)
	if page.Code != http.StatusOK || !strings.Contains(page.Body.String(), "Sign in") {
		t.Fatalf("anon page %d %s", page.Code, page.Body.String())
	}
	if got := claimPost(t, r, base, code, "", ""); got.Code != http.StatusForbidden {
		t.Fatalf("anon claim %d", got.Code)
	}

	personID, pToken, pCSRF := personOwner(t, db, "person-one")
	cookie := &http.Cookie{Name: ownerCookieName(base), Value: pToken}
	get := httptest.NewRecorder()
	getReq := httptest.NewRequest(http.MethodGet, "/claim/"+code, nil)
	getReq.AddCookie(cookie)
	r.ServeHTTP(get, getReq)
	if get.Code != http.StatusOK || !strings.Contains(get.Body.String(), "Claim this party") {
		t.Fatalf("signed page %d %s", get.Code, get.Body.String())
	}
	if got := claimPost(t, r, base, code, pToken, "bad-csrf"); got.Code != http.StatusForbidden {
		t.Fatalf("bad csrf %d", got.Code)
	}
	done := claimPost(t, r, base, code, pToken, pCSRF)
	if done.Code != http.StatusSeeOther {
		t.Fatalf("claim %d %s", done.Code, done.Body.String())
	}
	setCookie := done.Header().Get("Set-Cookie")
	if !strings.Contains(setCookie, ownerCookieName(base)+"=") {
		t.Fatalf("no session cookie %q", setCookie)
	}
	var issuer string
	var human bool
	if err := db.Pool.QueryRow(context.Background(),
		`SELECT issuer, COALESCE(human_login,false) FROM owners WHERE id=$1`, agentOwnerID).Scan(&issuer, &human); err != nil {
		t.Fatal(err)
	}
	if issuer != "https://identity.example" || !human {
		t.Fatalf("agent owner issuer=%q human=%v", issuer, human)
	}
	var orphaned bool
	if err := db.Pool.QueryRow(context.Background(),
		`SELECT issuer LIKE 'claimed:%' FROM owners WHERE id=$1`, personID).Scan(&orphaned); err != nil {
		t.Fatal(err)
	}
	if !orphaned {
		t.Fatalf("person owner identity not released")
	}

	gone := httptest.NewRecorder()
	r.ServeHTTP(gone, httptest.NewRequest(http.MethodGet, "/claim/"+code, nil))
	if gone.Code != http.StatusGone {
		t.Fatalf("reused code %d", gone.Code)
	}
	if got := claimPost(t, r, base, code, pToken, pCSRF); got.Code != http.StatusGone {
		t.Fatalf("replay claim %d", got.Code)
	}

	second := mintClaim(t, r, token)
	_, oToken, oCSRF := personOwner(t, db, "person-two")
	if got := claimPost(t, r, base, second, oToken, oCSRF); got.Code != http.StatusConflict {
		t.Fatalf("taken claim %d %s", got.Code, got.Body.String())
	}
}
