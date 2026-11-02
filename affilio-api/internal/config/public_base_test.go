package config

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestPublicBaseURLUsesTheEnvOrTheOneDefault(t *testing.T) {
	if PublicBaseURL("") != DefaultPublicBaseURL {
		t.Fatalf("empty %s", PublicBaseURL(""))
	}
	if PublicBaseURL("https://allagio.example/") != "https://allagio.example" {
		t.Fatalf("trim %s", PublicBaseURL("https://allagio.example/"))
	}
	if DocumentOrigin("http://127.0.0.1:8080") != DefaultPublicBaseURL {
		t.Fatal("loopback documents should use the default origin")
	}
	if DocumentOrigin("https://allagio.example") != "https://allagio.example" {
		t.Fatal("public origin")
	}
	got := ExpandPublicBase("See {{PUBLIC_BASE_URL}}/mcp", "https://allagio.example")
	if got != "See https://allagio.example/mcp" {
		t.Fatalf("expand %s", got)
	}
}

// TestDefaultPublicBaseURLIsTheOnlyGoLiteral fails when another non-test Go
// file copies the hosted origin instead of using DefaultPublicBaseURL.
func TestDefaultPublicBaseURLIsTheOnlyGoLiteral(t *testing.T) {
	root := filepath.Join("..", "..")
	var hits []string
	err := filepath.WalkDir(root, func(path string, entry os.DirEntry, err error) error {
		if err != nil || entry.IsDir() {
			return err
		}
		if !strings.HasSuffix(path, ".go") || strings.HasSuffix(path, "_test.go") {
			return nil
		}
		body, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		if !strings.Contains(string(body), "onrender.com") {
			return nil
		}
		rel := strings.TrimPrefix(path, root+string(filepath.Separator))
		if rel == "internal/config/public_base.go" {
			return nil
		}
		if rel == "internal/orp/publicdocs.go" {
			if strings.Contains(string(body), "DefaultPublicBaseURL") {
				t.Fatal("schema id host must be its own literal, not the public default")
			}
			return nil
		}
		hits = append(hits, rel)
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	if len(hits) > 0 {
		t.Fatalf("hosted origin is copied outside DefaultPublicBaseURL:\n%s", strings.Join(hits, "\n"))
	}
}

func TestCLIDefaultMatchesTheGoConstant(t *testing.T) {
	for _, rel := range []string{
		filepath.Join("..", "..", "..", "cli", "lib", "public-base.mjs"),
		filepath.Join("..", "..", "..", "ui", "landing", "src", "public-base.mjs"),
	} {
		body, err := os.ReadFile(rel)
		if err != nil {
			t.Fatal(err)
		}
		if !strings.Contains(string(body), `'`+DefaultPublicBaseURL+`'`) {
			t.Fatalf("%s default is not %s\n%s", rel, DefaultPublicBaseURL, body)
		}
	}
}
