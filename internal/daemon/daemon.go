// Package daemon 实现节点守护进程管理
// 支持 start/stop/restart/status 等命令
package daemon

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"time"
)

// 守护进程环境变量标记
const DaemonEnvKey = "AGENTNETWORK_DAEMON"

// Config 守护进程配置
type Config struct {
	// 数据目录（存放PID、日志等）
	DataDir string
	// 日志文件路径（空则使用默认）
	LogFile string
	// PID文件路径（空则使用默认）
	PidFile string
	// 节点名称（用于日志标识）
	NodeName string
	// 最大日志文件大小(MB)，超过则轮转
	MaxLogSizeMB int
	// 保留日志文件数量
	MaxLogFiles int
}

// DefaultConfig 返回默认配置
func DefaultConfig() *Config {
	return &Config{
		DataDir:      "./data",
		MaxLogSizeMB: 100,
		MaxLogFiles:  5,
	}
}

// Daemon 守护进程管理器
type Daemon struct {
	config *Config
}

// New 创建守护进程管理器
func New(cfg *Config) *Daemon {
	if cfg == nil {
		cfg = DefaultConfig()
	}
	return &Daemon{config: cfg}
}

// PidFile 获取PID文件路径
func (d *Daemon) PidFile() string {
	if d.config.PidFile != "" {
		return d.config.PidFile
	}
	return filepath.Join(d.config.DataDir, "node.pid")
}

// LogFile 获取日志文件路径
func (d *Daemon) LogFile() string {
	if d.config.LogFile != "" {
		return d.config.LogFile
	}
	return filepath.Join(d.config.DataDir, "node.log")
}

// StatusFile 获取状态文件路径
func (d *Daemon) StatusFile() string {
	return filepath.Join(d.config.DataDir, "node.status")
}

// IsDaemonProcess 检查当前进程是否是守护进程
func IsDaemonProcess() bool {
	return os.Getenv(DaemonEnvKey) == "1"
}

// ensureDir 确保目录存在
func ensureDir(path string) error {
	dir := filepath.Dir(path)
	return os.MkdirAll(dir, 0755)
}

// Start 启动守护进程
// 如果是父进程，会fork子进程并退出
// 如果是子进程（守护进程），返回true继续执行
func (d *Daemon) Start() (bool, error) {
	// 检查是否已经在运行
	if pid, running := d.IsRunning(); running {
		return false, fmt.Errorf("节点已在运行 (PID: %d)", pid)
	}

	// 如果是守护进程子进程，直接返回
	if IsDaemonProcess() {
		// 写入PID文件
		if err := d.writePid(); err != nil {
			return false, fmt.Errorf("写入PID文件失败: %w", err)
		}
		return true, nil
	}

	// 父进程：fork子进程
	if err := d.fork(); err != nil {
		return false, fmt.Errorf("启动守护进程失败: %w", err)
	}

	// 父进程在fork后退出
	return false, nil
}

// fork 创建子进程
func (d *Daemon) fork() error {
	// 确保目录存在
	if err := ensureDir(d.LogFile()); err != nil {
		return fmt.Errorf("创建日志目录失败: %w", err)
	}

	// 打开日志文件
	logFile, err := os.OpenFile(d.LogFile(), os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0644)
	if err != nil {
		return fmt.Errorf("打开日志文件失败: %w", err)
	}

	// 获取当前可执行文件路径
	executable, err := os.Executable()
	if err != nil {
		return fmt.Errorf("获取可执行文件路径失败: %w", err)
	}

	// 构建命令行参数（保留原有参数）
	args := os.Args[1:]
	
	// 移除 start 命令（避免重复fork）
	filteredArgs := make([]string, 0, len(args))
	for _, arg := range args {
		if arg != "start" && arg != "daemon" {
			filteredArgs = append(filteredArgs, arg)
		}
	}
	// 添加 run 命令
	filteredArgs = append([]string{"run"}, filteredArgs...)

	// 创建子进程
	cmd := exec.Command(executable, filteredArgs...)
	cmd.Stdout = logFile
	cmd.Stderr = logFile
	cmd.Env = append(os.Environ(), DaemonEnvKey+"=1")

	// 设置平台特定的进程属性
	setSysProcAttr(cmd)

	if err := cmd.Start(); err != nil {
		return fmt.Errorf("启动子进程失败: %w", err)
	}

	// 写入PID
	if err := ensureDir(d.PidFile()); err != nil {
		return err
	}
	if err := os.WriteFile(d.PidFile(), []byte(strconv.Itoa(cmd.Process.Pid)), 0644); err != nil {
		return fmt.Errorf("写入PID文件失败: %w", err)
	}

	fmt.Printf("节点已在后台启动 (PID: %d)\n", cmd.Process.Pid)
	fmt.Printf("日志文件: %s\n", d.LogFile())

	return nil
}

// writePid 写入当前进程PID
func (d *Daemon) writePid() error {
	if err := ensureDir(d.PidFile()); err != nil {
		return err
	}
	return os.WriteFile(d.PidFile(), []byte(strconv.Itoa(os.Getpid())), 0644)
}

// Stop 停止守护进程
func (d *Daemon) Stop() error {
	pid, running := d.IsRunning()
	if !running {
		return errors.New("节点未在运行")
	}

	// 发送停止信号（平台特定）
	if err := sendStopSignal(pid); err != nil {
		return fmt.Errorf("发送停止信号失败: %w", err)
	}

	// 等待进程退出（最多10秒）
	for i := 0; i < 100; i++ {
		if _, running := d.IsRunning(); !running {
			break
		}
		time.Sleep(100 * time.Millisecond)
	}

	// 如果还在运行，强制终止
	if _, running := d.IsRunning(); running {
		sendForceStopSignal(pid)
	}

	// 清理PID文件
	os.Remove(d.PidFile())

	fmt.Printf("节点已停止 (PID: %d)\n", pid)
	return nil
}

