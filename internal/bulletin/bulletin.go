// Package bulletin 实现去中心化网络留言板功能
// 支持 DHT 存储和 Gossip 广播两种消息传播方式
package bulletin

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

// 错误定义
var (
	ErrNilConfig         = errors.New("config cannot be nil")
	ErrEmptyNodeID       = errors.New("node ID cannot be empty")
	ErrEmptyContent      = errors.New("content cannot be empty")
	ErrEmptyTopic        = errors.New("topic cannot be empty")
	ErrMessageNotFound   = errors.New("message not found")
	ErrInvalidSignature  = errors.New("invalid signature")
	ErrMessageExpired    = errors.New("message has expired")
	ErrAlreadySubscribed = errors.New("already subscribed to topic")
	ErrNotSubscribed     = errors.New("not subscribed to topic")
	ErrDuplicateMessage  = errors.New("duplicate message")
	ErrMessageTooLarge   = errors.New("message content too large")
	ErrInvalidMessageID  = errors.New("invalid message ID")
)

// MessageStatus 消息状态
type MessageStatus string

const (
	StatusActive   MessageStatus = "active"   // 有效
	StatusExpired  MessageStatus = "expired"  // 已过期
	StatusRevoked  MessageStatus = "revoked"  // 已撤回
	StatusPinned   MessageStatus = "pinned"   // 置顶
)

// Message 留言消息
type Message struct {
	MessageID       string        `json:"message_id"`       // 消息唯一ID
	Author          string        `json:"author"`           // 作者节点ID (SM2公钥)
	Topic           string        `json:"topic"`            // 消息主题/话题
	Content         string        `json:"content"`          // 消息内容
	Timestamp       time.Time     `json:"timestamp"`        // 发布时间
	ExpiresAt       time.Time     `json:"expires_at"`       // 过期时间
	Signature       string        `json:"signature"`        // SM2签名
	ReputationScore float64       `json:"reputation_score"` // 作者声誉分
	Status          MessageStatus `json:"status"`           // 消息状态
	TTL             int           `json:"ttl"`              // 剩余转发次数
	Tags            []string      `json:"tags"`             // 标签
	ReplyTo         string        `json:"reply_to"`         // 回复的消息ID（可选）
	Attachments     []string      `json:"attachments"`      // 附件（哈希引用）
}

// MessageSummary 消息摘要（用于列表展示）
type MessageSummary struct {
	MessageID       string        `json:"message_id"`
	Author          string        `json:"author"`
	Topic           string        `json:"topic"`
	Preview         string        `json:"preview"`          // 内容预览（前N个字符）
	Timestamp       time.Time     `json:"timestamp"`
	ReputationScore float64       `json:"reputation_score"`
	Status          MessageStatus `json:"status"`
}

// Subscription 订阅信息
type Subscription struct {
	Topic       string    `json:"topic"`
	SubscribedAt time.Time `json:"subscribed_at"`
	MessageCount int64     `json:"message_count"` // 收到的消息数
}

// BulletinConfig 留言板配置
type BulletinConfig struct {
	NodeID           string        // 本节点ID
	DataDir          string        // 数据存储目录
	MaxContentSize   int           // 最大消息内容大小（字节）
	DefaultTTL       int           // 默认TTL
	DefaultExpiry    time.Duration // 默认过期时间
	MaxMessagesPerTopic int        // 每个话题最大消息数
	PreviewLength    int           // 预览长度
	CleanupInterval  time.Duration // 清理间隔
	GossipEnabled    bool          // 是否启用Gossip广播
	DHTEnabled       bool          // 是否启用DHT存储
	
	// 签名验证函数
	SignFunc   func(data []byte) (string, error)
	VerifyFunc func(publicKey string, data []byte, signature string) bool
	
	// 声誉查询函数
	GetReputationFunc func(nodeID string) float64
}

// DefaultBulletinConfig 返回默认配置
func DefaultBulletinConfig(nodeID string) *BulletinConfig {
	return &BulletinConfig{
		NodeID:             nodeID,
		DataDir:            "./data/bulletin",
		MaxContentSize:     65536, // 64KB
		DefaultTTL:         10,
		DefaultExpiry:      24 * time.Hour,
		MaxMessagesPerTopic: 1000,
		PreviewLength:      100,
		CleanupInterval:    10 * time.Minute,
		GossipEnabled:      true,
		DHTEnabled:         true,
	}
}

