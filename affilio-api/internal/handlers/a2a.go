package handlers

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"time"

	"affilio-api/internal/orp"

	"github.com/gin-gonic/gin"
)

// A2A serves JSON-RPC 2.0 task transport at POST /a2a. A tasks/send
// message carries an ORP envelope in a DataPart — the handler forwards it
// to this same engine's POST /orp so auth, validation, idempotent replay,
// waiting markers, and inbox delivery are identical to the wire endpoint.
// A TextPart message maps to POST /orp/suggest. A waiting reply is
// input-required; a problem is failed; anything else is completed.
type A2A struct {
	Entry  *AgentEntry
	Engine http.Handler
}

func (a *A2A) Mount(r gin.IRouter) {
	r.POST("/a2a", a.RPC)
}

type rpcRequest struct {
	Version string          `json:"jsonrpc"`
	Method  string          `json:"method"`
	ID      json.RawMessage `json:"id"`
	Params  json.RawMessage `json:"params"`
}

func (a *A2A) RPC(c *gin.Context) {
	c.Request.Body = http.MaxBytesReader(c.Writer, c.Request.Body, 1<<20)
	var req rpcRequest
	if err := json.NewDecoder(c.Request.Body).Decode(&req); err != nil {
		a.rpcError(c, nil, -32700, "Parse error")
		return
	}
	switch req.Method {
	case "tasks/send":
		a.send(c, req)
	case "tasks/get":
		a.get(c, req)
	case "tasks/cancel":
		a.rpcError(c, req.ID, -32002, "Tasks are synchronous and cannot be canceled.")
	default:
		a.rpcError(c, req.ID, -32601, "Method not found")
	}
}

type sendParams struct {
	ID        string `json:"id"`
	SessionID string `json:"sessionId"`
	Message   struct {
		Role  string `json:"role"`
		Parts []struct {
			Type string         `json:"type"`
			Text string         `json:"text"`
			Data map[string]any `json:"data"`
		} `json:"parts"`
	} `json:"message"`
}

func (a *A2A) send(c *gin.Context, req rpcRequest) {
	var p sendParams
	if err := json.Unmarshal(req.Params, &p); err != nil || len(p.Message.Parts) == 0 {
		a.rpcError(c, req.ID, -32602, "Invalid params: tasks/send needs a message with parts.")
		return
	}
	if p.ID == "" {
		p.ID = orp.LocalID("a2a_")
	}
	ownerID := a.ownerID(c)
	if doc, found, err := a.task(c, p.ID, ownerID); err == nil && found {
		a.rpcResult(c, req.ID, doc)
		return
	}

	var path string
	var body []byte
	var bearer bool
	for _, part := range p.Message.Parts {
		switch part.Type {
		case "data":
			if env, ok := part.Data["orp"].(string); ok && (env == "0.3" || env == "0.4") {
				path = "/orp"
				body, _ = json.Marshal(part.Data)
				bearer = true
			}
		case "text":
			if path == "" {
				path = "/orp/suggest"
				body, _ = json.Marshal(map[string]any{"text": part.Text, "limit": 5})
			}
		}
		if path != "" {
			break
		}
	}
	if path == "" {
		a.rpcError(c, req.ID, -32602, "No supported part: send a data part with an ORP envelope or a text part for suggest.")
		return
	}

	status, reply := a.forward(c, path, body, bearer)
	doc := a.taskDoc(p, reply, status, c)
	raw, err := json.Marshal(doc)
	if err != nil {
		a.rpcError(c, req.ID, -32603, "Internal error")
		return
	}
	if a.Entry != nil && a.Entry.DB != nil {
		if err := a.Entry.DB.SaveA2ATask(c.Request.Context(), p.ID, ownerID, p.SessionID, raw); err != nil {
			a.rpcError(c, req.ID, -32603, "Internal error")
			return
		}
	}
	a.rpcResult(c, req.ID, json.RawMessage(raw))
}

func (a *A2A) get(c *gin.Context, req rpcRequest) {
	var p struct {
		ID string `json:"id"`
	}
	if err := json.Unmarshal(req.Params, &p); err != nil || p.ID == "" {
		a.rpcError(c, req.ID, -32602, "Invalid params: tasks/get needs id.")
		return
	}
	doc, found, err := a.task(c, p.ID, a.ownerID(c))
	if err != nil {
		a.rpcError(c, req.ID, -32603, "Internal error")
		return
	}
	if !found {
		a.rpcError(c, req.ID, -32001, "Task not found")
		return
	}
	a.rpcResult(c, req.ID, doc)
}

// forward runs one in-process POST against this engine, keeping the
// caller's bearer and client IP so rate limits and credential checks see
// the real caller.
func (a *A2A) forward(c *gin.Context, path string, body []byte, bearer bool) (int, map[string]any) {
	req := httptest.NewRequest(http.MethodPost, path, bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	if bearer {
		req.Header.Set("Authorization", c.GetHeader("Authorization"))
	}
	if ip := c.ClientIP(); ip != "" {
		req.RemoteAddr = ip + ":0"
	}
	rec := httptest.NewRecorder()
	a.Engine.ServeHTTP(rec, req)
	var reply map[string]any
	_ = json.Unmarshal(rec.Body.Bytes(), &reply)
	return rec.Code, reply
}

// taskDoc shapes the A2A Task object. The ORP reply rides in an agent
// DataPart; waiting replies mark input-required, problems mark failed.
func (a *A2A) taskDoc(p sendParams, reply map[string]any, status int, c *gin.Context) map[string]any {
	state := "completed"
	switch {
	case status >= 400:
		state = "failed"
	default:
		if body, ok := reply["body"].(map[string]any); ok && body["waiting"] != nil {
			state = "input-required"
		}
	}
	return map[string]any{
		"id":        p.ID,
		"sessionId": p.SessionID,
		"status":    map[string]any{"state": state, "timestamp": time.Now().UTC().Format(time.RFC3339)},
		"history": []any{
			map[string]any{"role": "user", "parts": p.Message.Parts},
			map[string]any{"role": "agent", "parts": []any{map[string]any{"type": "data", "data": reply}}},
		},
		"metadata": map[string]any{"orp": true, "http_status": status},
	}
}

func (a *A2A) ownerID(c *gin.Context) string {
	if a.Entry == nil {
		return ""
	}
	grant, err := a.Entry.JoinGrant(c)
	if err != nil || grant == nil {
		return ""
	}
	return grant.OwnerID
}

func (a *A2A) task(c *gin.Context, id, ownerID string) (json.RawMessage, bool, error) {
	if a.Entry == nil || a.Entry.DB == nil {
		return nil, false, nil
	}
	return a.Entry.DB.GetA2ATask(c.Request.Context(), id, ownerID)
}

func (a *A2A) rpcResult(c *gin.Context, id json.RawMessage, result any) {
	c.JSON(http.StatusOK, gin.H{"jsonrpc": "2.0", "id": idOrNull(id), "result": result})
}

func (a *A2A) rpcError(c *gin.Context, id json.RawMessage, code int, message string) {
	c.JSON(http.StatusOK, gin.H{"jsonrpc": "2.0", "id": idOrNull(id), "error": gin.H{"code": code, "message": message}})
}

func idOrNull(raw json.RawMessage) any {
	if len(raw) == 0 {
		return nil
	}
	var v any
	if err := json.Unmarshal(raw, &v); err != nil {
		return nil
	}
	return v
}
