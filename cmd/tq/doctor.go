package main

import (
	"bytes"
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/larsartmann/go-taskqueue/internal/executor"
	"github.com/larsartmann/go-taskqueue/internal/harvest"
	"github.com/larsartmann/go-taskqueue/internal/journal"
	"github.com/larsartmann/go-taskqueue/internal/queue"
	"github.com/larsartmann/go-taskqueue/internal/queue/sqlite"
	"github.com/larsartmann/go-taskqueue/internal/review"
	"github.com/larsartmann/go-taskqueue/internal/session"
	"github.com/larsartmann/go-taskqueue/internal/status"
	"github.com/larsartmann/go-taskqueue/internal/task"
	_ "modernc.org/sqlite"
)

// tq doctor answers "why is nothing happening?" in one command: database
// health, queue mix, worker liveness, budget, and the pool's environment
// (crush binary, repo autonomy files). Exit codes: 0 = healthy or warnings,
// 1 = at least one failing check (doctor still prints every result first).

// Check statuses. WARN is advisory (something may be wrong, or simply idle);
// FAIL means a real malfunction the operator must act on.
const (
	checkOK   = "ok"
	checkWarn = "warn"
	checkFail = "fail"
)

// errDoctorFailed is the static exit-1 error: `tq doctor` found at least
// one FAIL check (the caller still printed every result first).
var errDoctorFailed = errors.New("failing checks found")

// checkResult is one doctor finding.
type checkResult struct {
	Name   string `json:"name"`
	Status string `json:"status"`
	Detail string `json:"detail"`
	// Items carries the per-subject findings behind Detail (additive, may
	// be empty): structured rows so tooling can act on stale pins without
	// parsing prose (id, reasons, will-fire live as separate strings today;
	// a fuller schema is a versioned --json change, not bundled here).
	Items []string `json:"items,omitempty"`
}

// doctorOptions controls which checks run.
type doctorOptions struct {
	DBPath      string
	DailyBudget int    // 0: skip the budget check
	Repos       string // comma-separated repo paths: enables autonomy checks
	ProjectsDir string // root for bare names in Repos (mirrors harvest/audit)
	AgentBin    string // agent binary override (defaults to crush)
	// MarkOrphans, when set, appends task.orphaned facts for stranded
	// Running tasks (expired lease, no reclaim) — the only write `tq
	// doctor` can perform, and only on explicit request.
	MarkOrphans bool
	// Hygiene, when set (--hygiene), audits PENDING agent tasks'
	// enqueue-time verify pins against each repo's current gate — the
	// stale-payload audit class (09-39 f2), encoded as a check.
	Hygiene bool
	// ServiceUnit, when set (--service-unit NAME), additionally
	// diagnoses the systemd unit's OWN environment (its PATH and
	// GOEXPERIMENT) instead of only the invoking shell's — the actual
	// failure surface of the 20h dead pool (05-38 report §f/e1).
	ServiceUnit string
}

// doctorHeartbeatWindow is how long ago a task.heartbeat fact still counts
// as "a worker is alive" evidence.
const doctorHeartbeatWindow = 10 * time.Minute

// doctorCrushMinVersion is the agent binary's version floor: v0.94.1 made
// `--reasoning-effort` accept the flash levels (owner ruling 2026-09-14:
// glm-5.3-flash is low|high|xhigh and the pool ALWAYS wants xhigh), so an
// older binary silently cannot honor the managed block's effort pin.
const doctorCrushMinVersion = "0.94.1"

// crushVersionProbeTimeout bounds one `--version` call: a hung binary must
// not hang the doctor.
const crushVersionProbeTimeout = 15 * time.Second

// doctorProbeCrushVersion runs `<bin> --version` and returns its trimmed
// output. Var so tests stub it hermetically.
var doctorProbeCrushVersion = func(ctx context.Context, bin string) (string, error) {
	ctx, cancel := context.WithTimeout(ctx, crushVersionProbeTimeout)
	defer func() { cancel() }()

	out, err := exec.CommandContext(ctx, bin, "--version").Output()

	return strings.TrimSpace(string(out)), err
}

// parseCrushVersion extracts a numeric version ("0.96.1") from a
// `crush --version` line like "crush version v0.96.1". Returns ok=false
// when no dotted-number token is present.
func parseCrushVersion(out string) (string, bool) {
	for _, field := range strings.Fields(out) {
		v := strings.TrimPrefix(field, "v")
		if v == "" || v[0] < '0' || v[0] > '9' {
			continue
		}

		numeric := true

		for _, part := range strings.Split(v, ".") {
			if part == "" {
				numeric = false

				break
			}

			for _, r := range part {
				if r < '0' || r > '9' {
					numeric = false

					break
				}
			}

			if !numeric {
				break
			}
		}

		if numeric {
			return v, true
		}
	}

	return "", false
}

// compareCrushVersions compares dotted numeric versions component-wise
// (missing components count as 0): -1 when a < b, 0 when equal, 1 when a > b.
func compareCrushVersions(a, b string) int {
	as := strings.Split(a, ".")
	bs := strings.Split(b, ".")

	for i := 0; i < len(as) || i < len(bs); i++ {
		av, bv := 0, 0

		if i < len(as) {
			av, _ = strconv.Atoi(as[i])
		}

		if i < len(bs) {
			bv, _ = strconv.Atoi(bs[i])
		}

		switch {
		case av < bv:
			return -1
		case av > bv:
			return 1
		}
	}

	return 0
}

// doctorCrushVersionCheck gates the agent binary against
// doctorCrushMinVersion. WARN (never FAIL): an old binary degrades effort
// pinning but does not corrupt the queue.
func doctorCrushVersionCheck(ctx context.Context, bin string) checkResult {
	const name = "crush-version"

	out, err := doctorProbeCrushVersion(ctx, bin)
	if err != nil {
		return checkResult{
			Name: name, Status: checkWarn,
			Detail: bin + " --version failed (" + err.Error() + ") — version floor check skipped",
		}
	}

	v, ok := parseCrushVersion(out)
	if !ok {
		return checkResult{
			Name: name, Status: checkWarn,
			Detail: "unparseable " + bin + " --version output " + strconv.Quote(out) +
				" — version floor check skipped",
		}
	}

	if compareCrushVersions(v, doctorCrushMinVersion) < 0 {
		return checkResult{
			Name: name, Status: checkWarn,
			Detail: "crush " + v + " is below the " + doctorCrushMinVersion +
				" floor — --reasoning-effort xhigh pinning unreliable; upgrade the binary",
		}
	}

	return checkResult{
		Name: name, Status: checkOK,
		Detail: "crush " + v + " (floor " + doctorCrushMinVersion + ")",
	}
}

