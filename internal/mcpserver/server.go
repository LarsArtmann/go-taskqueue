package mcpserver

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"sort"
	"strings"

	"github.com/larsartmann/go-taskqueue/internal/journal"
	"github.com/larsartmann/go-taskqueue/internal/queue"
	"github.com/larsartmann/go-taskqueue/internal/task"
)

// Store is the minimal read seam the tools ride — the subset of
// queue.Store the MCP surface needs, so the server module depends on
// reads alone and tests stub the seam without a database. *sqlite.Store
// satisfies it.
type Store interface {
	Get(ctx context.Context, id task.ID) (task.Task, error)
	List(ctx context.Context, f queue.Filter) ([]task.Task, error)
	FactsForTask(ctx context.Context, id string, limit int) ([]journal.Fact, error)
	StatusCounts(ctx context.Context) (map[task.Status]int, error)
	ProjectCounts(ctx context.Context) (map[string]map[task.Status]int, error)
}

// Tool is one registered MCP tool: a name, a description the model reads
// when choosing tools, a JSON Schema for the arguments, and the handler
// returning the tool's text result.
type Tool struct {
	Name        string
	Description string
	InputSchema any
	Handler     func(ctx context.Context, args json.RawMessage) (string, error)
}

// Server serves the MCP tool surface over newline-delimited JSON-RPC 2.0.
type Server struct {
	store Store
	tools []Tool
	// serverVersion rides the initialize result (serverInfo) so the
	// client can log which tq it talked to.
	serverVersion string
}

// New builds a server over the store with the default tool registry.
func New(store Store, serverVersion string) *Server {
	s := &Server{
		store:         store,
		serverVersion: serverVersion,
	}
	s.tools = s.defaultTools()

	return s
}

// SupportedProtocolVersions lists the MCP protocol revisions this server
// speaks (the go-sdk v1.8.0 revision set crush v0.102.0 negotiates with,
// verified 2026-11-23): initialize echoes the client's requested revision
// when it appears here, else answers with LatestProtocolVersion.
var SupportedProtocolVersions = []string{
	"2025-11-25",
	"2025-06-18",
	"2025-03-26",
}

// LatestProtocolVersion is the revision answered when the client asks
// for one this server does not know.
const LatestProtocolVersion = "2025-06-18"

// Serve runs the stdio loop: one JSON-RPC message per line in, one
// response line per request out, until r yields EOF or ctx is done.
// A read error ends the session (the client hung up); individual
// malformed lines produce parse-error responses, never a session death.
func (s *Server) Serve(ctx context.Context, r io.Reader, w io.Writer) error {
	out := bufio.NewWriter(w)

	scanner := bufio.NewScanner(r)
	scanner.Buffer(make([]byte, 0, 64*1024), 16*1024*1024)

	enc := json.NewEncoder(out)

	for scanner.Scan() {
		select {
		case <-ctx.Done():
			return ctx.Err()
		default:
		}

		line := scanner.Bytes()
		if len(trimJSONSpace(line)) == 0 {
			continue
		}

		if resp := s.handleLine(ctx, line); resp != nil {
			if err := enc.Encode(resp); err != nil {
				return fmt.Errorf("mcp: write response: %w", err)
			}

			if err := out.Flush(); err != nil {
				return fmt.Errorf("mcp: flush response: %w", err)
			}
		}

		if err := ctx.Err(); err != nil {
			return err
		}
	}

	return scanner.Err()
}

// handleLine decodes and dispatches one inbound message, returning the
// response to write (nil for notifications and acknowledged-but-silent
// messages).
func (s *Server) handleLine(ctx context.Context, line []byte) *rpcResponse {
	var req rpcRequest
	if err := json.Unmarshal(line, &req); err != nil {
		return newError(json.RawMessage("null"), codeParseError, "parse error: %v", err)
	}

	if req.JSONRPC != jsonrpcVersion {
		if isNotification(req.ID) {
			return nil
		}

		return newError(req.ID, codeInvalidRequest, "jsonrpc member must be %q", jsonrpcVersion)
	}

	if isNotification(req.ID) {
		// Notifications get no response ever — including unknown ones
		// (notifications/initialized is the handshake's silent half).
		return nil
	}

	switch {
	case req.Method == "initialize":
		return s.handleInitialize(req)
	case req.Method == "ping":
		return newResult(req.ID, struct{}{})
	case req.Method == "server/discover":
		// crush's MCP client probes this extension method BEFORE
		// initialize (observed live on crush v0.102.0, F21). We implement
		// no extensions, so the probe answers empty — a -32601 would also
		// be spec-legal, but the empty result keeps extension-aware
		// clients from treating the silence as a broken server.
		return newResult(req.ID, struct{}{})
	case req.Method == "tools/list":
		return newResult(req.ID, toolsListResult{Tools: s.toolSummaries()})
	case req.Method == "tools/call":
		return s.handleToolsCall(ctx, req)
	default:
		return newError(req.ID, codeMethodNotFound, "method not found: %s", req.Method)
	}
}

