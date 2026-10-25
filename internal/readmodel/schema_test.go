package readmodel

import (
	"database/sql"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"
)

func openProbe(t *testing.T, path string) *sql.DB {
	t.Helper()

	db, err := sql.Open("sqlite", path)
	if err != nil {
		t.Fatalf("open probe db: %v", err)
	}

	t.Cleanup(func() { _ = db.Close() })

	return db
}

func createPlannedTable(t *testing.T, path string, cols []string) {
	t.Helper()

	// Self-contained fixture: the create-side handle must be CLOSED by
	// the time the helper returns — ConvergeProjectionHome renames the
	// file, and on Windows a rename over a live sql.DB pool connection
	// fails "being used by another process" no matter how long the
	// snapshot retry waits (test-windows red 2026-10-25, run
	// 40167619168: retry.exhausted against the still-open fixture pool).
	db, err := sql.Open("sqlite", path)
	if err != nil {
		t.Fatalf("open planned-table fixture: %v", err)
	}
	defer func() { _ = db.Close() }()

	decl := "id TEXT PRIMARY KEY"

	var declSb28 strings.Builder

	for _, col := range cols {
		if col == "id" {
			continue
		}

		declSb28.WriteString(", " + col + " TEXT")
	}

	decl += declSb28.String()

	if _, err := db.Exec("CREATE TABLE " + tasksPlannedTable + " (" + decl + ")"); err != nil {
		t.Fatalf("create planned table: %v", err)
	}
}

func TestConvergeProjectionHome(t *testing.T) {
	t.Parallel()

	t.Run("legacy home missing columns is snapshotted aside", func(t *testing.T) {
		t.Parallel()

		path := filepath.Join(t.TempDir(), "tq.db.readmodel.db")
		createPlannedTable(t, path, []string{"id", "project", "status"})

		db := openProbe(t, path)
		if _, err := db.Exec(
			`INSERT INTO ` + tasksPlannedTable + ` (id, project, status) VALUES ('legacy', 'p', 'pending')`,
		); err != nil {
			t.Fatalf("seed legacy row: %v", err)
		}

		db.Close()

		if err := ConvergeProjectionHome(t.Context(), path); err != nil {
			t.Fatalf("converge: %v", err)
		}

		if _, err := os.Stat(path); !os.IsNotExist(err) {
			t.Fatalf("projection home still present after convergence: %v", err)
		}

		matches, err := filepath.Glob(path + ".legacy-schema-*.bak")
		if err != nil || len(matches) != 1 {
			t.Fatalf("expected exactly one snapshot, got %v (err %v)", matches, err)
		}

		snap := openProbe(t, matches[0])
		defer snap.Close()

		var project string
		if err := snap.QueryRow(`SELECT project FROM ` + tasksPlannedTable + ` WHERE id='legacy'`).
			Scan(&project); err != nil ||
			project != "p" {
			t.Fatalf("snapshot lost the row: project=%q err=%v", project, err)
		}
	})

	t.Run("converged home is untouched", func(t *testing.T) {
		t.Parallel()

		path := filepath.Join(t.TempDir(), "tq.db.readmodel.db")
		createPlannedTable(t, path, taskRowColumns())

		if err := ConvergeProjectionHome(t.Context(), path); err != nil {
			t.Fatalf("converge: %v", err)
		}

		if _, err := os.Stat(path); err != nil {
			t.Fatalf("converged home vanished: %v", err)
		}

		if matches, _ := filepath.Glob(path + ".legacy-schema-*.bak"); len(matches) != 0 {
			t.Fatalf("snapshot written for a converged home: %v", matches)
		}
	})

	t.Run("absent home is a no-op", func(t *testing.T) {
		t.Parallel()

		path := filepath.Join(t.TempDir(), "tq.db.readmodel.db")

		if err := ConvergeProjectionHome(t.Context(), path); err != nil {
			t.Fatalf("converge: %v", err)
		}

		if _, err := os.Stat(path); !os.IsNotExist(err) {
			t.Fatalf("absent home materialized: %v", err)
		}
	})

	t.Run("file without the planned table is a no-op", func(t *testing.T) {
		t.Parallel()

		path := filepath.Join(t.TempDir(), "tq.db.readmodel.db")

		other := openProbe(t, path)
		if _, err := other.Exec(`CREATE TABLE unrelated (x TEXT)`); err != nil {
			t.Fatalf("create unrelated table: %v", err)
		}

		other.Close()

		if err := ConvergeProjectionHome(t.Context(), path); err != nil {
			t.Fatalf("converge: %v", err)
		}

		if _, err := os.Stat(path); err != nil {
			t.Fatalf("no-planned-table home vanished: %v", err)
		}
	})
}

func TestTaskRowColumnsPin(t *testing.T) {
	t.Parallel()

	cols := taskRowColumns()

	for _, want := range []string{"not_before", "parked_by", "id", "status"} {
		if !slices.Contains(cols, want) {
			t.Errorf("taskRowColumns missing %q: %v", want, cols)
		}
	}
}
