package handlers

import (
	"archive/zip"
	"bytes"
	"io"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"
)

func TestPrototypeRouteIsStatic(t *testing.T) {
	gin.SetMode(gin.TestMode)
	h := &HostedHandler{BaseURL: "https://distribution.example", AssetsPath: "../../../affilio-wp/admin/build"}
	r := gin.New()
	public := r.Group("")
	h.mountWorkspace(public, public.Group("/owner"))
	for _, path := range []string{"/prototype", "/prototype/", "/prototype/share", "/prototype/audience", "/prototype/payments"} {
		rec := httptest.NewRecorder()
		r.ServeHTTP(rec, httptest.NewRequest("GET", path, nil))
		if rec.Code != 200 || !strings.Contains(rec.Body.String(), "Example data") || !strings.Contains(rec.Body.String(), "/assets/prototype.js") {
			t.Fatalf("%s -> %d", path, rec.Code)
		}
	}
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, httptest.NewRequest("GET", "/prototype/secret", nil))
	if rec.Code != 404 {
		t.Fatalf("unknown screen %d", rec.Code)
	}
	rec = httptest.NewRecorder()
	r.ServeHTTP(rec, httptest.NewRequest("GET", "/assets/prototype.js", nil))
	if rec.Code != 200 || strings.Contains(rec.Body.String(), "fetch(") {
		t.Fatalf("script %d", rec.Code)
	}
}

func TestWorkspaceDownloadsUseActualOrigin(t *testing.T) {
	path := t.TempDir()
	var source bytes.Buffer
	writer := zip.NewWriter(&source)
	entry, err := writer.Create("affilio/affilio.php")
	if err != nil {
		t.Fatal(err)
	}
	if _, err = io.WriteString(entry, "<?php define('AFFILIO_API_URL', 'https://affilio-production.up.railway.app/v1');"); err != nil {
		t.Fatal(err)
	}
	if err = writer.Close(); err != nil {
		t.Fatal(err)
	}
	if err = os.WriteFile(filepath.Join(path, "affilio.zip"), source.Bytes(), 0600); err != nil {
		t.Fatal(err)
	}
	h := &HostedHandler{BaseURL: "https://distribution.example", AssetsPath: "../../../affilio-wp/admin/build", DownloadsPath: path}
	r := gin.New()
	r.GET("/", h.Home)
	r.GET("/downloads/affilio.zip", h.DownloadWordPress)
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, httptest.NewRequest("GET", "/downloads/affilio.zip", nil))
	if rec.Code != 200 {
		t.Fatal(rec.Code)
	}
	archive, err := zip.NewReader(bytes.NewReader(rec.Body.Bytes()), int64(rec.Body.Len()))
	if err != nil {
		t.Fatal(err)
	}
	file, err := archive.File[0].Open()
	if err != nil {
		t.Fatal(err)
	}
	data, err := io.ReadAll(file)
	file.Close()
	if err != nil || !strings.Contains(string(data), "https://distribution.example/v1") || strings.Contains(string(data), "affilio-production") {
		t.Fatalf("wrong plugin origin or invalid archive: %v", err)
	}
	rec = httptest.NewRecorder()
	r.ServeHTTP(rec, httptest.NewRequest("GET", "/", nil))
	if rec.Code != 200 || !strings.Contains(rec.Body.String(), "Download WordPress plugin") || strings.Contains(rec.Body.String(), `id="install"`) {
		t.Fatal("home advertises unavailable downloads")
	}
}
