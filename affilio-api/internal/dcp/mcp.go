package dcp

import (
	"encoding/json"
	"net/url"
	"strings"
)

// Minimal MCP server core: JSON-RPC 2.0 messages over a line-delimited stdio
// transport (RunStdio). Only the methods an MCP client needs to drive tools
// are implemented.

const mcpProtocolVersion = "2025-06-18"

type rpcRequest struct {
	JSONRPC string          `json:"jsonrpc"`
	ID      json.RawMessage `json:"id"`
	Method  string          `json:"method"`
	Params  json.RawMessage `json:"params"`
}

type rpcError struct {
	Code    int    `json:"code"`
	Message string `json:"message"`
}

type rpcResponse struct {
	JSONRPC string          `json:"jsonrpc"`
	ID      json.RawMessage `json:"id"`
	Result  interface{}     `json:"result,omitempty"`
	Error   *rpcError       `json:"error,omitempty"`
}

type toolCallParams struct {
	Name      string                 `json:"name"`
	Arguments map[string]interface{} `json:"arguments"`
}

// HandleMessage processes one JSON-RPC message and returns the encoded
// response, or nil for notifications and unparseable input.
func HandleMessage(client *Client, raw []byte) []byte {
	var req rpcRequest
	if err := json.Unmarshal(raw, &req); err != nil {
		return nil
	}
	if len(req.ID) == 0 || string(req.ID) == "null" {
		return nil // notification: nothing to answer
	}
	resp := rpcResponse{JSONRPC: "2.0", ID: req.ID}
	switch req.Method {
	case "initialize":
		resp.Result = map[string]interface{}{
			"protocolVersion": mcpProtocolVersion,
			"capabilities":    map[string]interface{}{"tools": map[string]interface{}{}, "resources": map[string]interface{}{}},
			"serverInfo": map[string]interface{}{
				"name":    ServerName,
				"title":   ServerTitle,
				"version": "0.2.0",
			},
			"instructions": "Call get_started first — it returns the ordered recipe and the exact next_step to call; repeat after each step. Use the owner's existing agent. Makers prepare products and offers for review. Publishers find_opportunities, prepare_placement, wait for separate owner approval, commit_placement, publish approved copy with disclosure through their own tool, then record_placement_receipt. Poll list_agent_tasks to resume. undo_placement calls the API in the same request, with no delay. restore_placement does not call the API; it returns a sign in again link to the dashboard. Approval and payout confirmation are not agent tools. A scoped publisher key cannot join through legacy endpoints. Do not claim a sale, publication or payout is independently verified without evidence.",
		}
	case "ping":
		resp.Result = map[string]interface{}{}
	case "tools/list":
		all := AllTools()
		tools := make([]map[string]interface{}, 0, len(all))
		for _, tool := range all {
			tools = append(tools, map[string]interface{}{
				"name":        tool.Name,
				"title":       tool.Title(),
				"description": tool.Description,
				"inputSchema": tool.InputSchema,
			})
		}
		resp.Result = map[string]interface{}{"tools": tools}
	case "tools/call":
		var params toolCallParams
		if err := json.Unmarshal(req.Params, &params); err != nil {
			resp.Error = &rpcError{Code: -32602, Message: "invalid params"}
			break
		}
		result, err := CallTool(client, params.Name, params.Arguments)
		content, isError := toolResultContent(result, err)
		resp.Result = map[string]interface{}{
			"content": []map[string]interface{}{{"type": "text", "text": content}},
			"isError": isError,
		}
	case "resources/list":
		resp.Result = map[string]interface{}{"resources": []map[string]interface{}{
			{"uri": "orp://inbox", "name": "ORP agent inbox", "mimeType": "application/json", "description": "Messages addressed to this party: offers, accepts, receipts, deal states."},
		}}
	case "resources/templates/list":
		resp.Result = map[string]interface{}{"resourceTemplates": []map[string]interface{}{
			{"uriTemplate": "orp://inbox", "name": "ORP agent inbox", "mimeType": "application/json"},
			{"uriTemplate": "orp://deals/{deal_id}", "name": "ORP deal", "mimeType": "application/json", "description": "One deal's parties, terms and state by id."},
		}}
	case "resources/read":
		var params struct {
			URI string `json:"uri"`
		}
		if err := json.Unmarshal(req.Params, &params); err != nil {
			resp.Error = &rpcError{Code: -32602, Message: "invalid params"}
			break
		}
		path, ok := resourcePath(params.URI)
		if !ok {
			resp.Error = &rpcError{Code: -32602, Message: "unknown resource " + params.URI}
			break
		}
		out, err := client.Do("GET", path, nil)
		if err != nil {
			resp.Error = &rpcError{Code: -32000, Message: err.Error()}
			break
		}
		raw, _ := json.Marshal(out)
		resp.Result = map[string]interface{}{"contents": []map[string]interface{}{
			{"uri": params.URI, "mimeType": "application/json", "text": string(raw)},
		}}
	default:
		resp.Error = &rpcError{Code: -32601, Message: "method not found"}
	}
	encoded, err := json.Marshal(resp)
	if err != nil {
		return nil
	}
	return encoded
}

// resourcePath maps an orp:// resource URI to its loopback API path.
func resourcePath(uri string) (string, bool) {
	if uri == "orp://inbox" {
		return "/orp/inbox", true
	}
	if id, ok := strings.CutPrefix(uri, "orp://deals/"); ok && id != "" {
		return "/orp/deals/" + url.PathEscape(id), true
	}
	return "", false
}

func toolResultContent(result map[string]interface{}, err error) (string, bool) {
	if err != nil {
		return err.Error(), true
	}
	encoded, marshalErr := json.MarshalIndent(result, "", "  ")
	if marshalErr != nil {
		return marshalErr.Error(), true
	}
	return string(encoded), false
}
