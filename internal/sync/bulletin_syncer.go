// Package sync - 留言板同步器
// 实现跨节点留言板同步，支持Gossip广播和拉取同步
package sync

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"sync"
	"time"
)

var (
	ErrSyncTimeout    = errors.New("sync timeout")
	ErrNoResponse     = errors.New("no response from peer")
	ErrTopicNotFound  = errors.New("topic not found")
)

// BulletinStore 留言板存储接口
type BulletinStore interface {
	// GetMessage 获取消息
	GetMessage(messageID string) (*BulletinPayload, error)
	// GetMessagesByTopic 获取话题消息
	GetMessagesByTopic(topic string, since time.Time, limit int) ([]*BulletinPayload, error)
	// GetAllTopics 获取所有话题
	GetAllTopics() []string
	// StoreMessage 存储消息
	StoreMessage(msg *BulletinPayload) error
	// HasMessage 检查消息是否存在
	HasMessage(messageID string) bool
	// GetLatestTimestamp 获取最新消息时间
	GetLatestTimestamp(topic string) time.Time
}

// SyncerConfig 同步器配置
type SyncerConfig struct {
	NodeID           string
	SyncInterval     time.Duration // 定期同步间隔
	GossipTTL        int           // Gossip消息TTL
	MaxSyncBatch     int           // 单次同步最大消息数
	RequestTimeout   time.Duration // 请求超时
	EnableGossip     bool          // 启用Gossip广播
	EnablePullSync   bool          // 启用拉取同步
}

// DefaultSyncerConfig 默认同步器配置
func DefaultSyncerConfig(nodeID string) *SyncerConfig {
	return &SyncerConfig{
		NodeID:         nodeID,
		SyncInterval:   5 * time.Minute,
		GossipTTL:      5,
		MaxSyncBatch:   100,
		RequestTimeout: 30 * time.Second,
		EnableGossip:   true,
		EnablePullSync: true,
	}
}

// BulletinSyncer 留言板同步器
type BulletinSyncer struct {
	config *SyncerConfig
	
	store       BulletinStore
	connector   PeerConnector
	signer      MessageSigner
	neighbors   NeighborProvider
	reputation  ReputationChecker
	
	// 消息缓存（防止重复处理）
	messageCache map[string]time.Time
	
	// 待处理的同步响应
	pendingResponses map[string]chan *BulletinSyncResponse
	
	// 订阅的话题
	subscriptions map[string]bool
	
	// 回调
	onMessageReceived func(*BulletinPayload)
	onSyncComplete    func(topic string, count int)
	
	mu     sync.RWMutex
	ctx    context.Context
	cancel context.CancelFunc
	wg     sync.WaitGroup
}

// NewBulletinSyncer 创建留言板同步器
func NewBulletinSyncer(config *SyncerConfig) *BulletinSyncer {
	if config == nil {
		config = DefaultSyncerConfig("")
	}
	
	ctx, cancel := context.WithCancel(context.Background())
	
	return &BulletinSyncer{
		config:           config,
		messageCache:     make(map[string]time.Time),
		pendingResponses: make(map[string]chan *BulletinSyncResponse),
		subscriptions:    make(map[string]bool),
		ctx:              ctx,
		cancel:           cancel,
	}
}

// SetStore 设置存储
func (s *BulletinSyncer) SetStore(store BulletinStore) {
	s.store = store
}

// SetPeerConnector 设置节点连接器
func (s *BulletinSyncer) SetPeerConnector(c PeerConnector) {
	s.connector = c
}

// SetSigner 设置签名器
func (s *BulletinSyncer) SetSigner(signer MessageSigner) {
	s.signer = signer
}

// SetNeighborProvider 设置邻居提供者
func (s *BulletinSyncer) SetNeighborProvider(n NeighborProvider) {
	s.neighbors = n
}

// SetReputationChecker 设置声誉检查器
func (s *BulletinSyncer) SetReputationChecker(rc ReputationChecker) {
	s.reputation = rc
}

