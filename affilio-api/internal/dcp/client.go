// Package dcp implements the Distribution Context Protocol surface: a typed
// client over the maker HTTP API and the MCP tool registry that exposes it to
// agents. The verb names and shapes here are DCP v0.1 (see docs/dcp/) —
// extracted from the working network, not designed apart from it.
package dcp

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"
)

type Client struct {
	BaseURL  string
	MakerKey string
	SiteKey  string // publisher-side verbs authenticate as a site
	HTTP     *http.Client
}

func NewClient(baseURL, makerKey string) *Client {
	return &Client{
		BaseURL:  strings.TrimRight(baseURL, "/"),
		MakerKey: makerKey,
		HTTP:     &http.Client{Timeout: 30 * time.Second},
	}
}

// Do performs one maker-authenticated API call and returns the decoded JSON
// object. Paths are relative to the API root (e.g. "/v1/maker/products").
// A nil payload sends no body. Non-2xx responses come back as errors carrying
// the API's message.
func (c *Client) Do(method, path string, payload interface{}) (map[string]interface{}, error) {
	return c.request(method, path, payload, "X-Maker-Key", c.MakerKey)
}

// DoSite performs one publisher-authenticated API call (site key).
func (c *Client) DoSite(method, path string, payload interface{}) (map[string]interface{}, error) {
	if c.SiteKey == "" {
		return nil, fmt.Errorf("publisher verbs need a site key: set DISTRIBUTOOR_SITE_KEY (the Affilio site api key)")
	}
	return c.request(method, path, payload, "X-API-Key", c.SiteKey)
}

func (c *Client) request(method, path string, payload interface{}, authHeader, authKey string) (map[string]interface{}, error) {
	var body io.Reader
	if payload != nil {
		encoded, err := json.Marshal(payload)
		if err != nil {
			return nil, err
		}
		body = bytes.NewReader(encoded)
	}
	req, err := http.NewRequest(method, c.BaseURL+path, body)
	if err != nil {
		return nil, err
	}
	if payload != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	if authKey != "" {
		req.Header.Set(authHeader, authKey)
	}
	client := c.HTTP
	if client == nil {
		client = &http.Client{Timeout: 30 * time.Second}
	}
	resp, err := client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	raw, err := io.ReadAll(io.LimitReader(resp.Body, 4*1024*1024))
	if err != nil {
		return nil, err
	}
	out := map[string]interface{}{}
	if len(raw) > 0 {
		if err := json.Unmarshal(raw, &out); err != nil {
			return nil, fmt.Errorf("unexpected response (%d): %s", resp.StatusCode, truncate(string(raw), 200))
		}
	}
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		message, _ := out["error"].(string)
		if message == "" {
			message = fmt.Sprintf("request failed with status %d", resp.StatusCode)
		}
		return out, fmt.Errorf("%s (%d)", message, resp.StatusCode)
	}
	return out, nil
}

func truncate(s string, n int) string {
	if len(s) <= n {
		return s
	}
	return s[:n] + "…"
}
