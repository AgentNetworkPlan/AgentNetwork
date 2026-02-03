// Package mailbox 实现 P2P 网络中的消息邮箱功能
// 支持点对点消息发送、离线消息存储、消息签名验证
package mailbox

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"sync"
	"time"
)

// MessageStatus 消息状态
type MessageStatus string

const (
	StatusPending   MessageStatus = "pending"   // 待处理
	StatusDelivered MessageStatus = "delivered" // 已投递
	StatusRead      MessageStatus = "read"      // 已读
	StatusExpired   MessageStatus = "expired"   // 已过期
	StatusFailed    MessageStatus = "failed"    // 发送失败
)

// Message 邮箱消息结构
type Message struct {
	ID        string        `json:"id"`        // 消息唯一ID
	Sender    string        `json:"sender"`    // 发送者公钥/节点ID
	Receiver  string        `json:"receiver"`  // 接收者公钥/节点ID
	Subject   string        `json:"subject"`   // 消息主题（可选）
	Content   []byte        `json:"content"`   // 消息内容（可能加密）
	Encrypted bool          `json:"encrypted"` // 是否加密
	Timestamp time.Time     `json:"timestamp"` // 发送时间
	ExpiresAt time.Time     `json:"expires_at"`// 过期时间
	Status    MessageStatus `json:"status"`    // 消息状态
	Signature []byte        `json:"signature"` // SM2 签名
	ReadAt    *time.Time    `json:"read_at,omitempty"` // 阅读时间
}

// MessageSummary 消息摘要（用于列表展示）
type MessageSummary struct {
	ID        string        `json:"id"`
	Sender    string        `json:"sender"`
	Subject   string        `json:"subject"`
	Timestamp time.Time     `json:"timestamp"`
	Status    MessageStatus `json:"status"`
	Encrypted bool          `json:"encrypted"`
}

// SignFunc 签名函数类型
type SignFunc func(data []byte) ([]byte, error)

// VerifyFunc 验签函数类型
type VerifyFunc func(pubKey string, data, signature []byte) (bool, error)

// EncryptFunc 加密函数类型
type EncryptFunc func(pubKey string, data []byte) ([]byte, error)

// DecryptFunc 解密函数类型
type DecryptFunc func(data []byte) ([]byte, error)

// DeliverFunc 消息投递函数类型（用于在线投递）
type DeliverFunc func(receiver string, msg *Message) error

// MailboxConfig 邮箱配置
type MailboxConfig struct {
	NodeID          string        // 当前节点ID
	DataDir         string        // 数据存储目录
	MaxInboxSize    int           // 收件箱最大消息数
	MaxOutboxSize   int           // 发件箱最大消息数
	DefaultTTL      time.Duration // 默认消息存活时间
	CleanupInterval time.Duration // 清理间隔
	EnableEncrypt   bool          // 是否启用加密
}

// DefaultConfig 返回默认配置
func DefaultConfig(nodeID string) *MailboxConfig {
	return &MailboxConfig{
		NodeID:          nodeID,
		DataDir:         "./data/mailbox",
		MaxInboxSize:    1000,
		MaxOutboxSize:   500,
		DefaultTTL:      48 * time.Hour,
		CleanupInterval: 1 * time.Hour,
		EnableEncrypt:   true,
	}
}

// Mailbox 邮箱管理器
type Mailbox struct {
	config   *MailboxConfig
	inbox    map[string]*Message   // 收件箱: messageID -> Message
	outbox   map[string]*Message   // 发件箱: messageID -> Message
	pending  map[string][]*Message // 待投递消息: receiverID -> Messages (作为中继时使用)
	mu       sync.RWMutex

	signFunc    SignFunc    // 签名函数
	verifyFunc  VerifyFunc  // 验签函数
	encryptFunc EncryptFunc // 加密函数
	decryptFunc DecryptFunc // 解密函数
	deliverFunc DeliverFunc // 在线投递函数

	// 回调
	onMessageReceived func(*Message)
	onMessageSent     func(*Message)
	onMessageRead     func(*Message)

	stopCh chan struct{}
	wg     sync.WaitGroup
}