// runDoctor executes every check against the database and environment,
// returning results ordered worst-last. The returned error is non-nil only
// when the database cannot be inspected at all.
func runDoctor(ctx context.Context, opts doctorOptions) ([]checkResult, error) {
	var results []checkResult

	store, err := sqlite.Open(opts.DBPath)
	if err != nil {
		return nil, fmt.Errorf("open db: %w", err)
	}
	defer func() { _ = store.Close() }()

	results = append(results, doctorSQLiteChecks(ctx, opts.DBPath)...)
	results = append(results, doctorQueueMix(ctx, store)...)
	results = append(results, doctorRepoCoverage(ctx, store, opts.ProjectsDir)...)

	if opts.Hygiene {
		results = append(results, doctorVerifyPins(ctx, store, opts.ProjectsDir)...)
	}

	results = append(results, doctorWorkerLiveness(ctx, store)...)
	results = append(results, doctorWatermarkLiveness(ctx, store)...)
	results = append(results, doctorOpenSessions(ctx, store)...)
	results = append(results, doctorBudget(ctx, store, opts.DailyBudget)...)
	results = append(results, doctorEnvironment(ctx, opts)...)

	if opts.ServiceUnit != "" {
		results = append(results, doctorServiceContext(ctx, opts)...)
	}

	if opts.MarkOrphans {
		results = append(results, doctorMarkOrphans(ctx, store)...)
	}

	return results, nil
}

// doctorSQLiteChecks verifies the database file itself: openable, intact,
// and in WAL mode (the concurrency contract MaxOpenConns(1)+WAL relies on).
func doctorSQLiteChecks(ctx context.Context, path string) []checkResult {
	var results []checkResult

	db, err := sql.Open("sqlite", path)
	if err != nil {
		return append(results, checkResult{Name: "db", Status: checkFail, Detail: err.Error()})
	}
	defer func() { _ = db.Close() }()

	var quickCheck string
	if err := db.QueryRowContext(ctx, `PRAGMA quick_check(1)`).Scan(&quickCheck); err != nil {
		return append(results, checkResult{Name: "db", Status: checkFail, Detail: "quick_check: " + err.Error()})
	}

	if quickCheck != "ok" {
		results = append(results, checkResult{Name: "db", Status: checkFail, Detail: "integrity: " + quickCheck})
	} else {
		results = append(results, checkResult{Name: "db", Status: checkOK, Detail: "integrity ok"})
	}

	var mode string
	if err := db.QueryRowContext(ctx, `PRAGMA journal_mode`).Scan(&mode); err != nil {
		results = append(results, checkResult{Name: "wal", Status: checkWarn, Detail: "journal_mode: " + err.Error()})
	} else if !strings.EqualFold(mode, "wal") {
		results = append(results, checkResult{
			Name: "wal", Status: checkWarn,
			Detail: "journal_mode is " + mode + " (expected wal; open the db with tq once to set it)",
		})
	} else {
		results = append(results, checkResult{Name: "wal", Status: checkOK, Detail: "wal enabled"})
	}

	return results
}

// doctorQueueMix summarizes the queue and flags stuck work.
func doctorQueueMix(ctx context.Context, store queue.Store) []checkResult {
	counts, err := store.StatusCounts(ctx)
	if err != nil {
		return []checkResult{{Name: "queue", Status: checkFail, Detail: err.Error()}}
	}

	running := counts[task.Running]
	stuck := queue.CountStuckRunning(ctx, store, time.Now())

	detail := fmt.Sprintf("pending=%d running=%d completed=%d dead=%d cancelled=%d",
		counts[task.Pending], running, counts[task.Completed], counts[task.Dead], counts[task.Cancelled])

	status := checkOK
	if stuck > 0 {
		status = checkWarn
		detail += fmt.Sprintf(
			"; %d running task(s) have an EXPIRED lease and no worker reclaimed them (tq doctor --mark-orphans records them)",
			stuck,
		)
	}

	return []checkResult{
		{Name: "queue", Status: status, Detail: detail},
		{
			Name: "dlq", Status: doctorCountStatus(counts[task.Dead]),
			Detail: fmt.Sprintf("%d dead-lettered task(s)", counts[task.Dead]),
		},
		doctorParked(ctx, store),
		doctorDLQRepair(ctx, store),
	}
}

// doctorDLQRepair guards P3's silence (M12): dead letters accumulating
// while not ONE autopsy was ever minted means the pool runs without the
// repair loop (--dlq-fix) and nothing disposes the landfill — 676 tasks,
// 317 dead, zero autopsies before the 2026-10-01 diagnosis. The default
// stays opt-in (autopsies are paid second opinions; the ruling lives in
// docs/planning/2026-10-01_dlqfix-default-decision.md); this check is the
// cannot-recur-silently half.
func doctorDLQRepair(ctx context.Context, store queue.Store) checkResult {
	const name = "dlq-repair"

	counts, err := store.StatusCounts(ctx)
	if err != nil {
		return checkResult{Name: name, Status: checkWarn, Detail: "count: " + err.Error()}
	}

	if counts[task.Dead] == 0 {
		return checkResult{Name: name, Status: checkOK, Detail: "no dead letters"}
	}

	autopsyType := executor.TaskTypeDLQFix
	autopsies, err := store.CountTasks(ctx, queue.Filter{Type: &autopsyType})
	if err != nil {
		return checkResult{Name: name, Status: checkWarn, Detail: "count autopsies: " + err.Error()}
	}

	if autopsies == 0 {
		return checkResult{
			Name:   name,
			Status: checkWarn,
			Detail: fmt.Sprintf(
				"%d dead-lettered task(s) and zero autopsies ever minted (the repair loop looks disabled: agent-pool --dlq-fix, or dlq-fix=true in the pool config)",
				counts[task.Dead],
			),
		}
	}

	return checkResult{
		Name:   name,
		Status: checkOK,
		Detail: fmt.Sprintf("repair loop alive (%d autopsy task(s) minted)", autopsies),
	}
}

// doctorParked surfaces rate-limit-parked tasks (pending, not_before in the
// future): an idle pool with parked tasks is WAITING on the provider, not
// broken — the 13:29 incident's "is it dead or just limited?" question
// answered in one line. With tasks parked, the earliest not_before is
// named so the operator knows WHEN to look again (16-00 report f34).
func doctorParked(ctx context.Context, store queue.Store) checkResult {
	parked := true

	n, err := store.CountTasks(ctx, queue.Filter{Parked: &parked})
	if err != nil {
		return checkResult{Name: "parked", Status: checkWarn, Detail: "count: " + err.Error()}
	}

	if n == 0 {
		return checkResult{Name: "parked", Status: checkOK, Detail: "no rate-limit-parked tasks"}
	}

	detail := fmt.Sprintf(
		"%d task(s) parked by a provider rate limit — WAITING, not broken; see `tq tasks --parked`",
		n,
	)

	if tasks, err := store.List(ctx, queue.Filter{Parked: &parked}); err == nil {
		earliest := time.Time{}

		for _, tk := range tasks {
			if tk.NotBefore.IsZero() {
				continue
			}

			if earliest.IsZero() || tk.NotBefore.Before(earliest) {
				earliest = tk.NotBefore
			}
		}

		if !earliest.IsZero() {
			layout := "15:04"
			if earliest.Local().Day() != time.Now().Day() {
				layout = "Jan 2 15:04"
			}

			detail += fmt.Sprintf("; earliest release %s", earliest.Local().Format(layout))
		}
	}

	return checkResult{Name: "parked", Status: checkWarn, Detail: detail}
}

