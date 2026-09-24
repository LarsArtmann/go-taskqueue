// Package replay implements the ADR-0019 S1 data-migration tool: it
// replays a tq fact journal — the hand-rolled store's database — into a
// fresh go-cqrs-lite engine store (the sqlitev4 adapter over
// queue/sqlite/v4) and verifies projection equality (StatusCounts,
// ProjectCounts, the full fact stream, head seq, per-task fact tails,
// DLQ contents, watermarks, priority scores). It is S1's
// definition-of-done gate for the dogfood cutover (ADR-0019 §Data
// migration; C12/M056 in
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
// defect, not replay fuzz. The enqueued fact has since grown the full
// task snapshot (EnqueueDetail payload/deps/max_attempts/not_before/
// created_at, 2026-09-24), so post-growth journals could drive a
// transition applier — but every pre-growth journal (including the
// production dogfood journal's history) still needs this copy, and the
// upstream engine's own enqueued detail stays thin until the M4
// ratification lands. The copy remains until both are true.
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

	sectionStatusCounts  = "status counts"
	sectionProjectCounts = "project counts"
	sectionDLQ           = "dlq"
	sectionWatermarks    = "watermarks"
	sectionPriority      = "priority scores"
	sectionFactStream    = "fact stream"
	sectionHeadSeq       = "head seq"
	sectionTaskTails     = "task tails"
)