// SetOnMessageReceived 设置消息接收回调
func (s *BulletinSyncer) SetOnMessageReceived(fn func(*BulletinPayload)) {
	s.onMessageReceived = fn
}

// SetOnSyncComplete 设置同步完成回调
func (s *BulletinSyncer) SetOnSyncComplete(fn func(string, int)) {
	s.onSyncComplete = fn
}

// Start 启动同步器
func (s *BulletinSyncer) Start() {
	// 启动定期同步
	if s.config.EnablePullSync {
		s.wg.Add(1)
		go s.syncLoop()
	}
	
	// 启动缓存清理
	s.wg.Add(1)
	go s.cleanupLoop()
}

// Stop 停止同步器
func (s *BulletinSyncer) Stop() {
	s.cancel()
	s.wg.Wait()
}

// syncLoop 定期同步循环
func (s *BulletinSyncer) syncLoop() {
	defer s.wg.Done()
	ticker := time.NewTicker(s.config.SyncInterval)
	defer ticker.Stop()
	
	for {
		select {
		case <-s.ctx.Done():
			return
		case <-ticker.C:
			s.syncAllTopics()
		}
	}
}

// cleanupLoop 缓存清理循环
func (s *BulletinSyncer) cleanupLoop() {
	defer s.wg.Done()
	ticker := time.NewTicker(10 * time.Minute)
	defer ticker.Stop()
	
	for {
		select {
		case <-s.ctx.Done():
			return
		case <-ticker.C:
			s.cleanupCache()
		}
	}
}

// cleanupCache 清理缓存
func (s *BulletinSyncer) cleanupCache() {
	s.mu.Lock()
	now := time.Now()
	for id, t := range s.messageCache {
		if now.Sub(t) > 24*time.Hour {
			delete(s.messageCache, id)
		}
	}
	s.mu.Unlock()
}

// Subscribe 订阅话题
func (s *BulletinSyncer) Subscribe(topic string) {
	s.mu.Lock()
	s.subscriptions[topic] = true
	s.mu.Unlock()
	
	// 立即同步该话题
	go s.SyncTopic(topic)
}

// Unsubscribe 取消订阅
func (s *BulletinSyncer) Unsubscribe(topic string) {
	s.mu.Lock()
	delete(s.subscriptions, topic)
	s.mu.Unlock()
}

// syncAllTopics 同步所有订阅的话题
func (s *BulletinSyncer) syncAllTopics() {
	s.mu.RLock()
	topics := make([]string, 0, len(s.subscriptions))
	for topic := range s.subscriptions {
		topics = append(topics, topic)
	}
	s.mu.RUnlock()
	
	for _, topic := range topics {
		s.SyncTopic(topic)
	}
}

// SyncTopic 同步指定话题
func (s *BulletinSyncer) SyncTopic(topic string) error {
	if s.connector == nil || s.neighbors == nil {
		return errors.New("connector or neighbors not set")
	}
	
	// 获取最新时间戳
	var sinceTime time.Time
	if s.store != nil {
		sinceTime = s.store.GetLatestTimestamp(topic)
	}
	
	// 创建同步请求
	request := &BulletinSyncRequest{
		Topics:    []string{topic},
		SinceTime: sinceTime,
		Limit:     s.config.MaxSyncBatch,
	}
	
	// 向邻居请求同步
	neighbors := s.neighbors.GetNeighbors()
	for _, neighbor := range neighbors {
		if !s.connector.IsConnected(neighbor) {
			continue
		}
		
		response, err := s.requestSync(neighbor, request)
		if err != nil {
			continue
		}
		
		// 处理响应
		count := 0
		for _, msg := range response.Messages {
			if s.processMessage(&msg) {
				count++
			}
		}
		
		if s.onSyncComplete != nil {
			s.onSyncComplete(topic, count)
		}
		
		// 成功同步一个邻居后跳出
		if count > 0 {
			break
		}
	}
	
	return nil
}

