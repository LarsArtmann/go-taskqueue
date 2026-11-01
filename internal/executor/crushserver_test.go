package executor

import (
	"context"
	"encoding/json/jsontext"
	"encoding/json/v2"
	"net"
	"net/http"
	"os"
	"path/filepath"
	"testing"
	"time"
)

// Wire fixtures are the E7 captures (docs/planning/2026-10-30_04-43_
// crush-client-server-relations.md): GET .../sessions and .../messages
// answer bare JSON arrays; tool_call input and tool_result metadata are
// JSON STRINGS inside the part data.

const crushSessionsFixture = `[
  {"id":"986871b8-f118-461a-8175-390db4688c9a","parent_session_id":"","title":"ok",
   "message_count":4,"prompt_tokens":16764,"completion_tokens":29,
   "summary_message_id":"","cost":0,"created_at":1793497389,
   "updated_at":1793497390,"is_busy":false,"attached_clients":0}
]`

const crushMessagesFixture = `[
  {"parts":[{"type":"text","data":{"text":"please write the verdict file now"}},
            {"type":"finish","data":{"reason":"stop","time":0}}],
   "id":"40e8639c","role":"user","session_id":"986871b8","model":"","provider":"",
   "created_at":1793497389,"updated_at":1793497389},
  {"parts":[{"type":"tool_call","data":{"id":"call_1","name":"bash",
              "input":"{\"command\": \"printf %s x > verdict.json\", \"description\": \"write the verdict file\"}",
              "finished":true}},
            {"type":"finish","data":{"reason":"tool_use","time":1793497390}}],
   "id":"40fbe1aa","role":"assistant","session_id":"986871b8","model":"mock-1","provider":"mock",
   "created_at":1793497389,"updated_at":1793497390},
  {"parts":[{"type":"tool_result","data":{"tool_call_id":"call_1","name":"bash","content":"no output",
              "metadata":"{\"start_time\":1793497389917,\"end_time\":1793497390017,\"output\":\"\",\"description\":\"write the verdict file\",\"working_directory\":\"/tmp/repo\"}",
              "is_error":false}},
            {"type":"finish","data":{"reason":"stop","time":0}}],
   "id":"ab579ba9","role":"tool","session_id":"986871b8","model":"","provider":"",
   "created_at":1793497390,"updated_at":1793497390}
]`

func TestCrushWireShapesParse(t *testing.T) {
	t.Parallel()

	var sessions []crushSessionSummary
	if err := json.Unmarshal([]byte(crushSessionsFixture), &sessions); err != nil {
		t.Fatalf("sessions fixture: %v", err)
	}

	if len(sessions) != 1 || sessions[0].ID != "986871b8-f118-461a-8175-390db4688c9a" {
		t.Fatalf("sessions parse: %+v", sessions)
	}

	if sessions[0].PromptTokens != 16764 || sessions[0].CompletionTokens != 29 || sessions[0].MessageCount != 4 {
		t.Fatalf("usage members: %+v", sessions[0])
	}

	var messages []crushMessage
	if err := json.Unmarshal([]byte(crushMessagesFixture), &messages); err != nil {
		t.Fatalf("messages fixture: %v", err)
	}

	if len(messages) != 3 || len(messages[1].Parts) != 2 || messages[1].Parts[0].Type != "tool_call" {
		t.Fatalf("messages parse: %+v", messages)
	}
}

func TestToolCallFacts(t *testing.T) {
	t.Parallel()

	var messages []crushMessage
	if err := json.Unmarshal([]byte(crushMessagesFixture), &messages); err != nil {
		t.Fatalf("fixture: %v", err)
	}

	facts := toolCallFacts(messages)
	if len(facts) != 1 {
		t.Fatalf("want 1 fact, got %d: %+v", len(facts), facts)
	}

	want := ToolCallFact{
		Name:             "bash",
		Description:      "write the verdict file",
		WorkingDirectory: "/tmp/repo",
		DurationMS:       100,
		Command:          "printf %s x > verdict.json",
	}
	if facts[0] != want {
		t.Fatalf("fact mismatch:\n got %+v\nwant %+v", facts[0], want)
	}
}

func TestToolCallFactsUnpairedAndRedacted(t *testing.T) {
	t.Parallel()

	messages := []crushMessage{{
		Role: "assistant",
		Parts: []crushPart{
			{
				Type: "tool_call",
				Data: jsontext.Value(
					`{"id":"c2","name":"bash","input":"{\"command\":\"echo sk-live-0123456789abcdef\"}","finished":false}`,
				),
			},
			{Type: "tool_call", Data: jsontext.Value(`{"id":"","name":"bash","input":"{}"}`)}, // empty id: dropped
		},
	}}

	facts := toolCallFacts(messages)
	if len(facts) != 1 {
		t.Fatalf("want the one id-bearing call, got %d", len(facts))
	}

	if facts[0].IsError || facts[0].DurationMS != 0 {
		t.Fatalf("unpaired fact must stay zero-valued on result half: %+v", facts[0])
	}

	if want := RedactSecrets("echo sk-live-0123456789abcdef"); facts[0].Command != want {
		t.Fatalf("command not redacted: %q", facts[0].Command)
	}
}

