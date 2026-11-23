package mcpserver

import (
	"context"
	"encoding/json"
	"strings"
	"testing"
)

// conversation runs one Serve session over the input lines and returns
// the response lines — the pty-less pipe harness (M6 F29): full
// JSON-RPC conversations without a process.
func conversation(t *testing.T, store Store, lines ...string) []string {
	t.Helper()

	srv := New(store, "test")
	var out strings.Builder

	if err := srv.Serve(context.Background(), strings.NewReader(strings.Join(lines, "\n")+"\n"), &out); err != nil {
		t.Fatalf("serve: %v", err)
	}

	var responses []string

	for _, line := range strings.Split(strings.TrimSpace(out.String()), "\n") {
		if line != "" {
			responses = append(responses, line)
		}
	}

	return responses
}

// decodeInto unmarshals a response line's result (or error) into target.
func decodeInto(t *testing.T, line string, target any) {
	t.Helper()

	var resp struct {
		Result json.RawMessage `json:"result"`
		Error  *rpcError       `json:"error"`
	}
	if err := json.Unmarshal([]byte(line), &resp); err != nil {
		t.Fatalf("decode response %s: %v", line, err)
	}

	if resp.Error != nil {
		t.Fatalf("unexpected error response: %+v", resp.Error)
	}

	if err := json.Unmarshal(resp.Result, target); err != nil {
		t.Fatalf("decode result %s: %v", string(resp.Result), err)
	}
}

// TestServeConversation pins the full happy-path handshake: initialize
// echoes the client's protocol revision, the initialized notification is
// answered with silence, tools/list advertises the four read tools in
// name order, a tools/call round-trip returns the store's data, and ping
// closes cleanly.
func TestServeConversation(t *testing.T) {
	store := &stubStore{
		tasks:        []stubTask{{id: "t1", project: "demo", typ: "agent", status: "pending"}},
		statusCounts: map[string]int{"pending": 1},
	}

	responses := conversation(t, store,
		`{"jsonrpc":"2.0","id":1,"method":"initialize","params":{"protocolVersion":"2025-11-25"}}`,
		`{"jsonrpc":"2.0","method":"notifications/initialized"}`,
		`{"jsonrpc":"2.0","id":2,"method":"tools/list"}`,
		`{"jsonrpc":"2.0","id":3,"method":"tools/call","params":{"name":"task_list","arguments":{"project":"demo"}}}`,
		`{"jsonrpc":"2.0","id":4,"method":"ping"}`,
	)

	if len(responses) != 4 {
		t.Fatalf("want 4 responses (notification silent), got %d: %v", len(responses), responses)
	}

	var init initializeResult
	decodeInto(t, responses[0], &init)
	if init.ProtocolVersion != "2025-11-25" {
		t.Fatalf("initialize must echo the client revision, got %q", init.ProtocolVersion)
	}

	if init.ServerInfo.Name != "tq-mcp" {
		t.Fatalf("serverInfo name = %q", init.ServerInfo.Name)
	}

	var list toolsListResult
	decodeInto(t, responses[1], &list)

	names := make([]string, 0, len(list.Tools))
	for _, tool := range list.Tools {
		names = append(names, tool.Name)
	}

	want := []string{"dlq_list", "queue_stats", "task_list", "task_show"}
	if strings.Join(names, ",") != strings.Join(want, ",") {
		t.Fatalf("tools/list order = %v, want %v", names, want)
	}

	var call toolsCallResult
	decodeInto(t, responses[2], &call)
	if call.IsError {
		t.Fatalf("task_list call errored: %+v", call)
	}

	if !strings.Contains(call.Content[0].Text, `"id":"t1"`) {
		t.Fatalf("task_list payload missing t1: %s", call.Content[0].Text)
	}

	decodeInto(t, responses[3], &struct{}{})
}

// TestInitializeUnknownRevision pins the fallback: a revision this
// server does not know is answered with LatestProtocolVersion (the
// spec's incompatibility signal — the client disconnects if that is
// unacceptable).
func TestInitializeUnknownRevision(t *testing.T) {
	responses := conversation(t, &stubStore{},
		`{"jsonrpc":"2.0","id":1,"method":"initialize","params":{"protocolVersion":"1999-01-01"}}`,
	)

	var init initializeResult
	decodeInto(t, responses[0], &init)
	if init.ProtocolVersion != LatestProtocolVersion {
		t.Fatalf("unknown revision answered with %q, want %q", init.ProtocolVersion, LatestProtocolVersion)
	}
}

