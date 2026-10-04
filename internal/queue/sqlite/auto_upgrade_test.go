package sqlite

import (
	"context"
	"database/sql"
	"path/filepath"
	"testing"
	"time"

	"github.com/larsartmann/go-taskqueue/internal/task"
	_ "modernc.org/sqlite" // pure-Go SQLite driver (CGo-free)
)

// TestOpenAutoUpgradesLegacyDatabase pins the P1 wiring: the facade Open
// probes for the pre-flip hand-rolled schema (tasks WITHOUT lease_token),
// upgrades it in place under the migration package's verified safety net,
// and serves a live engine store. The projection-equality details are
// pinned in internal/queue/sqlitev4/migration; this test proves the seam.
func TestOpenAutoUpgradesLegacyDatabase(t *testing.T) {
	ctx := context.Background()

	path := filepath.Join(t.TempDir(), "tq.db")

	seedMinimalLegacy(t, path)

	store, err := Open(path)
	if err != nil {
		t.Fatalf("open legacy db through the facade: %v", err)
	}
	defer store.Close()

	// The pending task survived the upgrade and is claimable.
	got, _, err := store.ClaimDue(ctx, "worker-post-upgrade", time.Minute)
	if err != nil {
		t.Fatalf("claim after auto-upgrade: %v", err)
	}

	if got.ID != "t-pending" || got.Type != "sh" {
		t.Fatalf("claimed %+v, want the legacy pending sh task", got)
	}

	// New enqueues work and the store reports the history: the legacy
	// enqueued fact + the claim fact minted above.
	head, err := store.HeadSeq(ctx)
	if err != nil {
		t.Fatalf("head seq: %v", err)
	}

	if head != 2 {
		t.Fatalf("head seq %d, want 2 (legacy fact + claim)", head)
	}
}

// TestOpenFreshDatabaseStillWorks guards the no-op path: a brand-new file
// opens without any upgrade machinery interfering.
func TestOpenFreshDatabaseStillWorks(t *testing.T) {
	ctx := context.Background()

	path := filepath.Join(t.TempDir(), "fresh.db")

	store, err := Open(path)
	if err != nil {
		t.Fatalf("open fresh db: %v", err)
	}
	defer store.Close()

	created, err := store.Enqueue(ctx, task.New{Project: "p", Type: "sh", Payload: nil})
	if err != nil {
		t.Fatalf("enqueue on fresh db: %v", err)
	}

	if created.Status != task.Pending {
		t.Fatalf("status %q, want pending", created.Status)
	}
}

// seedMinimalLegacy writes the smallest honest pre-flip database: the
// hand-rolled DDL (tasks without lease_token, facts, watermarks) with one
// pending task and its enqueued fact.
func seedMinimalLegacy(t *testing.T, path string) {
	t.Helper()

	db, err := sql.Open("sqlite", "file:"+path+"?_pragma=journal_mode(WAL)")
	if err != nil {
		t.Fatalf("open legacy fixture: %v", err)
	}

	defer func() { _ = db.Close() }()

	const ddl = `
CREATE TABLE tasks (
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
CREATE TABLE facts (
	seq      INTEGER PRIMARY KEY AUTOINCREMENT,
	time     INTEGER NOT NULL,
	task_id  TEXT NOT NULL,
	type     TEXT NOT NULL,
	owner    TEXT NOT NULL DEFAULT '',
	attempt  INTEGER NOT NULL DEFAULT 0,
	error    TEXT NOT NULL DEFAULT '',
	detail   TEXT NOT NULL DEFAULT ''
);
CREATE TABLE watermarks (
	consumer   TEXT PRIMARY KEY,
	seq        INTEGER NOT NULL,
	updated_at INTEGER NOT NULL
);
CREATE TABLE priority_scores (
	item_key       TEXT PRIMARY KEY,
	score          INTEGER NOT NULL,
	effort_minutes INTEGER NOT NULL,
	source         TEXT NOT NULL,
	reasoning      TEXT NOT NULL,
	tokens         INTEGER NOT NULL,
	scored_at      INTEGER NOT NULL
);`

	if _, err := db.Exec(ddl); err != nil {
		t.Fatalf("legacy ddl: %v", err)
	}

	now := time.Now().UnixMilli()

	_, err = db.Exec(
		`INSERT INTO tasks (id, project, type, payload, status, created_at, updated_at)
		 VALUES ('t-pending', 'p', 'sh', '"echo hi"', 'pending', ?, ?)`, now, now)
	if err != nil {
		t.Fatalf("seed task: %v", err)
	}

	if _, err := db.Exec(
		`INSERT INTO facts (time, task_id, type, detail) VALUES (?, 't-pending', 'task.enqueued', '{}')`, now,
	); err != nil {
		t.Fatalf("seed fact: %v", err)
	}
}
