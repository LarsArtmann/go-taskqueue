package executor

import (
	"context"
	"database/sql"
	"encoding/json/v2"
	"os"
	"path/filepath"
	"testing"

	"github.com/larsartmann/go-taskqueue/internal/task"
	_ "modernc.org/sqlite" // fixture crush.db driver
)

// fixtureCrushDB writes the minimal sessions schema go-crush-data reads
// (every optional, probed column omitted — capability substitution fills
// them) into <dataDir>/crush.db and returns the handle.
func fixtureCrushDB(t *testing.T, dataDir string) *sql.DB {
	t.Helper()

	db, err := sql.Open("sqlite", filepath.Join(dataDir, "crush.db"))
	if err != nil {
		t.Fatal(err)
	}

	t.Cleanup(func() { _ = db.Close() })

	const schema = `CREATE TABLE sessions (
		id TEXT PRIMARY KEY,
		title TEXT,
		message_count INTEGER,
		prompt_tokens INTEGER,
		completion_tokens INTEGER,
		cost REAL,
		updated_at INTEGER,
		created_at INTEGER
	);
	CREATE TABLE messages (
		id TEXT,
		session_id TEXT,
		role TEXT,
		parts TEXT,
		created_at INTEGER,
		updated_at INTEGER
	)`

	if _, err := db.ExecContext(context.Background(), schema); err != nil {
		t.Fatalf("fixture schema: %v", err)
	}

	return db
}

func seedFixtureSession(t *testing.T, db *sql.DB, id string) {
	t.Helper()

	const insert = `INSERT INTO sessions
		(id, title, message_count, prompt_tokens, completion_tokens, cost, updated_at, created_at)
		VALUES (?, ?, 7, 1200, 3400, 0.42, 1790000000, 1790000000)`

	if _, err := db.ExecContext(context.Background(), insert, id, "fixture session"); err != nil {
		t.Fatalf("seed session: %v", err)
	}
}

// fixtureRegistryJSON renders a one-project registry mapping repo → dataDir.
func fixtureRegistryJSON(t *testing.T, repo, dataDir string) []byte {
	t.Helper()

	raw, err := json.Marshal(map[string]any{
		"projects": []map[string]any{
			{"path": repo, "data_dir": dataDir, "last_accessed": "2026-09-14T00:00:00Z"},
		},
	})
	if err != nil {
		t.Fatal(err)
	}

	return raw
}

// TestDeriveOutcomeSessionStats pins the go-crush-data enrichment: the
// run's session is looked up through a fixture registry and its usage row
// lands in the derivation.
func TestDeriveOutcomeSessionStats(t *testing.T) {
	repo := t.TempDir()
	global := t.TempDir()
	data := t.TempDir()

	registryPath := filepath.Join(global, "projects.json")

	if err := os.WriteFile(registryPath, fixtureRegistryJSON(t, repo, data), 0o600); err != nil {
		t.Fatal(err)
	}

	t.Setenv("CRUSH_GLOBAL_DATA", global)

	db := fixtureCrushDB(t, data)
	seedFixtureSession(t, db, "sess-42")

	got := deriveOutcome(context.Background(), repo, "sess-42", task.ID("000001a0testid000000000000"))

	if got.SessionCostUSD != 0.42 || got.SessionPromptTokens != 1200 || got.SessionCompletionTokens != 3400 {
		t.Fatalf("session usage = %+v, want the seeded row", got)
	}

	if got.SessionMessageCount != 7 {
		t.Fatalf("message count = %d, want 7", got.SessionMessageCount)
	}
}

func TestDeriveOutcomeUnknownSessionLeavesZero(t *testing.T) {
	repo := t.TempDir()
	global := t.TempDir()
	registryPath := filepath.Join(global, "projects.json")

	if err := os.WriteFile(registryPath, fixtureRegistryJSON(t, repo, global), 0o600); err != nil {
		t.Fatal(err)
	}

	t.Setenv("CRUSH_GLOBAL_DATA", global)

	got := deriveOutcome(context.Background(), repo, "no-such-session", task.ID("000001a0testid000000000000"))

	if got.SessionCostUSD != 0 || got.SessionMessageCount != 0 {
		t.Fatalf("unknown session must leave zeros, got %+v", got)
	}
}
