package handlers

import (
	"archive/zip"
	"bytes"
	"io"
	"net/http/httptest"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"
)

func repoRoot(t *testing.T) string {
	t.Helper()
	_, file, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("cannot locate the download test")
	}
	return filepath.Clean(filepath.Join(filepath.Dir(file), "..", "..", ".."))
}

func hostedBuildFile(name string) bool {
	switch name {
	case "home.html", "agent-poc.html", "setup-controls.js", "THIRD_PARTY_NOTICES.md":
		return true
	}
	return strings.HasPrefix(name, "workspace") || strings.HasPrefix(name, "landing") || strings.HasPrefix(name, "package")
}

func releasePluginZip(t *testing.T, mutateMain func([]byte) []byte) []byte {
	t.Helper()
	root := filepath.Join(repoRoot(t), "affilio-wp")
	var output bytes.Buffer
	writer := zip.NewWriter(&output)
	err := filepath.Walk(root, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		rel, err := filepath.Rel(filepath.Dir(root), path)
		if err != nil {
			return err
		}
		rel = filepath.ToSlash(rel)
		if info.IsDir() {
			base := filepath.Base(path)
			if base == "node_modules" || base == "e2e" || base == "tests" {
				return filepath.SkipDir
			}
			return nil
		}
		if strings.Contains(rel, "/node_modules/") || strings.HasPrefix(rel, "affilio-wp/e2e/") || strings.HasPrefix(rel, "affilio-wp/tests/") {
			return nil
		}
		if strings.HasPrefix(rel, "affilio-wp/admin/build/") && hostedBuildFile(filepath.Base(path)) {
			return nil
		}
		data, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		if rel == "affilio-wp/affilio.php" && mutateMain != nil {
			data = mutateMain(data)
		}
		header := &zip.FileHeader{Name: rel, Method: zip.Deflate}
		entry, err := writer.CreateHeader(header)
		if err != nil {
			return err
		}
		_, err = entry.Write(data)
		return err
	})
	if err != nil {
		t.Fatal(err)
	}
	if err = writer.Close(); err != nil {
		t.Fatal(err)
	}
	return output.Bytes()
}

func requireValidPluginZip(t *testing.T, data []byte) {
	t.Helper()
	cmd := exec.Command("python3", "-c", `
import sys
sys.path.insert(0, sys.argv[1])
import mvp_release
mvp_release.validate_plugin(sys.stdin.buffer.read())
`, filepath.Join(repoRoot(t), "scripts"))
	cmd.Stdin = bytes.NewReader(data)
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("served zip failed validate_plugin: %v\n%s", err, out)
	}
}

func servedPlugin(t *testing.T, h *HostedHandler, payload []byte) []byte {
	t.Helper()
	if err := os.WriteFile(filepath.Join(h.DownloadsPath, "affilio.zip"), payload, 0600); err != nil {
		t.Fatal(err)
	}
	r := gin.New()
	r.GET("/", h.Home)
	r.GET("/downloads/affilio.zip", h.DownloadWordPress)
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, httptest.NewRequest("GET", "/downloads/affilio.zip", nil))
	if rec.Code != 200 {
		t.Fatalf("download status %d", rec.Code)
	}
	if strings.Contains(rec.Body.String(), "railway.app") {
		t.Fatal("served plugin zip contains railway.app")
	}
	archive, err := zip.NewReader(bytes.NewReader(rec.Body.Bytes()), int64(rec.Body.Len()))
	if err != nil {
		t.Fatal(err)
	}
	var main []byte
	for _, file := range archive.File {
		if strings.HasPrefix(file.Name, "affilio/") && !strings.HasPrefix(file.Name, "affilio-wp/") {
			t.Fatalf("served zip still uses the old folder: %s", file.Name)
		}
		if file.Name != "affilio-wp/affilio.php" {
			continue
		}
		reader, err := file.Open()
		if err != nil {
			t.Fatal(err)
		}
		main, err = io.ReadAll(reader)
		reader.Close()
		if err != nil {
			t.Fatal(err)
		}
	}
	if !bytes.Contains(main, []byte("Version: 0.1.21")) {
		t.Fatal("served zip is missing affilio-wp/affilio.php 0.1.21")
	}
	requireValidPluginZip(t, rec.Body.Bytes())
	home := httptest.NewRecorder()
	r.ServeHTTP(home, httptest.NewRequest("GET", "/", nil))
	if home.Code != 200 || !strings.Contains(home.Body.String(), "Download WordPress plugin") || strings.Contains(home.Body.String(), `id="install"`) {
		t.Fatal("home advertises unavailable downloads")
	}
	return main
}

func TestWorkspaceDownloadsUseActualOrigin(t *testing.T) {
	path := t.TempDir()
	h := &HostedHandler{BaseURL: "https://distribution.example", AssetsPath: filepath.Join(repoRoot(t), "affilio-wp", "admin", "build"), DownloadsPath: path}
	clean := servedPlugin(t, h, releasePluginZip(t, nil))
	if bytes.Contains(clean, []byte("railway.app")) {
		t.Fatal("clean 0.1.21 package gained a railway host")
	}
	baked := releasePluginZip(t, func(main []byte) []byte {
		return append(main, []byte("\ndefine('AFFILIO_API_URL', 'https://affilio-production.up.railway.app/v1');\n")...)
	})
	rewritten := servedPlugin(t, h, baked)
	if !bytes.Contains(rewritten, []byte("https://distribution.example/v1")) || bytes.Contains(rewritten, []byte("railway.app")) || bytes.Contains(rewritten, []byte("affilio-production")) {
		t.Fatal("download did not retarget affilio-wp/affilio.php to the service origin")
	}
}
