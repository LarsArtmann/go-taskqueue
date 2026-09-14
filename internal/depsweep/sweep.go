package depsweep

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json/jsontext"
	"encoding/json/v2"
	"fmt"
	"log/slog"
	"sort"
	"strings"

	"github.com/larsartmann/go-taskqueue/internal/executor"
	"github.com/larsartmann/go-taskqueue/internal/queue"
	"github.com/larsartmann/go-taskqueue/internal/task"
)

// taskPriority places depbump tasks in the machine band: queue plumbing,
// not user work (same ruling as the prioritize batches).
var taskPriority = queue.MachineMin

// SweeperConfig controls one Sweeper.
type SweeperConfig struct {
	// Source supplies the plan. Required.
	Source PlanSource
	// PushReleases makes release payloads push master+tag after tagging,
	// which is what unblocks consumer bumps (they resolve the tag through
	// the module proxy). Default false: tags stay local and a human pushes
	// — consumers then retry until the tag lands (bounded by the retry
	// ladder; the DLQ is the surface if the push never comes).
	PushReleases bool
	// IncludeUnreleased also releases modules whose ONLY work is
	// unreleased feature commits (the release-suggestions lens). Default
	// false: the sweep converges dependency state; feature releases carry
	// version decisions a human should see. Stale builds always release —
	// their content is dependency pins that are already committed.
	IncludeUnreleased bool
	// Log receives one line per mint/skip. nil logs nothing.
	Log *slog.Logger
}

// WorkSpec is one repo's deterministic work for one plan state.
type WorkSpec struct {
	// Repo is the module key (task project + dedup identity).
	Repo string
	// Dir is the absolute module directory (payload Repo field).
	Dir string
	// Bumps are the exact pins to apply (consumer work).
	Bumps []executor.DepBump
	// Release is the tag to cut after verification (release work).
	Release *executor.DepBumpRelease
	// DepRepos are plan dependency repo keys whose tasks this spec's task
	// must wait for (the plan DAG becomes the queue DAG).
	DepRepos []string
	// DedupKey makes minting idempotent per exact work spec.
	DedupKey string
}

// Skip records one skipped plan row and why (surfaced in stats and logs —
// skips are the human-facing to-do of the sweep).
type Skip struct {
	Repo   string
	Reason string
}

// SweepStats summarizes one sweep pass.
type SweepStats struct {
	// Minted counts fresh tasks enqueued.
	Minted int
	// Known counts mints whose task already existed (dedup hit).
	Known int
	// Skips carries every skipped plan row with its reason.
	Skips []Skip
}

// releaseVersion decides a release spec's tag: the plan's nextVersion when
// present, else a patch bump of the current tag. Major suggestions are
// skipped upstream (module-path migration is code work).
func releaseVersion(m PlanModule) string {
	if m.NextVersion != "" {
		return m.NextVersion
	}

	return patchBump(m.CurrentVersion)
}

func patchBump(current string) string {
	parts := strings.Split(strings.TrimPrefix(current, "v"), ".")

	switch len(parts) {
	case 3:
		patch := 0

		if _, err := fmt.Sscanf(parts[2], "%d", &patch); err == nil {
			return fmt.Sprintf("v%s.%s.%d", parts[0], parts[1], patch+1)
		}
	case 2:
		return fmt.Sprintf("v%s.%s.1", parts[0], parts[1])
	case 1:
		return "v" + parts[0] + ".0.1"
	}

	return ""
}

