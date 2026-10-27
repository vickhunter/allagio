package handlers

import (
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"
)

func TestYouLimitsSignedOut(t *testing.T) {
	gin.SetMode(gin.TestMode)
	h := &HostedHandler{
		BaseURL:       "http://127.0.0.1:8080",
		AssetsPath:    filepath.Join(repoRoot(t), "affilio-wp", "admin", "build"),
		DownloadsPath: t.TempDir(),
	}
	r := gin.New()
	h.Mount(r)

	for _, path := range []string{"/you", "/you/"} {
		rec := httptest.NewRecorder()
		r.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, path, nil))
		assertYouLimitsPage(t, path+" signed out", rec)
	}
}

func TestYouLimitsSignedIn(t *testing.T) {
	gin.SetMode(gin.TestMode)
	db, cleanup := setupTestDB(t)
	defer cleanup()
	h := &HostedHandler{
		DB:            db,
		BaseURL:       "http://127.0.0.1:8080",
		AssetsPath:    filepath.Join(repoRoot(t), "affilio-wp", "admin", "build"),
		DownloadsPath: t.TempDir(),
	}
	r := gin.New()
	h.Mount(r)
	_, cookie := testOwnerSession(t, h, "you-limits-owner", "limits@example.com")
	for _, path := range []string{"/you", "/you/"} {
		req := httptest.NewRequest(http.MethodGet, path, nil)
		req.AddCookie(cookie)
		rec := httptest.NewRecorder()
		r.ServeHTTP(rec, req)
		assertYouLimitsPage(t, path+" signed in", rec)
	}
}

func TestYouLimitsMissingAssetIsPlainPage(t *testing.T) {
	gin.SetMode(gin.TestMode)
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "home.html"), []byte("<!doctype html><title>Offer Rails</title>"), 0o644); err != nil {
		t.Fatal(err)
	}
	h := &HostedHandler{BaseURL: "http://127.0.0.1:8080", AssetsPath: dir, DownloadsPath: t.TempDir()}
	r := gin.New()
	h.Mount(r)
	for _, path := range []string{"/you", "/you/"} {
		rec := httptest.NewRecorder()
		r.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, path, nil))
		assertYouLimitsPage(t, path+" missing file", rec)
	}

	if err := os.WriteFile(filepath.Join(dir, "you.html"), []byte("<html>{{if</html>"), 0o644); err != nil {
		t.Fatal(err)
	}
	broken := httptest.NewRecorder()
	r.ServeHTTP(broken, httptest.NewRequest(http.MethodGet, "/you/", nil))
	assertYouLimitsPage(t, "broken template", broken)
}

// TestYouPageIsABuiltImageInput fails when the page served for /you is not
// one of the files Dockerfile.mvp copies into the image.
func TestYouPageIsABuiltImageInput(t *testing.T) {
	src, err := os.ReadFile("workspace.go")
	if err != nil {
		t.Fatal(err)
	}
	text := string(src)
	start := strings.Index(text, "func (h *HostedHandler) You(")
	if start < 0 {
		t.Fatal("You handler is missing")
	}
	rest := text[start:]
	end := strings.Index(rest, "\nfunc ")
	if end < 0 {
		end = len(rest)
	}
	body := rest[:end]
	const name = "you.html"
	if !strings.Contains(body, `"`+name+`"`) {
		t.Fatalf("You handler does not serve %s", name)
	}
	df, err := os.ReadFile(filepath.Join("..", "..", "..", "Dockerfile.mvp"))
	if err != nil {
		t.Fatal(err)
	}
	want := "affilio-wp/admin/build/" + name
	found := false
	for _, line := range strings.Split(string(df), "\n") {
		f := strings.Fields(line)
		if len(f) > 2 && f[0] == "COPY" && f[len(f)-1] == "/app/assets/" {
			for _, c := range f[1 : len(f)-1] {
				if c == want {
					found = true
				}
			}
		}
	}
	if !found {
		t.Fatalf("Dockerfile.mvp does not copy %s into /app/assets/", want)
	}
	if _, err := os.Stat(filepath.Join("..", "..", "..", want)); err != nil {
		t.Fatalf("served page %s is missing from the image inputs: %v", want, err)
	}
}

func assertYouLimitsPage(t *testing.T, label string, rec *httptest.ResponseRecorder) {
	t.Helper()
	if rec.Code >= 500 || rec.Code != http.StatusOK {
		t.Fatalf("%s: %d %s", label, rec.Code, rec.Body.String())
	}
	if ct := rec.Header().Get("Content-Type"); !strings.Contains(ct, "text/html") {
		t.Fatalf("%s content type %q", label, ct)
	}
	body := rec.Body.String()
	for _, needle := range []string{
		"Offer Rails · Your limits",
		"This page is not a signup form",
		"Ask every time",
		"Manual. A new site opens here. Each placement waits for you.",
		"Approve within your rules",
		"Once you set it. While claims are not checked, this still waits for a person.",
		"Approve anything that passes the nine checks",
		"Only you can turn this on. While claims are not checked, this also waits for a person.",
		"POST /v1/agent-tokens",
		"Your agent agreed: link to X in your post Y, 8% per sale, ends 30 Nov.",
	} {
		if !strings.Contains(body, needle) {
			t.Fatalf("%s missing %q", label, needle)
		}
	}
	lower := strings.ToLower(body)
	for _, banned := range []string{"distributoor", "affilio-api", "offerrails.com", "open source", "open-source"} {
		if strings.Contains(lower, banned) {
			t.Fatalf("%s contains %q", label, banned)
		}
	}
}
