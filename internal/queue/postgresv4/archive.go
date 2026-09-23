package postgresv4

import (
	"context"
	"errors"
)

// ArchiveFactsBefore is NOT implemented in the S1 spike: the fact archive
// (facts_archive / journal_meta) has no upstream counterpart. The stub
// exists so the copied sqlite conformance suite compiles; its archive test
// skips itself with the divergence note.
func (s *Store) ArchiveFactsBefore(ctx context.Context, cutoff int64) (int64, error) {
	return 0, errors.New("postgresv4: fact archive not implemented in the S1 spike")
}

// ArchiveSummary is NOT implemented in the S1 spike: see ArchiveFactsBefore.
func (s *Store) ArchiveSummary(ctx context.Context) (ArchiveStats, error) {
	return ArchiveStats{}, errors.New("postgresv4: fact archive not implemented in the S1 spike")
}

// ArchiveStats mirrors the tq sqlite store's archive summary shape.
type ArchiveStats struct {
	Hot       int64
	Archived  int64
	Watermark int64
}