// NewMailbox 创建邮箱管理器
func NewMailbox(config *MailboxConfig) (*Mailbox, error) {
	if config == nil {
		return nil, errors.New("config is required")
	}
	if config.NodeID == "" {
		return nil, errors.New("node ID is required")
	}

	// 创建数据目录
	if config.DataDir != "" {
		if err := os.MkdirAll(config.DataDir, 0755); err != nil {
			return nil, fmt.Errorf("failed to create data dir: %w", err)
		}
	}

	mb := &Mailbox{
		config:  config,
		inbox:   make(map[string]*Message),
		outbox:  make(map[string]*Message),
		pending: make(map[string][]*Message),
		stopCh:  make(chan struct{}),
	}

	return mb, nil
}

// SetSignFunc 设置签名函数
func (m *Mailbox) SetSignFunc(fn SignFunc) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.signFunc = fn
}

// SetVerifyFunc 设置验签函数
func (m *Mailbox) SetVerifyFunc(fn VerifyFunc) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.verifyFunc = fn
}

// SetEncryptFunc 设置加密函数
func (m *Mailbox) SetEncryptFunc(fn EncryptFunc) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.encryptFunc = fn
}

// SetDecryptFunc 设置解密函数
func (m *Mailbox) SetDecryptFunc(fn DecryptFunc) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.decryptFunc = fn
}

// SetDeliverFunc 设置在线投递函数
func (m *Mailbox) SetDeliverFunc(fn DeliverFunc) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.deliverFunc = fn
}

// SetOnMessageReceived 设置消息接收回调
func (m *Mailbox) SetOnMessageReceived(fn func(*Message)) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.onMessageReceived = fn
}

// SetOnMessageSent 设置消息发送回调
func (m *Mailbox) SetOnMessageSent(fn func(*Message)) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.onMessageSent = fn
}

// SetOnMessageRead 设置消息阅读回调
func (m *Mailbox) SetOnMessageRead(fn func(*Message)) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.onMessageRead = fn
}

// Start 启动邮箱服务
func (m *Mailbox) Start() error {
	// 加载持久化数据
	if err := m.loadFromDisk(); err != nil {
		// 非致命错误，记录并继续
		fmt.Printf("Warning: failed to load mailbox data: %v\n", err)
	}

	// 启动清理协程
	m.wg.Add(1)
	go m.cleanupLoop()

	return nil
}

// Stop 停止邮箱服务
func (m *Mailbox) Stop() error {
	close(m.stopCh)
	m.wg.Wait()

	// 持久化数据
	return m.saveToDisk()
}

// SendMessage 发送消息
func (m *Mailbox) SendMessage(receiver, subject string, content []byte, encrypt bool) (*Message, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	if receiver == "" {
		return nil, errors.New("receiver is required")
	}
	if len(content) == 0 {
		return nil, errors.New("content is required")
	}

	// 检查发件箱大小
	if len(m.outbox) >= m.config.MaxOutboxSize {
		return nil, errors.New("outbox is full")
	}

	// 创建消息
	msg := &Message{
		Sender:    m.config.NodeID,
		Receiver:  receiver,
		Subject:   subject,
		Content:   content,
		Encrypted: false,
		Timestamp: time.Now(),
		ExpiresAt: time.Now().Add(m.config.DefaultTTL),
		Status:    StatusPending,
	}

	// 加密内容（如果需要）
	if encrypt && m.encryptFunc != nil {
		encryptedContent, err := m.encryptFunc(receiver, content)
		if err != nil {
			return nil, fmt.Errorf("failed to encrypt: %w", err)
		}
		msg.Content = encryptedContent
		msg.Encrypted = true
	}

	// 生成消息ID
	msg.ID = m.generateMessageID(msg)

	// 签名消息
	if m.signFunc != nil {
		signData := m.getSignData(msg)
		sig, err := m.signFunc(signData)
		if err != nil {
			return nil, fmt.Errorf("failed to sign: %w", err)
		}
		msg.Signature = sig
	}

	// 尝试在线投递
	if m.deliverFunc != nil {
		err := m.deliverFunc(receiver, msg)
		if err == nil {
			msg.Status = StatusDelivered
		}
		// 投递失败时保持 pending 状态，等待稍后重试
	}

	// 存入发件箱
	m.outbox[msg.ID] = msg

	// 触发回调
	if m.onMessageSent != nil {
		go m.onMessageSent(msg)
	}

	return msg, nil
}