// BulletinBoard 留言板管理器
type BulletinBoard struct {
	mu           sync.RWMutex
	config       *BulletinConfig
	messages     map[string]*Message           // MessageID -> Message
	topicIndex   map[string][]string           // Topic -> []MessageID
	authorIndex  map[string][]string           // Author -> []MessageID
	subscriptions map[string]*Subscription     // Topic -> Subscription
	subscribers  map[string][]func(*Message)  // Topic -> callbacks
	pinnedMessages []string                    // 置顶消息ID列表
	running      bool
	stopCh       chan struct{}
	
	// 回调函数
	OnMessagePublished func(*Message)
	OnMessageReceived  func(*Message)
	OnMessageRevoked   func(messageID string)
	OnTopicSubscribed  func(topic string)
	OnGossipMessage    func(*Message, string) // 消息, 来源节点
}

// NewBulletinBoard 创建留言板管理器
func NewBulletinBoard(config *BulletinConfig) (*BulletinBoard, error) {
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
	
	bb := &BulletinBoard{
		config:        config,
		messages:      make(map[string]*Message),
		topicIndex:    make(map[string][]string),
		authorIndex:   make(map[string][]string),
		subscriptions: make(map[string]*Subscription),
		subscribers:   make(map[string][]func(*Message)),
		pinnedMessages: make([]string, 0),
		stopCh:        make(chan struct{}),
	}
	
	// 加载持久化数据
	if err := bb.load(); err != nil {
		// 忽略加载错误，使用空数据
	}
	
	return bb, nil
}

// Start 启动留言板服务
func (bb *BulletinBoard) Start() {
	bb.mu.Lock()
	if bb.running {
		bb.mu.Unlock()
		return
	}
	bb.running = true
	bb.stopCh = make(chan struct{})
	bb.mu.Unlock()
	
	go bb.cleanupLoop()
}

// Stop 停止留言板服务
func (bb *BulletinBoard) Stop() {
	bb.mu.Lock()
	if !bb.running {
		bb.mu.Unlock()
		return
	}
	bb.running = false
	close(bb.stopCh)
	bb.mu.Unlock()
	
	// 保存数据
	bb.save()
}

// cleanupLoop 定期清理过期消息
func (bb *BulletinBoard) cleanupLoop() {
	ticker := time.NewTicker(bb.config.CleanupInterval)
	defer ticker.Stop()
	
	for {
		select {
		case <-ticker.C:
			bb.cleanup()
		case <-bb.stopCh:
			return
		}
	}
}

// cleanup 清理过期消息
func (bb *BulletinBoard) cleanup() {
	bb.mu.Lock()
	defer bb.mu.Unlock()
	
	now := time.Now()
	var expiredIDs []string
	
	for id, msg := range bb.messages {
		if msg.Status != StatusPinned && now.After(msg.ExpiresAt) {
			msg.Status = StatusExpired
			expiredIDs = append(expiredIDs, id)
		}
	}
	
	// 移除过期消息
	for _, id := range expiredIDs {
		bb.removeMessageLocked(id)
	}
	
	// 限制每个话题的消息数量
	for topic, messageIDs := range bb.topicIndex {
		if len(messageIDs) > bb.config.MaxMessagesPerTopic {
			// 按时间排序，保留最新的
			messages := make([]*Message, 0, len(messageIDs))
			for _, id := range messageIDs {
				if msg, ok := bb.messages[id]; ok {
					messages = append(messages, msg)
				}
			}
			sort.Slice(messages, func(i, j int) bool {
				return messages[i].Timestamp.After(messages[j].Timestamp)
			})
			
			// 删除超出限制的旧消息
			for i := bb.config.MaxMessagesPerTopic; i < len(messages); i++ {
				bb.removeMessageLocked(messages[i].MessageID)
			}
			
			// 更新索引
			newIDs := make([]string, 0, bb.config.MaxMessagesPerTopic)
			for i := 0; i < bb.config.MaxMessagesPerTopic && i < len(messages); i++ {
				newIDs = append(newIDs, messages[i].MessageID)
			}
			bb.topicIndex[topic] = newIDs
		}
	}
}

