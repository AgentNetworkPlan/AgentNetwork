package logging

import (
	"bytes"
	"os"
	"path/filepath"
	"testing"
	"time"
)

// 测试工具函数
func tempDir(t *testing.T) string {
	dir := filepath.Join(os.TempDir(), "logging_test_"+time.Now().Format("20060102150405"))
	t.Cleanup(func() {
		os.RemoveAll(dir)
	})
	return dir
}

func TestNewLogger(t *testing.T) {
	t.Run("nil config", func(t *testing.T) {
		_, err := NewLogger(nil)
		if err != ErrNilConfig {
			t.Errorf("expected ErrNilConfig, got %v", err)
		}
	})
	
	t.Run("empty node ID", func(t *testing.T) {
		config := &LogConfig{}
		_, err := NewLogger(config)
		if err != ErrEmptyNodeID {
			t.Errorf("expected ErrEmptyNodeID, got %v", err)
		}
	})
	
	t.Run("valid config", func(t *testing.T) {
		config := DefaultLogConfig("node1")
		config.DataDir = tempDir(t)
		
		l, err := NewLogger(config)
		if err != nil {
			t.Fatalf("failed to create logger: %v", err)
		}
		if l == nil {
			t.Fatal("logger is nil")
		}
		l.Stop()
	})
}

func TestDefaultLogConfig(t *testing.T) {
	config := DefaultLogConfig("node1")
	
	if config.NodeID != "node1" {
		t.Errorf("expected NodeID 'node1', got %s", config.NodeID)
	}
	if config.MaxFileSize != 10*1024*1024 {
		t.Errorf("expected MaxFileSize 10MB, got %d", config.MaxFileSize)
	}
	if config.MaxFileDays != 30 {
		t.Errorf("expected MaxFileDays 30, got %d", config.MaxFileDays)
	}
	if config.MinLevel != LevelInfo {
		t.Errorf("expected MinLevel Info, got %d", config.MinLevel)
	}
}

func TestLog(t *testing.T) {
	config := DefaultLogConfig("node1")
	config.DataDir = tempDir(t)
	config.MinLevel = LevelDebug
	
	l, _ := NewLogger(config)
	defer l.Stop()
	
	t.Run("empty event type", func(t *testing.T) {
		_, err := l.Log("", LevelInfo, nil)
		if err != ErrEmptyEventType {
			t.Errorf("expected ErrEmptyEventType, got %v", err)
		}
	})
	
	t.Run("below min level", func(t *testing.T) {
		l.SetMinLevel(LevelError)
		entry, err := l.Log(EventDebug, LevelDebug, nil)
		if err != nil {
			t.Errorf("unexpected error: %v", err)
		}
		if entry != nil {
			t.Error("expected nil entry for below min level")
		}
		l.SetMinLevel(LevelDebug)
	})
	
	t.Run("valid log", func(t *testing.T) {
		details := map[string]interface{}{
			"key": "value",
			"num": 123,
		}
		
		entry, err := l.Log(EventTaskCreate, LevelInfo, details)
		if err != nil {
			t.Fatalf("failed to log: %v", err)
		}
		
		if entry.LogID == "" {
			t.Error("expected non-empty LogID")
		}
		if entry.NodeID != "node1" {
			t.Errorf("expected NodeID 'node1', got %s", entry.NodeID)
		}
		if entry.EventType != EventTaskCreate {
			t.Errorf("expected EventType TaskCreate, got %s", entry.EventType)
		}
		if entry.Level != LevelInfo {
			t.Errorf("expected Level Info, got %d", entry.Level)
		}
		if entry.Details["key"] != "value" {
			t.Errorf("expected key='value', got %v", entry.Details["key"])
		}
	})
}

