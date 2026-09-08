package harvest

import (
	"context"
	"encoding/json"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
)

// captureHandler records slog records so tests can pin the fallback
// contract: one warning per failed daemon discovery, never an error return.
type captureHandler struct {
	mu      sync.Mutex
	records []slog.Record
}

func (h *captureHandler) Enabled(context.Context, slog.Level) bool { return true }

func (h *captureHandler) Handle(_ context.Context, r slog.Record) error {
	h.mu.Lock()
	defer h.mu.Unlock()

	h.records = append(h.records, r)

	return nil
}

func (h *captureHandler) WithAttrs([]slog.Attr) slog.Handler { return h }

func (h *captureHandler) WithGroup(string) slog.Handler { return h }

func (h *captureHandler) warnings() []string {
	h.mu.Lock()
	defer h.mu.Unlock()

	msgs := make([]string, 0, len(h.records))

	for _, r := range h.records {
		if r.Level >= slog.LevelWarn {
			msgs = append(msgs, r.Message)
		}
	}

	return msgs
}

// daemonFixture is a projects dir with two harvestable repos and one repo
// without a todo file.
type daemonFixture struct {
	dir       string
	withTodoA string
	withTodoB string
	noTodo    string
}

func newDaemonFixture(t *testing.T) daemonFixture {
	t.Helper()

	dir := t.TempDir()

	noTodo := filepath.Join(dir, "gamma")
	if err := os.MkdirAll(noTodo, 0o755); err != nil {
		t.Fatal(err)
	}

	return daemonFixture{
		dir:       dir,
		withTodoA: writeRepo(t, dir, "alpha", "## Work\n\n- [ ] alpha item\n"),
		withTodoB: writeRepo(t, dir, "beta", "- [ ] beta item\n"),
		noTodo:    noTodo,
	}
}

// newDaemonStub starts an httptest server speaking the POST /v1/discover
// contract: it asserts method + path on every request, decodes the
// searchPaths request body, and replies with the given projects. It reports
// the requests it saw.
func newDaemonStub(t *testing.T, projects []daemonProject) (*httptest.Server, func() []daemonDiscoverRequest) {
	t.Helper()

	var (
		mu       sync.Mutex
		requests []daemonDiscoverRequest
	)

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost || r.URL.Path != "/v1/discover" {
			http.NotFound(w, r)

			return
		}

		var req daemonDiscoverRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			http.Error(w, "bad request body", http.StatusBadRequest)

			return
		}

		mu.Lock()
		requests = append(requests, req)
		mu.Unlock()

		w.Header().Set("Content-Type", "application/json")

		_ = json.NewEncoder(w).Encode(daemonDiscoverResponse{Projects: projects})
	}))
	t.Cleanup(srv.Close)

	return srv, func() []daemonDiscoverRequest {
		mu.Lock()
		defer mu.Unlock()

		return requests
	}
}

// TestDiscoverReposDaemonMapping pins the response mapping contract: daemon
// project paths become harvestable repos — absolute paths only (relative ones
// resolve against the projects dir), repos without the todo file drop out,
// duplicates and empty paths collapse, and the result is sorted.
func TestDiscoverReposDaemonMapping(t *testing.T) {
	t.Parallel()

	fx := newDaemonFixture(t)
	delta := writeRepo(t, fx.dir, "delta", "- [ ] delta item\n")

	srv, sawRequests := newDaemonStub(t, []daemonProject{
		{Path: fx.withTodoB},
		{Path: fx.noTodo},
		{Path: fx.withTodoA},
		{Path: fx.withTodoB},
		{Path: ""},
		{Path: "delta"},
	})

	repos, err := DiscoverReposDaemon(context.Background(), srv.URL, fx.dir, DefaultTodoFile)
	if err != nil {
		t.Fatalf("DiscoverReposDaemon: %v", err)
	}

	want := []string{filepath.Join(fx.dir, "alpha"), filepath.Join(fx.dir, "beta"), delta}
	if len(repos) != len(want) {
		t.Fatalf("repos = %v, want %v", repos, want)
	}

	for i, w := range want {
		if repos[i] != w {
			t.Fatalf("repos[%d] = %s, want %s", i, repos[i], w)
		}
	}

	requests := sawRequests()
	if len(requests) != 1 {
		t.Fatalf("daemon saw %d requests, want 1", len(requests))
	}

	if req := requests[0]; len(req.SearchPaths) != 1 || req.SearchPaths[0] != fx.dir {
		t.Fatalf("request searchPaths = %v, want [%s]", req.SearchPaths, fx.dir)
	}
}

func TestDiscoverReposDaemonErrors(t *testing.T) {
	t.Parallel()

	fx := newDaemonFixture(t)

	tests := []struct {
		name    string
		handler http.HandlerFunc
		wantErr string
	}{
		{
			name: "non-200 reports the status",
			handler: func(w http.ResponseWriter, _ *http.Request) {
				http.Error(w, `{"error":"discovery failed: boom"}`, http.StatusInternalServerError)
			},
			wantErr: "status 500",
		},
		{
			name: "non-JSON body is an error",
			handler: func(w http.ResponseWriter, _ *http.Request) {
				w.Header().Set("Content-Type", "application/json")

				_, _ = w.Write([]byte("<html>not json</html>"))
			},
			wantErr: "decode daemon discovery response",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			srv := httptest.NewServer(tt.handler)
			t.Cleanup(srv.Close)

			_, err := DiscoverReposDaemon(context.Background(), srv.URL, fx.dir, DefaultTodoFile)
			if err == nil || !strings.Contains(err.Error(), tt.wantErr) {
				t.Fatalf("err = %v, want containing %q", err, tt.wantErr)
			}
		})
	}
}