// TestMalformedLineIsParseError pins the parse-error shape: an
// unparseable line gets a response with id null and code -32700, and the
// session lives on.
func TestMalformedLineIsParseError(t *testing.T) {
	responses := conversation(t, &stubStore{},
		`{this is not json`,
		`{"jsonrpc":"2.0","id":7,"method":"ping"}`,
	)

	if len(responses) != 2 {
		t.Fatalf("session must survive a parse error, got %d responses: %v", len(responses), responses)
	}

	var resp struct {
		ID    json.RawMessage `json:"id"`
		Error *rpcError       `json:"error"`
	}
	if err := json.Unmarshal([]byte(responses[0]), &resp); err != nil {
		t.Fatal(err)
	}

	if string(resp.ID) != "null" || resp.Error == nil || resp.Error.Code != codeParseError {
		t.Fatalf("parse-error shape wrong: %s", responses[0])
	}
}

// TestUnknownMethodIsMinus32601 pins method dispatch: an unknown request
// method gets -32601; an unknown NOTIFICATION stays silent.
func TestUnknownMethodIsMinus32601(t *testing.T) {
	responses := conversation(t, &stubStore{},
		`{"jsonrpc":"2.0","id":1,"method":"resources/list"}`,
		`{"jsonrpc":"2.0","method":"notifications/made/up"}`,
	)

	if len(responses) != 1 {
		t.Fatalf("unknown notification must stay silent, got %d responses", len(responses))
	}

	var resp struct {
		Error *rpcError `json:"error"`
	}
	if err := json.Unmarshal([]byte(responses[0]), &resp); err != nil {
		t.Fatal(err)
	}

	if resp.Error == nil || resp.Error.Code != codeMethodNotFound {
		t.Fatalf("unknown method must be -32601: %s", responses[0])
	}
}

// TestServerDiscoverBeforeInitialize pins the crush extension probe:
// crush's MCP client sends server/discover BEFORE initialize (observed
// live on crush v0.102.0, F21) and the server must answer it (empty
// result — we implement no extensions) rather than -32601, so the
// handshake that follows stays alive.
func TestServerDiscoverBeforeInitialize(t *testing.T) {
	responses := conversation(t, &stubStore{},
		`{"jsonrpc":"2.0","id":1,"method":"server/discover"}`,
		`{"jsonrpc":"2.0","id":2,"method":"initialize","params":{"protocolVersion":"2025-06-18"}}`,
	)

	if len(responses) != 2 {
		t.Fatalf("want 2 responses (discover + initialize), got %d: %v", len(responses), responses)
	}

	if strings.Contains(responses[0], `"error"`) {
		t.Fatalf("server/discover must not error: %s", responses[0])
	}

	if !strings.Contains(responses[0], `"result":{}`) {
		t.Fatalf("server/discover must answer an empty result: %s", responses[0])
	}

	var init initializeResult
	decodeInto(t, responses[1], &init)
	if init.ProtocolVersion != "2025-06-18" {
		t.Fatalf("initialize after discover must still negotiate: %+v", init)
	}
}

// TestToolsCallUnknownTool pins the invalid-params class: calling a tool
// that is not registered is a protocol error (the model asked for
// something the registry never advertised).
func TestToolsCallUnknownTool(t *testing.T) {
	responses := conversation(t, &stubStore{},
		`{"jsonrpc":"2.0","id":1,"method":"tools/call","params":{"name":"nope"}}`,
	)

	var resp struct {
		Error *rpcError `json:"error"`
	}
	if err := json.Unmarshal([]byte(responses[0]), &resp); err != nil {
		t.Fatal(err)
	}

	if resp.Error == nil || resp.Error.Code != codeInvalidParams {
		t.Fatalf("unknown tool must be -32602: %s", responses[0])
	}
}

// TestToolHandlerErrorIsInBand pins the MCP error distinction: a handler
// failure surfaces as a tools-call result with isError=true (the model
// reads it and recovers), never as a JSON-RPC error.
func TestToolHandlerErrorIsInBand(t *testing.T) {
	responses := conversation(t, &stubStore{listErr: errStub},
		`{"jsonrpc":"2.0","id":1,"method":"tools/call","params":{"name":"task_list"}}`,
	)

	var resp struct {
		Result toolsCallResult `json:"result"`
		Error  *rpcError       `json:"error"`
	}
	if err := json.Unmarshal([]byte(responses[0]), &resp); err != nil {
		t.Fatal(err)
	}

	if resp.Error != nil {
		t.Fatalf("handler failure leaked as protocol error: %+v", resp.Error)
	}

	if !resp.Result.IsError {
		t.Fatalf("handler failure must set isError: %s", responses[0])
	}
}

// TestBlankLinesAreSkipped pins framing tolerance: empty lines between
// messages produce no responses and no errors.
func TestBlankLinesAreSkipped(t *testing.T) {
	responses := conversation(t, &stubStore{},
		``,
		`   `,
		`{"jsonrpc":"2.0","id":1,"method":"ping"}`,
		``,
	)

	if len(responses) != 1 {
		t.Fatalf("blank lines must be skipped, got %d responses", len(responses))
	}
}
