package harvest

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"net/url"
	"path/filepath"
	"strings"
	"sync"
	"time"
)

// Watch reconnect backoff tuning, mirroring the daemon client's own loop: a
// dropped socket (daemon restart, crash) must not silently kill the trigger,
// and a hammering reconnect must not spin.
const (
	defaultWatchInitialBackoff = 500 * time.Millisecond
	defaultWatchMaxBackoff     = 30 * time.Second
)

// watchScanLimit bounds one SSE data frame. Watch events carry the enriched
// project JSON; the bufio.Scanner default of 64KB would silently truncate it.
const watchScanLimit = 1 << 20

// Daemon watch event type names (project-discovery-daemon wire protocol:
// domain.EventType strings). Only project mutations matter for harvesting;
// StreamConnected doubles as the connection-ready signal.
const (
	watchEventTypeStreamConnected = "StreamConnected"
	watchEventTypeProjectAdded    = "WatchProjectAdded"
	watchEventTypeProjectChanged  = "WatchProjectChanged"
)

// daemonWatchEvent is one SSE data frame from GET /v1/watch (domain.Event,
// stripped to the fields the trigger needs).
type daemonWatchEvent struct {
	Type string `json:"type"`
	Path string `json:"path"`
}

// WatchConfig controls the watch-driven harvest trigger.
type WatchConfig struct {
	// Addr is the daemon endpoint, same forms as Config.DiscoveryAddr
	// (unix socket path, unix:// prefixed, or host:port).
	Addr string
	// ProjectsDir scopes triggers to repos under this directory; events for
	// projects elsewhere on the daemon never fire a tick. Ignored when
	// Repos is set.
	ProjectsDir string
	// Repos, when set, restricts triggers to exactly these repos (the
	// pool's --repos mode) and overrides the ProjectsDir scope.
	Repos []string
	// RepoIntervals debounces triggers per repo, keyed by repo dir base
	// name (the same key the Harvester paces enqueues by): after a trigger
	// for a repo, further watch events for it wait out the repo's gap.
	// Repos without an entry have no explicit gap — the coalescing trigger
	// channel and the harvester's own pacing still apply.
	RepoIntervals map[string]time.Duration
	// Log receives connection and drop warnings; nil discards.
	Log *slog.Logger
}

// Watcher subscribes to the daemon's GET /v1/watch SSE stream and converts
// project events into coalesced harvest-tick triggers. It is purely additive:
// a dead or missing watch stream never stalls the pool — the interval tick
// stays the fallback heartbeat, and the trigger channel simply stays silent.
type Watcher struct {
	cfg            WatchConfig
	triggers       chan struct{}
	initialBackoff time.Duration

	mu          sync.Mutex
	lastTrigger map[string]time.Time
}

// NewWatcher creates a Watcher. Signals are delivered on a coalescing channel
// of capacity one: while a tick runs, further events collapse into at most
// one pending tick.
func NewWatcher(cfg WatchConfig) *Watcher {
	return &Watcher{
		cfg:            cfg,
		triggers:       make(chan struct{}, 1),
		initialBackoff: defaultWatchInitialBackoff,
		lastTrigger:    make(map[string]time.Time),
	}
}

// Triggers yields harvest-tick signals. The channel is never closed; when the
// daemon is unreachable it simply stays quiet (the interval tick owns
// progress). A nil channel (watch off) blocks its select case forever.
func (w *Watcher) Triggers() <-chan struct{} { return w.triggers }

// Run subscribes to the watch stream and reconnects with exponential backoff
// until ctx is cancelled. It never returns an error for a dead daemon: each
// failed connect or dropped stream logs one warning and retries — degradation
// to interval-only harvesting, never a stalled pool.
func (w *Watcher) Run(ctx context.Context) error {
	backoff := w.initialBackoff
	if backoff <= 0 {
		backoff = defaultWatchInitialBackoff
	}

	for {
		if ctx.Err() != nil {
			return nil
		}

		err := w.streamOnce(ctx)
		if ctx.Err() != nil {
			return nil
		}

		if w.cfg.Log != nil {
			w.cfg.Log.Warn(
				"harvest: daemon watch stream dropped, reconnecting (interval tick remains the fallback)",
				"addr", w.cfg.Addr,
				"err", err,
				"backoff", backoff,
			)
		}

		if !sleepContext(ctx, backoff) {
			return nil
		}

		backoff = min(backoff*2, defaultWatchMaxBackoff)
	}
}