// initializeResult is the MCP initialize reply: the negotiated protocol
// revision, the server's capabilities (tools only — no resources, no
// prompts in v1), and identity for the client's logs.
type initializeResult struct {
	ProtocolVersion string             `json:"protocolVersion"`
	Capabilities    serverCapabilities `json:"capabilities"`
	ServerInfo      serverInfo         `json:"serverInfo"`
}

type serverCapabilities struct {
	Tools struct {
		ListChanged bool `json:"listChanged"`
	} `json:"tools"`
}

type serverInfo struct {
	Name    string `json:"name"`
	Version string `json:"version"`
}

type toolsListResult struct {
	Tools []toolSummary `json:"tools"`
}

type toolSummary struct {
	Name        string `json:"name"`
	Description string `json:"description"`
	InputSchema any    `json:"inputSchema"`
}

// handleInitialize negotiates the protocol revision: echo the client's
// requested revision when supported, else answer LatestProtocolVersion
// (the client disconnects if that is unacceptable to it — the spec's
// incompatibility signal).
func (s *Server) handleInitialize(req rpcRequest) *rpcResponse {
	var params struct {
		ProtocolVersion string `json:"protocolVersion"`
	}
	_ = json.Unmarshal(req.Params, &params)

	negotiated := LatestProtocolVersion
	if params.ProtocolVersion != "" && containsString(SupportedProtocolVersions, params.ProtocolVersion) {
		negotiated = params.ProtocolVersion
	}

	result := initializeResult{
		ProtocolVersion: negotiated,
		ServerInfo:      serverInfo{Name: "tq-mcp", Version: s.serverVersion},
	}
	result.Capabilities.Tools.ListChanged = false

	return newResult(req.ID, result)
}

// toolsCallParams is the tools/call request payload.
type toolsCallParams struct {
	Name      string          `json:"name"`
	Arguments json.RawMessage `json:"arguments,omitempty"`
}

// toolsCallResult is the MCP tool-result envelope: content blocks plus
// the isError flag (a tool-level failure the model can read and recover
// from, distinct from a protocol error).
type toolsCallResult struct {
	Content []contentBlock `json:"content"`
	IsError bool           `json:"isError,omitempty"`
}

type contentBlock struct {
	Type string `json:"type"`
	Text string `json:"text"`
}

// handleToolsCall dispatches a tool call; a missing tool or bad arguments
// is a PROTOCOL error, a handler failure is a tool-result error (the MCP
// distinction — the model sees the latter in-band and can retry or
// rephrase).
func (s *Server) handleToolsCall(ctx context.Context, req rpcRequest) *rpcResponse {
	var params toolsCallParams
	if err := json.Unmarshal(req.Params, &params); err != nil {
		return newError(req.ID, codeInvalidParams, "invalid tools/call params: %v", err)
	}

	tool, ok := s.toolByName(params.Name)
	if !ok {
		return newError(req.ID, codeInvalidParams, "unknown tool: %s", params.Name)
	}

	text, err := tool.Handler(ctx, params.Arguments)
	if err != nil {
		return newResult(req.ID, toolsCallResult{
			Content: []contentBlock{{Type: "text", Text: err.Error()}},
			IsError: true,
		})
	}

	return newResult(req.ID, toolsCallResult{
		Content: []contentBlock{{Type: "text", Text: text}},
	})
}

func (s *Server) toolByName(name string) (Tool, bool) {
	for _, tool := range s.tools {
		if tool.Name == name {
			return tool, true
		}
	}

	return Tool{}, false
}

// toolSummaries renders the registry in stable name order — tools/list
// output must be deterministic for golden tests and client caches.
func (s *Server) toolSummaries() []toolSummary {
	sorted := make([]Tool, len(s.tools))
	copy(sorted, s.tools)
	sort.Slice(sorted, func(i, j int) bool { return sorted[i].Name < sorted[j].Name })

	out := make([]toolSummary, 0, len(sorted))
	for _, tool := range sorted {
		out = append(out, toolSummary{
			Name:        tool.Name,
			Description: tool.Description,
			InputSchema: tool.InputSchema,
		})
	}

	return out
}

func containsString(haystack []string, needle string) bool {
	for _, straw := range haystack {
		if strings.EqualFold(straw, needle) {
			return true
		}
	}

	return false
}