// doctorMarkOrphans records stranded Running tasks in the journal
// (--mark-orphans): one task.orphaned fact each, idempotently. The check
// result reports how many were newly marked.
func doctorMarkOrphans(ctx context.Context, store queue.Store) []checkResult {
	marked, err := store.MarkOrphaned(ctx, time.Now())
	if err != nil {
		return []checkResult{{Name: "mark-orphans", Status: checkFail, Detail: err.Error()}}
	}

	detail := "no stranded tasks marked"
	if marked > 0 {
		detail = fmt.Sprintf(
			"marked %d stranded task(s) as task.orphaned (still Running; the next reclaiming worker picks them up)",
			marked,
		)
	}

	return []checkResult{{Name: "mark-orphans", Status: checkOK, Detail: detail}}
}

// doctorRepoCoverage checks the PENDING-forever class: PENDING tasks whose
// project resolves to a missing repo directory under the projects root. Pool
// coverage (--repos lists) is process state and not journaled, so the one
// durable signal is directory existence: a PENDING task for a project whose
// directory is gone (moved/renamed/deleted) can never be claimed by a
// harvest-driven pool — the tasks sit until an operator cancels them.
func doctorRepoCoverage(ctx context.Context, store queue.Store, projectsDir string) []checkResult {
	if projectsDir == "" {
		return nil
	}

	pending := task.Pending
	tasks, err := store.List(ctx, queue.Filter{Status: &pending})
	if err != nil {
		return []checkResult{{Name: "repo-coverage", Status: checkFail, Detail: "list pending: " + err.Error()}}
	}

	counts := map[string]int{}
	for _, t := range tasks {
		if t.Project != "" {
			counts[t.Project]++
		}
	}

	var missing []string
	for p, n := range counts {
		dir := p
		if !filepath.IsAbs(dir) {
			dir = filepath.Join(projectsDir, p)
		}

		if _, err := os.Stat(dir); err == nil {
			continue
		}

		missing = append(missing, fmt.Sprintf("%s (%d pending task(s); no dir at %s)", p, n, dir))
	}

	if len(missing) == 0 {
		return []checkResult{{
			Name:   "repo-coverage",
			Status: checkOK,
			Detail: fmt.Sprintf("every project with PENDING tasks resolves under %s", projectsDir),
		}}
	}

	sort.Strings(missing)

	return []checkResult{{
		Name:   "repo-coverage",
		Status: checkWarn,
		Detail: fmt.Sprintf(
			"%d project(s) with PENDING tasks have no repo directory — no pool can ever claim them (cancel via tq cancel): %s",
			len(missing),
			strings.Join(missing, "; "),
		),
	}}
}

// doctorVerifyPins is the stale-payload hygiene check (09-39 report f2/§e3):
// every PENDING agent task carries an enqueue-time verify pin, and a repo
// whose verify contract changed after enqueue would fire that stale command
// at claim time (the f46 incident class). Each pin is compared against the
// repo's CURRENT gate — the .tq-verify file if present, else today's
// auto-detection (executor.DetectVerify), the same ladder runVerify
// resolves at run time. Pins are ALSO matched against the KNOWN-STALE
// minted forms (executor.StaleVerifyReasons: the root-module-only gate,
// the exit-swallowing -execdir walk, and pre-env-self-contained commands
// without GOEXPERIMENT=jsonv2) — a repo-independent content check, so a
// stale pin gates even where the repo directory is absent (the f46-style
// audit as a repeatable check, not an investigation). Warn, never fail:
// a pin that a current .tq-verify overrides is latent, not firing, and
// cancelling queued work is an operator decision (tq cancel).
func doctorVerifyPins(ctx context.Context, store queue.Store, projectsDir string) []checkResult {
	pending := task.Pending
	tasks, err := store.List(ctx, queue.Filter{Status: &pending})
	if err != nil {
		return []checkResult{{Name: "verify-pins", Status: checkFail, Detail: "list pending: " + err.Error()}}
	}

	var stale []string
	pinned := 0

	for _, t := range tasks {
		if t.Type != executor.TaskTypeAgent {
			continue
		}

		var p executor.AgentPayload
		if err := json.Unmarshal(t.Payload, &p); err != nil {
			stale = append(stale, fmt.Sprintf("%s: payload does not parse as an agent payload (%v)", t.ID, err))

			continue
		}

		if p.Verify == "" {
			continue // nothing pinned (auto-detect owns the gate)
		}

		pinned++

		var (
			patternStale string
			repoVerdict  string
		)

		// Both verdicts are computed for EVERY pin (a pin can be
		// pattern-stale AND diverge from the repo's current gate), then
		// merged with the more actionable repo verdict first.
		if reasons := executor.StaleVerifyReasons(p.Verify); len(reasons) > 0 {
			patternStale = fmt.Sprintf(
				"known-stale verify pin — %s: %q",
				strings.Join(reasons, "; "), excerpt(p.Verify))
		}

		if p.Repo != "" {
			repoDir := p.Repo
			if !filepath.IsAbs(repoDir) {
				repoDir = filepath.Join(projectsDir, repoDir)
			}

			if _, err := os.Stat(repoDir); err == nil {
				current := executor.ReadTQVerify(repoDir)

				switch {
				case current == p.Verify:
					// pin matches the repo's current gate
				case current != "":
					repoVerdict = fmt.Sprintf(
						"stale pin %q — .tq-verify currently overrides it, but it fires again if the file is deleted",
						excerpt(p.Verify))
				case executor.DetectVerify(repoDir) != p.Verify:
					repoVerdict = fmt.Sprintf(
						"STALE PIN WILL FIRE — no .tq-verify and today's auto-detected gate differs: pin %q vs detect %q",
						excerpt(p.Verify),
						excerpt(executor.DetectVerify(repoDir)),
					)
				}
			}
		}

		switch {
		case repoVerdict != "":
			// The will-fire verdict is the most actionable signal; a pin
			// that is BOTH pattern-stale and will-fire carries both halves.
			if patternStale != "" {
				repoVerdict += " (also " + patternStale + ")"
			}

			stale = append(stale, fmt.Sprintf("%s (%s): %s", t.ID, p.Repo, repoVerdict))
		case patternStale != "":
			stale = append(stale, fmt.Sprintf("%s: %s", t.ID, patternStale))
		}
	}

	if len(stale) == 0 {
		detail := fmt.Sprintf(
			"%d pending agent task(s) pin a verify command, all matching the repos' current gates and no known-stale pattern",
			pinned,
		)
		if pinned == 0 {
			detail = "no pending agent task pins a verify command"
		}

		return []checkResult{{Name: "verify-pins", Status: checkOK, Detail: detail}}
	}

	// Small finding sets stay verbatim in Detail (grep-able, test-pinned);
	// larger ones summarize to the first 3 and point at the companion
	// surface for the full list. Items always carries the full rows.
	detail := fmt.Sprintf(
		"%d of %d pinned task(s) carry stale verify pins — --reresolve-verify (agent-pool / worker --agents) ignores enqueue-time pins entirely",
		len(stale), pinned,
	)
	if len(stale) <= 3 {
		detail = fmt.Sprintf(
			"%d of %d pinned task(s) carry stale verify pins: %s — --reresolve-verify (agent-pool / worker --agents) ignores enqueue-time pins entirely",
			len(stale), pinned, strings.Join(stale, "; "),
		)
	}

	return []checkResult{{
		Name:   "verify-pins",
		Status: checkWarn,
		Detail: detail,
		Items:  append([]string{staleSummary(stale, pinned)}, stale...),
	}}
}

