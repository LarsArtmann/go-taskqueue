// Package mcpserver is the MCP (Model Context Protocol) stdio server
// behind `tq mcp` (plan 2026-10-16_01-11 Phase 1, dispatch 2026-09-29):
// a hand-rolled JSON-RPC 2.0 transport — no MCP SDK dependency, pure-Go
// deps only — exposing the queue's READ surfaces as tools so agent
// sessions stop burning shell round-trips (`tq tasks | grep …`) and
// inherit nothing from the ambient environment (the server takes an
// explicit --db, the argv-quoting and ambient-TQ_DB hazard classes die
// together).
//
// Protocol surface: initialize / notifications/initialized / tools/list /
// tools/call / ping, newline-delimited JSON over stdin/stdout (the MCP
// stdio transport). Write tools (park_task) join behind the same registry
// once the owner rules them into v1.
package mcpserver

import (
	"encoding/json"
	"fmt"
)

// jsonrpcVersion is the only JSON-RPC dialect this server speaks.
const jsonrpcVersion = "2.0"

// rpcRequest is one inbound JSON-RPC request or notification. A nil or
// absent id means NOTIFICATION — processed (or ignored) with no response,
// per the JSON-RPC 2.0 spec.
type rpcRequest struct {
	JSONRPC string          `json:"jsonrpc"`
	ID      json.RawMessage `json:"id,omitempty"`
	Method  string          `json:"method"`
	Params  json.RawMessage `json:"params,omitempty"`
}

// rpcError is the JSON-RPC error envelope. Code is a JSON-RPC 2.0 code
// (the -32700..-32603 standard set plus application errors); Message is
// human-readable; Data is optional structured detail.
type rpcError struct {
	Code    int             `json:"code"`
	Message string          `json:"message"`
	Data    json.RawMessage `json:"data,omitempty"`
}

// rpcResponse is the outbound reply for a request (never a notification).
type rpcResponse struct {
	JSONRPC string          `json:"jsonrpc"`
	ID      json.RawMessage `json:"id"`
	Result  any             `json:"result,omitempty"`
	Error   *rpcError       `json:"error,omitempty"`
}

// isNotification reports whether the raw id encodes "no id" (absent,
// null, or empty) — the requests that must never get a response.
func isNotification(id json.RawMessage) bool {
	trimmed := trimJSONSpace(id)

	return len(trimmed) == 0 || string(trimmed) == "null"
}

// trimJSONSpace strips ASCII whitespace from both ends of raw JSON.
func trimJSONSpace(b []byte) []byte {
	start, end := 0, len(b)

	for start < end && (b[start] == ' ' || b[start] == '\t' || b[start] == '\n' || b[start] == '\r') {
		start++
	}

	for end > start && (b[end-1] == ' ' || b[end-1] == '\t' || b[end-1] == '\n' || b[end-1] == '\r') {
		end--
	}

	return b[start:end]
}

// Standard JSON-RPC 2.0 error codes (the spec's reserved set; application
// errors use ServerError and above).
const (
	codeParseError     = -32700
	codeInvalidRequest = -32600
	codeMethodNotFound = -32601
	codeInvalidParams  = -32602
	codeInternalError  = -32603
)

// newResult builds a success response for the given request id.
func newResult(id json.RawMessage, result any) *rpcResponse {
	return &rpcResponse{JSONRPC: jsonrpcVersion, ID: id, Result: result}
}

// newError builds an error response for the given request id. A nil id
// (an unparseable message) still gets id:null per the spec's parse-error
// shape.
func newError(id json.RawMessage, code int, format string, args ...any) *rpcResponse {
	return &rpcResponse{
		JSONRPC: jsonrpcVersion,
		ID:      id,
		Error:   &rpcError{Code: code, Message: fmt.Sprintf(format, args...)},
	}
}