// removeMessageLocked 移除消息（需要持有锁）
func (bb *BulletinBoard) removeMessageLocked(messageID string) {
	msg, ok := bb.messages[messageID]
	if !ok {
		return
	}
	
	// 从消息表删除
	delete(bb.messages, messageID)
	
	// 从话题索引删除
	if ids, ok := bb.topicIndex[msg.Topic]; ok {
		newIDs := make([]string, 0, len(ids))
		for _, id := range ids {
			if id != messageID {
				newIDs = append(newIDs, id)
			}
		}
		bb.topicIndex[msg.Topic] = newIDs
	}
	
	// 从作者索引删除
	if ids, ok := bb.authorIndex[msg.Author]; ok {
		newIDs := make([]string, 0, len(ids))
		for _, id := range ids {
			if id != messageID {
				newIDs = append(newIDs, id)
			}
		}
		bb.authorIndex[msg.Author] = newIDs
	}
	
	// 从置顶列表删除
	newPinned := make([]string, 0, len(bb.pinnedMessages))
	for _, id := range bb.pinnedMessages {
		if id != messageID {
			newPinned = append(newPinned, id)
		}
	}
	bb.pinnedMessages = newPinned
}

// PublishMessage 发布留言
func (bb *BulletinBoard) PublishMessage(content, topic string) (*Message, error) {
	return bb.PublishMessageWithOptions(content, topic, nil, nil, "")
}

// PublishMessageWithOptions 带选项发布留言
func (bb *BulletinBoard) PublishMessageWithOptions(content, topic string, tags []string, attachments []string, replyTo string) (*Message, error) {
	if content == "" {
		return nil, ErrEmptyContent
	}
	if topic == "" {
		return nil, ErrEmptyTopic
	}
	if len(content) > bb.config.MaxContentSize {
		return nil, ErrMessageTooLarge
	}
	
	now := time.Now()
	
	// 生成消息ID
	idData := fmt.Sprintf("%s%d%s", bb.config.NodeID, now.UnixNano(), content)
	hash := sha256.Sum256([]byte(idData))
	messageID := hex.EncodeToString(hash[:])
	
	// 获取声誉分
	var reputationScore float64 = 50.0 // 默认分
	if bb.config.GetReputationFunc != nil {
		reputationScore = bb.config.GetReputationFunc(bb.config.NodeID)
	}
	
	msg := &Message{
		MessageID:       messageID,
		Author:          bb.config.NodeID,
		Topic:           topic,
		Content:         content,
		Timestamp:       now,
		ExpiresAt:       now.Add(bb.config.DefaultExpiry),
		ReputationScore: reputationScore,
		Status:          StatusActive,
		TTL:             bb.config.DefaultTTL,
		Tags:            tags,
		ReplyTo:         replyTo,
		Attachments:     attachments,
	}
	
	// 签名消息
	if bb.config.SignFunc != nil {
		signData := bb.getSignData(msg)
		sig, err := bb.config.SignFunc(signData)
		if err != nil {
			return nil, fmt.Errorf("failed to sign message: %w", err)
		}
		msg.Signature = sig
	}
	
	bb.mu.Lock()
	
	// 存储消息
	bb.messages[messageID] = msg
	
	// 更新话题索引
	bb.topicIndex[topic] = append(bb.topicIndex[topic], messageID)
	
	// 更新作者索引
	bb.authorIndex[bb.config.NodeID] = append(bb.authorIndex[bb.config.NodeID], messageID)
	
	bb.mu.Unlock()
	
	// 保存
	bb.save()
	
	// 触发回调
	if bb.OnMessagePublished != nil {
		bb.OnMessagePublished(msg)
	}
	
	// 通知订阅者
	bb.notifySubscribers(topic, msg)
	
	return msg, nil
}

// getSignData 获取签名数据
func (bb *BulletinBoard) getSignData(msg *Message) []byte {
	data := fmt.Sprintf("%s|%s|%s|%s|%d",
		msg.MessageID,
		msg.Author,
		msg.Topic,
		msg.Content,
		msg.Timestamp.UnixNano())
	return []byte(data)
}

