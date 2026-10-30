package webui

import (
	"bufio"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"net"
	"net/http"
	"strings"
	"sync"
	"time"

	"github.com/larsartmann/go-sse"
)

// AgentEvent is one live agent line for the L2 pane: a distilled frame from
// a routed run's workspace event stream (tool calls, tool results, session
// lifecycle), labeled with the workspace so the pane can group output when
// several agents run at once. Read-only observability (ADR-0003 posture):
// the feed never influences dispatch.
type AgentEvent struct {
	Workspace string `json:"workspace"`
	Kind      string `json:"kind"`
	Line      string `json:"line"`
}

// AgentFeed fans out live agent events to pane subscribers. Unlike the hub
// (payload-less ticks driving full snapshots), events carry their payload:
// the pane is an append-only log, so replay is impossible and every event
// is its own snapshot.
type AgentFeed struct {
	bc *sse.Broadcaster[AgentEvent]
}

// NewAgentFeed creates a ready-to-use feed.
func NewAgentFeed() *AgentFeed {
	return &AgentFeed{
		bc: sse.NewBroadcaster[AgentEvent](sse.WithBufferSize[AgentEvent](subscriberBufferSize)),
	}
}

func (f *AgentFeed) broadcast(e AgentEvent) {
	f.bc.Broadcast(e)
}

// Subscribe returns a channel receiving live agent events.
func (f *AgentFeed) Subscribe() <-chan AgentEvent {
	return f.bc.Subscribe()
}

// Unsubscribe removes a subscriber channel.
func (f *AgentFeed) Unsubscribe(ch <-chan AgentEvent) {
	f.bc.Unsubscribe(ch)
}

// Shutdown drains the broadcaster.
func (f *AgentFeed) Shutdown(ctx context.Context) error {
	if err := f.bc.Shutdown(ctx); err != nil {
		return fmt.Errorf("shutdown agent feed: %w", err)
	}

	return nil
}

// crushUnixClient builds an HTTP client speaking to a unix socket (the
// pool's `crush server --host unix://<socket>`; the path form and the
// unix:// URL form both reduce to the raw socket path).
func crushUnixClient(socket string) *http.Client {
	socket = strings.TrimPrefix(socket, "unix://")
	return &http.Client{
		Transport: &http.Transport{
			DialContext: func(ctx context.Context, _, _ string) (net.Conn, error) {
				var d net.Dialer
				return d.DialContext(ctx, "unix", socket)
			},
		},
		Timeout: 0, // SSE attaches stream for minutes; per-request contexts bound it
	}
}

// crushWorkspace is the subset of GET /v1/workspaces entries the collector
// needs: an identity for the events URL (id preferred, path as fallback —
// the API has carried both shapes across versions).
type crushWorkspace struct {
	ID   string `json:"id"`
	Path string `json:"path"`
}

func (w crushWorkspace) key() string {
	if w.ID != "" {
		return w.ID
	}

	return w.Path
}

// discoverWorkspaces lists the crush server's current workspaces.
func discoverWorkspaces(ctx context.Context, client *http.Client) ([]crushWorkspace, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, "http://crush/v1/workspaces", nil)
	if err != nil {
		return nil, fmt.Errorf("build workspaces request: %w", err)
	}

	resp, err := client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("list workspaces: %w", err)
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("list workspaces: status %d", resp.StatusCode)
	}

	body, err := io.ReadAll(io.LimitReader(resp.Body, workspaceBodyLimit))
	if err != nil {
		return nil, fmt.Errorf("read workspaces body: %w", err)
	}

	return decodeWorkspacesBody(body)
}

// workspaceBodyLimit caps the workspace-list read: a pathological or
// hostile response must not balloon serve memory.
const workspaceBodyLimit = 1 << 20

// decodeWorkspacesBody accepts both list shapes the crush API has carried:
// {"workspaces":[...]} and a bare [...].
func decodeWorkspacesBody(body []byte) ([]crushWorkspace, error) {
	var list struct {
		Workspaces []crushWorkspace `json:"workspaces"`
	}

	if err := json.Unmarshal(body, &list); err != nil {
		var bare []crushWorkspace
		if err2 := json.Unmarshal(body, &bare); err2 != nil {
			return nil, fmt.Errorf("decode workspaces: %w", err)
		}

		return bare, nil
	}

	return list.Workspaces, nil
}

// distillFrame renders one workspace SSE data frame into a pane line, or
// "" for frames the pane does not care about (session token counters,
// assistant prose streaming). The pane's charter is OPERATOR signal:
// what tools the fleet is running right now, not chat transcripts.
func distillFrame(frame string) (kind, line string) {
	switch {
	case strings.Contains(frame, `"tool_call"`):
		// The crush envelope nests the part under "part" (E3 captures:
		// {"type":"message.updated","part":{"type":"tool_call",
		// "tool_call":{"name":...,"command":...}}}).
		var envelope struct {
			Part struct {
				ToolCall struct {
					Name    string          `json:"name"`
					Input   json.RawMessage `json:"input"`
					Command string          `json:"command"`
				} `json:"tool_call"`
			} `json:"part"`
		}
		if err := json.Unmarshal([]byte(frame), &envelope); err != nil || envelope.Part.ToolCall.Name == "" {
			return "tool", excerpt(frame)
		}
		if envelope.Part.ToolCall.Command != "" {
			return "tool", envelope.Part.ToolCall.Name + " " + excerpt(envelope.Part.ToolCall.Command)
		}
		return "tool", envelope.Part.ToolCall.Name + " " + excerpt(string(envelope.Part.ToolCall.Input))
	case strings.Contains(frame, `"tool_result"`):
		return "result", excerpt(frame)
	case strings.Contains(frame, "session.created"):
		return "session", "session started"
	case strings.Contains(frame, "session.updated"):
		return "", ""
	default:
		return "", ""
	}
}

