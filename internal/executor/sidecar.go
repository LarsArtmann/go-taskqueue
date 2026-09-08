package executor

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"
)

// SweepSidecars deletes agent-output sidecar logs older than maxAge from
// dir and returns how many were removed — the retention hatch behind
// --log-dir-max-age, so a long-running pool's sidecar directory cannot
// grow forever. Only *.log files directly inside dir are considered; other
// content is left alone. Errors on individual files are skipped (retention
// is best effort: a locked file must never fail the pool tick).
func SweepSidecars(dir string, maxAge time.Duration) (int, error) {
	if dir == "" || maxAge <= 0 {
		return 0, nil
	}

	entries, err := os.ReadDir(dir)
	if err != nil {
		return 0, fmt.Errorf("executor: sweep sidecars: %w", err)
	}

	cutoff := time.Now().Add(-maxAge)
	removed := 0

	for _, entry := range entries {
		if entry.IsDir() || !strings.HasSuffix(entry.Name(), ".log") {
			continue
		}

		info, err := entry.Info()
		if err != nil || info.ModTime().After(cutoff) {
			continue
		}

		if err := os.Remove(filepath.Join(dir, entry.Name())); err != nil {
			continue
		}

		removed++
	}

	return removed, nil
}

// SweepSidecarsByBytes caps the total size of sidecar logs in dir: when the
// *.log files together exceed maxBytes, the OLDEST are deleted until the
// total fits — the retention hatch behind --log-dir-max-bytes (the round-6
// retention item's open half). Age-based retention cannot bound a
// high-traffic dir that stays under --log-dir-max-age; a byte budget does.
// Only *.log files directly inside dir are considered; other content is
// left alone, and per-file errors are skipped (best effort, like the age
// sweep).
func SweepSidecarsByBytes(dir string, maxBytes int64) (int, error) {
	if dir == "" || maxBytes <= 0 {
		return 0, nil
	}

	entries, err := os.ReadDir(dir)
	if err != nil {
		return 0, fmt.Errorf("executor: sweep sidecars by bytes: %w", err)
	}

	type sidecar struct {
		path string
		size int64
		mod  time.Time
	}

	var (
		total  int64
		logged []sidecar
	)

	for _, entry := range entries {
		if entry.IsDir() || !strings.HasSuffix(entry.Name(), ".log") {
			continue
		}

		info, err := entry.Info()
		if err != nil {
			continue
		}

		total += info.Size()
		logged = append(logged, sidecar{
			path: filepath.Join(dir, entry.Name()),
			size: info.Size(),
			mod:  info.ModTime(),
		})
	}

	if total <= maxBytes {
		return 0, nil
	}

	// Oldest first: the freshest output is the most valuable forensics.
	sort.Slice(logged, func(i, j int) bool { return logged[i].mod.Before(logged[j].mod) })

	removed := 0
	for _, sc := range logged {
		if total <= maxBytes {
			break
		}

		if err := os.Remove(sc.path); err != nil {
			continue // locked or already gone: keep walking the budget down
		}

		total -= sc.size
		removed++
	}

	return removed, nil
}