// Stats reports how many rows the migration copied per table.
type Stats struct {
	Tasks          int
	Deps           int
	Facts          int
	Watermarks     int
	PriorityScores int
	FactsArchive   int
	JournalMeta    int
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
func (report Report) OK() bool {
	for _, section := range report.Sections {
		if !section.OK {
			return false
		}
	}

	return true
}

// Summary renders the report for terminal output.
func (report Report) Summary() string {
	var out strings.Builder

	for _, section := range report.Sections {
		verdict := "ok"
		if !section.OK {
			verdict = "MISMATCH"
		}

		fmt.Fprintf(&out, "%-18s %-9s %s\n", section.Name, verdict, section.Detail)
	}

	return out.String()
}

// copyDSN opens a SQLite file for the migration copy (single writer, WAL).
func copyDSN(path string) string {
	return fmt.Sprintf("file:%s?_pragma=journal_mode(WAL)&_pragma=busy_timeout(5000)&_pragma=foreign_keys(1)", path)
}

// readOnlyDSN opens a SQLite file read-only: the migration never writes
// to the source journal.
func readOnlyDSN(path string) string {
	return fmt.Sprintf("file:%s?mode=ro&_pragma=busy_timeout(5000)", path)
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

	defer func() { _ = src.Close() }()

	// Open+close the target through the real adapter once: it creates the
	// engine schema AND the companion tables, exactly as a cutover store
	// would boot.
	boot, err := sqlitev4.Open(toPath) //nolint:contextcheck // the adapter's Open bootstraps its own migration
	if err != nil {
		return stats, fmt.Errorf("replay: bootstrap engine store: %w", err)
	}

	if err := boot.Close(); err != nil {
		return stats, fmt.Errorf("replay: close bootstrap store: %w", err)
	}

	dst, err := sql.Open("sqlite", copyDSN(toPath))
	if err != nil {
		return stats, fmt.Errorf("replay: open target: %w", err)
	}

	defer func() { _ = dst.Close() }()

	copyTx, err := dst.BeginTx(ctx, nil)
	if err != nil {
		return stats, fmt.Errorf("replay: begin copy tx: %w", err)
	}

	defer func() { _ = copyTx.Rollback() }()

	if err := copyAllTables(ctx, src, copyTx, &stats); err != nil {
		return stats, err
	}

	if err := copyTx.Commit(); err != nil {
		return stats, fmt.Errorf("replay: commit copy: %w", err)
	}

	return stats, nil
}

// copyAllTables copies every table in FK-safe order inside the caller's
// transaction: tasks first (deps reference them), then everything else.
func copyAllTables(ctx context.Context, src *sql.DB, copyTx *sql.Tx, stats *Stats) error {
	copiers := []struct {
		name string
		run  func(context.Context, *sql.DB, *sql.Tx) (int, error)
	}{
		{name: "tasks", run: copyTasks},
		{name: "deps", run: copyDeps},
		{name: "facts", run: copyFacts},
		{name: "watermarks", run: copyWatermarks},
		{name: "priority_scores", run: copyPriorityScores},
		{name: "facts_archive", run: copyLegacyFactsArchive},
		{name: "journal_meta", run: copyLegacyJournalMeta},
	}

	for _, copier := range copiers {
		copied, err := copier.run(ctx, src, copyTx)
		if err != nil {
			return fmt.Errorf("replay: %s: %w", copier.name, err)
		}

		setStat(stats, copier.name, copied)
	}

	return nil
}

func setStat(stats *Stats, table string, copied int) {
	switch table {
	case "tasks":
		stats.Tasks = copied
	case "deps":
		stats.Deps = copied
	case "facts":
		stats.Facts = copied
	case "watermarks":
		stats.Watermarks = copied
	case "priority_scores":
		stats.PriorityScores = copied
	case "facts_archive":
		stats.FactsArchive = copied
	case "journal_meta":
		stats.JournalMeta = copied
	}
}

// copyTasks copies task rows with the two schema conversions: payload
// TEXT→BLOB and lease_token NULL (the old schema has no fencing token).
func copyTasks(ctx context.Context, src *sql.DB, copyTx *sql.Tx) (int, error) {
	rows, err := src.QueryContext(ctx, `
		SELECT id, project, type, payload, deps, priority, attempts, max_attempts,
		       not_before, status, lease_owner, lease_expires, last_error,
		       created_at, updated_at, completed_at, dedup_key
		FROM tasks ORDER BY created_at ASC, id ASC`)
	if err != nil {
		return 0, err
	}

	defer func() { _ = rows.Close() }()

	const insert = `
		INSERT INTO tasks (id, project, type, payload, deps, priority, attempts, max_attempts,
		                   not_before, status, lease_owner, lease_expires, lease_token, last_error,
		                   created_at, updated_at, completed_at, dedup_key)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, NULL, ?, ?, ?, ?, ?)`

	copied := 0

	for rows.Next() {
		var (
			id, project, typ, payload, deps, status, leaseOwner, lastError, dedupKey string
			priority, attempts, maxAttempts, notBefore, createdAt, updatedAt         int64
			leaseExpires, completedAt                                                sql.NullInt64
		)
		if err := rows.Scan(&id, &project, &typ, &payload, &deps, &priority, &attempts,
			&maxAttempts, &notBefore, &status, &leaseOwner, &leaseExpires, &lastError,
			&createdAt, &updatedAt, &completedAt, &dedupKey); err != nil {
			return 0, err
		}

		if _, err := copyTx.ExecContext(ctx, insert,
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
func copyDeps(ctx context.Context, src *sql.DB, copyTx *sql.Tx) (int, error) {
	rows, err := src.QueryContext(ctx, `SELECT task_id, dep_id FROM deps ORDER BY task_id, dep_id`)
	if err != nil {
		return 0, err
	}

	defer func() { _ = rows.Close() }()

	copied := 0

	for rows.Next() {
		var taskID, depID string
		if err := rows.Scan(&taskID, &depID); err != nil {
			return 0, err
		}

		if _, err := copyTx.ExecContext(
			ctx,
			`INSERT INTO deps (task_id, dep_id) VALUES (?, ?)`,
			taskID,
			depID,
		); err != nil {
			return 0, err
		}

		copied++
	}

	return copied, rows.Err()
}

// copyFacts copies the fact journal verbatim, seq numbers included: the
// engine's facts table IS the unified journal (S2), so byte-identical
// history is the whole point.
func copyFacts(ctx context.Context, src *sql.DB, copyTx *sql.Tx) (int, error) {
	const query = `SELECT seq, time, task_id, type, owner, attempt, error, detail FROM facts ORDER BY seq ASC`

	const insert = `INSERT INTO facts (seq, time, task_id, type, owner, attempt, error, detail)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?)`

	return copyQueriedRows(ctx, src, copyTx, query, insert)
}

func copyWatermarks(ctx context.Context, src *sql.DB, copyTx *sql.Tx) (int, error) {
	const query = `SELECT consumer, seq, updated_at FROM watermarks ORDER BY consumer`

	const insert = `INSERT INTO watermarks (consumer, seq, updated_at) VALUES (?, ?, ?)`

	return copyQueriedRows(ctx, src, copyTx, query, insert)
}

func copyPriorityScores(ctx context.Context, src *sql.DB, copyTx *sql.Tx) (int, error) {
	const query = `SELECT item_key, score, effort_minutes, source, reasoning, tokens, scored_at FROM priority_scores ORDER BY item_key`

	const insert = `INSERT INTO priority_scores (item_key, score, effort_minutes, source, reasoning, tokens, scored_at)
		VALUES (?, ?, ?, ?, ?, ?, ?)`

	return copyQueriedRows(ctx, src, copyTx, query, insert)
}

// copyLegacyFactsArchive recreates the old-only facts_archive table in
// the target verbatim — the engine never reads it, but cutover must not
// lose history. Absent in the source means nothing to carry.
func copyLegacyFactsArchive(ctx context.Context, src *sql.DB, copyTx *sql.Tx) (int, error) {
	const legacyCols = "seq, time, task_id, type, owner, attempt, error, detail"

	return copyLegacyTable(ctx, src, copyTx, "facts_archive", legacyCols)
}

// copyLegacyJournalMeta recreates the old-only journal_meta table (same
// rationale as facts_archive).
func copyLegacyJournalMeta(ctx context.Context, src *sql.DB, copyTx *sql.Tx) (int, error) {
	return copyLegacyTable(ctx, src, copyTx, "journal_meta", "key, value")
}

func copyLegacyTable(ctx context.Context, src *sql.DB, copyTx *sql.Tx, table, cols string) (int, error) {
	var createSQL sql.NullString

	err := src.QueryRowContext(ctx,
		`SELECT sql FROM sqlite_master WHERE type = 'table' AND name = ?`, table).Scan(&createSQL)
	if errors.Is(err, sql.ErrNoRows) {
		return 0, nil
	}

	if err != nil {
		return 0, err
	}

	if _, err := copyTx.ExecContext(ctx, createSQL.String); err != nil {
		return 0, err
	}

	query := fmt.Sprintf(`SELECT %s FROM %s ORDER BY 1 ASC`, cols, table)
	insert := fmt.Sprintf("INSERT INTO %s (%s) VALUES (%s)", table, cols,
		strings.TrimSuffix(strings.Repeat("?, ", strings.Count(cols, ",")+1), ", "))

	return copyQueriedRows(ctx, src, copyTx, query, insert)
}

// copyQueriedRows copies every row of a fixed query into a fixed insert,
// column-for-column, converting nothing.
func copyQueriedRows(ctx context.Context, src *sql.DB, copyTx *sql.Tx, query, insert string) (int, error) {
	rows, err := src.QueryContext(ctx, query)
	if err != nil {
		return 0, err
	}

	defer func() { _ = rows.Close() }()

	columns := strings.Count(insert, "?")
	dest := make([]any, 0, columns)

	for range columns {
		dest = append(dest, nil)
	}

	pointers := make([]any, 0, columns)

	for i := range dest {
		pointers = append(pointers, &dest[i])
	}

	copied := 0

	for rows.Next() {
		if err := rows.Scan(pointers...); err != nil {
			return 0, err
		}

		if _, err := copyTx.ExecContext(ctx, insert, dest...); err != nil {
			return 0, err
		}

		copied++
	}

	return copied, rows.Err()
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

	defer func() { _ = src.Close() }()

	target, err := sqlitev4.Open(toPath) //nolint:contextcheck // the adapter's Open bootstraps its own migration
	if err != nil {
		return report, fmt.Errorf("replay: open engine store: %w", err)
	}

	defer func() { _ = target.Close() }()

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

	defer func() { _ = rows.Close() }()

	counts := map[task.Status]int{}

	for rows.Next() {
		var (
			status task.Status
			count  int
		)

		if err := rows.Scan(&status, &count); err != nil {
			return nil, err
		}

		counts[status] = count
	}

	return counts, rows.Err()
}

func mismatch(name, detail string) Section {
	return Section{Name: name, Detail: detail}
}

func match(name, detail string) Section {
	return Section{Name: name, OK: true, Detail: detail}
}

func verifyStatusCounts(ctx context.Context, src *sql.DB, target *sqlitev4.Store) Section {
	sourceCounts, err := oldStatusCounts(ctx, src)
	if err != nil {
		return mismatch(sectionStatusCounts, fmt.Sprintf("source read failed: %v", err))
	}

	targetCounts, err := target.StatusCounts(ctx)
	if err != nil {
		return mismatch(sectionStatusCounts, fmt.Sprintf("target read failed: %v", err))
	}

	if equalMaps(sourceCounts, targetCounts) {
		return match(sectionStatusCounts, formatCounts(sourceCounts))
	}

	return mismatch(sectionStatusCounts,
		fmt.Sprintf("source %v vs target %v", formatCounts(sourceCounts), formatCounts(targetCounts)))
}

func verifyProjectCounts(ctx context.Context, src *sql.DB, target *sqlitev4.Store) Section {
	rows, err := src.QueryContext(ctx, `SELECT project, status, COUNT(*) FROM tasks GROUP BY project, status`)
	if err != nil {
		return mismatch(sectionProjectCounts, fmt.Sprintf("source read failed: %v", err))
	}

	defer func() { _ = rows.Close() }()

	sourceCounts := map[string]map[task.Status]int{}

	for rows.Next() {
		var (
			project string
			status  task.Status
			count   int
		)

		if err := rows.Scan(&project, &status, &count); err != nil {
			return mismatch(sectionProjectCounts, fmt.Sprintf("source scan failed: %v", err))
		}

		if sourceCounts[project] == nil {
			sourceCounts[project] = map[task.Status]int{}
		}

		sourceCounts[project][status] = count
	}

	if err := rows.Err(); err != nil {
		return mismatch(sectionProjectCounts, fmt.Sprintf("source read failed: %v", err))
	}

	targetCounts, err := target.ProjectCounts(ctx)
	if err != nil {
		return mismatch(sectionProjectCounts, fmt.Sprintf("target read failed: %v", err))
	}

	if len(sourceCounts) != len(targetCounts) {
		return mismatch(
			sectionProjectCounts,
			fmt.Sprintf("source has %d projects, target %d", len(sourceCounts), len(targetCounts)),
		)
	}

	for project, source := range sourceCounts {
		if !equalMaps(source, targetCounts[project]) {
			return mismatch(sectionProjectCounts, fmt.Sprintf("project %q differs", project))
		}
	}

	return match(sectionProjectCounts, fmt.Sprintf("%d projects", len(sourceCounts)))
}

func verifyDLQ(ctx context.Context, src *sql.DB, target *sqlitev4.Store) Section {
	sourceTasks, err := oldListStatus(ctx, src, task.Dead)
	if err != nil {
		return mismatch(sectionDLQ, fmt.Sprintf("source read failed: %v", err))
	}

	dead := task.Dead

	targetTasks, err := target.List(ctx, queue.Filter{Status: &dead})
	if err != nil {
		return mismatch(sectionDLQ, fmt.Sprintf("target read failed: %v", err))
	}

	if len(sourceTasks) != len(targetTasks) {
		return mismatch(
			sectionDLQ,
			fmt.Sprintf("source has %d dead tasks, target %d", len(sourceTasks), len(targetTasks)),
		)
	}

	sort.Slice(sourceTasks, func(i, j int) bool { return sourceTasks[i].ID < sourceTasks[j].ID })
	sort.Slice(targetTasks, func(i, j int) bool { return targetTasks[i].ID < targetTasks[j].ID })

	for i := range sourceTasks {
		if !equalTasks(sourceTasks[i], targetTasks[i]) {
			return mismatch(sectionDLQ, fmt.Sprintf("dead task %s differs", sourceTasks[i].ID))
		}
	}

	return match(sectionDLQ, fmt.Sprintf("%d dead tasks identical", len(sourceTasks)))
}

func verifyWatermarks(ctx context.Context, src *sql.DB, target *sqlitev4.Store) Section {
	rows, err := src.QueryContext(ctx, `SELECT consumer, seq, updated_at FROM watermarks ORDER BY consumer`)
	if err != nil {
		return mismatch(sectionWatermarks, fmt.Sprintf("source read failed: %v", err))
	}

	defer func() { _ = rows.Close() }()

	source := map[string]queue.WatermarkEntry{}

	for rows.Next() {
		var entry queue.WatermarkEntry
		if err := rows.Scan(&entry.Consumer, &entry.Seq, &entry.UpdatedAt); err != nil {
			return mismatch(sectionWatermarks, fmt.Sprintf("source scan failed: %v", err))
		}

		source[entry.Consumer] = entry
	}

	if err := rows.Err(); err != nil {
		return mismatch(sectionWatermarks, fmt.Sprintf("source read failed: %v", err))
	}

	entries, err := target.ListWatermarks(ctx)
	if err != nil {
		return mismatch(sectionWatermarks, fmt.Sprintf("target read failed: %v", err))
	}

	targets := make(map[string]queue.WatermarkEntry, len(entries))
	for _, entry := range entries {
		targets[entry.Consumer] = entry
	}

	if len(source) != len(targets) {
		return mismatch(
			sectionWatermarks,
			fmt.Sprintf("source has %d watermarks, target %d", len(source), len(targets)),
		)
	}

	for consumer, sourceEntry := range source {
		if targets[consumer] != sourceEntry {
			return mismatch(sectionWatermarks, fmt.Sprintf("watermark %q differs", consumer))
		}
	}

	return match(sectionWatermarks, fmt.Sprintf("%d watermarks identical", len(source)))
}

func verifyPriorityScores(ctx context.Context, src *sql.DB, target *sqlitev4.Store) Section {
	rows, err := src.QueryContext(
		ctx,
		`SELECT item_key, score, effort_minutes, source, reasoning, tokens, scored_at FROM priority_scores ORDER BY item_key`,
	)
	if err != nil {
		return mismatch(sectionPriority, fmt.Sprintf("source read failed: %v", err))
	}

	defer func() { _ = rows.Close() }()

	var source []queue.PriorityScore

	for rows.Next() {
		var score queue.PriorityScore
		if err := rows.Scan(&score.ItemKey, &score.Score, &score.EffortMinutes,
			&score.Source, &score.Reasoning, &score.Tokens, &score.ScoredAt); err != nil {
			return mismatch(sectionPriority, fmt.Sprintf("source scan failed: %v", err))
		}

		source = append(source, score)
	}

	if err := rows.Err(); err != nil {
		return mismatch(sectionPriority, fmt.Sprintf("source read failed: %v", err))
	}

	targets, err := target.PriorityScores(ctx)
	if err != nil {
		return mismatch(sectionPriority, fmt.Sprintf("target read failed: %v", err))
	}

	slices.SortFunc(source, func(a, b queue.PriorityScore) int { return strings.Compare(a.ItemKey, b.ItemKey) })
	slices.SortFunc(targets, func(a, b queue.PriorityScore) int { return strings.Compare(a.ItemKey, b.ItemKey) })

	if len(source) != len(targets) {
		return mismatch(sectionPriority, fmt.Sprintf("source has %d scores, target %d", len(source), len(targets)))
	}

	for i := range source {
		if source[i] != targets[i] {
			return mismatch(sectionPriority, fmt.Sprintf("score %q differs", source[i].ItemKey))
		}
	}

	return match(sectionPriority, fmt.Sprintf("%d scores identical", len(source)))
}

// verifyFacts compares the complete fact stream (and head seq) plus every
// task's fact tail.
func verifyFacts(ctx context.Context, src *sql.DB, target *sqlitev4.Store) ([]Section, error) {
	sourceFacts, err := oldAllFacts(ctx, src)
	if err != nil {
		return nil, err
	}

	targetFacts, targetHead, err := streamTargetFacts(ctx, target)
	if err != nil {
		return nil, err
	}

	stream := Section{
		Name:   sectionFactStream,
		Detail: fmt.Sprintf("source %d facts, target %d facts", len(sourceFacts), len(targetFacts)),
	}

	if len(sourceFacts) != len(targetFacts) {
		return []Section{stream}, nil
	}

	for i := range sourceFacts {
		if !equalFacts(sourceFacts[i], targetFacts[i]) {
			stream.Detail = fmt.Sprintf("fact seq %d differs", sourceFacts[i].Seq)

			return []Section{stream}, nil
		}
	}

	stream.OK = true
	stream.Detail = fmt.Sprintf("%d facts identical", len(sourceFacts))

	headSection, err := verifyHeadSeq(ctx, src, targetHead)
	if err != nil {
		return []Section{stream}, err
	}

	tailsSection, err := verifyTaskTails(ctx, src, target)
	if err != nil {
		return []Section{stream, headSection}, err
	}

	return []Section{stream, headSection, tailsSection}, nil
}

func streamTargetFacts(ctx context.Context, target *sqlitev4.Store) ([]journal.Fact, int64, error) {
	facts := make([]journal.Fact, 0)
	after := int64(0)

	for {
		page, err := target.Facts(ctx, after, factPageLimit)
		if err != nil {
			return nil, 0, fmt.Errorf("target read: %w", err)
		}

		facts = append(facts, page...)

		if len(page) > 0 {
			after = page[len(page)-1].Seq
		}

		if len(page) < factPageLimit {
			break
		}
	}

	return facts, after, nil
}

func verifyHeadSeq(ctx context.Context, src *sql.DB, targetHead int64) (Section, error) {
	var sourceHead sql.NullInt64
	if err := src.QueryRowContext(ctx, `SELECT MAX(seq) FROM facts`).Scan(&sourceHead); err != nil {
		return Section{Name: sectionHeadSeq}, err
	}

	return Section{
		Name:   sectionHeadSeq,
		OK:     sourceHead.Int64 == targetHead,
		Detail: fmt.Sprintf("source %d, target %d", sourceHead.Int64, targetHead),
	}, nil
}

// verifyTaskTails walks every task (and synthetic fact-only identity,
// e.g. session:<id>) in the source and compares its per-task fact tail
// through the target's FactsForTask.
func verifyTaskTails(ctx context.Context, src *sql.DB, target *sqlitev4.Store) (Section, error) {
	tails, err := oldTailsByTaskID(ctx, src)
	if err != nil {
		return Section{Name: sectionTaskTails}, err
	}

	for taskID, sourceTail := range tails {
		targetTail, err := target.FactsForTask(ctx, taskID, taskTailLimit)
		if err != nil {
			return Section{Name: sectionTaskTails}, fmt.Errorf("target tail for %s: %w", taskID, err)
		}

		slices.SortFunc(sourceTail, func(a, b journal.Fact) int { return int(a.Seq - b.Seq) })
		slices.SortFunc(targetTail, func(a, b journal.Fact) int { return int(a.Seq - b.Seq) })

		if len(sourceTail) != len(targetTail) {
			return mismatch(sectionTaskTails,
				fmt.Sprintf("task %s: source %d facts, target %d", taskID, len(sourceTail), len(targetTail))), nil
		}

		for i := range sourceTail {
			if !equalFacts(sourceTail[i], targetTail[i]) {
				return mismatch(sectionTaskTails,
					fmt.Sprintf("task %s: fact seq %d differs", taskID, sourceTail[i].Seq)), nil
			}
		}
	}

	return match(sectionTaskTails, fmt.Sprintf("%d task tails identical", len(tails))), nil
}

// oldTailsByTaskID groups the source's full fact stream by task id — the
// per-task tails, including fact-only identities (session:<id>, never
// task rows).
func oldTailsByTaskID(ctx context.Context, src *sql.DB) (map[string][]journal.Fact, error) {
	all, err := oldAllFacts(ctx, src)
	if err != nil {
		return nil, err
	}

	tails := make(map[string][]journal.Fact)

	for _, fact := range all {
		tails[fact.TaskID] = append(tails[fact.TaskID], fact)
	}

	return tails, nil
}

// --- source-side readers (the frozen old schema, read-only) ---

func oldAllFacts(ctx context.Context, src *sql.DB) ([]journal.Fact, error) {
	rows, err := src.QueryContext(ctx,
		`SELECT seq, time, task_id, type, owner, attempt, error, detail FROM facts ORDER BY seq ASC`)
	if err != nil {
		return nil, err
	}

	defer func() { _ = rows.Close() }()

	facts := make([]journal.Fact, 0)

	for rows.Next() {
		fact, err := scanOldFact(rows)
		if err != nil {
			return nil, err
		}

		facts = append(facts, fact)
	}

	return facts, rows.Err()
}

func scanOldFact(rows *sql.Rows) (journal.Fact, error) {
	var (
		fact   journal.Fact
		millis int64
		detail string
	)
	if err := rows.Scan(&fact.Seq, &millis, &fact.TaskID, &fact.Type, &fact.Owner,
		&fact.Attempt, &fact.Error, &detail); err != nil {
		return journal.Fact{}, err
	}

	fact.Time = timeFromMillis(millis)
	if detail != "" {
		fact.Detail = jsonText(detail)
	}

	return fact, nil
}

func oldListStatus(ctx context.Context, src *sql.DB, status task.Status) ([]task.Task, error) {
	rows, err := src.QueryContext(ctx, `
		SELECT id, project, type, payload, deps, priority, attempts, max_attempts,
		       not_before, status, lease_owner, lease_expires, last_error,
		       created_at, updated_at, completed_at, dedup_key
		FROM tasks WHERE status = ?`, string(status))
	if err != nil {
		return nil, err
	}

	defer func() { _ = rows.Close() }()

	var tasks []task.Task

	for rows.Next() {
		one, err := scanOldTask(rows)
		if err != nil {
			return nil, err
		}

		tasks = append(tasks, one)
	}

	return tasks, rows.Err()
}

// --- small shared helpers ---

func equalMaps[Key comparable, Value comparable](a, b map[Key]Value) bool {
	if len(a) != len(b) {
		return false
	}

	for key, value := range a {
		if other, ok := b[key]; !ok || other != value {
			return false
		}
	}

	return true
}

func formatCounts(counts map[task.Status]int) string {
	names := make([]string, 0, len(counts))
	total := 0

	for status, count := range counts {
		names = append(names, fmt.Sprintf("%s=%d", status, count))
		total += count
	}

	sort.Strings(names)

	return fmt.Sprintf("total=%d %s", total, strings.Join(names, " "))
}
