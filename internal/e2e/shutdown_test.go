//go:build unix

package e2e

import (
	"bufio"
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"
	"syscall"
	"testing"
	"time"

	"github.com/larsartmann/go-taskqueue/internal/journal"
	"github.com/larsartmann/go-taskqueue/internal/queue"
	"github.com/larsartmann/go-taskqueue/internal/queue/sqlite"
	"github.com/larsartmann/go-taskqueue/internal/task"
	"github.com/larsartmann/go-taskqueue/internal/webui"
)

// TestShutdownOrderingUnderSIGTERM pins the actor composition's teardown
// order with the REAL worker binary (runactor rollout): on SIGTERM the
// in-flight task's execution scope finishes and records its outcome, the
// alert bridge's checkpoint is flushed, and only then does the store
// close. Pinned by observable effects, not timestamps: the task claimed
// before the signal COMPLETED after it, and the bridge's persisted
// watermark covers the dead-letter fact it forwarded (a store-closed-first
// write would have failed and left the cursor behind).
func TestShutdownOrderingUnderSIGTERM(t *testing.T) {
	dir := t.TempDir()
	db := filepath.Join(dir, "q.db")

	var mu sync.Mutex

	ingests := 0

	alerted := make(chan struct{}, 16)

	pap := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body, _ := io.ReadAll(r.Body)

		if strings.Contains(string(body), "alert.triggered") {
			mu.Lock()
			ingests++
			mu.Unlock()

			alerted <- struct{}{}
		}

		w.WriteHeader(http.StatusOK)
	}))
	defer pap.Close()

	// The sleeper outlives the signal (execution scope): 1s of sleep
	// against a 30s task timeout. The failer dead-letters instantly so the
	// bridge has something to forward and checkpoint. Higher priority
	// claims the sleeper first.
	runCLI(t, db, "enqueue", "--type", "sh", "--payload", "sleep 1", "--priority", "9")
	runCLI(t, db, "enqueue", "--type", "sh", "--payload", "exit 1", "--max-attempts", "1")

	worker := exec.Command(tqBin, "worker",
		"--db", db, "--poll", "50ms", "--lease", "5s", "--task-timeout", "30s",
		"--alert-url", pap.URL, "--alert-poll", "100ms")

	worker.Stdout = io.Discard
	worker.Stderr = os.Stderr

	if err := worker.Start(); err != nil {
		t.Fatalf("start worker: %v", err)
	}

	t.Cleanup(func() {
		_ = worker.Process.Signal(syscall.SIGKILL)
		_, _ = worker.Process.Wait()
	})

	select {
	case <-alerted:
	case <-time.After(10 * time.Second):
		t.Fatal("bridge never forwarded the dead-letter alert")
	}

	waitForCond(t, 10*time.Second, "sleeper claimed and running", func() bool {
		return storeHasRunningPayload(t, db, `"sleep 1"`)
	})

	if err := worker.Process.Signal(syscall.SIGTERM); err != nil {
		t.Fatalf("signal worker: %v", err)
	}

	done := make(chan error, 1)

	go func() { done <- worker.Wait() }()

	select {
	case err := <-done:
		if err != nil {
			t.Fatalf("worker exit after SIGTERM = %v, want graceful 0", err)
		}
	case <-time.After(20 * time.Second):
		t.Fatal("worker did not exit after SIGTERM")
	}

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	s, err := sqlite.Open(db)
	if err != nil {
		t.Fatalf("open store after shutdown: %v", err)
	}
	defer s.Close()

	completed := false

	var dlSeq int64

	facts, err := s.Facts(ctx, 0, 0)
	if err != nil {
		t.Fatalf("facts: %v", err)
	}

	for _, f := range facts {
		if f.Type == journal.DeadLettered {
			dlSeq = f.Seq
		}

		if f.Type != journal.Completed {
			continue
		}

		tk, err := s.Get(ctx, task.ID(f.TaskID))
		if err != nil {
			t.Fatalf("get task %s: %v", f.TaskID, err)
		}

		if string(tk.Payload) == `"sleep 1"` {
			completed = true
		}
	}

	if !completed {
		t.Fatal("in-flight task did not complete after shutdown — execution scope was cancelled by the signal")
	}

	wm, _, err := s.Watermark(ctx, "papdashboard:"+pap.URL)
	if err != nil {
		t.Fatalf("read watermark: %v", err)
	}

	if wm < dlSeq {
		t.Fatalf(
			"bridge watermark = %d, below the dead-letter fact %d — checkpoint lost to shutdown ordering",
			wm,
			dlSeq,
		)
	}

	mu.Lock()
	got := ingests
	mu.Unlock()

	if got != 1 {
		t.Fatalf("stub dashboard saw %d alert.triggered ingests, want 1", got)
	}
}

