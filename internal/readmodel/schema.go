package readmodel

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"log/slog"
	"os"
	"reflect"
	"strings"
	"time"
)

// tasksPlannedTable is the metaengine planned-table name for the tasks
// collection: meta_planned_<collection> over tasksCollection. The name
// formula lives upstream (metaengine/v4 layout); the collection itself is
// pinned here, so the derivation is restated once instead of imported.
const tasksPlannedTable = "meta_planned_" + tasksCollection

// ConvergeProjectionHome reconciles an on-disk projection home with the
// current TaskRow model BEFORE any engine opens the file: a projection
// written by an older model (a planned-table column the current fold
// inserts does not exist) makes every task.enqueued fold fail, the
// metaengine's CREATE TABLE IF NOT EXISTS never alters an existing
// table, and the projection worker crash-loops on the first fact until
// it exhausts its restart budget — the Oct 2026 production outage where
// meta_planned_tasks predated not_before.
//
// The projection is DISPOSABLE by contract (delete the file to force a
// full journal replay), so a divergent home is snapshotted aside —
// renamed, never deleted — and the next open recreates it from the
// current model; WithDurableCursor already replays from zero exactly
// once when the surviving watermark points past an empty projection.
// A healthy or absent home is left untouched.
func ConvergeProjectionHome(ctx context.Context, path string) error {
	if _, err := os.Stat(path); err != nil {
		if os.IsNotExist(err) {
			return nil
		}

		return fmt.Errorf("readmodel: probe projection home: %w", err)
	}

	missing, err := probeMissingColumns(ctx, path)
	if err != nil {
		return err
	}

	if len(missing) == 0 {
		return nil
	}

	snapshot, err := snapshotProjectionHome(path)
	if err != nil {
		return err
	}

	slog.Warn("readmodel: legacy projection home snapshotted for replay",
		"path", path,
		"snapshot", snapshot,
		"missing_columns", strings.Join(missing, ","),
	)

	return nil
}

// probeMissingColumns opens the projection read-only and reports which
// TaskRow columns the planned table lacks (empty = converged). A missing
// table means the engine will create it fresh.
func probeMissingColumns(ctx context.Context, path string) (missing []string, err error) {
	db, err := sql.Open("sqlite", "file:"+path+"?mode=ro&_pragma=busy_timeout(5000)")
	if err != nil {
		return nil, fmt.Errorf("readmodel: probe projection home: %w", err)
	}
	defer func() {
		if cerr := db.Close(); cerr != nil && err == nil {
			err = fmt.Errorf("readmodel: close projection home probe: %w", cerr)
		}
	}()

	var table string

	err = db.QueryRowContext(ctx,
		`SELECT name FROM sqlite_master WHERE type='table' AND name=?`,
		tasksPlannedTable,
	).Scan(&table)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, nil
	}

	if err != nil {
		return nil, fmt.Errorf("readmodel: probe %s: %w", tasksPlannedTable, err)
	}

	rows, err := db.QueryContext(ctx, fmt.Sprintf(`PRAGMA table_info(%s)`, tasksPlannedTable))
	if err != nil {
		return nil, fmt.Errorf("readmodel: probe %s columns: %w", tasksPlannedTable, err)
	}
	defer func() {
		if cerr := rows.Close(); cerr != nil && err == nil {
			err = fmt.Errorf("readmodel: close %s column cursor: %w", tasksPlannedTable, cerr)
		}
	}()

	actual := map[string]bool{}

	for rows.Next() {
		var (
			cid           int
			name, colType string
			notNull       int
			dflt          sql.NullString
			pk            int
		)
		if err := rows.Scan(&cid, &name, &colType, &notNull, &dflt, &pk); err != nil {
			return nil, fmt.Errorf("readmodel: scan %s columns: %w", tasksPlannedTable, err)
		}

		actual[name] = true
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("readmodel: scan %s columns: %w", tasksPlannedTable, err)
	}

	for _, col := range taskRowColumns() {
		if !actual[col] {
			missing = append(missing, col)
		}
	}

	return missing, nil
}

// taskRowColumns derives the planned-table column names from TaskRow's
// json tags — one source of truth, so model evolution cannot drift the
// convergence probe.
func taskRowColumns() []string {
	t := reflect.TypeFor[TaskRow]()

	cols := make([]string, 0, t.NumField())
	for field := range t.Fields() {
		name, _, _ := strings.Cut(field.Tag.Get("json"), ",")
		if name != "" {
			cols = append(cols, name)
		}
	}

	return cols
}

// snapshotProjectionHome renames the projection file (plus its WAL/SHM
// sidecars when present) to a dated .bak beside it — reversible by
// moving it back, unlike a delete.
func snapshotProjectionHome(path string) (string, error) {
	snapshot := path + fmt.Sprintf(".legacy-schema-%s.bak", time.Now().UTC().Format("20060102-150405"))

	for _, suffix := range []string{"", "-wal", "-shm"} {
		if err := os.Rename(path+suffix, snapshot+suffix); err != nil && !os.IsNotExist(err) {
			return "", fmt.Errorf("readmodel: snapshot legacy projection home: %w", err)
		}
	}

	return snapshot, nil
}
