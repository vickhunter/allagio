package handlers

import (
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"
)

var publicCopyBanned = regexp.MustCompile(`(?i)\bads?\b|fully automatic|\bpilot\b|\brevolutionary\b|best-converting`)

func publicCopyHits(text string) []string {
	var hits []string
	for _, loc := range publicCopyBanned.FindAllStringIndex(text, -1) {
		word := text[loc[0]:loc[1]]
		before := ""
		if loc[0] > 0 {
			before = text[loc[0]-1 : loc[0]]
		}
		after := ""
		if loc[1] < len(text) {
			after = text[loc[1] : loc[1]+1]
		}
		starts := before == "" || strings.TrimSpace(before) == ""
		ends := after == "" || strings.TrimSpace(after) == "" || strings.ContainsAny(after, ".,;:!?")
		if !starts || !ends {
			continue
		}
		lineStart := strings.LastIndex(text[:loc[0]], "\n") + 1
		lineEnd := strings.Index(text[loc[1]:], "\n")
		if lineEnd < 0 {
			lineEnd = len(text)
		} else {
			lineEnd += loc[1]
		}
		if len(word) <= 3 && !strings.Contains(strings.TrimSpace(text[lineStart:lineEnd]), " ") {
			continue
		}
		hits = append(hits, word)
	}
	return hits
}

func addPublicCopyHits(t *testing.T, hits *[]string, where, text string) {
	t.Helper()
	for _, word := range publicCopyHits(text) {
		*hits = append(*hits, where+" contains "+word)
	}
}

func stripEmbeddedCode(text string) string {
	text = regexp.MustCompile(`(?is)<script\b[^>]*>.*?</script>`).ReplaceAllString(text, " ")
	text = regexp.MustCompile(`(?is)<style\b[^>]*>.*?</style>`).ReplaceAllString(text, " ")
	return regexp.MustCompile(`(?is)<[^>]+>`).ReplaceAllString(text, " ")
}

func proseStrings(src string) string {
	var b strings.Builder
	var quote byte
	for i := 0; i < len(src); i++ {
		c := src[i]
		if quote == 0 {
			if c == '"' || c == '\'' || c == '`' {
				quote = c
			}
			continue
		}
		if c == '\\' && i+1 < len(src) {
			b.WriteByte(' ')
			i++
			continue
		}
		if quote == '`' && c == '$' && i+1 < len(src) && src[i+1] == '{' {
			depth := 1
			i += 2
			for i < len(src) && depth > 0 {
				if src[i] == '{' {
					depth++
				} else if src[i] == '}' {
					depth--
				}
				i++
			}
			b.WriteByte(' ')
			i--
			continue
		}
		if c == quote {
			b.WriteByte('\n')
			quote = 0
			continue
		}
		b.WriteByte(c)
	}
	return b.String()
}

func walkPublicCopy(t *testing.T, root string, hits *[]string) {
	t.Helper()
	_ = filepath.WalkDir(root, func(path string, entry os.DirEntry, err error) error {
		if err != nil || entry.IsDir() {
			if entry != nil && (entry.Name() == "node_modules" || entry.Name() == "dist" || entry.Name() == "fonts") {
				return filepath.SkipDir
			}
			return nil
		}
		name := entry.Name()
		if strings.HasSuffix(name, "_test.go") || strings.Contains(name, ".test.") || strings.HasSuffix(name, ".d.ts") {
			return nil
		}
		ext := filepath.Ext(name)
		body, err := os.ReadFile(path)
		if err != nil {
			t.Fatal(err)
		}
		text := string(body)
		switch ext {
		case ".html", ".htm":
			text = stripEmbeddedCode(text)
		case ".js":
			text = proseStrings(text)
		case ".go", ".tsx", ".ts", ".mjs", ".txt", ".css", ".php":
		default:
			return nil
		}
		if ext == ".go" {
			var kept []string
			for _, line := range strings.Split(text, "\n") {
				cleaned := strings.ReplaceAll(line, `"/pilot"`, `""`)
				cleaned = strings.ReplaceAll(cleaned, "/pilot", "")
				kept = append(kept, cleaned)
			}
			text = strings.Join(kept, "\n")
		}
		addPublicCopyHits(t, hits, path, text)
		return nil
	})
}

func TestPublicPagesAvoidBannedCopy(t *testing.T) {
	gin.SetMode(gin.TestMode)
	var hits []string

	r := gin.New()
	(&HostedHandler{BaseURL: "http://127.0.0.1:8080"}).mountDemo(r.Group("/"))
	for _, path := range []string{"/demo", "/demo/start", "/demo/pricing", "/demo/docs", "/demo/privacy", "/demo/terms"} {
		rec := httptest.NewRecorder()
		r.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, path, nil))
		if rec.Code != http.StatusOK {
			t.Fatalf("%s: %d", path, rec.Code)
		}
		addPublicCopyHits(t, &hits, path, stripEmbeddedCode(rec.Body.String()))
	}

	addPublicCopyHits(t, &hits, "llms.txt fallback", llmsBody())
	for _, name := range []string{"device-login.md", "settings.md", "listing-card.md", "modes.md", "account.md"} {
		body, ok := agentDoc(name)
		if !ok {
			t.Fatalf("missing %s", name)
		}
		addPublicCopyHits(t, &hits, name, body)
	}
	page := (&MerchantConnectionsHandler{BaseURL: "http://127.0.0.1:8080"}).connectionBody("Notebook", "product", "csrf", "")
	addPublicCopyHits(t, &hits, "stripe connect page", stripEmbeddedCode(string(page)))

	repo := filepath.Join("..", "..", "..")
	walkPublicCopy(t, filepath.Join(repo, "ui", "landing"), &hits)
	walkPublicCopy(t, filepath.Join(repo, "affilio-api"), &hits)
	walkPublicCopy(t, filepath.Join(repo, "affilio-wp"), &hits)

	if len(hits) > 0 {
		t.Fatalf("public copy:\n%s", strings.Join(hits, "\n"))
	}
}
