//go:build unix

package main

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

// TestRunCrushServerActorDisarmsIdleShutdown pins the keeper's idle-shutdown
// disarm (plan 2026-11-23_00-57, M2): the server process must spawn with
// CRUSH_SERVER_IDLE_TIMEOUT in its env — on crush v0.102.0's idle model
// (upstream 2923a2f0) a plain GET holds nothing, so without the knob the
// server exits 60 s into every dispatch gap and the restart ladder churns.
// The stub "server" records its env and idles until the pool context tears
// it down (the actor's clean-exit path).
func TestRunCrushServerActorDisarmsIdleShutdown(t *testing.T) {
	dir := t.TempDir()
	envFile := filepath.Join(dir, "server-env")
	stub := filepath.Join(dir, "crush-server-stub")
	t.Setenv("TQ_TEST_SERVER_ENVFILE", envFile)

	body := "#!/bin/sh\n" +
		"printf '%s\\n' \"$CRUSH_SERVER_IDLE_TIMEOUT\" > \"$TQ_TEST_SERVER_ENVFILE\"\n" +
		"while :; do sleep 1; done\n"
	if err := os.WriteFile(stub, []byte(body), 0o755); err != nil {
		t.Fatal(err)
	}

	ctx, cancel := context.WithCancel(context.Background())

	done := make(chan error, 1)
	go func() {
		done <- runCrushServerActor(ctx, stub, filepath.Join(dir, "crush.sock"))
	}()

	deadline := time.Now().Add(5 * time.Second)
	for {
		if _, err := os.Stat(envFile); err == nil {
			break
		}

		if time.Now().After(deadline) {
			cancel()
			t.Fatal("stub server never recorded its env")
		}

		time.Sleep(50 * time.Millisecond)
	}

	cancel()

	select {
	case err := <-done:
		if err != nil {
			t.Fatalf("actor must exit cleanly on pool shutdown, got %v", err)
		}
	case <-time.After(10 * time.Second):
		t.Fatal("actor did not return after pool shutdown")
	}

	got, err := os.ReadFile(envFile)
	if err != nil {
		t.Fatal(err)
	}

	if trimmed := strings.TrimSpace(string(got)); trimmed != "43200" {
		t.Fatalf("server env CRUSH_SERVER_IDLE_TIMEOUT = %q, want 43200", trimmed)
	}
}