// TestDiscoverReposDaemonUnreachable proves an unreachable socket is an
// ordinary error return (the fallback lives in DiscoverReposFor).
func TestDiscoverReposDaemonUnreachable(t *testing.T) {
	t.Parallel()

	socket := filepath.Join(t.TempDir(), "missing.sock")

	_, err := DiscoverReposDaemon(context.Background(), socket, t.TempDir(), DefaultTodoFile)
	if err == nil {
		t.Fatal("unreachable socket must return an error")
	}
}

// TestDiscoverReposForFallsBackToScan pins the additive-daemon contract: an
// unreachable daemon degrades to the local scan with exactly one warning and
// never fails the tick.
func TestDiscoverReposForFallsBackToScan(t *testing.T) {
	t.Parallel()

	fx := newDaemonFixture(t)

	capture := &captureHandler{}

	socket := filepath.Join(t.TempDir(), "missing.sock")

	repos, err := DiscoverReposFor(context.Background(), socket, fx.dir, DefaultTodoFile, slog.New(capture))
	if err != nil {
		t.Fatalf("DiscoverReposFor: %v", err)
	}

	want := []string{fx.withTodoA}
	if len(repos) != len(want) || repos[0] != want[0] {
		t.Fatalf("repos = %v, want %v", repos, want)
	}

	warnings := capture.warnings()
	if len(warnings) != 1 {
		t.Fatalf("got %d warnings, want exactly 1: %q", len(warnings), warnings)
	}
}

// TestDiscoverReposForPrefersDaemonResult proves a reachable daemon replaces
// (not merges with) the local scan.
func TestDiscoverReposForPrefersDaemonResult(t *testing.T) {
	t.Parallel()

	fx := newDaemonFixture(t)

	srv, _ := newDaemonStub(t, []daemonProject{{Path: fx.withTodoA}, {Path: "gamma"}})

	repos, err := DiscoverReposFor(context.Background(), srv.URL, fx.dir, DefaultTodoFile, slog.New(&captureHandler{}))
	if err != nil {
		t.Fatalf("DiscoverReposFor: %v", err)
	}

	want := []string{fx.withTodoA}
	if len(repos) != 1 || repos[0] != want[0] {
		t.Fatalf("repos = %v, want %v (gamma has no todo file)", repos, want)
	}
}

// TestDiscoverReposForScanDefault pins the zero-external-services default:
// no addr means the plain local scan.
func TestDiscoverReposForScanDefault(t *testing.T) {
	t.Parallel()

	fx := newDaemonFixture(t)

	repos, err := DiscoverReposFor(context.Background(), "", fx.dir, DefaultTodoFile, nil)
	if err != nil {
		t.Fatalf("DiscoverReposFor: %v", err)
	}

	if len(repos) != 1 || repos[0] != fx.withTodoA {
		t.Fatalf("repos = %v, want [%s]", repos, fx.withTodoA)
	}
}

// TestRunUsesDaemonDiscovery wires the whole loop end to end: the daemon
// names only one of the two locally harvestable repos, and the tick must
// enqueue exactly that repo's item.
func TestRunUsesDaemonDiscovery(t *testing.T) {
	t.Parallel()

	fx := newDaemonFixture(t)

	srv, _ := newDaemonStub(t, []daemonProject{{Path: fx.withTodoB}})

	q := openQueue(t)

	res, err := New(q, Config{ProjectsDir: fx.dir, DiscoveryAddr: srv.URL, Log: slog.New(&captureHandler{})}).Run(context.Background())
	if err != nil {
		t.Fatalf("Run: %v", err)
	}

	if res.Repos != 1 {
		t.Fatalf("res.Repos = %d, want 1 (daemon offered only beta)", res.Repos)
	}

	if len(res.Enqueued) != 1 || res.Enqueued[0].Item.RepoName != "beta" {
		t.Fatalf("enqueued = %+v, want exactly beta's item", res.Enqueued)
	}

	if strings.Contains(res.Enqueued[0].Item.Repo, fx.withTodoA) {
		t.Fatalf("alpha must not be harvested via daemon mode: %+v", res.Enqueued[0])
	}
}

// TestRunDaemonDownStillHarvests pins the tick guarantee: a dead daemon
// never fails or empties a harvest tick — the scan result feeds it instead.
func TestRunDaemonDownStillHarvests(t *testing.T) {
	t.Parallel()

	fx := newDaemonFixture(t)

	socket := filepath.Join(t.TempDir(), "missing.sock")

	q := openQueue(t)

	res, err := New(q, Config{ProjectsDir: fx.dir, DiscoveryAddr: socket, Log: slog.New(&captureHandler{})}).Run(context.Background())
	if err != nil {
		t.Fatalf("Run must not fail when the daemon is down: %v", err)
	}

	found := false

	for _, en := range res.Enqueued {
		if en.Item.RepoName == filepath.Base(fx.withTodoB) {
			found = true
		}
	}

	if !found {
		t.Fatalf("fallback scan must harvest beta: %+v", res.Enqueued)
	}
}
