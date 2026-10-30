package webui

import (
	"bufio"
	"context"
	"io"
	"log/slog"
	"net"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

// fakeCrushServer serves the two L2 endpoints over a real unix socket: the
// workspace list and one workspace's data:-only SSE event stream (the
// shapes captured in the client-server relations doc, M1/M2/E3).
func fakeCrushServer(t *testing.T, frames ...string) string {
	t.Helper()

	mux := http.NewServeMux()
	mux.HandleFunc("/v1/workspaces", func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"workspaces":[{"id":"ws-1","path":"/repos/demo"}]}`))
	})
	mux.HandleFunc("/v1/workspaces/ws-1/events", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/event-stream")
		fl := w.(http.Flusher)
		for _, f := range frames {
			select {
			case <-r.Context().Done():
				return
			case <-time.After(10 * time.Millisecond):
			}
			_, _ = w.Write([]byte("data: " + f + "\n\n"))
			fl.Flush()
		}
		<-r.Context().Done()
	})

	ln, err := net.Listen("unix", filepath.Join(t.TempDir(), "crush.sock"))
	if err != nil {
		t.Fatalf("unix listen: %v", err)
	}
	t.Cleanup(func() { _ = ln.Close() })

	srv := &http.Server{Handler: mux, ReadHeaderTimeout: 5 * time.Second}
	go func() { _ = srv.Serve(ln) }()
	t.Cleanup(func() { _ = srv.Close() })

	return "unix://" + ln.Addr().String()
}

func TestDistillFrame(t *testing.T) {
	t.Parallel()

	for _, tc := range []struct {
		frame    string
		wantKind string
		wantIn   string
	}{
		{
			frame:    `{"type":"message.updated","part":{"type":"tool_call","tool_call":{"name":"bash","command":"gofmt -l ."}}}`,
			wantKind: "tool",
			wantIn:   "bash gofmt -l .",
		},
		{
			frame:    `{"type":"message.updated","part":{"type":"tool_call","tool_call":{"name":"view","input":{"path":"main.go"}}}}`,
			wantKind: "tool",
			wantIn:   "view",
		},
		{
			frame:    `{"type":"message.updated","part":{"type":"tool_result"}}`,
			wantKind: "result",
			wantIn:   "tool_result",
		},
		{
			frame:    `{"type":"session.created","session":{"id":"s1"}}`,
			wantKind: "session",
			wantIn:   "session started",
		},
		{
			frame:    `{"type":"session.updated","session":{"prompt_tokens":161}}`,
			wantKind: "",
			wantIn:   "",
		},
		{
			frame:    `{"type":"message.created","message":{"content":"prose"}}`,
			wantKind: "",
			wantIn:   "",
		},
	} {
		kind, line := distillFrame(tc.frame)
		if kind != tc.wantKind {
			t.Fatalf("distillFrame(%s) kind = %q, want %q", tc.frame, kind, tc.wantKind)
		}
		if tc.wantIn != "" && !strings.Contains(line, tc.wantIn) {
			t.Fatalf("distillFrame(%s) line = %q, want containing %q", tc.frame, line, tc.wantIn)
		}
	}
}

func TestDistillFrameExcerptCapsLongLines(t *testing.T) {
	t.Parallel()

	long := strings.Repeat("x", 500)
	_, line := distillFrame(`{"type":"tool_result","output":"` + long + `"}`)
	if len(line) > 220 {
		t.Fatalf("excerpt cap failed: %d bytes", len(line))
	}
}

func TestRunAgentFeedAttachesAndStreams(t *testing.T) {
	socket := fakeCrushServer(t,
		`{"type":"session.created","session":{"id":"s1"}}`,
		`{"type":"message.updated","part":{"type":"tool_call","tool_call":{"name":"bash","command":"sleep 30"}}}`,
	)

	feed := NewAgentFeed()
	sub := feed.Subscribe()

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	go func() { _ = RunAgentFeed(ctx, socket, feed, discardLogger()) }()

	deadline := time.After(10 * time.Second)
	var got []AgentEvent

	for len(got) < 2 {
		select {
		case <-deadline:
			t.Fatalf("timed out with %d events: %v", len(got), got)
		case evt := <-sub:
			got = append(got, evt)
		}
	}

	if got[0].Kind != "session" || got[0].Workspace != "/repos/demo" {
		t.Fatalf("first event = %+v, want session for /repos/demo", got[0])
	}

	if got[1].Kind != "tool" || !strings.Contains(got[1].Line, "sleep 30") {
		t.Fatalf("second event = %+v, want tool sleep 30", got[1])
	}
}

func TestHandleAgentLiveDisabledServes503(t *testing.T) {
	t.Parallel()

	s := New(nil, Config{})
	req := httptest.NewRequest(http.MethodGet, "/agents/live", nil)
	rec := httptest.NewRecorder()
	s.handleAgentLive(rec, req)

	if rec.Code != http.StatusServiceUnavailable {
		t.Fatalf("status = %d, want 503", rec.Code)
	}
	if !strings.Contains(rec.Body.String(), "agent feed disabled") {
		t.Fatalf("body = %q, want disabled reason", rec.Body.String())
	}
}

func TestHandleAgentLiveStreamsEvents(t *testing.T) {
	// A real server: httptest.Recorder is not concurrency-safe for reads
	// racing the streaming handler's writes, and SSE needs a Flusher.
	s := New(nil, Config{CrushServer: "unix:///tmp/never-used.sock"})
	srv := httptest.NewServer(http.HandlerFunc(s.handleAgentLive))
	t.Cleanup(srv.Close)

	go func() {
		time.Sleep(50 * time.Millisecond)
		s.agentFeed.broadcast(AgentEvent{Workspace: "/repos/demo", Kind: "tool", Line: "bash go test ./..."})
	}()

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	req, _ := http.NewRequestWithContext(ctx, http.MethodGet, srv.URL+"/agents/live", nil)
	resp, err := srv.Client().Do(req)
	if err != nil {
		t.Fatalf("GET /agents/live: %v", err)
	}
	defer func() { _ = resp.Body.Close() }()

	scanner := bufio.NewScanner(resp.Body)
	for scanner.Scan() {
		line := scanner.Text()
		if strings.Contains(line, `"kind":"tool"`) && strings.Contains(line, "go test") {
			return // event streamed live; ctx cancel tears down
		}
	}
	t.Fatal("no agent event streamed before timeout")
}

func TestDecodeWorkspacesBodyBothShapes(t *testing.T) {
	t.Parallel()

	wrapped, err := decodeWorkspacesBody([]byte(`{"workspaces":[{"id":"a","path":"/p/a"},{"path":"/p/b"}]}`))
	if err != nil || len(wrapped) != 2 || wrapped[0].key() != "a" || wrapped[1].key() != "/p/b" {
		t.Fatalf("wrapped shape: %v %v", wrapped, err)
	}

	bare, err := decodeWorkspacesBody([]byte(`[{"id":"a"},{"path":"/p/b"}]`))
	if err != nil || len(bare) != 2 || bare[0].key() != "a" || bare[1].key() != "/p/b" {
		t.Fatalf("bare shape: %v %v", bare, err)
	}

	if _, err := decodeWorkspacesBody([]byte(`{nope`)); err == nil {
		t.Fatal("malformed body must error")
	}
}

func discardLogger() *slog.Logger {
	return slog.New(slog.NewTextHandler(io.Discard, nil))
}
