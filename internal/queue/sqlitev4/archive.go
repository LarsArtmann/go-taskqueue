package sqlitev4

import (
	"context"
	"errors"

	"github.com/larsartmann/go-taskqueue/internal/queue/companion"
)

// ArchiveFactsBefore is NOT implemented in the S1 spike: the fact archive
// (facts_archive / journal_meta) has no upstream counterpart. The stub
// exists so the copied sqlite conformance suite compiles; its archive test
// skips itself with the divergence note.
func (s *Store) ArchiveFactsBefore(ctx context.Context, cutoff int64) (int64, error) {
	return 0, errors.New("sqlitev4: fact archive not implemented in the S1 spike")
}

// ArchiveSummary is NOT implemented in the S1 spike: see ArchiveFactsBefore.
func (s *Store) ArchiveSummary(ctx context.Context) (ArchiveStats, error) {
	return ArchiveStats{}, errors.New("sqlitev4: fact archive not implemented in the S1 spike")
}

// ArchiveStats aliases the companion archive-summary shape (conform
// suite surface; the tq sqlite store's summary shape).
type ArchiveStats = companion.ArchiveStats