// BuildWork turns plan modules into per-repo work specs. Rules (ported
// from the verified reference driver, ~/projects/scripts/depgraph-upgrade-sweep.sh):
//
//   - stale builds release (verify+tag): their newer pins are already
//     committed in the working tree — the work is NOT go get.
//   - consumers bump exact pins, grouped one spec per consumer repo.
//   - never-released modules skip (v0.1.0 is a human decision).
//   - major bumps and major release suggestions skip (import-path
//     migration is agent/human work, never scripted).
//   - targets must be stable semver strictly newer than the current pin
//     (the plan carries dev-version and downgrade traps).
func BuildWork(modules []PlanModule, cfg SweeperConfig) ([]WorkSpec, []Skip) {
	specs := make([]WorkSpec, 0)
	skips := make([]Skip, 0)

	skip := func(repo, reason string) {
		skips = append(skips, Skip{Repo: repo, Reason: reason})
	}

	repoDirs := make(map[string]string, len(modules))
	repoHasWork := make(map[string]bool, len(modules))

	for _, m := range modules {
		if m.Dir == "" {
			continue
		}

		repoDirs[m.Key] = m.Dir

		if !m.StaleBuild && !(cfg.IncludeUnreleased && m.HasUnreleasedWork) {
			continue
		}

		repoHasWork[m.Key] = true

		switch {
		case m.CurrentVersion == "":
			skip(m.Key, "never released: pick v0.1.0 manually")
		case m.SuggestedBump == "major":
			skip(m.Key, "major release suggested (v"+strings.TrimPrefix(m.NextVersion, "v")+
				"): module-path migration is human/agent work")
		default:
			version := releaseVersion(m)
			if version == "" || !executor.IsStableSemver(version) {
				skip(m.Key, "cannot derive a stable next version from "+m.CurrentVersion)

				continue
			}

			specs = append(specs, WorkSpec{
				Repo:     m.Key,
				Dir:      m.Dir,
				Release:  &executor.DepBumpRelease{Version: version, Push: cfg.PushReleases},
				DepRepos: depKeys(m),
				DedupKey: fmt.Sprintf("depsweep:%s:release:%s", m.Key, version),
			})
		}
	}

	// Consumer bumps grouped per repo: one task per repo applying all its
	// pending pins (fewer tasks, one verification pass, one commit).
	type consumerWork struct {
		dir  string
		deps map[string]bool
		set  map[string]string // module -> target version
	}

	consumers := make(map[string]*consumerWork)

	for _, m := range modules {
		for _, consumer := range m.OutdatedConsumers {
			if consumer.Dir == "" || consumer.Key == "" {
				continue
			}

			switch {
			case consumer.IsMajorBump:
				skip(consumer.Key, "major bump "+consumer.CurrentVersion+" -> "+consumer.TargetVersion+
					" of "+consumer.DependencyKey+": import-path migration is agent work")
			case !executor.IsStableSemver(consumer.TargetVersion):
				skip(consumer.Key, "unstable target "+consumer.TargetVersion+" of "+consumer.DependencyKey)
			case !isNewerVersion(consumer.TargetVersion, consumer.CurrentVersion):
				skip(consumer.Key, "target "+consumer.TargetVersion+" not newer than pin "+consumer.CurrentVersion)
			default:
				work := consumers[consumer.Key]
				if work == nil {
					work = &consumerWork{dir: consumer.Dir, deps: make(map[string]bool), set: make(map[string]string)}
					consumers[consumer.Key] = work
				}

				work.set[consumer.DependencyKey] = consumer.TargetVersion
				work.deps[m.Key] = true
			}
		}
	}

	for repo, work := range consumers {
		bumps := make([]executor.DepBump, 0, len(work.set))
		modules2 := make([]string, 0, len(work.set))

		for module := range work.set {
			modules2 = append(modules2, module)
		}

		sort.Strings(modules2)

		for _, module := range modules2 {
			bumps = append(bumps, executor.DepBump{Module: module, Version: work.set[module]})
		}

		deps := make([]string, 0, len(work.deps))
		for dep := range work.deps {
			deps = append(deps, dep)
		}

		sort.Strings(deps)

		specs = append(specs, WorkSpec{
			Repo:     repo,
			Dir:      work.dir,
			Bumps:    bumps,
			DepRepos: deps,
			DedupKey: fmt.Sprintf("depsweep:%s:bump:%s", repo, hashWork(modules2, work.set)),
		})
	}

	sort.Slice(specs, func(i, j int) bool {
		a, b := waveOf(specs[i], repoHasWork), waveOf(specs[j], repoHasWork)
		if a != b {
			return a < b
		}

		return specs[i].Repo < specs[j].Repo
	})

	return specs, skips
}

// waveOf sorts releases before their consumers: release specs carry the
// plan wave; consumer specs sort after every release (their dep tasks are
// minted first, so the DAG references exist).
func waveOf(spec WorkSpec, repoHasWork map[string]bool) int {
	if spec.Release != nil && repoHasWork[spec.Repo] {
		return 0
	}

	return 1
}

func depKeys(m PlanModule) []string {
	keys := make([]string, 0, len(m.Dependencies))
	for _, dep := range m.Dependencies {
		if dep.Key != "" {
			keys = append(keys, dep.Key)
		}
	}

	sort.Strings(keys)

	return keys
}

func hashWork(modules []string, targets map[string]string) string {
	parts := make([]string, 0, len(modules))
	for _, module := range modules {
		parts = append(parts, module+"@"+targets[module])
	}

	sum := sha256.Sum256([]byte(strings.Join(parts, ",")))

	return hex.EncodeToString(sum[:8])
}

// isNewerVersion reports whether target is strictly newer than current
// under plain semver ordering; an empty current (unpinned) counts as older
// than everything. Non-semver inputs compare unequal-and-newer only when
// the string ordering agrees with segment ordering — callers should pass
// stable semver (validated upstream).
func isNewerVersion(target, current string) bool {
	if current == "" {
		return true
	}

	c := compareSemver(target, current)

	return c > 0
}

