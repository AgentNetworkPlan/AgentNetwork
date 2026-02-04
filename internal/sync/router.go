// Package sync - P2P消息路由器
// 实现邮件的点对点传输和中继转发
package sync

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"sync"
	"time"
)

var (
	ErrNodeNotFound   = errors.New("node not found")
	ErrDeliveryFailed = errors.New("delivery failed")
	ErrMessageExpired = errors.New("message expired")
	ErrInvalidMessage = errors.New("invalid message")
	ErrBlacklisted    = errors.New("sender is blacklisted")
)

// RouteStrategy 路由策略
type RouteStrategy string

const (
	StrategyDirect  RouteStrategy = "direct"  // 直接发送
	StrategyRelay   RouteStrategy = "relay"   // 中继转发
	StrategyFlood   RouteStrategy = "flood"   // 泛洪
	StrategySmart   RouteStrategy = "smart"   // 智能选择
)

// PeerConnector 节点连接器接口
type PeerConnector interface {
	// IsConnected 检查节点是否连接
	IsConnected(nodeID string) bool
	// Connect 连接到节点
	Connect(ctx context.Context, nodeID string) error
	// Send 发送消息
	Send(ctx context.Context, nodeID string, data []byte) error
	// GetConnectedPeers 获取已连接节点
	GetConnectedPeers() []string
	// GetPeerInfo 获取节点信息
	GetPeerInfo(nodeID string) (*PeerInfo, error)
}

// MessageSigner 消息签名器接口
type MessageSigner interface {
	Sign(data []byte) ([]byte, error)
	Verify(publicKey string, data, signature []byte) (bool, error)
	GetNodeID() string
	GetPublicKey() string
}

// NeighborProvider 邻居提供者接口
type NeighborProvider interface {
	GetNeighbors() []string
	GetRelayNodes() []string
}

// ReputationChecker 声誉检查器接口
type ReputationChecker interface {
	GetReputation(nodeID string) float64
	IsBlacklisted(nodeID string) bool
}

// RouterConfig 路由器配置
type RouterConfig struct {
	NodeID             string
	MaxTTL             int           // 最大跳数
	MessageTimeout     time.Duration // 消息超时
	RetryCount         int           // 重试次数
	RetryInterval      time.Duration // 重试间隔
	CacheSize          int           // 缓存大小
	CacheExpiry        time.Duration // 缓存过期时间
	EnableDeliveryReceipt bool       // 启用送达回执
	EnableReadReceipt    bool        // 启用已读回执
}

// DefaultRouterConfig 默认路由器配置
func DefaultRouterConfig(nodeID string) *RouterConfig {
	return &RouterConfig{
		NodeID:             nodeID,
		MaxTTL:             10,
		MessageTimeout:     30 * time.Second,
		RetryCount:         3,
		RetryInterval:      5 * time.Second,
		CacheSize:          10000,
		CacheExpiry:        24 * time.Hour,
		EnableDeliveryReceipt: true,
		EnableReadReceipt:    true,
	}
}

// messageCache 消息缓存（防重放）
type messageCache struct {
	seen   map[string]time.Time
	mu     sync.RWMutex
	expiry time.Duration
}

func newMessageCache(expiry time.Duration) *messageCache {
	return &messageCache{
		seen:   make(map[string]time.Time),
		expiry: expiry,
	}
}

func (c *messageCache) add(id string) {
	c.mu.Lock()
	c.seen[id] = time.Now()
	c.mu.Unlock()
}

func (c *messageCache) has(id string) bool {
	c.mu.RLock()
	_, exists := c.seen[id]
	c.mu.RUnlock()
	return exists
}

func (c *messageCache) cleanup() {
	c.mu.Lock()
	now := time.Now()
	for id, t := range c.seen {
		if now.Sub(t) > c.expiry {
			delete(c.seen, id)
		}
	}
	c.mu.Unlock()
}

// pendingMessage 待处理消息
type pendingMessage struct {
	msg       *SyncMessage
	attempts  int
	nextRetry time.Time
}

// MailRouter 邮件路由器
type MailRouter struct {
	config *RouterConfig
	
	connector   PeerConnector
	signer      MessageSigner
	neighbors   NeighborProvider
	reputation  ReputationChecker
	
	// 消息缓存
	cache *messageCache
	
	// 待处理队列
	pending map[string]*pendingMessage
	
	// 回执回调
	onDelivered func(receipt *DeliveryReceipt)
	onRead      func(receipt *ReadReceipt)
	onReceive   func(msg *SyncMessage, payload *MailPayload)
	
	mu     sync.RWMutex
	ctx    context.Context
	cancel context.CancelFunc
	wg     sync.WaitGroup
}

