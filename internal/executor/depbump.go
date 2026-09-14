package executor

import (
	"bytes"
	"context"
	"encoding/json/v2"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/larsartmann/go-taskqueue/internal/task"
)

// TaskTypeDepBump runs a deterministic dependency bump (and optional
// release) in one repository: baseline, go get each module at an EXACT
// version, tidy/vendor/templ fixups, verify, commit, tag (and optionally
// push). It is the executor side of the depsweep pipeline: the sweeper
// (internal/depsweep) turns a project-dependency-graph plan into depbump
// tasks chained by the plan's own DAG. Register it under this type to make
// a pool upgrade-capable.
const TaskTypeDepBump = "depbump"

// DepBump contract sentinels: call sites match with errors.Is.
var (
	// ErrDepBumpEmptyPayload pins the empty-payload rejection.
	ErrDepBumpEmptyPayload = errors.New("depbump: empty payload, want {repo, bumps|release}")
	// ErrDepBumpNoWork rejects payloads with neither bumps nor a release.
	ErrDepBumpNoWork = errors.New("depbump: payload needs at least one bump or a release")
	// ErrDepBumpBadVersion rejects unparsable/empty bump target versions.
	ErrDepBumpBadVersion = errors.New("depbump: bump target version is empty or not stable semver")
)

// DepBump is one exact module pin: go get <module>@<version>.
type DepBump struct {
	Module  string `json:"module"`
	Version string `json:"version"`
}

// DepBumpRelease cuts a release after the (optional) bumps verify: an
// annotated tag at the repo HEAD. Version is the exact tag ("v1.2.3"),
// resolved from the plan's nextVersion or patch-bumped by the sweeper —
// the executor never invents versions. Push additionally pushes master and
// the new tag, which is what makes the tag proxy-resolvable for consumers;
// it stays opt-in because pushes are the irreversible step.
type DepBumpRelease struct {
	Version string `json:"version"`
	Push    bool   `json:"push,omitempty"`
}

// DepBumpPayload is the payload contract for "depbump" tasks. Everything
// is deterministic: no model, no prompt — a fixed list of exact pins and
// an optional release. The mechanical gate is the executor's own verify
// (build + test) before anything is committed.
type DepBumpPayload struct {
	// Repo is the repository to bump: a name resolved against the
	// executor's ProjectsDir, or an absolute path. Required.
	Repo string `json:"repo"`
	// RepoName is the display name (task listings, logs).
	RepoName string `json:"repo_name,omitempty"`
	// Bumps are the exact pins to apply, in order. May be empty when the
	// task is release-only (the stale-build case: deps are already
	// committed, only the tag is missing).
	Bumps []DepBump `json:"bumps,omitempty"`
	// Release optionally tags (and pushes) after verification.
	Release *DepBumpRelease `json:"release,omitempty"`
	// V is the payload contract version. Zero decodes as v1; a version
	// above what this binary understands fails fast as a permanent error.
	V int `json:"v,omitempty"`
	// RequireClean refuses to start unless the repo's git tree is clean,
	// so the bump never interleaves with an agent's or human's WIP.
	// Default true.
	RequireClean *bool `json:"require_clean,omitempty"`
	// TimeoutMinutes caps each go command. Default 10; the worker's task
	// timeout still applies as the hard ceiling above it.
	TimeoutMinutes int `json:"timeout_minutes,omitempty"`
}

// currentDepBumpPayloadV is the highest payload contract version this
// binary understands.
const currentDepBumpPayloadV = 1

const defaultDepBumpCommandTimeout = 10 * time.Minute

// depbumpTouchedPaths are the paths a bump can modify; rollback and the
// commit stage exactly this scope, so concurrent changes outside it are
// never folded in and never reverted.
var depbumpTouchedPaths = []string{"go.mod", "go.sum", "vendor"}

// DepBumpExecutor applies DepBumpPayloads deterministically.
type DepBumpExecutor struct {
	// ProjectsDir resolves relative Repo names in payloads ("demo" →
	// <ProjectsDir>/demo). Absolute Repo paths bypass it.
	ProjectsDir string
	// ExtraEnv is appended to every spawned go/git environment (e.g.
	// GOEXPERIMENT=jsonv2 — the pool unit env does not carry it).
	ExtraEnv []string
	// GoBin and GitBin default to "go" and "git" from PATH.
	GoBin  string
	GitBin string
}

// NewDepBumpExecutor returns a DepBumpExecutor resolving repos under
// projectsDir.
func NewDepBumpExecutor(projectsDir string) *DepBumpExecutor {
	return &DepBumpExecutor{ProjectsDir: projectsDir}
}