// excerpt caps a line to pane-friendly length. Byte-exact: the cut
// plus the three-byte ellipsis stays within the 220-byte pane budget.
func excerpt(s string) string {
	s = strings.TrimSpace(s)
	if len(s) > 217 {
		return s[:216] + "…"
	}

	return s
}

// attachWorkspace streams one workspace's events into the feed until the
// workspace disappears (404/410) or ctx is done. Reconnects with a short
// backoff on transport errors — the race-attach pattern from the
// client-server relations doc (E3): attach mid-run is supported, so the
// collector may connect at any point of a run's lifetime.
func attachWorkspace(ctx context.Context, client *http.Client, feed *AgentFeed, ws crushWorkspace, clientID string) {
	key := ws.key()
	label := ws.Path
	if label == "" {
		label = key
	}

	for {
		if ctx.Err() != nil {
			return
		}

		url := fmt.Sprintf("http://crush/v1/workspaces/%s/events?client_id=%s", key, clientID)
		req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
		if err != nil {
			return
		}
		req.Header.Set("Accept", "text/event-stream")

		resp, err := client.Do(req)
		if err != nil {
			if ctx.Err() != nil {
				return
			}
			select {
			case <-ctx.Done():
				return
			case <-time.After(time.Second):
			}
			continue
		}

		if resp.StatusCode == http.StatusNotFound || resp.StatusCode == http.StatusGone {
			_ = resp.Body.Close()
			return // workspace reaped: normal end of life
		}
		if resp.StatusCode != http.StatusOK {
			_ = resp.Body.Close()
			select {
			case <-ctx.Done():
				return
			case <-time.After(2 * time.Second):
			}
			continue
		}

		scanFrames(ctx, resp.Body, feed, label)
		_ = resp.Body.Close()

		if ctx.Err() != nil {
			return
		}
		select {
		case <-ctx.Done():
			return
		case <-time.After(time.Second):
		}
	}
}

// scanFrames reads data:-only SSE frames (the crush server sends no
// event: names) and broadcasts the distilled ones.
func scanFrames(ctx context.Context, body io.Reader, feed *AgentFeed, label string) {
	scanner := bufio.NewScanner(body)
	scanner.Buffer(make([]byte, 0, 64*1024), 1<<20)

	for scanner.Scan() {
		if ctx.Err() != nil {
			return
		}

		line := scanner.Text()
		if after, ok := strings.CutPrefix(line, "data:"); ok {
			if kind, distilled := distillFrame(strings.TrimSpace(after)); kind != "" {
				feed.broadcast(AgentEvent{Workspace: label, Kind: kind, Line: distilled})
			}
		}
	}
}

// RunAgentFeed is the L2 collector: poll the crush server's workspace list,
// attach an event stream to every new workspace, and stop attaching to
// gone ones (their attach loops exit on 404). One goroutine per active
// workspace; the whole collector stops with ctx. A missing server is a
// warn-and-retry (the pool may start it later; tq serve outlives pools).
func RunAgentFeed(ctx context.Context, socket string, feed *AgentFeed, log *slog.Logger) error {
	client := crushUnixClient(socket)

	var (
		mu   sync.Mutex
		live = map[string]struct{}{}
	)

	ticker := time.NewTicker(2 * time.Second)
	defer ticker.Stop()

	for {
		workspaces, err := discoverWorkspaces(ctx, client)
		if err != nil {
			if ctx.Err() != nil {
				return nil
			}
			log.Warn("agent feed: crush server unreachable", "socket", socket, "err", err.Error())
		}

		seen := map[string]struct{}{}
		for _, ws := range workspaces {
			key := ws.key()
			if key == "" {
				continue
			}
			seen[key] = struct{}{}

			mu.Lock()
			_, attached := live[key]
			if !attached {
				live[key] = struct{}{}
			}
			mu.Unlock()

			if !attached {
				go attachWorkspace(ctx, client, feed, ws, "tq-webui-"+socket)
			}
		}

		mu.Lock()
		for key := range live {
			if _, ok := seen[key]; !ok {
				delete(live, key) // attach loop exits on its own 404
			}
		}
		mu.Unlock()

		select {
		case <-ctx.Done():
			return nil
		case <-ticker.C:
		}
	}
}

// errAgentFeedDisabled is served on /agents/live when Config.CrushServer
// is empty: the pane is opt-in (it needs routed runs, L1).
var errAgentFeedDisabled = errors.New("agent feed disabled: serve without --crush-server")