// ReceiveMessage 接收消息（验证并存入收件箱）
func (m *Mailbox) ReceiveMessage(msg *Message) error {
	if msg == nil {
		return errors.New("message is nil")
	}

	m.mu.Lock()
	defer m.mu.Unlock()

	// 检查是否是发给自己的
	if msg.Receiver != m.config.NodeID {
		return errors.New("message is not for this node")
	}

	// 检查是否已过期
	if time.Now().After(msg.ExpiresAt) {
		return errors.New("message has expired")
	}

	// 检查是否已存在
	if _, exists := m.inbox[msg.ID]; exists {
		return errors.New("message already exists")
	}

	// 验证签名
	if m.verifyFunc != nil && len(msg.Signature) > 0 {
		signData := m.getSignData(msg)
		valid, err := m.verifyFunc(msg.Sender, signData, msg.Signature)
		if err != nil {
			return fmt.Errorf("failed to verify signature: %w", err)
		}
		if !valid {
			return errors.New("invalid signature")
		}
	}

	// 检查收件箱大小
	if len(m.inbox) >= m.config.MaxInboxSize {
		// 删除最旧的消息
		m.removeOldestInbox()
	}

	// 更新状态为已投递
	msg.Status = StatusDelivered

	// 存入收件箱
	m.inbox[msg.ID] = msg

	// 触发回调
	if m.onMessageReceived != nil {
		go m.onMessageReceived(msg)
	}

	return nil
}

// GetMessage 获取指定消息
func (m *Mailbox) GetMessage(messageID string) (*Message, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	if msg, ok := m.inbox[messageID]; ok {
		return msg, nil
	}
	if msg, ok := m.outbox[messageID]; ok {
		return msg, nil
	}

	return nil, errors.New("message not found")
}

// GetMessageContent 获取消息内容（解密如果需要）
func (m *Mailbox) GetMessageContent(messageID string) ([]byte, error) {
	m.mu.RLock()
	msg, ok := m.inbox[messageID]
	decryptFunc := m.decryptFunc
	m.mu.RUnlock()

	if !ok {
		return nil, errors.New("message not found")
	}

	content := msg.Content
	if msg.Encrypted && decryptFunc != nil {
		decrypted, err := decryptFunc(content)
		if err != nil {
			return nil, fmt.Errorf("failed to decrypt: %w", err)
		}
		content = decrypted
	}

	return content, nil
}

// MarkAsRead 标记消息为已读
func (m *Mailbox) MarkAsRead(messageID string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	msg, ok := m.inbox[messageID]
	if !ok {
		return errors.New("message not found")
	}

	if msg.Status == StatusRead {
		return nil // 已经是已读状态
	}

	now := time.Now()
	msg.Status = StatusRead
	msg.ReadAt = &now

	// 触发回调
	if m.onMessageRead != nil {
		go m.onMessageRead(msg)
	}

	return nil
}

// ListInbox 列出收件箱消息
func (m *Mailbox) ListInbox(limit, offset int) []*MessageSummary {
	m.mu.RLock()
	defer m.mu.RUnlock()

	messages := make([]*Message, 0, len(m.inbox))
	for _, msg := range m.inbox {
		messages = append(messages, msg)
	}

	// 按时间倒序排列
	sort.Slice(messages, func(i, j int) bool {
		return messages[i].Timestamp.After(messages[j].Timestamp)
	})

	// 分页
	if offset >= len(messages) {
		return nil
	}
	end := offset + limit
	if end > len(messages) {
		end = len(messages)
	}
	messages = messages[offset:end]

	// 转换为摘要
	summaries := make([]*MessageSummary, len(messages))
	for i, msg := range messages {
		summaries[i] = &MessageSummary{
			ID:        msg.ID,
			Sender:    msg.Sender,
			Subject:   msg.Subject,
			Timestamp: msg.Timestamp,
			Status:    msg.Status,
			Encrypted: msg.Encrypted,
		}
	}

	return summaries
}

