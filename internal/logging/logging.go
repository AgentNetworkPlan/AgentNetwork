// Package logging 实现去中心化日志系统
// 支持日志分类、签名、存储、查询和传播
package logging

import (
	"compress/gzip"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"time"
)

// 错误定义
var (
	ErrNilConfig     = errors.New("config cannot be nil")
	ErrEmptyNodeID   = errors.New("node ID cannot be empty")
	ErrLogNotFound   = errors.New("log entry not found")
	ErrInvalidLogID  = errors.New("invalid log ID")
	ErrEmptyEventType = errors.New("event type cannot be empty")
)

// EventType 事件类型
type EventType string

const (
	// 节点管理事件
	EventNodeRegister    EventType = "node_register"
	EventNodeJoin        EventType = "node_join"
	EventNodeLeave       EventType = "node_leave"
	EventNeighborUpdate  EventType = "neighbor_update"
	EventVoteCast        EventType = "vote_cast"
	EventSupernodeElect  EventType = "supernode_elect"
	EventSupernodeRemove EventType = "supernode_remove"
	
	// 任务事件
	EventTaskCreate   EventType = "task_create"
	EventTaskAccept   EventType = "task_accept"
	EventTaskSubmit   EventType = "task_submit"
	EventTaskComplete EventType = "task_complete"
	EventTaskFail     EventType = "task_fail"
	EventTaskTimeout  EventType = "task_timeout"
	
	// 声誉事件
	EventReputationIncrease  EventType = "reputation_increase"
	EventReputationDecrease  EventType = "reputation_decrease"
	EventReputationPropagate EventType = "reputation_propagate"
	EventReputationDecay     EventType = "reputation_decay"
	EventToleranceExceed     EventType = "tolerance_exceed"
	
	// 指责事件
	EventAccuseCreate    EventType = "accuse_create"
	EventAccuseReceive   EventType = "accuse_receive"
	EventAccuseVerify    EventType = "accuse_verify"
	EventAccuseReject    EventType = "accuse_reject"
	EventAccusePropagate EventType = "accuse_propagate"
	
	// 消息事件
	EventMessageSend    EventType = "message_send"
	EventMessageReceive EventType = "message_receive"
	EventMessageRelay   EventType = "message_relay"
	EventBulletinPost   EventType = "bulletin_post"
	
	// 系统事件
	EventSystemStart  EventType = "system_start"
	EventSystemStop   EventType = "system_stop"
	EventSystemError  EventType = "system_error"
	EventSystemWarn   EventType = "system_warn"
	EventDebug        EventType = "debug"
)

// LogLevel 日志级别
type LogLevel int

const (
	LevelDebug LogLevel = iota
	LevelInfo
	LevelWarn
	LevelError
)

func (l LogLevel) String() string {
	switch l {
	case LevelDebug:
		return "DEBUG"
	case LevelInfo:
		return "INFO"
	case LevelWarn:
		return "WARN"
	case LevelError:
		return "ERROR"
	default:
		return "UNKNOWN"
	}
}

// LogEntry 日志条目
type LogEntry struct {
	LogID     string                 `json:"log_id"`
	NodeID    string                 `json:"node_id"`
	Timestamp time.Time              `json:"timestamp"`
	EventType EventType              `json:"event_type"`
	Level     LogLevel               `json:"level"`
	Details   map[string]interface{} `json:"details"`
	Signature string                 `json:"signature"`
}

// LogConfig 日志配置
type LogConfig struct {
	NodeID           string
	DataDir          string
	MaxFileSize      int64         // 最大文件大小 (bytes)
	MaxFileDays      int           // 最大保留天数
	RotateInterval   time.Duration // 滚动间隔
	CompressOldLogs  bool          // 压缩旧日志
	MinLevel         LogLevel      // 最低日志级别
	EnableConsole    bool          // 是否输出到控制台
	
	// 签名函数
	SignFunc   func(data []byte) (string, error)
	VerifyFunc func(publicKey string, data []byte, signature string) bool
}

// DefaultLogConfig 返回默认配置
func DefaultLogConfig(nodeID string) *LogConfig {
	return &LogConfig{
		NodeID:          nodeID,
		DataDir:         "./data/logs",
		MaxFileSize:     10 * 1024 * 1024, // 10MB
		MaxFileDays:     30,
		RotateInterval:  24 * time.Hour,
		CompressOldLogs: true,
		MinLevel:        LevelInfo,
		EnableConsole:   false,
	}
}

