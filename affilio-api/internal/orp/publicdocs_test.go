package orp

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"html"
	"io/fs"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"testing"
)

func TestPublicBundleMatchesRepo(t *testing.T) {
	root, ok := protocolRoot()
	if !ok {
		t.Fatal("docs/protocol not found")
	}
	repo := mappedDocs{root: root}
	bundled, err := fs.Sub(publicBundle, "bundled")
	if err != nil {
		t.Fatal(err)
	}
	repoFiles := map[string][]byte{}
	collect := func(fsys fs.FS, into map[string][]byte) {
		t.Helper()
		for _, name := range []string{"SPEC.md", "GAPS.md"} {
			body, err := fs.ReadFile(fsys, name)
			if err != nil {
				t.Fatalf("%s: %v", name, err)
			}
			into[name] = body
		}
		for _, dir := range []string{"schemas", "examples"} {
			err := fs.WalkDir(fsys, dir, func(entryPath string, entry fs.DirEntry, err error) error {
				if err != nil || entry.IsDir() {
					return err
				}
				body, err := fs.ReadFile(fsys, entryPath)
				if err != nil {
					return err
				}
				into[entryPath] = body
				return nil
			})
			if err != nil {
				t.Fatal(err)
			}
		}
	}
	collect(repo, repoFiles)
	bundleFiles := map[string][]byte{}
	collect(bundled, bundleFiles)
	if len(repoFiles) != len(bundleFiles) {
		t.Fatalf("bundle has %d files and the repo has %d", len(bundleFiles), len(repoFiles))
	}
	for name, want := range repoFiles {
		got, ok := bundleFiles[name]
		if !ok {
			t.Fatalf("bundle missing %s", name)
		}
		if !bytes.Equal(want, got) {
			t.Fatalf("bundle drifted from %s", name)
		}
		if bytes.Contains(want, []byte("offerrails.com")) || bytes.Contains(bytes.ToLower(want), []byte("open source")) {
			t.Fatalf("served doc %s points at the unpicked domain or says open source", name)
		}
	}
}

// TestMarkdownTwinsMatchHTMLPages fails when a public page's visible words
// leave its Markdown twin.
func TestMarkdownTwinsMatchHTMLPages(t *testing.T) {
	repo := filepath.Dir(filepath.Dir(mustProtocolRoot(t)))
	pages := []struct{ html, md string }{
		{"affilio-wp/admin/build/home.html", "affilio-wp/admin/build/landing-v9/home.md"},
		{"affilio-wp/admin/build/makers.html", "affilio-wp/admin/build/landing-v9/makers.md"},
		{"affilio-wp/admin/build/publishers.html", "affilio-wp/admin/build/landing-v9/publishers.md"},
		{"affilio-wp/admin/build/publishers-setup.html", "affilio-wp/admin/build/landing-v9/publishers-setup.md"},
		{"affilio-wp/admin/build/protocol.html", "affilio-wp/admin/build/landing-v9/protocol.md"},
		{"affilio-wp/admin/build/you.html", "affilio-wp/admin/build/landing-v9/you.md"},
		{"affilio-wp/admin/build/landing-v9/privacy/index.html", "affilio-wp/admin/build/landing-v9/privacy.md"},
		{"affilio-wp/admin/build/landing-v9/terms/index.html", "affilio-wp/admin/build/landing-v9/terms.md"},
		{"affilio-wp/admin/build/landing-v9/agents/index.html", "affilio-wp/admin/build/landing-v9/agents.md"},
	}
	for _, page := range pages {
		body, err := os.ReadFile(filepath.Join(repo, page.html))
		if err != nil {
			t.Fatal(err)
		}
		twin, err := os.ReadFile(filepath.Join(repo, page.md))
		if err != nil {
			t.Fatal(err)
		}
		text := string(twin)
		if strings.HasSuffix(page.md, "you.md") && !strings.Contains(text, "## Connect a hosted assistant") {
			t.Fatal("you.md is missing Connect a hosted assistant")
		}
		for _, phrase := range pagePhrases(string(body)) {
			if !strings.Contains(text, phrase) {
				t.Fatalf("%s dropped %q", page.md, phrase)
			}
		}
		check := text
		if strings.HasSuffix(page.md, "you.md") {
			skill, err := os.ReadFile(filepath.Join(repo, "affilio-wp/admin/build/landing-v9/skill.md"))
			if err != nil {
				t.Fatal(err)
			}
			youSection, youOK := connectSection(text)
			skillSection, skillOK := connectSection(string(skill))
			if !youOK || !skillOK || youSection != connectBlock || skillSection != connectBlock {
				t.Fatal("connect section does not match the shared block")
			}
			check = strings.Split(text, "## Connect a hosted assistant")[0]
		}
		allowed := strings.Join(pagePhrases(string(body)), "\n")
		if strings.HasSuffix(page.md, "privacy.md") || strings.HasSuffix(page.md, "terms.md") {
			legal, err := os.ReadFile(filepath.Join(repo, "ui/landing/src/legal/copy.ts"))
			if err != nil {
				t.Fatal(err)
			}
			allowed += "\n" + string(legal)
		}
		for _, line := range strings.Split(check, "\n") {
			line = strings.TrimSpace(line)
			if line == "" {
				continue
			}
			if !strings.Contains(allowed, line) {
				t.Fatalf("%s has a sentence that is not on the page: %q", page.md, line)
			}
		}
	}
}

