// Package sync - 消息确认机制
// 实现送达回执和已读回执功能
package sync

import (
	"encoding/json"
	"sync"
	"time"
)

// ReceiptStatus 回执状态
type ReceiptStatus string

const (
	ReceiptPending   ReceiptStatus = "pending"   // 等待确认
	ReceiptDelivered ReceiptStatus = "delivered" // 已送达
	ReceiptRead      ReceiptStatus = "read"      // 已读
	ReceiptFailed    ReceiptStatus = "failed"    // 发送失败
	ReceiptExpired   ReceiptStatus = "expired"   // 已过期
)

// MessageReceipt 消息回执记录
type MessageReceipt struct {
	MessageID    string        `json:"message_id"`    // 消息ID
	Sender       string        `json:"sender"`        // 发送者
	Receiver     string        `json:"receiver"`      // 接收者
	Status       ReceiptStatus `json:"status"`        // 状态
	SentAt       time.Time     `json:"sent_at"`       // 发送时间
	DeliveredAt  *time.Time    `json:"delivered_at"`  // 送达时间
	ReadAt       *time.Time    `json:"read_at"`       // 已读时间
	RetryCount   int           `json:"retry_count"`   // 重试次数
	LastError    string        `json:"last_error"`    // 最后错误
}

// ReceiptManagerConfig 回执管理器配置
type ReceiptManagerConfig struct {
	NodeID           string
	ReceiptTimeout   time.Duration // 回执超时时间
	MaxRetries       int           // 最大重试次数
	CleanupInterval  time.Duration // 清理间隔
	RetentionPeriod  time.Duration // 记录保留时间
}

// DefaultReceiptManagerConfig 默认配置
func DefaultReceiptManagerConfig(nodeID string) *ReceiptManagerConfig {
	return &ReceiptManagerConfig{
		NodeID:          nodeID,
		ReceiptTimeout:  5 * time.Minute,
		MaxRetries:      3,
		CleanupInterval: 10 * time.Minute,
		RetentionPeriod: 7 * 24 * time.Hour, // 保留7天
	}
}

// ReceiptCallback 回执回调
type ReceiptCallback func(*MessageReceipt)

// ReceiptManager 回执管理器
type ReceiptManager struct {
	config *ReceiptManagerConfig
	
	// 待确认的消息: messageID -> MessageReceipt
	pending map[string]*MessageReceipt
	
	// 已完成的回执: messageID -> MessageReceipt
	completed map[string]*MessageReceipt
	
	// 回调函数
	onDelivered ReceiptCallback
	onRead      ReceiptCallback
	onFailed    ReceiptCallback
	onTimeout   ReceiptCallback
	
	mu     sync.RWMutex
	ctx    chan struct{}
	stopCh chan struct{}
}

// NewReceiptManager 创建回执管理器
func NewReceiptManager(config *ReceiptManagerConfig) *ReceiptManager {
	if config == nil {
		config = DefaultReceiptManagerConfig("")
	}
	
	return &ReceiptManager{
		config:    config,
		pending:   make(map[string]*MessageReceipt),
		completed: make(map[string]*MessageReceipt),
		stopCh:    make(chan struct{}),
	}
}

// SetOnDelivered 设置送达回调
func (rm *ReceiptManager) SetOnDelivered(fn ReceiptCallback) {
	rm.onDelivered = fn
}

// SetOnRead 设置已读回调
func (rm *ReceiptManager) SetOnRead(fn ReceiptCallback) {
	rm.onRead = fn
}

// SetOnFailed 设置失败回调
func (rm *ReceiptManager) SetOnFailed(fn ReceiptCallback) {
	rm.onFailed = fn
}

// SetOnTimeout 设置超时回调
func (rm *ReceiptManager) SetOnTimeout(fn ReceiptCallback) {
	rm.onTimeout = fn
}

// Start 启动回执管理器
func (rm *ReceiptManager) Start() {
	go rm.checkTimeoutLoop()
	go rm.cleanupLoop()
}

// Stop 停止回执管理器
func (rm *ReceiptManager) Stop() {
	close(rm.stopCh)
}

// checkTimeoutLoop 检查超时循环
func (rm *ReceiptManager) checkTimeoutLoop() {
	ticker := time.NewTicker(30 * time.Second)
	defer ticker.Stop()
	
	for {
		select {
		case <-rm.stopCh:
			return
		case <-ticker.C:
			rm.checkTimeouts()
		}
	}
}

