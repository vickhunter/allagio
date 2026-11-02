package handlers

import (
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"

	"affilio-api/internal/config"
)

func TestServedCopyHasNoPublicBasePlaceholder(t *testing.T) {
	gin.SetMode(gin.TestMode)
	dir := filepath.Join(repoRoot(t), "affilio-wp", "admin", "build")
	r := gin.New()
	(&HostedHandler{BaseURL: "https://allagio.example", AssetsPath: dir, DownloadsPath: t.TempDir()}).Mount(r)
	(&AgentEntry{BaseURL: "https://allagio.example", AssetsPath: dir}).Mount(r)
	for _, path := range []string{"/llms.txt", "/llms-full.txt", "/skill.md", "/home.md", "/protocol.md", "/you.md", "/agents.md", "/makers.md", "/publishers.md"} {
		rec := httptest.NewRecorder()
		r.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, path, nil))
		if rec.Code != http.StatusOK {
			t.Fatalf("%s: %d", path, rec.Code)
		}
		if strings.Contains(rec.Body.String(), "{{PUBLIC_BASE_URL}}") {
			t.Fatalf("%s still has the placeholder", path)
		}
		if strings.Contains(rec.Body.String(), "https://allagio.example/") {
			continue
		}
	}
	err := filepath.WalkDir(dir, func(path string, entry os.DirEntry, err error) error {
		if err != nil || entry.IsDir() {
			return err
		}
		ext := filepath.Ext(path)
		if ext != ".js" && ext != ".html" && ext != ".md" {
			return nil
		}
		body, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		if !strings.Contains(string(body), config.PublicBasePlaceholder) {
			return nil
		}
		if ext == ".md" {
			expanded := config.ExpandPublicBase(string(body), "https://allagio.example")
			if strings.Contains(expanded, config.PublicBasePlaceholder) || !strings.Contains(expanded, "https://allagio.example/") {
				t.Errorf("%s markdown placeholder did not expand", path)
			}
			return nil
		}
		t.Errorf("%s contains {{PUBLIC_BASE_URL}}", path)
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
}
