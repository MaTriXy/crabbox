//go:build windows

package cli

import (
	"os"
	"os/exec"
)

func configureDaemonCommand(_ *exec.Cmd) {}

func stopDaemonProcess(process *os.Process) error {
	if process == nil {
		return nil
	}
	return process.Kill()
}
