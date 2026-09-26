package main

import (
	"context"
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/larsartmann/go-taskqueue/internal/journal"
	"github.com/larsartmann/go-taskqueue/internal/queue"
	"github.com/larsartmann/go-taskqueue/internal/queue/sqlite"
	"github.com/larsartmann/go-taskqueue/internal/session"
	"github.com/larsartmann/go-taskqueue/internal/status"
	"github.com/larsartmann/go-taskqueue/internal/task"
)

func doctorTestStore(t *testing.T) string {
	t.Helper()

	path := filepath.Join(t.TempDir(), "q.db")

	s, err := sqlite.Open(path)
	if err != nil {
		t.Fatalf("OpenSQLite: %v", err)
	}

	defer func() { _ = s.Close() }()

	return path
}

func resultByName(results []checkResult, name string) checkResult {
	for _, r := range results {
		if r.Name == name {
			return r
		}
	}

	return checkResult{Name: name, Status: "missing", Detail: "not found"}
}

func TestDoctorHealthyEmptyDB(t *testing.T) {
	path := doctorTestStore(t)

	// Point the agent-binary check at the test binary itself: worst==ok
	// must not depend on `crush` being installed on the host (the nix
	// sandbox has no crush — its checkPhase caught this assumption). The
	// tool:git/tool:go checks are likewise host-PATH-dependent, so they are
	// excluded from the worst computation.
	self, err := filepath.Abs(os.Args[0])
	if err != nil {
		t.Fatalf("abs test binary: %v", err)
	}

	results, err := runDoctor(context.Background(), doctorOptions{DBPath: path, AgentBin: self})
	if err != nil {
		t.Fatalf("runDoctor: %v", err)
	}

	if worst := doctorWorst(doctorIgnoreTools(results)); worst != checkOK {
		t.Fatalf("worst = %s, want ok; results: %+v", worst, results)
	}

	for _, name := range []string{"db", "wal", "queue", "worker"} {
		if r := resultByName(results, name); r.Status != checkOK {
			t.Errorf("%s = %s (%s), want ok", name, r.Status, r.Detail)
		}
	}
}

func TestDoctorFlagsDeadWorker(t *testing.T) {
	path := doctorTestStore(t)

	s, err := sqlite.Open(path)
	if err != nil {
		t.Fatalf("OpenSQLite: %v", err)
	}
	defer func() { _ = s.Close() }()

	ctx := context.Background()

	// A pending task nobody claims, plus a running task whose lease died
	// with its worker: the classic "pool is down" signature.
	if _, err := s.Enqueue(ctx, task.New{Type: "sh"}); err != nil {
		t.Fatalf("enqueue: %v", err)
	}

	if _, _, err := s.ClaimDue(ctx, "dead-worker", time.Millisecond); err != nil {
		t.Fatalf("claim: %v", err)
	}

	// The lease dies here; no heartbeat ever extends it.
	time.Sleep(20 * time.Millisecond)

	if _, err := s.Enqueue(ctx, task.New{Type: "sh"}); err != nil {
		t.Fatalf("enqueue second: %v", err)
	}

	results, err := runDoctor(ctx, doctorOptions{DBPath: path})
	if err != nil {
		t.Fatalf("runDoctor: %v", err)
	}

	if r := resultByName(results, "worker"); r.Status != checkFail {
		t.Errorf("worker = %s (%s), want fail (pending work, no heartbeats)", r.Status, r.Detail)
	}

	if r := resultByName(results, "queue"); r.Status != checkWarn {
		t.Errorf("queue = %s (%s), want warn (expired lease unreclaimed)", r.Status, r.Detail)
	}

	if worst := doctorWorst(results); worst != checkFail {
		t.Errorf("worst = %s, want fail", worst)
	}
}