func (e *DepBumpExecutor) goBin() string {
	if e.GoBin != "" {
		return e.GoBin
	}

	return "go"
}

// repoDir resolves a payload repo name: absolute paths pass through,
// relative names resolve against ProjectsDir (same rules as AgentExecutor).
func (e *DepBumpExecutor) repoDir(repo string) (string, error) {
	if filepath.IsAbs(repo) {
		if info, err := os.Stat(repo); err != nil || !info.IsDir() {
			return "", fmt.Errorf("depbump: repo directory does not exist: %s", repo)
		}

		return repo, nil
	}

	if e.ProjectsDir == "" {
		return "", fmt.Errorf(
			"depbump: relative repo %q needs a projects dir on the executor",
			repo,
		)
	}

	dir := filepath.Join(e.ProjectsDir, repo)

	info, err := os.Stat(dir)
	if err != nil || !info.IsDir() {
		return "", fmt.Errorf("depbump: repo %q does not exist under the projects dir", repo)
	}

	return dir, nil
}

func (e *DepBumpExecutor) gitBin() string {
	if e.GitBin != "" {
		return e.GitBin
	}

	return "git"
}

// Execute runs one bump task. Error classes follow the executor contract:
// payload/contract misses are Permanent; a dirty tree is a Preflight
// requeue (someone else holds the repo — retry later without burning an
// attempt); go failures are regular attempts (bounded retries, then DLQ).
// Any failure AFTER mutations rolls the touched paths back to HEAD so the
// repo is left clean for the retry.
func (e *DepBumpExecutor) Execute(ctx context.Context, t task.Task) error {
	var payload DepBumpPayload

	if len(t.Payload) == 0 {
		return Permanent(ErrDepBumpEmptyPayload)
	}

	if err := json.Unmarshal(t.Payload, &payload); err != nil {
		return Permanent(fmt.Errorf("depbump: decode payload: %w", err))
	}

	if payload.V > currentDepBumpPayloadV {
		return Permanent(fmt.Errorf(
			"depbump: payload contract version %d is newer than this binary understands (%d); upgrade tq",
			payload.V,
			currentDepBumpPayloadV,
		))
	}

	if payload.Repo == "" {
		return Permanent(errors.New("depbump: payload needs a repo"))
	}

	if len(payload.Bumps) == 0 && payload.Release == nil {
		return Permanent(ErrDepBumpNoWork)
	}

	for _, bump := range payload.Bumps {
		if bump.Module == "" || !IsStableSemver(bump.Version) {
			return Permanent(
				fmt.Errorf("%w: %s@%s", ErrDepBumpBadVersion, bump.Module, bump.Version),
			)
		}
	}

	if payload.Release != nil && !IsStableSemver(payload.Release.Version) {
		return Permanent(
			fmt.Errorf("%w: release %s", ErrDepBumpBadVersion, payload.Release.Version),
		)
	}

	repoDir, err := e.repoDir(payload.Repo)
	if err != nil {
		return Permanent(err)
	}

	if requireClean(AgentPayload{RequireClean: payload.RequireClean}) {
		if _, err := os.Stat(filepath.Join(repoDir, ".git")); err == nil {
			if err := assertCleanTree(ctx, repoDir); err != nil {
				return &PreflightError{Cause: err}
			}
		}
	}

	cmdTimeout := time.Duration(payload.TimeoutMinutes) * time.Minute
	if cmdTimeout <= 0 {
		cmdTimeout = defaultDepBumpCommandTimeout
	}

	if err := e.verify(ctx, repoDir, cmdTimeout, "baseline"); err != nil {
		return fmt.Errorf(
			"depbump: baseline failed in %s (pre-existing breakage, repo untouched): %w",
			payload.Repo,
			err,
		)
	}

	if len(payload.Bumps) > 0 {
		if err := e.applyBumps(ctx, repoDir, payload.Bumps, cmdTimeout); err != nil {
			return err // applyBumps rolled back already
		}
	}

	if err := e.verify(ctx, repoDir, cmdTimeout, "verify"); err != nil {
		_ = e.rollback(ctx, repoDir)
		return fmt.Errorf(
			"depbump: verify failed in %s (touched paths rolled back): %w",
			payload.Repo,
			err,
		)
	}

	if len(payload.Bumps) > 0 {
		if err := e.commit(ctx, repoDir, payload.Bumps); err != nil {
			_ = e.rollback(ctx, repoDir)
			return fmt.Errorf(
				"depbump: commit failed in %s (touched paths rolled back): %w",
				payload.Repo,
				err,
			)
		}
	}

	if payload.Release != nil {
		if err := e.release(ctx, repoDir, payload.Release); err != nil {
			return fmt.Errorf(
				"depbump: release failed in %s (bumps, if any, stay committed): %w",
				payload.Repo,
				err,
			)
		}
	}

	return nil
}

