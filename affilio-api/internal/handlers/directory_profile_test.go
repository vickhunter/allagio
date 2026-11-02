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

func TestHeadDirectoryRoutesMatchMain(t *testing.T) {
	gin.SetMode(gin.TestMode)
	dir := t.TempDir()
	for _, name := range []string{"profile", "not-found"} {
		pageDir := filepath.Join(dir, "landing-v9", name)
		if err := os.MkdirAll(pageDir, 0o755); err != nil {
			t.Fatal(err)
		}
		body := "<!doctype html><main><h1>" + name + "-marker</h1></main>"
		if err := os.WriteFile(filepath.Join(pageDir, "index.html"), []byte(body), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	if err := os.WriteFile(filepath.Join(dir, "publishers-setup.html"), []byte("<!doctype html><title>Publisher setup</title>"), 0o644); err != nil {
		t.Fatal(err)
	}
	h := &HostedHandler{AssetsPath: dir, BaseURL: "http://127.0.0.1:8080"}
	r := gin.New()
	h.Mount(r)

	getMaker := httptest.NewRecorder()
	r.ServeHTTP(getMaker, httptest.NewRequest(http.MethodGet, "/makers/acme", nil))
	headMaker := httptest.NewRecorder()
	r.ServeHTTP(headMaker, httptest.NewRequest(http.MethodHead, "/makers/acme", nil))
	if getMaker.Code != http.StatusNotFound || !strings.Contains(getMaker.Body.String(), "profile-marker") {
		t.Fatalf("get maker %d %s", getMaker.Code, getMaker.Body.String())
	}
	if headMaker.Code != http.StatusNotFound || !strings.Contains(headMaker.Body.String(), "not-found-marker") || strings.Contains(headMaker.Body.String(), "profile-marker") {
		t.Fatalf("head maker %d %s", headMaker.Code, headMaker.Body.String())
	}
	for _, path := range []string{"/publishers/setup", "/publishers/setup/"} {
		rec := httptest.NewRecorder()
		r.ServeHTTP(rec, httptest.NewRequest(http.MethodHead, path, nil))
		if rec.Code != http.StatusNotFound || !strings.Contains(rec.Body.String(), "not-found-marker") || strings.Contains(rec.Body.String(), "Publisher setup") {
			t.Fatalf("head %s %d %s", path, rec.Code, rec.Body.String())
		}
	}
}

func TestDirectoryUnknownSlugIs404(t *testing.T) {
	gin.SetMode(gin.TestMode)
	dir := t.TempDir()
	pageDir := filepath.Join(dir, "landing-v9", "profile")
	if err := os.MkdirAll(pageDir, 0o755); err != nil {
		t.Fatal(err)
	}
	const marker = "That page is not on Allagio."
	body := "<!doctype html><header><nav aria-label=\"Main\"><a href=\"/makers/\">Makers</a></nav></header><main><h1>" + marker + "</h1><a href=\"/\">Home</a><a href=\"/agents\">Agents</a></main>"
	if err := os.WriteFile(filepath.Join(pageDir, "index.html"), []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
	setupHTML := filepath.Join(dir, "publishers-setup.html")
	if err := os.WriteFile(setupHTML, []byte("<!doctype html><title>Allagio · Publisher setup</title>"), 0o644); err != nil {
		t.Fatal(err)
	}
	h := &HostedHandler{AssetsPath: dir, BaseURL: "http://127.0.0.1:8080"}
	r := gin.New()
	h.Mount(r)

	for _, path := range []string{"/makers/missing", "/publishers/missing", "/makers/test-vibesales", "/publishers/test-journal/"} {
		rec := httptest.NewRecorder()
		r.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, path, nil))
		if rec.Code != http.StatusNotFound {
			t.Fatalf("%s status %d", path, rec.Code)
		}
		if !strings.Contains(rec.Body.String(), marker) || !strings.Contains(rec.Body.String(), `href="/agents"`) || !strings.Contains(rec.Body.String(), ">Home<") {
			t.Fatalf("%s body %s", path, rec.Body.String())
		}
		if !strings.Contains(rec.Header().Get("Content-Type"), "text/html") {
			t.Fatalf("%s type %s", path, rec.Header().Get("Content-Type"))
		}
	}

	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/publishers/setup", nil))
	if rec.Code != http.StatusOK || !strings.Contains(rec.Body.String(), "Publisher setup") {
		t.Fatalf("setup %d %s", rec.Code, rec.Body.String())
	}
	for _, path := range []string{"/makers/setup", "/makers/setup/"} {
		makerSetup := httptest.NewRecorder()
		r.ServeHTTP(makerSetup, httptest.NewRequest(http.MethodGet, path, nil))
		if makerSetup.Code != http.StatusNotFound || strings.Contains(makerSetup.Body.String(), "Publisher setup") || !strings.Contains(makerSetup.Body.String(), marker) {
			t.Fatalf("%s %d %s", path, makerSetup.Code, makerSetup.Body.String())
		}
	}

	missing := &HostedHandler{AssetsPath: t.TempDir(), BaseURL: "http://127.0.0.1:8080"}
	plain := gin.New()
	missing.Mount(plain)
	empty := httptest.NewRecorder()
	plain.ServeHTTP(empty, httptest.NewRequest(http.MethodGet, "/makers/missing", nil))
	if empty.Code != http.StatusNotFound || !strings.Contains(empty.Body.String(), marker) || !strings.Contains(empty.Body.String(), `aria-label="Main"`) {
		t.Fatalf("missing file %d %s", empty.Code, empty.Body.String())
	}
}
