package main

import (
	"errors"
	"fmt"
	"os"
)

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