// verify runs build + test as the mechanical gate. label names the phase
// for error context (baseline vs verify).
func (e *DepBumpExecutor) verify(
	ctx context.Context,
	repoDir string,
	timeout time.Duration,
	label string,
) error {
	buildOut := filepath.Join(
		os.TempDir(),
		fmt.Sprintf("tq-depbump-build-%d", time.Now().UnixNano()),
	)

	if out, err := e.runGo(ctx, repoDir, timeout, "build", "-o", buildOut, "./..."); err != nil {
		return fmt.Errorf("%s build: %w: %s", label, err, tailOutput(out))
	}

	_ = os.RemoveAll(buildOut)

	if out, err := e.runGo(ctx, repoDir, timeout, "test", "./...", "-count=1"); err != nil {
		return fmt.Errorf("%s test: %w: %s", label, err, tailOutput(out))
	}

	return nil
}

// applyBumps pins every module at its exact version, then runs the fixups
// (tidy, vendor, templ). Any failure rolls the touched paths back and
// returns the error (regular attempt: a later retry re-runs cleanly).
func (e *DepBumpExecutor) applyBumps(
	ctx context.Context,
	repoDir string,
	bumps []DepBump,
	timeout time.Duration,
) error {
	restoreWork, err := e.quarantineGoWork(repoDir)
	if err != nil {
		return fmt.Errorf("depbump: go.work quarantine failed: %w", err)
	}

	defer restoreWork()

	for _, bump := range bumps {
		if out, err := e.runGo(ctx, repoDir, timeout, "get", bump.Module+"@"+bump.Version); err != nil {
			_ = e.rollback(ctx, repoDir)
			return fmt.Errorf("depbump: go get %s@%s failed (rolled back): %w: %s",
				bump.Module, bump.Version, err, tailOutput(out))
		}
	}

	if out, err := e.runGo(ctx, repoDir, timeout, "mod", "tidy"); err != nil {
		_ = e.rollback(ctx, repoDir)
		return fmt.Errorf("depbump: go mod tidy failed (rolled back): %w: %s", err, tailOutput(out))
	}

	if _, err := os.Stat(filepath.Join(repoDir, "vendor")); err == nil {
		if out, err := e.runGo(ctx, repoDir, timeout, "mod", "vendor"); err != nil {
			_ = e.rollback(ctx, repoDir)
			return fmt.Errorf(
				"depbump: go mod vendor failed (rolled back): %w: %s",
				err,
				tailOutput(out),
			)
		}
	}

	if err := e.regenerateTempl(ctx, repoDir); err != nil {
		_ = e.rollback(ctx, repoDir)
		return fmt.Errorf("depbump: templ regenerate failed (rolled back): %w", err)
	}

	// The pin must have actually landed — go get can report success and
	// change nothing under workspaces.
	for _, bump := range bumps {
		pinned, err := e.pinnedVersion(ctx, repoDir, timeout, bump.Module)
		if err != nil {
			_ = e.rollback(ctx, repoDir)

			return fmt.Errorf(
				"depbump: pin check failed for %s (rolled back): %w",
				bump.Module,
				err,
			)
		}

		if pinned != bump.Version {
			_ = e.rollback(ctx, repoDir)

			return fmt.Errorf(
				"depbump: %s pinned at %q, want %q (rolled back)",
				bump.Module,
				pinned,
				bump.Version,
			)
		}
	}

	return nil
}

// regenerateTempl re-runs the templ generator when the repo has .templ
// sources; missing templ binary is a soft skip (the delivering layer is
// then unverified and the commit gate says so via the message suffix).
func (e *DepBumpExecutor) regenerateTempl(ctx context.Context, repoDir string) error {
	hasTempl := false

	err := filepath.WalkDir(repoDir, func(_ string, d os.DirEntry, err error) error {
		if err != nil || !d.IsDir() {
			return err
		}

		if d.Name() == ".git" || d.Name() == "vendor" || d.Name() == "node_modules" {
			return filepath.SkipDir
		}

		entries, err := os.ReadDir(filepath.Join(repoDir, d.Name()))
		if err != nil {
			return nil //nolint:nilerr // unreadable dir is not fatal for detection
		}

		for _, entry := range entries {
			if !entry.IsDir() && strings.HasSuffix(entry.Name(), ".templ") {
				hasTempl = true

				return filepath.SkipAll
			}
		}

		return nil
	})
	if err != nil || !hasTempl {
		return nil //nolint:nilerr // detection walk errors degrade to skip
	}

	bin, err := exec.LookPath("templ")
	if err != nil {
		return nil //nolint:nilerr // no templ on PATH: soft skip, documented
	}

	cmd := exec.CommandContext(ctx, bin, "generate")
	cmd.Dir = repoDir

	var out bytes.Buffer

	cmd.Stdout = &out
	cmd.Stderr = &out
	cmd.Env = append(os.Environ(), e.ExtraEnv...)

	if err := cmd.Run(); err != nil {
		return fmt.Errorf("%w: %s", err, tailOutput(out.String()))
	}

	return nil
}

