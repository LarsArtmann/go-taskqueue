package depsweep

import (
	"context"
	"path/filepath"
	"testing"

	"github.com/larsartmann/go-taskqueue/internal/executor"
	"github.com/larsartmann/go-taskqueue/internal/queue"
	"github.com/larsartmann/go-taskqueue/internal/queue/sqlite"
	"github.com/larsartmann/go-taskqueue/internal/task"
)

func newTestStore(t *testing.T) *sqlite.Store {
	t.Helper()

	s, err := sqlite.Open(filepath.Join(t.TempDir(), "q.db"))
	if err != nil {
		t.Fatalf("sqlite.Open: %v", err)
	}

	t.Cleanup(func() { _ = s.Close() })

	return s
}

func queueFilterType(t *testing.T, taskType string) queue.Filter {
	t.Helper()

	return queue.Filter{Type: &taskType}
}

// stubSource is a PlanSource over a fixed module set.
type stubSource struct{ modules []PlanModule }

func (s stubSource) Plan(context.Context) ([]PlanModule, error) { return s.modules, nil }

func staleModule(key, dir, current, next string) PlanModule {
	return PlanModule{
		Key:            key,
		Dir:            dir,
		StaleBuild:     true,
		CurrentVersion: current,
		NextVersion:    next,
		Wave:           1,
	}
}

func consumerOf(dep PlanModule, key, dir, current, target string) PlanModule {
	dep.OutdatedConsumers = append(dep.OutdatedConsumers, PlanConsumer{
		Key:            key,
		Dir:            dir,
		CurrentVersion: current,
		TargetVersion:  target,
		DependencyKey:  dep.Key,
	})

	return dep
}

func TestBuildWorkStaleReleases(t *testing.T) {
	t.Parallel()

	modules := []PlanModule{
		staleModule("libx", "/p/libx", "v0.1.0", "v0.2.0"), // plan nextVersion wins
		staleModule("liby", "/p/liby", "v0.5.1", ""),       // patch fallback
		{Key: "never", Dir: "/p/never", StaleBuild: true},  // never released
		{Key: "majorlib", Dir: "/p/ml", StaleBuild: true, CurrentVersion: "v1.0.0",
			NextVersion: "v2.0.0", SuggestedBump: "major"},
	}

	specs, skips := BuildWork(modules, SweeperConfig{})

	if len(specs) != 2 {
		t.Fatalf("want 2 release specs, got %d (%+v)", len(specs), specs)
	}

	if specs[0].Repo != "libx" || specs[0].Release.Version != "v0.2.0" {
		t.Errorf("libx must release the planned v0.2.0, got %+v", specs[0])
	}

	if specs[1].Repo != "liby" || specs[1].Release.Version != "v0.5.2" {
		t.Errorf("liby must patch-fallback to v0.5.2, got %+v", specs[1])
	}

	skipReasons := map[string]string{}
	for _, skip := range skips {
		skipReasons[skip.Repo] = skip.Reason
	}

	if _, ok := skipReasons["never"]; !ok {
		t.Error("never-released module must be skipped")
	}

	if _, ok := skipReasons["majorlib"]; !ok {
		t.Error("major release suggestion must be skipped")
	}
}

func TestBuildWorkUnreleasedRequiresOptIn(t *testing.T) {
	t.Parallel()

	modules := []PlanModule{
		{Key: "feats", Dir: "/p/feats", HasUnreleasedWork: true, CurrentVersion: "v1.0.0", NextVersion: "v1.1.0"},
	}

	specs, _ := BuildWork(modules, SweeperConfig{})
	if len(specs) != 0 {
		t.Fatalf("unreleased work must stay out unless IncludeUnreleased, got %d specs", len(specs))
	}

	specs, _ = BuildWork(modules, SweeperConfig{IncludeUnreleased: true})
	if len(specs) != 1 || specs[0].Release.Version != "v1.1.0" {
		t.Fatalf("IncludeUnreleased must mint the planned release, got %+v", specs)
	}
}

func TestBuildWorkConsumerTraps(t *testing.T) {
	t.Parallel()

	lib := staleModule("libx", "/p/libx", "v0.1.0", "v0.2.0")
	lib = consumerOf(lib, "downgrade", "/p/d", "v0.12.0", "v0.11.1-dev")
	lib = consumerOf(lib, "major", "/p/m", "v0.12.0", "v1.0.1")
	lib = consumerOf(lib, "notnewer", "/p/n", "v0.2.0", "v0.2.0")
	lib = consumerOf(lib, "stalepin", "/p/s", "v0.1.0", "v0.2.0")

	specs, skips := BuildWork([]PlanModule{lib}, SweeperConfig{})

	bumps := map[string]bool{}
	for _, spec := range specs {
		if spec.Release == nil {
			bumps[spec.Repo] = true
		}
	}

	if !bumps["stalepin"] {
		t.Error("the legit consumer bump must be minted")
	}

	if bumps["downgrade"] || bumps["major"] || bumps["notnewer"] {
		t.Errorf("trap rows must never mint: %+v", specs)
	}

	if len(skips) < 3 {
		t.Errorf("each trap must be surfaced as a skip, got %d: %+v", len(skips), skips)
	}
}