// NewMailRouter 创建邮件路由器
func NewMailRouter(config *RouterConfig) *MailRouter {
	if config == nil {
		config = DefaultRouterConfig("")
	}
	
	ctx, cancel := context.WithCancel(context.Background())
	
	return &MailRouter{
		config:  config,
		cache:   newMessageCache(config.CacheExpiry),
		pending: make(map[string]*pendingMessage),
		ctx:     ctx,
		cancel:  cancel,
	}
}

// SetPeerConnector 设置节点连接器
func (r *MailRouter) SetPeerConnector(c PeerConnector) {
	r.connector = c
}

// SetSigner 设置签名器
func (r *MailRouter) SetSigner(s MessageSigner) {
	r.signer = s
}

// SetNeighborProvider 设置邻居提供者
func (r *MailRouter) SetNeighborProvider(n NeighborProvider) {
	r.neighbors = n
}

// SetReputationChecker 设置声誉检查器
func (r *MailRouter) SetReputationChecker(rc ReputationChecker) {
	r.reputation = rc
}

// SetOnDelivered 设置送达回调
func (r *MailRouter) SetOnDelivered(fn func(*DeliveryReceipt)) {
	r.onDelivered = fn
}

// SetOnRead 设置已读回调
func (r *MailRouter) SetOnRead(fn func(*ReadReceipt)) {
	r.onRead = fn
}

// SetOnReceive 设置接收回调
func (r *MailRouter) SetOnReceive(fn func(*SyncMessage, *MailPayload)) {
	r.onReceive = fn
}

// Start 启动路由器
func (r *MailRouter) Start() {
	// 启动缓存清理
	r.wg.Add(1)
	go r.cacheCleanupLoop()
	
	// 启动重试处理
	r.wg.Add(1)
	go r.retryLoop()
}

// Stop 停止路由器
func (r *MailRouter) Stop() {
	r.cancel()
	r.wg.Wait()
}

// cacheCleanupLoop 缓存清理循环
func (r *MailRouter) cacheCleanupLoop() {
	defer r.wg.Done()
	ticker := time.NewTicker(10 * time.Minute)
	defer ticker.Stop()
	
	for {
		select {
		case <-r.ctx.Done():
			return
		case <-ticker.C:
			r.cache.cleanup()
		}
	}
}

// retryLoop 重试循环
func (r *MailRouter) retryLoop() {
	defer r.wg.Done()
	ticker := time.NewTicker(r.config.RetryInterval)
	defer ticker.Stop()
	
	for {
		select {
		case <-r.ctx.Done():
			return
		case <-ticker.C:
			r.processRetries()
		}
	}
}

// processRetries 处理重试
func (r *MailRouter) processRetries() {
	r.mu.Lock()
	now := time.Now()
	toRetry := make([]*pendingMessage, 0)
	
	for id, pm := range r.pending {
		if now.After(pm.nextRetry) {
			if pm.attempts >= r.config.RetryCount {
				// 超过重试次数，删除
				delete(r.pending, id)
			} else {
				toRetry = append(toRetry, pm)
			}
		}
	}
	r.mu.Unlock()
	
	// 重试发送
	for _, pm := range toRetry {
		go r.retrySend(pm)
	}
}

// retrySend 重试发送
func (r *MailRouter) retrySend(pm *pendingMessage) {
	pm.attempts++
	pm.nextRetry = time.Now().Add(r.config.RetryInterval)
	
	if err := r.doSend(pm.msg); err != nil {
		// 发送失败，等待下次重试
		return
	}
	
	// 发送成功，移除待处理
	r.mu.Lock()
	delete(r.pending, pm.msg.ID)
	r.mu.Unlock()
}

// SendMail 发送邮件
func (r *MailRouter) SendMail(receiver, subject string, content []byte, encrypted bool) error {
	if r.connector == nil {
		return errors.New("peer connector not set")
	}
	
	// 检查接收者声誉
	if r.reputation != nil && r.reputation.IsBlacklisted(receiver) {
		return ErrBlacklisted
	}
	
	// 创建载荷
	payload := &MailPayload{
		MessageID: generateID(),
		Subject:   subject,
		Content:   content,
	}
	
	payloadBytes, err := json.Marshal(payload)
	if err != nil {
		return fmt.Errorf("marshal payload: %w", err)
	}
	
	// 创建同步消息
	msg := &SyncMessage{
		ID:        generateID(),
		Type:      TypeMailSend,
		Sender:    r.config.NodeID,
		Receiver:  receiver,
		Timestamp: time.Now(),
		TTL:       r.config.MaxTTL,
		Nonce:     generateNonce(),
		Payload:   payloadBytes,
		Encrypted: encrypted,
	}
	
	// 签名
	if r.signer != nil {
		signData := r.getSignData(msg)
		sig, err := r.signer.Sign(signData)
		if err != nil {
			return fmt.Errorf("sign message: %w", err)
		}
		msg.Signature = hex.EncodeToString(sig)
	}
	
	// 发送
	return r.sendMessage(msg)
}