// compareSemver compares two v-prefixed semver strings by numeric
// segments; pre-release suffixes compare as OLDER than the bare version
// (so "-dev" traps never look newer). Returns -1, 0, or 1.
func compareSemver(a, b string) int {
	asegs, apre := splitSemver(a)
	bsegs, bpre := splitSemver(b)

	for i := 0; i < 3; i++ {
		if asegs[i] != bsegs[i] {
			if asegs[i] < bsegs[i] {
				return -1
			}

			return 1
		}
	}

	switch {
	case apre == bpre:
		return 0
	case apre == "":
		return 1 // bare > pre-release
	case bpre == "":
		return -1
	case apre < bpre:
		return -1
	default:
		return 1
	}
}

func splitSemver(v string) ([3]int, string) {
	var segments [3]int

	core := strings.TrimPrefix(v, "v")
	pre := ""

	if idx := strings.IndexByte(core, '-'); idx >= 0 {
		pre = core[idx+1:]
		core = core[:idx]
	}

	if idx := strings.IndexByte(core, '+'); idx >= 0 {
		core = core[:idx]
	}

	for i, part := range strings.Split(core, ".") {
		if i >= 3 {
			break
		}

		// #nosec G602 -- false positive: the guard above caps i at 2 and
		// segments is a [3]int; the index cannot leave bounds.
		_, _ = fmt.Sscanf(part, "%d", &segments[i])
	}

	return segments, pre
}

// Sweeper mints depbump tasks from plan states. Safe for concurrent use.
type Sweeper struct {
	store queue.Store
	cfg   SweeperConfig
}

// NewSweeper returns a sweeper minting into store.
func NewSweeper(store queue.Store, cfg SweeperConfig) *Sweeper {
	return &Sweeper{store: store, cfg: cfg}
}

// Sweep pulls one plan and converges the queue onto it: every work spec
// becomes one depbump task (idempotent via dedup keys), wired to its plan
// dependencies' tasks. Call periodically; dedup makes repeated calls free.
func (s *Sweeper) Sweep(ctx context.Context) (SweepStats, error) {
	stats := SweepStats{}

	modules, err := s.cfg.Source.Plan(ctx)
	if err != nil {
		return stats, err
	}

	specs, skips := BuildWork(modules, s.cfg)
	stats.Skips = skips

	taskIDs := make(map[string]task.ID, len(specs))

	for _, spec := range specs {
		payload, err := payloadFor(spec, s.cfg)
		if err != nil {
			skipped := Skip{Repo: spec.Repo, Reason: "payload: " + err.Error()}
			stats.Skips = append(stats.Skips, skipped)

			continue
		}

		deps := make([]task.ID, 0, len(spec.DepRepos))
		for _, depRepo := range spec.DepRepos {
			if id, ok := taskIDs[depRepo]; ok {
				deps = append(deps, id)
			}
		}

		created, err := queue.New(s.store).Enqueue(ctx, task.New{
			Project:  spec.Repo,
			Type:     executor.TaskTypeDepBump,
			Payload:  payload,
			Deps:     deps,
			Priority: taskPriority,
			DedupKey: spec.DedupKey,
		})
		if err != nil {
			return stats, fmt.Errorf("depsweep: enqueue %s: %w", spec.Repo, err)
		}

		taskIDs[spec.Repo] = created.ID

		if created.Attempts == 0 && created.Status == task.Pending {
			stats.Minted++
		} else {
			stats.Known++
		}

		if s.cfg.Log != nil {
			s.cfg.Log.Info("depsweep: enqueued",
				"repo", spec.Repo,
				"task", created.ID.String(),
				"dedup", spec.DedupKey,
				"deps", len(deps),
				"kind", kindOf(spec),
			)
		}
	}

	for _, skipRow := range skips {
		if s.cfg.Log != nil {
			s.cfg.Log.Warn("depsweep: skipped", "repo", skipRow.Repo, "reason", skipRow.Reason)
		}
	}

	return stats, nil
}

func kindOf(spec WorkSpec) string {
	if spec.Release != nil {
		return "release"
	}

	return "bump"
}

func payloadFor(spec WorkSpec, cfg SweeperConfig) (jsontext.Value, error) {
	payload := executor.DepBumpPayload{
		Repo:     spec.Dir,
		RepoName: spec.Repo,
		Bumps:    spec.Bumps,
		Release:  spec.Release,
		V:        1,
	}

	encoded, err := json.Marshal(payload)
	if err != nil {
		return nil, fmt.Errorf("encode: %w", err)
	}

	return jsontext.Value(encoded), nil
}
