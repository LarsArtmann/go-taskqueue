//go:build windows

package executor

import "os/exec"

// prepareProcessGroup is a no-op on Windows: there is no portable process
// group kill; the context kill of the direct child still applies.
//
// art-dupl:accept stdlib type plumbing: the unix/windows pair must each
// restate the *exec.Cmd parameter to provide the platform split.
func prepareProcessGroup(*exec.Cmd) {}