// Logger 日志管理器
type Logger struct {
	mu         sync.RWMutex
	config     *LogConfig
	entries    []*LogEntry                // 内存日志缓存
	entryIndex map[string]*LogEntry       // LogID -> Entry
	file       *os.File                   // 当前日志文件
	fileSize   int64                      // 当前文件大小
	running    bool
	stopCh     chan struct{}
	
	// 回调
	OnLog func(*LogEntry)
}

// NewLogger 创建日志管理器
func NewLogger(config *LogConfig) (*Logger, error) {
	if config == nil {
		return nil, ErrNilConfig
	}
	if config.NodeID == "" {
		return nil, ErrEmptyNodeID
	}
	
	// 创建数据目录
	if config.DataDir != "" {
		if err := os.MkdirAll(config.DataDir, 0755); err != nil {
			return nil, fmt.Errorf("failed to create data directory: %w", err)
		}
	}
	
	l := &Logger{
		config:     config,
		entries:    make([]*LogEntry, 0),
		entryIndex: make(map[string]*LogEntry),
		stopCh:     make(chan struct{}),
	}
	
	// 打开或创建日志文件
	if err := l.openLogFile(); err != nil {
		return nil, err
	}
	
	return l, nil
}

// Start 启动日志管理器
func (l *Logger) Start() {
	l.mu.Lock()
	if l.running {
		l.mu.Unlock()
		return
	}
	l.running = true
	l.stopCh = make(chan struct{})
	l.mu.Unlock()
	
	go l.mainLoop()
}

// Stop 停止日志管理器
func (l *Logger) Stop() {
	l.mu.Lock()
	if !l.running {
		l.mu.Unlock()
		return
	}
	l.running = false
	close(l.stopCh)
	
	if l.file != nil {
		l.file.Close()
		l.file = nil
	}
	l.mu.Unlock()
}

// mainLoop 主循环
func (l *Logger) mainLoop() {
	rotateTicker := time.NewTicker(l.config.RotateInterval)
	cleanupTicker := time.NewTicker(24 * time.Hour)
	
	defer rotateTicker.Stop()
	defer cleanupTicker.Stop()
	
	for {
		select {
		case <-rotateTicker.C:
			l.rotateIfNeeded()
		case <-cleanupTicker.C:
			l.cleanupOldLogs()
		case <-l.stopCh:
			return
		}
	}
}

// openLogFile 打开日志文件
func (l *Logger) openLogFile() error {
	if l.config.DataDir == "" {
		return nil
	}
	
	filename := filepath.Join(l.config.DataDir, fmt.Sprintf("log_%s.jsonl", time.Now().Format("20060102")))
	
	f, err := os.OpenFile(filename, os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0644)
	if err != nil {
		return err
	}
	
	// 获取文件大小
	info, err := f.Stat()
	if err != nil {
		f.Close()
		return err
	}
	
	l.file = f
	l.fileSize = info.Size()
	
	return nil
}

// rotateIfNeeded 检查并滚动日志
func (l *Logger) rotateIfNeeded() {
	l.mu.Lock()
	defer l.mu.Unlock()
	
	if l.file == nil {
		return
	}
	
	// 检查文件大小
	if l.fileSize < l.config.MaxFileSize {
		return
	}
	
	// 关闭当前文件
	oldFile := l.file.Name()
	l.file.Close()
	l.file = nil
	
	// 压缩旧文件
	if l.config.CompressOldLogs {
		go l.compressFile(oldFile)
	}
	
	// 打开新文件
	l.openLogFile()
}

// compressFile 压缩日志文件
func (l *Logger) compressFile(filename string) error {
	// 读取原文件
	data, err := os.ReadFile(filename)
	if err != nil {
		return err
	}
	
	// 创建压缩文件
	gzFile, err := os.Create(filename + ".gz")
	if err != nil {
		return err
	}
	defer gzFile.Close()
	
	gzWriter := gzip.NewWriter(gzFile)
	if _, err := gzWriter.Write(data); err != nil {
		return err
	}
	gzWriter.Close()
	
	// 删除原文件
	os.Remove(filename)
	
	return nil
}

// cleanupOldLogs 清理旧日志
func (l *Logger) cleanupOldLogs() {
	if l.config.DataDir == "" || l.config.MaxFileDays <= 0 {
		return
	}
	
	cutoff := time.Now().AddDate(0, 0, -l.config.MaxFileDays)
	
	entries, err := os.ReadDir(l.config.DataDir)
	if err != nil {
		return
	}
	
	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}
		
		name := entry.Name()
		if !strings.HasPrefix(name, "log_") {
			continue
		}
		
		info, err := entry.Info()
		if err != nil {
			continue
		}
		
		if info.ModTime().Before(cutoff) {
			os.Remove(filepath.Join(l.config.DataDir, name))
		}
	}
}

