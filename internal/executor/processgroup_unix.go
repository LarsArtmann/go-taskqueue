//go:build !windows

package executor

import (
	"errors"
	"os/exec"
	"syscall"
)

// prepareProcessGroup puts the command in its own process group so a context
// cancellation can kill the whole tree (agents routinely spawn children).
func prepareProcessGroup(cmd *exec.Cmd) {
	cmd.SysProcAttr = &syscall.SysProcAttr{Setpgid: true}
	cmd.Cancel = func() error {
		if cmd.Process == nil {
			return nil
		}

		err := syscall.Kill(-cmd.Process.Pid, syscall.SIGKILL)
		if err != nil && !errors.Is(err, syscall.ESRCH) {
			return err
		}

		return nil
	}
}
