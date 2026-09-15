// Package depsweep converges the ecosystem on the dependency plan computed
// by project-dependency-graph: it pulls the machine-readable
// release-overview plan, turns it into deterministic "depbump" tasks
// (executor.TaskTypeDepBump) chained by the plan's own DAG, and lets the
// queue's scheduling replace wave arithmetic. Releases are tagged (and,
// when configured, pushed) by the executor; consumers bump exact pins and
// verify before committing.
//
// The plan JSON is the ONLY seam to depgraph — this package never imports
// it. That keeps the contract explicit (a versioned wire format both tools
// pin in tests) and the dependency graph of the queue itself untouched.
//
// Loop safety mirrors the prioritize sweeper's structural rules: depbump
// tasks are never journal-triggered (the sweeper runs on a timer or on
// demand), and every mint carries a dedup key hashing the exact work spec,
// so an unchanged plan never mints twice. A permanently failed task lands
// in the DLQ and its dedup key suppresses remints until the plan changes —
// the DLQ is the human surface, same ruling as dead prioritize batches.
package depsweep

import (
	"bytes"
	"context"
	"encoding/json/v2"
	"fmt"
	"os/exec"
	"strings"
	"time"
)

// PlanModule is one own module in the release-overview JSON contract
// (subset the sweeper acts on; unknown fields are ignored on decode).
type PlanModule struct {
	Key   string `json:"key"`
	Label string `json:"label"`
	Dir   string `json:"dir"`

	HasUnreleasedWork bool   `json:"hasUnreleasedWork"`
	CurrentVersion    string `json:"currentVersion"`
	NextVersion       string `json:"nextVersion"`
	SuggestedBump     string `json:"suggestedBump"`

	HasOutdatedConsumers bool           `json:"hasOutdatedConsumers"`
	OutdatedConsumers    []PlanConsumer `json:"outdatedConsumers"`
	StaleBuild           bool           `json:"staleBuild"`
	StaleDeps            []PlanStaleDep `json:"staleDeps"`
	Wave                 int            `json:"wave"`
	ConsumerCount        int            `json:"consumerCount"`
	Dependencies         []PlanDepKey   `json:"dependencies"`
}

// PlanConsumer is one outdated consumer of a released module.
type PlanConsumer struct {
	Key            string `json:"key"`
	Dir            string `json:"dir"`
	CurrentVersion string `json:"currentVersion"`
	TargetVersion  string `json:"targetVersion"`
	DependencyKey  string `json:"dependencyKey"`
	IsMajorBump    bool   `json:"isMajorBump"`
}

// PlanStaleDep documents why a build is stale (informational: the release
// task does not re-apply these — they are already in the working tree).
type PlanStaleDep struct {
	Key        string `json:"key"`
	OldVersion string `json:"oldVersion"`
	NewVersion string `json:"newVersion"`
}

// PlanDepKey names one dependency module of a plan module.
type PlanDepKey struct {
	Key string `json:"key"`
}

// ParsePlan decodes a release-overview JSON payload.
func ParsePlan(data []byte) ([]PlanModule, error) {
	var plan struct {
		Modules []PlanModule `json:"modules"`
	}

	if err := json.Unmarshal(data, &plan); err != nil {
		return nil, fmt.Errorf("depsweep: parse plan: %w", err)
	}

	return plan.Modules, nil
}

// PlanSource produces the current plan. The interface keeps the sweeper
// testable against fixture plans and lets future sources (the depgraph
// daemon socket, a cached file) slot in without touching sweep logic.
type PlanSource interface {
	Plan(ctx context.Context) ([]PlanModule, error)
}

// DepgraphSource runs the project-dependency-graph binary and parses its
// release-overview JSON. Bin defaults to "project-dependency-graph";
// ExtraArgs lets callers pin flags (e.g. an org prefix) without a struct
// per flag.
type DepgraphSource struct {
	Bin       string
	Dir       string
	ExtraArgs []string
	Timeout   time.Duration
}

// defaultDepgraphTimeout bounds one plan computation.
const defaultDepgraphTimeout = 2 * time.Minute

// Plan runs the binary: <bin> release-overview --dir <dir> --format json.
func (s DepgraphSource) Plan(ctx context.Context) ([]PlanModule, error) {
	bin := s.Bin
	if bin == "" {
		bin = "project-dependency-graph"
	}

	timeout := s.Timeout
	if timeout <= 0 {
		timeout = defaultDepgraphTimeout
	}

	ctx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()

	args := make([]string, 0, 5+len(s.ExtraArgs))
	args = append(args, "release-overview", "--dir", s.Dir, "--format", "json")
	args = append(args, s.ExtraArgs...)

	cmd := exec.CommandContext(ctx, bin, args...)

	var out, errOut bytes.Buffer

	cmd.Stdout = &out
	cmd.Stderr = &errOut

	if err := cmd.Run(); err != nil {
		return nil, fmt.Errorf("depsweep: run %s: %w: %s", bin, err, tail(errOut.String()))
	}

	modules, err := ParsePlan(out.Bytes())
	if err != nil {
		return nil, err
	}

	return modules, nil
}

func tail(s string) string {
	lines := strings.Split(strings.TrimSpace(s), "\n")
	if len(lines) > 5 {
		lines = lines[len(lines)-5:]
	}

	return strings.TrimSpace(strings.Join(lines, "\n"))
}