// Log 记录日志
func (l *Logger) Log(eventType EventType, level LogLevel, details map[string]interface{}) (*LogEntry, error) {
	if eventType == "" {
		return nil, ErrEmptyEventType
	}
	
	// 检查日志级别
	if level < l.config.MinLevel {
		return nil, nil
	}
	
	now := time.Now()
	
	// 生成日志ID
	idData := fmt.Sprintf("%s%d%s", l.config.NodeID, now.UnixNano(), eventType)
	hash := sha256.Sum256([]byte(idData))
	logID := hex.EncodeToString(hash[:16])
	
	entry := &LogEntry{
		LogID:     logID,
		NodeID:    l.config.NodeID,
		Timestamp: now,
		EventType: eventType,
		Level:     level,
		Details:   details,
	}
	
	// 签名
	if l.config.SignFunc != nil {
		signData := l.getSignData(entry)
		sig, err := l.config.SignFunc(signData)
		if err == nil {
			entry.Signature = sig
		}
	}
	
	l.mu.Lock()
	l.entries = append(l.entries, entry)
	l.entryIndex[logID] = entry
	l.mu.Unlock()
	
	// 写入文件
	l.writeToFile(entry)
	
	// 输出到控制台
	if l.config.EnableConsole {
		l.printToConsole(entry)
	}
	
	// 触发回调
	if l.OnLog != nil {
		l.OnLog(entry)
	}
	
	return entry, nil
}

// getSignData 获取签名数据
func (l *Logger) getSignData(entry *LogEntry) []byte {
	data := fmt.Sprintf("%s|%s|%d|%s", entry.LogID, entry.NodeID, entry.Timestamp.UnixNano(), entry.EventType)
	return []byte(data)
}

// writeToFile 写入日志文件
func (l *Logger) writeToFile(entry *LogEntry) {
	l.mu.Lock()
	defer l.mu.Unlock()
	
	if l.file == nil {
		return
	}
	
	data, err := json.Marshal(entry)
	if err != nil {
		return
	}
	
	data = append(data, '\n')
	n, err := l.file.Write(data)
	if err == nil {
		l.fileSize += int64(n)
	}
}

// printToConsole 输出到控制台
func (l *Logger) printToConsole(entry *LogEntry) {
	nodeIDShort := entry.NodeID
	if len(nodeIDShort) > 8 {
		nodeIDShort = nodeIDShort[:8]
	}
	fmt.Printf("[%s] [%s] [%s] %s: %v\n",
		entry.Timestamp.Format("2006-01-02 15:04:05"),
		entry.Level.String(),
		nodeIDShort,
		entry.EventType,
		entry.Details)
}

// 便捷日志方法
func (l *Logger) Debug(eventType EventType, details map[string]interface{}) (*LogEntry, error) {
	return l.Log(eventType, LevelDebug, details)
}

func (l *Logger) Info(eventType EventType, details map[string]interface{}) (*LogEntry, error) {
	return l.Log(eventType, LevelInfo, details)
}

func (l *Logger) Warn(eventType EventType, details map[string]interface{}) (*LogEntry, error) {
	return l.Log(eventType, LevelWarn, details)
}

func (l *Logger) Error(eventType EventType, details map[string]interface{}) (*LogEntry, error) {
	return l.Log(eventType, LevelError, details)
}

// LogNodeEvent 记录节点事件
func (l *Logger) LogNodeEvent(eventType EventType, targetNode string, extra map[string]interface{}) (*LogEntry, error) {
	details := map[string]interface{}{
		"target_node": targetNode,
	}
	for k, v := range extra {
		details[k] = v
	}
	return l.Info(eventType, details)
}

// LogTaskEvent 记录任务事件
func (l *Logger) LogTaskEvent(eventType EventType, taskID string, extra map[string]interface{}) (*LogEntry, error) {
	details := map[string]interface{}{
		"task_id": taskID,
	}
	for k, v := range extra {
		details[k] = v
	}
	return l.Info(eventType, details)
}