// staleSummary renders the verify-pin findings for one line: the first 3
// verbatim, then a count + the companion surface for the full list.
func staleSummary(stale []string, pinned int) string {
	const maxVerbatim = 3

	head := stale
	if len(head) > maxVerbatim {
		head = head[:maxVerbatim]
	}

	suffix := ""
	if len(stale) > maxVerbatim {
		suffix = fmt.Sprintf(" … (+%d more; full list: tq tasks --status pending --type agent --verify-contains '<pin substring>')", len(stale)-maxVerbatim)
	}

	return fmt.Sprintf("%d of %d pinned: %s%s", len(stale), pinned, strings.Join(head, "; "), suffix)
}

// excerpt shortens a verify command for one-line doctor output: the full
// minted Go gates are hundreds of characters and the detail only needs to
// identify the command.
func excerpt(s string) string {
	if len(s) <= 60 {
		return s
	}

	return s[:60] + "…"
}

// doctorCountStatus maps DLQ size to severity: 0-2 is normal operation,
// more deserves a look.
func doctorCountStatus(dead int) string {
	if dead > 2 {
		return checkWarn
	}

	return checkOK
}

// doctorWatermarkLiveness checks the journal-consumer cursors (the review
// and status sweepers): a cursor lagging the journal head means its sweeper
// is not running — agent completions pile up unreviewed / unreported. A
// missing cursor just means that sweeper never ran here (idle, not sick).
func doctorWatermarkLiveness(ctx context.Context, store queue.Store) []checkResult {
	head, err := store.HeadSeq(ctx)
	if err != nil {
		return []checkResult{{Name: "watermarks", Status: checkFail, Detail: "read journal head: " + err.Error()}}
	}

	var results []checkResult

	for _, chk := range []struct {
		name     string
		consumer string
	}{
		{"review-sweeper", review.ConsumerKey},
		{"status-sweeper", status.ConsumerKey},
	} {
		seq, exists, err := store.Watermark(ctx, chk.consumer)
		if err != nil {
			results = append(results, checkResult{Name: chk.name, Status: checkFail, Detail: err.Error()})

			continue
		}

		if !exists {
			results = append(
				results,
				checkResult{Name: chk.name, Status: checkOK, Detail: "no cursor (sweeper never ran here)"},
			)

			continue
		}

		if lag := head - seq; lag > 0 {
			results = append(results, checkResult{
				Name:   chk.name,
				Status: checkWarn,
				Detail: fmt.Sprintf(
					"%d fact(s) behind the journal head — the sweeper is not running (inspect/rewind: tq watermarks show)",
					lag,
				),
			})
		} else {
			results = append(
				results,
				checkResult{Name: chk.name, Status: checkOK, Detail: fmt.Sprintf("at head (#%d)", seq)},
			)
		}
	}

	return results
}

// doctorWorkerLiveness looks for recent heartbeats as worker-alive
// evidence. No heartbeats with an empty queue is just idle; no heartbeats
// with pending or stuck work is the "worker is down" signature.
func doctorWorkerLiveness(ctx context.Context, store queue.Store) []checkResult {
	beats, err := store.CountFacts(ctx, journal.Heartbeat, time.Now().Add(-doctorHeartbeatWindow))
	if err != nil {
		return []checkResult{{Name: "worker", Status: checkWarn, Detail: err.Error()}}
	}

	counts, err := store.StatusCounts(ctx)
	if err != nil {
		return []checkResult{{Name: "worker", Status: checkWarn, Detail: err.Error()}}
	}

	idle := counts[task.Pending] == 0 && counts[task.Running] == 0

	if beats > 0 {
		return []checkResult{{
			Name: "worker", Status: checkOK,
			Detail: fmt.Sprintf("%d heartbeat fact(s) in the last %s", beats, doctorHeartbeatWindow),
		}}
	}

	if idle {
		return []checkResult{{
			Name: "worker", Status: checkOK,
			Detail: "no recent heartbeats, but the queue is empty (idle, not stuck)",
		}}
	}

	return []checkResult{
		{
			Name:   "worker",
			Status: checkFail,
			Detail: fmt.Sprintf(
				"no heartbeats in the last %s while %d pending / %d running tasks wait — is a worker running?",
				doctorHeartbeatWindow,
				counts[task.Pending],
				counts[task.Running],
			),
		},
	}
}

// doctorOpenSessions surfaces interactive sessions that began and never
// closed (03-28 §f9): an open session means its close-out (review + status)
// never ran. Doctor only REPORTS them — closing enqueues budget-spending
// tasks, so it never happens without an explicit operator ruling
// (tq session close / sweep); the check says so in its detail.
func doctorOpenSessions(ctx context.Context, store *sqlite.Store) []checkResult {
	open, err := session.List(ctx, store)
	if err != nil {
		return []checkResult{{Name: "open-sessions", Status: checkFail, Detail: err.Error()}}
	}

	if len(open) == 0 {
		return []checkResult{{Name: "open-sessions", Status: checkOK, Detail: "no open sessions"}}
	}

	parts := make([]string, 0, len(open))

	for _, s := range open {
		repo := s.Repo
		if repo == "" {
			repo = "-"
		}

		parts = append(parts, fmt.Sprintf("%s (opened %s, repo %s)", s.ID, s.OpenedAt.Format("2006-01-02 15:04"), repo))
	}

	return []checkResult{{
		Name:   "open-sessions",
		Status: checkWarn,
		Detail: fmt.Sprintf(
			"%d session(s) began and never closed — their close-out never ran; close explicitly with tq session close (doctor never auto-closes): %s",
			len(open),
			strings.Join(parts, "; "),
		),
	}}
}

