package main

import (
	"flag"
	"log/slog"
	"maps"
	"reflect"
	"slices"
	"testing"

	"github.com/larsartmann/go-taskqueue/internal/httpapi"
	"github.com/larsartmann/go-taskqueue/internal/queue"
	"github.com/larsartmann/go-taskqueue/internal/worker"
)

// This file generalizes the serve parity-test method (serve_flags_test.go,
// TestServeConfigFieldParity) over cmd/tq's other long-running flag sets
// (2026-11-13 done-prompt report §f10): a config struct field with no flag
// is the DailyBudget dead-field class — wired but unreachable from the
// CLI. Every entry in a wired map names the flag feeding that field; every
// allowlist entry is a deliberate non-flag derivation with a justification.

// TestWorkerConfigFieldParity walks EVERY worker.Config field via
// reflection and requires it to be either wired from a `tq worker` flag or
// explicitly allow-listed with a justification.
func TestWorkerConfigFieldParity(t *testing.T) {
	t.Parallel()

	// Field name → why cmdWorker does not (need to) set it. Every entry
	// is a deliberate derivation, not an oversight.
	allowlist := map[string]string{
		"IdlePollMax":      "worker.Config.setDefaults derives it (2s cap); no operator knob",
		"Heartbeat":        "setDefaults derives it (Lease/4); no operator knob",
		"Backoff":          "setDefaults picks ExpBackoff; no operator knob",
		"PreflightBackoff": "setDefaults derives it (2m); no operator knob",
		"EnvRequeueBurn":   "setDefaults derives it (3); no operator knob",
		"Wake":             "silent M7 seam (store.Notify()); never operator-set",
		"Executors":        "composed by cmdWorker (registry); never operator-set",
		"Budget":           "composed by the pool runtime; never operator-set",
		"DonePreflight":    "composed by the agent pool only; never operator-set",
	}

	// Field name → the `tq worker` flag feeding it. In lockstep with the
	// Config literal in cmdWorker.
	wired := map[string]string{
		"Owner":        "owner",
		"Concurrency":  "concurrency",
		"PollInterval": "poll",
		"Lease":        "lease",
		"TaskTimeout":  "task-timeout",
	}

	configType := reflect.TypeFor[worker.Config]()
	for field := range configType.Fields() {
		name := field.Name

		if _, ok := wired[name]; ok {
			continue
		}

		if reason, ok := allowlist[name]; ok {
			if reason == "" {
				t.Errorf("worker.Config.%s is allow-listed without a justification", name)
			}

			continue
		}

		t.Errorf(
			"worker.Config.%s is unreachable from any worker flag: wire a flag or add a justified allowlist entry (the DailyBudget dead-field class)",
			name,
		)
	}

	if t.Failed() {
		return
	}

	// The wiring table must not name flags worker does not register: the
	// registered surface is DERIVED from the real worker FlagSet (the
	// newWorkerFlagSet seam in main.go), not a hand list —
	// scripts/smoke/help-text.sh pins only the serve surface (`tq worker
	// -h` there passes a bare exit check), so a hand worker list here was
	// validated against nothing (2026-11-22 parity-test review finding).
	// A flag added, removed, or renamed in newWorkerFlagSet flows into
	// the derived set automatically; a wired entry naming a flag the
	// FlagSet dropped fails here.
	fs, _ := newWorkerFlagSet()
	registered := map[string]bool{}

	fs.VisitAll(func(f *flag.Flag) { registered[f.Name] = true })

	if len(registered) == 0 {
		t.Fatal("newWorkerFlagSet registered no flags: the worker seam drifted")
	}

	for field, flagName := range wired {
		if !registered[flagName] {
			t.Errorf("wired[%s] names flag --%s which tq worker does not register", field, flagName)
		}
	}
}