// LogReputationEvent 记录声誉事件
func (l *Logger) LogReputationEvent(eventType EventType, targetNode string, delta float64, extra map[string]interface{}) (*LogEntry, error) {
	details := map[string]interface{}{
		"target_node":      targetNode,
		"delta_reputation": delta,
	}
	for k, v := range extra {
		details[k] = v
	}
	return l.Info(eventType, details)
}

// LogAccuseEvent 记录指责事件
func (l *Logger) LogAccuseEvent(eventType EventType, accuser, accused string, extra map[string]interface{}) (*LogEntry, error) {
	details := map[string]interface{}{
		"accuser": accuser,
		"accused": accused,
	}
	for k, v := range extra {
		details[k] = v
	}
	return l.Info(eventType, details)
}

// LogMessageEvent 记录消息事件
func (l *Logger) LogMessageEvent(eventType EventType, messageID, from, to string, extra map[string]interface{}) (*LogEntry, error) {
	details := map[string]interface{}{
		"message_id": messageID,
		"from":       from,
		"to":         to,
	}
	for k, v := range extra {
		details[k] = v
	}
	return l.Info(eventType, details)
}

// LogSystemError 记录系统错误
func (l *Logger) LogSystemError(err error, context string) (*LogEntry, error) {
	details := map[string]interface{}{
		"error":   err.Error(),
		"context": context,
	}
	return l.Error(EventSystemError, details)
}

// GetEntry 获取日志条目
func (l *Logger) GetEntry(logID string) (*LogEntry, error) {
	l.mu.RLock()
	defer l.mu.RUnlock()
	
	if entry, ok := l.entryIndex[logID]; ok {
		return entry, nil
	}
	return nil, ErrLogNotFound
}

// QueryFilter 查询过滤器
type QueryFilter struct {
	NodeID    string
	EventType EventType
	Level     LogLevel
	StartTime time.Time
	EndTime   time.Time
	Limit     int
}

// Query 查询日志
func (l *Logger) Query(filter *QueryFilter) []*LogEntry {
	l.mu.RLock()
	defer l.mu.RUnlock()
	
	result := make([]*LogEntry, 0)
	
	for _, entry := range l.entries {
		// 过滤节点ID
		if filter.NodeID != "" && entry.NodeID != filter.NodeID {
			continue
		}
		
		// 过滤事件类型
		if filter.EventType != "" && entry.EventType != filter.EventType {
			continue
		}
		
		// 过滤日志级别
		if entry.Level < filter.Level {
			continue
		}
		
		// 过滤时间范围
		if !filter.StartTime.IsZero() && entry.Timestamp.Before(filter.StartTime) {
			continue
		}
		if !filter.EndTime.IsZero() && entry.Timestamp.After(filter.EndTime) {
			continue
		}
		
		result = append(result, entry)
		
		// 限制数量
		if filter.Limit > 0 && len(result) >= filter.Limit {
			break
		}
	}
	
	return result
}

// GetRecentLogs 获取最近的日志
func (l *Logger) GetRecentLogs(count int) []*LogEntry {
	l.mu.RLock()
	defer l.mu.RUnlock()
	
	total := len(l.entries)
	if count > total {
		count = total
	}
	
	result := make([]*LogEntry, count)
	copy(result, l.entries[total-count:])
	
	// 逆序（最新的在前）
	for i, j := 0, len(result)-1; i < j; i, j = i+1, j-1 {
		result[i], result[j] = result[j], result[i]
	}
	
	return result
}

// GetLogsByEventType 按事件类型获取日志
func (l *Logger) GetLogsByEventType(eventType EventType) []*LogEntry {
	l.mu.RLock()
	defer l.mu.RUnlock()
	
	result := make([]*LogEntry, 0)
	for _, entry := range l.entries {
		if entry.EventType == eventType {
			result = append(result, entry)
		}
	}
	return result
}

// LogStats 日志统计
type LogStats struct {
	TotalEntries   int                `json:"total_entries"`
	EntriesByType  map[EventType]int  `json:"entries_by_type"`
	EntriesByLevel map[LogLevel]int   `json:"entries_by_level"`
	OldestEntry    time.Time          `json:"oldest_entry"`
	NewestEntry    time.Time          `json:"newest_entry"`
	FileCount      int                `json:"file_count"`
	TotalFileSize  int64              `json:"total_file_size"`
}

