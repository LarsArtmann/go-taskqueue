package webui

import (
	"bufio"
	"context"
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
	s := New(nil, Config{CrushServer: "unix:///tmp/never-used.sock"})
	sub := s.agentFeed.Subscribe()

	go func() {
		time.Sleep(50 * time.Millisecond)
		s.agentFeed.broadcast(AgentEvent{Workspace: "/repos/demo", Kind: "tool", Line: "bash go test ./..."})
	}()

	req := httptest.NewRequest(http.MethodGet, "/agents/live", nil)
	rec := httptest.NewRecorder()

	done := make(chan struct{})
	go func() {
		defer close(done)
		s.handleAgentLive(rec, req)
	}()

	deadline := time.After(5 * time.Second)
	for {
		select {
		case <-deadline:
			t.Fatal("no agent event streamed before timeout")
		case <-time.After(20 * time.Millisecond):
			body := rec.Body.String()
			if strings.Contains(body, `"kind":"tool"`) && strings.Contains(body, "go test") {
				return // event landed; teardown via test end
			}
		case <-done:
			t.Fatal("stream closed before the event")
		}
	}
}

func TestDiscoverWorkspacesBareArrayShape(t *testing.T) {
	t.Parallel()

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		_, _ = w.Write([]byte(`[{"id":"a"},{"path":"/p/b"}]`))
	}))
	t.Cleanup(srv.Close)

	req, _ := http.NewRequestWithContext(context.Background(), http.MethodGet, srv.URL, nil)
	resp, err := srv.Client().Do(req)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = resp.Body.Close() }()

	// Reuse the decoder path indirectly: the endpoint under test is the
	// unix client, so drive it through the same handler logic here.
	ws, err := decodeWorkspaces(resp.Body)
	if err != nil {
		t.Fatal(err)
	}
	if len(ws) != 2 || ws[0].key() != "a" || ws[1].key() != "/p/b" {
		t.Fatalf("workspaces = %+v", ws)
	}
}

// decodeWorkspaces mirrors discoverWorkspaces' body decode for the bare
// array test (shared shape contract).
func decodeWorkspaces(r *bufio.Reader) ([]crushWorkspace, error) {
	return decodeWorkspacesBytes(func(b []byte) ([]crushWorkspace, error) {
		var list struct {
			Workspaces []crushWorkspace `json:"workspaces"`
		}
		if err := jsonUnmarshal(b, &list); err != nil {
			var bare []crushWorkspace
			if err2 := jsonUnmarshal(b, &bare); err2 != nil {
				return nil, err
			}
			return bare, nil
		}
		return list.Workspaces, nil
	})(r)
}
