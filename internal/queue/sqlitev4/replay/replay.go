// Package replay implements the ADR-0019 S1 data-migration tool: it
// replays a tq fact journal — the hand-rolled store's database — into a
// fresh go-cqrs-lite engine store (this module's adapter over
// queue/sqlite/v4) and verifies projection equality (StatusCounts,
// ProjectCounts, the full fact stream, per-task fact tails, DLQ
// contents, watermarks, priority scores). It is S1's definition-of-done
// gate for the dogfood cutover (ADR-0019 §Data migration; C12/M056 in
// docs/planning/2026-09-22_23-49_go-cqrs-lite-platform-migration.md).
//
// C12 replay-design decision (resolving the memo's open question): the
// applier is a VERBATIM projection copy, not an operation-by-operation
// transition replay. The journal alone cannot reconstruct task rows —
// task.enqueued details carry project/type/priority/dedup key but NOT
// the payload, deps, or max attempts — so a pure transition replay would
// silently drop every prompt. The copy preserves task IDs, dedup keys,
// fact seq numbers, timestamps, and detail bytes exactly, which makes
// the equality gate meaningful: any divergence between the old store's
// projections and the engine-backed adapter's read paths is a real
// defect, not replay fuzz. If the enqueued fact later grows a full task
// snapshot (upstream-grow candidate), a transition applier can replace
// the row copy without changing the verify half.
//
// Known shape conversions (the two schemas differ deliberately):
//   - tasks.payload: old TEXT → engine BLOB (identity codec, byte-equal)
//   - tasks.lease_token: absent in the old schema → NULL. Running tasks
//     therefore migrate without a claim-fencing token (upstream ADR-0134
//     finalizes); a lapsed-lease reclaim re-mints one. Divergence D1 of
//     the S1 conformance report covers the finalize semantics.
//   - facts_archive and journal_meta (old-only tables) are recreated
//     verbatim so no history is lost; the engine does not read them.
//
// Usage:
//
//	go run ./replay --from <old.db> --to <new.db> [--verify-only]
//
// Exit 0 on a green report, 1 on any projection mismatch, 2 on setup
// errors. The tool never writes to the source database.
package main

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"slices"
	"sort"
	"strings"

	"github.com/larsartmann/go-taskqueue/internal/journal"
	"github.com/larsartmann/go-taskqueue/internal/queue"
	"github.com/larsartmann/go-taskqueue/internal/queue/sqlitev4"
	"github.com/larsartmann/go-taskqueue/internal/task"

	_ "modernc.org/sqlite" // pure-Go SQLite driver (CGo-free)
)

const (
	// factPageLimit is the pagination page for streaming the new store's
	// fact journal during verification.
	factPageLimit = 500
	// taskTailLimit is the per-task fact-tail depth verified on both
	// sides; the tails are compared as complete sequences, so any depth
	// at or above every real tail is equivalent.
	taskTailLimit = 100000
)

// copyDSN opens a SQLite file for the migration copy (single writer, WAL).
func copyDSN(path string) string {
	return fmt.Sprintf("file:%s?_pragma=journal_mode(WAL)&_pragma=busy_timeout(5000)&_pragma=foreign_keys(1)", path)
}

// readOnlyDSN opens a SQLite file read-only: the migration never writes
// to the source journal.
func readOnlyDSN(path string) string {
	return fmt.Sprintf("file:%s?mode=ro&_pragma=busy_timeout(5000)", path)
}

// Stats reports how many rows the migration copied per table.
type Stats struct {
	Tasks         int
	Deps          int
	Facts         int
	Watermarks    int
	PriorityScores int
	FactsArchive  int
	JournalMeta   int
}

// Section is one projection-equality verdict.
type Section struct {
	Name   string
	OK     bool
	Detail string
}

// Report is the full projection-equality verdict.
type Report struct {
	Sections []Section
}

// OK reports whether every section passed.
func (r Report) OK() bool {
	for _, s := range r.Sections {
		if !s.OK {
			return false
		}
	}

	return true
}

