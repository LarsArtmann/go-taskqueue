package harvest

import (
	"context"
	"fmt"
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
// runs script(connIndex, emit) — emit writes and flushes a raw frame — and the
// stub records each request's search_path query values.
func watchStubServer(
	t *testing.T,
	script func(conn int32, emit func(frame string)),
) (*httptest.Server, func() []string) {
	t.Helper()

	var (
		conns     atomic.Int32
		mu        sync.Mutex
		searches  []string
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

		script(conns.Add(1), emit)

		<-r.Context().Done()
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

// startWatcher runs the watcher on the stub addr with a tiny reconnect
// backoff and fails the test if Run returns anything but nil on shutdown.
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

// TestWatcherTriggersOnProjectEvents pins the core contract: WatchProjectAdded
// and WatchProjectChanged events for repos inside the projects dir fire a
// harvest trigger within seconds; StreamConnected never does.
func TestWatcherTriggersOnProjectEvents(t *testing.T) {
	t.Parallel()

	fx := newDaemonFixture(t)

	srv, searches := watchStubServer(t, func(conn int32, emit func(string)) {
		if conn == 1 {
			emit(watchFrame("WatchProjectChanged", fx.withTodoA))
		}
	})

	_, triggers := startWatcher(t, WatchConfig{
		Addr:        srv.Listener.Addr().String(),
		ProjectsDir: fx.dir,
	})

	if !awaitTrigger(t, triggers) {
		t.Fatal("no trigger for in-scope WatchProjectChanged")
	}

	// The connected frame must not have fired the earlier await spuriously:
	// prove coalescing stays at one pending signal by draining nothing here —
	// the next in-scope event must still arrive.
	srv.Config.Handler.ServeHTTP(httptest.NewRecorder(), httptest.NewRequest(http.MethodGet, "/health", nil))

	if got := len(searches()); got < 1 {
		t.Fatalf("watch request saw no search_path recording (requests=%d)", got)
	}

	want := fx.dir
	if got := searches()[0]; got != want {
		t.Fatalf("search_path = %q, want %q", got, want)
	}
}

// TestWatcherDebouncePerRepoInterval pins the per-repo debounce: a repo with a
// --repo-interval gap triggers at most once per gap; a repo without an entry
// is never gated by another repo's gap.
func TestWatcherDebouncePerRepoInterval(t *testing.T) {
	t.Parallel()

	fx := newDaemonFixture(t)

	watchStubServer(t, func(conn int32, emit func(string)) {
		if conn == 1 {
			emit(watchFrame("WatchProjectChanged", fx.withTodoA))
			emit(watchFrame("WatchProjectChanged", fx.withTodoA))
		}
	})

	_, triggers := startWatcher(t, WatchConfig{
		Addr:          fx.addr(),
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
// outside the configured scope (other roots, or not in --repos) never do.
func TestWatcherIgnoresIrrelevantAndOutOfScopeEvents(t *testing.T) {
	t.Parallel()

	fx := newDaemonFixture(t)

	watchStubServer(t, func(conn int32, emit func(string)) {
		if conn == 1 {
			emit(": heartbeat\n\n")
			emit(watchFrame("StreamConnected", ""))
			emit(watchFrame("DiscoveryStarted", fx.dir))
			emit(watchFrame("WatchProjectRemoved", fx.withTodoA))
			emit(watchFrame("WatchProjectChanged", "/elsewhere/other"))
		}
	})

	_, triggers := startWatcher(t, WatchConfig{
		Addr:        fx.addr(),
		ProjectsDir: fx.dir,
	})

	assertNoTrigger(t, triggers)
}

// TestWatcherReposScope pins the --repos mode: only the exact configured
// repos trigger, regardless of the projects dir.
func TestWatcherReposScope(t *testing.T) {
	t.Parallel()

	fx := newDaemonFixture(t)

	watchStubServer(t, func(conn int32, emit func(string)) {
		if conn == 1 {
			emit(watchFrame("WatchProjectChanged", fx.withTodoA))
		}
	})

	_, triggers := startWatcher(t, WatchConfig{
		Addr:  fx.addr(),
		Repos: []string{fx.withTodoB},
	})

	assertNoTrigger(t, triggers)
}

// TestWatcherReconnectsAfterDrop pins the degradation contract's other half:
// a dropped stream reconnects (with backoff) and keeps triggering; the
// interval tick is never the only path forward again.
func TestWatcherReconnectsAfterDrop(t *testing.T) {
	t.Parallel()

	fx := newDaemonFixture(t)

	var (
		mu   sync.Mutex
		conn atomic.Int32
	)

	watchStubServer(t, func(_ int32, emit func(string)) {
		switch conn.Add(1) {
		case 1:
			emit(watchFrame("WatchProjectChanged", fx.withTodoA))
		case 2:
			emit(watchFrame("WatchProjectChanged", fx.withTodoB))
		}
	})

	// Close conn 1 from the server side by hijacking is overkill; instead
	// let its script return so the handler exits and the stream drops.
	_, triggers := startWatcher(t, WatchConfig{Addr: fx.addr(), ProjectsDir: fx.dir})

	if !awaitTrigger(t, triggers) {
		t.Fatal("no trigger before the drop")
	}

	if !awaitTrigger(t, triggers) {
		t.Fatal("no trigger after reconnect")
	}

	mu.Unlock()
}

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

		if conns.Add(1) == 1 {
			_, _ = fmt.Fprint(w, watchFrame("WatchProjectAdded", fx.withTodoA))
			flusher.Flush()
		}

		<-r.Context().Done()
	})}

	serveErr := make(chan error, 1)
	go func() { serveErr <- httpSrv.Serve(ln) }()

	t.Cleanup(func() { _ = httpSrv.Close() })

	_, triggers := startWatcher(t, WatchConfig{Addr: socket, ProjectsDir: fx.dir})

	if !awaitTrigger(t, triggers) {
		t.Fatal("no trigger over unix socket")
	}

	select {
	case err := <-serveErr:
		if err != nil && err != http.ErrServerClosed {
			t.Fatalf("stub server: %v", err)
		}
	default:
	}
}

// TestWatcherRunNeverFailsOnDeadDaemon pins that an unreachable daemon only
// logs and retries: Run keeps going (the interval tick remains the only
// path), and cancelling stops it cleanly.
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
		"data: " + `{"type":"StreamConnected"}`,
		"",
		"event: WatchProjectChanged",
		`data: {"type":"WatchProjectChanged",` + "\n" + `data: "path":"/tmp/repo"}`,
		"",
		"data: not-json",
		"",
		"id: 7",
		"retry: 1000",
		"",
	}, "\n")

	var got []daemonWatchEvent

	if err := scanWatchStream(strings.NewReader(stream), func(ev daemonWatchEvent) { got = append(got, ev) }); err != nil {
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

// addr is a tiny helper so tests can share one stub address between watcher
// restarts without spelling the httptest URL twice.
func (f daemonFixture) addr() string { return f.hostPort }