func TestConvenienceMethods(t *testing.T) {
	config := DefaultLogConfig("node1")
	config.DataDir = tempDir(t)
	config.MinLevel = LevelDebug
	
	l, _ := NewLogger(config)
	defer l.Stop()
	
	t.Run("Debug", func(t *testing.T) {
		entry, err := l.Debug(EventDebug, map[string]interface{}{"msg": "debug"})
		if err != nil {
			t.Fatalf("Debug failed: %v", err)
		}
		if entry.Level != LevelDebug {
			t.Errorf("expected LevelDebug, got %d", entry.Level)
		}
	})
	
	t.Run("Info", func(t *testing.T) {
		entry, err := l.Info(EventTaskCreate, map[string]interface{}{"task": "t1"})
		if err != nil {
			t.Fatalf("Info failed: %v", err)
		}
		if entry.Level != LevelInfo {
			t.Errorf("expected LevelInfo, got %d", entry.Level)
		}
	})
	
	t.Run("Warn", func(t *testing.T) {
		entry, err := l.Warn(EventSystemWarn, map[string]interface{}{"msg": "warning"})
		if err != nil {
			t.Fatalf("Warn failed: %v", err)
		}
		if entry.Level != LevelWarn {
			t.Errorf("expected LevelWarn, got %d", entry.Level)
		}
	})
	
	t.Run("Error", func(t *testing.T) {
		entry, err := l.Error(EventSystemError, map[string]interface{}{"error": "test"})
		if err != nil {
			t.Fatalf("Error failed: %v", err)
		}
		if entry.Level != LevelError {
			t.Errorf("expected LevelError, got %d", entry.Level)
		}
	})
}

func TestSpecializedMethods(t *testing.T) {
	config := DefaultLogConfig("node1")
	config.DataDir = tempDir(t)
	config.MinLevel = LevelDebug
	
	l, _ := NewLogger(config)
	defer l.Stop()
	
	t.Run("LogNodeEvent", func(t *testing.T) {
		entry, err := l.LogNodeEvent(EventNodeJoin, "target1", map[string]interface{}{"extra": "data"})
		if err != nil {
			t.Fatalf("LogNodeEvent failed: %v", err)
		}
		if entry.Details["target_node"] != "target1" {
			t.Errorf("expected target_node='target1', got %v", entry.Details["target_node"])
		}
		if entry.Details["extra"] != "data" {
			t.Errorf("expected extra='data', got %v", entry.Details["extra"])
		}
	})
	
	t.Run("LogTaskEvent", func(t *testing.T) {
		entry, err := l.LogTaskEvent(EventTaskComplete, "task123", nil)
		if err != nil {
			t.Fatalf("LogTaskEvent failed: %v", err)
		}
		if entry.Details["task_id"] != "task123" {
			t.Errorf("expected task_id='task123', got %v", entry.Details["task_id"])
		}
	})
	
	t.Run("LogReputationEvent", func(t *testing.T) {
		entry, err := l.LogReputationEvent(EventReputationIncrease, "nodeA", 5.0, nil)
		if err != nil {
			t.Fatalf("LogReputationEvent failed: %v", err)
		}
		if entry.Details["target_node"] != "nodeA" {
			t.Errorf("expected target_node='nodeA', got %v", entry.Details["target_node"])
		}
		if entry.Details["delta_reputation"] != 5.0 {
			t.Errorf("expected delta_reputation=5.0, got %v", entry.Details["delta_reputation"])
		}
	})
	
	t.Run("LogAccuseEvent", func(t *testing.T) {
		entry, err := l.LogAccuseEvent(EventAccuseCreate, "accuserA", "accusedB", nil)
		if err != nil {
			t.Fatalf("LogAccuseEvent failed: %v", err)
		}
		if entry.Details["accuser"] != "accuserA" {
			t.Errorf("expected accuser='accuserA', got %v", entry.Details["accuser"])
		}
		if entry.Details["accused"] != "accusedB" {
			t.Errorf("expected accused='accusedB', got %v", entry.Details["accused"])
		}
	})
	
	t.Run("LogMessageEvent", func(t *testing.T) {
		entry, err := l.LogMessageEvent(EventMessageSend, "msg1", "from1", "to1", nil)
		if err != nil {
			t.Fatalf("LogMessageEvent failed: %v", err)
		}
		if entry.Details["message_id"] != "msg1" {
			t.Errorf("expected message_id='msg1', got %v", entry.Details["message_id"])
		}
	})
	
	t.Run("LogSystemError", func(t *testing.T) {
		testErr := ErrLogNotFound
		entry, err := l.LogSystemError(testErr, "test context")
		if err != nil {
			t.Fatalf("LogSystemError failed: %v", err)
		}
		if entry.EventType != EventSystemError {
			t.Errorf("expected EventSystemError, got %s", entry.EventType)
		}
		if entry.Details["error"] != testErr.Error() {
			t.Errorf("expected error message, got %v", entry.Details["error"])
		}
	})
}

