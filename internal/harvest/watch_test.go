package harvest

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"net"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"
)

// watchFrame renders one daemon SSE frame: the event name field plus the
// domain.Event JSON payload (the trigger only decodes data:).
func watchFrame(eventType, path string) string {
	payload := fmt.Sprintf(`{"type":%q,"timestamp":"2026-09-08T12:00:00Z","path":%q}`, eventType, path)

	return "event: " + eventType + "\ndata: " + payload + "\n\n"
}

// watchStubServer serves GET /v1/watch over TCP (httptest). Every connection
// runs script(connIndex, emit, ctx) — emit writes and flushes a raw frame —
// and the stub records each request's search_path query values. When script
// returns, the connection closes (the reconnect-test drop mechanism).
func watchStubServer(
	t *testing.T,
	script func(conn int32, emit func(frame string), ctx context.Context),
) (*httptest.Server, func() []string) {
	t.Helper()

	var (
		conns    atomic.Int32
		mu       sync.Mutex
		searches []string
	)

	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet || r.URL.Path != "/v1/watch" {
			http.NotFound(w, r)

			return
		}

		mu.Lock()

		searches = append(searches, strings.Join(r.URL.Query()["search_path"], ","))
		mu.Unlock()

		w.Header().Set("Content-Type", "text/event-stream")

		flusher, ok := w.(http.Flusher)
		if !ok {
			http.Error(w, "stub needs a flusher", http.StatusInternalServerError)

			return
		}

		emit := func(frame string) {
			_, _ = fmt.Fprint(w, frame)

			flusher.Flush()
		}

		emit(watchFrame("StreamConnected", ""))

		script(conns.Add(1), emit, r.Context())
	})

	srv := httptest.NewServer(handler)
	t.Cleanup(srv.Close)

	return srv, func() []string {
		mu.Lock()
		defer mu.Unlock()

		return append([]string(nil), searches...)
	}
}

// awaitTrigger waits for one trigger signal; ok is false on timeout.
func awaitTrigger(t *testing.T, triggers <-chan struct{}) bool {
	t.Helper()

	select {
	case <-triggers:
		return true
	case <-time.After(2 * time.Second):
		return false
	}
}

// assertNoTrigger pins that no signal arrives within the quiet window.
func assertNoTrigger(t *testing.T, triggers <-chan struct{}) {
	t.Helper()

	select {
	case <-triggers:
		t.Fatal("unexpected harvest trigger")
	case <-time.After(150 * time.Millisecond):
	}
}

// startWatcher runs the watcher with a tiny reconnect backoff until the test
// ends, and fails the test if Run returns anything but nil on shutdown.
func startWatcher(t *testing.T, cfg WatchConfig) (*Watcher, <-chan struct{}) {
	t.Helper()

	watcher := NewWatcher(cfg)
	watcher.initialBackoff = 10 * time.Millisecond
	triggers := watcher.Triggers()

	ctx, cancel := context.WithCancel(context.Background())

	done := make(chan error, 1)
	go func() { done <- watcher.Run(ctx) }()

	t.Cleanup(func() {
		cancel()

		select {
		case err := <-done:
			if err != nil {
				t.Errorf("watcher Run: %v", err)
			}
		case <-time.After(2 * time.Second):
			t.Error("watcher Run did not stop on cancel")
		}
	})

	return watcher, triggers
}

// holdStream blocks until the connection ends (the stay-open tail for stub
// scripts that should NOT drop the stream).
func holdStream(ctx context.Context) {
	<-ctx.Done()
}

// TestWatcherTriggersOnProjectEvents pins the core contract: a
// WatchProjectChanged event for a repo inside the projects dir fires a
// harvest trigger within seconds, and the watch request asks the daemon to
// scope events to that dir (search_path prefilter).
func TestWatcherTriggersOnProjectEvents(t *testing.T) {
	t.Parallel()

	fx := newDaemonFixture(t)

	srv, searches := watchStubServer(t, func(conn int32, emit func(string), ctx context.Context) {
		if conn == 1 {
			emit(watchFrame("WatchProjectChanged", fx.withTodoA))
		}

		holdStream(ctx)
	})

	_, triggers := startWatcher(t, WatchConfig{
		Addr:        srv.Listener.Addr().String(),
		ProjectsDir: fx.dir,
	})

	if !awaitTrigger(t, triggers) {
		t.Fatal("no trigger for in-scope WatchProjectChanged")
	}

	if got := searches(); len(got) != 1 || got[0] != fx.dir {
		t.Fatalf("search_path records = %v, want exactly [%s]", got, fx.dir)
	}
}

