// Package docsgates is the docs-gates bridge sweeper: on a timer it runs
// `docs-gates run <repo> --json` per pool repo and mints ONE fix task per
// FAIL gate, dedup-keyed `docs-gates:<repo>:<gate>` — fleet doc-drift
// findings flow into the queue the way incident reports do (ADR-0021
// pattern). A journal watermark cursor ("docsgates-bridge" in the
// watermarks table, the internal/watermark pattern) checkpoints the
// journal head after every fully-successful sweep pass, so `tq watermarks
// show` pins when the fleet's docs were last assessed and a rewind replays
// the same deduped mints.
package docsgates

import (
	"bytes"
	"context"
	"encoding/json/jsontext"
	"encoding/json/v2"
	"errors"
	"fmt"
	"log/slog"
	"os/exec"
	"strings"
	"time"

	"github.com/larsartmann/go-taskqueue/internal/executor"
	"github.com/larsartmann/go-taskqueue/internal/queue"
	"github.com/larsartmann/go-taskqueue/internal/task"
)

// WatermarkKey is the bridge's identity in the watermarks table, shared by
// every bridge over the same database.
const WatermarkKey = "docsgates-bridge"

// DefaultPriority places minted fix tasks in the machine band: operational
// doc-drift work, not user backlog (the dep-sweep ruling).
const DefaultPriority = queue.MachineMin

// DefaultMaxAttempts bounds burned failures on a minted fix task — a repo
// with an unfixable gate must not drain the attempt budget.
const DefaultMaxAttempts = 2

// DefaultTimeout bounds one repo's docs-gates run.
const DefaultTimeout = 5 * time.Minute

// Repo is one pool repo the bridge assesses.
type Repo struct {
	// Name is the project key (dedup identity, AgentPayload.Repo resolved
	// against the pool's projects dir).
	Name string
	// Dir is the working directory the binary runs in. Empty means the
	// name resolves against the executor's ProjectsDir as-is.
	Dir string
}

// Gate is one gate verdict from the `docs-gates run --json` matrix.
type Gate struct {
	Name    string `json:"name"`
	Verdict string `json:"verdict"`
	Skipped bool   `json:"skipped"`
}

// RunResult is the parsed `docs-gates run --json` document.
type RunResult struct {
	Pass  bool   `json:"pass"`
	Gates []Gate `json:"gates"`
}

// Runner assesses one repo, returning its verdict matrix. The production
// runner shells out to the docs-gates binary; tests inject stubs.
type Runner func(ctx context.Context, repo Repo) (RunResult, error)

// BridgeConfig controls one Bridge.
type BridgeConfig struct {
	// Store enqueues fix tasks and owns the watermark row. Required.
	Store queue.Store
	// Runner assesses a repo. nil selects the binary runner (Bin).
	Runner Runner
	// Bin is the docs-gates binary (PATH name or absolute) for the
	// default runner.
	Bin string
	// Timeout bounds one repo run; 0 selects the default.
	Timeout time.Duration
	// Repos are the assessed pool repos.
	Repos []Repo
	// Priority mints fix tasks; 0 selects the default (machine band).
	Priority int
	// MaxAttempts bounds minted fix tasks; 0 selects the default.
	MaxAttempts int
	// Log receives mint/skip lines. nil logs nothing.
	Log *slog.Logger
}

// Bridge is the timer-driven docs-gates sweeper. Safe for concurrent use
// is not claimed: wire ONE tick per interval, like every mint pass.
type Bridge struct {
	store queue.Store
	run   Runner
	cfg   BridgeConfig
}

// NewBridge returns a bridge over store. A nil Runner falls back to the
// Bin runner; a zero Timeout/Priority/MaxAttempts selects its default.
func NewBridge(cfg BridgeConfig) *Bridge {
	if cfg.Runner == nil {
		cfg.Runner = binaryRunner(cfg.Bin, cfg.timeout())
	}

	if cfg.Priority <= 0 {
		cfg.Priority = DefaultPriority
	}

	if cfg.MaxAttempts <= 0 {
		cfg.MaxAttempts = DefaultMaxAttempts
	}

	return &Bridge{store: cfg.Store, run: cfg.Runner, cfg: cfg}
}

func (c BridgeConfig) timeout() time.Duration {
	if c.Timeout <= 0 {
		return DefaultTimeout
	}

	return c.Timeout
}

func binaryRunner(bin string, timeout time.Duration) Runner {
	if bin == "" {
		bin = "docs-gates"
	}

	return func(ctx context.Context, repo Repo) (RunResult, error) {
		dir := repo.Dir
		if dir == "" {
			dir = "."
		}

		runCtx, cancel := context.WithTimeout(ctx, timeout)
		defer cancel()

		cmd := exec.CommandContext(runCtx, bin, "run", dir, "--json")
		cmd.Dir = dir

		// Capture stdout ourselves: the binary exits 1 (ExitFindings)
		// whenever a gate FAILs, and the matrix on stdout is the verdict —
		// an ExitError whose output parses is a result, not a failure.
		var stdout bytes.Buffer

		cmd.Stdout = &stdout

		runErr := cmd.Run()

		var res RunResult
		if perr := json.Unmarshal(stdout.Bytes(), &res); perr != nil {
			if runErr == nil {
				return RunResult{}, fmt.Errorf("docsgates: parse %s output for %s: %w", bin, repo.Name, perr)
			}

			return RunResult{}, fmt.Errorf("docsgates: run %s in %s: %w", bin, dir, runErr)
		}

		return res, nil
	}
}

