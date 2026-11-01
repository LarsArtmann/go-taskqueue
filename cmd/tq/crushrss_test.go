package main

import (
	"os"
	"path/filepath"
	"strconv"
	"testing"
)

// TestProcRSSKB pins the /proc parser: a live process answers with a
// positive VmRSS, pid<=0 and missing processes answer !ok (the keeper
// skips the sample instead of logging garbage).
func TestProcRSSKB(t *testing.T) {
	if kb, ok := procRSSKB(0); ok || kb != 0 {
		t.Fatalf("pid 0 = (%d,%v), want (0,false)", kb, ok)
	}

	if _, ok := procRSSKB(1 << 30); ok {
		t.Fatal("an absurd pid must not resolve")
	}

	if kb, ok := procRSSKB(int64(os.Getpid())); !ok || kb <= 0 {
		t.Fatalf("own process = (%d,%v), want a positive VmRSS", kb, ok)
	}
}

// TestReadCrushServerRSS pins the fold: samples counted, last/max
// correct, malformed lines skipped, absent log = nil (unrouted pool),
// and the WARN band only trips above crushServerRSSWarnKB.
func TestReadCrushServerRSS(t *testing.T) {
	if got := readCrushServerRSS(filepath.Join(t.TempDir(), "absent.log")); got != nil {
		t.Fatalf("absent log = %+v, want nil", got)
	}

	path := filepath.Join(t.TempDir(), "rss.log")
	body := "1700000000000 100000\n" +
		"garbage line\n" +
		"1700000045000 300000\n" +
		"1700000090000 200000\n" +
		"1700000135000 -5\n" + // negative: skipped
		"\n"
	if err := os.WriteFile(path, []byte(body), 0o600); err != nil {
		t.Fatal(err)
	}

	got := readCrushServerRSS(path)
	if got == nil {
		t.Fatal("valid log folded to nil")
	}

	if got.Samples != 3 || got.LastKB != 200000 || got.MaxKB != 300000 {
		t.Fatalf("fold = %+v, want samples 3, last 200000, max 300000", got)
	}

	if got.Warn {
		t.Fatalf("200000 kB must not WARN (band %d)", int64(crushServerRSSWarnKB))
	}

	hot := filepath.Join(t.TempDir(), "hot.log")
	if err := os.WriteFile(hot, []byte("1700000000000 "+strconv.FormatInt(crushServerRSSWarnKB+1, 10)+"\n"), 0o600); err != nil {
		t.Fatal(err)
	}

	if got := readCrushServerRSS(hot); got == nil || !got.Warn {
		t.Fatalf("above-band sample must WARN: %+v", got)
	}
}

// TestCrushServerRSSLogPath pins the evidence-file placement: next to
// the journal when TQ_DB names one, disabled otherwise.
func TestCrushServerRSSLogPath(t *testing.T) {
	t.Setenv("TQ_DB", "")

	if got := crushServerRSSLogPath(); got != "" {
		t.Fatalf("no TQ_DB = %q, want empty (disabled)", got)
	}

	t.Setenv("TQ_DB", "/mnt/pool/services/tq/tq.db")

	if got := crushServerRSSLogPath(); got != "/mnt/pool/services/tq/crush-server-rss.log" {
		t.Fatalf("path = %q, want the journal-dir sibling", got)
	}
}
