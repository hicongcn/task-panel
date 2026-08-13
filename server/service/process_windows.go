//go:build windows

package service

import (
	"os"
	"os/exec"
)

// setProcessGroup: Windows 上不设进程组,直接用进程级 kill。
func setProcessGroup(cmd *exec.Cmd) {}

// killProcessGroup 在 Windows 上退化为杀主进程。
func killProcessGroup(p *os.Process) {
	if p == nil {
		return
	}
	_ = p.Kill()
}
