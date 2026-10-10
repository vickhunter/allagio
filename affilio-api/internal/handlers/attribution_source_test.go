package handlers

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"

	"affilio-api/internal/database"
	"affilio-api/internal/middleware"
	"affilio-api/internal/services"
)

func TestNormalizeAttributionSource(t *testing.T) {
	cases := map[string]string{
		"":          "direct",
		"  ":        "direct",
		"facebook":  "direct",
		"ChatGPT":   "chatgpt",
		"claude":    "claude",
		" cursor ":  "cursor",
		"grok":      "grok",
		"x":         "x",
		"community": "community",
		"DIRECT":    "direct",
	}
	for in, want := range cases {
		if got := database.NormalizeAttributionSource(in); got != want {
			t.Fatalf("%q -> %q, want %q", in, got, want)
		}
	}
}

func ownerSourceBySubject(t *testing.T, db *database.DB, subject string) string {
	t.Helper()
	var source string
	if err := db.Pool.QueryRow(context.Background(), `SELECT source FROM owners WHERE subject=$1`, subject).Scan(&source); err != nil {
		t.Fatal(err)
	}
	return source
}

func finishOwnerLogin(t *testing.T, h *HostedHandler, r http.Handler, subject, email, path string) {
	t.Helper()
	h.Login.(*fakeOwnerLogin).owner = database.Owner{Issuer: "https://identity.example", Subject: subject, Email: email, Name: "Publisher"}
	start := httptest.NewRecorder()
	r.ServeHTTP(start, httptest.NewRequest(http.MethodGet, path, nil))
	if start.Code != http.StatusFound {
		t.Fatalf("login start %s: %d %s", path, start.Code, start.Body.String())
	}
	loginURL, err := url.Parse(start.Header().Get("Location"))
	if err != nil {
		t.Fatal(err)
	}
	cookies := start.Result().Cookies()
	if len(cookies) != 1 {
		t.Fatal("missing login cookie")
	}
	callback := httptest.NewRequest(http.MethodGet, "/auth/callback?code=valid-code&state="+url.QueryEscape(loginURL.Query().Get("state")), nil)
	callback.AddCookie(cookies[0])
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, callback)
	if rec.Code != http.StatusFound {
		t.Fatalf("callback: %d %s", rec.Code, rec.Body.String())
	}
}

func TestAttributionSourceSignup(t *testing.T) {
	db, cleanup := setupTestDB(t)
	defer cleanup()
	h, r, _ := hostedTestServer(t, db)

	finishOwnerLogin(t, h, r, "source-default", "default-source@example.com", "/auth/login")
	if got := ownerSourceBySubject(t, db, "source-default"); got != "direct" {
		t.Fatalf("default source = %q", got)
	}
	finishOwnerLogin(t, h, r, "source-unknown", "unknown-source@example.com", "/auth/login?source=facebook")
	if got := ownerSourceBySubject(t, db, "source-unknown"); got != "direct" {
		t.Fatalf("unknown source = %q", got)
	}
	finishOwnerLogin(t, h, r, "source-chatgpt", "chatgpt-source@example.com", "/auth/login?source=chatgpt")
	if got := ownerSourceBySubject(t, db, "source-chatgpt"); got != "chatgpt" {
		t.Fatalf("signup source = %q", got)
	}
	finishOwnerLogin(t, h, r, "source-chatgpt", "chatgpt-source@example.com", "/auth/login")
	if got := ownerSourceBySubject(t, db, "source-chatgpt"); got != "chatgpt" {
		t.Fatalf("relogin cleared source to %q", got)
	}
	q := url.Values{"return_to": {"/owner/placements"}, "source": {"Claude"}}
	finishOwnerLogin(t, h, r, "source-claude", "claude-source@example.com", "/auth/login?"+q.Encode())
	if got := ownerSourceBySubject(t, db, "source-claude"); got != "claude" {
		t.Fatalf("return_to signup source = %q", got)
	}
}

