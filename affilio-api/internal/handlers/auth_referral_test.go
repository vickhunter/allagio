package handlers

import (
	"context"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/http/cookiejar"
	"net/url"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"testing"

	"affilio-api/internal/database"

	"github.com/gin-gonic/gin"
)

type sameOriginLogin struct {
	inner *fakeOwnerLogin
	base  string
}

func (s *sameOriginLogin) AuthorizationURL(state, nonce, verifier string) string {
	s.inner.AuthorizationURL(state, nonce, verifier)
	return s.base + "/fake-oidc/authorize?state=" + url.QueryEscape(state)
}

func (s *sameOriginLogin) Verify(ctx context.Context, code, nonce, verifier string) (database.Owner, error) {
	return s.inner.Verify(ctx, code, nonce, verifier)
}

func mountAuthDemo(db *database.DB, base string) *gin.Engine {
	login := &fakeOwnerLogin{owner: database.Owner{Issuer: "https://identity.example", Subject: "e2e-publisher", Email: "publisher@example.com", Name: "Publisher"}}
	r := gin.New()
	h := &HostedHandler{DB: db, BaseURL: base, LoopbackURL: base, Login: &sameOriginLogin{inner: login, base: base}}
	h.Mount(r)
	r.GET("/fake-oidc/authorize", func(c *gin.Context) {
		c.Redirect(http.StatusFound, "/auth/callback?code=valid-code&state="+url.QueryEscape(c.Query("state")))
	})
	return r
}

func TestServeDemoAuthForE2E(t *testing.T) {
	if os.Getenv("AUTH_E2E") != "1" {
		t.Skip()
	}
	db, cleanup := setupTestDB(t)
	defer cleanup()
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	base := "http://" + ln.Addr().String()
	srv := &http.Server{Handler: mountAuthDemo(db, base)}
	go func() { _ = srv.Serve(ln) }()
	if file := os.Getenv("AUTH_E2E_ADDR_FILE"); file != "" {
		if err = os.WriteFile(file, []byte(ln.Addr().String()), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	fmt.Fprintf(os.Stdout, "AUTH_READY %s\n", ln.Addr().String())
	_ = os.Stdout.Sync()
	sig := make(chan os.Signal, 1)
	signal.Notify(sig, syscall.SIGINT, syscall.SIGTERM)
	<-sig
	_ = srv.Close()
}

func TestReferralSurvivesDemoSignIn(t *testing.T) {
	db, cleanup := setupTestDB(t)
	defer cleanup()
	gin.SetMode(gin.TestMode)
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	base := "http://" + ln.Addr().String()
	srv := &http.Server{Handler: mountAuthDemo(db, base)}
	go func() { _ = srv.Serve(ln) }()
	t.Cleanup(func() { _ = srv.Close() })

	t.Run("start page carries ref and via into sign-in", func(t *testing.T) {
		resp, err := http.Get(base + "/demo/start?ref=camp&via=post2615")
		if err != nil {
			t.Fatal(err)
		}
		body, _ := io.ReadAll(resp.Body)
		resp.Body.Close()
		html := string(body)
		want := `href="/auth/login?ref=camp&amp;via=post2615"`
		if strings.Count(html, want) < 2 {
			t.Fatalf("sign-in and register must keep ref and via: %s", html)
		}
	})

	t.Run("callback lands on the owner page with ref and via", func(t *testing.T) {
		jar, err := cookiejar.New(nil)
		if err != nil {
			t.Fatal(err)
		}
		client := &http.Client{Jar: jar}
		resp, err := client.Get(base + "/auth/login?ref=camp&via=post2615")
		if err != nil {
			t.Fatal(err)
		}
		resp.Body.Close()
		final := resp.Request.URL
		if !strings.HasPrefix(final.Path, "/owner") || !strings.Contains(final.RawQuery, "ref=camp") || !strings.Contains(final.RawQuery, "via=post2615") {
			t.Fatalf("callback landed on %s", final)
		}
	})
}