// connectBlock is the shared connect section. Both copies are checked
// against this text, so adding a sentence to you.md and skill.md fails.
const connectBlock = "## Connect a hosted assistant\n\n" +
	"POST /v1/agent-tokens with {}.\n" +
	"Send the access_token as Authorization: Bearer to /mcp.\n" +
	"The bearer is not an X-API-Key or X-Admin-Key.\n" +
	"POST /orp/join with {}.\n" +
	"Read tools/list on /mcp.\n" +
	"No sign-in link.\n" +
	"A new site stores the publish step as ask, so prepare waits for a person. A stored auto still waits for a person until the domain is verified and claim checks are live, and only then does prepare commit. A site key can set that publish step to auto. Ask me every time, Approve within my rules, and Approve anything that passes the checks are a separate control for the decide step. An agent tool can only tighten a site or offer mode. While claims are not checked, Approve within my rules and Approve anything that passes the checks still wait for a person.\n" +
	"On an ORP 0.4 deal, GET /orp/deals/{deal} returns approved_by.\n" +
	"Each entry has step, party, and agent or owner.\n" +
	"A 0.3 deal does not include approved_by.\n" +
	"When that read fails, an ORP 0.4 deal stays HTTP 200 and sets approvals_unavailable to true.\n" +
	"approved_by is omitted.\n" +
	"A 0.3 deal does not include approvals_unavailable.\n" +
	"The deal steps are in /llms-full.txt.\n"

func connectSection(text string) (string, bool) {
	const marker = "## Connect a hosted assistant\n"
	i := strings.Index(text, marker)
	if i < 0 {
		return "", false
	}
	return text[i:], true
}

func mustProtocolRoot(t *testing.T) string {
	t.Helper()
	root, ok := protocolRoot()
	if !ok {
		t.Fatal("docs/protocol not found")
	}
	return root
}

func pagePhrases(src string) []string {
	chunk := ""
	if i := strings.Index(src, `id="static-landing"`); i >= 0 {
		chunk = src[i:]
		if j := strings.Index(chunk, `<div id="root"`); j >= 0 {
			chunk = chunk[:j]
		}
	}
	var out []string
	seen := map[string]bool{}
	add := func(s string) {
		s = html.UnescapeString(regexp.MustCompile(`<[^>]+>`).ReplaceAllString(s, ""))
		s = strings.Join(strings.Fields(s), " ")
		if s == "" || s == "Skip to content" || strings.Contains(s, "{{") || seen[s] {
			return
		}
		seen[s] = true
		out = append(out, s)
	}
	if m := regexp.MustCompile(`(?s)<title>(.*?)</title>`).FindStringSubmatch(src); m != nil {
		add(m[1])
	}
	if m := regexp.MustCompile(`name="description" content="([^"]*)"`).FindStringSubmatch(src); m != nil {
		add(m[1])
	}
	if m := regexp.MustCompile(`(?s)data-draft-banner[^>]*>(.*?)</`).FindStringSubmatch(src); m != nil {
		add(m[1])
	}
	chunk = regexp.MustCompile(`(?s)\{\{if .*?\}\}(.*?)\{\{else\}\}(.*?)\{\{end\}\}`).ReplaceAllString(chunk, "$1\n$2")
	for _, tag := range []string{"h1", "h2", "h3", "p", "a", "li", "figcaption"} {
		re := regexp.MustCompile(`(?s)<` + tag + `\b[^>]*>(.*?)</` + tag + `>`)
		for _, m := range re.FindAllStringSubmatch(chunk, -1) {
			for _, part := range strings.Split(m[1], "\n") {
				add(part)
			}
		}
	}
	return out
}

// TestSchemaIDsUseOneHost fails when ORP $id values use more than one host,
// or a host other than SchemaIDHost.
func TestSchemaIDsUseOneHost(t *testing.T) {
	repo := filepath.Dir(filepath.Dir(mustProtocolRoot(t)))
	roots := []string{
		filepath.Join(repo, "docs", "protocol", "schema"),
		filepath.Join(repo, "affilio-api", "internal", "orp", "bundled", "schemas"),
	}
	hosts := map[string][]string{}
	idRe := regexp.MustCompile(`"\$id"\s*:\s*"(https?://[^"/]+)`)
	for _, root := range roots {
		err := filepath.WalkDir(root, func(path string, entry fs.DirEntry, err error) error {
			if err != nil || entry.IsDir() || !strings.HasSuffix(path, ".json") {
				return err
			}
			body, err := os.ReadFile(path)
			if err != nil {
				return err
			}
			var doc map[string]any
			if json.Unmarshal(body, &doc) != nil {
				return nil
			}
			for _, match := range idRe.FindAllSubmatch(body, -1) {
				host := string(match[1])
				hosts[host] = append(hosts[host], path)
			}
			return nil
		})
		if err != nil {
			t.Fatal(err)
		}
	}
	if len(hosts) != 1 {
		t.Fatalf("ORP $id hosts = %v", hosts)
	}
	for host := range hosts {
		if host != SchemaIDHost {
			t.Fatalf("$id host %s, want %s", host, SchemaIDHost)
		}
	}
}