// Restart 重启守护进程
func (d *Daemon) Restart() error {
	// 先停止（忽略错误，可能本来就没运行）
	d.Stop()
	
	// 等待一下确保端口释放
	time.Sleep(time.Second)

	// 重新启动
	_, err := d.Start()
	return err
}

// IsRunning 检查守护进程是否在运行
func (d *Daemon) IsRunning() (int, bool) {
	pidData, err := os.ReadFile(d.PidFile())
	if err != nil {
		return 0, false
	}

	pid, err := strconv.Atoi(strings.TrimSpace(string(pidData)))
	if err != nil {
		return 0, false
	}

	// 使用平台特定的方式检查进程是否存在
	if !isProcessRunning(pid) {
		return pid, false
	}

	return pid, true
}

// NodeStatus 节点状态信息
type NodeStatus struct {
	Running     bool      `json:"running"`
	PID         int       `json:"pid,omitempty"`
	StartTime   time.Time `json:"start_time,omitempty"`
	Uptime      string    `json:"uptime,omitempty"`
	NodeID      string    `json:"node_id,omitempty"`
	Version     string    `json:"version,omitempty"`
	ListenAddrs []string  `json:"listen_addrs,omitempty"`
	PeerCount   int       `json:"peer_count,omitempty"`
	DataDir     string    `json:"data_dir"`
	LogFile     string    `json:"log_file"`
	PidFile     string    `json:"pid_file"`
}

// Status 获取守护进程状态
func (d *Daemon) Status() *NodeStatus {
	status := &NodeStatus{
		DataDir: d.config.DataDir,
		LogFile: d.LogFile(),
		PidFile: d.PidFile(),
	}

	pid, running := d.IsRunning()
	status.Running = running
	status.PID = pid

	// 读取状态文件（如果存在）
	if statusData, err := os.ReadFile(d.StatusFile()); err == nil {
		var fileStatus NodeStatus
		if json.Unmarshal(statusData, &fileStatus) == nil {
			status.StartTime = fileStatus.StartTime
			status.NodeID = fileStatus.NodeID
			status.Version = fileStatus.Version
			status.ListenAddrs = fileStatus.ListenAddrs
			status.PeerCount = fileStatus.PeerCount
			
			if !status.StartTime.IsZero() {
				status.Uptime = time.Since(status.StartTime).Round(time.Second).String()
			}
		}
	}

	return status
}

// WriteStatus 写入状态信息（由运行中的节点调用）
func (d *Daemon) WriteStatus(status *NodeStatus) error {
	if err := ensureDir(d.StatusFile()); err != nil {
		return err
	}
	data, err := json.MarshalIndent(status, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(d.StatusFile(), data, 0644)
}

// Cleanup 清理（停止时调用）
func (d *Daemon) Cleanup() {
	os.Remove(d.PidFile())
	os.Remove(d.StatusFile())
}

// Logs 获取日志内容
func (d *Daemon) Logs(lines int, follow bool) error {
	logPath := d.LogFile()
	
	if _, err := os.Stat(logPath); os.IsNotExist(err) {
		return errors.New("日志文件不存在")
	}

	if follow {
		return d.tailFollow(logPath)
	}

	return d.tailLines(logPath, lines)
}

// tailLines 读取最后N行
func (d *Daemon) tailLines(path string, n int) error {
	file, err := os.Open(path)
	if err != nil {
		return err
	}
	defer file.Close()

	// 读取所有内容
	content, err := io.ReadAll(file)
	if err != nil {
		return err
	}

	lines := strings.Split(string(content), "\n")
	
	// 获取最后N行
	start := len(lines) - n
	if start < 0 {
		start = 0
	}

	for _, line := range lines[start:] {
		if line != "" {
			fmt.Println(line)
		}
	}

	return nil
}

// tailFollow 实时跟踪日志
func (d *Daemon) tailFollow(path string) error {
	file, err := os.Open(path)
	if err != nil {
		return err
	}
	defer file.Close()

	// 移动到文件末尾
	file.Seek(0, io.SeekEnd)

	fmt.Println("实时日志输出 (Ctrl+C 退出)...")
	fmt.Println(strings.Repeat("-", 60))

	buf := make([]byte, 4096)
	for {
		n, err := file.Read(buf)
		if err != nil && err != io.EOF {
			return err
		}
		if n > 0 {
			fmt.Print(string(buf[:n]))
		}
		time.Sleep(100 * time.Millisecond)
	}
}

// RotateLogs 轮转日志文件
func (d *Daemon) RotateLogs() error {
	logPath := d.LogFile()
	
	info, err := os.Stat(logPath)
	if err != nil {
		return nil // 文件不存在，无需轮转
	}

	// 检查文件大小
	maxSize := int64(d.config.MaxLogSizeMB) * 1024 * 1024
	if info.Size() < maxSize {
		return nil
	}

	// 轮转旧日志
	for i := d.config.MaxLogFiles - 1; i > 0; i-- {
		oldPath := fmt.Sprintf("%s.%d", logPath, i)
		newPath := fmt.Sprintf("%s.%d", logPath, i+1)
		os.Rename(oldPath, newPath)
	}

	// 当前日志变为 .1
	os.Rename(logPath, logPath+".1")

	// 创建新日志文件
	file, err := os.Create(logPath)
	if err != nil {
		return err
	}
	file.Close()

	return nil
}