// Stats summarizes one sweep pass.
type Stats struct {
	// Repos is the number of repos assessed.
	Repos int
	// Failed counts gates that FAILed (the mint candidates).
	Failed int
	// Minted counts fresh fix tasks enqueued.
	Minted int
	// Known counts mints whose task already existed (dedup hit).
	Known int
	// Skipped counts SKIPped gates (not assessed this run — never minted).
	Skipped int
}

// DedupKey is the mint identity: one fix task per repo+gate, converged
// across passes.
func DedupKey(repo Repo, gate Gate) string {
	return fmt.Sprintf("docs-gates:%s:%s", repo.Name, gate.Name)
}

// Cursor returns the persisted journal-head cursor of the last
// fully-successful sweep pass. The row is written only AFTER a pass
// completes, so the cursor is ABSENT (exists=false) until the first
// fully-successful sweep — there is no bootstrap-at-head.
func (b *Bridge) Cursor(ctx context.Context) (int64, bool, error) {
	return b.store.Watermark(ctx, WatermarkKey)
}

// Sweep assesses every configured repo and mints one fix task per FAIL
// gate. A repo runner failure aborts the pass WITHOUT checkpointing the
// cursor — an unassessed repo must not read as covered. Mint failures are
// per-gate: the pass continues (the next tick re-hits the dedup key), but
// the sweep returns the first mint error after checkpointing.
func (b *Bridge) Sweep(ctx context.Context) (Stats, error) {
	stats := Stats{Repos: len(b.cfg.Repos)}

	var mintErr error

	// Index existing fix tasks by dedup key: Enqueue's dedup hit returns
	// the stored task with no "already existed" signal (the depsweep
	// pattern), so mints from EARLIER passes count as known here (one
	// list query per sweep).
	agentType := executor.TaskTypeAgent

	existing, err := b.store.List(ctx, queue.Filter{Type: &agentType})
	if err != nil {
		return stats, fmt.Errorf("docsgates: list existing tasks: %w", err)
	}

	knownKeys := make(map[string]bool, len(existing))
	for i := range existing {
		if strings.HasPrefix(existing[i].DedupKey, "docs-gates:") {
			knownKeys[existing[i].DedupKey] = true
		}
	}

	for _, repo := range b.cfg.Repos {
		res, err := b.run(ctx, repo)
		if err != nil {
			return stats, fmt.Errorf("docsgates: assess %s: %w", repo.Name, err)
		}

		for _, gate := range res.Gates {
			if gate.Skipped {
				stats.Skipped++

				continue
			}

			if gate.Verdict != "FAIL" {
				continue
			}

			stats.Failed++

			minted, err := b.mint(ctx, knownKeys, repo, gate)
			if err != nil {
				if b.cfg.Log != nil {
					b.cfg.Log.Warn("docsgates mint failed", "repo", repo.Name, "gate", gate.Name, "err", err)
				}

				if mintErr == nil {
					mintErr = err
				}

				continue
			}

			if minted {
				stats.Minted++
			} else {
				stats.Known++
			}
		}
	}

	head, err := b.store.HeadSeq(ctx)
	if err != nil {
		return stats, fmt.Errorf("docsgates: read journal head: %w", err)
	}

	if err := b.store.SaveWatermark(ctx, WatermarkKey, head); err != nil {
		return stats, fmt.Errorf("docsgates: checkpoint watermark: %w", err)
	}

	return stats, mintErr
}

// mint enqueues one fix task for the failed gate; returns false on a dedup
// hit (the task already exists — ErrTaskDone from a completed fix is the
// system saying the drift is fixed, never a failure).
func (b *Bridge) mint(ctx context.Context, knownKeys map[string]bool, repo Repo, gate Gate) (bool, error) {
	payload, err := json.Marshal(executor.AgentPayload{
		Repo:   repo.Name,
		Prompt: fixPrompt(repo, gate),
		Verify: verifyCommand(),
		Dedup:  DedupKey(repo, gate),
		Item:   "docs-gates: " + repo.Name + " gate " + gate.Name + " FAILed",
	})
	if err != nil {
		return false, fmt.Errorf("docsgates: encode payload for %s/%s: %w", repo.Name, gate.Name, err)
	}

	_, err = queue.New(b.store).Enqueue(ctx, task.New{
		Project:     repo.Name,
		Type:        executor.TaskTypeAgent,
		Payload:     jsontext.Value(payload),
		Priority:    b.cfg.Priority,
		MaxAttempts: b.cfg.MaxAttempts,
		DedupKey:    DedupKey(repo, gate),
	})
	if err != nil {
		if errors.Is(err, queue.ErrTaskDone) {
			return false, nil
		}

		return false, fmt.Errorf("docsgates: enqueue fix task for %s/%s: %w", repo.Name, gate.Name, err)
	}

	if knownKeys[DedupKey(repo, gate)] {
		return false, nil
	}

	if b.cfg.Log != nil {
		b.cfg.Log.Info("docsgates minted fix task", "repo", repo.Name, "gate", gate.Name)
	}

	return true, nil
}

// fixPrompt is the OUTCOME contract for the fix agent: rails only — what
// must be true when the run ends, never the recipe.
func fixPrompt(repo Repo, gate Gate) string {
	return "The docs-gates gate `" + gate.Name + "` FAILs in this repo. Fix the underlying doc drift it reports " +
		"(run `docs-gates help " + gate.Name + "` for the gate's contract; findings name the offending files). " +
		"Do NOT weaken .docs-gates.json thresholds or add skip_gates to make the gate green unless the finding is " +
		"genuinely out of the gate's scope — a narrowed gate must be a separate, owner-visible change. " +
		"Outcome: `docs-gates run .` exits 0 (every gate PASS or SKIP) on a clean tree, committed."
}

// verifyCommand is the minted task's enforced quality gate: the battery
// itself must green in the repo.
func verifyCommand() string {
	return "docs-gates run ."
}
