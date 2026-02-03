package daemon

import (
	"os"
	"path/filepath"
	"strconv"
	"testing"
	"time"
)

func TestDefaultConfig(t *testing.T) {
	cfg := DefaultConfig()
	
	if cfg.DataDir != "./data" {
		t.Errorf("DataDir = %v, want ./data", cfg.DataDir)
	}
	if cfg.MaxLogSizeMB != 100 {
		t.Errorf("MaxLogSizeMB = %v, want 100", cfg.MaxLogSizeMB)
	}
	if cfg.MaxLogFiles != 5 {
		t.Errorf("MaxLogFiles = %v, want 5", cfg.MaxLogFiles)
	}
}

func TestNew(t *testing.T) {
	// nil config
	d := New(nil)
	if d == nil {
		t.Fatal("New(nil) should not return nil")
	}
	if d.config.DataDir != "./data" {
		t.Error("Should use default config")
	}

	// custom config
	cfg := &Config{
		DataDir:  "/tmp/test",
		LogFile:  "/tmp/test.log",
		PidFile:  "/tmp/test.pid",
		NodeName: "test-node",
	}
	d = New(cfg)
	if d.config.DataDir != "/tmp/test" {
		t.Errorf("DataDir = %v, want /tmp/test", d.config.DataDir)
	}
}

func TestPaths(t *testing.T) {
	cfg := &Config{
		DataDir: "/tmp/testdata",
	}
	d := New(cfg)

	// 默认路径
	if d.PidFile() != filepath.Join("/tmp/testdata", "node.pid") {
		t.Errorf("PidFile() = %v", d.PidFile())
	}
	if d.LogFile() != filepath.Join("/tmp/testdata", "node.log") {
		t.Errorf("LogFile() = %v", d.LogFile())
	}
	if d.StatusFile() != filepath.Join("/tmp/testdata", "node.status") {
		t.Errorf("StatusFile() = %v", d.StatusFile())
	}

	// 自定义路径
	cfg.PidFile = "/custom/test.pid"
	cfg.LogFile = "/custom/test.log"
	d = New(cfg)

	if d.PidFile() != "/custom/test.pid" {
		t.Errorf("Custom PidFile() = %v", d.PidFile())
	}
	if d.LogFile() != "/custom/test.log" {
		t.Errorf("Custom LogFile() = %v", d.LogFile())
	}
}

func TestIsDaemonProcess(t *testing.T) {
	// 未设置环境变量
	os.Unsetenv(DaemonEnvKey)
	if IsDaemonProcess() {
		t.Error("Should return false when env not set")
	}

	// 设置环境变量
	os.Setenv(DaemonEnvKey, "1")
	defer os.Unsetenv(DaemonEnvKey)
	
	if !IsDaemonProcess() {
		t.Error("Should return true when env is set to 1")
	}
}

func TestIsRunning(t *testing.T) {
	tmpDir := t.TempDir()
	cfg := &Config{DataDir: tmpDir}
	d := New(cfg)

	// PID文件不存在
	pid, running := d.IsRunning()
	if running {
		t.Error("Should not be running when PID file doesn't exist")
	}

	// 写入当前进程PID
	currentPid := os.Getpid()
	os.WriteFile(d.PidFile(), []byte(strconv.Itoa(currentPid)), 0644)

	pid, running = d.IsRunning()
	if !running {
		t.Error("Should be running when process exists")
	}
	if pid != currentPid {
		t.Errorf("PID = %d, want %d", pid, currentPid)
	}

	// 写入不存在的PID
	os.WriteFile(d.PidFile(), []byte("999999999"), 0644)
	_, running = d.IsRunning()
	if running {
		t.Error("Should not be running for non-existent PID")
	}

	// 无效的PID内容
	os.WriteFile(d.PidFile(), []byte("invalid"), 0644)
	_, running = d.IsRunning()
	if running {
		t.Error("Should not be running for invalid PID")
	}
}

func TestStatus(t *testing.T) {
	tmpDir := t.TempDir()
	cfg := &Config{DataDir: tmpDir}
	d := New(cfg)

	// 未运行状态
	status := d.Status()
	if status.Running {
		t.Error("Should not be running")
	}
	if status.DataDir != tmpDir {
		t.Errorf("DataDir = %v, want %v", status.DataDir, tmpDir)
	}

	// 写入状态文件
	testStatus := &NodeStatus{
		Running:   true,
		PID:       12345,
		StartTime: time.Now().Add(-1 * time.Hour),
		NodeID:    "test-node-id",
		Version:   "1.0.0",
	}
	d.WriteStatus(testStatus)

	// 再次获取状态
	status = d.Status()
	if status.NodeID != "test-node-id" {
		t.Errorf("NodeID = %v, want test-node-id", status.NodeID)
	}
	if status.Version != "1.0.0" {
		t.Errorf("Version = %v, want 1.0.0", status.Version)
	}
}