// cleanupLoop 清理循环
func (rm *ReceiptManager) cleanupLoop() {
	ticker := time.NewTicker(rm.config.CleanupInterval)
	defer ticker.Stop()
	
	for {
		select {
		case <-rm.stopCh:
			return
		case <-ticker.C:
			rm.cleanup()
		}
	}
}

// checkTimeouts 检查超时消息
func (rm *ReceiptManager) checkTimeouts() {
	rm.mu.Lock()
	now := time.Now()
	var timedOut []*MessageReceipt
	
	for id, receipt := range rm.pending {
		if now.Sub(receipt.SentAt) > rm.config.ReceiptTimeout {
			receipt.Status = ReceiptExpired
			timedOut = append(timedOut, receipt)
			delete(rm.pending, id)
			rm.completed[id] = receipt
		}
	}
	rm.mu.Unlock()
	
	// 触发超时回调
	for _, receipt := range timedOut {
		if rm.onTimeout != nil {
			go rm.onTimeout(receipt)
		}
	}
}

// cleanup 清理过期记录
func (rm *ReceiptManager) cleanup() {
	rm.mu.Lock()
	defer rm.mu.Unlock()
	
	now := time.Now()
	for id, receipt := range rm.completed {
		if now.Sub(receipt.SentAt) > rm.config.RetentionPeriod {
			delete(rm.completed, id)
		}
	}
}

// TrackMessage 跟踪消息
func (rm *ReceiptManager) TrackMessage(messageID, sender, receiver string) {
	rm.mu.Lock()
	defer rm.mu.Unlock()
	
	rm.pending[messageID] = &MessageReceipt{
		MessageID:  messageID,
		Sender:     sender,
		Receiver:   receiver,
		Status:     ReceiptPending,
		SentAt:     time.Now(),
		RetryCount: 0,
	}
}

// MarkDelivered 标记为已送达
func (rm *ReceiptManager) MarkDelivered(messageID string, deliveredAt time.Time) {
	rm.mu.Lock()
	receipt, ok := rm.pending[messageID]
	if !ok {
		// 检查已完成的记录
		receipt, ok = rm.completed[messageID]
		if !ok {
			rm.mu.Unlock()
			return
		}
	}
	
	receipt.Status = ReceiptDelivered
	receipt.DeliveredAt = &deliveredAt
	
	// 移动到已完成
	if _, inPending := rm.pending[messageID]; inPending {
		delete(rm.pending, messageID)
		rm.completed[messageID] = receipt
	}
	rm.mu.Unlock()
	
	// 触发回调
	if rm.onDelivered != nil {
		go rm.onDelivered(receipt)
	}
}

// MarkRead 标记为已读
func (rm *ReceiptManager) MarkRead(messageID string, readAt time.Time) {
	rm.mu.Lock()
	receipt, ok := rm.pending[messageID]
	if !ok {
		receipt, ok = rm.completed[messageID]
		if !ok {
			rm.mu.Unlock()
			return
		}
	}
	
	receipt.Status = ReceiptRead
	receipt.ReadAt = &readAt
	
	// 如果之前没有送达时间，设置为已读时间
	if receipt.DeliveredAt == nil {
		receipt.DeliveredAt = &readAt
	}
	
	// 移动到已完成
	if _, inPending := rm.pending[messageID]; inPending {
		delete(rm.pending, messageID)
		rm.completed[messageID] = receipt
	}
	rm.mu.Unlock()
	
	// 触发回调
	if rm.onRead != nil {
		go rm.onRead(receipt)
	}
}

// MarkFailed 标记为失败
func (rm *ReceiptManager) MarkFailed(messageID, errorMsg string) {
	rm.mu.Lock()
	receipt, ok := rm.pending[messageID]
	if !ok {
		rm.mu.Unlock()
		return
	}
	
	receipt.RetryCount++
	receipt.LastError = errorMsg
	
	if receipt.RetryCount >= rm.config.MaxRetries {
		receipt.Status = ReceiptFailed
		delete(rm.pending, messageID)
		rm.completed[messageID] = receipt
		rm.mu.Unlock()
		
		// 触发回调
		if rm.onFailed != nil {
			go rm.onFailed(receipt)
		}
		return
	}
	rm.mu.Unlock()
}

