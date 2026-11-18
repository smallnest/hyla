//go:build unix

package daemonhost

import (
	"os/exec"
	"syscall"
)

func setProcGroupAttr(cmd *exec.Cmd) (cleanup func(), err error) {
	cmd.SysProcAttr = &syscall.SysProcAttr{Setsid: true}
	return func() {}, nil
}

func attachWindowsJob(cmd *exec.Cmd) func() {
	return func() {}
}

func killProcessTree(cmd *exec.Cmd) {
	if cmd == nil || cmd.Process == nil {
		return
	}
	if syscall.Kill(-cmd.Process.Pid, syscall.SIGKILL) != nil {
		_ = cmd.Process.Kill()
	}
}