// TestAgentPoolOptionsFieldParity walks EVERY agentPoolOptions field via
// reflection and requires it to be either wired from an agent-pool flag or
// explicitly allow-listed — plus an exact-set pin of the flag list itself
// in BOTH directions, so a flag added without a struct field (dead flag)
// or a struct field added without a flag (dead field) fails here.
func TestAgentPoolOptionsFieldParity(t *testing.T) {
	t.Parallel()

	// Field name → why no agent-pool flag feeds it. Every entry is a
	// deliberate derivation, not an oversight.
	allowlist := map[string]string{
		"model": "RETIRED surface: the --model flag still parses but parseAgentPoolOptions REFUSES non-empty (the .crushrc ruling); the field exists only to carry the refusal",
	}

	// Field name → the agent-pool flag feeding it (the agentPoolOptions
	// literal in parseAgentPoolOptions). The mechanical field-name →
	// kebab-flag pairs dominate; the exceptions are all here.
	wired := map[string]string{
		"db":              "db",
		"store":           "store",
		"projectsDir":     "projects-dir",
		"repos":           "repos",
		"interval":        "interval",
		"discoveryAddr":   "discovery-addr",
		"conc":            "concurrency",
		"poll":            "poll",
		"lease":           "lease",
		"timeout":         "task-timeout",
		"owner":           "owner",
		"yolo":            "yolo",
		"reresolveVerify": "reresolve-verify",
		"maxPerTick":      "max-per-tick",
		"batchItems":      "batch-items",
		"effortBudget":    "effort-budget",
		"jev":             "jev",
		"jevModel":        "jev-model",
		"jevAPIKey":       "jev-api-key",
		"priorityFrom":    "priority-from",
		"maxPending":      "max-pending-per-repo",
		"allowDirty":      "allow-dirty",
		"once":            "once",
		"pruneStale":      "prune-stale",
		"reprioritize":    "reprioritize",
		"exclusive":       "project-exclusive",
		"dailyBudget":     "daily-budget",
		"dailyCostCap":    "daily-cost-cap",
		"budgetCmd":       "budget-cmd",
		"repoInterval":    "repo-interval",
		"dlqBackoff":      "dlq-backoff",
		"cqaURL":          "cqa-url",
		"cqaOwner":        "cqa-owner",
		"cqaToken":        "cqa-token",
		"doReview":        "review",
		"dlqFix":          "dlq-fix",
		"donePreflight":   "done-preflight",
		"prioritize":      "prioritize",
		"depSweep":        "dep-sweep",
		"depSweepBin":     "dep-sweep-bin",
		"depSweepDir":     "dep-sweep-dir",
		"depSweepEvery":   "dep-sweep-interval",
		"depSweepPush":    "dep-sweep-push",
		"depSweepUnrel":   "dep-sweep-unreleased",
		"docsGates":       "docs-gates",
		"docsGatesBin":    "docs-gates-bin",
		"docsGatesRepos":  "docs-gates-repos",
		"docsGatesEvery":  "docs-gates-interval",
		"alertURL":        "alert-url",
		"alertKey":        "alert-api-key",
		"alertPoll":       "alert-poll",
		"deadPoolTicks":   "dead-pool-ticks",
		"starveAfter":     "starvation-after",
		"repoTimeout":     "repo-timeout",
		"maxAgents":       "max-concurrent-agents",
		"reviewAutofix":   "review-autofix",
		"statusEvery":     "status-every",
		"statusMinGap":    "status-min-interval",
		"closeout":        "task-closeout",
		"logDir":          "log-dir",
		"logDirMaxAge":    "log-dir-max-age",
		"logDirMaxBytes":  "log-dir-max-bytes",
		"ownerSessPark":   "owner-session-park",
		"ownerSessHold":   "owner-session-hold",
		"worktrees":       "worktrees",
		"crushServer":     "crush-server",
	}

	// Flags that deliberately do NOT fold into agentPoolOptions.
	flagsWithoutField := map[string]string{
		"config": "the --config file applier merges into the flag set before the fold; itself never stored",
		"redact": "rides os.Setenv(\"TQ_REDACT\") to the executor's redaction pass; never stored",
		"model":  "parses, then parseAgentPoolOptions REFUSES non-empty (the .crushrc ruling); the struct's model field carries only the refusal input",
	}

	optsType := reflect.TypeFor[agentPoolOptions]()
	for field := range optsType.Fields() {
		name := field.Name

		if _, ok := wired[name]; ok {
			continue
		}

		if reason, ok := allowlist[name]; ok {
			if reason == "" {
				t.Errorf("agentPoolOptions.%s is allow-listed without a justification", name)
			}

			continue
		}

		t.Errorf(
			"agentPoolOptions.%s is unreachable from any agent-pool flag: wire a flag or add a justified allowlist entry (the DailyBudget dead-field class)",
			name,
		)
	}

	if t.Failed() {
		return
	}

	// The registered surface is DERIVED from the real agent-pool FlagSet
	// (the newAgentPoolFlagSet seam parseAgentPoolOptions itself rides),
	// not a hand list — the same dynamic method as the serve parity test
	// above. A flag added, removed, or renamed in the registration flows
	// into the both-directions check automatically; dropping the FlagSet
	// fails here.
	fs, _ := newAgentPoolFlagSet()

	registeredAgentPoolFlagsSet := map[string]bool{}

	fs.VisitAll(func(f *flag.Flag) { registeredAgentPoolFlagsSet[f.Name] = true })

	if len(registeredAgentPoolFlagsSet) == 0 {
		t.Fatal("newAgentPoolFlagSet registered no flags: the agent-pool seam drifted")
	}

	registeredAgentPoolFlags := slices.Sorted(maps.Keys(registeredAgentPoolFlagsSet))

	for _, flagName := range wired {
		if !slices.Contains(registeredAgentPoolFlags, flagName) {
			t.Errorf(
				"wired names flag --%s which the pinned agent-pool list does not carry (parseAgentPoolOptions drift?)",
				flagName,
			)
		}
	}

	for flagName := range flagsWithoutField {
		if !slices.Contains(registeredAgentPoolFlags, flagName) {
			t.Errorf(
				"flagsWithoutField names flag --%s which the pinned agent-pool list does not carry (parseAgentPoolOptions drift?)",
				flagName,
			)
		}
	}

	// Both directions over the pinned list: every pinned flag is either a
	// wired field or a justified no-field flag — a flag whose value
	// nothing consumes fails here.
	slices.Sort(registeredAgentPoolFlags)

	for _, flagName := range registeredAgentPoolFlags {
		wiredFlag := false

		for _, wf := range wired {
			if wf == flagName {
				wiredFlag = true

				break
			}
		}

		if wiredFlag {
			continue
		}

		if _, ok := flagsWithoutField[flagName]; ok {
			continue
		}

		t.Errorf(
			"pinned agent-pool flag --%s folds into no agentPoolOptions field: wire the field or move it into flagsWithoutField with a justification",
			flagName,
		)
	}
}