// doctorBudget compares today's enqueues against the operator's cap.
func doctorBudget(ctx context.Context, store queue.Store, dailyBudget int) []checkResult {
	if dailyBudget <= 0 {
		return nil
	}

	now := time.Now()
	today := time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, now.Location())

	spent, err := store.CountFacts(ctx, journal.Enqueued, today)
	if err != nil {
		return []checkResult{{Name: "budget", Status: checkWarn, Detail: err.Error()}}
	}

	status := checkOK

	switch {
	case int(spent) >= dailyBudget:
		status = checkWarn
	case dailyBudget > 0 && int(spent)*4 >= dailyBudget*3:
		status = checkWarn
	}

	return []checkResult{{
		Name: "budget", Status: status,
		Detail: fmt.Sprintf("%d/%d enqueued today (cap %d)", spent, dailyBudget, dailyBudget),
	}}
}

// doctorEnvironment checks the pool's dependencies: the agent binary, the
// build tools the task paths rely on (git, go), and, when --repos is
// given, each repo's harvest + autonomy files (bare repo names resolve
// against --projects-dir, like harvest/audit). The deployed pool once
// shipped with a systemd PATH missing git/go/crush — these checks make
// that failure class visible from the pool context (02:00 f18).
func doctorEnvironment(ctx context.Context, opts doctorOptions) []checkResult {
	var results []checkResult

	bin := opts.AgentBin
	if bin == "" {
		bin = "crush"
	}

	if _, err := exec.LookPath(bin); err != nil {
		results = append(results, checkResult{
			Name: "agent-binary", Status: checkWarn,
			Detail: fmt.Sprintf("%q not found on PATH (agent tasks cannot run; TQ_AGENT_BIN overrides)", bin),
		})
	} else {
		results = append(results, checkResult{Name: "agent-binary", Status: checkOK, Detail: bin + " found"})
		results = append(results, doctorCrushVersionCheck(ctx, bin))
	}

	for _, tool := range []struct {
		name, why string
	}{
		{"git", "harvest scans and task verify commands that use it cannot run"},
		{"go", "task verify commands that build/test cannot run"},
	} {
		if _, err := exec.LookPath(tool.name); err != nil {
			results = append(results, checkResult{
				Name:   "tool:" + tool.name,
				Status: checkWarn,
				Detail: fmt.Sprintf(
					"%q not found on PATH (%s; check the service PATH, e.g. agentPath)",
					tool.name,
					tool.why,
				),
			})
		} else {
			results = append(
				results,
				checkResult{Name: "tool:" + tool.name, Status: checkOK, Detail: tool.name + " found"},
			)
		}
	}

	results = append(results, doctorProbeGoEnv(ctx))

	for _, repo := range expandRepoSpecs(opts.ProjectsDir, splitRepos(opts.Repos)) {
		if repo == "" {
			continue
		}

		results = append(results, doctorRepoAutonomy(repo)...)
		results = append(results, doctorTreeGofmt(ctx, repo))
	}

	results = append(results, doctorTagAncestry())

	return results
}

// doctorTreeProbeTimeout bounds one live-tree gofmt walk: big repos take
// seconds, never minutes, and a hung walk must not hang the doctor.
const doctorTreeProbeTimeout = 60 * time.Second

// doctorTreeGofmt flags unformatted NON-gitignored Go files on a repo's
// live tree (M11): exactly the set the scoped verify gate (M6's
// gitignore-aware gofmt stage) dies on, seen before a dispatch pays for
// it. Gitignored drift (the vendor-gofmt class) is harmless under the
// scoped gate and stays doctorVerifyPins' finding when a PINNED verify is
// still unscoped; the two checks deliberately do not double-report.
func doctorTreeGofmt(ctx context.Context, repo string) checkResult {
	name := "gofmt:" + filepath.Base(repo)

	gofmt := resolveGofmt()
	if gofmt == "" {
		return checkResult{
			Name:   name,
			Status: checkWarn,
			Detail: "gofmt not on PATH or beside go (cannot probe the live tree)",
		}
	}

	ctx, cancel := context.WithTimeout(ctx, doctorTreeProbeTimeout)
	defer func() { cancel() }()

	walk := exec.CommandContext(ctx, gofmt, "-l", ".")
	walk.Dir = repo

	out, err := walk.Output()
	if err != nil {
		return checkResult{Name: name, Status: checkWarn, Detail: "gofmt -l failed: " + err.Error()}
	}

	unformatted := nonEmptyLines(out)
	if len(unformatted) == 0 {
		return checkResult{Name: name, Status: checkOK, Detail: "live tree gofmt-clean"}
	}

	// Scope to non-gitignored files, the M6 idiom: git check-ignore
	// answers "::\t<path>" for NOT ignored and
	// "<src>:<line>:<pattern>\t<path>" for ignored. Outside a git work
	// tree every unformatted file counts; the scoped mint would degrade
	// to a pass there, but the drift still deserves eyes.
	filter := exec.CommandContext(ctx, "git", "-C", repo, "check-ignore", "--stdin", "-v", "--non-matching")
	filter.Stdin = bytes.NewReader(out)

	filtered, filterErr := filter.Output()

	findings := unformatted
	caveat := " (git unavailable: unfiltered)"

	if filterErr == nil {
		findings = findings[:0]
		caveat = ""

		for _, line := range nonEmptyLines(filtered) {
			if path, ok := strings.CutPrefix(line, "::\t"); ok {
				findings = append(findings, path)
			}
		}
	}

	count := len(findings)

	if count == 0 {
		return checkResult{
			Name:   name,
			Status: checkOK,
			Detail: fmt.Sprintf(
				"gofmt drift confined to gitignored files (%d; harmless under the scoped gate)",
				len(unformatted),
			),
		}
	}

	if count > 20 {
		findings = append(findings[:20], fmt.Sprintf("+%d more", count-20))
	}

	return checkResult{
		Name:   name,
		Status: checkWarn,
		Detail: fmt.Sprintf(
			"%d non-gitignored file(s) fail gofmt (the scoped verify gate dies on these: gofmt them, or gitignore deliberately)%s",
			count,
			caveat,
		),
		Items: findings,
	}
}