// requestSync 请求同步
func (s *BulletinSyncer) requestSync(peerID string, request *BulletinSyncRequest) (*BulletinSyncResponse, error) {
	payloadBytes, err := json.Marshal(request)
	if err != nil {
		return nil, err
	}
	
	msg := &SyncMessage{
		ID:        generateID(),
		Type:      TypeBulletinSync,
		Sender:    s.config.NodeID,
		Receiver:  peerID,
		Timestamp: time.Now(),
		TTL:       1,
		Nonce:     generateNonce(),
		Payload:   payloadBytes,
	}
	
	// 创建响应通道
	responseCh := make(chan *BulletinSyncResponse, 1)
	s.mu.Lock()
	s.pendingResponses[msg.ID] = responseCh
	s.mu.Unlock()
	
	defer func() {
		s.mu.Lock()
		delete(s.pendingResponses, msg.ID)
		s.mu.Unlock()
	}()
	
	// 发送请求
	data, _ := json.Marshal(msg)
	ctx, cancel := context.WithTimeout(s.ctx, s.config.RequestTimeout)
	defer cancel()
	
	if err := s.connector.Send(ctx, peerID, data); err != nil {
		return nil, err
	}
	
	// 等待响应
	select {
	case <-ctx.Done():
		return nil, ErrSyncTimeout
	case response := <-responseCh:
		return response, nil
	}
}

// PublishMessage 发布消息（广播到网络）
func (s *BulletinSyncer) PublishMessage(msg *BulletinPayload) error {
	if s.connector == nil {
		return errors.New("connector not set")
	}
	
	// 存储本地
	if s.store != nil {
		s.store.StoreMessage(msg)
	}
	
	// 标记已处理
	s.mu.Lock()
	s.messageCache[msg.MessageID] = time.Now()
	s.mu.Unlock()
	
	// Gossip广播
	if s.config.EnableGossip {
		s.gossipMessage(msg)
	}
	
	return nil
}

// gossipMessage Gossip广播消息
func (s *BulletinSyncer) gossipMessage(msg *BulletinPayload) {
	if s.neighbors == nil {
		return
	}
	
	payloadBytes, _ := json.Marshal(msg)
	
	syncMsg := &SyncMessage{
		ID:        generateID(),
		Type:      TypeBulletinPublish,
		Sender:    s.config.NodeID,
		Timestamp: time.Now(),
		TTL:       s.config.GossipTTL,
		Nonce:     generateNonce(),
		Payload:   payloadBytes,
	}
	
	// 签名
	if s.signer != nil {
		signData := fmt.Sprintf("%s|%s|%s|%d",
			syncMsg.ID, syncMsg.Sender, string(syncMsg.Type), syncMsg.Timestamp.UnixNano())
		sig, _ := s.signer.Sign([]byte(signData))
		syncMsg.Signature = string(sig)
	}
	
	data, _ := json.Marshal(syncMsg)
	
	// 发送给所有邻居
	ctx, cancel := context.WithTimeout(s.ctx, 10*time.Second)
	defer cancel()
	
	neighbors := s.neighbors.GetNeighbors()
	for _, neighbor := range neighbors {
		if s.connector.IsConnected(neighbor) {
			go s.connector.Send(ctx, neighbor, data)
		}
	}
}

// HandleMessage 处理收到的消息
func (s *BulletinSyncer) HandleMessage(data []byte) error {
	var msg SyncMessage
	if err := json.Unmarshal(data, &msg); err != nil {
		return err
	}
	
	// 检查是否已处理
	s.mu.RLock()
	_, exists := s.messageCache[msg.ID]
	s.mu.RUnlock()
	if exists {
		return nil
	}
	
	// 标记已处理
	s.mu.Lock()
	s.messageCache[msg.ID] = time.Now()
	s.mu.Unlock()
	
	switch msg.Type {
	case TypeBulletinPublish:
		return s.handlePublish(&msg)
	case TypeBulletinSync:
		return s.handleSyncRequest(&msg)
	case TypeBulletinResp:
		return s.handleSyncResponse(&msg)
	}
	
	return nil
}