func TestAttributionSourceConnect(t *testing.T) {
	db, cleanup := setupTestDB(t)
	defer cleanup()
	h, r, _ := hostedTestServer(t, db)
	r.Group("/v1", middleware.APIKeyAuth(db, nil, h.resource())).POST("/sites/owner-connection", h.StartSiteConnection)
	start := func(key string) string {
		t.Helper()
		req := httptest.NewRequest(http.MethodPost, "/v1/sites/owner-connection", strings.NewReader(`{}`))
		req.Header.Set("X-API-Key", key)
		rec := httptest.NewRecorder()
		r.ServeHTTP(rec, req)
		if rec.Code != http.StatusCreated {
			t.Fatalf("start: %d %s", rec.Code, rec.Body.String())
		}
		var data map[string]any
		if err := json.Unmarshal(rec.Body.Bytes(), &data); err != nil {
			t.Fatal(err)
		}
		u, err := url.Parse(data["connect_url"].(string))
		if err != nil {
			t.Fatal(err)
		}
		return u.Query().Get("token")
	}

	siteID := createTestSite(t, db, "connect-source-key", "blog.example", "Blog")
	o, cookie := testOwnerSession(t, h, "connect-cursor", "connect-cursor@example.com")
	if _, err := db.Pool.Exec(context.Background(), `UPDATE owners SET source='chatgpt' WHERE id=$1`, o.ID); err != nil {
		t.Fatal(err)
	}
	token := start("connect-source-key")
	review := httptest.NewRequest(http.MethodGet, "/owner/connect-site?token="+url.QueryEscape(token)+"&source=cursor", nil)
	review.AddCookie(cookie)
	reviewed := httptest.NewRecorder()
	r.ServeHTTP(reviewed, review)
	if reviewed.Code != http.StatusOK || !strings.Contains(reviewed.Body.String(), `name="source" value="cursor"`) {
		t.Fatalf("connect form: %d %s", reviewed.Code, reviewed.Body.String())
	}
	if rec := ownerForm(t, h, r, cookie, "/owner/connect-site", url.Values{"csrf": {o.CSRF}, "token": {token}, "source": {"cursor"}}); rec.Code != http.StatusOK {
		t.Fatalf("connect: %d %s", rec.Code, rec.Body.String())
	}
	if got := ownerSourceBySubject(t, db, "connect-cursor"); got != "cursor" {
		t.Fatalf("connect source = %q", got)
	}
	linked, err := db.OwnerSession(context.Background(), services.HashAPIKey(cookie.Value))
	if err != nil || linked.SiteID != siteID {
		t.Fatal("site was not linked")
	}

	unknownSite := createTestSite(t, db, "connect-unknown-key", "unknown.example", "Unknown")
	unknown, unknownCookie := testOwnerSession(t, h, "connect-unknown", "connect-unknown@example.com")
	unknownToken := start("connect-unknown-key")
	unknownReview := httptest.NewRequest(http.MethodGet, "/owner/connect-site?token="+url.QueryEscape(unknownToken)+"&source=not-a-real-source", nil)
	unknownReview.AddCookie(unknownCookie)
	unknownReviewed := httptest.NewRecorder()
	r.ServeHTTP(unknownReviewed, unknownReview)
	if !strings.Contains(unknownReviewed.Body.String(), `name="source" value="not-a-real-source"`) {
		t.Fatalf("raw source was rewritten before post: %s", unknownReviewed.Body.String())
	}
	if rec := ownerForm(t, h, r, unknownCookie, "/owner/connect-site", url.Values{"csrf": {unknown.CSRF}, "token": {unknownToken}, "source": {"not-a-real-source"}}); rec.Code != http.StatusOK {
		t.Fatalf("unknown connect: %d %s", rec.Code, rec.Body.String())
	}
	if got := ownerSourceBySubject(t, db, "connect-unknown"); got != "direct" {
		t.Fatalf("unknown connect source = %q", got)
	}
	if linked, err = db.OwnerSession(context.Background(), services.HashAPIKey(unknownCookie.Value)); err != nil || linked.SiteID != unknownSite {
		t.Fatal("unknown source did not link the site")
	}

	keptSite := createTestSite(t, db, "connect-kept-key", "kept.example", "Kept")
	kept, keptCookie := testOwnerSession(t, h, "connect-kept", "connect-kept@example.com")
	if _, err = db.Pool.Exec(context.Background(), `UPDATE owners SET source='grok' WHERE id=$1`, kept.ID); err != nil {
		t.Fatal(err)
	}
	keptToken := start("connect-kept-key")
	if rec := ownerForm(t, h, r, keptCookie, "/owner/connect-site", url.Values{"csrf": {kept.CSRF}, "token": {keptToken}}); rec.Code != http.StatusOK {
		t.Fatalf("connect without source: %d %s", rec.Code, rec.Body.String())
	}
	if got := ownerSourceBySubject(t, db, "connect-kept"); got != "grok" {
		t.Fatalf("omitted source changed stored value to %q", got)
	}
	if linked, err = db.OwnerSession(context.Background(), services.HashAPIKey(keptCookie.Value)); err != nil || linked.SiteID != keptSite {
		t.Fatal("connect without source did not link the site")
	}
}
