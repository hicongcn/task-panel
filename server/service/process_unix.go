//go:build !windows

package service

import (
	"os"
	"os/exec"
	"syscall"
)

// setProcessGroup 把子进程放进独立进程组,便于整体 kill。
func setProcessGroup(cmd *exec.Cmd) {
	if cmd.SysProcAttr == nil {
		cmd.SysProcAttr = &syscall.SysProcAttr{}
	}
	cmd.SysProcAttr.Setpgid = true
}

// killProcessGroup 杀整个进程组(含子进程),再 kill 主进程兜底。
func killProcessGroup(p *os.Process) {
	if p == nil {
		return
	}
	_ = syscall.Kill(-p.Pid, syscall.SIGTERM)
	_ = p.Kill()
}