// Summary renders the report for terminal output.
func (r Report) Summary() string {
	var b strings.Builder

	for _, s := range r.Sections {
		status := "ok"
		if !s.OK {
			status = "MISMATCH"
		}

		fmt.Fprintf(&b, "%-18s %-9s %s\n", s.Name, status, s.Detail)
	}

	return b.String()
}

// Migrate replays the source journal into a fresh engine store at
// toPath. toPath must not already exist: a fresh store is part of the
// definition (no in-place surgery on a live engine DB).
func Migrate(ctx context.Context, fromPath, toPath string) (Stats, error) {
	var stats Stats

	src, err := sql.Open("sqlite", readOnlyDSN(fromPath))
	if err != nil {
		return stats, fmt.Errorf("replay: open source: %w", err)
	}
	defer src.Close()

	// Open+close the target through the real adapter once: it creates the
	// engine schema AND the companion tables, exactly as a cutover store
	// would boot.
	fresh, err := sqlitev4.Open(toPath)
	if err != nil {
		return stats, fmt.Errorf("replay: bootstrap engine store: %w", err)
	}
	if err := fresh.Close(); err != nil {
		return stats, fmt.Errorf("replay: close bootstrap store: %w", err)
	}

	dst, err := sql.Open("sqlite", copyDSN(toPath))
	if err != nil {
		return stats, fmt.Errorf("replay: open target: %w", err)
	}
	defer dst.Close()

	tx, err := dst.BeginTx(ctx, nil)
	if err != nil {
		return stats, fmt.Errorf("replay: begin copy tx: %w", err)
	}
	defer func() { _ = tx.Rollback() }()

	if stats.Tasks, err = copyTasks(ctx, src, tx); err != nil {
		return stats, fmt.Errorf("replay: tasks: %w", err)
	}
	if stats.Deps, err = copyDeps(ctx, src, tx); err != nil {
		return stats, fmt.Errorf("replay: deps: %w", err)
	}
	if stats.Facts, err = copyRows(ctx, src, tx, "facts",
		"seq, time, task_id, type, owner, attempt, error, detail", &stats.Facts); err != nil {
		return stats, fmt.Errorf("replay: facts: %w", err)
	}
	if stats.Watermarks, err = copyRows(ctx, src, tx, "watermarks",
		"consumer, seq, updated_at", &stats.Watermarks); err != nil {
		return stats, fmt.Errorf("replay: watermarks: %w", err)
	}
	if stats.PriorityScores, err = copyRows(ctx, src, tx, "priority_scores",
		"item_key, score, effort_minutes, source, reasoning, tokens, scored_at", &stats.PriorityScores); err != nil {
		return stats, fmt.Errorf("replay: priority_scores: %w", err)
	}
	if stats.FactsArchive, err = copyLegacyTable(ctx, src, tx, "facts_archive",
		"seq, time, task_id, type, owner, attempt, error, detail", &stats.FactsArchive); err != nil {
		return stats, fmt.Errorf("replay: facts_archive: %w", err)
	}
	if stats.JournalMeta, err = copyLegacyTable(ctx, src, tx, "journal_meta",
		"key, value", &stats.JournalMeta); err != nil {
		return stats, fmt.Errorf("replay: journal_meta: %w", err)
	}

	if err := tx.Commit(); err != nil {
		return stats, fmt.Errorf("replay: commit copy: %w", err)
	}

	return stats, nil
}

