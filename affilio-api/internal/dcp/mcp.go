package dcp

import (
	"encoding/json"
)

// Minimal MCP server core: JSON-RPC 2.0 messages over a line-delimited stdio
// transport (the loop lives in cmd/distributoor-mcp). Only the methods an MCP
// client needs to drive tools are implemented.

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
			"capabilities":    map[string]interface{}{"tools": map[string]interface{}{}},
			"serverInfo":      map[string]interface{}{"name": "distributoor", "version": "0.1.0"},
			"instructions":    "Distributoor: the agent-native distribution network. Register a maker, register the product, run the readiness assessment and fix failing checks via their remediation fields, create and submit an offer, then track earnings.",
		}
	case "ping":
		resp.Result = map[string]interface{}{}
	case "tools/list":
		all := AllTools()
		tools := make([]map[string]interface{}, 0, len(all))
		for _, tool := range all {
			tools = append(tools, map[string]interface{}{
				"name":        tool.Name,
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
	default:
		resp.Error = &rpcError{Code: -32601, Message: "method not found"}
	}
	encoded, err := json.Marshal(resp)
	if err != nil {
		return nil
	}
	return encoded
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
