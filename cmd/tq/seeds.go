package main

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
)

// allReposAbsolute reports whether every comma-separated entry in spec is an
// absolute path; a fully-absolute --repos list needs no projects dir.
func allReposAbsolute(spec string) bool {
	repos := splitRepos(spec)
	if len(repos) == 0 {
		return false
	}

	for _, repo := range repos {
		if !filepath.IsAbs(repo) {
			return false
		}
	}

	return true
}

// checkProjectsDir refuses project roots that would turn an unattended pool
// into a whole-machine agent sweep: "/" and the home directory both look
// plausible on the command line and both are almost always a mistake (every
// repo with a TODO_LIST.md becomes billable work).
func checkProjectsDir(dir string) error {
	if dir == "/" {
		return errors.New(
			"--projects-dir / would harvest the whole filesystem; name a directory that contains only repos you want agents to touch",
		)
	}

	if dir == "" {
		return nil
	}

	home, err := os.UserHomeDir()
	if err == nil && home != "" && dir == home {
		return fmt.Errorf(
			"--projects-dir %s is your home directory; point at the subdirectory holding the projects (e.g. %s/projects)",
			home,
			home,
		)
	}

	return nil
}
