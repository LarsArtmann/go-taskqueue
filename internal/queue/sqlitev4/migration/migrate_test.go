package migration

import (
	"context"
	"database/sql"
	"errors"
	"os"
	"path/filepath"
	"strconv"
	"testing"
	"time"

	"github.com/larsartmann/go-taskqueue/internal/queue/sqlitev4"
	_ "modernc.org/sqlite" // pure-Go SQLite driver (CGo-free)
)

// legacySchema is the pre-flip hand-rolled store's DDL, verbatim from
// f0643178^ (internal/queue/sqlite/sqlite.go const schema): the engine
// schema minus lease_token, plus the legacy-only facts_archive and
// journal_meta tables. It exists so tests can build REAL legacy fixtures
// — the hand-rolled producer is deleted, and seeding through the thin
// driver would produce an engine-shaped file, not a legacy one.
const legacySchema = `
CREATE TABLE IF NOT EXISTS tasks (
	id             TEXT PRIMARY KEY,
	project        TEXT NOT NULL DEFAULT '',
	type           TEXT NOT NULL,
	payload        TEXT NOT NULL DEFAULT '',
	deps           TEXT NOT NULL DEFAULT '[]',
	priority       INTEGER NOT NULL DEFAULT 0,
	attempts       INTEGER NOT NULL DEFAULT 0,
	max_attempts   INTEGER NOT NULL DEFAULT 3,
	not_before     INTEGER NOT NULL DEFAULT 0,
	status         TEXT NOT NULL DEFAULT 'pending',
	lease_owner    TEXT NOT NULL DEFAULT '',
	lease_expires  INTEGER,
	last_error     TEXT NOT NULL DEFAULT '',
	created_at     INTEGER NOT NULL,
	updated_at     INTEGER NOT NULL,
	completed_at   INTEGER,
	dedup_key      TEXT NOT NULL DEFAULT ''
);
CREATE TABLE IF NOT EXISTS deps (
	task_id TEXT NOT NULL REFERENCES tasks(id) ON DELETE CASCADE,
	dep_id  TEXT NOT NULL REFERENCES tasks(id) ON DELETE CASCADE,
	PRIMARY KEY (task_id, dep_id)
);
CREATE TABLE IF NOT EXISTS facts (
	seq      INTEGER PRIMARY KEY AUTOINCREMENT,
	time     INTEGER NOT NULL,
	task_id  TEXT NOT NULL,
	type     TEXT NOT NULL,
	owner    TEXT NOT NULL DEFAULT '',
	attempt  INTEGER NOT NULL DEFAULT 0,
	error    TEXT NOT NULL DEFAULT '',
	detail   TEXT NOT NULL DEFAULT ''
);
CREATE TABLE IF NOT EXISTS facts_archive (
	seq      INTEGER PRIMARY KEY,
	time     INTEGER NOT NULL,
	task_id  TEXT NOT NULL,
	type     TEXT NOT NULL,
	owner    TEXT NOT NULL DEFAULT '',
	attempt  INTEGER NOT NULL DEFAULT 0,
	error    TEXT NOT NULL DEFAULT '',
	detail   TEXT NOT NULL DEFAULT ''
);
CREATE TABLE IF NOT EXISTS journal_meta (
	key   TEXT PRIMARY KEY,
	value TEXT NOT NULL
);
CREATE TABLE IF NOT EXISTS watermarks (
	consumer   TEXT PRIMARY KEY,
	seq        INTEGER NOT NULL,
	updated_at INTEGER NOT NULL
);
CREATE TABLE IF NOT EXISTS priority_scores (
	item_key       TEXT PRIMARY KEY,
	score          INTEGER NOT NULL,
	effort_minutes INTEGER NOT NULL,
	source         TEXT NOT NULL,
	reasoning      TEXT NOT NULL,
	tokens         INTEGER NOT NULL,
	scored_at      INTEGER NOT NULL
);`