// TestDoctorParkedNamesEarliestRelease pins the 16-00 f34 observability:
// when tasks are parked by a provider rate limit, the doctor's parked
// check names the earliest not_before ("earliest release 19:40") so the
// operator knows when to look again.
func TestDoctorParkedNamesEarliestRelease(t *testing.T) {
	path := doctorTestStore(t)

	s, err := sqlite.Open(path)
	if err != nil {
		t.Fatalf("OpenSQLite: %v", err)
	}
	defer func() { _ = s.Close() }()

	ctx := context.Background()

	park := func(delay time.Duration) {
		t.Helper()

		tk, err := s.Enqueue(ctx, task.New{Type: "agent"})
		if err != nil {
			t.Fatalf("enqueue: %v", err)
		}

		if _, claimW1, err := s.ClaimDue(ctx, "w1", time.Minute); err != nil {
			t.Fatalf("claim: %v", err)
		} else if err := s.Requeue(ctx, tk.ID, claimW1, "rate limited", delay, false); err != nil {
			t.Fatalf("requeue: %v", err)
		}
	}

	park(2 * time.Hour)
	park(time.Hour)

	parkedFlag := true

	var earliest time.Time

	if parked, err := s.List(ctx, queue.Filter{Parked: &parkedFlag}); err != nil {
		t.Fatalf("list parked: %v", err)
	} else {
		for _, tk := range parked {
			if earliest.IsZero() || tk.NotBefore.Before(earliest) {
				earliest = tk.NotBefore
			}
		}
	}

	if earliest.IsZero() {
		t.Fatal("no parked not_before found — fixture broken")
	}

	r := doctorParked(ctx, s)

	if r.Status != checkWarn {
		t.Errorf("parked = %s, want warn", r.Status)
	}

	// Mirror doctorParked's day-aware layout: crossing midnight flips the
	// format to "Jan 2 15:04" (the 23:xx flake). In(time.Now().Location())
	// equals Local() but keeps gosmopolitan quiet about time.Local.
	localNow := time.Now()
	layout := "15:04"
	if earliest.In(localNow.Location()).Day() != localNow.Day() {
		layout = "Jan 2 15:04"
	}

	want := "earliest release " + earliest.In(localNow.Location()).Format(layout)
	if !strings.Contains(r.Detail, want) {
		t.Errorf("parked detail = %q, want it to name %q", r.Detail, want)
	}
}

func TestDoctorBudgetAtCap(t *testing.T) {
	path := doctorTestStore(t)

	s, err := sqlite.Open(path)
	if err != nil {
		t.Fatalf("OpenSQLite: %v", err)
	}
	defer func() { _ = s.Close() }()

	ctx := context.Background()
	for range 2 {
		if _, err := s.Enqueue(ctx, task.New{Type: "sh"}); err != nil {
			t.Fatalf("enqueue: %v", err)
		}
	}

	results, err := runDoctor(ctx, doctorOptions{DBPath: path, DailyBudget: 2})
	if err != nil {
		t.Fatalf("runDoctor: %v", err)
	}

	if r := resultByName(results, "budget"); r.Status != checkWarn {
		t.Errorf("budget = %s (%s), want warn at cap", r.Status, r.Detail)
	}
}

func TestDoctorCorruptDB(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "broken.db")

	if err := os.WriteFile(path, []byte("this is definitely not a sqlite database"), 0o644); err != nil {
		t.Fatal(err)
	}

	_, err := runDoctor(context.Background(), doctorOptions{DBPath: path})
	if err == nil {
		t.Fatal("runDoctor on a corrupt file must fail")
	}
}

func TestDoctorRepoAutonomy(t *testing.T) {
	dir := t.TempDir()

	healthy := filepath.Join(dir, "healthy")
	if err := os.MkdirAll(filepath.Join(healthy, ".git"), 0o755); err != nil {
		t.Fatal(err)
	}

	if err := os.WriteFile(
		filepath.Join(healthy, "TODO_LIST.md"),
		[]byte("## Work\n\n- [ ] item\n"),
		0o644,
	); err != nil {
		t.Fatal(err)
	}

	if err := os.WriteFile(filepath.Join(healthy, ".crushrc"), []byte("{}"), 0o644); err != nil {
		t.Fatal(err)
	}

	bare := filepath.Join(dir, "bare")
	if err := os.MkdirAll(bare, 0o755); err != nil {
		t.Fatal(err)
	}

	path := doctorTestStore(t)

	results, err := runDoctor(context.Background(), doctorOptions{DBPath: path, Repos: healthy + "," + bare})
	if err != nil {
		t.Fatalf("runDoctor: %v", err)
	}

	if r := resultByName(results, "repo:healthy"); r.Status != checkOK {
		t.Errorf("repo:healthy = %s (%s), want ok", r.Status, r.Detail)
	}

	if r := resultByName(results, "autonomy:healthy"); r.Status != checkOK {
		t.Errorf("autonomy:healthy = %s (%s), want ok", r.Status, r.Detail)
	}

	if r := resultByName(results, "repo:bare"); r.Status != checkWarn {
		t.Errorf("repo:bare = %s (%s), want warn", r.Status, r.Detail)
	}

	if r := resultByName(results, "autonomy:bare"); r.Status != checkWarn {
		t.Errorf("autonomy:bare = %s (%s), want warn", r.Status, r.Detail)
	}
}