// ListOutbox 列出发件箱消息
func (m *Mailbox) ListOutbox(limit, offset int) []*MessageSummary {
	m.mu.RLock()
	defer m.mu.RUnlock()

	messages := make([]*Message, 0, len(m.outbox))
	for _, msg := range m.outbox {
		messages = append(messages, msg)
	}

	// 按时间倒序排列
	sort.Slice(messages, func(i, j int) bool {
		return messages[i].Timestamp.After(messages[j].Timestamp)
	})

	// 分页
	if offset >= len(messages) {
		return nil
	}
	end := offset + limit
	if end > len(messages) {
		end = len(messages)
	}
	messages = messages[offset:end]

	// 转换为摘要
	summaries := make([]*MessageSummary, len(messages))
	for i, msg := range messages {
		summaries[i] = &MessageSummary{
			ID:        msg.ID,
			Sender:    msg.Sender,
			Subject:   msg.Subject,
			Timestamp: msg.Timestamp,
			Status:    msg.Status,
			Encrypted: msg.Encrypted,
		}
	}

	return summaries
}

// DeleteMessage 删除消息
func (m *Mailbox) DeleteMessage(messageID string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	if _, ok := m.inbox[messageID]; ok {
		delete(m.inbox, messageID)
		return nil
	}
	if _, ok := m.outbox[messageID]; ok {
		delete(m.outbox, messageID)
		return nil
	}

	return errors.New("message not found")
}

// GetUnreadCount 获取未读消息数量
func (m *Mailbox) GetUnreadCount() int {
	m.mu.RLock()
	defer m.mu.RUnlock()

	count := 0
	for _, msg := range m.inbox {
		if msg.Status != StatusRead {
			count++
		}
	}
	return count
}

// GetInboxCount 获取收件箱消息总数
func (m *Mailbox) GetInboxCount() int {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return len(m.inbox)
}

// GetOutboxCount 获取发件箱消息总数
func (m *Mailbox) GetOutboxCount() int {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return len(m.outbox)
}

// === 中继功能 ===

// StoreForRelay 作为中继存储离线消息
func (m *Mailbox) StoreForRelay(msg *Message) error {
	if msg == nil {
		return errors.New("message is nil")
	}

	m.mu.Lock()
	defer m.mu.Unlock()

	// 验证签名
	if m.verifyFunc != nil && len(msg.Signature) > 0 {
		signData := m.getSignData(msg)
		valid, err := m.verifyFunc(msg.Sender, signData, msg.Signature)
		if err != nil {
			return fmt.Errorf("failed to verify signature: %w", err)
		}
		if !valid {
			return errors.New("invalid signature")
		}
	}

	// 存储消息
	m.pending[msg.Receiver] = append(m.pending[msg.Receiver], msg)

	return nil
}

// FetchPendingMessages 获取待投递的离线消息
func (m *Mailbox) FetchPendingMessages(receiverID string, limit int) []*Message {
	m.mu.Lock()
	defer m.mu.Unlock()

	messages, ok := m.pending[receiverID]
	if !ok || len(messages) == 0 {
		return nil
	}

	// 获取指定数量
	if limit <= 0 || limit > len(messages) {
		limit = len(messages)
	}

	result := make([]*Message, limit)
	copy(result, messages[:limit])

	// 从待处理列表中移除
	m.pending[receiverID] = messages[limit:]
	if len(m.pending[receiverID]) == 0 {
		delete(m.pending, receiverID)
	}

	return result
}

// GetPendingCount 获取某节点的待投递消息数量
func (m *Mailbox) GetPendingCount(receiverID string) int {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return len(m.pending[receiverID])
}

// === 内部方法 ===

// generateMessageID 生成消息ID
func (m *Mailbox) generateMessageID(msg *Message) string {
	data := fmt.Sprintf("%s|%s|%d|%s",
		msg.Sender,
		msg.Receiver,
		msg.Timestamp.UnixNano(),
		string(msg.Content),
	)
	hash := sha256.Sum256([]byte(data))
	return hex.EncodeToString(hash[:16]) // 使用前16字节
}

// getSignData 获取用于签名的数据
func (m *Mailbox) getSignData(msg *Message) []byte {
	data := fmt.Sprintf("%s|%s|%s|%d|%s",
		msg.ID,
		msg.Sender,
		msg.Receiver,
		msg.Timestamp.UnixNano(),
		string(msg.Content),
	)
	return []byte(data)
}

