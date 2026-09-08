package executor

import (
	"fmt"
	"os"
	"path/filepath"
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
