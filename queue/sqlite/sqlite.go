// Package sqlite is the public facade over the SQLite store driver
// (ADR-0016): the embedded default backend. The implementation stays
// internal; this package pins the names.
package sqlite

import (
	internalsqlite "github.com/larsartmann/go-taskqueue/internal/queue/sqlite"
)

type (
	Store        = internalsqlite.Store
	StoreOption  = internalsqlite.StoreOption
	ArchiveStats = internalsqlite.ArchiveStats
)

var (
	Open                   = internalsqlite.Open
	WithProjectExclusivity = internalsqlite.WithProjectExclusivity
)