// ReceiveMessage 接收外部消息（来自 Gossip 或 DHT）
func (bb *BulletinBoard) ReceiveMessage(msg *Message, fromNode string) error {
	if msg == nil {
		return errors.New("message is nil")
	}
	if msg.MessageID == "" {
		return ErrInvalidMessageID
	}
	
	// 检查是否过期
	if time.Now().After(msg.ExpiresAt) {
		return ErrMessageExpired
	}
	
	// 验证签名
	if bb.config.VerifyFunc != nil && msg.Signature != "" {
		signData := bb.getSignData(msg)
		if !bb.config.VerifyFunc(msg.Author, signData, msg.Signature) {
			return ErrInvalidSignature
		}
	}
	
	bb.mu.Lock()
	
	// 检查重复
	if _, exists := bb.messages[msg.MessageID]; exists {
		bb.mu.Unlock()
		return ErrDuplicateMessage
	}
	
	// 减少TTL
	msg.TTL--
	
	// 存储消息
	bb.messages[msg.MessageID] = msg
	
	// 更新索引
	bb.topicIndex[msg.Topic] = append(bb.topicIndex[msg.Topic], msg.MessageID)
	bb.authorIndex[msg.Author] = append(bb.authorIndex[msg.Author], msg.MessageID)
	
	// 更新订阅统计
	if sub, ok := bb.subscriptions[msg.Topic]; ok {
		sub.MessageCount++
	}
	
	bb.mu.Unlock()
	
	// 触发回调
	if bb.OnMessageReceived != nil {
		bb.OnMessageReceived(msg)
	}
	if bb.OnGossipMessage != nil {
		bb.OnGossipMessage(msg, fromNode)
	}
	
	// 通知订阅者
	bb.notifySubscribers(msg.Topic, msg)
	
	return nil
}

// notifySubscribers 通知订阅者
func (bb *BulletinBoard) notifySubscribers(topic string, msg *Message) {
	bb.mu.RLock()
	callbacks := bb.subscribers[topic]
	bb.mu.RUnlock()
	
	for _, cb := range callbacks {
		go cb(msg)
	}
}

// QueryMessage 查询单条消息
func (bb *BulletinBoard) QueryMessage(messageID string) (*Message, error) {
	bb.mu.RLock()
	defer bb.mu.RUnlock()
	
	msg, ok := bb.messages[messageID]
	if !ok {
		return nil, ErrMessageNotFound
	}
	
	return msg, nil
}

// QueryByTopic 查询话题下的消息
func (bb *BulletinBoard) QueryByTopic(topic string, limit, offset int) ([]*Message, error) {
	if topic == "" {
		return nil, ErrEmptyTopic
	}
	
	bb.mu.RLock()
	defer bb.mu.RUnlock()
	
	messageIDs, ok := bb.topicIndex[topic]
	if !ok {
		return []*Message{}, nil
	}
	
	// 获取消息并按时间排序
	messages := make([]*Message, 0)
	for _, id := range messageIDs {
		if msg, ok := bb.messages[id]; ok && msg.Status == StatusActive {
			messages = append(messages, msg)
		}
	}
	
	// 按声誉分和时间排序（高声誉优先，同声誉按时间）
	sort.Slice(messages, func(i, j int) bool {
		if messages[i].Status == StatusPinned && messages[j].Status != StatusPinned {
			return true
		}
		if messages[i].Status != StatusPinned && messages[j].Status == StatusPinned {
			return false
		}
		if messages[i].ReputationScore != messages[j].ReputationScore {
			return messages[i].ReputationScore > messages[j].ReputationScore
		}
		return messages[i].Timestamp.After(messages[j].Timestamp)
	})
	
	// 分页
	if offset >= len(messages) {
		return []*Message{}, nil
	}
	end := offset + limit
	if end > len(messages) {
		end = len(messages)
	}
	
	return messages[offset:end], nil
}

// QueryByAuthor 查询作者的消息
func (bb *BulletinBoard) QueryByAuthor(author string, limit, offset int) ([]*Message, error) {
	if author == "" {
		return nil, errors.New("author cannot be empty")
	}
	
	bb.mu.RLock()
	defer bb.mu.RUnlock()
	
	messageIDs, ok := bb.authorIndex[author]
	if !ok {
		return []*Message{}, nil
	}
	
	messages := make([]*Message, 0)
	for _, id := range messageIDs {
		if msg, ok := bb.messages[id]; ok && msg.Status == StatusActive {
			messages = append(messages, msg)
		}
	}
	
	// 按时间排序
	sort.Slice(messages, func(i, j int) bool {
		return messages[i].Timestamp.After(messages[j].Timestamp)
	})
	
	// 分页
	if offset >= len(messages) {
		return []*Message{}, nil
	}
	end := offset + limit
	if end > len(messages) {
		end = len(messages)
	}
	
	return messages[offset:end], nil
}