// doctorIgnoreTools drops the tool:<name> checks: they reflect the host
// PATH, which a hermetic test cannot assume.
func doctorIgnoreTools(results []checkResult) []checkResult {
	var kept []checkResult

	for _, r := range results {
		if strings.HasPrefix(r.Name, "tool:") {
			continue
		}

		kept = append(kept, r)
	}

	return kept
}

func TestDoctorToolPathChecks(t *testing.T) {
	empty := t.TempDir()
	t.Setenv("PATH", empty)

	opts := doctorOptions{DBPath: doctorTestStore(t)}

	self, err := filepath.Abs(os.Args[0])
	if err != nil {
		t.Fatalf("abs test binary: %v", err)
	}

	opts.AgentBin = self

	results := doctorEnvironment(context.Background(), opts)

	for _, name := range []string{"tool:git", "tool:go"} {
		r := resultByName(results, name)
		if r.Status != checkWarn {
			t.Errorf("%s = %s (%s), want warn (empty PATH)", name, r.Status, r.Detail)
		}
	}
}

// TestClassifyGoEnvProbe pins the env-lie verdict table: bare-build failure
// with a working experiment build is the FAIL (one-line fix, never an
// attempt burn); both failing is a toolchain gap (warn); a clean bare build
// is ok regardless of the forced-experiment arm.
func TestClassifyGoEnvProbe(t *testing.T) {
	t.Parallel()

	const constraintErr = "package probe imports encoding/json/v2: build constraints exclude all Go files"

	cases := []struct {
		name        string
		bareErr     string
		envErr      string
		wantStatus  string
		wantInDetal []string
	}{
		{
			"clean env",
			"", "",
			checkOK,
			[]string{"no env lie"},
		},
		{
			"env lie",
			constraintErr, "",
			checkFail,
			[]string{"ENV-LIE", "export GOEXPERIMENT=jsonv2", "Environment=GOEXPERIMENT=jsonv2"},
		},
		{
			"toolchain cannot build jsonv2 at all",
			constraintErr, "go: updates to go.mod needed; requires go >= 1.27",
			checkWarn,
			[]string{"toolchain/version gate"},
		},
		{
			"bare ok wins even if forced arm failed",
			"", "anything",
			checkOK,
			nil,
		},
	}

	for _, tc := range cases {
		got := classifyGoEnvProbe(tc.bareErr, tc.envErr)
		if got.Status != tc.wantStatus {
			t.Errorf("%s: status = %s, want %s (detail %q)", tc.name, got.Status, tc.wantStatus, got.Detail)
		}

		for _, want := range tc.wantInDetal {
			if !strings.Contains(got.Detail, want) {
				t.Errorf("%s: detail %q missing %q", tc.name, got.Detail, want)
			}
		}
	}
}

// TestDoctorEnvironmentIncludesGoEnvCheck pins the wiring: the doctor's
// environment sweep reports a go-env result (stubbed hermetically — the
// real probe needs a go toolchain and is covered separately).
func TestDoctorEnvironmentIncludesGoEnvCheck(t *testing.T) {
	t.Setenv("PATH", t.TempDir())

	orig := doctorProbeGoEnv

	t.Cleanup(func() { doctorProbeGoEnv = orig })

	doctorProbeGoEnv = func(context.Context) checkResult {
		return checkResult{Name: "go-env", Status: checkFail, Detail: "ENV-LIE (stub)"}
	}

	results := doctorEnvironment(context.Background(), doctorOptions{DBPath: doctorTestStore(t)})

	r := resultByName(results, "go-env")
	if r.Status != checkFail || !strings.Contains(r.Detail, "ENV-LIE") {
		t.Errorf("go-env check = %+v, want stubbed ENV-LIE fail", r)
	}
}