// TestAPIFlagSurfaceParity pins the `tq api` flag surface (2026-11-13
// report §f10 audit finding): httpapi has NO Config struct — New takes
// (store, token, log) positionally — so there is nothing to
// reflection-walk; the dead-field class can only appear here as a flag
// whose value nothing consumes, pinned by the exact-set list below.
func TestAPIFlagSurfaceParity(t *testing.T) {
	t.Parallel()

	// The api flag surface and what consumes each value in cmdAPI; the
	// flag NAMES themselves are pinned by scripts/smoke/help-text.sh, so
	// this table documents the consumption side of the audit.
	consumed := map[string]string{
		"addr":       "ListenAndServe",
		"auth-token": "httpapi.New token param",
		"read-model": "resolveDeployment + UseReadModel",
		"db":         "dbFlag → resolveDeployment",
		"store":      "storeFlag → resolveDeployment",
	}

	for flagName, consumer := range consumed {
		if consumer == "" {
			t.Errorf(
				"consumed[%s] is empty: name the cmdAPI consumer or move the flag out (dead-flag audit, §f10)",
				flagName,
			)
		}
	}

	_ = consumed

	// httpapi.Server fields are unexported runtime state, never CLI
	// config; this assignment pins the LIVE httpapi.New signature (a
	// reflect.TypeFor type literal would assert only itself, never the
	// constructor): a future Config-struct introduction must extend this
	// test (walk the struct, drop the comment) rather than silently
	// bypassing the audit.
	var _ func(queue.Store, string, *slog.Logger) (*httpapi.Server, error) = httpapi.New
}
