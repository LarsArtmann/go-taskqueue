package papdashboard

import (
	"context"
	"fmt"
	"log/slog"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/larsartmann/go-taskqueue/internal/journal"
	"github.com/larsartmann/go-taskqueue/internal/task"
)

// fakeWatermarks is an in-memory WatermarkStore with injectable failures.
type fakeWatermarks struct {
	mu       sync.Mutex
	saved    map[string]int64
	saves    int
	failFrom int // 1-based: fail the Nth and every later save (0 = never fail)
}

func newFakeWatermarks() *fakeWatermarks {
	return &fakeWatermarks{saved: map[string]int64{}}
}

func (f *fakeWatermarks) Watermark(_ context.Context, consumer string) (int64, error) {
	f.mu.Lock()
	defer f.mu.Unlock()

	return f.saved[consumer], nil
}

func (f *fakeWatermarks) SaveWatermark(_ context.Context, consumer string, seq int64) error {
	f.mu.Lock()
	defer f.mu.Unlock()

	f.saves++

	if f.failFrom > 0 && f.saves >= f.failFrom {
		return fmt.Errorf("watermark store down (save %d)", f.saves)
	}

	if seq > f.saved[consumer] {
		f.saved[consumer] = seq
	}

	return nil
}

func (f *fakeWatermarks) current(consumer string) int64 {
	f.mu.Lock()
	defer f.mu.Unlock()

	return f.saved[consumer]
}

// runBridgeUntil runs b.Run in the background until cond holds, then
// cancels and waits for a clean return.
func runBridgeUntil(t *testing.T, b *Bridge, cond func() bool) error {
	t.Helper()

	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan error, 1)

	go func() { done <- b.Run(ctx) }()

	if !waitFor(cond) {
		cancel()
		<-done

		t.Fatal("condition not reached before deadline")
	}

	cancel()

	return <-done
}

// waitFor polls cond every 2ms until it holds (2s deadline).
func waitFor(cond func() bool) bool {
	deadline := time.Now().Add(2 * time.Second)

	for time.Now().Before(deadline) {
		if cond() {
			return true
		}

		time.Sleep(2 * time.Millisecond)
	}

	return cond()
}

// dlFact builds a DeadLettered fact for a dead task.
func dlFact(seq int64, id string) journal.Fact {
	return journal.Fact{Seq: seq, TaskID: id, Type: journal.DeadLettered, Error: "boom"}
}

// fillFacts builds seq 1..n of quiet facts (enqueued) for a filler task.
func fillFacts(n int64) []journal.Fact {
	out := make([]journal.Fact, 0, n)

	for seq := int64(1); seq <= n; seq++ {
		out = append(out, journal.Fact{Seq: seq, TaskID: "t-filler", Type: journal.Enqueued})
	}

	return out
}