// SearchMessages 搜索消息
func (bb *BulletinBoard) SearchMessages(keyword string, limit int) []*Message {
	bb.mu.RLock()
	defer bb.mu.RUnlock()
	
	results := make([]*Message, 0)
	
	for _, msg := range bb.messages {
		if msg.Status != StatusActive {
			continue
		}
		// 简单关键词匹配
		if containsIgnoreCase(msg.Content, keyword) || containsIgnoreCase(msg.Topic, keyword) {
			results = append(results, msg)
		}
		if len(results) >= limit {
			break
		}
	}
	
	// 按声誉排序
	sort.Slice(results, func(i, j int) bool {
		return results[i].ReputationScore > results[j].ReputationScore
	})
	
	return results
}

// containsIgnoreCase 忽略大小写的包含检查
func containsIgnoreCase(s, substr string) bool {
	// 简单实现
	return len(s) >= len(substr) && (s == substr || len(substr) == 0 ||
		(len(s) > 0 && findSubstring(s, substr)))
}

func findSubstring(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		match := true
		for j := 0; j < len(substr); j++ {
			sc := s[i+j]
			pc := substr[j]
			// 转小写比较
			if sc >= 'A' && sc <= 'Z' {
				sc += 32
			}
			if pc >= 'A' && pc <= 'Z' {
				pc += 32
			}
			if sc != pc {
				match = false
				break
			}
		}
		if match {
			return true
		}
	}
	return false
}

// SubscribeTopic 订阅话题
func (bb *BulletinBoard) SubscribeTopic(topic string, callback func(*Message)) error {
	if topic == "" {
		return ErrEmptyTopic
	}
	
	bb.mu.Lock()
	defer bb.mu.Unlock()
	
	// 添加订阅
	if _, exists := bb.subscriptions[topic]; !exists {
		bb.subscriptions[topic] = &Subscription{
			Topic:        topic,
			SubscribedAt: time.Now(),
			MessageCount: 0,
		}
	}
	
	// 添加回调
	if callback != nil {
		bb.subscribers[topic] = append(bb.subscribers[topic], callback)
	}
	
	// 触发回调
	if bb.OnTopicSubscribed != nil {
		go bb.OnTopicSubscribed(topic)
	}
	
	return nil
}

// UnsubscribeTopic 取消订阅
func (bb *BulletinBoard) UnsubscribeTopic(topic string) error {
	if topic == "" {
		return ErrEmptyTopic
	}
	
	bb.mu.Lock()
	defer bb.mu.Unlock()
	
	if _, exists := bb.subscriptions[topic]; !exists {
		return ErrNotSubscribed
	}
	
	delete(bb.subscriptions, topic)
	delete(bb.subscribers, topic)
	
	return nil
}

// GetSubscriptions 获取订阅列表
func (bb *BulletinBoard) GetSubscriptions() []*Subscription {
	bb.mu.RLock()
	defer bb.mu.RUnlock()
	
	subs := make([]*Subscription, 0, len(bb.subscriptions))
	for _, sub := range bb.subscriptions {
		subs = append(subs, sub)
	}
	return subs
}

// RevokeMessage 撤回消息
func (bb *BulletinBoard) RevokeMessage(messageID string) error {
	bb.mu.Lock()
	defer bb.mu.Unlock()
	
	msg, ok := bb.messages[messageID]
	if !ok {
		return ErrMessageNotFound
	}
	
	// 只能撤回自己的消息
	if msg.Author != bb.config.NodeID {
		return errors.New("cannot revoke message from other author")
	}
	
	msg.Status = StatusRevoked
	
	// 触发回调
	if bb.OnMessageRevoked != nil {
		go bb.OnMessageRevoked(messageID)
	}
	
	return nil
}

// PinMessage 置顶消息
func (bb *BulletinBoard) PinMessage(messageID string) error {
	bb.mu.Lock()
	defer bb.mu.Unlock()
	
	msg, ok := bb.messages[messageID]
	if !ok {
		return ErrMessageNotFound
	}
	
	if msg.Status == StatusPinned {
		return nil
	}
	
	msg.Status = StatusPinned
	bb.pinnedMessages = append(bb.pinnedMessages, messageID)
	
	return nil
}