// copyTasks copies task rows with the two schema conversions: payload
// TEXT→BLOB and lease_token NULL (the old schema has no fencing token).
func copyTasks(ctx context.Context, src *sql.DB, tx *sql.Tx) (int, error) {
	rows, err := src.QueryContext(ctx, `
		SELECT id, project, type, payload, deps, priority, attempts, max_attempts,
		       not_before, status, lease_owner, lease_expires, last_error,
		       created_at, updated_at, completed_at, dedup_key
		FROM tasks ORDER BY created_at ASC, id ASC`)
	if err != nil {
		return 0, err
	}
	defer rows.Close()

	const insert = `
		INSERT INTO tasks (id, project, type, payload, deps, priority, attempts, max_attempts,
		                   not_before, status, lease_owner, lease_expires, lease_token, last_error,
		                   created_at, updated_at, completed_at, dedup_key)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, NULL, ?, ?, ?, ?, ?)`

	copied := 0

	for rows.Next() {
		var (
			id, project, typ, payload, deps, status, leaseOwner, lastError, dedupKey string
			priority, attempts, maxAttempts, notBefore, createdAt, updatedAt          int64
			leaseExpires, completedAt                                                 sql.NullInt64
		)
		if err := rows.Scan(&id, &project, &typ, &payload, &deps, &priority, &attempts,
			&maxAttempts, &notBefore, &status, &leaseOwner, &leaseExpires, &lastError,
			&createdAt, &updatedAt, &completedAt, &dedupKey); err != nil {
			return 0, err
		}

		if _, err := tx.ExecContext(ctx, insert,
			id, project, typ, []byte(payload), deps, priority, attempts, maxAttempts,
			notBefore, status, leaseOwner, leaseExpires, lastError,
			createdAt, updatedAt, completedAt, dedupKey); err != nil {
			return 0, err
		}

		copied++
	}

	return copied, rows.Err()
}

// copyDeps copies dependency edges; the tasks rows already exist, so the
// foreign keys resolve.
func copyDeps(ctx context.Context, src *sql.DB, tx *sql.Tx) (int, error) {
	rows, err := src.QueryContext(ctx, `SELECT task_id, dep_id FROM deps ORDER BY task_id, dep_id`)
	if err != nil {
		return 0, err
	}
	defer rows.Close()

	copied := 0

	for rows.Next() {
		var taskID, depID string
		if err := rows.Scan(&taskID, &depID); err != nil {
			return 0, err
		}

		if _, err := tx.ExecContext(ctx, `INSERT INTO deps (task_id, dep_id) VALUES (?, ?)`, taskID, depID); err != nil {
			return 0, err
		}

		copied++
	}

	return copied, rows.Err()
}

// copyRows copies a same-shape table (facts, watermarks, priority_scores)
// verbatim; the engine and companion schemas already define them.
func copyRows(ctx context.Context, src *sql.DB, tx *sql.Tx, table, cols string, _ *int) (int, error) {
	rows, err := src.QueryContext(ctx, fmt.Sprintf(`SELECT %s FROM %s ORDER BY 1 ASC`, cols, table)) //nolint:gosec // table/cols are compile-time constants
	if err != nil {
		return 0, err
	}
	defer rows.Close()

	dest := make([]any, strings.Count(cols, ",")+1)
	vals := make([]any, len(dest))
	for i := range vals {
		vals[i] = &dest[i]
	}

	copied := 0

	for rows.Next() {
		if err := rows.Scan(vals...); err != nil {
			return 0, err
		}

		placeholders := strings.TrimSuffix(strings.Repeat("?, ", len(dest)), ", ")
		if _, err := tx.ExecContext(ctx,
			fmt.Sprintf("INSERT INTO %s (%s) VALUES (%s)", table, cols, placeholders), //nolint:gosec // compile-time constants
			dest...); err != nil {
			return 0, err
		}

		copied++
	}

	return copied, rows.Err()
}

// copyLegacyTable recreates an old-only table (facts_archive,
// journal_meta) in the target verbatim — the engine never reads it, but
// cutover must not lose history.
func copyLegacyTable(ctx context.Context, src *sql.DB, tx *sql.Tx, table, cols string, _ *int) (int, error) {
	var creates sql.NullString
	if err := src.QueryRowContext(ctx,
		`SELECT sql FROM sqlite_master WHERE type = 'table' AND name = ?`, table).Scan(&creates); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return 0, nil // table does not exist in the source: nothing to carry
		}

		return 0, err
	}

	if _, err := tx.ExecContext(ctx, creates.String); err != nil {
		return 0, err
	}

	return copyRows(ctx, src, tx, table, cols, nil)
}

