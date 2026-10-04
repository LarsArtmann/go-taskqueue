package migration

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/larsartmann/go-taskqueue/internal/queue/sqlitev4"
	_ "modernc.org/sqlite" // pure-Go SQLite driver (CGo-free)
)

// The on-open auto-upgrade (ADR-0019 endgame P1): a pre-flip hand-rolled
// database opened by the deployed binary must upgrade itself — backward
// auto-upgradeable, no owner-run replay step — before the store serves.
//
// Convergence is IN PLACE (the upstream engine's migrate() ALTERs the
// legacy tasks table: dedup_key, lease_token — the schemas are otherwise
// the same tables, transcribed), wrapped in a verified safety net:
//
//  1. Probe (read-only): classify the file's schema generation.
//  2. Snapshot the legacy file via VACUUM INTO (consistent, includes any
//     uncheckpointed WAL; normalizes sidecars on close).
//  3. Converge: sqlitev4.Open runs the engine + companion migrations.
//  4. Verify: the full projection-equality gate (Verify) against the
//     snapshot. Any mismatch RESTORES the snapshot over the file and
//     fails the open — the pool never serves a diverged store.
//
// The hook lives at the FACADE layer (internal/queue/sqlite.Open): the
// migration package imports sqlitev4, so sqlitev4.Open itself cannot call
// it (import cycle) and stays the pure engine store. Direct sqlitev4
// openers (tests, replay CLI) deliberately bypass the shim.
//
// Kill switch: TQ_NO_AUTO_UPGRADE=1 refuses the upgrade with instructions
// for the manual replay tool (go run ./replay --from --to), for owners
// who want the verified copy path instead.

// Kind classifies a database file's schema generation.
type Kind int

const (
	// KindFresh is a file with no tasks table: new or empty — the engine
	// migrations create everything from scratch.
	KindFresh Kind = iota
	// KindEngine is the go-cqrs-lite engine generation: the tasks table
	// carries the ADR-0134 lease_token column.
	KindEngine
	// KindLegacy is the pre-flip hand-rolled generation: tasks without
	// lease_token (the deployed production journal's shape until cutover).
	KindLegacy
)

// String renders the kind for logs and errors.
func (k Kind) String() string {
	switch k {
	case KindFresh:
		return "fresh"
	case KindEngine:
		return "engine"
	case KindLegacy:
		return "legacy"
	default:
		return fmt.Sprintf("kind(%d)", int(k))
	}
}

// ErrAutoUpgradeDisabled is returned when a legacy database needs the
// auto-upgrade but TQ_NO_AUTO_UPGRADE=1 refused it.
var ErrAutoUpgradeDisabled = errors.New(
	"migration: legacy database needs the on-open auto-upgrade, refused by TQ_NO_AUTO_UPGRADE=1" +
		" — run the replay tool (go run ./replay --from <db> --to <new.db>) or unset the variable",
)

// noAutoUpgradeEnv is the kill-switch variable name.
const noAutoUpgradeEnv = "TQ_NO_AUTO_UPGRADE"

// UpgradeResult reports what the auto-upgrade did.
type UpgradeResult struct {
	// Kind is the probed generation before the upgrade (always KindLegacy
	// when an upgrade ran).
	Kind Kind
	// BackupPath is the VACUUM INTO snapshot of the legacy file, kept for
	// audit/rollback after a successful upgrade.
	BackupPath string
	// Report is the projection-equality verdict of the converged store
	// against the snapshot.
	Report Report
}

// UpgradeIfNeeded probes path and, when it is a legacy hand-rolled
// database, upgrades it in place under the verified safety net. Fresh and
// engine files return (nil, nil): nothing to do.
func UpgradeIfNeeded(ctx context.Context, path string) (*UpgradeResult, error) {
	kind, err := Probe(path)
	if err != nil {
		return nil, fmt.Errorf("migration: probe %s: %w", path, err)
	}

	if kind != KindLegacy {
		return nil, nil
	}

	if os.Getenv(noAutoUpgradeEnv) == "1" {
		return nil, ErrAutoUpgradeDisabled
	}

	backupPath, err := snapshotLegacy(ctx, path)
	if err != nil {
		return nil, fmt.Errorf("migration: snapshot legacy db: %w", err)
	}

	// Converge in place: the engine's own migrations ALTER the legacy
	// table (dedup_key, lease_token) and create the companion tables;
	// existing rows are preserved.
	store, err := sqlitev4.Open(path) //nolint:contextcheck // the adapter's Open bootstraps its own migrations
	if err != nil {
		restoreOnConvergeFailure(path, backupPath)

		return nil, fmt.Errorf("migration: converge legacy db: %w", err)
	}

	if err := store.Close(); err != nil {
		return nil, fmt.Errorf("migration: close converged store: %w", err)
	}

	report, err := Verify(ctx, backupPath, path)
	if err != nil {
		restoreOnConvergeFailure(path, backupPath)

		return nil, fmt.Errorf("migration: verify converged db: %w", err)
	}

	if !report.OK() {
		restoreOnConvergeFailure(path, backupPath)

		return nil, fmt.Errorf(
			"migration: converged db diverged from the legacy snapshot — RESTORED the original file; mismatch report:\n%s",
			report.Summary(),
		)
	}

	slog.InfoContext(
		ctx,
		"migration: auto-upgraded legacy database onto the go-cqrs-lite engine (verified, snapshot kept)",
		"path", path, "backup", backupPath,
	)

	return &UpgradeResult{Kind: kind, BackupPath: backupPath, Report: report}, nil
}