func TestResolveCrushSession(t *testing.T) {
	t.Parallel()

	now := time.Now()
	summaries := []crushSessionSummary{
		{ID: "old", CreatedAt: now.Add(-time.Hour).Unix()},
		{ID: "fresh", CreatedAt: now.Add(-2 * time.Second).Unix()},
	}

	if got := resolveCrushSession(summaries, "old", now); got == nil || got.ID != "old" {
		t.Fatalf("extracted id must win: %+v", got)
	}

	if got := resolveCrushSession(summaries, "", now); got == nil || got.ID != "fresh" {
		t.Fatalf("window match must pick the newest in-window session: %+v", got)
	}

	if got := resolveCrushSession([]crushSessionSummary{
		{ID: "ancient", CreatedAt: now.Add(-time.Minute).Unix()},
	}, "", now); got != nil {
		t.Fatalf("no in-window session must resolve nil: %+v", got)
	}

	if got := resolveCrushSession(summaries, "missing", now); got != nil {
		t.Fatalf("unknown extracted id with no window fallback must stay nil (stdout value untouched): %+v", got)
	}
}

// TestEnrichCloseoutFactsAgainstLiveServer pins the full enricher against
// a real HTTP server on a real unix socket speaking the E7 wire shapes:
// register → sessions → messages, including the fallback contract (a
// failing endpoint leaves the stdout-derived fields exactly as they were).
func TestEnrichCloseoutFactsAgainstLiveServer(t *testing.T) {
	if _, err := os.Stat("/proc/self/net/unix"); err != nil {
		t.Skip("unix sockets unavailable")
	}

	dir := t.TempDir()
	sock := filepath.Join(dir, "crush.sock")

	listener, err := net.Listen("unix", sock)
	if err != nil {
		t.Skipf("unix listen: %v", err)
	}

	t.Cleanup(func() { listener.Close() })

	mux := http.NewServeMux()
	mux.HandleFunc("POST /v1/workspaces", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"id":"ws-1","path":"/repo","data_dir":"/repo/.crush","version":"v0.99.0"}`))
	})
	mux.HandleFunc("GET /v1/workspaces/{id}/sessions", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(crushSessionsFixture))
	})
	mux.HandleFunc("GET /v1/workspaces/{id}/sessions/{sid}/messages", func(w http.ResponseWriter, r *http.Request) {
		if r.PathValue("sid") != "986871b8-f118-461a-8175-390db4688c9a" {
			http.NotFound(w, r)

			return
		}

		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(crushMessagesFixture))
	})

	server := &http.Server{Handler: mux}
	go server.Serve(listener)

	t.Cleanup(func() { server.Close() })

	e := &AgentExecutor{CrushServer: "unix://" + sock}
	result := &AgentResult{}
	runStart := time.Unix(1793497389-2, 0).UTC()

	e.enrichCloseoutFacts(context.Background(), "/repo", result, runStart)

	if result.SessionID != "986871b8-f118-461a-8175-390db4688c9a" {
		t.Fatalf("session id not resolved from the window: %+v", result.sessionUsage)
	}

	if result.SessionPromptTokens != 16764 || result.SessionCompletionTokens != 29 || result.SessionMessageCount != 4 {
		t.Fatalf("usage not filled from the summary: %+v", result.sessionUsage)
	}

	if len(result.ToolCalls) != 1 || result.ToolCalls[0].Name != "bash" || result.ToolCalls[0].DurationMS != 100 {
		t.Fatalf("tool inventory: %+v", result.ToolCalls)
	}
}

// TestEnrichCloseoutFactsDegradesSilently pins the fallback contract: an
// unreachable server (the routed seam's worst case) leaves every
// stdout-derived field untouched.
func TestEnrichCloseoutFactsDegradesSilently(t *testing.T) {
	t.Parallel()

	e := &AgentExecutor{CrushServer: "unix:///nonexistent/tq-test.sock"}
	result := &AgentResult{}
	result.SessionID = "from-stdout"
	result.SessionPromptTokens = 42

	e.enrichCloseoutFacts(context.Background(), "/repo", result, time.Now())

	if result.SessionID != "from-stdout" || result.SessionPromptTokens != 42 || len(result.ToolCalls) != 0 {
		t.Fatalf("fallback contract broken: %+v", result)
	}
}
