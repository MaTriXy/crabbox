//go:build !windows

package cli

import (
	"os"
	"os/exec"
	"syscall"
)

func configureDaemonCommand(cmd *exec.Cmd) {
	cmd.SysProcAttr = &syscall.SysProcAttr{Setpgid: true}
}

func stopDaemonProcess(process *os.Process) error {
	if process == nil {
		return nil
	}
	err := syscall.Kill(-process.Pid, syscall.SIGTERM)
	if err == syscall.ESRCH {
		return nil
	}
	return err
}