// Probe classifies the schema generation of the database at path without
// mutating anything. A missing file is KindFresh (the store creates it).
func Probe(path string) (Kind, error) {
	db, err := sql.Open("sqlite", fmt.Sprintf("file:%s?mode=ro&_pragma=busy_timeout(5000)", path))
	if err != nil {
		return KindFresh, err
	}

	defer func() { _ = db.Close() }()

	var tables int

	if err := db.QueryRow(
		`SELECT COUNT(*) FROM sqlite_master WHERE type = 'table' AND name = 'tasks'`,
	).Scan(&tables); err != nil {
		// A read-only open of a nonexistent file fails here: fresh.
		if strings.Contains(err.Error(), "unable to open database file") {
			return KindFresh, nil
		}

		return KindFresh, fmt.Errorf("probe sqlite_master: %w", err)
	}

	if tables == 0 {
		return KindFresh, nil
	}

	var leaseToken int
	if err := db.QueryRow(
		`SELECT COUNT(*) FROM pragma_table_info('tasks') WHERE name = 'lease_token'`,
	).Scan(&leaseToken); err != nil {
		return KindFresh, fmt.Errorf("probe tasks columns: %w", err)
	}

	if leaseToken > 0 {
		return KindEngine, nil
	}

	return KindLegacy, nil
}

// snapshotLegacy writes a consistent snapshot of the legacy database to
// <path>.legacy-<timestamp>.bak via VACUUM INTO. The rw open also
// recovers and checkpoints any uncheckpointed WAL the old binary left
// behind, normalizing the file before the engine converges it.
func snapshotLegacy(ctx context.Context, path string) (string, error) {
	db, err := sql.Open("sqlite", fmt.Sprintf(
		"file:%s?_pragma=journal_mode(WAL)&_pragma=busy_timeout(5000)&_pragma=foreign_keys(1)", path))
	if err != nil {
		return "", err
	}

	defer func() { _ = db.Close() }()

	backupPath := fmt.Sprintf("%s.legacy-%s.bak", path, time.Now().Format("20060102-150405"))

	if _, err := db.ExecContext(ctx, "VACUUM INTO "+quoteSQLString(backupPath)); err != nil {
		return "", fmt.Errorf("vacuum into %s: %w", backupPath, err)
	}

	return backupPath, nil
}

// restoreOnConvergeFailure copies the snapshot back over the database
// file after a failed convergence or verification, so a retry starts from
// the untouched legacy shape. Best effort: failures are logged, never
// masked — the caller's error already fails the open.
func restoreOnConvergeFailure(path, backupPath string) {
	removeSQLiteSidecars(path)

	data, err := os.ReadFile(backupPath)
	if err != nil {
		slog.Error("migration: restore: read snapshot failed", "backup", backupPath, "err", err)

		return
	}

	if err := os.WriteFile(path, data, 0o600); err != nil {
		slog.Error("migration: restore: write db failed", "path", path, "err", err)

		return
	}

	slog.Warn("migration: restored legacy database after failed auto-upgrade", "path", path, "backup", backupPath)
}

// removeSQLiteSidecars deletes a database's -wal and -shm companions so a
// restored or replaced main file is never recovered against a stale WAL.
func removeSQLiteSidecars(path string) {
	for _, suffix := range []string{"-wal", "-shm"} {
		if err := os.Remove(path + suffix); err != nil && !errors.Is(err, os.ErrNotExist) {
			slog.Warn("migration: remove sidecar failed", "file", path+suffix, "err", err)
		}
	}
}

// quoteSQLString renders a single-quoted SQL string literal with escaped
// single quotes (VACUUM INTO takes a filename literal, not a parameter).
func quoteSQLString(s string) string {
	return "'" + strings.ReplaceAll(filepath.ToSlash(s), "'", "''") + "'"
}