// handlePublish 处理发布消息
func (s *BulletinSyncer) handlePublish(msg *SyncMessage) error {
	var payload BulletinPayload
	if err := json.Unmarshal(msg.Payload, &payload); err != nil {
		return err
	}
	
	// 处理消息
	if s.processMessage(&payload) {
		// 继续广播
		if msg.TTL > 1 && s.config.EnableGossip {
			msg.TTL--
			s.forwardMessage(msg)
		}
	}
	
	return nil
}

// processMessage 处理单条消息
func (s *BulletinSyncer) processMessage(msg *BulletinPayload) bool {
	// 检查是否已存在
	if s.store != nil && s.store.HasMessage(msg.MessageID) {
		return false
	}
	
	// 检查是否过期
	if time.Now().After(msg.ExpiresAt) {
		return false
	}
	
	// 检查发送者声誉
	if s.reputation != nil && s.reputation.IsBlacklisted(msg.Author) {
		return false
	}
	
	// 存储消息
	if s.store != nil {
		s.store.StoreMessage(msg)
	}
	
	// 触发回调
	if s.onMessageReceived != nil {
		s.onMessageReceived(msg)
	}
	
	return true
}

// forwardMessage 转发消息
func (s *BulletinSyncer) forwardMessage(msg *SyncMessage) {
	if s.neighbors == nil {
		return
	}
	
	data, _ := json.Marshal(msg)
	ctx, cancel := context.WithTimeout(s.ctx, 10*time.Second)
	defer cancel()
	
	neighbors := s.neighbors.GetNeighbors()
	for _, neighbor := range neighbors {
		if neighbor != msg.Sender && s.connector.IsConnected(neighbor) {
			go s.connector.Send(ctx, neighbor, data)
		}
	}
}

// handleSyncRequest 处理同步请求
func (s *BulletinSyncer) handleSyncRequest(msg *SyncMessage) error {
	var request BulletinSyncRequest
	if err := json.Unmarshal(msg.Payload, &request); err != nil {
		return err
	}
	
	// 准备响应
	response := &BulletinSyncResponse{
		Messages: make([]BulletinPayload, 0),
	}
	
	if s.store != nil {
		for _, topic := range request.Topics {
			messages, err := s.store.GetMessagesByTopic(topic, request.SinceTime, request.Limit)
			if err != nil {
				continue
			}
			for _, m := range messages {
				response.Messages = append(response.Messages, *m)
			}
		}
	}
	
	response.HasMore = len(response.Messages) >= request.Limit
	
	// 发送响应
	payloadBytes, _ := json.Marshal(response)
	respMsg := &SyncMessage{
		ID:        msg.ID, // 使用相同ID关联
		Type:      TypeBulletinResp,
		Sender:    s.config.NodeID,
		Receiver:  msg.Sender,
		Timestamp: time.Now(),
		TTL:       1,
		Payload:   payloadBytes,
	}
	
	data, _ := json.Marshal(respMsg)
	ctx, cancel := context.WithTimeout(s.ctx, 10*time.Second)
	defer cancel()
	
	return s.connector.Send(ctx, msg.Sender, data)
}

// handleSyncResponse 处理同步响应
func (s *BulletinSyncer) handleSyncResponse(msg *SyncMessage) error {
	var response BulletinSyncResponse
	if err := json.Unmarshal(msg.Payload, &response); err != nil {
		return err
	}
	
	// 查找等待的响应通道
	s.mu.RLock()
	ch, ok := s.pendingResponses[msg.ID]
	s.mu.RUnlock()
	
	if ok {
		select {
		case ch <- &response:
		default:
		}
	}
	
	return nil
}