// frozenORP03Roots are the ORP 0.3 files that stay byte-identical to the
// re-frozen state of 2026-11-12 (a §19 change-control amendment on top of
// 8a6c6fa: the 0.3 tracked-link docs landed post-freeze and the bundle was
// re-synced; 8a6c6fa was itself the rename + optional consumer attestation
// amendment on top of f203ca97), including SPEC.md and GAPS.md in the repo
// and the bundle.
var frozenORP03Roots = []string{
	"docs/protocol/SPEC.md",
	"docs/protocol/GAPS.md",
	"docs/protocol/schema/0.3",
	"docs/protocol/schema/examples",
	"affilio-api/internal/orp/bundled/SPEC.md",
	"affilio-api/internal/orp/bundled/GAPS.md",
	"affilio-api/internal/orp/bundled/schemas/messages",
	"affilio-api/internal/orp/bundled/schemas/http",
	"affilio-api/internal/orp/bundled/schemas/envelope.schema.json",
	"affilio-api/internal/orp/bundled/schemas/message.schema.json",
	"affilio-api/internal/orp/bundled/schemas/defs.schema.json",
	"affilio-api/internal/orp/bundled/schemas/discovery.schema.json",
	"affilio-api/internal/orp/bundled/schemas/constraints.schema.json",
}

// frozenORP03SHA256 is sha256 over those files at the 2026-11-12
// re-freeze, sorted by repo-relative path. Each file contributes
// "<path>\n<len>\n" and its bytes. A shallow checkout has no 8a6c6fa
// object, so the test uses this digest and does not require git history.
const frozenORP03SHA256 = "4d8b135d534bc1cf7b9423f1f6bd64fbc178abbaa31afc866acef3b96d424a4e"

func frozenORP03Digest(repo string) (string, error) {
	var files []string
	for _, rel := range frozenORP03Roots {
		abs := filepath.Join(repo, filepath.FromSlash(rel))
		info, err := os.Stat(abs)
		if err != nil {
			return "", err
		}
		if !info.IsDir() {
			files = append(files, rel)
			continue
		}
		err = filepath.WalkDir(abs, func(path string, entry os.DirEntry, err error) error {
			if err != nil || entry.IsDir() {
				return err
			}
			item, err := filepath.Rel(repo, path)
			if err != nil {
				return err
			}
			files = append(files, filepath.ToSlash(item))
			return nil
		})
		if err != nil {
			return "", err
		}
	}
	sort.Strings(files)
	if len(files) != 146 {
		return "", fmt.Errorf("frozen ORP 0.3 file count %d, want 146", len(files))
	}
	sum := sha256.New()
	for _, rel := range files {
		body, err := os.ReadFile(filepath.Join(repo, filepath.FromSlash(rel)))
		if err != nil {
			return "", err
		}
		if _, err := sum.Write([]byte(rel + "\n" + strconv.Itoa(len(body)) + "\n")); err != nil {
			return "", err
		}
		if _, err := sum.Write(body); err != nil {
			return "", err
		}
	}
	return hex.EncodeToString(sum.Sum(nil)), nil
}

// TestORP03MatchesFrozenCommit fails when SPEC, GAPS, 0.3 schemas, or
// examples differ from the frozen main commit. The check is a content digest,
// so a shallow checkout still fails on a real edit. When f203ca97 is present,
// a mismatch also prints git diff.
func TestORP03MatchesFrozenCommit(t *testing.T) {
	repo := filepath.Dir(filepath.Dir(mustProtocolRoot(t)))
	got, err := frozenORP03Digest(repo)
	if err != nil {
		t.Fatal(err)
	}
	if got == frozenORP03SHA256 {
		return
	}
	check := exec.Command("git", "cat-file", "-e", "8a6c6fa^{commit}")
	check.Dir = repo
	if check.Run() != nil {
		t.Fatalf("ORP 0.3 digest %s, want %s (frozen commit 8a6c6fa is not in this checkout)", got, frozenORP03SHA256)
	}
	cmd := exec.Command("git", "diff", "--exit-code", "8a6c6fa", "--")
	cmd.Args = append(cmd.Args, frozenORP03Roots...)
	cmd.Dir = repo
	out, diffErr := cmd.CombinedOutput()
	t.Fatalf("ORP 0.3 digest %s, want %s: %v\n%s", got, frozenORP03SHA256, diffErr, out)
}