func TestBuildWorkConsumerGroupingAndDedupStability(t *testing.T) {
	t.Parallel()

	libA := consumerOf(staleModule("liba", "/p/liba", "v1.0.0", "v1.1.0"), "app", "/p/app", "v1.0.0", "v1.1.0")
	libB := consumerOf(staleModule("libb", "/p/libb", "v2.0.0", "v2.1.0"), "app", "/p/app", "v2.0.0", "v2.1.0")

	specs, _ := BuildWork([]PlanModule{libA, libB}, SweeperConfig{})

	var appSpec *WorkSpec

	for i, spec := range specs {
		if spec.Repo == "app" {
			appSpec = &specs[i]
		}
	}

	if appSpec == nil {
		t.Fatal("app must have one grouped bump spec")
	}

	if len(appSpec.Bumps) != 2 {
		t.Fatalf("both deps group into one task, got %+v", appSpec.Bumps)
	}

	if appSpec.Bumps[0].Module != "liba" || appSpec.Bumps[1].Module != "libb" {
		t.Errorf("bumps sort by module: %+v", appSpec.Bumps)
	}

	if len(appSpec.DepRepos) != 2 {
		t.Errorf("app waits on both releases: %+v", appSpec.DepRepos)
	}

	// The bump dedup key must be stable under plan row ORDER changes (the
	// hash runs over the sorted pin set) — reorder never forks a second task.
	again, _ := BuildWork([]PlanModule{libB, libA}, SweeperConfig{})

	for _, spec := range again {
		if spec.Repo == "app" && spec.DedupKey != appSpec.DedupKey {
			t.Fatalf("dedup key must be order-stable: %s vs %s", spec.DedupKey, appSpec.DedupKey)
		}
	}
}

func TestSweepMintsDedupedTasksWithDeps(t *testing.T) {
	t.Parallel()

	s := newTestStore(t)

	libA := consumerOf(staleModule("liba", "/p/liba", "v1.0.0", "v1.1.0"), "app", "/p/app", "v1.0.0", "v1.1.0")
	sweeper := NewSweeper(s, SweeperConfig{Source: stubSource{modules: []PlanModule{libA}}})

	stats, err := sweeper.Sweep(context.Background())
	if err != nil {
		t.Fatalf("sweep: %v", err)
	}

	if stats.Minted != 2 {
		t.Fatalf("liba release + app bump minted, got %d", stats.Minted)
	}

	// The second sweep over the SAME plan is fully deduped.
	stats, err = sweeper.Sweep(context.Background())
	if err != nil {
		t.Fatalf("second sweep: %v", err)
	}

	if stats.Minted != 0 || stats.Known != 2 {
		t.Fatalf("unchanged plan re-mints nothing, got minted=%d known=%d", stats.Minted, stats.Known)
	}

	// The app task waits on the liba release task (plan DAG → task deps).
	tasks, err := s.List(context.Background(), queueFilterType(t, executor.TaskTypeDepBump))
	if err != nil {
		t.Fatalf("list: %v", err)
	}

	var releaseID, appID task.ID

	for _, tk := range tasks {
		switch tk.Project {
		case "liba":
			releaseID = tk.ID
		case "app":
			appID = tk.ID
			if len(tk.Deps) != 1 || tk.Deps[0] != releaseID {
				t.Fatalf("app bump must depend on the liba release task, got deps=%v (release=%s)", tk.Deps, releaseID)
			}
		}
	}

	if releaseID == "" || appID == "" {
		t.Fatalf("both tasks must exist, got release=%s app=%s", releaseID, appID)
	}
}

func TestParsePlanDecodesTheWireContract(t *testing.T) {
	t.Parallel()

	const fixture = `{"modules":[
		{"key":"libx","label":"libx","dir":"/p/libx","staleBuild":true,
		 "currentVersion":"v0.1.0","nextVersion":"v0.2.0","suggestedBump":"patch",
		 "wave":1,"consumerCount":2,
		 "dependencies":[{"key":"liby","label":"liby","wave":1}],
		 "outdatedConsumers":[{"key":"app","label":"app","dir":"/p/app",
			"currentVersion":"v0.1.0","targetVersion":"v0.2.0",
			"dependencyKey":"libx","isMajorBump":false,"dependencyWave":1}]}
	],"totalStale":1}`

	modules, err := ParsePlan([]byte(fixture))
	if err != nil {
		t.Fatalf("parse: %v", err)
	}

	if len(modules) != 1 {
		t.Fatalf("one module, got %d", len(modules))
	}

	m := modules[0]
	if m.Key != "libx" || m.Dir != "/p/libx" || !m.StaleBuild || m.NextVersion != "v0.2.0" {
		t.Fatalf("module fields decoded wrong: %+v", m)
	}

	if len(m.OutdatedConsumers) != 1 || m.OutdatedConsumers[0].DependencyKey != "libx" {
		t.Fatalf("consumer decoded wrong: %+v", m.OutdatedConsumers)
	}

	if len(m.Dependencies) != 1 || m.Dependencies[0].Key != "liby" {
		t.Fatalf("dependencies decoded wrong: %+v", m.Dependencies)
	}
}

func TestCompareSemver(t *testing.T) {
	t.Parallel()

	cases := []struct {
		a, b string
		want int
	}{
		{"v1.2.3", "v1.2.3", 0},
		{"v1.2.4", "v1.2.3", 1},
		{"v1.2.3", "v1.2.4", -1},
		{"v2.0.0", "v1.9.9", 1},
		{"v1.10.0", "v1.9.0", 1},
		{"v0.0.3", "v0.0.2", 1},
		{"v1.2.3", "v1.2.3-rc1", 1}, // bare beats pre-release: dev traps never look newer
		{"v1.2.3-rc1", "v1.2.3", -1},
		{"v1.2.3+build", "v1.2.3", 0}, // build metadata ignores
	}

	for _, tc := range cases {
		if got := compareSemver(tc.a, tc.b); got != tc.want {
			t.Errorf("compareSemver(%s, %s) = %d, want %d", tc.a, tc.b, got, tc.want)
		}
	}
}
