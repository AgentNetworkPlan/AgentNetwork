//go:build !windows

package daemon

import (
	"os/exec"
	"syscall"
)

// setSysProcAttr 设置 Unix 平台特定的进程属性
func setSysProcAttr(cmd *exec.Cmd) {
	cmd.SysProcAttr = &syscall.SysProcAttr{
		Setsid: true,
	}
}

// isProcessRunning 检查进程是否运行 (Unix)
func isProcessRunning(pid int) bool {
	// 发送信号 0 来检查进程是否存在
	err := syscall.Kill(pid, 0)
	return err == nil
}

// sendStopSignal 发送停止信号 (Unix)
func sendStopSignal(pid int) error {
	return syscall.Kill(pid, syscall.SIGTERM)
}

// sendForceStopSignal 发送强制停止信号 (Unix)
func sendForceStopSignal(pid int) error {
	return syscall.Kill(pid, syscall.SIGKILL)
}