// sendMessage 发送消息
func (r *MailRouter) sendMessage(msg *SyncMessage) error {
	// 添加到缓存防重复
	r.cache.add(msg.ID)
	
	// 尝试直接发送
	if err := r.doSend(msg); err != nil {
		// 添加到待处理队列
		r.mu.Lock()
		r.pending[msg.ID] = &pendingMessage{
			msg:       msg,
			attempts:  0,
			nextRetry: time.Now().Add(r.config.RetryInterval),
		}
		r.mu.Unlock()
		return err
	}
	
	return nil
}

// doSend 执行发送
func (r *MailRouter) doSend(msg *SyncMessage) error {
	// 检查连接器是否存在
	if r.connector == nil {
		return ErrDeliveryFailed
	}
	
	// 序列化消息
	data, err := json.Marshal(msg)
	if err != nil {
		return fmt.Errorf("marshal message: %w", err)
	}
	
	ctx, cancel := context.WithTimeout(r.ctx, r.config.MessageTimeout)
	defer cancel()
	
	// 策略1: 直接发送
	if r.connector.IsConnected(msg.Receiver) {
		if err := r.connector.Send(ctx, msg.Receiver, data); err == nil {
			return nil
		}
	}
	
	// 策略2: 尝试连接后发送
	if err := r.connector.Connect(ctx, msg.Receiver); err == nil {
		if err := r.connector.Send(ctx, msg.Receiver, data); err == nil {
			return nil
		}
	}
	
	// 策略3: 中继转发
	if r.neighbors != nil {
		// 先尝试中继节点
		relays := r.neighbors.GetRelayNodes()
		for _, relay := range relays {
			if r.connector.IsConnected(relay) {
				// 发送中继请求
				relayMsg := &SyncMessage{
					ID:        generateID(),
					Type:      TypeMailRelay,
					Sender:    r.config.NodeID,
					Receiver:  relay,
					Timestamp: time.Now(),
					TTL:       msg.TTL - 1,
					Nonce:     generateNonce(),
					Payload:   data, // 原消息作为载荷
				}
				
				relayData, _ := json.Marshal(relayMsg)
				if err := r.connector.Send(ctx, relay, relayData); err == nil {
					return nil
				}
			}
		}
		
		// 尝试普通邻居转发
		neighbors := r.neighbors.GetNeighbors()
		for _, neighbor := range neighbors {
			if r.connector.IsConnected(neighbor) && neighbor != msg.Receiver {
				relayMsg := &SyncMessage{
					ID:        generateID(),
					Type:      TypeMailRelay,
					Sender:    r.config.NodeID,
					Receiver:  neighbor,
					Timestamp: time.Now(),
					TTL:       msg.TTL - 1,
					Nonce:     generateNonce(),
					Payload:   data,
				}
				
				relayData, _ := json.Marshal(relayMsg)
				if err := r.connector.Send(ctx, neighbor, relayData); err == nil {
					return nil
				}
			}
		}
	}
	
	return ErrDeliveryFailed
}

// HandleMessage 处理收到的消息
func (r *MailRouter) HandleMessage(data []byte) error {
	var msg SyncMessage
	if err := json.Unmarshal(data, &msg); err != nil {
		return ErrInvalidMessage
	}
	
	// 检查消息是否已处理
	if r.cache.has(msg.ID) {
		return nil // 已处理过
	}
	r.cache.add(msg.ID)
	
	// 检查TTL
	if msg.TTL <= 0 {
		return ErrMessageExpired
	}
	
	// 检查发送者声誉
	if r.reputation != nil && r.reputation.IsBlacklisted(msg.Sender) {
		return ErrBlacklisted
	}
	
	// 验证签名
	if r.signer != nil && msg.Signature != "" {
		signData := r.getSignData(&msg)
		sig, _ := hex.DecodeString(msg.Signature)
		valid, err := r.signer.Verify(msg.Sender, signData, sig)
		if err != nil || !valid {
			return ErrInvalidMessage
		}
	}
	
	switch msg.Type {
	case TypeMailSend:
		return r.handleMailSend(&msg)
	case TypeMailRelay:
		return r.handleMailRelay(&msg)
	case TypeMailDelivered:
		return r.handleDeliveryReceipt(&msg)
	case TypeMailRead:
		return r.handleReadReceipt(&msg)
	}
	
	return nil
}