// commit stages exactly the touched paths scope and commits with a
// conventional message naming every bump.
func (e *DepBumpExecutor) commit(ctx context.Context, repoDir string, bumps []DepBump) error {
	args := []string{"-C", repoDir, "add", "--"}

	if _, err := os.Stat(filepath.Join(repoDir, "vendor")); err == nil {
		args = append(args, depbumpTouchedPaths...)
		args = append(args, "vendor")
		args = append(args, "*_templ.go", "*_templ.txt")
	} else {
		args = append(args, "go.mod", "go.sum", "*_templ.go", "*_templ.txt")
	}

	if out, err := e.runGit(ctx, args...); err != nil {
		return fmt.Errorf("stage: %w: %s", err, tailOutput(out))
	}

	message := commitMessageForBumps(bumps)

	if out, err := e.runGit(ctx, "-C", repoDir, "commit", "-m", message); err != nil {
		return fmt.Errorf("commit: %w: %s", err, tailOutput(out))
	}

	return nil
}

// commitMessageForBumps renders the conventional commit line for a bump
// set: short when one module, summarized when many.
func commitMessageForBumps(bumps []DepBump) string {
	if len(bumps) == 1 {
		return fmt.Sprintf("chore(deps): bump %s to %s", bumps[0].Module, bumps[0].Version)
	}

	names := make([]string, 0, len(bumps))
	for _, bump := range bumps {
		names = append(names, bump.Module)
	}

	sort.Strings(names)

	return fmt.Sprintf("chore(deps): bump %d modules (%s)", len(bumps), strings.Join(names, ", "))
}

// release tags the repo HEAD with the exact version (dir-prefixed for
// monorepo sub-modules, matching depgraph's tag resolution) and optionally
// pushes.
func (e *DepBumpExecutor) release(ctx context.Context, repoDir string, rel *DepBumpRelease) error {
	root, subdir, err := e.gitRootAndSubdir(repoDir)
	if err != nil {
		return fmt.Errorf("resolve git root: %w", err)
	}

	tagName := rel.Version
	if subdir != "" {
		tagName = subdir + "/" + rel.Version
	}

	if out, err := e.runGit(ctx, "-C", root, "rev-parse", "-q", "--verify", "refs/tags/"+tagName); err == nil &&
		out != "" {
		return nil //nolint:nilerr // tag exists: idempotent success
	}

	if out, err := e.runGit(ctx, "-C", root, "tag", "-a", tagName, "-m", "release "+tagName); err != nil {
		return fmt.Errorf("tag %s: %w: %s", tagName, err, tailOutput(out))
	}

	if rel.Push {
		if out, err := e.runGit(ctx, "-C", root, "push", "--follow-tags", "origin", "HEAD"); err != nil {
			return fmt.Errorf("push: %w: %s", err, tailOutput(out))
		}
	}

	return nil
}

// rollback restores the touched-path scope to HEAD so a failed attempt
// leaves the repo exactly as it started.
func (e *DepBumpExecutor) rollback(ctx context.Context, repoDir string) error {
	args := []string{"-C", repoDir, "checkout", "HEAD", "--"}
	args = append(args, depbumpTouchedPaths...)

	if _, err := os.Stat(filepath.Join(repoDir, "vendor")); err == nil {
		args = append(args, "*_templ.go", "*_templ.txt")
	} else {
		args = append(args, "*_templ.go", "*_templ.txt")
	}

	_, _ = e.runGit(ctx, args...)

	return nil
}

// quarantineGoWork renames go.work to go.work.bak for the duration of the
// bump: GOWORK=off does not isolate go get (it can write go.work.sum and
// report success while changing nothing). The returned restore function
// puts it back.
func (e *DepBumpExecutor) quarantineGoWork(repoDir string) (func(), error) {
	noop := func() {}

	workFile := filepath.Join(repoDir, "go.work")

	if _, err := os.Stat(workFile); err != nil {
		return noop, nil //nolint:nilerr // absent go.work: nothing to quarantine
	}

	if err := os.Rename(workFile, workFile+".bak"); err != nil {
		return noop, fmt.Errorf("rename go.work: %w", err)
	}

	return func() { _ = os.Rename(workFile+".bak", workFile) }, nil
}