// TestRestartMidStreamLosesZeroFacts is the core at-least-once proof: a
// bridge that dies mid-batch (forward failed, checkpoint not yet written)
// hands its successor a cursor at the last PERSISTED seq, so every fact is
// delivered exactly once across the restart and re-sent facts carry
// bit-identical idempotency keys.
func TestRestartMidStreamLosesZeroFacts(t *testing.T) {
	pap := newFakePap(t)
	wm := newFakeWatermarks()

	facts := fillFacts(600)
	facts[149] = dlFact(150, "t-dead-a") // inside batch 1 (101..600 from cursor 100)
	facts[549] = dlFact(550, "t-dead-b") // same batch, later

	src := &fakeSource{
		facts: facts[:100],
		tasks: map[string]task.Task{
			"t-dead-a": {ID: task.ID("t-dead-a"), Project: "p", Type: "deploy"},
			"t-dead-b": {ID: task.ID("t-dead-b"), Project: "p", Type: "deploy"},
		},
	}

	// Bridge A boots at head 100, then the burst arrives. The dashboard
	// accepts the first dead letter but rejects the second (502), so A's
	// drain stops mid-batch with nothing checkpointed past 100.
	pap.failAfter(1)

	ba := New(src, wm, Config{
		Endpoint: pap.server.URL, Logger: quietLogger(), PollInterval: 2 * time.Millisecond,
	})

	ctxA, cancelA := context.WithCancel(context.Background())
	doneA := make(chan error, 1)

	go func() { doneA <- ba.Run(ctxA) }()

	if !waitFor(func() bool { return wm.current("papdashboard:"+pap.server.URL) == 100 }) {
		cancelA()
		<-doneA

		t.Fatal("bridge A never bootstrapped")
	}

	src.add(facts[100:]...) // the burst: seq 101..600, dead letters at 150 and 550

	if !waitFor(func() bool { return len(pap.calls()) >= 1 }) {
		cancelA()
		<-doneA

		t.Fatal("bridge A never delivered the first dead letter")
	}

	// A "crashes": cancel without waiting for a retry tick.
	cancelA()
	<-doneA

	if got := wm.current("papdashboard:" + pap.server.URL); got != 100 {
		t.Fatalf("bridge A checkpoint = %d before crash, want 100 (mid-batch, unpersisted)", got)
	}

	// The dashboard heals; bridge B resumes from the checkpoint at 100.
	pap.failAfter(0)

	bb := New(src, wm, Config{
		Endpoint: pap.server.URL, Logger: quietLogger(), PollInterval: 2 * time.Millisecond,
	})

	if err := runBridgeUntil(t, bb, func() bool { return len(pap.calls()) >= 3 }); err != nil {
		t.Fatalf("bridge B: %v", err)
	}

	keys := map[string]int{}

	for _, c := range pap.calls() {
		keys[c.IdempotencyKey]++
	}

	// t-dead-a was accepted by A and re-sent by B with the SAME key; the
	// dashboard dedupes. t-dead-b failed under A and is delivered by B.
	if keys[SourceApp+"-dlq-150"] != 2 {
		t.Errorf("dlq-150 key seen %d times, want 2 (re-send after restart)", keys[SourceApp+"-dlq-150"])
	}

	if keys[SourceApp+"-dlq-550"] != 1 {
		t.Errorf("dlq-550 key seen %d times, want 1 (never lost)", keys[SourceApp+"-dlq-550"])
	}

	if got := wm.current("papdashboard:"+pap.server.URL); got != 600 {
		t.Errorf("final watermark = %d, want 600", got)
	}
}

