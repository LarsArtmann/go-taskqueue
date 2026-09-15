package executor

import (
	"context"
	"encoding/json/v2"
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/larsartmann/go-taskqueue/internal/task"
)

func depBumpTaskT(t *testing.T, payload DepBumpPayload) task.Task {
	t.Helper()

	raw, err := json.Marshal(payload)
	if err != nil {
		t.Fatalf("marshal payload: %v", err)
	}

	return task.Task{ID: task.NewID(), Type: TaskTypeDepBump, Payload: raw}
}

func TestIsStableSemver(t *testing.T) {
	t.Parallel()

	cases := []struct {
		version string
		want    bool
	}{
		{"v1.2.3", true},
		{"v0.0.1", true},
		{"v10.20.30", true},
		{"", false},
		{"1.2.3", false},   // missing v prefix
		{"v1.2", false},    // two segments
		{"v1.2.3.4", false}, // four segments
		{"v1.2.3-rc1", false},
		{"v1.2.3-dev", false},
		{"v1.2.x", false},
		{"v1.2.-3", false},
		{"0.0.0-dev", false},
	}

	for _, tc := range cases {
		if got := IsStableSemver(tc.version); got != tc.want {
			t.Errorf("IsStableSemver(%q) = %v, want %v", tc.version, got, tc.want)
		}
	}
}

func TestCommitMessageForBumps(t *testing.T) {
	t.Parallel()

	one := commitMessageForBumps([]DepBump{{Module: "example.com/a", Version: "v1.0.0"}})
	if one != "chore(deps): bump example.com/a to v1.0.0" {
		t.Errorf("single bump message = %q", one)
	}

	many := commitMessageForBumps([]DepBump{
		{Module: "example.com/b", Version: "v2.0.0"},
		{Module: "example.com/a", Version: "v1.0.0"},
	})
	if !strings.HasPrefix(many, "chore(deps): bump 2 modules (") ||
		strings.Index(many, "example.com/a") > strings.Index(many, "example.com/b") {
		t.Errorf("multi bump message must name sorted modules: %q", many)
	}
}

func TestDepBumpPayloadContract(t *testing.T) {
	t.Parallel()

	e := &DepBumpExecutor{}

	// Empty payload is permanent.
	err := e.Execute(context.Background(), task.Task{ID: task.NewID(), Type: TaskTypeDepBump})
	if !isPermanent(t, err) {
		t.Fatalf("empty payload must be permanent, got %v", err)
	}

	if !errors.Is(err, ErrDepBumpEmptyPayload) {
		t.Errorf("sentinel mismatch: %v", err)
	}

	// No repo: permanent.
	err = e.Execute(context.Background(), depBumpTaskT(t, DepBumpPayload{Bumps: []DepBump{{Module: "m", Version: "v1.0.0"}}}))
	if !isPermanent(t, err) {
		t.Fatalf("missing repo must be permanent, got %v", err)
	}

	// Neither bumps nor release: permanent.
	err = e.Execute(context.Background(), depBumpTaskT(t, DepBumpPayload{Repo: t.TempDir()}))
	if !isPermanent(t, err) || !errors.Is(err, ErrDepBumpNoWork) {
		t.Fatalf("no-work payload must be permanent ErrDepBumpNoWork, got %v", err)
	}

	// Unstable target: permanent.
	err = e.Execute(context.Background(), depBumpTaskT(t, DepBumpPayload{
		Repo:  t.TempDir(),
		Bumps: []DepBump{{Module: "m", Version: "v1.2.3-dev"}},
	}))
	if !isPermanent(t, err) || !errors.Is(err, ErrDepBumpBadVersion) {
		t.Fatalf("dev target must be permanent ErrDepBumpBadVersion, got %v", err)
	}

	// Payload contract version from the future: permanent.
	raw := `{"v":99,"repo":"/tmp","bumps":[{"module":"m","version":"v1.0.0"}]}`
	err = e.Execute(context.Background(), task.Task{ID: task.NewID(), Type: TaskTypeDepBump, Payload: []byte(raw)})
	if !isPermanent(t, err) || !strings.Contains(err.Error(), "upgrade tq") {
		t.Fatalf("future payload version must fail permanently with upgrade guidance, got %v", err)
	}
}

func isPermanent(t *testing.T, err error) bool {
	t.Helper()

	permanent, ok := errors.AsType[*PermanentError](err)

	return ok && permanent != nil
}
