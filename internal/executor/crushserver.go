package executor

import (
	"bytes"
	"context"
	"crypto/rand"
	"encoding/hex"
	"encoding/json/jsontext"
	"encoding/json/v2"
	"fmt"
	"net"
	"net/http"
	"strings"
	"time"
)

// Structured closeout facts from the crush server's messages endpoint
// (TODO row 1446; shapes pinned live in docs/planning/2026-10-30_04-43_
// crush-client-server-relations.md §E7): routed runs (L1 seam,
// e.CrushServer) get their session identity, usage block, and per-tool-
// call inventory from the API instead of scraping `crush run` stdout.
// Best effort by design — every failure leaves the stdout-derived fields
// untouched (the fallback the row mandates).
//
// Read seam: sessions persist per-project in the workspace's data_dir, so
// ANY workspace registered over the run's repo path lists the run's
// session — the executor registers its own (POST /v1/workspaces with a
// random client_id, the E7 "doorway") instead of needing the run's own
// workspace id, which v0.99.0 never exposes for non-interactive runs.

// crushServerHTTPTimeout bounds every dial against the pool's server; a
// slow fetch must never outlive the task's context anyway.
const crushServerHTTPTimeout = 10 * time.Second

// ToolCallFact is one structured tool invocation from the session's
// message parts: the closeout report's raw material. The bash command
// passes the secrets redactor (tool input is as token-bearing as output
// tails).
type ToolCallFact struct {
	Name             string `json:"name"`
	Description      string `json:"description,omitempty"`
	WorkingDirectory string `json:"working_directory,omitempty"`
	DurationMS       int64  `json:"duration_ms,omitempty"`
	Command          string `json:"command,omitempty"`
	IsError          bool   `json:"is_error,omitempty"`
}

// crushWorkspaceRequest is the POST /v1/workspaces body (E7): a client_id
// is mandatory — without it the server answers 400 invalid client_id.
type crushWorkspaceRequest struct {
	ClientID string `json:"client_id"`
	Path     string `json:"path"`
}

// crushSessionSummary mirrors GET /v1/workspaces/{id}/sessions entries
// (bare JSON array, newest first, timestamps in unix SECONDS).
type crushSessionSummary struct {
	ID               string  `json:"id"`
	Title            string  `json:"title"`
	MessageCount     int     `json:"message_count"`
	PromptTokens     int64   `json:"prompt_tokens"`
	CompletionTokens int64   `json:"completion_tokens"`
	CostUSD          float64 `json:"cost"`
	CreatedAt        int64   `json:"created_at"`
	IsBusy           bool    `json:"is_busy"`
	AttachedClients  int     `json:"attached_clients"`
}

// crushMessage mirrors GET .../sessions/{sid}/messages entries: the parts
// column's wire shape, where tool_call/tool_result carry their payload as
// JSON STRINGS inside data.
type crushMessage struct {
	Role      string       `json:"role"`
	Parts     []crushPart `json:"parts"`
	CreatedAt int64        `json:"created_at"`
}

type crushPart struct {
	Type string         `json:"type"`
	Data jsontext.Value `json:"data"`
}

type crushToolCallData struct {
	ID       string `json:"id"`
	Name     string `json:"name"`
	Input    string `json:"input"`
	Finished bool   `json:"finished"`
}

type crushToolResultData struct {
	ToolCallID string `json:"tool_call_id"`
	Name       string `json:"name"`
	Content    string `json:"content"`
	Metadata   string `json:"metadata"`
	IsError    bool   `json:"is_error"`
}

type crushToolResultMetadata struct {
	StartTime        int64  `json:"start_time"`
	EndTime          int64  `json:"end_time"`
	Description      string `json:"description"`
	WorkingDirectory string `json:"working_directory"`
}

type crushToolCallInput struct {
	Command     string `json:"command"`
	Description string `json:"description"`
}

// crushServerClient speaks HTTP over the pool server's unix socket.
type crushServerClient struct {
	http *http.Client
}

func newCrushServerClient(socket string) *crushServerClient {
	socket = strings.TrimPrefix(socket, "unix://")
	return &crushServerClient{
		http: &http.Client{
			Timeout: crushServerHTTPTimeout,
			Transport: &http.Transport{
				DialContext: func(ctx context.Context, _, _ string) (net.Conn, error) {
					d := net.Dialer{}
					return d.DialContext(ctx, "unix", socket)
				},
			},
		},
	}
}

// registerWorkspace opens the read doorway: a workspace over repoDir so
// the session list becomes readable. The id is a fresh uuid4-shaped hex —
// the server only requires non-empty.
func (c *crushServerClient) registerWorkspace(ctx context.Context, repoDir string) (string, error) {
	var body struct {
		ID string `json:"id"`
	}
	if err := c.postJSON(ctx, "/v1/workspaces", crushWorkspaceRequest{
		ClientID: randomClientID(),
		Path:     repoDir,
	}, &body); err != nil {
		return "", err
	}
	if body.ID == "" {
		return "", fmt.Errorf("crush server returned no workspace id")
	}
	return body.ID, nil
}

func (c *crushServerClient) sessions(ctx context.Context, workspaceID string) ([]crushSessionSummary, error) {
	var out []crushSessionSummary
	err := c.getJSON(ctx, "/v1/workspaces/"+workspaceID+"/sessions", &out)
	return out, err
}

func (c *crushServerClient) messages(ctx context.Context, workspaceID, sessionID string) ([]crushMessage, error) {
	var out []crushMessage
	err := c.getJSON(ctx, "/v1/workspaces/"+workspaceID+"/sessions/"+sessionID+"/messages", &out)
	return out, err
}