func TestGetEntry(t *testing.T) {
	config := DefaultLogConfig("node1")
	config.DataDir = tempDir(t)
	
	l, _ := NewLogger(config)
	defer l.Stop()
	
	entry, _ := l.Info(EventTaskCreate, nil)
	
	t.Run("found", func(t *testing.T) {
		found, err := l.GetEntry(entry.LogID)
		if err != nil {
			t.Fatalf("GetEntry failed: %v", err)
		}
		if found.LogID != entry.LogID {
			t.Errorf("expected LogID %s, got %s", entry.LogID, found.LogID)
		}
	})
	
	t.Run("not found", func(t *testing.T) {
		_, err := l.GetEntry("notexist")
		if err != ErrLogNotFound {
			t.Errorf("expected ErrLogNotFound, got %v", err)
		}
	})
}

func TestQuery(t *testing.T) {
	config := DefaultLogConfig("node1")
	config.DataDir = tempDir(t)
	config.MinLevel = LevelDebug
	
	l, _ := NewLogger(config)
	defer l.Stop()
	
	// 添加多个日志
	l.Debug(EventDebug, nil)
	l.Info(EventTaskCreate, nil)
	l.Info(EventTaskComplete, nil)
	l.Warn(EventSystemWarn, nil)
	l.Error(EventSystemError, nil)
	
	t.Run("all", func(t *testing.T) {
		result := l.Query(&QueryFilter{})
		if len(result) != 5 {
			t.Errorf("expected 5 entries, got %d", len(result))
		}
	})
	
	t.Run("by event type", func(t *testing.T) {
		result := l.Query(&QueryFilter{EventType: EventTaskCreate})
		if len(result) != 1 {
			t.Errorf("expected 1 entry, got %d", len(result))
		}
	})
	
	t.Run("by level", func(t *testing.T) {
		result := l.Query(&QueryFilter{Level: LevelWarn})
		if len(result) != 2 { // Warn + Error
			t.Errorf("expected 2 entries, got %d", len(result))
		}
	})
	
	t.Run("with limit", func(t *testing.T) {
		result := l.Query(&QueryFilter{Limit: 2})
		if len(result) != 2 {
			t.Errorf("expected 2 entries, got %d", len(result))
		}
	})
	
	t.Run("by time range", func(t *testing.T) {
		future := time.Now().Add(time.Hour)
		result := l.Query(&QueryFilter{StartTime: future})
		if len(result) != 0 {
			t.Errorf("expected 0 entries, got %d", len(result))
		}
	})
}

func TestGetRecentLogs(t *testing.T) {
	config := DefaultLogConfig("node1")
	config.DataDir = tempDir(t)
	
	l, _ := NewLogger(config)
	defer l.Stop()
	
	// 添加日志
	for i := 0; i < 5; i++ {
		l.Info(EventTaskCreate, map[string]interface{}{"index": i})
		time.Sleep(time.Millisecond)
	}
	
	result := l.GetRecentLogs(3)
	if len(result) != 3 {
		t.Errorf("expected 3 entries, got %d", len(result))
	}
	
	// 检查顺序（最新的在前）
	if result[0].Details["index"].(int) != 4 {
		t.Errorf("expected newest entry first")
	}
}

