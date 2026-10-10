// Package config is tq's single deployment description: where the state
// lives (driver, path/DSN), which IO tier the sqlite homes run at
// (SyncPolicy), and whether the read-model projection is enabled. Every
// layer that opens a database — the sqlitev4 driver, the composition
// root's projection engine, the CLI flags — consumes THIS struct, so
// exactly one pragma builder and one env reader exist in the repo
// (ADR-0022's deployment lane).
//
// The package is a leaf: no internal dependencies, stdlib only.
package config

import (
	"fmt"
	"os"
	"strings"
)

// Driver names the storage backend behind a Deployment.
type Driver string

const (
	// DriverSQLite is the embedded default: one file, one process.
	DriverSQLite Driver = "sqlite"
	// DriverPostgres is the shared-server backend selected by a
	// postgres:// --store value.
	DriverPostgres Driver = "postgres"
)

// SyncPolicy is tq's sqlite IO durability tier (the TQ_SQLITE_SYNC
// vocabulary): NORMAL is the steady-state default (fsync at WAL
// checkpoints only — the queue's recovery model tolerates a lost tail);
// FULL restores per-commit fsync; OFF disables fsync entirely.
type SyncPolicy string

const (
	SyncFull   SyncPolicy = "full"
	SyncNormal SyncPolicy = "normal"
	SyncOff    SyncPolicy = "off"
)

// EnvSyncPolicy is the environment variable that overrides the sync
// tier; the name and the full|normal|off vocabulary are a deployment
// contract (existing operators' settings keep their meaning).
const EnvSyncPolicy = "TQ_SQLITE_SYNC"

// Deployment is the one description of where tq's state lives. Exactly
// one of DBPath (sqlite) or DSN (postgres) is meaningful — the driver
// decides which — and Validate makes a mixed or empty deployment an
// error at construction instead of a mystery at open.
type Deployment struct {
	Driver           Driver
	DBPath           string     // sqlite file path
	DSN              string     // postgres DSN (postgres://…)
	SyncPolicy       SyncPolicy // sqlite IO tier (both sqlite homes)
	ReadModelEnabled bool       // serve-time projection toggle
}

// SQLite returns a sqlite Deployment over path at the default sync tier.
func SQLite(path string) Deployment {
	return Deployment{Driver: DriverSQLite, DBPath: path, SyncPolicy: SyncNormal}
}

// Postgres returns a postgres Deployment over dsn.
func Postgres(dsn string) Deployment {
	return Deployment{Driver: DriverPostgres, DSN: dsn}
}

// Validate enforces the deployment's shape invariants.
func (d Deployment) Validate() error {
	switch d.Driver {
	case DriverSQLite:
		if d.DBPath == "" {
			return fmt.Errorf("config: sqlite deployment needs a DBPath")
		}

		if d.DSN != "" {
			return fmt.Errorf("config: sqlite deployment must not carry a DSN")
		}
	case DriverPostgres:
		if d.DSN == "" {
			return fmt.Errorf("config: postgres deployment needs a DSN")
		}

		if d.DBPath != "" {
			return fmt.Errorf("config: postgres deployment must not carry a DBPath")
		}
	default:
		return fmt.Errorf("config: unknown driver %q (want sqlite or postgres)", d.Driver)
	}

	switch d.SyncPolicy {
	case SyncFull, SyncNormal, SyncOff:
	default:
		return fmt.Errorf("config: unknown sync policy %q (want full, normal, off)", d.SyncPolicy)
	}

	return nil
}

// SyncPolicyFromEnv reads EnvSyncPolicy and validates the vocabulary.
// Empty means the default tier; the case-insensitive full|normal|off
// contract and the error wording match the pre-struct sqlitev4 reader
// byte for byte, so existing operators' environments keep their exact
// meaning.
func SyncPolicyFromEnv() (SyncPolicy, error) {
	return parseSyncPolicy(os.Getenv(EnvSyncPolicy))
}

func parseSyncPolicy(v string) (SyncPolicy, error) {
	v = strings.ToUpper(strings.TrimSpace(v))
	if v == "" {
		return SyncNormal, nil
	}

	switch SyncPolicy(v) {
	case SyncFull, SyncNormal, SyncOff:
		return SyncPolicy(v), nil
	default:
		return "", fmt.Errorf("TQ_SQLITE_SYNC must be one of full, normal, off (got %q)",
			strings.TrimSpace(os.Getenv(EnvSyncPolicy)))
	}
}

// FromFlags builds the Deployment from the CLI-shaped store value and
// the sync-policy environment: a bare path is a sqlite file, a
// postgres:// URL selects the shared-server driver. The environment
// merge happens here — the ONE TQ_SQLITE_SYNC reader — so the struct
// every opener consumes is already resolved.
func FromFlags(store string, readModel bool) (Deployment, error) {
	d := SQLite(store)

	if strings.HasPrefix(store, "postgres://") || strings.HasPrefix(store, "postgresql://") {
		d = Postgres(store)
	}

	policy, err := SyncPolicyFromEnv()
	if err != nil {
		return Deployment{}, err
	}

	d.SyncPolicy = policy
	d.ReadModelEnabled = readModel

	if err := d.Validate(); err != nil {
		return Deployment{}, err
	}

	return d, nil
}

// QueuePragmas renders the sqlite connection pragmas for the QUEUE home
// in DSN order — the ONE pragma literal in the repo (the struct's
// builder). The order is load-bearing: journal_mode must precede
// synchronous so WAL is active before the tier applies.
func (d Deployment) QueuePragmas() []string {
	return []string{
		"journal_mode(WAL)",
		"busy_timeout(5000)",
		"foreign_keys(1)",
		"synchronous(" + d.syncTier() + ")",
		"temp_store(MEMORY)",
		"cache_size(-32768)",
		"journal_size_limit(8388608)",
	}
}

// ProjectionPragmas renders the caller-side pragmas the projection-home
// engine takes (the sqliteengine factory prepends journal_mode=WAL and
// busy_timeout on every connection, so the caller list carries only the
// tier and the cache budget). Derived from the same SyncPolicy as the
// queue home: one knob, both sqlite homes.
func (d Deployment) ProjectionPragmas() []string {
	return []string{
		"synchronous=" + d.syncTier(),
		"cache_size=-32768",
	}
}

func (d Deployment) syncTier() string {
	return strings.ToUpper(string(d.SyncPolicy))
}