// GetStats 获取统计信息
func (l *Logger) GetStats() *LogStats {
	l.mu.RLock()
	defer l.mu.RUnlock()
	
	stats := &LogStats{
		TotalEntries:   len(l.entries),
		EntriesByType:  make(map[EventType]int),
		EntriesByLevel: make(map[LogLevel]int),
	}
	
	for _, entry := range l.entries {
		stats.EntriesByType[entry.EventType]++
		stats.EntriesByLevel[entry.Level]++
		
		if stats.OldestEntry.IsZero() || entry.Timestamp.Before(stats.OldestEntry) {
			stats.OldestEntry = entry.Timestamp
		}
		if entry.Timestamp.After(stats.NewestEntry) {
			stats.NewestEntry = entry.Timestamp
		}
	}
	
	// 统计文件
	if l.config.DataDir != "" {
		entries, _ := os.ReadDir(l.config.DataDir)
		for _, e := range entries {
			if e.IsDir() {
				continue
			}
			if strings.HasPrefix(e.Name(), "log_") {
				stats.FileCount++
				info, _ := e.Info()
				if info != nil {
					stats.TotalFileSize += info.Size()
				}
			}
		}
	}
	
	return stats
}

// Export 导出日志
func (l *Logger) Export(writer io.Writer, format string, filter *QueryFilter) error {
	entries := l.Query(filter)
	
	switch format {
	case "json":
		encoder := json.NewEncoder(writer)
		encoder.SetIndent("", "  ")
		return encoder.Encode(entries)
	case "jsonl":
		for _, entry := range entries {
			data, err := json.Marshal(entry)
			if err != nil {
				return err
			}
			writer.Write(data)
			writer.Write([]byte("\n"))
		}
		return nil
	case "csv":
		// CSV 格式
		writer.Write([]byte("LogID,NodeID,Timestamp,EventType,Level,Details\n"))
		for _, entry := range entries {
			details, _ := json.Marshal(entry.Details)
			line := fmt.Sprintf("%s,%s,%s,%s,%s,%s\n",
				entry.LogID,
				entry.NodeID,
				entry.Timestamp.Format(time.RFC3339),
				entry.EventType,
				entry.Level.String(),
				string(details))
			writer.Write([]byte(line))
		}
		return nil
	default:
		return errors.New("unsupported format")
	}
}

// LoadFromFile 从文件加载日志
func (l *Logger) LoadFromFile(filename string) error {
	file, err := os.Open(filename)
	if err != nil {
		return err
	}
	defer file.Close()
	
	// 检查是否是压缩文件
	var reader io.Reader = file
	if strings.HasSuffix(filename, ".gz") {
		gzReader, err := gzip.NewReader(file)
		if err != nil {
			return err
		}
		defer gzReader.Close()
		reader = gzReader
	}
	
	// 读取所有内容
	data, err := io.ReadAll(reader)
	if err != nil {
		return err
	}
	
	// 按行解析
	lines := strings.Split(string(data), "\n")
	
	l.mu.Lock()
	defer l.mu.Unlock()
	
	for _, line := range lines {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		
		var entry LogEntry
		if err := json.Unmarshal([]byte(line), &entry); err != nil {
			continue
		}
		
		if _, exists := l.entryIndex[entry.LogID]; !exists {
			l.entries = append(l.entries, &entry)
			l.entryIndex[entry.LogID] = &entry
		}
	}
	
	// 按时间排序
	sort.Slice(l.entries, func(i, j int) bool {
		return l.entries[i].Timestamp.Before(l.entries[j].Timestamp)
	})
	
	return nil
}

// VerifyEntry 验证日志条目签名
func (l *Logger) VerifyEntry(entry *LogEntry) bool {
	if l.config.VerifyFunc == nil {
		return true
	}
	if entry.Signature == "" {
		return true
	}
	
	signData := l.getSignData(entry)
	return l.config.VerifyFunc(entry.NodeID, signData, entry.Signature)
}

// Clear 清空内存日志
func (l *Logger) Clear() {
	l.mu.Lock()
	defer l.mu.Unlock()
	
	l.entries = make([]*LogEntry, 0)
	l.entryIndex = make(map[string]*LogEntry)
}

// SetMinLevel 设置最低日志级别
func (l *Logger) SetMinLevel(level LogLevel) {
	l.mu.Lock()
	l.config.MinLevel = level
	l.mu.Unlock()
}

// SetEnableConsole 设置控制台输出
func (l *Logger) SetEnableConsole(enable bool) {
	l.mu.Lock()
	l.config.EnableConsole = enable
	l.mu.Unlock()
}

// GetEntryCount 获取条目数量
func (l *Logger) GetEntryCount() int {
	l.mu.RLock()
	defer l.mu.RUnlock()
	return len(l.entries)
}