// streamOnce opens one SSE connection and dispatches events until the stream
// ends (server close, transport error, or ctx cancellation).
func (w *Watcher) streamOnce(ctx context.Context) error {
	client, requestURL := daemonHTTPClient(w.cfg.Addr)
	if client != http.DefaultClient {
		// Only the per-watcher unix-socket client is ours to close; the
		// shared default client's idle connections are not.
		defer client.CloseIdleConnections()
	}

	watchURL := requestURL + "/v1/watch"
	if w.cfg.ProjectsDir != "" && len(w.cfg.Repos) == 0 {
		// Server-side prefilter: only events under our projects root.
		watchURL += "?" + url.Values{"search_path": []string{w.cfg.ProjectsDir}}.Encode()
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, watchURL, nil)
	if err != nil {
		return fmt.Errorf("harvest: create watch request: %w", err)
	}

	req.Header.Set("Accept", "text/event-stream")

	resp, err := client.Do(req)
	if err != nil {
		return fmt.Errorf("harvest: daemon watch at %s: %w", w.cfg.Addr, err)
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode != http.StatusOK {
		reply, _ := io.ReadAll(io.LimitReader(resp.Body, 4<<10))

		return fmt.Errorf(
			"harvest: daemon watch at %s: status %d: %s",
			w.cfg.Addr,
			resp.StatusCode,
			strings.TrimSpace(string(reply)),
		)
	}

	if contentType := resp.Header.Get("Content-Type"); !strings.HasPrefix(contentType, "text/event-stream") {
		return fmt.Errorf(
			"harvest: daemon watch at %s: content type %q, want text/event-stream",
			w.cfg.Addr,
			contentType,
		)
	}

	return scanWatchStream(resp.Body, w.dispatchEvent)
}

// scanWatchStream parses an SSE stream (WHATWG subset) and emits each decoded
// data frame: blank lines dispatch accumulated "data:" lines joined with
// newlines, comment frames (heartbeats) and other fields are ignored, and one
// leading space after "data:" is stripped per spec.
func scanWatchStream(r io.Reader, emit func(daemonWatchEvent)) error {
	scanner := bufio.NewScanner(r)
	scanner.Buffer(make([]byte, 0, 64<<10), watchScanLimit)

	var dataLines []string

	for scanner.Scan() {
		line := scanner.Text()

		if line == "" {
			if len(dataLines) > 0 {
				payload := strings.Join(dataLines, "\n")
				dataLines = dataLines[:0]

				var event daemonWatchEvent
				if err := json.Unmarshal([]byte(payload), &event); err == nil {
					emit(event)
				}
			}

			continue
		}

		if strings.HasPrefix(line, ":") || !strings.HasPrefix(line, "data:") {
			continue
		}

		value := line[len("data:"):]
		if len(value) > 0 && value[0] == ' ' {
			value = value[1:]
		}

		dataLines = append(dataLines, value)
	}

	if err := scanner.Err(); err != nil {
		return fmt.Errorf("harvest: parse watch stream: %w", err)
	}

	return nil
}

// dispatchEvent applies the trigger rules to one daemon event: only project
// additions and changes fire, the project must be in the configured scope,
// and the repo must be outside its --repo-interval debounce gap.
func (w *Watcher) dispatchEvent(event daemonWatchEvent) {
	if event.Type != watchEventTypeProjectAdded && event.Type != watchEventTypeProjectChanged {
		return
	}

	repo := cleanWatchPath(event.Path)

	if !w.inScope(repo) {
		return
	}

	if !w.debounceElapsed(repo) {
		return
	}

	if w.cfg.Log != nil {
		w.cfg.Log.Info(
			"harvest: watch event triggers tick",
			"repo", filepath.Base(repo),
			"event", event.Type,
		)
	}

	select {
	case w.triggers <- struct{}{}:
	default:
	}
}

// inScope reports whether a repo (cleaned, absolute) belongs to the
// configured harvest set: exactly one of --repos when set, otherwise under
// the projects dir, otherwise (neither configured) everything.
func (w *Watcher) inScope(repo string) bool {
	if len(w.cfg.Repos) > 0 {
		for _, candidate := range w.cfg.Repos {
			if cleanWatchPath(candidate) == repo {
				return true
			}
		}

		return false
	}

	if w.cfg.ProjectsDir == "" {
		return true
	}

	root := cleanWatchPath(w.cfg.ProjectsDir)

	rel, err := filepath.Rel(root, repo)

	return err == nil && rel != ".." && !strings.HasPrefix(rel, ".."+string(filepath.Separator))
}

// debounceElapsed records and reports whether this repo may trigger now: a
// repo triggers at most once per its --repo-interval gap (looked up by repo
// dir base name, the same key the Harvester paces enqueues by).
func (w *Watcher) debounceElapsed(repo string) bool {
	gap := w.cfg.RepoIntervals[filepath.Base(repo)]

	w.mu.Lock()
	defer w.mu.Unlock()

	now := time.Now()

	if gap > 0 {
		if last, seen := w.lastTrigger[repo]; seen && now.Sub(last) < gap {
			return false
		}
	}

	w.lastTrigger[repo] = now

	return true
}

// cleanWatchPath normalizes an event path for comparison: absolute and
// cleaned, so daemon-normalized paths match local --repos/--projects-dir
// spellings.
func cleanWatchPath(path string) string {
	if abs, err := filepath.Abs(path); err == nil {
		return abs
	}

	return filepath.Clean(path)
}

// sleepContext blocks for d (or until ctx is cancelled), reporting whether
// the sleep completed.
func sleepContext(ctx context.Context, d time.Duration) bool {
	if d <= 0 {
		return ctx.Err() == nil
	}

	timer := time.NewTimer(d)
	defer timer.Stop()

	select {
	case <-ctx.Done():
		return false
	case <-timer.C:
		return true
	}
}