// removeOldestInbox 移除收件箱中最旧的消息
func (m *Mailbox) removeOldestInbox() {
	var oldest *Message
	var oldestID string

	for id, msg := range m.inbox {
		if oldest == nil || msg.Timestamp.Before(oldest.Timestamp) {
			oldest = msg
			oldestID = id
		}
	}

	if oldestID != "" {
		delete(m.inbox, oldestID)
	}
}

// cleanupLoop 清理过期消息
func (m *Mailbox) cleanupLoop() {
	defer m.wg.Done()

	ticker := time.NewTicker(m.config.CleanupInterval)
	defer ticker.Stop()

	for {
		select {
		case <-ticker.C:
			m.cleanup()
		case <-m.stopCh:
			return
		}
	}
}

// cleanup 清理过期消息
func (m *Mailbox) cleanup() {
	m.mu.Lock()
	defer m.mu.Unlock()

	now := time.Now()

	// 清理收件箱
	for id, msg := range m.inbox {
		if now.After(msg.ExpiresAt) {
			delete(m.inbox, id)
		}
	}

	// 清理发件箱
	for id, msg := range m.outbox {
		if now.After(msg.ExpiresAt) {
			delete(m.outbox, id)
		}
	}

	// 清理待投递消息
	for receiver, messages := range m.pending {
		filtered := make([]*Message, 0, len(messages))
		for _, msg := range messages {
			if !now.After(msg.ExpiresAt) {
				filtered = append(filtered, msg)
			}
		}
		if len(filtered) == 0 {
			delete(m.pending, receiver)
		} else {
			m.pending[receiver] = filtered
		}
	}
}

// === 持久化 ===

type mailboxData struct {
	Inbox   map[string]*Message   `json:"inbox"`
	Outbox  map[string]*Message   `json:"outbox"`
	Pending map[string][]*Message `json:"pending"`
}

// saveToDisk 保存到磁盘
func (m *Mailbox) saveToDisk() error {
	if m.config.DataDir == "" {
		return nil
	}

	m.mu.RLock()
	data := &mailboxData{
		Inbox:   m.inbox,
		Outbox:  m.outbox,
		Pending: m.pending,
	}
	m.mu.RUnlock()

	jsonData, err := json.MarshalIndent(data, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to marshal mailbox data: %w", err)
	}

	filePath := filepath.Join(m.config.DataDir, "mailbox.json")
	if err := os.WriteFile(filePath, jsonData, 0644); err != nil {
		return fmt.Errorf("failed to write mailbox data: %w", err)
	}

	return nil
}

// loadFromDisk 从磁盘加载
func (m *Mailbox) loadFromDisk() error {
	if m.config.DataDir == "" {
		return nil
	}

	filePath := filepath.Join(m.config.DataDir, "mailbox.json")
	if _, err := os.Stat(filePath); os.IsNotExist(err) {
		return nil // 文件不存在，正常情况
	}

	jsonData, err := os.ReadFile(filePath)
	if err != nil {
		return fmt.Errorf("failed to read mailbox data: %w", err)
	}

	var data mailboxData
	if err := json.Unmarshal(jsonData, &data); err != nil {
		return fmt.Errorf("failed to unmarshal mailbox data: %w", err)
	}

	m.mu.Lock()
	defer m.mu.Unlock()

	if data.Inbox != nil {
		m.inbox = data.Inbox
	}
	if data.Outbox != nil {
		m.outbox = data.Outbox
	}
	if data.Pending != nil {
		m.pending = data.Pending
	}

	return nil
}

// Stats 邮箱统计信息
type Stats struct {
	InboxCount    int `json:"inbox_count"`
	OutboxCount   int `json:"outbox_count"`
	UnreadCount   int `json:"unread_count"`
	PendingCount  int `json:"pending_count"` // 作为中继时的待投递消息总数
}

// GetStats 获取统计信息
func (m *Mailbox) GetStats() *Stats {
	m.mu.RLock()
	defer m.mu.RUnlock()

	stats := &Stats{
		InboxCount:  len(m.inbox),
		OutboxCount: len(m.outbox),
	}

	for _, msg := range m.inbox {
		if msg.Status != StatusRead {
			stats.UnreadCount++
		}
	}

	for _, messages := range m.pending {
		stats.PendingCount += len(messages)
	}

	return stats
}
