package handlers

import (
	"encoding/json"
	"io/fs"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"

	"affilio-api/internal/config"
	"affilio-api/internal/orp"

	"github.com/gin-gonic/gin"
)

var markdownLink = regexp.MustCompile(`\[[^\]]*\]\(([^)\s]+)\)`)
var hostedURL = regexp.MustCompile(`https://offer-rails\.onrender\.com[^\s)\]"'<>]*`)

func TestORPDocumentLinks(t *testing.T) {
	gin.SetMode(gin.TestMode)
	r := gin.New()
	entry := &AgentEntry{}
	host := &ORPHost{Entry: entry}
	entry.Mount(r)
	host.Mount(r)
	r.GET("/.well-known/dcp.json", ServeDCPDiscovery(""))
	root := repoRoot(t)
	r.Static("/landing-v9", filepath.Join(root, "affilio-wp", "admin", "build", "landing-v9"))

	seen := map[string]bool{}
	var queue []string
	add := func(raw string) {
		t.Helper()
		cleaned := strings.Trim(raw, "`.,);")
		parsed, err := url.Parse(cleaned)
		if err != nil || parsed.Host != "offer-rails.onrender.com" || parsed.Scheme != "https" {
			t.Fatalf("doc link %q", raw)
		}
		if parsed.Path == "" || parsed.Path == "/" {
			return
		}
		if strings.Contains(parsed.Path, "<") {
			t.Fatalf("doc link is a template %s", cleaned)
		}
		parsed.Fragment = ""
		key := parsed.RequestURI()
		if seen[key] {
			return
		}
		seen[key] = true
		queue = append(queue, key)
	}
	scan := func(label, body string) {
		t.Helper()
		body = config.ExpandPublicBase(body, "")
		lower := strings.ToLower(body)
		if strings.Contains(body, "offerrails.com") || strings.Contains(lower, "open source") || strings.Contains(lower, "open-source") {
			t.Fatalf("%s points at the unpicked domain or says open source", label)
		}
		for _, match := range markdownLink.FindAllStringSubmatch(body, -1) {
			target := match[1]
			if strings.HasPrefix(target, "#") {
				continue
			}
			if strings.HasPrefix(target, "https://offer-rails.onrender.com/") {
				add(target)
				continue
			}
			if strings.HasPrefix(target, "http://") || strings.HasPrefix(target, "https://") {
				continue
			}
			t.Fatalf("%s has a relative doc link %s", label, target)
		}
		for _, match := range hostedURL.FindAllString(body, -1) {
			add(match)
		}
	}

	scan("fallback llms", llmsBody())
	scan("full llms", llmsFullDocument())
	scan("public llms", readRepo(t, root, "ui/landing/public/llms.txt"))
	scan("plugin llms", readRepo(t, root, "affilio-wp/admin/build/landing-v9/llms.txt"))
	scan("plugin llms full", readRepo(t, root, "affilio-wp/admin/build/landing-v9/llms-full.txt"))
	scan("spec", readRepo(t, root, "docs/protocol/SPEC.md"))
	scan("gaps", readRepo(t, root, "docs/protocol/GAPS.md"))
	err := fs.WalkDir(os.DirFS(filepath.Join(root, "docs/protocol/schema/0.3")), ".", func(name string, entry fs.DirEntry, err error) error {
		if err != nil || entry.IsDir() || !strings.HasSuffix(name, ".json") {
			return err
		}
		raw, err := os.ReadFile(filepath.Join(root, "docs/protocol/schema/0.3", name))
		if err != nil {
			return err
		}
		var doc struct {
			ID string `json:"$id"`
		}
		if err = json.Unmarshal(raw, &doc); err != nil {
			return err
		}
		if doc.ID == "" || !strings.HasPrefix(doc.ID, orp.PublicHost+"/orp/schemas/") {
			t.Fatalf("schema %s $id %s", name, doc.ID)
		}
		add(doc.ID)
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	if len(queue) < 10 {
		t.Fatalf("found %d doc links", len(queue))
	}

	for i := 0; i < len(queue); i++ {
		target := queue[i]
		parsed, err := url.Parse(target)
		if err != nil {
			t.Fatal(err)
		}
		if parsed.Path == "/mcp" {
			continue
		}
		rec := httptest.NewRecorder()
		r.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, target, nil))
		if rec.Code != http.StatusOK {
			t.Fatalf("%s: %d %s", target, rec.Code, rec.Body.String())
		}
		want := docContentType(t, parsed.Path)
		if !strings.Contains(rec.Header().Get("Content-Type"), want) {
			t.Fatalf("%s content type %q, want %s", target, rec.Header().Get("Content-Type"), want)
		}
		body := rec.Body.String()
		scan(target, body)
		if want == "application/json" {
			var doc map[string]any
			if err := json.Unmarshal(rec.Body.Bytes(), &doc); err != nil {
				t.Fatalf("%s json: %v", target, err)
			}
			if id, _ := doc["$id"].(string); id != "" && id != orp.PublicHost+parsed.Path {
				t.Fatalf("%s $id %s", target, id)
			}
			if files, ok := doc["files"].([]any); ok && len(files) > 0 {
				base := strings.TrimRight(parsed.Path, "/") + "/"
				for _, item := range files {
					name, _ := item.(string)
					if name == "" || strings.Contains(name, "..") {
						t.Fatalf("%s index file %v", target, item)
					}
					add(orp.PublicHost + base + name)
				}
			}
		}
	}
}

func docContentType(t *testing.T, path string) string {
	t.Helper()
	switch {
	case path == "/orp/spec" || path == "/orp/gaps" || strings.HasPrefix(path, "/docs/"):
		return "text/markdown"
	case path == "/llms.txt" || path == "/llms-full.txt":
		return "text/plain"
	case path == "/skill.md":
		return "text/markdown"
	case path == "/.well-known/orp.json" || path == "/.well-known/dcp.json" || strings.HasPrefix(path, "/orp/schemas") || strings.HasPrefix(path, "/orp/examples"):
		return "application/json"
	case strings.HasPrefix(path, "/landing-v9/"):
		return "text/html"
	default:
		t.Fatalf("no content type for %s", path)
		return ""
	}
}

func readRepo(t *testing.T, root, rel string) string {
	t.Helper()
	body, err := os.ReadFile(filepath.Join(root, rel))
	if err != nil {
		t.Fatal(err)
	}
	return string(body)
}
