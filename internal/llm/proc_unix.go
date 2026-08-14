//go:build unix

package llm

import (
	"os/exec"
	"strconv"
	"syscall"
	"time"
)

func configureServeCmd(cmd *exec.Cmd) {
	cmd.SysProcAttr = &syscall.SysProcAttr{Setpgid: true}
}

func killProcessTree(pid int) {
	if pid <= 0 {
		return
	}
	_ = exec.Command("pkill", "-TERM", "-P", strconv.Itoa(pid)).Run()
	_ = syscall.Kill(-pid, syscall.SIGTERM)
	_ = syscall.Kill(pid, syscall.SIGTERM)
	deadline := time.Now().Add(1500 * time.Millisecond)
	for time.Now().Before(deadline) {
		if err := syscall.Kill(pid, 0); err != nil {
			return
		}
		time.Sleep(50 * time.Millisecond)
	}
	_ = exec.Command("pkill", "-KILL", "-P", strconv.Itoa(pid)).Run()
	_ = syscall.Kill(-pid, syscall.SIGKILL)
	_ = syscall.Kill(pid, syscall.SIGKILL)
}