func (c *crushServerClient) getJSON(ctx context.Context, path string, dst any) error {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, "http://crush"+path, nil)
	if err != nil {
		return err
	}
	resp, err := c.http.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("crush server %s: %s", path, resp.Status)
	}
	return json.UnmarshalRead(resp.Body, dst)
}

func (c *crushServerClient) postJSON(ctx context.Context, path string, body, dst any) error {
	payload, err := json.Marshal(body)
	if err != nil {
		return err
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, "http://crush"+path, bytes.NewReader(payload))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/json")
	resp, err := c.http.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("crush server %s: %s", path, resp.Status)
	}
	return json.UnmarshalRead(resp.Body, dst)
}

// enrichCloseoutFacts fills the structured session facts for a routed
// run: the session id when stdout scraping missed it (matched by the
// run's start-time window — the summaries are newest-first, unix-second
// granularity), the usage block from the session summary, and the
// per-tool-call inventory from the message parts. Routed runs only
// (e.CrushServer set); every failure degrades silently to whatever the
// stdout derivation already produced.
func (e *AgentExecutor) enrichCloseoutFacts(ctx context.Context, repoDir string, result *AgentResult, runStart time.Time) {
	if e.CrushServer == "" {
		return
	}

	client := newCrushServerClient(e.CrushServer)

	workspaceID, err := client.registerWorkspace(ctx, repoDir)
	if err != nil {
		return // stdout fallback stays the only channel
	}

	summaries, err := client.sessions(ctx, workspaceID)
	if err != nil || len(summaries) == 0 {
		return
	}

	session := resolveCrushSession(summaries, result.SessionID, runStart)
	if session == nil {
		return
	}

	if result.SessionID == "" {
		result.SessionID = session.ID
	}
	if result.SessionPromptTokens == 0 {
		result.SessionPromptTokens = session.PromptTokens
	}
	if result.SessionCompletionTokens == 0 {
		result.SessionCompletionTokens = session.CompletionTokens
	}
	if result.SessionCostUSD == 0 {
		result.SessionCostUSD = session.CostUSD
	}
	if result.SessionMessageCount == 0 {
		result.SessionMessageCount = session.MessageCount
	}

	messages, err := client.messages(ctx, workspaceID, session.ID)
	if err != nil {
		return // usage block landed; the tool inventory is optional
	}
	if facts := toolCallFacts(messages); len(facts) > 0 {
		result.ToolCalls = facts
	}
}

// resolveCrushSession picks the run's session: the stdout-extracted id
// when it matches, else the newest summary whose created_at falls in the
// run's start window (± the unix-second granularity and the crush-side
// session-create lag).
func resolveCrushSession(summaries []crushSessionSummary, extractedID string, runStart time.Time) *crushSessionSummary {
	if extractedID != "" {
		for i := range summaries {
			if summaries[i].ID == extractedID {
				return &summaries[i]
			}
		}
	}
	windowStart := runStart.Add(-15 * time.Second).Unix()
	windowEnd := time.Now().Add(5 * time.Second).Unix()
	for i := range summaries {
		if summaries[i].CreatedAt >= windowStart && summaries[i].CreatedAt <= windowEnd {
			return &summaries[i]
		}
	}
	return nil
}

// toolCallFacts folds the message parts into the per-tool-call inventory:
// tool_call carries the input (command + description as a JSON string),
// its tool_result partner carries duration (ms), working directory and
// the error flag. Unpaired calls (a run cut mid-turn) still surface with
// everything the call half knows.
func toolCallFacts(messages []crushMessage) []ToolCallFact {
	type pair struct {
		fact ToolCallFact
	}
	byCallID := map[string]*pair{}
	var order []string

	for _, msg := range messages {
		for _, part := range msg.Parts {
			switch part.Type {
			case "tool_call":
				var data crushToolCallData
				if json.Unmarshal(part.Data, &data) != nil || data.ID == "" {
					continue
				}
				fact := ToolCallFact{Name: data.Name}
				if input := decodeCrushToolInput(data.Input); input.Command != "" || input.Description != "" {
					fact.Command = RedactSecrets(input.Command)
					fact.Description = input.Description
				}
				byCallID[data.ID] = &pair{fact: fact}
				order = append(order, data.ID)
			case "tool_result":
				var data crushToolResultData
				if json.Unmarshal(part.Data, &data) != nil {
					continue
				}
				p, ok := byCallID[data.ToolCallID]
				if !ok {
					continue
				}
				p.fact.IsError = data.IsError
				var meta crushToolResultMetadata
				if json.Unmarshal([]byte(data.Metadata), &meta) == nil {
					p.fact.DurationMS = meta.EndTime - meta.StartTime
					p.fact.WorkingDirectory = meta.WorkingDirectory
				}
			}
		}
	}

	facts := make([]ToolCallFact, 0, len(order))
	for _, id := range order {
		facts = append(facts, byCallID[id].fact)
	}
	return facts
}

// decodeCrushToolInput parses the tool_call input JSON string ({"command":
// ..., "description": ...}); a non-JSON input leaves the zero value.
func decodeCrushToolInput(input string) crushToolCallInput {
	var parsed crushToolCallInput
	_ = json.Unmarshal([]byte(input), &parsed)
	return parsed
}

// randomClientID mints a uuid4-shaped id (the server's only requirement
// is a non-empty client_id it has not seen as attached).
func randomClientID() string {
	var b [16]byte
	if _, err := rand.Read(b[:]); err != nil {
		return "tq-executor"
	}
	b[6] = (b[6] & 0x0f) | 0x40
	b[8] = (b[8] & 0x3f) | 0x80
	s := hex.EncodeToString(b[:])
	return s[0:8] + "-" + s[8:12] + "-" + s[12:16] + "-" + s[16:20] + "-" + s[20:32]
}