// UnpinMessage 取消置顶
func (bb *BulletinBoard) UnpinMessage(messageID string) error {
	bb.mu.Lock()
	defer bb.mu.Unlock()
	
	msg, ok := bb.messages[messageID]
	if !ok {
		return ErrMessageNotFound
	}
	
	if msg.Status != StatusPinned {
		return nil
	}
	
	msg.Status = StatusActive
	
	// 从置顶列表移除
	newPinned := make([]string, 0, len(bb.pinnedMessages)-1)
	for _, id := range bb.pinnedMessages {
		if id != messageID {
			newPinned = append(newPinned, id)
		}
	}
	bb.pinnedMessages = newPinned
	
	return nil
}

// GetPinnedMessages 获取置顶消息
func (bb *BulletinBoard) GetPinnedMessages() []*Message {
	bb.mu.RLock()
	defer bb.mu.RUnlock()
	
	messages := make([]*Message, 0, len(bb.pinnedMessages))
	for _, id := range bb.pinnedMessages {
		if msg, ok := bb.messages[id]; ok {
			messages = append(messages, msg)
		}
	}
	return messages
}

// GetTopics 获取所有话题
func (bb *BulletinBoard) GetTopics() []string {
	bb.mu.RLock()
	defer bb.mu.RUnlock()
	
	topics := make([]string, 0, len(bb.topicIndex))
	for topic := range bb.topicIndex {
		topics = append(topics, topic)
	}
	return topics
}

// GetTopicStats 获取话题统计
func (bb *BulletinBoard) GetTopicStats(topic string) (int, int) {
	bb.mu.RLock()
	defer bb.mu.RUnlock()
	
	ids, ok := bb.topicIndex[topic]
	if !ok {
		return 0, 0
	}
	
	total := len(ids)
	active := 0
	for _, id := range ids {
		if msg, ok := bb.messages[id]; ok && msg.Status == StatusActive {
			active++
		}
	}
	
	return total, active
}

// GetMessageSummaries 获取消息摘要列表
func (bb *BulletinBoard) GetMessageSummaries(topic string, limit, offset int) []*MessageSummary {
	bb.mu.RLock()
	defer bb.mu.RUnlock()
	
	messageIDs := bb.topicIndex[topic]
	if len(messageIDs) == 0 {
		return []*MessageSummary{}
	}
	
	// 获取消息
	messages := make([]*Message, 0)
	for _, id := range messageIDs {
		if msg, ok := bb.messages[id]; ok && msg.Status == StatusActive {
			messages = append(messages, msg)
		}
	}
	
	// 排序
	sort.Slice(messages, func(i, j int) bool {
		if messages[i].ReputationScore != messages[j].ReputationScore {
			return messages[i].ReputationScore > messages[j].ReputationScore
		}
		return messages[i].Timestamp.After(messages[j].Timestamp)
	})
	
	// 分页
	if offset >= len(messages) {
		return []*MessageSummary{}
	}
	end := offset + limit
	if end > len(messages) {
		end = len(messages)
	}
	
	// 转换为摘要
	summaries := make([]*MessageSummary, 0, end-offset)
	for i := offset; i < end; i++ {
		msg := messages[i]
		preview := msg.Content
		if len(preview) > bb.config.PreviewLength {
			preview = preview[:bb.config.PreviewLength] + "..."
		}
		summaries = append(summaries, &MessageSummary{
			MessageID:       msg.MessageID,
			Author:          msg.Author,
			Topic:           msg.Topic,
			Preview:         preview,
			Timestamp:       msg.Timestamp,
			ReputationScore: msg.ReputationScore,
			Status:          msg.Status,
		})
	}
	
	return summaries
}

// GetMessagesForGossip 获取需要Gossip传播的消息
func (bb *BulletinBoard) GetMessagesForGossip(limit int) []*Message {
	bb.mu.RLock()
	defer bb.mu.RUnlock()
	
	messages := make([]*Message, 0)
	for _, msg := range bb.messages {
		if msg.Status == StatusActive && msg.TTL > 0 {
			messages = append(messages, msg)
		}
	}
	
	// 按时间排序，最新的优先
	sort.Slice(messages, func(i, j int) bool {
		return messages[i].Timestamp.After(messages[j].Timestamp)
	})
	
	if len(messages) > limit {
		messages = messages[:limit]
	}
	
	return messages
}