// TestServeSSEClosesBeforeExit pins the serve half of the ordering:
// SIGTERM closes the event stream (client sees EOF) and the process exits
// gracefully.
func TestServeSSEClosesBeforeExit(t *testing.T) {
	dir := t.TempDir()
	db := filepath.Join(dir, "q.db")
	runCLI(t, db, "enqueue", "--type", "sh", "--payload", "echo hi")

	serve := exec.Command(tqBin, "serve", "--db", db, "--addr", "127.0.0.1:0")
	serve.Stdout = io.Discard

	stderr, err := serve.StderrPipe()
	if err != nil {
		t.Fatalf("stderr pipe: %v", err)
	}

	if err := serve.Start(); err != nil {
		t.Fatalf("start serve: %v", err)
	}

	t.Cleanup(func() {
		_ = serve.Process.Signal(syscall.SIGKILL)
		_, _ = serve.Process.Wait()
	})

	// The banner names the bound address ("http://127.0.0.1:PORT").
	addrCh := make(chan string, 1)

	go func() {
		sc := bufio.NewScanner(stderr)

		for sc.Scan() {
			line := sc.Text()

			if at, ok := strings.CutPrefix(line, webui.BannerPrefix); ok {
				addrCh <- strings.TrimSpace(strings.TrimSuffix(at, webui.BannerReadOnlySuffix))

				return
			}
		}

		addrCh <- ""
	}()

	var addr string

	select {
	case addr = <-addrCh:
	case <-time.After(5 * time.Second):
		t.Fatal("serve never printed its bound address")
	}

	if addr == "" {
		t.Fatal("could not parse serve address")
	}

	deadline := time.Now().Add(5 * time.Second)

	for time.Now().Before(deadline) {
		resp, err := http.Get("http://" + addr + "/healthz")
		if err == nil {
			resp.Body.Close()

			break
		}

		time.Sleep(50 * time.Millisecond)
	}

	streamClosed := make(chan error, 1)

	go func() {
		resp, err := http.Get("http://" + addr + "/api/events")
		if err != nil {
			streamClosed <- err

			return
		}

		defer resp.Body.Close()

		_, err = io.Copy(io.Discard, resp.Body) // returns when the server closes
		streamClosed <- err
	}()

	time.Sleep(200 * time.Millisecond) // let the stream establish

	if err := serve.Process.Signal(syscall.SIGTERM); err != nil {
		t.Fatalf("signal serve: %v", err)
	}

	select {
	case <-streamClosed:
	case <-time.After(10 * time.Second):
		t.Fatal("SSE stream did not close on SIGTERM")
	}

	done := make(chan error, 1)

	go func() { done <- serve.Wait() }()

	select {
	case err := <-done:
		if err != nil {
			t.Fatalf("serve exit = %v, want graceful 0", err)
		}
	case <-time.After(10 * time.Second):
		t.Fatal("serve did not exit after SIGTERM")
	}
}

func runCLI(t *testing.T, db string, args ...string) string {
	t.Helper()

	out, err := runWithTimeout(exec.Command(tqBin, append(args, "--db", db)...), 15*time.Second)
	if err != nil {
		t.Fatalf("tq %s: %v\n%s", strings.Join(args, " "), err, out)
	}

	return out
}

func waitForCond(t *testing.T, timeout time.Duration, what string, cond func() bool) {
	t.Helper()

	deadline := time.Now().Add(timeout)

	for time.Now().Before(deadline) {
		if cond() {
			return
		}

		time.Sleep(50 * time.Millisecond)
	}

	t.Fatalf("condition not reached: %s", what)
}

func storeHasRunningPayload(t *testing.T, db, payload string) bool {
	t.Helper()

	s, err := sqlite.Open(db)
	if err != nil {
		return false
	}
	defer s.Close()

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	running := task.Running

	tasks, err := s.List(ctx, queue.Filter{Status: &running})
	if err != nil {
		return false
	}

	for _, tk := range tasks {
		if string(tk.Payload) == payload {
			return true
		}
	}

	return false
}