// Verify compares the source journal's projections against the target
// engine store's read paths. The source side is read with the same SQL
// the hand-rolled store runs (that store is frozen history at cutover);
// the target side goes through the real queue.Store API — the gate
// proves what consumers see after the flip.
func Verify(ctx context.Context, fromPath, toPath string) (Report, error) {
	var report Report

	src, err := sql.Open("sqlite", readOnlyDSN(fromPath))
	if err != nil {
		return report, fmt.Errorf("replay: open source: %w", err)
	}
	defer src.Close()

	target, err := sqlitev4.Open(toPath)
	if err != nil {
		return report, fmt.Errorf("replay: open engine store: %w", err)
	}
	defer target.Close()

	report.Sections = append(report.Sections,
		verifyStatusCounts(ctx, src, target),
		verifyProjectCounts(ctx, src, target),
		verifyDLQ(ctx, src, target),
		verifyWatermarks(ctx, src, target),
		verifyPriorityScores(ctx, src, target),
	)
	factSections, err := verifyFacts(ctx, src, target)
	if err != nil {
		return report, fmt.Errorf("replay: fact stream: %w", err)
	}

	report.Sections = append(report.Sections, factSections...)

	return report, nil
}

// oldStatusCounts mirrors the hand-rolled StatusCounts query.
func oldStatusCounts(ctx context.Context, src *sql.DB) (map[task.Status]int, error) {
	rows, err := src.QueryContext(ctx, `SELECT status, COUNT(*) FROM tasks GROUP BY status`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	out := map[task.Status]int{}

	for rows.Next() {
		var st task.Status
		var n int
		if err := rows.Scan(&st, &n); err != nil {
			return nil, err
		}

		out[st] = n
	}

	return out, rows.Err()
}

func verifyStatusCounts(ctx context.Context, src *sql.DB, target *sqlitev4.Store) Section {
	oldCounts, err := oldStatusCounts(ctx, src)
	if err != nil {
		return Section{Name: "status counts", Detail: fmt.Sprintf("source read failed: %v", err)}
	}

	newCounts, err := target.StatusCounts(ctx)
	if err != nil {
		return Section{Name: "status counts", Detail: fmt.Sprintf("target read failed: %v", err)}
	}

	if equalMaps(oldCounts, newCounts) {
		return Section{Name: "status counts", OK: true, Detail: formatCounts(oldCounts)}
	}

	return Section{Name: "status counts", Detail: fmt.Sprintf("source %v vs target %v", formatCounts(oldCounts), formatCounts(newCounts))}
}

func verifyProjectCounts(ctx context.Context, src *sql.DB, target *sqlitev4.Store) Section {
	rows, err := src.QueryContext(ctx, `SELECT project, status, COUNT(*) FROM tasks GROUP BY project, status`)
	if err != nil {
		return Section{Name: "project counts", Detail: fmt.Sprintf("source read failed: %v", err)}
	}
	defer rows.Close()

	oldCounts := map[string]map[task.Status]int{}

	for rows.Next() {
		var project string
		var st task.Status
		var n int
		if err := rows.Scan(&project, &st, &n); err != nil {
			return Section{Name: "project counts", Detail: fmt.Sprintf("source scan failed: %v", err)}
		}

		if oldCounts[project] == nil {
			oldCounts[project] = map[task.Status]int{}
		}

		oldCounts[project][st] = n
	}

	if err := rows.Err(); err != nil {
		return Section{Name: "project counts", Detail: fmt.Sprintf("source read failed: %v", err)}
	}

	newCounts, err := target.ProjectCounts(ctx)
	if err != nil {
		return Section{Name: "project counts", Detail: fmt.Sprintf("target read failed: %v", err)}
	}

	if len(oldCounts) != len(newCounts) {
		return Section{Name: "project counts", Detail: fmt.Sprintf("source has %d projects, target %d", len(oldCounts), len(newCounts))}
	}

	for project, old := range oldCounts {
		new, ok := newCounts[project]
		if !ok || !equalMaps(old, new) {
			return Section{Name: "project counts", Detail: fmt.Sprintf("project %q differs", project)}
		}
	}

	return Section{Name: "project counts", OK: true, Detail: fmt.Sprintf("%d projects", len(oldCounts))}
}

func verifyDLQ(ctx context.Context, src *sql.DB, target *sqlitev4.Store) Section {
	oldTasks, err := oldListStatus(ctx, src, task.Dead)
	if err != nil {
		return Section{Name: "dlq", Detail: fmt.Sprintf("source read failed: %v", err)}
	}

	newTasks, err := target.List(ctx, queue.Filter{Status: deadStatus()})
	if err != nil {
		return Section{Name: "dlq", Detail: fmt.Sprintf("target read failed: %v", err)}
	}

	if len(oldTasks) != len(newTasks) {
		return Section{Name: "dlq", Detail: fmt.Sprintf("source has %d dead tasks, target %d", len(oldTasks), len(newTasks))}
	}

	sort.Slice(oldTasks, func(i, j int) bool { return oldTasks[i].ID < oldTasks[j].ID })
	sort.Slice(newTasks, func(i, j int) bool { return newTasks[i].ID < newTasks[j].ID })

	for i := range oldTasks {
		if !equalTasks(oldTasks[i], newTasks[i]) {
			return Section{Name: "dlq", Detail: fmt.Sprintf("dead task %s differs", oldTasks[i].ID)}
		}
	}

	return Section{Name: "dlq", OK: true, Detail: fmt.Sprintf("%d dead tasks identical", len(oldTasks))}
}

func verifyWatermarks(ctx context.Context, src *sql.DB, target *sqlitev4.Store) Section {
	rows, err := src.QueryContext(ctx, `SELECT consumer, seq, updated_at FROM watermarks ORDER BY consumer`)
	if err != nil {
		return Section{Name: "watermarks", Detail: fmt.Sprintf("source read failed: %v", err)}
	}
	defer rows.Close()

	old := map[string]queue.WatermarkEntry{}

	for rows.Next() {
		var e queue.WatermarkEntry
		if err := rows.Scan(&e.Consumer, &e.Seq, &e.UpdatedAt); err != nil {
			return Section{Name: "watermarks", Detail: fmt.Sprintf("source scan failed: %v", err)}
		}

		old[e.Consumer] = e
	}

	if err := rows.Err(); err != nil {
		return Section{Name: "watermarks", Detail: fmt.Sprintf("source read failed: %v", err)}
	}

	entries, err := target.ListWatermarks(ctx)
	if err != nil {
		return Section{Name: "watermarks", Detail: fmt.Sprintf("target read failed: %v", err)}
	}

	new := make(map[string]queue.WatermarkEntry, len(entries))
	for _, e := range entries {
		new[e.Consumer] = e
	}

	if len(old) != len(new) {
		return Section{Name: "watermarks", Detail: fmt.Sprintf("source has %d watermarks, target %d", len(old), len(new))}
	}

	for consumer, oldEntry := range old {
		newEntry, ok := new[consumer]
		if !ok || oldEntry != newEntry {
			return Section{Name: "watermarks", Detail: fmt.Sprintf("watermark %q differs", consumer)}
		}
	}

	return Section{Name: "watermarks", OK: true, Detail: fmt.Sprintf("%d watermarks identical", len(old))}
}

func verifyPriorityScores(ctx context.Context, src *sql.DB, target *sqlitev4.Store) Section {
	rows, err := src.QueryContext(ctx,
		`SELECT item_key, score, effort_minutes, source, reasoning, tokens, scored_at FROM priority_scores ORDER BY item_key`)
	if err != nil {
		return Section{Name: "priority scores", Detail: fmt.Sprintf("source read failed: %v", err)}
	}
	defer rows.Close()

	var old []queue.PriorityScore

	for rows.Next() {
		var s queue.PriorityScore
		if err := rows.Scan(&s.ItemKey, &s.Score, &s.EffortMinutes, &s.Source, &s.Reasoning, &s.Tokens, &s.ScoredAt); err != nil {
			return Section{Name: "priority scores", Detail: fmt.Sprintf("source scan failed: %v", err)}
		}

		old = append(old, s)
	}

	if err := rows.Err(); err != nil {
		return Section{Name: "priority scores", Detail: fmt.Sprintf("source read failed: %v", err)}
	}

	new, err := target.PriorityScores(ctx)
	if err != nil {
		return Section{Name: "priority scores", Detail: fmt.Sprintf("target read failed: %v", err)}
	}

	slices.SortFunc(old, func(a, b queue.PriorityScore) int { return strings.Compare(a.ItemKey, b.ItemKey) })
	slices.SortFunc(new, func(a, b queue.PriorityScore) int { return strings.Compare(a.ItemKey, b.ItemKey) })

	if len(old) != len(new) {
		return Section{Name: "priority scores", Detail: fmt.Sprintf("source has %d scores, target %d", len(old), len(new))}
	}

	for i := range old {
		if old[i] != new[i] {
			return Section{Name: "priority scores", Detail: fmt.Sprintf("score %q differs", old[i].ItemKey)}
		}
	}

	return Section{Name: "priority scores", OK: true, Detail: fmt.Sprintf("%d scores identical", len(old))}
}

// verifyFacts compares the complete fact stream (and head seq) plus every
// task's fact tail.
func verifyFacts(ctx context.Context, src *sql.DB, target *sqlitev4.Store) ([]Section, error) {
	oldFacts, err := oldAllFacts(ctx, src)
	if err != nil {
		return nil, err
	}

	var newFacts []journal.Fact

	after := int64(0)
	for {
		page, err := target.Facts(ctx, after, factPageLimit)
		if err != nil {
			return nil, fmt.Errorf("target read: %w", err)
		}

		newFacts = append(newFacts, page...)

		if len(page) < factPageLimit {
			break
		}

		after = page[len(page)-1].Seq
	}

	stream := Section{Name: "fact stream", Detail: fmt.Sprintf("source %d facts, target %d facts (head seq %d)", len(oldFacts), len(newFacts), after)}

	if len(oldFacts) != len(newFacts) {
		return []Section{stream}, nil
	}

	for i := range oldFacts {
		if !equalFacts(oldFacts[i], newFacts[i]) {
			stream.Detail = fmt.Sprintf("fact seq %d differs", oldFacts[i].Seq)

			return []Section{stream}, nil
		}
	}

	stream.OK = true
	stream.Detail = fmt.Sprintf("%d facts identical (head seq %d)", len(oldFacts), after)

	heads, err := oldHeadSeq(ctx, src)
	if err != nil {
		return []Section{stream}, err
	}

	newHead, err := target.HeadSeq(ctx)
	if err != nil {
		return []Section{stream}, fmt.Errorf("target head seq: %w", err)
	}

	head := Section{Name: "head seq", OK: heads == newHead, Detail: fmt.Sprintf("source %d, target %d", heads, newHead)}

	tails, err := verifyTaskTails(ctx, src, target)
	if err != nil {
		return []Section{stream, head}, err
	}

	return []Section{stream, head, tails}, nil
}

// verifyTaskTails walks every task (and synthetic fact-only identity,
// e.g. session:<id>) in the source and compares its per-task fact tail
// through the target's FactsForTask.
func verifyTaskTails(ctx context.Context, src *sql.DB, target *sqlitev4.Store) (Section, error) {
	ids, err := oldTaskIDs(ctx, src)
	if err != nil {
		return Section{Name: "task tails"}, err
	}

	tails := map[string][]journal.Fact{}
	for _, id := range ids {
		old, err := oldFactsForTask(ctx, src, id)
		if err != nil {
			return Section{Name: "task tails"}, err
		}

		tails[id] = old
	}

	// Fact-only identities (session:<id>, never task rows) appear in the
	// stream but not the tasks table; derive their ids from the stream so
	// their tails are verified too.
	all, err := oldAllFacts(ctx, src)
	if err != nil {
		return Section{Name: "task tails"}, err
	}

	for _, f := range all {
		if _, ok := tails[f.TaskID]; !ok {
			old, err := oldFactsForTask(ctx, src, f.TaskID)
			if err != nil {
				return Section{Name: "task tails"}, err
			}

			tails[f.TaskID] = old
		}
	}

	for id, old := range tails {
		new, err := target.FactsForTask(ctx, id, taskTailLimit)
		if err != nil {
			return Section{Name: "task tails"}, fmt.Errorf("target tail for %s: %w", id, err)
		}

		slices.SortFunc(old, func(a, b journal.Fact) int { return int(a.Seq - b.Seq) })
		slices.SortFunc(new, func(a, b journal.Fact) int { return int(a.Seq - b.Seq) })

		if len(old) != len(new) {
			return Section{Name: "task tails", Detail: fmt.Sprintf("task %s: source %d facts, target %d", id, len(old), len(new))}, nil
		}

		for i := range old {
			if !equalFacts(old[i], new[i]) {
				return Section{Name: "task tails", Detail: fmt.Sprintf("task %s: fact seq %d differs", id, old[i].Seq)}, nil
			}
		}
	}

	return Section{Name: "task tails", OK: true, Detail: fmt.Sprintf("%d task tails identical", len(tails))}, nil
}

// --- source-side readers (the frozen old schema, read-only) ---

func oldAllFacts(ctx context.Context, src *sql.DB) ([]journal.Fact, error) {
	rows, err := src.QueryContext(ctx,
		`SELECT seq, time, task_id, type, owner, attempt, error, detail FROM facts ORDER BY seq ASC`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var out []journal.Fact

	for rows.Next() {
		f, err := scanOldFact(rows)
		if err != nil {
			return nil, err
		}

		out = append(out, f)
	}

	return out, rows.Err()
}

func oldFactsForTask(ctx context.Context, src *sql.DB, id string) ([]journal.Fact, error) {
	rows, err := src.QueryContext(ctx,
		`SELECT seq, time, task_id, type, owner, attempt, error, detail FROM facts WHERE task_id = ? ORDER BY seq ASC`, id)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var out []journal.Fact

	for rows.Next() {
		f, err := scanOldFact(rows)
		if err != nil {
			return nil, err
		}

		out = append(out, f)
	}

	return out, rows.Err()
}

func scanOldFact(rows *sql.Rows) (journal.Fact, error) {
	var (
		f      journal.Fact
		ms     int64
		detail string
	)
	if err := rows.Scan(&f.Seq, &ms, &f.TaskID, &f.Type, &f.Owner, &f.Attempt, &f.Error, &detail); err != nil {
		return journal.Fact{}, err
	}

	f.Time = msToTime(ms)
	if detail != "" {
		f.Detail = jsontextValue(detail)
	}

	return f, nil
}

func oldHeadSeq(ctx context.Context, src *sql.DB) (int64, error) {
	var head sql.NullInt64
	if err := src.QueryRowContext(ctx, `SELECT MAX(seq) FROM facts`).Scan(&head); err != nil {
		return 0, err
	}

	return head.Int64, nil
}

func oldTaskIDs(ctx context.Context, src *sql.DB) ([]string, error) {
	rows, err := src.QueryContext(ctx, `SELECT id FROM tasks ORDER BY id`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var out []string

	for rows.Next() {
		var id string
		if err := rows.Scan(&id); err != nil {
			return nil, err
		}

		out = append(out, id)
	}

	return out, rows.Err()
}

// oldListStatus reads full task rows for one status, mirroring the
// hand-rolled store's scan (NULL lease expiry/completed time → zero).
func oldListStatus(ctx context.Context, src *sql.DB, status task.Status) ([]task.Task, error) {
	rows, err := src.QueryContext(ctx, `
		SELECT id, project, type, payload, deps, priority, attempts, max_attempts,
		       not_before, status, lease_owner, lease_expires, last_error,
		       created_at, updated_at, completed_at, dedup_key
		FROM tasks WHERE status = ?`, string(status))
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var out []task.Task

	for rows.Next() {
		t, err := scanOldTask(rows)
		if err != nil {
			return nil, err
		}

		out = append(out, t)
	}

	return out, rows.Err()
}

// --- small shared helpers ---

func equalMaps[K comparable, V comparable](a, b map[K]V) bool {
	if len(a) != len(b) {
		return false
	}

	for k, v := range a {
		if other, ok := b[k]; !ok || other != v {
			return false
		}
	}

	return true
}

func formatCounts(counts map[task.Status]int) string {
	names := make([]string, 0, len(counts))
	total := 0

	for st, n := range counts {
		names = append(names, fmt.Sprintf("%s=%d", st, n))
		total += n
	}

	sort.Strings(names)

	return fmt.Sprintf("total=%d %s", total, strings.Join(names, " "))
}

func deadStatus() *task.Status {
	dead := task.Dead

	return &dead
}