// seedLegacyJournal writes a representative dogfood-shaped LEGACY journal
// with raw SQL: four tasks (completed, dead, pending, parked), eleven
// facts including a tq-side session fact, one dep edge, a watermark, a
// priority score, and rows in the legacy-only archive/meta tables.
func seedLegacyJournal(t *testing.T, path string) {
	t.Helper()

	db, err := sql.Open("sqlite", copyDSN(path))
	if err != nil {
		t.Fatalf("open legacy fixture: %v", err)
	}

	defer func() { _ = db.Close() }()

	if _, err := db.Exec(legacySchema); err != nil {
		t.Fatalf("create legacy schema: %v", err)
	}

	now := time.Now().UnixMilli()
	future := time.Now().Add(time.Hour).UnixMilli()
	ms := func(n int64) string { return strconv.FormatInt(n, 10) }

	tasks := []string{
		`INSERT INTO tasks (id, project, type, payload, deps, priority, attempts, max_attempts,
			not_before, status, lease_owner, lease_expires, last_error, created_at, updated_at, completed_at, dedup_key)
		VALUES ('t-completed', 'go-taskqueue', 'agent', '{"repo":"go-taskqueue","prompt":"do a thing"}', '[]', 0, 1, 3,
			0, 'completed', '', NULL, '', ` + ms(now-9000) + `, ` + ms(now-8000) + `, ` + ms(now-8000) + `, '')`,
		`INSERT INTO tasks (id, project, type, payload, deps, priority, attempts, max_attempts,
			not_before, status, lease_owner, lease_expires, last_error, created_at, updated_at, completed_at, dedup_key)
		VALUES ('t-dead', 'go-taskqueue', 'agent', '{"repo":"go-taskqueue","prompt":"boom"}', '[]', 0, 1, 1,
			0, 'dead', '', NULL, 'verify failed', ` + ms(now-7000) + `, ` + ms(now-6000) + `, NULL, 'go-taskqueue:boom')`,
		`INSERT INTO tasks (id, project, type, payload, deps, priority, attempts, max_attempts,
			not_before, status, lease_owner, lease_expires, last_error, created_at, updated_at, completed_at, dedup_key)
		VALUES ('t-pending', 'overview', 'sh', '"echo hi"', '[]', 5, 0, 3,
			0, 'pending', '', NULL, '', ` + ms(now-5000) + `, ` + ms(now-4000) + `, NULL, '')`,
		`INSERT INTO tasks (id, project, type, payload, deps, priority, attempts, max_attempts,
			not_before, status, lease_owner, lease_expires, last_error, created_at, updated_at, completed_at, dedup_key)
		VALUES ('t-parked', 'overview', 'sh', '"echo parked"', '[]', 0, 0, 3,
			` + ms(future) + `, 'pending', '', NULL, '', ` + ms(now-3000) + `, ` + ms(now-3000) + `, NULL, '')`,
	}

	for _, stmt := range tasks {
		if _, err := db.Exec(stmt); err != nil {
			t.Fatalf("seed task: %v\nstmt: %s", err, stmt)
		}
	}

	other := []string{
		`INSERT INTO deps (task_id, dep_id) VALUES ('t-pending', 't-completed')`,
		`INSERT INTO facts (time, task_id, type, detail) VALUES (` + ms(
			now-9000,
		) + `, 't-completed', 'task.enqueued', '{"project":"go-taskqueue","type":"agent"}')`,
		`INSERT INTO facts (time, task_id, type, owner, attempt) VALUES (` + ms(
			now-8500,
		) + `, 't-completed', 'task.claimed', 'worker-1', 1)`,
		`INSERT INTO facts (time, task_id, type, owner, attempt, detail) VALUES (` + ms(
			now-8000,
		) + `, 't-completed', 'task.completed', 'worker-1', 1, '{"ok":true}')`,
		`INSERT INTO facts (time, task_id, type, detail) VALUES (` + ms(
			now-7000,
		) + `, 't-dead', 'task.enqueued', '{"project":"go-taskqueue","type":"agent"}')`,
		`INSERT INTO facts (time, task_id, type, owner, attempt) VALUES (` + ms(
			now-6500,
		) + `, 't-dead', 'task.claimed', 'worker-2', 1)`,
		`INSERT INTO facts (time, task_id, type, owner, attempt, error, detail) VALUES (` + ms(
			now-6200,
		) + `, 't-dead', 'task.failed', 'worker-2', 1, 'verify failed', '{"stage":"verify"}')`,
		`INSERT INTO facts (time, task_id, type, owner, attempt, detail) VALUES (` + ms(
			now-6000,
		) + `, 't-dead', 'task.dead-lettered', 'worker-2', 1, '{"class":"exhausted"}')`,
		`INSERT INTO facts (time, task_id, type, detail) VALUES (` + ms(
			now-5000,
		) + `, 't-pending', 'task.enqueued', '{"project":"overview","type":"sh"}')`,
		`INSERT INTO facts (time, task_id, type, detail) VALUES (` + ms(
			now-4500,
		) + `, 't-pending', 'task.reprioritized', '{"from":0,"to":5,"source":"manual","reason":"test"}')`,
		`INSERT INTO facts (time, task_id, type) VALUES (` + ms(now-3000) + `, 't-parked', 'task.enqueued')`,
		`INSERT INTO facts (time, task_id, type, detail) VALUES (` + ms(
			now-2000,
		) + `, 'session:abc123', 'session.opened', '{"repo":"go-taskqueue"}')`,
		`INSERT INTO watermarks (consumer, seq, updated_at) VALUES ('papdashboard:http://pap', 3, ` + ms(
			now-1000,
		) + `)`,
		`INSERT INTO priority_scores (item_key, score, effort_minutes, source, reasoning, tokens, scored_at)
			VALUES ('go-taskqueue:some item', 80, 30, 'ai:batch-scorer', 'high impact', 500, ` + ms(now-1500) + `)`,
		`INSERT INTO facts_archive (seq, time, task_id, type) VALUES (1, ` + ms(
			now-8000,
		) + `, 't-ancient', 'task.enqueued')`,
		`INSERT INTO journal_meta (key, value) VALUES ('schema_version', '4')`,
	}

	for _, stmt := range other {
		if _, err := db.Exec(stmt); err != nil {
			t.Fatalf("seed row: %v\nstmt: %s", err, stmt)
		}
	}
}