// handleMailSend 处理邮件发送
func (r *MailRouter) handleMailSend(msg *SyncMessage) error {
	// 检查是否是发给自己的
	if msg.Receiver == r.config.NodeID {
		// 解析载荷
		var payload MailPayload
		if err := json.Unmarshal(msg.Payload, &payload); err != nil {
			return err
		}
		
		// 触发接收回调
		if r.onReceive != nil {
			r.onReceive(msg, &payload)
		}
		
		// 发送送达回执
		if r.config.EnableDeliveryReceipt {
			r.sendDeliveryReceipt(msg.Sender, payload.MessageID)
		}
		
		return nil
	}
	
	// 不是发给自己的，尝试转发
	if msg.TTL > 1 {
		msg.TTL--
		return r.doSend(msg)
	}
	
	return nil
}

// handleMailRelay 处理邮件中继
func (r *MailRouter) handleMailRelay(msg *SyncMessage) error {
	// 解析原始消息
	var originalMsg SyncMessage
	if err := json.Unmarshal(msg.Payload, &originalMsg); err != nil {
		return err
	}
	
	// 检查是否是发给自己的
	if originalMsg.Receiver == r.config.NodeID {
		return r.handleMailSend(&originalMsg)
	}
	
	// 继续转发
	if originalMsg.TTL > 1 {
		originalMsg.TTL--
		return r.doSend(&originalMsg)
	}
	
	return nil
}

// handleDeliveryReceipt 处理送达回执
func (r *MailRouter) handleDeliveryReceipt(msg *SyncMessage) error {
	var receipt DeliveryReceipt
	if err := json.Unmarshal(msg.Payload, &receipt); err != nil {
		return err
	}
	
	if r.onDelivered != nil {
		r.onDelivered(&receipt)
	}
	
	return nil
}

// handleReadReceipt 处理已读回执
func (r *MailRouter) handleReadReceipt(msg *SyncMessage) error {
	var receipt ReadReceipt
	if err := json.Unmarshal(msg.Payload, &receipt); err != nil {
		return err
	}
	
	if r.onRead != nil {
		r.onRead(&receipt)
	}
	
	return nil
}

// sendDeliveryReceipt 发送送达回执
func (r *MailRouter) sendDeliveryReceipt(sender, messageID string) {
	receipt := &DeliveryReceipt{
		MessageID:   messageID,
		DeliveredAt: time.Now(),
		ReceiverID:  r.config.NodeID,
	}
	
	payloadBytes, _ := json.Marshal(receipt)
	
	msg := &SyncMessage{
		ID:        generateID(),
		Type:      TypeMailDelivered,
		Sender:    r.config.NodeID,
		Receiver:  sender,
		Timestamp: time.Now(),
		TTL:       r.config.MaxTTL,
		Nonce:     generateNonce(),
		Payload:   payloadBytes,
	}
	
	r.sendMessage(msg)
}

// SendReadReceipt 发送已读回执
func (r *MailRouter) SendReadReceipt(sender, messageID string) error {
	if !r.config.EnableReadReceipt {
		return nil
	}
	
	receipt := &ReadReceipt{
		MessageID: messageID,
		ReadAt:    time.Now(),
		ReaderID:  r.config.NodeID,
	}
	
	payloadBytes, _ := json.Marshal(receipt)
	
	msg := &SyncMessage{
		ID:        generateID(),
		Type:      TypeMailRead,
		Sender:    r.config.NodeID,
		Receiver:  sender,
		Timestamp: time.Now(),
		TTL:       r.config.MaxTTL,
		Nonce:     generateNonce(),
		Payload:   payloadBytes,
	}
	
	return r.sendMessage(msg)
}

// getSignData 获取签名数据
func (r *MailRouter) getSignData(msg *SyncMessage) []byte {
	data := fmt.Sprintf("%s|%s|%s|%s|%d|%s",
		msg.ID, msg.Type, msg.Sender, msg.Receiver,
		msg.Timestamp.UnixNano(), msg.Nonce)
	return []byte(data)
}

// generateID 生成唯一ID
func generateID() string {
	b := make([]byte, 16)
	rand.Read(b)
	hash := sha256.Sum256(append(b, []byte(time.Now().String())...))
	return hex.EncodeToString(hash[:16])
}

// generateNonce 生成随机数
func generateNonce() string {
	b := make([]byte, 16)
	rand.Read(b)
	return hex.EncodeToString(b)
}