// GetReceipt 获取回执状态
func (rm *ReceiptManager) GetReceipt(messageID string) *MessageReceipt {
	rm.mu.RLock()
	defer rm.mu.RUnlock()
	
	if receipt, ok := rm.pending[messageID]; ok {
		return receipt
	}
	if receipt, ok := rm.completed[messageID]; ok {
		return receipt
	}
	return nil
}

// GetPendingReceipts 获取待确认的回执
func (rm *ReceiptManager) GetPendingReceipts() []*MessageReceipt {
	rm.mu.RLock()
	defer rm.mu.RUnlock()
	
	receipts := make([]*MessageReceipt, 0, len(rm.pending))
	for _, receipt := range rm.pending {
		receipts = append(receipts, receipt)
	}
	return receipts
}

// GetReceiptsByReceiver 获取指定接收者的回执
func (rm *ReceiptManager) GetReceiptsByReceiver(receiver string) []*MessageReceipt {
	rm.mu.RLock()
	defer rm.mu.RUnlock()
	
	var receipts []*MessageReceipt
	
	for _, receipt := range rm.pending {
		if receipt.Receiver == receiver {
			receipts = append(receipts, receipt)
		}
	}
	for _, receipt := range rm.completed {
		if receipt.Receiver == receiver {
			receipts = append(receipts, receipt)
		}
	}
	
	return receipts
}

// GetStats 获取统计信息
func (rm *ReceiptManager) GetStats() map[string]int {
	rm.mu.RLock()
	defer rm.mu.RUnlock()
	
	stats := map[string]int{
		"pending":   0,
		"delivered": 0,
		"read":      0,
		"failed":    0,
		"expired":   0,
	}
	
	stats["pending"] = len(rm.pending)
	
	for _, receipt := range rm.completed {
		switch receipt.Status {
		case ReceiptDelivered:
			stats["delivered"]++
		case ReceiptRead:
			stats["read"]++
		case ReceiptFailed:
			stats["failed"]++
		case ReceiptExpired:
			stats["expired"]++
		}
	}
	
	return stats
}

// ExportReceipts 导出回执（用于持久化）
func (rm *ReceiptManager) ExportReceipts() ([]byte, error) {
	rm.mu.RLock()
	defer rm.mu.RUnlock()
	
	data := struct {
		Pending   map[string]*MessageReceipt `json:"pending"`
		Completed map[string]*MessageReceipt `json:"completed"`
	}{
		Pending:   rm.pending,
		Completed: rm.completed,
	}
	
	return json.Marshal(data)
}

// ImportReceipts 导入回执（从持久化恢复）
func (rm *ReceiptManager) ImportReceipts(data []byte) error {
	var imported struct {
		Pending   map[string]*MessageReceipt `json:"pending"`
		Completed map[string]*MessageReceipt `json:"completed"`
	}
	
	if err := json.Unmarshal(data, &imported); err != nil {
		return err
	}
	
	rm.mu.Lock()
	defer rm.mu.Unlock()
	
	for k, v := range imported.Pending {
		rm.pending[k] = v
	}
	for k, v := range imported.Completed {
		rm.completed[k] = v
	}
	
	return nil
}

// ReceiptNotification 回执通知（用于P2P传输）
type ReceiptNotification struct {
	Type      string    `json:"type"`       // "delivered" 或 "read"
	MessageID string    `json:"message_id"` // 消息ID
	Timestamp time.Time `json:"timestamp"`  // 时间戳
	NodeID    string    `json:"node_id"`    // 发送通知的节点
	Signature string    `json:"signature"`  // 签名
}

// CreateDeliveryNotification 创建送达通知
func CreateDeliveryNotification(messageID, nodeID string) *ReceiptNotification {
	return &ReceiptNotification{
		Type:      "delivered",
		MessageID: messageID,
		Timestamp: time.Now(),
		NodeID:    nodeID,
	}
}

// CreateReadNotification 创建已读通知
func CreateReadNotification(messageID, nodeID string) *ReceiptNotification {
	return &ReceiptNotification{
		Type:      "read",
		MessageID: messageID,
		Timestamp: time.Now(),
		NodeID:    nodeID,
	}
}
