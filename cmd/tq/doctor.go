package main

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	"github.com/larsartmann/go-taskqueue/internal/harvest"
	"github.com/larsartmann/go-taskqueue/internal/journal"
	"github.com/larsartmann/go-taskqueue/internal/queue"
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

// checkResult is one doctor finding.
type checkResult struct {
	Name   string `json:"name"`
	Status string `json:"status"`
	Detail string `json:"detail"`
}

// doctorOptions controls which checks run.
type doctorOptions struct {
	DBPath      string
	DailyBudget int    // 0: skip the budget check
	Repos       string // comma-separated repo paths: enables autonomy checks
	AgentBin    string // agent binary override (defaults to crush)
	// MarkOrphans, when set, appends task.orphaned facts for stranded
	// Running tasks (expired lease, no reclaim) — the only write `tq
	// doctor` can perform, and only on explicit request.
	MarkOrphans bool
}

// doctorHeartbeatWindow is how long ago a task.heartbeat fact still counts
// as "a worker is alive" evidence.
const doctorHeartbeatWindow = 10 * time.Minute

// runDoctor executes every check against the database and environment,
// returning results ordered worst-last. The returned error is non-nil only
// when the database cannot be inspected at all.
func runDoctor(ctx context.Context, opts doctorOptions) ([]checkResult, error) {
	var results []checkResult

	store, err := queue.OpenSQLite(opts.DBPath)
	if err != nil {
		return nil, fmt.Errorf("open db: %w", err)
	}
	defer func() { _ = store.Close() }()

	results = append(results, doctorSQLiteChecks(ctx, opts.DBPath)...)
	results = append(results, doctorQueueMix(ctx, store)...)
	results = append(results, doctorWorkerLiveness(ctx, store)...)
	results = append(results, doctorBudget(ctx, store, opts.DailyBudget)...)
	results = append(results, doctorEnvironment(opts)...)

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
	stuck := doctorStuckRunning(ctx, store, time.Now())

	detail := fmt.Sprintf("pending=%d running=%d completed=%d dead=%d cancelled=%d",
		counts[task.Pending], running, counts[task.Completed], counts[task.Dead], counts[task.Cancelled])

	status := checkOK
	if stuck > 0 {
		status = checkWarn
		detail += fmt.Sprintf("; %d running task(s) have an EXPIRED lease and no worker reclaimed them (tq doctor --mark-orphans records them)", stuck)
	}

	return []checkResult{
		{Name: "queue", Status: status, Detail: detail},
		{
			Name: "dlq", Status: doctorCountStatus(counts[task.Dead]),
			Detail: fmt.Sprintf("%d dead-lettered task(s)", counts[task.Dead]),
		},
	}
}

// doctorStuckRunning counts running tasks whose lease expired without a
// reclaim: work that WOULD run if any worker were claiming.
func doctorStuckRunning(ctx context.Context, store queue.Store, now time.Time) int {
	running := task.Running

	tasks, err := store.List(ctx, queue.Filter{Status: &running})
	if err != nil {
		return 0
	}

	stuck := 0
	for _, t := range tasks {
		if t.LeaseExpires != nil && t.LeaseExpires.Before(now) {
			stuck++
		}
	}

	return stuck
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
		detail = fmt.Sprintf("marked %d stranded task(s) as task.orphaned (still Running; the next reclaiming worker picks them up)", marked)
	}

	return []checkResult{{Name: "mark-orphans", Status: checkOK, Detail: detail}}
}

// doctorCountStatus maps DLQ size to severity: 0-2 is normal operation,
// more deserves a look.
func doctorCountStatus(dead int) string {
	if dead > 2 {
		return checkWarn
	}

	return checkOK
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

	return []checkResult{{
		Name: "worker", Status: checkFail,
		Detail: fmt.Sprintf("no heartbeats in the last %s while %d pending / %d running tasks wait — is a worker running?",
			doctorHeartbeatWindow, counts[task.Pending], counts[task.Running]),
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

// doctorEnvironment checks the pool's dependencies: the agent binary and,
// when --repos is given, each repo's harvest + autonomy files.
func doctorEnvironment(opts doctorOptions) []checkResult {
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
	}

	for _, repo := range splitRepos(opts.Repos) {
		if repo == "" {
			continue
		}

		results = append(results, doctorRepoAutonomy(repo)...)
	}

	return results
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

	if _, err := os.Stat(filepath.Join(repo, ".crushrc")); err != nil {
		results = append(results, checkResult{
			Name: "autonomy:" + name, Status: checkWarn,
			Detail: "no .crushrc (--yolo agent tasks will fail fast in this repo)",
		})
	} else {
		results = append(results, checkResult{Name: "autonomy:" + name, Status: checkOK, Detail: ".crushrc present"})
	}

	return results
}

// doctorWorst summarizes a result set.
func doctorWorst(results []checkResult) string {
	worst := checkOK
	for _, r := range results {
		switch {
		case r.Status == checkFail:
			return checkFail
		case r.Status == checkWarn:
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
	repos := fs.String("repos", "", "comma-separated repo paths: check TODO_LIST.md and .crushrc autonomy files")
	agentBin := fs.String("agent-bin", "", "agent binary to look for (default crush)")
	markOrphans := fs.Bool("mark-orphans", false, "record stranded Running tasks (expired lease, no reclaim) as task.orphaned facts — doctor's only write")

	if err := fs.Parse(args); err != nil {
		return err
	}

	opts := doctorOptions{
		DBPath:      resolveDB(*db),
		DailyBudget: *dailyBudget,
		Repos:       *repos,
		AgentBin:    *agentBin,
		MarkOrphans: *markOrphans,
	}

	results, err := runDoctor(context.Background(), opts)
	if err != nil {
		if *asJSON {
			enc := json.NewEncoder(os.Stdout)
			_ = enc.Encode(map[string]any{"status": checkFail, "checks": []checkResult{{Name: "doctor", Status: checkFail, Detail: err.Error()}}})
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
		return errors.New("failing checks found")
	}

	return nil
}
