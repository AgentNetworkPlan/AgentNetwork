//go:build windows

package daemon

import (
	"fmt"
	"os/exec"
	"syscall"
)

// setSysProcAttr 设置 Windows 平台特定的进程属性
func setSysProcAttr(cmd *exec.Cmd) {
	cmd.SysProcAttr = &syscall.SysProcAttr{
		CreationFlags: syscall.CREATE_NEW_PROCESS_GROUP,
	}
}

// isProcessRunning 检查进程是否运行 (Windows)
func isProcessRunning(pid int) bool {
	const PROCESS_QUERY_LIMITED_INFORMATION = 0x1000
	handle, err := syscall.OpenProcess(PROCESS_QUERY_LIMITED_INFORMATION, false, uint32(pid))
	if err != nil {
		return false
	}
	syscall.CloseHandle(handle)
	return true
}

// sendStopSignal 发送停止信号 (Windows)
func sendStopSignal(pid int) error {
	// Windows 下使用 taskkill
	cmd := exec.Command("taskkill", "/PID", fmt.Sprintf("%d", pid))
	return cmd.Run()
}

// sendForceStopSignal 发送强制停止信号 (Windows)
func sendForceStopSignal(pid int) error {
	cmd := exec.Command("taskkill", "/F", "/PID", fmt.Sprintf("%d", pid))
	return cmd.Run()
}