// resolveGofmt finds the gofmt binary: PATH first, then the toolchain's
// GOROOT/bin (wrapped toolchains often hide it from PATH).
func resolveGofmt() string {
	if p, err := exec.LookPath("gofmt"); err == nil {
		return p
	}

	out, err := exec.Command("go", "env", "GOROOT").Output()
	if err != nil {
		return ""
	}

	candidate := filepath.Join(strings.TrimSpace(string(out)), "bin", "gofmt")
	if executableFile(candidate) {
		return candidate
	}

	return ""
}

// nonEmptyLines splits command output into its non-blank trimmed lines.
func nonEmptyLines(b []byte) []string {
	var lines []string

	for _, line := range strings.Split(string(b), "\n") {
		if line = strings.TrimSpace(line); line != "" {
			lines = append(lines, line)
		}
	}

	return lines
}

// doctorReadServiceUnit returns the unit file content via `systemctl cat`.
// Var so tests stub it hermetically (no systemd on all hosts).
var doctorReadServiceUnit = func(ctx context.Context, unit string) (string, error) {
	ctx, cancel := context.WithTimeout(ctx, crushVersionProbeTimeout)
	defer func() { cancel() }()

	out, err := exec.CommandContext(ctx, "systemctl", "cat", unit).Output()

	return string(out), err
}

// doctorServiceContext diagnoses the systemd unit's OWN environment — its
// PATH and GOEXPERIMENT — instead of only the invoking shell's. The 20h
// dead pool died on a unit PATH missing git/go/crush (02:00 f18); a
// doctor run from a healthy laptop said "ok" the whole time (05-38 §f).
func doctorServiceContext(ctx context.Context, opts doctorOptions) []checkResult {
	unit := opts.ServiceUnit
	if unit != "" && !strings.Contains(unit, ".") {
		unit += ".service"
	}

	content, err := doctorReadServiceUnit(ctx, unit)
	if err != nil {
		return []checkResult{{
			Name: "svc-unit", Status: checkWarn,
			Detail: fmt.Sprintf("cannot read unit %q: %v (systemctl available? unit name right?)", unit, err),
		}}
	}

	env, err := parseSystemdUnitEnv(unit, content)
	if err != nil {
		return []checkResult{{Name: "svc-unit", Status: checkWarn, Detail: err.Error()}}
	}

	results := []checkResult{{
		Name: "svc-unit", Status: checkOK,
		Detail: fmt.Sprintf("unit %q environment read (%d vars)", unit, len(env)),
	}}

	path := env["PATH"]
	if path == "" {
		results = append(results, checkResult{
			Name: "svc-path", Status: checkWarn,
			Detail: "unit sets no PATH (systemd default is minimal — likely missing git/go/crush)",
		})
	}

	bin := opts.AgentBin
	if bin == "" {
		bin = "crush"
	}

	for _, tool := range []struct{ name, why string }{
		{bin, "agent tasks cannot run"},
		{"git", "harvest scans and verify commands that use it cannot run"},
		{"go", "task verify commands that build/test cannot run"},
	} {
		if p := lookupOnPath(path, tool.name); p == "" {
			results = append(results, checkResult{
				Name: "svc:" + tool.name, Status: checkWarn,
				Detail: fmt.Sprintf("%q not on the unit's PATH (%s; extend agentPath/Environment in the unit)", tool.name, tool.why),
			})
		} else {
			results = append(results, checkResult{
				Name: "svc:" + tool.name, Status: checkOK,
				Detail: p + " (unit PATH)",
			})
		}
	}

	switch exp := env["GOEXPERIMENT"]; exp {
	case "jsonv2":
		results = append(results, checkResult{
			Name: "svc:goexp", Status: checkOK,
			Detail: "GOEXPERIMENT=jsonv2 set (json/v2 verify gates build)",
		})
	default:
		results = append(results, checkResult{
			Name: "svc:goexp", Status: checkWarn,
			Detail: fmt.Sprintf("GOEXPERIMENT=%q on the unit (want jsonv2) — verify commands importing encoding/json/v2 die with \"build constraints exclude all Go files\" (the env-lie, 2026-09-11 000001a08ebf)", exp),
		})
	}

	return results
}

// parseSystemdUnitEnv extracts the environment a systemd unit's processes
// see: every Environment= assignment (quoted values supported, multiple
// per line, repeated lines merge) plus EnvironmentFile= drops (a leading
// `-` makes the file optional). Returns an error when a required
// EnvironmentFile is unreadable.
func parseSystemdUnitEnv(unit, content string) (map[string]string, error) {
	env := map[string]string{}

	for line := range strings.SplitSeq(content, "\n") {
		line = strings.TrimSpace(line)

		switch {
		case strings.HasPrefix(line, "Environment="):
			for _, kv := range splitSystemdAssignments(strings.TrimPrefix(line, "Environment=")) {
				if k, v, ok := strings.Cut(kv, "="); ok {
					env[k] = v
				}
			}
		case strings.HasPrefix(line, "EnvironmentFile="):
			ref := strings.TrimPrefix(line, "EnvironmentFile=")
			optional := strings.HasPrefix(ref, "-")
			ref = strings.TrimPrefix(ref, "-")

			data, err := os.ReadFile(ref)
			if err != nil {
				if optional {
					continue
				}

				return nil, fmt.Errorf("unit %q EnvironmentFile %q: %w", unit, ref, err)
			}

			for fl := range strings.SplitSeq(string(data), "\n") {
				fl = strings.TrimSpace(fl)
				if fl == "" || strings.HasPrefix(fl, "#") || strings.HasPrefix(fl, ";") {
					continue
				}
				if k, v, ok := strings.Cut(fl, "="); ok {
					env[k] = unquoteSystemdValue(v)
				}
			}
		}
	}

	return env, nil
}

// splitSystemdAssignments splits one Environment= value into KEY=VALUE
// tokens, honoring systemd's double- and single-quote grouping (spaces
// inside quotes stay part of the value; quotes are stripped).
func splitSystemdAssignments(s string) []string {
	var (
		tokens []string
		cur    strings.Builder
		quoted rune
	)

	flush := func() {
		if cur.Len() > 0 {
			tokens = append(tokens, cur.String())
			cur.Reset()
		}
	}

	for _, r := range s {
		switch {
		case quoted != 0:
			if r == quoted {
				quoted = 0
			} else {
				cur.WriteRune(r)
			}
		case r == '"' || r == '\'':
			quoted = r
		case r == ' ' || r == '\t':
			flush()
		default:
			cur.WriteRune(r)
		}
	}

	flush()

	return tokens
}

// unquoteSystemdValue strips one layer of matching surrounding quotes
// from an EnvironmentFile value.
func unquoteSystemdValue(v string) string {
	if len(v) >= 2 {
		if (v[0] == '"' && v[len(v)-1] == '"') || (v[0] == '\'' && v[len(v)-1] == '\'') {
			return v[1 : len(v)-1]
		}
	}

	return v
}