// TestDoctorGoEnvProbeReal exercises the real probe when a toolchain is
// present (nix checkPhase, dev shells): any sane verdict is acceptable —
// the environment decides — but the probe must produce a classified result
// and never panic or hang. GOCACHE is isolated so sandboxed homes cannot
// poison the run.
func TestDoctorGoEnvProbeReal(t *testing.T) {
	if _, err := exec.LookPath("go"); err != nil {
		t.Skip("go not on PATH")
	}

	dir := t.TempDir()
	cache := filepath.Join(t.TempDir(), "gocache")

	env := append(os.Environ(), "GOCACHE="+cache)

	ambErr, capErr := runGoEnvProbe(context.Background(), dir, env)
	got := classifyGoEnvProbe(ambErr, capErr)

	switch got.Status {
	case checkOK, checkWarn, checkFail:
		if got.Detail == "" {
			t.Fatalf("probe verdict %s must carry a detail", got.Status)
		}
	default:
		t.Fatalf("probe produced unknown status %q (%s)", got.Status, got.Detail)
	}
}

func TestDoctorJSONShape(t *testing.T) {
	path := doctorTestStore(t)

	results, err := runDoctor(context.Background(), doctorOptions{DBPath: path})
	if err != nil {
		t.Fatalf("runDoctor: %v", err)
	}

	payload, err := json.Marshal(map[string]any{"status": doctorWorst(results), "checks": results})
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}

	if !strings.Contains(string(payload), `"name":"db"`) {
		t.Errorf("json payload missing db check: %s", payload)
	}
}

func TestDoctorMarkOrphans(t *testing.T) {
	path := doctorTestStore(t)

	s, err := sqlite.Open(path)
	if err != nil {
		t.Fatalf("OpenSQLite: %v", err)
	}
	defer func() { _ = s.Close() }()

	ctx := context.Background()

	enq, err := s.Enqueue(ctx, task.New{Type: "sh"})
	if err != nil {
		t.Fatal(err)
	}

	if _, _, err := s.ClaimDue(ctx, "victim", time.Millisecond); err != nil {
		t.Fatal(err)
	}

	time.Sleep(5 * time.Millisecond) // lease dies

	results, err := runDoctor(ctx, doctorOptions{DBPath: path, MarkOrphans: true})
	if err != nil {
		t.Fatalf("runDoctor: %v", err)
	}

	r := resultByName(results, "mark-orphans")
	if r.Status != checkOK || !strings.Contains(r.Detail, "1 stranded") {
		t.Errorf("mark-orphans = %s (%s), want ok with 1 stranded", r.Status, r.Detail)
	}

	// The fact is in the journal, exactly once.
	facts, err := s.Facts(ctx, 0, 0)
	if err != nil {
		t.Fatal(err)
	}

	count := 0

	for _, f := range facts {
		if f.TaskID == enq.ID.String() && f.Type == "task.orphaned" {
			count++
		}
	}

	if count != 1 {
		t.Fatalf("task.orphaned facts = %d, want 1", count)
	}
}

func TestDoctorWatermarkLiveness(t *testing.T) {
	s, err := sqlite.Open(filepath.Join(t.TempDir(), "q.db"))
	if err != nil {
		t.Fatalf("OpenSQLite: %v", err)
	}
	defer func() { _ = s.Close() }()

	ctx := context.Background()

	// No cursors: the sweepers never ran here — idle, not sick.
	results := doctorWatermarkLiveness(ctx, s)
	for _, name := range []string{"review-sweeper", "status-sweeper"} {
		if r := resultByName(results, name); r.Status != checkOK {
			t.Errorf("%s = %s (%s), want ok", name, r.Status, r.Detail)
		}
	}

	// A cursor below the head is the "sweeper not running" signature.
	payload, err := json.Marshal(map[string]string{"cmd": "true"})
	if err != nil {
		t.Fatalf("marshal payload: %v", err)
	}

	if _, err := s.Enqueue(ctx, task.New{Type: "sh", Project: "demo", Payload: payload}); err != nil {
		t.Fatalf("enqueue: %v", err)
	}

	if err := s.SaveWatermark(ctx, status.ConsumerKey, 1); err != nil {
		t.Fatalf("save watermark: %v", err)
	}

	if _, err := s.Enqueue(ctx, task.New{Type: "sh", Project: "demo", Payload: payload}); err != nil {
		t.Fatalf("enqueue: %v", err)
	}

	results = doctorWatermarkLiveness(ctx, s)

	if r := resultByName(results, "status-sweeper"); r.Status != checkWarn {
		t.Errorf("status-sweeper = %s (%s), want warn (cursor lags the head)", r.Status, r.Detail)
	}

	if r := resultByName(results, "review-sweeper"); r.Status != checkOK {
		t.Errorf("review-sweeper = %s (%s), want ok (never ran)", r.Status, r.Detail)
	}
}

