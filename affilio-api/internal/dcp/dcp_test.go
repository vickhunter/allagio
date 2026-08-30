package dcp

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

// stubAPI records the last request and answers with a fixed body per route.
func stubAPI(t *testing.T) (*httptest.Server, *http.Request, *[]byte) {
	t.Helper()
	var lastReq http.Request
	var lastBody []byte
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		lastReq = *r
		body := make([]byte, r.ContentLength)
		if r.ContentLength > 0 {
			r.Body.Read(body)
		}
		lastBody = body
		switch {
		case r.URL.Path == "/v1/maker/products" && r.Method == http.MethodPost:
			w.Write([]byte(`{"id":"prod-1","readiness_status":"unassessed"}`))
		case r.URL.Path == "/v1/maker/products/prod-1/assess":
			w.Write([]byte(`{"readiness_status":"ready","readiness_score":92}`))
		case r.URL.Path == "/v1/maker/offers/offer-1/submit":
			w.WriteHeader(http.StatusConflict)
			w.Write([]byte(`{"error":"product has not passed readiness assessment"}`))
		default:
			w.Write([]byte(`{"ok":true}`))
		}
	}))
	t.Cleanup(server.Close)
	return server, &lastReq, &lastBody
}

func TestClientSendsMakerKeyAndDecodes(t *testing.T) {
	server, lastReq, _ := stubAPI(t)
	client := NewClient(server.URL, "maker-key-1")

	result, err := client.Do(http.MethodPost, "/v1/maker/products", map[string]interface{}{"name": "X"})
	if err != nil {
		t.Fatalf("do: %v", err)
	}
	if result["id"] != "prod-1" {
		t.Fatalf("unexpected result %v", result)
	}
	if lastReq.Header.Get("X-Maker-Key") != "maker-key-1" {
		t.Fatalf("expected maker key header, got %q", lastReq.Header.Get("X-Maker-Key"))
	}
}

func TestClientSurfacesAPIErrors(t *testing.T) {
	server, _, _ := stubAPI(t)
	client := NewClient(server.URL, "maker-key-1")

	_, err := client.Do(http.MethodPost, "/v1/maker/offers/offer-1/submit", nil)
	if err == nil || !strings.Contains(err.Error(), "readiness") {
		t.Fatalf("expected readiness error, got %v", err)
	}
}

func TestCallToolRoutesPathParams(t *testing.T) {
	server, lastReq, lastBody := stubAPI(t)
	client := NewClient(server.URL, "maker-key-1")

	result, err := CallTool(client, "run_assessment", map[string]interface{}{
		"product_id": "prod-1",
		"ref_param":  "ref",
	})
	if err != nil {
		t.Fatalf("call: %v", err)
	}
	if result["readiness_status"] != "ready" {
		t.Fatalf("unexpected result %v", result)
	}
	if lastReq.URL.Path != "/v1/maker/products/prod-1/assess" {
		t.Fatalf("unexpected path %s", lastReq.URL.Path)
	}
	// product_id is a path param and must not leak into the body.
	if strings.Contains(string(*lastBody), "product_id") || !strings.Contains(string(*lastBody), "ref_param") {
		t.Fatalf("unexpected body %s", string(*lastBody))
	}

	if _, err := CallTool(client, "run_assessment", map[string]interface{}{}); err == nil {
		t.Fatalf("expected error for missing product_id")
	}
	if _, err := CallTool(client, "does_not_exist", nil); err == nil {
		t.Fatalf("expected error for unknown tool")
	}
}

func rpc(t *testing.T, client *Client, message string) map[string]interface{} {
	t.Helper()
	raw := HandleMessage(client, []byte(message))
	if raw == nil {
		t.Fatalf("expected response for %s", message)
	}
	out := map[string]interface{}{}
	if err := json.Unmarshal(raw, &out); err != nil {
		t.Fatalf("decode: %v", err)
	}
	return out
}

func TestMCPHandshakeAndToolFlow(t *testing.T) {
	server, _, _ := stubAPI(t)
	client := NewClient(server.URL, "maker-key-1")

	init := rpc(t, client, `{"jsonrpc":"2.0","id":1,"method":"initialize","params":{"protocolVersion":"2025-06-18"}}`)
	result := init["result"].(map[string]interface{})
	if result["protocolVersion"] == "" || result["serverInfo"].(map[string]interface{})["name"] != "distributoor" {
		t.Fatalf("unexpected initialize result %v", result)
	}

	// Notifications get no response.
	if resp := HandleMessage(client, []byte(`{"jsonrpc":"2.0","method":"notifications/initialized"}`)); resp != nil {
		t.Fatalf("expected nil for notification, got %s", resp)
	}

	list := rpc(t, client, `{"jsonrpc":"2.0","id":2,"method":"tools/list"}`)
	tools := list["result"].(map[string]interface{})["tools"].([]interface{})
	if len(tools) != len(Tools) {
		t.Fatalf("expected %d tools, got %d", len(Tools), len(tools))
	}

	call := rpc(t, client, `{"jsonrpc":"2.0","id":3,"method":"tools/call","params":{"name":"run_assessment","arguments":{"product_id":"prod-1"}}}`)
	callResult := call["result"].(map[string]interface{})
	if callResult["isError"] != false {
		t.Fatalf("expected success, got %v", callResult)
	}
	text := callResult["content"].([]interface{})[0].(map[string]interface{})["text"].(string)
	if !strings.Contains(text, "ready") {
		t.Fatalf("unexpected tool output %s", text)
	}

	// API errors surface as isError tool results, not protocol errors.
	call = rpc(t, client, `{"jsonrpc":"2.0","id":4,"method":"tools/call","params":{"name":"submit_offer","arguments":{"offer_id":"offer-1"}}}`)
	callResult = call["result"].(map[string]interface{})
	if callResult["isError"] != true {
		t.Fatalf("expected isError for failed submit, got %v", callResult)
	}

	unknown := rpc(t, client, `{"jsonrpc":"2.0","id":5,"method":"bogus/method"}`)
	if unknown["error"] == nil {
		t.Fatalf("expected method-not-found error")
	}
}