// lookupOnPath resolves name the way exec.LookPath does, but against an
// explicit PATH string (the unit's, not this process's). Empty on miss.
func lookupOnPath(path, name string) string {
	if name == "" {
		return ""
	}

	if strings.ContainsRune(name, '/') {
		if executableFile(name) {
			return name
		}

		return ""
	}

	for dir := range strings.SplitSeq(path, ":") {
		if dir == "" {
			continue
		}

		if p := filepath.Join(dir, name); executableFile(p) {
			return p
		}
	}

	return ""
}

// executableFile reports whether path exists as a non-directory with at
// least one execute bit set.
func executableFile(path string) bool {
	info, err := os.Stat(path)

	return err == nil && !info.IsDir() && info.Mode()&0o111 != 0
}

// goEnvProbeTimeout bounds one probe build: a cold-cache build of the
// synthetic jsonv2 module is seconds, not minutes; a hung toolchain must
// not hang the doctor.
const goEnvProbeTimeout = time.Minute

// doctorProbeGoEnv is the env-lie detector (round-13 T3): probes whether
// this shell can build encoding/json/v2 — the import that dies with
// "build constraints exclude all Go files" when GOEXPERIMENT=jsonv2 is
// missing (the tq-agent-pool unit's env; five-plus windows burned judging
// finished work on that lying gate). A failing check means every
// bare-shell verify of a jsonv2 repo will lie. Var so tests stub it
// hermetically (the real probe needs a go toolchain).
var doctorProbeGoEnv = func(ctx context.Context) checkResult {
	if _, err := exec.LookPath("go"); err != nil {
		return checkResult{Name: "go-env", Status: checkOK, Detail: "go not on PATH — env-lie probe skipped"}
	}

	dir, err := os.MkdirTemp("", "tq-doctor-goenv")
	if err != nil {
		return checkResult{Name: "go-env", Status: checkWarn, Detail: "probe scratch: " + err.Error()}
	}
	defer func() { _ = os.RemoveAll(dir) }()

	ambErr, capErr := runGoEnvProbe(ctx, dir, os.Environ())

	return classifyGoEnvProbe(ambErr, capErr)
}

// runGoEnvProbe builds a synthetic module importing encoding/json/v2 twice
// in dir: once with baseEnv untouched (the AMBIENT environment — does this
// very shell lie?), once with executor.GoEnvExperiment forced on (the
// toolchain capability check that separates a missing env var from a
// version gate). Empty return = build succeeded.
func runGoEnvProbe(ctx context.Context, dir string, baseEnv []string) (ambErr, capErr string) {
	const (
		goMod  = "module tqenvprobe\n\ngo 1.26\n"
		mainGo = "package main\n\nimport _ \"encoding/json/v2\"\n\nfunc main() {}\n"
	)

	if err := os.WriteFile(filepath.Join(dir, "go.mod"), []byte(goMod), 0o600); err != nil {
		return "probe scratch: " + err.Error(), "probe scratch: " + err.Error()
	}

	if err := os.WriteFile(filepath.Join(dir, "main.go"), []byte(mainGo), 0o600); err != nil {
		return "probe scratch: " + err.Error(), "probe scratch: " + err.Error()
	}

	ambient := goEnvBuild(ctx, dir, baseEnv)
	capable := goEnvBuild(ctx, dir, append(baseEnv, executor.GoEnvExperiment))

	return ambient, capable
}

// goEnvBuild runs one `go build ./...` in dir with env, returning the
// trimmed output on failure and "" on success.
func goEnvBuild(ctx context.Context, dir string, env []string) string {
	ctx, cancel := context.WithTimeout(ctx, goEnvProbeTimeout)
	defer func() { cancel() }()

	cmd := exec.CommandContext(ctx, "go", "build", "./...")
	cmd.Dir = dir
	cmd.Env = env

	var buf bytes.Buffer

	cmd.Stdout = &buf
	cmd.Stderr = &buf

	if err := cmd.Run(); err != nil {
		return strings.TrimSpace(buf.String())
	}

	return ""
}

// classifyGoEnvProbe turns the two probe results into one verdict:
// ambient-build failure + experiment-build success is the ENV-LIE (a
// missing env var, fixable with one line); both failing is a toolchain
// capability gap (version gate), warned — never a reason to burn attempts.
func classifyGoEnvProbe(ambErr, capErr string) checkResult {
	const name = "go-env"

	switch {
	case ambErr == "":
		return checkResult{
			Name: name, Status: checkOK,
			Detail: "encoding/json/v2 builds in this environment — no env lie",
		}
	case capErr == "":
		return checkResult{
			Name: name, Status: checkFail,
			Detail: "ENV-LIE: encoding/json/v2 fails in this environment (" + ambErr + ") but builds with " +
				executor.GoEnvExperiment + " — bare-shell verifies of jsonv2 repos will lie; fix: `export " +
				executor.GoEnvExperiment + "` or `Environment=" + executor.GoEnvExperiment +
				"` on the tq-agent-pool unit (SystemNix)",
		}
	default:
		return checkResult{
			Name: name, Status: checkWarn,
			Detail: "go toolchain cannot build encoding/json/v2 even WITH " + executor.GoEnvExperiment +
				" (" + capErr + ") — toolchain/version gate, not a missing env var",
		}
	}
}

// doctorTagAncestry is the release-hygiene check (round-11 T15): a release
// tag that HEAD cannot reach means the lineage forked (the 2026-09-10
// reword incident's failure class) — the next release would gate its
// version ordering against a tag on the wrong side and `git describe`
// would answer with the fork's tags. Warn, never fail: tags are immutable
// and healing the fork is an owner decision.
func doctorTagAncestry() checkResult {
	const name = "tag-ancestry"

	revParse := exec.Command("git", "rev-parse", "--git-dir")
	if err := revParse.Run(); err != nil {
		return checkResult{Name: name, Status: checkOK, Detail: "not a git repo — ancestry check skipped"}
	}

	out, err := exec.Command("git", "tag", "--list", "v*").Output()
	if err != nil {
		return checkResult{Name: name, Status: checkWarn, Detail: "git tag: " + err.Error()}
	}

	var unreachable []string

	for tag := range strings.SplitSeq(strings.TrimSpace(string(out)), "\n") {
		tag = strings.TrimSpace(tag)
		if tag == "" {
			continue
		}

		if err := exec.Command("git", "merge-base", "--is-ancestor", tag, "HEAD").Run(); err != nil {
			unreachable = append(unreachable, tag)
		}
	}

	if len(unreachable) == 0 {
		return checkResult{Name: name, Status: checkOK, Detail: "all release tags reachable from HEAD"}
	}

	return checkResult{
		Name:   name,
		Status: checkWarn,
		Detail: fmt.Sprintf(
			"%d release tag(s) unreachable from HEAD (forked lineage — releases must not cut until healed): %s",
			len(unreachable), strings.Join(unreachable, ", "),
		),
	}
}