func TestReplayRoundTripProjectionEquality(t *testing.T) {
	ctx := context.Background()

	from := filepath.Join(t.TempDir(), "old.db")
	to := filepath.Join(t.TempDir(), "new.db")

	seedLegacyJournal(t, from)

	stats, err := Migrate(ctx, from, to)
	if err != nil {
		t.Fatalf("migrate: %v", err)
	}

	if stats.Tasks != 4 || stats.Deps != 1 || stats.Facts != 11 || stats.Watermarks != 1 ||
		stats.PriorityScores != 1 || stats.FactsArchive != 1 || stats.JournalMeta != 1 {
		t.Fatalf("unexpected stats: %+v", stats)
	}

	report, err := Verify(ctx, from, to)
	if err != nil {
		t.Fatalf("verify: %v", err)
	}

	for _, s := range report.Sections {
		if !s.OK {
			t.Errorf("projection %q mismatched: %s", s.Name, s.Detail)
		}
	}

	if !report.OK() {
		t.Logf("report:\n%s", report.Summary())
	}
}

func TestVerifyDetectsTamperedTarget(t *testing.T) {
	ctx := context.Background()

	from := filepath.Join(t.TempDir(), "old.db")
	to := filepath.Join(t.TempDir(), "new.db")

	seedLegacyJournal(t, from)

	if _, err := Migrate(ctx, from, to); err != nil {
		t.Fatalf("migrate: %v", err)
	}

	db, err := sql.Open("sqlite", copyDSN(to))
	if err != nil {
		t.Fatalf("open target: %v", err)
	}

	if _, err := db.ExecContext(ctx, `UPDATE tasks SET status = 'cancelled' WHERE status = 'dead'`); err != nil {
		t.Fatalf("tamper: %v", err)
	}

	if err := db.Close(); err != nil {
		t.Fatalf("close target: %v", err)
	}

	report, err := Verify(ctx, from, to)
	if err != nil {
		t.Fatalf("verify: %v", err)
	}

	if report.OK() {
		t.Fatalf("tampered target passed verification:\n%s", report.Summary())
	}

	var statusOrDLQFailed bool

	for _, s := range report.Sections {
		if !s.OK && (s.Name == "status counts" || s.Name == "dlq") {
			statusOrDLQFailed = true
		}
	}

	if !statusOrDLQFailed {
		t.Fatalf("expected status-counts or dlq section to catch the tamper:\n%s", report.Summary())
	}
}

func TestVerifyDetectsDeletedFact(t *testing.T) {
	ctx := context.Background()

	from := filepath.Join(t.TempDir(), "old.db")
	to := filepath.Join(t.TempDir(), "new.db")

	seedLegacyJournal(t, from)

	if _, err := Migrate(ctx, from, to); err != nil {
		t.Fatalf("migrate: %v", err)
	}

	db, err := sql.Open("sqlite", copyDSN(to))
	if err != nil {
		t.Fatalf("open target: %v", err)
	}

	if _, err := db.ExecContext(ctx, `DELETE FROM facts WHERE seq = 1`); err != nil {
		t.Fatalf("tamper: %v", err)
	}

	if err := db.Close(); err != nil {
		t.Fatalf("close target: %v", err)
	}

	report, err := Verify(ctx, from, to)
	if err != nil {
		t.Fatalf("verify: %v", err)
	}

	if report.OK() {
		t.Fatalf("deleted fact passed verification:\n%s", report.Summary())
	}
}

func TestMigrateRefusesExistingTarget(t *testing.T) {
	ctx := context.Background()

	dir := t.TempDir()
	from := filepath.Join(dir, "old.db")
	to := filepath.Join(dir, "new.db")

	seedLegacyJournal(t, from)

	if _, err := Migrate(ctx, from, to); err != nil {
		t.Fatalf("first migrate: %v", err)
	}

	if _, err := Migrate(ctx, from, to); err == nil {
		t.Fatal("second migrate into existing target must fail")
	}
}

