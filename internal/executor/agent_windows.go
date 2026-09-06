//go:build windows

package executor

import "os/exec"

// prepareProcessGroup is a no-op on Windows: there is no portable process
// group kill; the context kill of the direct child still applies.
func prepareProcessGroup(*exec.Cmd) {}
