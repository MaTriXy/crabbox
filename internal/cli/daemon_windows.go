//go:build windows

package cli

import (
	"os"
	"os/exec"
)

func configureDaemonCommand(_ *exec.Cmd) {}

func stopDaemonProcess(process *os.Process, _ int) error {
	return process.Kill()
}