func TestVerifyMatchesSQLiteV4Store(t *testing.T) {
	ctx := context.Background()

	from := filepath.Join(t.TempDir(), "old.db")
	to := filepath.Join(t.TempDir(), "new.db")

	seedLegacyJournal(t, from)

	if _, err := Migrate(ctx, from, to); err != nil {
		t.Fatalf("migrate: %v", err)
	}

	store, err := sqlitev4.Open(to)
	if err != nil {
		t.Fatalf("open replayed store: %v", err)
	}
	defer store.Close()

	claimed, _, err := store.ClaimDue(ctx, "worker-after-cutover", time.Minute)
	if err != nil {
		t.Fatalf("claim from replayed store: %v", err)
	}

	if claimed.Type != "sh" {
		t.Fatalf("claimed task %s type %q, want the pending sh task", claimed.ID, claimed.Type)
	}

	if _, _, err = store.ClaimDue(ctx, "worker-after-cutover", time.Minute); err == nil {
		t.Fatal("second claim should find nothing due (parked NotBefore)")
	}
}

func TestProbeClassifiesSchemaGenerations(t *testing.T) {
	dir := t.TempDir()

	missing := filepath.Join(dir, "missing.db")
	if kind, err := Probe(missing); err != nil || kind != KindFresh {
		t.Fatalf("probe missing file: kind=%v err=%v, want fresh/nil", kind, err)
	}

	fresh := filepath.Join(dir, "fresh.db")
	if _, err := sqlitev4.Open(fresh); err != nil {
		t.Fatalf("open fresh store: %v", err)
	}

	if kind, err := Probe(fresh); err != nil || kind != KindEngine {
		t.Fatalf("probe engine file: kind=%v err=%v, want engine/nil", kind, err)
	}

	legacy := filepath.Join(dir, "legacy.db")
	seedLegacyJournal(t, legacy)

	if kind, err := Probe(legacy); err != nil || kind != KindLegacy {
		t.Fatalf("probe legacy file: kind=%v err=%v, want legacy/nil", kind, err)
	}
}

func TestUpgradeIfNeededConvergesLegacyInPlace(t *testing.T) {
	ctx := context.Background()

	dir := t.TempDir()
	path := filepath.Join(dir, "tq.db")

	seedLegacyJournal(t, path)

	result, err := UpgradeIfNeeded(ctx, path)
	if err != nil {
		t.Fatalf("auto-upgrade: %v", err)
	}

	if result == nil {
		t.Fatal("auto-upgrade result nil for a legacy db")
	}

	if result.Kind != KindLegacy {
		t.Fatalf("result kind %v, want legacy", result.Kind)
	}

	if !result.Report.OK() {
		t.Fatalf("converged db failed verification:\n%s", result.Report.Summary())
	}

	if _, err := os.Stat(result.BackupPath); err != nil {
		t.Fatalf("snapshot not kept: %v", err)
	}

	// The upgraded file is now engine-generation: a second open is a no-op.
	kind, err := Probe(path)
	if err != nil || kind != KindEngine {
		t.Fatalf("post-upgrade probe: kind=%v err=%v, want engine/nil", kind, err)
	}

	if again, err := UpgradeIfNeeded(ctx, path); err != nil || again != nil {
		t.Fatalf("second upgrade: result=%v err=%v, want nil/nil", again, err)
	}

	// The converged store is a LIVE queue: the pending task is claimable
	// (its dep completed), the parked one is not due.
	store, err := sqlitev4.Open(path)
	if err != nil {
		t.Fatalf("open converged store: %v", err)
	}
	defer store.Close()

	claimed, _, err := store.ClaimDue(ctx, "worker-post-upgrade", time.Minute)
	if err != nil {
		t.Fatalf("claim from converged store: %v", err)
	}

	if claimed.Type != "sh" {
		t.Fatalf("claimed %s type %q, want the pending sh task", claimed.ID, claimed.Type)
	}
}

func TestUpgradeIfNeededRefusesKillSwitch(t *testing.T) {
	ctx := context.Background()

	path := filepath.Join(t.TempDir(), "tq.db")
	seedLegacyJournal(t, path)

	t.Setenv("TQ_NO_AUTO_UPGRADE", "1")

	_, err := UpgradeIfNeeded(ctx, path)
	if !errors.Is(err, ErrAutoUpgradeDisabled) {
		t.Fatalf("err %v, want ErrAutoUpgradeDisabled", err)
	}

	// The legacy file must be untouched after the refusal.
	if kind, probeErr := Probe(path); probeErr != nil || kind != KindLegacy {
		t.Fatalf("post-refusal probe: kind=%v err=%v, want legacy/nil", kind, probeErr)
	}
}