func TestWriteStatus(t *testing.T) {
	tmpDir := t.TempDir()
	cfg := &Config{DataDir: tmpDir}
	d := New(cfg)

	status := &NodeStatus{
		Running:     true,
		PID:         os.Getpid(),
		StartTime:   time.Now(),
		NodeID:      "node-001",
		Version:     "0.1.0",
		ListenAddrs: []string{"/ip4/0.0.0.0/tcp/9000"},
		PeerCount:   5,
	}

	err := d.WriteStatus(status)
	if err != nil {
		t.Fatalf("WriteStatus error: %v", err)
	}

	// 检查文件是否存在
	if _, err := os.Stat(d.StatusFile()); os.IsNotExist(err) {
		t.Error("Status file should exist")
	}
}

func TestCleanup(t *testing.T) {
	tmpDir := t.TempDir()
	cfg := &Config{DataDir: tmpDir}
	d := New(cfg)

	// 创建文件
	os.WriteFile(d.PidFile(), []byte("12345"), 0644)
	os.WriteFile(d.StatusFile(), []byte("{}"), 0644)

	// 清理
	d.Cleanup()

	// 检查文件是否被删除
	if _, err := os.Stat(d.PidFile()); !os.IsNotExist(err) {
		t.Error("PID file should be removed")
	}
	if _, err := os.Stat(d.StatusFile()); !os.IsNotExist(err) {
		t.Error("Status file should be removed")
	}
}

func TestEnsureDir(t *testing.T) {
	tmpDir := t.TempDir()
	testPath := filepath.Join(tmpDir, "a", "b", "c", "test.txt")

	err := ensureDir(testPath)
	if err != nil {
		t.Fatalf("ensureDir error: %v", err)
	}

	// 检查目录是否存在
	dir := filepath.Dir(testPath)
	if _, err := os.Stat(dir); os.IsNotExist(err) {
		t.Error("Directory should exist")
	}
}

func TestTailLines(t *testing.T) {
	tmpDir := t.TempDir()
	logFile := filepath.Join(tmpDir, "test.log")

	// 写入测试日志
	content := "line1\nline2\nline3\nline4\nline5\n"
	os.WriteFile(logFile, []byte(content), 0644)

	cfg := &Config{DataDir: tmpDir, LogFile: logFile}
	d := New(cfg)

	// 测试读取最后3行（仅测试不报错）
	err := d.tailLines(logFile, 3)
	if err != nil {
		t.Errorf("tailLines error: %v", err)
	}
}

func TestRotateLogs(t *testing.T) {
	tmpDir := t.TempDir()
	logFile := filepath.Join(tmpDir, "test.log")

	cfg := &Config{
		DataDir:      tmpDir,
		LogFile:      logFile,
		MaxLogSizeMB: 0, // 0MB 触发轮转
		MaxLogFiles:  3,
	}
	d := New(cfg)

	// 创建日志文件（需要大于0字节才会触发轮转）
	// MaxLogSizeMB=0 意味着任何大小都会触发
	// 但实际上 0 * 1024 * 1024 = 0，所以需要特殊处理
	// 让我们用小一点的值

	cfg.MaxLogSizeMB = 1 // 1字节就触发（测试用）
	d = New(cfg)

	// 写入超过1MB的数据测试轮转
	largeContent := make([]byte, 2*1024*1024) // 2MB
	os.WriteFile(logFile, largeContent, 0644)

	err := d.RotateLogs()
	if err != nil {
		t.Errorf("RotateLogs error: %v", err)
	}

	// 检查轮转后的文件
	if _, err := os.Stat(logFile + ".1"); os.IsNotExist(err) {
		t.Error("Rotated log file should exist")
	}
}

func TestLogsFileNotExist(t *testing.T) {
	tmpDir := t.TempDir()
	cfg := &Config{DataDir: tmpDir}
	d := New(cfg)

	err := d.Logs(10, false)
	if err == nil {
		t.Error("Should return error when log file doesn't exist")
	}
}

func TestNodeStatus(t *testing.T) {
	status := &NodeStatus{
		Running:     true,
		PID:         12345,
		StartTime:   time.Now(),
		NodeID:      "test-node",
		Version:     "1.0.0",
		ListenAddrs: []string{"/ip4/127.0.0.1/tcp/9000"},
		PeerCount:   10,
		DataDir:     "/data",
		LogFile:     "/data/node.log",
		PidFile:     "/data/node.pid",
	}

	if !status.Running {
		t.Error("Running should be true")
	}
	if status.PID != 12345 {
		t.Errorf("PID = %d, want 12345", status.PID)
	}
	if status.NodeID != "test-node" {
		t.Errorf("NodeID = %s, want test-node", status.NodeID)
	}
}

func TestStartAlreadyRunning(t *testing.T) {
	tmpDir := t.TempDir()
	cfg := &Config{DataDir: tmpDir}
	d := New(cfg)

	// 写入当前进程的PID（模拟已运行）
	os.WriteFile(d.PidFile(), []byte(strconv.Itoa(os.Getpid())), 0644)

	_, err := d.Start()
	if err == nil {
		t.Error("Should return error when already running")
	}
}

func TestStopNotRunning(t *testing.T) {
	tmpDir := t.TempDir()
	cfg := &Config{DataDir: tmpDir}
	d := New(cfg)

	err := d.Stop()
	if err == nil {
		t.Error("Should return error when not running")
	}
}