func TestGetLogsByEventType(t *testing.T) {
	config := DefaultLogConfig("node1")
	config.DataDir = tempDir(t)
	
	l, _ := NewLogger(config)
	defer l.Stop()
	
	l.Info(EventTaskCreate, nil)
	l.Info(EventTaskCreate, nil)
	l.Info(EventTaskComplete, nil)
	
	result := l.GetLogsByEventType(EventTaskCreate)
	if len(result) != 2 {
		t.Errorf("expected 2 entries, got %d", len(result))
	}
}

func TestGetStats(t *testing.T) {
	config := DefaultLogConfig("node1")
	config.DataDir = tempDir(t)
	config.MinLevel = LevelDebug
	
	l, _ := NewLogger(config)
	defer l.Stop()
	
	l.Debug(EventDebug, nil)
	l.Info(EventTaskCreate, nil)
	l.Error(EventSystemError, nil)
	
	stats := l.GetStats()
	
	if stats.TotalEntries != 3 {
		t.Errorf("expected 3 entries, got %d", stats.TotalEntries)
	}
	if stats.EntriesByType[EventDebug] != 1 {
		t.Errorf("expected 1 debug entry, got %d", stats.EntriesByType[EventDebug])
	}
	if stats.EntriesByLevel[LevelError] != 1 {
		t.Errorf("expected 1 error entry, got %d", stats.EntriesByLevel[LevelError])
	}
}

func TestExport(t *testing.T) {
	config := DefaultLogConfig("node1")
	config.DataDir = tempDir(t)
	
	l, _ := NewLogger(config)
	defer l.Stop()
	
	l.Info(EventTaskCreate, map[string]interface{}{"task": "t1"})
	l.Info(EventTaskComplete, map[string]interface{}{"task": "t1"})
	
	t.Run("json", func(t *testing.T) {
		var buf bytes.Buffer
		err := l.Export(&buf, "json", &QueryFilter{})
		if err != nil {
			t.Fatalf("Export JSON failed: %v", err)
		}
		if buf.Len() == 0 {
			t.Error("expected non-empty output")
		}
	})
	
	t.Run("jsonl", func(t *testing.T) {
		var buf bytes.Buffer
		err := l.Export(&buf, "jsonl", &QueryFilter{})
		if err != nil {
			t.Fatalf("Export JSONL failed: %v", err)
		}
		if buf.Len() == 0 {
			t.Error("expected non-empty output")
		}
	})
	
	t.Run("csv", func(t *testing.T) {
		var buf bytes.Buffer
		err := l.Export(&buf, "csv", &QueryFilter{})
		if err != nil {
			t.Fatalf("Export CSV failed: %v", err)
		}
		if buf.Len() == 0 {
			t.Error("expected non-empty output")
		}
	})
	
	t.Run("unsupported", func(t *testing.T) {
		var buf bytes.Buffer
		err := l.Export(&buf, "xml", &QueryFilter{})
		if err == nil {
			t.Error("expected error for unsupported format")
		}
	})
}

func TestSignature(t *testing.T) {
	config := DefaultLogConfig("node1")
	config.DataDir = tempDir(t)
	
	signed := false
	config.SignFunc = func(data []byte) (string, error) {
		signed = true
		return "test_signature", nil
	}
	
	l, _ := NewLogger(config)
	defer l.Stop()
	
	entry, _ := l.Info(EventTaskCreate, nil)
	
	if !signed {
		t.Error("expected SignFunc to be called")
	}
	if entry.Signature != "test_signature" {
		t.Errorf("expected signature 'test_signature', got %s", entry.Signature)
	}
}