// TestWatcherDebouncePerRepoInterval pins the per-repo debounce: a repo with a
// --repo-interval gap triggers at most once per gap.
func TestWatcherDebouncePerRepoInterval(t *testing.T) {
	t.Parallel()

	fx := newDaemonFixture(t)

	srv, _ := watchStubServer(t, func(conn int32, emit func(string), ctx context.Context) {
		if conn == 1 {
			emit(watchFrame("WatchProjectChanged", fx.withTodoA))
			emit(watchFrame("WatchProjectChanged", fx.withTodoA))
		}

		holdStream(ctx)
	})

	_, triggers := startWatcher(t, WatchConfig{
		Addr:          srv.Listener.Addr().String(),
		ProjectsDir:   fx.dir,
		RepoIntervals: map[string]time.Duration{"alpha": time.Hour},
	})

	if !awaitTrigger(t, triggers) {
		t.Fatal("first event for alpha must trigger")
	}

	assertNoTrigger(t, triggers) // second alpha event inside the 1h gap
}

// TestWatcherIgnoresIrrelevantAndOutOfScopeEvents pins the filter rules:
// event types other than project add/change never trigger, and projects
// outside the configured root never do.
func TestWatcherIgnoresIrrelevantAndOutOfScopeEvents(t *testing.T) {
	t.Parallel()

	fx := newDaemonFixture(t)

	srv, _ := watchStubServer(t, func(conn int32, emit func(string), ctx context.Context) {
		if conn == 1 {
			emit(": heartbeat\n\n")
			emit(watchFrame("DiscoveryStarted", fx.dir))
			emit(watchFrame("WatchProjectRemoved", fx.withTodoA))
			emit(watchFrame("WatchProjectChanged", "/elsewhere/other"))
		}

		holdStream(ctx)
	})

	_, triggers := startWatcher(t, WatchConfig{
		Addr:        srv.Listener.Addr().String(),
		ProjectsDir: fx.dir,
	})

	assertNoTrigger(t, triggers)
}

// TestWatcherReposScope pins the --repos mode: only the exact configured
// repos trigger; a sibling inside the projects dir does not.
func TestWatcherReposScope(t *testing.T) {
	t.Parallel()

	fx := newDaemonFixture(t)

	srv, searches := watchStubServer(t, func(conn int32, emit func(string), ctx context.Context) {
		if conn == 1 {
			emit(watchFrame("WatchProjectChanged", fx.withTodoA))
		}

		holdStream(ctx)
	})

	_, triggers := startWatcher(t, WatchConfig{
		Addr:  srv.Listener.Addr().String(),
		Repos: []string{fx.withTodoB},
	})

	assertNoTrigger(t, triggers)

	// --repos mode must not send a search_path prefilter (repos may live
	// anywhere; scoping is client-side).
	if got := searches(); len(got) != 1 || got[0] != "" {
		t.Fatalf("search_path records = %v, want exactly [\"\"]", got)
	}
}

// TestWatcherReconnectsAfterDrop pins the other half of the degradation
// contract: a dropped stream reconnects (with backoff) and keeps triggering,
// so the interval tick never becomes the only path again.
func TestWatcherReconnectsAfterDrop(t *testing.T) {
	t.Parallel()

	fx := newDaemonFixture(t)

	srv, _ := watchStubServer(t, func(conn int32, emit func(string), ctx context.Context) {
		switch conn {
		case 1:
			emit(watchFrame("WatchProjectChanged", fx.withTodoA))
			// Script returns: the stub closes this connection (the drop).
		case 2:
			emit(watchFrame("WatchProjectChanged", fx.withTodoB))
			holdStream(ctx)
		default:
			holdStream(ctx)
		}
	})

	_, triggers := startWatcher(t, WatchConfig{
		Addr:        srv.Listener.Addr().String(),
		ProjectsDir: fx.dir,
	})

	if !awaitTrigger(t, triggers) {
		t.Fatal("no trigger before the drop")
	}

	if !awaitTrigger(t, triggers) {
		t.Fatal("no trigger after reconnect")
	}
}

// TestWatcherLogsDropWarning pins the observability contract: each dropped
// stream produces a warning naming the addr, and the configured
// InitialBackoff/MaxBackoff knobs drive the ladder instead of the defaults.
func TestWatcherLogsDropWarning(t *testing.T) {
	t.Parallel()

	fx := newDaemonFixture(t)

	var mu sync.Mutex
	var messages []string
	log := slog.New(countingHandler{mu: &mu, messages: &messages})

	srv, _ := watchStubServer(t, func(conn int32, emit func(string), ctx context.Context) {
		if conn <= 2 {
			emit(watchFrame("WatchProjectChanged", fx.withTodoA))
			// conn 1 returns (the scripted drop); conn 2 proves reconnect.
		} else {
			holdStream(ctx)
		}
	})

	_, triggers := startWatcher(t, WatchConfig{
		Addr:           srv.Listener.Addr().String(),
		ProjectsDir:    fx.dir,
		Log:            log,
		InitialBackoff: 5 * time.Millisecond,
		MaxBackoff:     15 * time.Millisecond,
	})

	if !awaitTrigger(t, triggers) {
		t.Fatal("no trigger before the drop")
	}
	if !awaitTrigger(t, triggers) {
		t.Fatal("no trigger after reconnect")
	}

	mu.Lock()
	defer mu.Unlock()
	dropped := 0
	for _, msg := range messages {
		if strings.Contains(msg, "watch stream dropped") && strings.Contains(msg, srv.Listener.Addr().String()) {
			dropped++
		}
	}
	if dropped == 0 {
		t.Fatalf("no drop warning logged; got %d warnings", len(messages))
	}
}

