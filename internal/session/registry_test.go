package session

import (
	"context"
	"encoding/json"
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func writeRegistry(t *testing.T, entries []RegistryEntry) string {
	t.Helper()

	path := filepath.Join(t.TempDir(), "session-registry.jsonl")
	if err := RewriteRegistry(path, entries); err != nil {
		t.Fatalf("seed registry: %v", err)
	}

	return path
}

func TestPingRegistryAppendsAndLoadIsLatestWins(t *testing.T) {
	path := filepath.Join(t.TempDir(), "session-registry.jsonl")
	old := time.Now().Add(-time.Hour).UTC()

	if err := PingRegistry(path, "sess-a", "/repos/demo", old); err != nil {
		t.Fatalf("ping 1: %v", err)
	}
	if err := PingRegistry(path, "sess-a", "/repos/demo", time.Now().UTC()); err != nil {
		t.Fatalf("ping 2: %v", err)
	}

	entries, err := LoadRegistry(path)
	if err != nil {
		t.Fatalf("load: %v", err)
	}

	if len(entries) != 1 {
		t.Fatalf("entries = %d, want one per session id", len(entries))
	}

	e := entries[0]
	if e.ID != "sess-a" || e.CWD != "/repos/demo" {
		t.Fatalf("entry = %+v", e)
	}
	if e.LastSeen.Round(time.Second).Equal(old.Round(time.Second)) {
		t.Fatalf("latest-wins violated: kept the oldest ping (%s)", e.LastSeen)
	}
}

func TestPingRegistryRefusesEmptyID(t *testing.T) {
	path := filepath.Join(t.TempDir(), "session-registry.jsonl")

	if err := PingRegistry(path, "", "/repos/demo", time.Now()); err == nil {
		t.Fatal("empty id accepted")
	}
}

func TestLoadRegistryMissingFileIsEmptyAndTornLinesAreSkipped(t *testing.T) {
	entries, err := LoadRegistry(filepath.Join(t.TempDir(), "absent.jsonl"))
	if err != nil {
		t.Fatalf("missing file: %v", err)
	}
	if len(entries) != 0 {
		t.Fatalf("missing file yielded %d entries", len(entries))
	}

	path := filepath.Join(t.TempDir(), "torn.jsonl")
	good, err := json.Marshal(RegistryEntry{ID: "sess-a", CWD: "/repos/demo", LastSeen: time.Now().UTC()})
	if err != nil {
		t.Fatal(err)
	}
	body := "\n" + string(good) + "\n" + `{"id": "sess-b", "cwd": ` + "\n" + "garbage\n"
	if err := os.WriteFile(path, []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}

	entries, err = LoadRegistry(path)
	if err != nil {
		t.Fatalf("load torn: %v", err)
	}
	if len(entries) != 1 || entries[0].ID != "sess-a" {
		t.Fatalf("entries = %+v, want only sess-a", entries)
	}
}

type stubOwner struct {
	owned map[string]bool
	err   error
}

func (o stubOwner) Owns(_ context.Context, id string) (bool, error) {
	if o.err != nil {
		return false, o.err
	}

	return o.owned[id], nil
}

func TestSweepClosesQuietUnownedAndPrunes(t *testing.T) {
	ctx := context.Background()
	s := testStore(t)

	old := time.Now().Add(-time.Hour).UTC()
	path := writeRegistry(t, []RegistryEntry{
		{ID: "sess-old", CWD: "/repos/demo", LastSeen: old},
		{ID: "sess-fresh", CWD: "/repos/demo", LastSeen: time.Now().UTC()},
	})

	outcomes, err := Sweep(ctx, s, stubScanner(nil), SweepInput{
		RegistryPath: path,
		StaleAfter:   10 * time.Minute,
		Owner:        stubOwner{owned: map[string]bool{}},
		Now:          time.Now().UTC(),
	})
	if err != nil {
		t.Fatalf("sweep: %v", err)
	}

	if len(outcomes) != 2 {
		t.Fatalf("outcomes = %d, want 2", len(outcomes))
	}

	byID := map[string]SweepOutcome{}
	for _, o := range outcomes {
		byID[o.Entry.ID] = o
	}
	if !byID["sess-old"].Closed {
		t.Fatalf("sess-old not closed: %+v", byID["sess-old"])
	}
	if byID["sess-fresh"].Closed || byID["sess-fresh"].Err != nil {
		t.Fatalf("sess-fresh should be kept as fresh: %+v", byID["sess-fresh"])
	}

	facts, err := s.Facts(ctx, 0, 0)
	if err != nil {
		t.Fatal(err)
	}
	sawClosed := false
	for _, f := range facts {
		if f.Type == "session.closed" && f.TaskID == "session:sess-old" {
			sawClosed = true
		}
	}
	if !sawClosed {
		t.Fatalf("no session.closed fact for sess-old in %d fact(s)", len(facts))
	}

	kept, err := LoadRegistry(path)
	if err != nil {
		t.Fatal(err)
	}
	if len(kept) != 1 || kept[0].ID != "sess-fresh" {
		t.Fatalf("registry after sweep = %+v, want only sess-fresh", kept)
	}
}

func TestSweepKeepsOwnedSessions(t *testing.T) {
	ctx := context.Background()
	s := testStore(t)

	path := writeRegistry(t, []RegistryEntry{
		{ID: "sess-live", CWD: "/repos/demo", LastSeen: time.Now().Add(-time.Hour).UTC()},
	})

	outcomes, err := Sweep(ctx, s, stubScanner(nil), SweepInput{
		RegistryPath: path,
		StaleAfter:   10 * time.Minute,
		Owner:        stubOwner{owned: map[string]bool{"sess-live": true}},
		Now:          time.Now().UTC(),
	})
	if err != nil {
		t.Fatalf("sweep: %v", err)
	}
	if outcomes[0].Closed {
		t.Fatalf("owned session closed: %+v", outcomes[0])
	}

	kept, err := LoadRegistry(path)
	if err != nil {
		t.Fatal(err)
	}
	if len(kept) != 1 {
		t.Fatalf("owned session pruned from registry: %+v", kept)
	}
}

func TestSweepKeepsEntryWhenOwnerCheckFails(t *testing.T) {
	ctx := context.Background()
	s := testStore(t)

	path := writeRegistry(t, []RegistryEntry{
		{ID: "sess-x", CWD: "/repos/demo", LastSeen: time.Now().Add(-time.Hour).UTC()},
	})

	outcomes, err := Sweep(ctx, s, stubScanner(nil), SweepInput{
		RegistryPath: path,
		StaleAfter:   10 * time.Minute,
		Owner:        stubOwner{err: errors.New("pgrep unavailable")},
		Now:          time.Now().UTC(),
	})
	if err != nil {
		t.Fatalf("sweep: %v", err)
	}
	if outcomes[0].Err == nil || outcomes[0].Closed {
		t.Fatalf("owner-check failure should keep the entry: %+v", outcomes[0])
	}

	kept, err := LoadRegistry(path)
	if err != nil {
		t.Fatal(err)
	}
	if len(kept) != 1 {
		t.Fatalf("failed-ownership session pruned from registry: %+v", kept)
	}
}

func TestSweepDefaultsOwnerAndNow(t *testing.T) {
	path := filepath.Join(t.TempDir(), "empty.jsonl")

	outcomes, err := Sweep(context.Background(), testStore(t), stubScanner(nil), SweepInput{
		RegistryPath: path,
		StaleAfter:   time.Minute,
	})
	if err != nil {
		t.Fatalf("sweep with defaults: %v", err)
	}
	if len(outcomes) != 0 {
		t.Fatalf("outcomes = %+v, want none for an empty registry", outcomes)
	}

	if _, err := os.Stat(path); err != nil {
		t.Fatalf("registry file not rewritten: %v", err)
	}
}

func TestPgrepOwnerFindsNothingForAbsentPattern(t *testing.T) {
	if _, err := exec.LookPath("pgrep"); err != nil {
		t.Skip("pgrep unavailable")
	}

	owned, err := PgrepOwner{}.Owns(context.Background(), "tq-session-registry-test-no-such-process")
	if err != nil {
		t.Fatalf("pgrep: %v", err)
	}
	if owned {
		t.Fatal("pgrep reported ownership for a pattern no process carries")
	}
}

func TestRegistryEntriesAreJSONLines(t *testing.T) {
	var b strings.Builder
	e := RegistryEntry{ID: "sess-a", CWD: "/repos/demo", LastSeen: time.Date(2026, 9, 17, 0, 0, 0, 0, time.UTC)}
	line, err := json.Marshal(e)
	if err != nil {
		t.Fatal(err)
	}
	b.Write(line)

	if strings.Contains(b.String(), `"last_seen"`) == strings.Contains(b.String(), `"LastSeen"`) {
		t.Fatalf("registry wire format must stay snake_case: %s", b.String())
	}
}