func TestVerifyEntry(t *testing.T) {
	config := DefaultLogConfig("node1")
	config.DataDir = tempDir(t)
	
	verifyResult := true
	config.VerifyFunc = func(publicKey string, data []byte, signature string) bool {
		return verifyResult
	}
	
	l, _ := NewLogger(config)
	defer l.Stop()
	
	entry := &LogEntry{
		LogID:     "test",
		NodeID:    "node1",
		Timestamp: time.Now(),
		EventType: EventDebug,
		Signature: "sig",
	}
	
	t.Run("valid", func(t *testing.T) {
		verifyResult = true
		if !l.VerifyEntry(entry) {
			t.Error("expected verification to pass")
		}
	})
	
	t.Run("invalid", func(t *testing.T) {
		verifyResult = false
		if l.VerifyEntry(entry) {
			t.Error("expected verification to fail")
		}
	})
}

func TestCallback(t *testing.T) {
	config := DefaultLogConfig("node1")
	config.DataDir = tempDir(t)
	
	l, _ := NewLogger(config)
	defer l.Stop()
	
	var callbackEntry *LogEntry
	l.OnLog = func(entry *LogEntry) {
		callbackEntry = entry
	}
	
	entry, _ := l.Info(EventTaskCreate, nil)
	
	if callbackEntry == nil {
		t.Fatal("callback not called")
	}
	if callbackEntry.LogID != entry.LogID {
		t.Errorf("expected LogID %s, got %s", entry.LogID, callbackEntry.LogID)
	}
}

func TestStartStop(t *testing.T) {
	config := DefaultLogConfig("node1")
	config.DataDir = tempDir(t)
	config.RotateInterval = 100 * time.Millisecond
	
	l, _ := NewLogger(config)
	
	l.Start()
	time.Sleep(50 * time.Millisecond)
	
	// 再次启动不应有问题
	l.Start()
	
	l.Stop()
	time.Sleep(50 * time.Millisecond)
	
	// 再次停止不应有问题
	l.Stop()
}

func TestClear(t *testing.T) {
	config := DefaultLogConfig("node1")
	config.DataDir = tempDir(t)
	
	l, _ := NewLogger(config)
	defer l.Stop()
	
	l.Info(EventTaskCreate, nil)
	l.Info(EventTaskComplete, nil)
	
	if l.GetEntryCount() != 2 {
		t.Errorf("expected 2 entries, got %d", l.GetEntryCount())
	}
	
	l.Clear()
	
	if l.GetEntryCount() != 0 {
		t.Errorf("expected 0 entries after clear, got %d", l.GetEntryCount())
	}
}

func TestSetters(t *testing.T) {
	config := DefaultLogConfig("node1")
	config.DataDir = tempDir(t)
	
	l, _ := NewLogger(config)
	defer l.Stop()
	
	l.SetMinLevel(LevelError)
	if l.config.MinLevel != LevelError {
		t.Errorf("expected MinLevel Error, got %d", l.config.MinLevel)
	}
	
	l.SetEnableConsole(true)
	if !l.config.EnableConsole {
		t.Error("expected EnableConsole true")
	}
}

func TestLogLevelString(t *testing.T) {
	tests := []struct {
		level    LogLevel
		expected string
	}{
		{LevelDebug, "DEBUG"},
		{LevelInfo, "INFO"},
		{LevelWarn, "WARN"},
		{LevelError, "ERROR"},
		{LogLevel(99), "UNKNOWN"},
	}
	
	for _, tt := range tests {
		if tt.level.String() != tt.expected {
			t.Errorf("expected %s, got %s", tt.expected, tt.level.String())
		}
	}
}

