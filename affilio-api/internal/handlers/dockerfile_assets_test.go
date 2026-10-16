package handlers

import (
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"
)

// Every HTML template the hosted server loads from AssetsPath must be copied
// into /app/assets by Dockerfile.mvp, or the route answers 500 in production.
func TestDockerfileCopiesServerTemplates(t *testing.T) {
	src, err := os.ReadFile("workspace.go")
	if err != nil {
		t.Fatal(err)
	}
	names := map[string]bool{}
	for _, re := range []*regexp.Regexp{
		regexp.MustCompile(`h\.landing\(c, "([^"]+\.html)"\)`),
		regexp.MustCompile(`filepath\.Join\(h\.AssetsPath, "([^"]+\.html)"\)`),
	} {
		for _, m := range re.FindAllStringSubmatch(string(src), -1) {
			names[m[1]] = true
		}
	}
	if len(names) == 0 {
		t.Fatal("found no templates in workspace.go")
	}
	df, err := os.ReadFile(filepath.Join("..", "..", "..", "Dockerfile.mvp"))
	if err != nil {
		t.Fatal(err)
	}
	var copied []string
	for _, line := range strings.Split(string(df), "\n") {
		f := strings.Fields(line)
		if len(f) > 2 && f[0] == "COPY" && f[len(f)-1] == "/app/assets/" {
			copied = append(copied, f[1:len(f)-1]...)
		}
	}
	for name := range names {
		want := "affilio-wp/admin/build/" + name
		found := false
		for _, c := range copied {
			if c == want {
				found = true
			}
		}
		if !found {
			t.Errorf("Dockerfile.mvp does not copy %s into /app/assets/", want)
		}
		if _, err := os.Stat(filepath.Join("..", "..", "..", want)); err != nil {
			t.Errorf("template %s missing from repo: %v", want, err)
		}
	}
}
