package handlers

import (
	"bytes"
	"compress/gzip"
	"context"
	"encoding/base64"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"
)

func encodeScanField(t *testing.T, value, encoding string) string {
	t.Helper()
	raw := []byte(value)
	if encoding == "gzip+base64" {
		var buf bytes.Buffer
		zw := gzip.NewWriter(&buf)
		if _, err := zw.Write(raw); err != nil {
			t.Fatal(err)
		}
		if err := zw.Close(); err != nil {
			t.Fatal(err)
		}
		raw = buf.Bytes()
	}
	return base64.StdEncoding.EncodeToString(raw)
}

func postScan(t *testing.T, single bool, payload interface{}) *httptest.ResponseRecorder {
	t.Helper()
	gin.SetMode(gin.TestMode)
	handler := &ScanHandler{}
	router := gin.New()
	path := "/scan/full"
	router.POST(path, func(c *gin.Context) {
		defer func() { _ = recover() }()
		setSiteID(c, "site")
		handler.ScanFull(c)
	})
	router.POST("/scan/single", func(c *gin.Context) {
		defer func() { _ = recover() }()
		setSiteID(c, "site")
		handler.ScanSingle(c)
	})
	if single {
		path = "/scan/single"
	}
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, httptest.NewRequest(http.MethodPost, path, newJSONRequest(t, http.MethodPost, path, payload)))
	return rec
}

func TestScanRejectsMalformedAndOversizedEncodings(t *testing.T) {
	oversized := strings.Repeat("a", (1<<20)+1)
	longTitle := strings.Repeat("t", 513)
	cases := []struct {
		name    string
		single  bool
		payload interface{}
	}{
		{name: "bad base64", single: true, payload: map[string]interface{}{"wp_post_id": 1, "content_b64": "!!!", "content_encoding": "base64", "url": "https://example.com/p"}},
		{name: "bad gzip", single: true, payload: map[string]interface{}{"wp_post_id": 1, "content_b64": base64.StdEncoding.EncodeToString([]byte("not-gzip")), "content_encoding": "gzip+base64", "url": "https://example.com/p"}},
		{name: "unknown encoding", single: true, payload: map[string]interface{}{"wp_post_id": 1, "content_b64": encodeScanField(t, "hello", "base64"), "content_encoding": "rot13", "url": "https://example.com/p"}},
		{name: "both raw and encoded", single: true, payload: map[string]interface{}{"wp_post_id": 1, "content": "raw", "content_b64": encodeScanField(t, "encoded", "base64"), "content_encoding": "base64", "url": "https://example.com/p"}},
		{name: "raw content too large", single: true, payload: map[string]interface{}{"wp_post_id": 1, "title": "T", "content": oversized, "url": "https://example.com/p"}},
		{name: "decoded content too large", single: true, payload: map[string]interface{}{"wp_post_id": 1, "title_b64": encodeScanField(t, "T", "gzip+base64"), "content_b64": encodeScanField(t, oversized, "gzip+base64"), "content_encoding": "gzip+base64", "url": "https://example.com/p"}},
		{name: "raw title too large", single: false, payload: map[string]interface{}{"articles": []map[string]interface{}{{"wp_post_id": 1, "title": longTitle, "content": "ok", "url": "https://example.com/p"}}}},
		{name: "decoded title too large", single: false, payload: map[string]interface{}{"articles": []map[string]interface{}{{"wp_post_id": 1, "title_b64": encodeScanField(t, longTitle, "base64"), "content_b64": encodeScanField(t, "ok", "base64"), "content_encoding": "base64", "url": "https://example.com/p"}}}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			rec := postScan(t, tc.single, tc.payload)
			if rec.Code != http.StatusBadRequest {
				t.Fatalf("expected 400, got %d body %s", rec.Code, rec.Body.String())
			}
		})
	}
}

func TestScanAcceptsRawAndEncodedBodies(t *testing.T) {
	db, cleanup := setupTestDB(t)
	defer cleanup()
	siteID := createTestSite(t, db, "test-key", "example.com", "Example Site")
	handler := &ScanHandler{DB: db, Classifier: nil}
	gin.SetMode(gin.TestMode)
	text := "We compared Semrush for SEO research and ranking insights."
	payloads := []map[string]interface{}{
		{"wp_post_id": 1, "title": "SEO Tools", "content": text, "url": "https://example.com/seo-tools"},
		{"wp_post_id": 2, "title_b64": encodeScanField(t, "SEO Tools", "base64"), "content_b64": encodeScanField(t, text, "base64"), "content_encoding": "base64", "url": "https://example.com/seo-tools-b64"},
		{"wp_post_id": 3, "title_b64": encodeScanField(t, "SEO Tools", "gzip+base64"), "content_b64": encodeScanField(t, text, "gzip+base64"), "content_encoding": "gzip+base64", "url": "https://example.com/seo-tools-gzip"},
	}
	for _, payload := range payloads {
		router := gin.New()
		router.POST("/scan/single", func(c *gin.Context) {
			setSiteID(c, siteID)
			handler.ScanSingle(c)
		})
		rec := httptest.NewRecorder()
		router.ServeHTTP(rec, httptest.NewRequest(http.MethodPost, "/scan/single", newJSONRequest(t, http.MethodPost, "/scan/single", payload)))
		if rec.Code != http.StatusOK {
			t.Fatalf("wp_post_id %v: expected 200, got %d %s", payload["wp_post_id"], rec.Code, rec.Body.String())
		}
		var body map[string]interface{}
		if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
			t.Fatal(err)
		}
		recs, _ := body["recommendations"].([]interface{})
		if len(recs) == 0 {
			t.Fatalf("wp_post_id %v: encoded or raw content was not analyzed: %s", payload["wp_post_id"], rec.Body.String())
		}
	}
	stored := []string{}
	rows, err := db.Pool.Query(context.Background(), `SELECT title FROM articles WHERE site_id=$1 ORDER BY wp_post_id`, siteID)
	if err != nil {
		t.Fatal(err)
	}
	defer rows.Close()
	for rows.Next() {
		var title string
		if err := rows.Scan(&title); err != nil {
			t.Fatal(err)
		}
		stored = append(stored, title)
	}
	if strings.Join(stored, ",") != "SEO Tools,SEO Tools,SEO Tools" {
		t.Fatalf("titles were not decoded before save: %v", stored)
	}
}
