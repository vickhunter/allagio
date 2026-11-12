package handlers

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func rpc(t *testing.T, r http.Handler, method string, params map[string]any, headers map[string]string) (*httptest.ResponseRecorder, map[string]any) {
	t.Helper()
	return postJSON(t, r, http.MethodPost, "/a2a", map[string]any{
		"jsonrpc": "2.0", "id": "req_1", "method": method, "params": params,
	}, headers)
}

func a2aDataPart(env map[string]any) map[string]any {
	return map[string]any{"type": "data", "data": env}
}

func a2aSend(t *testing.T, r http.Handler, id string, parts []any, headers map[string]string) (*httptest.ResponseRecorder, map[string]any) {
	t.Helper()
	return rpc(t, r, "tasks/send", map[string]any{
		"id": id, "message": map[string]any{"role": "user", "parts": parts},
	}, headers)
}

func agentReplyData(t *testing.T, result map[string]any) map[string]any {
	t.Helper()
	history, _ := result["history"].([]any)
	agent, _ := history[1].(map[string]any)
	parts, _ := agent["parts"].([]any)
	data, _ := parts[0].(map[string]any)["data"].(map[string]any)
	return data
}

// tasks/send forwards an ORP envelope through the same /orp dispatch:
// hello completes, and the reply rides in an agent data part.
func TestA2ASendHello(t *testing.T) {
	_, r := trustHost(t)
	token, party := joinParty(t, r)

	code, body := a2aSend(t, r, "task_hello_01", []any{
		a2aDataPart(orpEnvelope("hello", "m_a2a_hello1", party, map[string]any{"versions": []any{"0.3"}})),
	}, bearerHeader(token))
	if code.Code != http.StatusOK {
		t.Fatalf("send %d %s", code.Code, mustJSON(body))
	}
	result, _ := body["result"].(map[string]any)
	status, _ := result["status"].(map[string]any)
	if status["state"] != "completed" {
		t.Fatalf("task %s", mustJSON(body))
	}
	if data := agentReplyData(t, result); data["type"] != "hello" {
		t.Fatalf("reply %s", mustJSON(result))
	}

	get, getBody := rpc(t, r, "tasks/get", map[string]any{"id": "task_hello_01"}, bearerHeader(token))
	got, _ := getBody["result"].(map[string]any)
	if get.Code != http.StatusOK || got["id"] != "task_hello_01" {
		t.Fatalf("get %d %s", get.Code, mustJSON(getBody))
	}
}

// A waiting marker is input-required — the human step is outside the task.
func TestA2ASendWaitingOffer(t *testing.T) {
	_, r := trustHost(t)
	makerToken, maker := joinParty(t, r)

	code, body := a2aSend(t, r, "task_offer_01", []any{
		a2aDataPart(orpEnvelope("offer", "m_a2a_offer1", maker, standingOffer())),
	}, bearerHeader(makerToken))
	if code.Code != http.StatusOK {
		t.Fatalf("send %d %s", code.Code, mustJSON(body))
	}
	result, _ := body["result"].(map[string]any)
	status, _ := result["status"].(map[string]any)
	if status["state"] != "input-required" {
		t.Fatalf("task %s", mustJSON(body))
	}
}

// An unauthenticated ORP envelope fails the same way POST /orp does.
func TestA2ASendUnauthenticated(t *testing.T) {
	_, r := trustHost(t)
	code, body := a2aSend(t, r, "", []any{
		a2aDataPart(orpEnvelope("hello", "m_a2a_noauth", "orp:x/y", map[string]any{})),
	}, nil)
	if code.Code != http.StatusOK {
		t.Fatalf("send %d %s", code.Code, mustJSON(body))
	}
	result, _ := body["result"].(map[string]any)
	status, _ := result["status"].(map[string]any)
	if status["state"] != "failed" {
		t.Fatalf("task %s", mustJSON(body))
	}
}

// A text part maps to suggest; get on an unknown id is -32001; cancel
// refuses; unknown methods are -32601.
func TestA2AEdges(t *testing.T) {
	_, r := trustHost(t)

	code, body := a2aSend(t, r, "task_text_01", []any{
		map[string]any{"type": "text", "text": "zqtestmention"},
	}, nil)
	result, _ := body["result"].(map[string]any)
	if code.Code != http.StatusOK || result["id"] != "task_text_01" {
		t.Fatalf("text send %d %s", code.Code, mustJSON(body))
	}

	get, getBody := rpc(t, r, "tasks/get", map[string]any{"id": "task_nope"}, nil)
	errObj, _ := getBody["error"].(map[string]any)
	if get.Code != http.StatusOK || errObj["code"] != float64(-32001) {
		t.Fatalf("get %d %s", get.Code, mustJSON(getBody))
	}

	cancel, cancelBody := rpc(t, r, "tasks/cancel", map[string]any{"id": "task_text_01"}, nil)
	errObj, _ = cancelBody["error"].(map[string]any)
	if cancel.Code != http.StatusOK || errObj["code"] != float64(-32002) {
		t.Fatalf("cancel %d %s", cancel.Code, mustJSON(cancelBody))
	}

	other, otherBody := rpc(t, r, "message/send", map[string]any{}, nil)
	errObj, _ = otherBody["error"].(map[string]any)
	if other.Code != http.StatusOK || errObj["code"] != float64(-32601) {
		t.Fatalf("method %d %s", other.Code, mustJSON(otherBody))
	}
}

// tasks/sendSubscribe streams a submitted status then the terminal
// update marked final over text/event-stream.
func TestA2ASendSubscribe(t *testing.T) {
	_, r := trustHost(t)
	token, party := joinParty(t, r)

	payload, _ := json.Marshal(map[string]any{
		"jsonrpc": "2.0", "id": "req_sub", "method": "tasks/sendSubscribe",
		"params": map[string]any{
			"id": "task_sub_01",
			"message": map[string]any{"role": "user", "parts": []any{
				a2aDataPart(orpEnvelope("hello", "m_a2a_sub1", party, map[string]any{"versions": []any{"0.3"}})),
			}},
		},
	})
	req := httptest.NewRequest(http.MethodPost, "/a2a", bytes.NewReader(payload))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+token)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	if w.Code != http.StatusOK || !strings.HasPrefix(w.Header().Get("Content-Type"), "text/event-stream") {
		t.Fatalf("subscribe %d %s", w.Code, w.Header().Get("Content-Type"))
	}
	var events []map[string]any
	for _, chunk := range strings.Split(w.Body.String(), "\n\n") {
		chunk = strings.TrimSpace(strings.TrimPrefix(chunk, "data:"))
		if chunk == "" {
			continue
		}
		var event map[string]any
		if err := json.Unmarshal([]byte(chunk), &event); err != nil {
			t.Fatalf("event %q: %v", chunk, err)
		}
		events = append(events, event)
	}
	if len(events) != 2 {
		t.Fatalf("events %d %s", len(events), w.Body.String())
	}
	first, _ := events[0]["result"].(map[string]any)
	if first["status"].(map[string]any)["state"] != "submitted" || first["final"] != false {
		t.Fatalf("first %s", w.Body.String())
	}
	last, _ := events[1]["result"].(map[string]any)
	if last["status"].(map[string]any)["state"] != "completed" || last["final"] != true {
		t.Fatalf("last %s", w.Body.String())
	}
}