// countingHandler records warning messages for assertions.
type countingHandler struct {
	mu       *sync.Mutex
	messages *[]string
}

func (h countingHandler) Enabled(_ context.Context, _ slog.Level) bool { return true }

func (h countingHandler) Handle(_ context.Context, r slog.Record) error {
	h.mu.Lock()
	defer h.mu.Unlock()
	line := r.Message
	r.Attrs(func(a slog.Attr) bool {
		line += " " + a.String()
		return true
	})
	*h.messages = append(*h.messages, line)
	return nil
}

func (h countingHandler) WithAttrs(_ []slog.Attr) slog.Handler { return h }

func (h countingHandler) WithGroup(_ string) slog.Handler { return h }

// TestWatcherUnixSocket covers the primary deployment form: the daemon's
// watch endpoint on a unix socket (bare path and unix:// prefixed addr).
func TestWatcherUnixSocket(t *testing.T) {
	t.Parallel()

	fx := newDaemonFixture(t)

	socket := filepath.Join(t.TempDir(), "daemon.sock")

	ln, err := net.Listen("unix", socket)
	if err != nil {
		t.Fatalf("listen unix: %v", err)
	}

	var conns atomic.Int32

	httpSrv := &http.Server{Handler: http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/event-stream")

		flusher := w.(http.Flusher)
		_, _ = fmt.Fprint(w, watchFrame("StreamConnected", ""))

		flusher.Flush()

		// Each addr form gets its own watcher (its own debounce state),
		// so every connection must carry the event.
		_, _ = fmt.Fprint(w, watchFrame("WatchProjectAdded", fx.withTodoA))

		flusher.Flush()

		conns.Add(1)

		<-r.Context().Done()
	})}

	serveErr := make(chan error, 1)
	go func() { serveErr <- httpSrv.Serve(ln) }()

	t.Cleanup(func() { _ = httpSrv.Close() })

	for _, addr := range []string{socket, "unix://" + socket} {
		_, triggers := startWatcher(t, WatchConfig{Addr: addr, ProjectsDir: fx.dir})

		if !awaitTrigger(t, triggers) {
			t.Fatalf("addr %s: no trigger over unix socket", addr)
		}
	}

	select {
	case err := <-serveErr:
		if err != nil && !errors.Is(err, http.ErrServerClosed) {
			t.Fatalf("stub server: %v", err)
		}
	default:
	}
}

// TestWatcherRunNeverFailsOnDeadDaemon pins that an unreachable daemon only
// logs and retries (the interval tick remains the only trigger), and that
// cancelling stops Run cleanly.
func TestWatcherRunNeverFailsOnDeadDaemon(t *testing.T) {
	t.Parallel()

	fx := newDaemonFixture(t)

	watcher := NewWatcher(WatchConfig{Addr: "127.0.0.1:1", ProjectsDir: fx.dir})
	watcher.initialBackoff = 5 * time.Millisecond

	ctx, cancel := context.WithCancel(context.Background())

	done := make(chan error, 1)
	go func() { done <- watcher.Run(ctx) }()

	time.Sleep(50 * time.Millisecond)
	cancel()

	select {
	case err := <-done:
		if err != nil {
			t.Fatalf("Run on dead daemon returned %v, want nil", err)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("Run did not stop on cancel")
	}
}

// TestScanWatchStream pins the SSE parser subset: multi-line data joins with
// newlines, one leading space after data: is stripped, comment frames and
// other fields are ignored, malformed JSON frames are skipped.
func TestScanWatchStream(t *testing.T) {
	t.Parallel()

	stream := strings.Join([]string{
		": heartbeat",
		"",
		"event: connected",
		`data: {"type":"StreamConnected"}`,
		"",
		"event: WatchProjectChanged",
		`data: {"type":"WatchProjectChanged",`,
		`data: "path":"/tmp/repo"}`,
		"",
		"data: not-json",
		"",
		"id: 7",
		"retry: 1000",
		"",
	}, "\n")

	var got []daemonWatchEvent

	if err := scanWatchStream(
		strings.NewReader(stream),
		func(ev daemonWatchEvent) { got = append(got, ev) },
	); err != nil {
		t.Fatalf("scanWatchStream: %v", err)
	}

	want := []daemonWatchEvent{
		{Type: "StreamConnected"},
		{Type: "WatchProjectChanged", Path: "/tmp/repo"},
	}

	if len(got) != len(want) {
		t.Fatalf("events = %+v, want %+v", got, want)
	}

	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("event[%d] = %+v, want %+v", i, got[i], want[i])
		}
	}
}