// pinnedVersion reads the currently pinned version of module from go.mod.
func (e *DepBumpExecutor) pinnedVersion(
	ctx context.Context,
	repoDir string,
	timeout time.Duration,
	module string,
) (string, error) {
	out, err := e.runGo(ctx, repoDir, timeout, "mod", "edit", "-json")
	if err != nil {
		return "", fmt.Errorf("go mod edit -json: %w: %s", err, tailOutput(out))
	}

	var goMod struct {
		Require []struct {
			Path    string `json:"Path"`
			Version string `json:"Version"`
		} `json:"Require"`
	}

	if err := json.Unmarshal([]byte(out), &goMod); err != nil {
		return "", fmt.Errorf("parse go mod json: %w", err)
	}

	for _, req := range goMod.Require {
		if req.Path == module {
			return req.Version, nil
		}
	}

	return "", fmt.Errorf("module %s not required in go.mod after go get", module)
}

// gitRootAndSubdir returns the git worktree root and, when the module dir
// is a sub-directory, its slash-separated path relative to the root (used
// for dir-prefixed release tags).
func (e *DepBumpExecutor) gitRootAndSubdir(repoDir string) (string, string, error) {
	out, err := e.runGit(context.Background(), "-C", repoDir, "rev-parse", "--show-toplevel")
	if err != nil {
		return "", "", fmt.Errorf("rev-parse: %w: %s", err, tailOutput(out))
	}

	root := strings.TrimSpace(out)
	if root == "" {
		return "", "", errors.New("empty git root")
	}

	abs, err := filepath.Abs(repoDir)
	if err != nil {
		return "", "", fmt.Errorf("abs repo dir: %w", err)
	}

	rel, err := filepath.Rel(root, abs)
	if err != nil {
		return "", "", fmt.Errorf("rel path: %w", err)
	}

	if rel == "." {
		return root, "", nil
	}

	return root, filepath.ToSlash(rel), nil
}

// runGo runs the go binary in repoDir with the executor's extra env.
func (e *DepBumpExecutor) runGo(
	ctx context.Context,
	repoDir string,
	timeout time.Duration,
	args ...string,
) (string, error) {
	if _, ok := ctx.Deadline(); !ok {
		var cancel context.CancelFunc

		ctx, cancel = context.WithTimeout(ctx, timeout)
		defer cancel()
	} else if deadline, _ := ctx.Deadline(); time.Until(deadline) > timeout {
		var cancel context.CancelFunc

		ctx, cancel = context.WithTimeout(ctx, timeout)
		defer cancel()
	}

	return e.run(ctx, e.goBin(), repoDir, args...)
}

// runGit runs the git binary with the executor's extra env.
func (e *DepBumpExecutor) runGit(ctx context.Context, args ...string) (string, error) {
	dir := ""
	if len(args) >= 2 && args[0] == "-C" {
		dir = args[1]
		args = args[2:]
	}

	return e.run(ctx, e.gitBin(), dir, args...)
}

func (e *DepBumpExecutor) run(
	ctx context.Context,
	bin, dir string,
	args ...string,
) (string, error) {
	cmd := exec.CommandContext(ctx, bin, args...)
	if dir != "" {
		cmd.Dir = dir
	}

	var out bytes.Buffer

	cmd.Stdout = &out
	cmd.Stderr = &out
	cmd.Env = append(os.Environ(), e.ExtraEnv...)

	err := cmd.Run()

	return out.String(), err
}

func tailOutput(out string) string {
	lines := strings.Split(strings.TrimSpace(out), "\n")
	if len(lines) > 12 {
		lines = lines[len(lines)-12:]
	}

	s := strings.Join(lines, "\n")

	if len(s) > 2048 {
		s = s[len(s)-2048:]
	}

	return strings.TrimSpace(s)
}

// IsStableSemver reports whether v is a non-empty stable semver version
// ("v1.2.3" form, no pre-release/build suffixes). Bump targets must be
// stable: dev/-rc targets are exactly the class the sweeper refuses.
func IsStableSemver(v string) bool {
	if v == "" || !strings.HasPrefix(v, "v") {
		return false
	}

	parts := strings.Split(strings.TrimPrefix(v, "v"), ".")
	if len(parts) != 3 {
		return false
	}

	for _, part := range parts {
		if part == "" || strings.TrimLeft(part, "0123456789") != "" {
			return false
		}
	}

	return true
}