// BulletinStats 统计信息
type BulletinStats struct {
	TotalMessages    int64         `json:"total_messages"`
	ActiveMessages   int64         `json:"active_messages"`
	TotalTopics      int           `json:"total_topics"`
	TotalAuthors     int           `json:"total_authors"`
	Subscriptions    int           `json:"subscriptions"`
	PinnedMessages   int           `json:"pinned_messages"`
}

// GetStats 获取统计信息
func (bb *BulletinBoard) GetStats() *BulletinStats {
	bb.mu.RLock()
	defer bb.mu.RUnlock()
	
	var active int64
	for _, msg := range bb.messages {
		if msg.Status == StatusActive {
			active++
		}
	}
	
	return &BulletinStats{
		TotalMessages:  int64(len(bb.messages)),
		ActiveMessages: active,
		TotalTopics:    len(bb.topicIndex),
		TotalAuthors:   len(bb.authorIndex),
		Subscriptions:  len(bb.subscriptions),
		PinnedMessages: len(bb.pinnedMessages),
	}
}

// VerifyMessage 验证消息签名
func (bb *BulletinBoard) VerifyMessage(msg *Message) bool {
	if bb.config.VerifyFunc == nil {
		return true
	}
	if msg.Signature == "" {
		return false
	}
	
	signData := bb.getSignData(msg)
	return bb.config.VerifyFunc(msg.Author, signData, msg.Signature)
}

// SetExpiry 设置消息过期时间
func (bb *BulletinBoard) SetExpiry(messageID string, expiry time.Time) error {
	bb.mu.Lock()
	defer bb.mu.Unlock()
	
	msg, ok := bb.messages[messageID]
	if !ok {
		return ErrMessageNotFound
	}
	
	msg.ExpiresAt = expiry
	return nil
}

// persistState 持久化状态
type persistState struct {
	Messages      map[string]*Message     `json:"messages"`
	Subscriptions map[string]*Subscription `json:"subscriptions"`
	PinnedMessages []string                `json:"pinned_messages"`
}

// save 保存数据
func (bb *BulletinBoard) save() error {
	if bb.config.DataDir == "" {
		return nil
	}
	
	bb.mu.RLock()
	state := &persistState{
		Messages:       bb.messages,
		Subscriptions:  bb.subscriptions,
		PinnedMessages: bb.pinnedMessages,
	}
	bb.mu.RUnlock()
	
	data, err := json.MarshalIndent(state, "", "  ")
	if err != nil {
		return err
	}
	
	filePath := filepath.Join(bb.config.DataDir, "bulletin.json")
	return os.WriteFile(filePath, data, 0644)
}

// load 加载数据
func (bb *BulletinBoard) load() error {
	if bb.config.DataDir == "" {
		return nil
	}
	
	filePath := filepath.Join(bb.config.DataDir, "bulletin.json")
	data, err := os.ReadFile(filePath)
	if err != nil {
		if os.IsNotExist(err) {
			return nil
		}
		return err
	}
	
	var state persistState
	if err := json.Unmarshal(data, &state); err != nil {
		return err
	}
	
	bb.mu.Lock()
	defer bb.mu.Unlock()
	
	if state.Messages != nil {
		bb.messages = state.Messages
	}
	if state.Subscriptions != nil {
		bb.subscriptions = state.Subscriptions
	}
	if state.PinnedMessages != nil {
		bb.pinnedMessages = state.PinnedMessages
	}
	
	// 重建索引
	for id, msg := range bb.messages {
		bb.topicIndex[msg.Topic] = append(bb.topicIndex[msg.Topic], id)
		bb.authorIndex[msg.Author] = append(bb.authorIndex[msg.Author], id)
	}
	
	return nil
}

// Clear 清空所有数据
func (bb *BulletinBoard) Clear() {
	bb.mu.Lock()
	defer bb.mu.Unlock()
	
	bb.messages = make(map[string]*Message)
	bb.topicIndex = make(map[string][]string)
	bb.authorIndex = make(map[string][]string)
	bb.pinnedMessages = make([]string, 0)
}