// TestDoctorRepoCoverageOrphanAndCovered pins the PENDING-forever check:
// a project whose directory is missing under the projects root warns with
// its pending count; a covered project and an empty journal stay ok.
func TestDoctorRepoCoverageOrphanAndCovered(t *testing.T) {
	t.Parallel()

	root := t.TempDir()

	s, err := sqlite.Open(filepath.Join(t.TempDir(), "coverage.db"))
	if err != nil {
		t.Fatalf("open: %v", err)
	}
	defer func() { _ = s.Close() }()

	ctx := context.Background()

	if err := os.MkdirAll(filepath.Join(root, "covered"), 0o755); err != nil {
		t.Fatalf("mkdir covered: %v", err)
	}

	for _, project := range []string{"covered", "ghosted"} {
		if _, err := s.Enqueue(ctx, task.New{
			Type:    "agent",
			Project: project,
			Payload: []byte(`{"repo":"` + project + `","prompt":"p"}`),
		}); err != nil {
			t.Fatalf("enqueue %s: %v", project, err)
		}
	}

	results := doctorRepoCoverage(ctx, s, root)
	got := resultByName(results, "repo-coverage")

	if got.Status != checkWarn {
		t.Fatalf("status = %q, want warn (ghosted project must surface)", got.Status)
	}

	if !strings.Contains(got.Detail, "ghosted") || !strings.Contains(got.Detail, "1 pending task(s)") {
		t.Fatalf("detail = %q, want ghosted project with pending count", got.Detail)
	}

	if strings.Contains(got.Detail, "covered (") {
		t.Fatalf("detail = %q: existing project must not be reported", got.Detail)
	}

	empty, err := sqlite.Open(filepath.Join(t.TempDir(), "empty.db"))
	if err != nil {
		t.Fatalf("open empty: %v", err)
	}
	defer func() { _ = empty.Close() }()

	ok := doctorRepoCoverage(ctx, empty, root)
	if got := resultByName(ok, "repo-coverage"); got.Status != checkOK {
		t.Fatalf("empty journal status = %q (%s), want ok", got.Status, got.Detail)
	}

	if got := doctorRepoCoverage(ctx, empty, ""); got != nil {
		t.Fatalf("empty projects dir must disable the check (nil), got %+v", got)
	}
}

