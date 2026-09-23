package executor

import (
	"context"
	"os"
	"path/filepath"
	"time"
)

// prepareRepo resolves the repo directory and, when requireClean holds,
// refuses a dirty working tree: a missing repo is permanent, a dirty tree
// is a PreflightError (the worker requeues without burning an attempt;
// repos without .git skip the guard).
func prepareRepo(ctx context.Context, a *AgentExecutor, repo string, requireClean bool) (string, error) {
	repoDir, err := a.repoDir(repo)
	if err != nil {
		return "", Permanent(err)
	}

	if requireClean {
		if _, err := os.Stat(filepath.Join(repoDir, ".git")); err == nil {
			if err := assertCleanTree(ctx, repoDir); err != nil {
				return "", &PreflightError{Cause: err}
			}
		}
	}

	return repoDir, nil
}

// payloadTimeout resolves the effective run timeout: the payload override
// in whole minutes when positive, the executor's default otherwise.
func payloadTimeout(defaultTimeout time.Duration, minutes int) time.Duration {
	if minutes > 0 {
		return time.Duration(minutes) * time.Minute
	}

	return defaultTimeout
}