// TestCheckpointFailureGatesForwarding proves a failing checkpoint store
// cannot silently degrade delivery to at-most-once: while the cursor is
// unpersisted, later batches are not forwarded at all, and healing the
// store resumes exactly where the drain stopped.
func TestCheckpointFailureGatesForwarding(t *testing.T) {
	pap := newFakePap(t)
	wm := newFakeWatermarks()

	facts := fillFacts(700)
	facts[149] = dlFact(150, "t-dead-a") // batch 1 (101..600)
	facts[649] = dlFact(650, "t-dead-b") // batch 2 (601..700)

	src := &fakeSource{
		facts: facts[:100],
		tasks: map[string]task.Task{
			"t-dead-a": {ID: task.ID("t-dead-a"), Project: "p", Type: "deploy"},
			"t-dead-b": {ID: task.ID("t-dead-b"), Project: "p", Type: "deploy"},
		},
	}

	// Save #1 is the eager bootstrap (100); save #2 is batch 1's
	// checkpoint — fail it and every later save until healed.
	wm.failFrom = 2

	ba := New(src, wm, Config{
		Endpoint: pap.server.URL, Logger: quietLogger(), PollInterval: 2 * time.Millisecond,
	})

	ctxA, cancelA := context.WithCancel(context.Background())
	doneA := make(chan error, 1)

	go func() { doneA <- ba.Run(ctxA) }()

	if !waitFor(func() bool { return wm.current("papdashboard:"+pap.server.URL) == 100 }) {
		cancelA()
		<-doneA

		t.Fatal("bridge A never bootstrapped")
	}

	src.add(facts[100:]...) // burst: batch 1 = 101..600 (dlq-150), batch 2 = 601..700 (dlq-650)

	// Batch 1 forwards (dlq-150 accepted), its checkpoint fails, and the
	// pending gate blocks batch 2 for as long as the store is down.
	if !waitFor(func() bool { return len(pap.calls()) >= 1 }) {
		cancelA()
		<-doneA

		t.Fatal("batch 1 never forwarded")
	}

	time.Sleep(30 * time.Millisecond) // several poll ticks with the gate shut

	if got := len(pap.calls()); got != 1 {
		t.Fatalf("got %d ingests while checkpoint store down, want 1 (gate must block batch 2)", got)
	}

	if got := wm.current("papdashboard:"+pap.server.URL); got != 100 {
		t.Fatalf("persisted watermark = %d while store down, want 100", got)
	}

	// Heal the store: the next tick flushes the pending checkpoint and
	// batch 2 delivers its dead letter.
	wm.mu.Lock()
	wm.failFrom = 0
	wm.saves = 0
	wm.mu.Unlock()

	if !waitFor(func() bool { return len(pap.calls()) >= 2 }) {
		cancelA()
		<-doneA

		t.Fatal("batch 2 never forwarded after healing")
	}

	cancelA()

	if err := <-doneA; err != nil {
		t.Fatalf("bridge A returned %v", err)
	}

	if got := wm.current("papdashboard:"+pap.server.URL); got != 700 {
		t.Errorf("final watermark = %d, want 700", got)
	}
}

// TestFromSeqOverridesPersistedCheckpoint pins the precedence: an explicit
// FromSeq beats the stored cursor (ops replay), and the startup log names
// the branch that decided.
func TestFromSeqOverridesPersistedCheckpoint(t *testing.T) {
	pap := newFakePap(t)
	wm := newFakeWatermarks()

	facts := fillFacts(200)
	facts[149] = dlFact(150, "t-dead")

	src := &fakeSource{
		facts: facts,
		tasks: map[string]task.Task{
			"t-dead": {ID: task.ID("t-dead"), Project: "p", Type: "deploy"},
		},
	}

	// A cursor persisted at 500 would skip seq 150; FromSeq=100 wins and
	// the dead letter is replayed.
	wm.saved["papdashboard:"+pap.server.URL] = 500

	var logMu sync.Mutex

	logLines := &syncBuffer{mu: &logMu}

	fromSeq := int64(100)

	b := New(src, wm, Config{
		Endpoint:     pap.server.URL,
		Logger:       slog.New(slog.NewTextHandler(logLines, &slog.HandlerOptions{Level: slog.LevelInfo})),
		PollInterval: 2 * time.Millisecond,
		FromSeq:      &fromSeq,
	})

	if err := runBridgeUntil(t, b, func() bool { return len(pap.calls()) >= 1 }); err != nil {
		t.Fatalf("bridge: %v", err)
	}

	if !strings.Contains(logLines.String(), "--from-seq override") {
		t.Errorf("startup log does not name the from-seq branch:\n%s", logLines.String())
	}

	for _, c := range pap.calls() {
		if c.IdempotencyKey != SourceApp+"-dlq-150" {
			t.Errorf("ingest key = %q, want dlq-150 replayed from before the checkpoint", c.IdempotencyKey)
		}
	}
}