// doctorRepoAutonomy checks one repo's TODO_LIST.md (harvestable) and
// .crushrc (--yolo autonomy) files.
func doctorRepoAutonomy(repo string) []checkResult {
	var results []checkResult

	name := filepath.Base(repo)

	todo := filepath.Join(repo, harvest.DefaultTodoFile)
	if _, err := os.Stat(todo); err != nil {
		results = append(results, checkResult{
			Name: "repo:" + name, Status: checkWarn,
			Detail: "no " + harvest.DefaultTodoFile + " (nothing to harvest)",
		})
	} else {
		results = append(results, checkResult{
			Name: "repo:" + name, Status: checkOK,
			Detail: harvest.DefaultTodoFile + " present",
		})
	}

	crushrc := filepath.Join(repo, ".crushrc")
	if _, err := os.Stat(crushrc); err != nil {
		results = append(results, checkResult{
			Name: "autonomy:" + name, Status: checkWarn,
			Detail: "no .crushrc (--yolo agent tasks will fail fast in this repo)",
		})
	} else {
		results = append(results, checkResult{Name: "autonomy:" + name, Status: checkOK, Detail: ".crushrc present"})
		results = append(results, doctorCrushManagedBlock(name, crushrc))
	}

	return results
}

// doctorCrushManagedBlock verifies the repo's .crushrc tq-managed block
// pins the pool's reasoning effort: the owner ruling (2026-09-14) is that
// the pool ALWAYS wants xhigh, and the managed block is the only
// effort-carrying mechanism. Warn on drift, never FAIL.
func doctorCrushManagedBlock(repoName, path string) checkResult {
	const name = "crush-pin:"

	b, err := os.ReadFile(path)
	if err != nil {
		return checkResult{Name: name + repoName, Status: checkWarn, Detail: "read .crushrc: " + err.Error()}
	}

	inBlock := false
	effort := ""

	for _, line := range strings.Split(string(b), "\n") {
		switch strings.TrimSpace(line) {
		case tqManagedStart:
			inBlock = true
		case tqManagedEnd:
			inBlock = false
		default:
			if inBlock {
				fields := strings.Fields(line)

				for i, f := range fields {
					if f == "--reasoning-effort" && i+1 < len(fields) {
						effort = fields[i+1]
					}
				}
			}
		}
	}

	switch {
	case !inBlockSeen(b):
		return checkResult{
			Name: name + repoName, Status: checkWarn,
			Detail: ".crushrc has no tq managed block (run tq bootstrap to enroll autonomy + pins)",
		}
	case effort == "":
		return checkResult{
			Name:   name + repoName,
			Status: checkWarn,
			Detail: "managed block pins no --reasoning-effort (the pool wants xhigh; run tq bootstrap --model <provider/model>)",
		}
	case effort != "xhigh":
		return checkResult{
			Name: name + repoName, Status: checkWarn,
			Detail: "managed block pins --reasoning-effort " + effort + " (the pool wants xhigh; run tq bootstrap)",
		}
	default:
		return checkResult{
			Name:   name + repoName,
			Status: checkOK,
			Detail: "managed block pins --reasoning-effort xhigh",
		}
	}
}

// inBlockSeen reports whether the file contains the managed-block opener.
func inBlockSeen(b []byte) bool {
	return strings.Contains(string(b), tqManagedStart)
}

// doctorWorst summarizes a result set.
func doctorWorst(results []checkResult) string {
	worst := checkOK

	for _, r := range results {
		switch r.Status {
		case checkFail:
			return checkFail
		case checkWarn:
			worst = checkWarn
		}
	}

	return worst
}

func cmdDoctor(args []string) error {
	fs := flag.NewFlagSet("doctor", flag.ExitOnError)

	db := dbFlag(fs)
	asJSON := fs.Bool("json", false, "machine-readable output")
	dailyBudget := fs.Int("daily-budget", 0, "report spend against this daily enqueue cap (0 = skip)")
	repos := fs.String(
		"repos",
		"",
		"comma-separated repo paths: check TODO_LIST.md and .crushrc autonomy files (bare names resolve against --projects-dir)",
	)
	projectsDir := fs.String(
		"projects-dir",
		defaultProjectsDir(),
		"root for bare repo names in --repos (default $TQ_PROJECTS_DIR or ~/projects)",
	)
	agentBin := fs.String("agent-bin", "", "agent binary to look for (default crush)")
	markOrphans := fs.Bool(
		"mark-orphans",
		false,
		"record stranded Running tasks (expired lease, no reclaim) as task.orphaned facts — doctor's only write",
	)
	hygiene := fs.Bool(
		"hygiene",
		false,
		"audit PENDING agent tasks' enqueue-time verify pins against each repo's current gate (.tq-verify, else auto-detect) — the stale-payload check",
	)
	serviceUnit := fs.String(
		"service-unit",
		"",
		"systemd unit name (e.g. tq-agent-pool): diagnose the unit's OWN PATH and GOEXPERIMENT via systemctl cat, not the caller's",
	)

	if err := fs.Parse(args); err != nil {
		return err
	}

	opts := doctorOptions{
		DBPath:      resolveDB(*db),
		DailyBudget: *dailyBudget,
		Repos:       *repos,
		ProjectsDir: *projectsDir,
		AgentBin:    *agentBin,
		MarkOrphans: *markOrphans,
		Hygiene:     *hygiene,
		ServiceUnit: *serviceUnit,
	}

	results, err := runDoctor(context.Background(), opts)
	if err != nil {
		if *asJSON {
			enc := json.NewEncoder(os.Stdout)
			_ = enc.Encode(
				map[string]any{
					"status": checkFail,
					"checks": []checkResult{{Name: "doctor", Status: checkFail, Detail: err.Error()}},
				},
			)
		} else {
			fmt.Fprintf(os.Stderr, "tq doctor: %v\n", err)
		}

		return err
	}

	if *asJSON {
		enc := json.NewEncoder(os.Stdout)

		return enc.Encode(map[string]any{"status": doctorWorst(results), "checks": results})
	}

	worst := doctorWorst(results)
	for _, r := range results {
		mark := map[string]string{checkOK: "ok", checkWarn: "WARN", checkFail: "FAIL"}[r.Status]
		fmt.Printf("%-4s %-16s %s\n", mark, r.Name, r.Detail)
	}

	fmt.Printf("doctor: %s\n", worst)

	if worst == checkFail {
		return errDoctorFailed
	}

	return nil
}
