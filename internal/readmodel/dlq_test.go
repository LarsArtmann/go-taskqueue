package readmodel

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/larsartmann/go-taskqueue/internal/config"
)

// TestOpenDeadLettersInputContract pins the sidecar seam itself — the
// 2026-10-11 split-brain fix's tests cover the doctor + httpapi
// surfaces, not the seam: a QUEUE-home deployment yields the
// projection-beside sidecar (DLQPathFor over PathFor, never
// DLQPathFor over the queue path) opened at the deployment's DLQ
// pragma tier.
func TestOpenDeadLettersInputContract(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	d := config.SQLite(filepath.Join(dir, "q.db"))

	wantDSN := "file:" + filepath.Join(dir, "q.db.readmodel.db.dlq.db") +
		"?_pragma=journal_mode(WAL)&_pragma=busy_timeout(5000)" +
		"&_pragma=foreign_keys(1)&_pragma=synchronous(NORMAL)"

	if got := dlqDSN(d); got != wantDSN {
		t.Fatalf("dlqDSN =\n%s\nwant\n%s", got, wantDSN)
	}

	full := d
	full.SyncPolicy = config.SyncFull

	if got := dlqDSN(full); !strings.HasSuffix(got, "&_pragma=synchronous(FULL)") {
		t.Fatalf("full-tier DSN = %s, want synchronous(FULL) tier", got)
	}

	ctx := context.Background()

	dlq, err := OpenDeadLetters(ctx, d)
	if err != nil {
		t.Fatalf("OpenDeadLetters: %v", err)
	}

	if err := dlq.Close(); err != nil {
		t.Fatalf("close: %v", err)
	}

	if _, err := os.Stat(DLQPathFor(PathFor(d.DBPath))); err != nil {
		t.Fatalf("projection-beside sidecar missing: %v", err)
	}

	if _, err := os.Stat(DLQPathFor(d.DBPath)); !os.IsNotExist(err) {
		t.Fatalf("pre-fix queue-beside sidecar %s exists (split-brain re-planted): %v", DLQPathFor(d.DBPath), err)
	}
}