// TestFirstRunBootstrapsAtHead pins the no-history-replay default: a first
// run inserts the head as its checkpoint and forwards nothing that
// predates the bridge.
func TestFirstRunBootstrapsAtHead(t *testing.T) {
	pap := newFakePap(t)
	wm := newFakeWatermarks()

	facts := fillFacts(100)
	facts[49] = dlFact(50, "t-dead-old") // predates the bridge

	src := &fakeSource{
		facts: facts,
		tasks: map[string]task.Task{
			"t-dead-old": {ID: task.ID("t-dead-old"), Project: "p", Type: "deploy"},
		},
	}

	b := New(src, wm, Config{
		Endpoint: pap.server.URL, Logger: quietLogger(), PollInterval: 2 * time.Millisecond,
	})

	if err := runBridgeUntil(t, b, func() bool {
		return wm.current("papdashboard:"+pap.server.URL) == 100
	}); err != nil {
		t.Fatalf("bridge: %v", err)
	}

	if got := len(pap.calls()); got != 0 {
		t.Fatalf("first run forwarded %d historical facts, want 0 (bootstrap is forward-only)", got)
	}
}

// TestResolveAfterRestartClosesPreRestartAlert is the second half of the
// volatility fix: the alert was raised by a previous bridge process, the
// DeadLettered fact is never re-delivered after the restart, and the
// completion still resolves the alert because the correlation is derived
// from the task's own fact trail, not process memory.
func TestResolveAfterRestartClosesPreRestartAlert(t *testing.T) {
	pap := newFakePap(t)
	wm := newFakeWatermarks()

	src := &fakeSource{
		facts: fillFacts(50),
		tasks: map[string]task.Task{
			"t-dead": {ID: task.ID("t-dead"), Project: "p", Type: "deploy"},
		},
	}

	// Bridge A: boots at head 50, then the task dead-letters and is
	// checkpointed past it.
	ba := New(src, wm, Config{
		Endpoint: pap.server.URL, Logger: quietLogger(), PollInterval: 2 * time.Millisecond,
	})

	ctxA, cancelA := context.WithCancel(context.Background())
	doneA := make(chan error, 1)

	go func() { doneA <- ba.Run(ctxA) }()

	if !waitFor(func() bool { return wm.current("papdashboard:"+pap.server.URL) == 50 }) {
		cancelA()
		<-doneA

		t.Fatal("bridge A never bootstrapped")
	}

	src.add(dlFact(51, "t-dead"))

	if !waitFor(func() bool {
		return len(pap.calls()) >= 1 && wm.current("papdashboard:"+pap.server.URL) >= 51
	}) {
		cancelA()
		<-doneA

		t.Fatal("bridge A never alerted on the dead letter")
	}

	// Crash. The task is rescued out-of-band and completes.
	cancelA()
	<-doneA

	src.add(journal.Fact{Seq: 52, TaskID: "t-dead", Type: journal.Completed})

	// Bridge B never saw the DeadLettered fact (the checkpoint is past
	// it); the derivation still finds it in the task's trail.
	bb := New(src, wm, Config{
		Endpoint: pap.server.URL, Logger: quietLogger(), PollInterval: 2 * time.Millisecond,
	})

	if err := runBridgeUntil(t, bb, func() bool { return len(pap.calls()) >= 2 }); err != nil {
		t.Fatalf("bridge B: %v", err)
	}

	calls := pap.calls()
	if calls[0].Event != "alert.triggered" || calls[0].IdempotencyKey != SourceApp+"-dlq-51" {
		t.Fatalf("first ingest = %s/%s, want alert.triggered/dlq-51", calls[0].Event, calls[0].IdempotencyKey)
	}

	if calls[1].Event != "alert.resolved" || calls[1].IdempotencyKey != SourceApp+"-resolve-52" {
		t.Fatalf("second ingest = %s/%s, want alert.resolved/resolve-52 (derived, not remembered)", calls[1].Event, calls[1].IdempotencyKey)
	}
}

// syncBuffer is a mutex-guarded bytes.Buffer for slog capture.
type syncBuffer struct {
	mu  *sync.Mutex
	buf strings.Builder
}

func (b *syncBuffer) Write(p []byte) (int, error) {
	b.mu.Lock()
	defer b.mu.Unlock()

	return b.buf.Write(p)
}

func (b *syncBuffer) String() string {
	b.mu.Lock()
	defer b.mu.Unlock()

	return b.buf.String()
}