func TestDoctorVerifyPinsFlagsStalePins(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	root := t.TempDir()

	for repo, gate := range map[string]string{
		"repo-current": "echo current-gate",
		"repo-same":    "echo same-gate",
		"repo-gone":    "", // no .tq-verify; a go.mod below makes auto-detect own the gate
	} {
		if err := os.MkdirAll(filepath.Join(root, repo), 0o755); err != nil {
			t.Fatalf("mkdir %s: %v", repo, err)
		}

		if gate != "" {
			if err := os.WriteFile(filepath.Join(root, repo, ".tq-verify"), []byte(gate+"\n"), 0o600); err != nil {
				t.Fatalf("write .tq-verify in %s: %v", repo, err)
			}
		}
	}

	if err := os.WriteFile(filepath.Join(root, "repo-gone", "go.mod"), []byte("module gone\n"), 0o600); err != nil {
		t.Fatalf("write go.mod: %v", err)
	}

	s, err := sqlite.Open(filepath.Join(t.TempDir(), "pins.db"))
	if err != nil {
		t.Fatalf("open: %v", err)
	}
	defer func() { _ = s.Close() }()

	enqueued := map[string]task.Task{}

	for name, tc := range map[string]struct {
		typ, payload string
	}{
		"staleOverride": {"agent", `{"repo":"repo-current","verify":"echo old-gate"}`},
		"staleFires":    {"agent", `{"repo":"repo-gone","verify":"echo pre-split-gate"}`},
		"current":       {"agent", `{"repo":"repo-same","verify":"echo same-gate"}`},
		"unpinned":      {"agent", `{"repo":"repo-same","prompt":"p"}`},
		"review":        {"review", `{"repo":"repo-same"}`},
	} {
		tt, err := s.Enqueue(ctx, task.New{Type: tc.typ, Payload: []byte(tc.payload)})
		if err != nil {
			t.Fatalf("enqueue %s: %v", name, err)
		}

		enqueued[name] = tt
	}

	got := resultByName(doctorVerifyPins(ctx, s, root), "verify-pins")

	if got.Status != checkWarn {
		t.Fatalf("status = %q (%s), want warn (two stale pins must surface)", got.Status, got.Detail)
	}

	for _, id := range []task.ID{enqueued["staleOverride"].ID, enqueued["staleFires"].ID} {
		if !strings.Contains(got.Detail, string(id)) {
			t.Errorf("detail must name stale task %s: %s", id, got.Detail)
		}
	}

	if strings.Contains(got.Detail, string(enqueued["current"].ID)) {
		t.Errorf("current pin %s must not be reported: %s", enqueued["current"].ID, got.Detail)
	}

	if !strings.Contains(got.Detail, "--reresolve-verify") {
		t.Errorf("detail must point at the reresolve remedy: %s", got.Detail)
	}

	if !strings.Contains(got.Detail, "STALE PIN WILL FIRE") || !strings.Contains(got.Detail, "overrides it") {
		t.Errorf("detail must separate the firing class from the overridden class: %s", got.Detail)
	}

	// With only current + unpinned tasks in the store, the check flips to
	// ok and counts just the pinned ones.
	fresh, err := sqlite.Open(filepath.Join(t.TempDir(), "fresh.db"))
	if err != nil {
		t.Fatalf("open fresh: %v", err)
	}
	defer func() { _ = fresh.Close() }()

	for _, payload := range []string{`{"repo":"repo-same","verify":"echo same-gate"}`, `{"repo":"repo-same","prompt":"p"}`} {
		if _, err := fresh.Enqueue(ctx, task.New{Type: "agent", Payload: []byte(payload)}); err != nil {
			t.Fatalf("enqueue fresh: %v", err)
		}
	}

	ok := resultByName(doctorVerifyPins(ctx, fresh, root), "verify-pins")

	if ok.Status != checkOK {
		t.Fatalf("fresh status = %q (%s), want ok", ok.Status, ok.Detail)
	}

	if !strings.Contains(ok.Detail, "1 pending agent task(s) pin a verify command") {
		t.Errorf("fresh detail must count only pinned tasks: %s", ok.Detail)
	}
}

// TestDoctorOpenSessions pins the stale-open-session visibility check
// (03-28 §f9): a session that began and never closed surfaces as a WARN
// naming the session and pointing at the explicit close path, while a
// clean store is ok. The check is observation-only — its detail must
// carry the never-auto-close disclaimer.
func TestDoctorOpenSessions(t *testing.T) {
	path := doctorTestStore(t)

	s, err := sqlite.Open(path)
	if err != nil {
		t.Fatalf("OpenSQLite: %v", err)
	}
	defer func() { _ = s.Close() }()

	ctx := context.Background()

	// Bare store: no open sessions, check is ok.
	results, err := runDoctor(ctx, doctorOptions{DBPath: path})
	if err != nil {
		t.Fatalf("runDoctor: %v", err)
	}

	if r := resultByName(results, "open-sessions"); r.Status != checkOK {
		t.Errorf("bare store open-sessions = %s (%s), want ok", r.Status, r.Detail)
	}

	if err := session.Begin(ctx, s, "sess-abc", "/tmp/repo", "repo"); err != nil {
		t.Fatalf("session begin: %v", err)
	}

	results, err = runDoctor(ctx, doctorOptions{DBPath: path})
	if err != nil {
		t.Fatalf("runDoctor: %v", err)
	}

	r := resultByName(results, "open-sessions")

	if r.Status != checkWarn {
		t.Errorf("open-sessions = %s (%s), want warn", r.Status, r.Detail)
	}

	if !strings.Contains(r.Detail, "sess-abc") {
		t.Errorf("detail must name the open session: %s", r.Detail)
	}

	if !strings.Contains(r.Detail, "never auto-closes") {
		t.Errorf("detail must carry the never-auto-close disclaimer: %s", r.Detail)
	}

	if err := s.AppendFact(ctx, journal.Fact{TaskID: "session:sess-abc", Type: journal.SessionClosed}); err != nil {
		t.Fatalf("append closed fact: %v", err)
	}

	results, err = runDoctor(ctx, doctorOptions{DBPath: path})
	if err != nil {
		t.Fatalf("runDoctor: %v", err)
	}

	if r := resultByName(results, "open-sessions"); r.Status != checkOK {
		t.Errorf("post-close open-sessions = %s (%s), want ok", r.Status, r.Detail)
	}
}