func TestEventTypes(t *testing.T) {
	config := DefaultLogConfig("node1")
	config.DataDir = tempDir(t)
	
	l, _ := NewLogger(config)
	defer l.Stop()
	
	eventTypes := []EventType{
		EventNodeRegister,
		EventNodeJoin,
		EventNodeLeave,
		EventNeighborUpdate,
		EventVoteCast,
		EventSupernodeElect,
		EventSupernodeRemove,
		EventTaskCreate,
		EventTaskAccept,
		EventTaskSubmit,
		EventTaskComplete,
		EventTaskFail,
		EventTaskTimeout,
		EventReputationIncrease,
		EventReputationDecrease,
		EventReputationPropagate,
		EventReputationDecay,
		EventToleranceExceed,
		EventAccuseCreate,
		EventAccuseReceive,
		EventAccuseVerify,
		EventAccuseReject,
		EventAccusePropagate,
		EventMessageSend,
		EventMessageReceive,
		EventMessageRelay,
		EventBulletinPost,
		EventSystemStart,
		EventSystemStop,
		EventSystemError,
		EventSystemWarn,
		EventDebug,
	}
	
	for _, et := range eventTypes {
		entry, err := l.Info(et, nil)
		if err != nil {
			t.Errorf("failed to log event type %s: %v", et, err)
		}
		if entry.EventType != et {
			t.Errorf("expected %s, got %s", et, entry.EventType)
		}
	}
}

func TestLoadFromFileAndPersistence(t *testing.T) {
	dir := tempDir(t)
	
	// 创建并写入日志
	config1 := DefaultLogConfig("node1")
	config1.DataDir = dir
	
	l1, _ := NewLogger(config1)
	l1.Info(EventTaskCreate, map[string]interface{}{"task": "t1"})
	l1.Info(EventTaskComplete, map[string]interface{}{"task": "t1"})
	l1.Stop()
	
	// 查找日志文件
	entries, _ := os.ReadDir(dir)
	var logFile string
	for _, e := range entries {
		if !e.IsDir() && len(e.Name()) > 4 {
			logFile = filepath.Join(dir, e.Name())
			break
		}
	}
	
	if logFile == "" {
		t.Skip("no log file found")
	}
	
	// 加载日志
	config2 := DefaultLogConfig("node2")
	config2.DataDir = tempDir(t)
	
	l2, _ := NewLogger(config2)
	defer l2.Stop()
	
	err := l2.LoadFromFile(logFile)
	if err != nil {
		t.Fatalf("failed to load log file: %v", err)
	}
	
	if l2.GetEntryCount() < 2 {
		t.Errorf("expected at least 2 entries, got %d", l2.GetEntryCount())
	}
}

func TestConsoleOutput(t *testing.T) {
	config := DefaultLogConfig("node1")
	config.DataDir = tempDir(t)
	config.EnableConsole = true
	
	l, _ := NewLogger(config)
	defer l.Stop()
	
	// 这只是测试不会 panic
	l.Info(EventTaskCreate, map[string]interface{}{"test": "console"})
}

func TestQueryByNodeID(t *testing.T) {
	config := DefaultLogConfig("node1")
	config.DataDir = tempDir(t)
	
	l, _ := NewLogger(config)
	defer l.Stop()
	
	l.Info(EventTaskCreate, nil)
	
	// 查询本节点
	result := l.Query(&QueryFilter{NodeID: "node1"})
	if len(result) != 1 {
		t.Errorf("expected 1 entry, got %d", len(result))
	}
	
	// 查询其他节点
	result = l.Query(&QueryFilter{NodeID: "node2"})
	if len(result) != 0 {
		t.Errorf("expected 0 entries, got %d", len(result))
	}
}

func TestQueryByEndTime(t *testing.T) {
	config := DefaultLogConfig("node1")
	config.DataDir = tempDir(t)
	
	l, _ := NewLogger(config)
	defer l.Stop()
	
	l.Info(EventTaskCreate, nil)
	
	// 过去的结束时间
	past := time.Now().Add(-time.Hour)
	result := l.Query(&QueryFilter{EndTime: past})
	if len(result) != 0 {
		t.Errorf("expected 0 entries, got %d", len(result))
	}
	
	// 未来的结束时间
	future := time.Now().Add(time.Hour)
	result = l.Query(&QueryFilter{EndTime: future})
	if len(result) != 1 {
		t.Errorf("expected 1 entry, got %d", len(result))
	}
}
