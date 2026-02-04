// Package sync - 邻居自动发现
// 实现节点自动发现和连接功能
package sync

import (
	"context"
	"encoding/json"
	"errors"
	"sync"
	"time"
)

var (
	ErrDiscoveryDisabled = errors.New("discovery is disabled")
	ErrMaxPeersReached   = errors.New("max peers reached")
	ErrAlreadyConnected  = errors.New("peer already connected")
)

// DiscoveredPeer 发现的节点
type DiscoveredPeer struct {
	Info       PeerInfo  `json:"info"`
	DiscoveredAt time.Time `json:"discovered_at"`
	LastContact  time.Time `json:"last_contact"`
	HopCount     int       `json:"hop_count"`      // 发现时的跳数
	Source       string    `json:"source"`         // 发现来源节点
	Score        float64   `json:"score"`          // 综合评分
}

// DiscoveryConfig 发现配置
type DiscoveryConfig struct {
	NodeID             string
	EnableDiscovery    bool          // 启用发现
	AnnounceInterval   time.Duration // 广播间隔
	QueryInterval      time.Duration // 查询间隔
	MaxHops            int           // 最大跳数
	MaxPeers           int           // 最大节点数
	MinReputation      float64       // 最低声誉要求
	ConnectionTimeout  time.Duration // 连接超时
	PeerCacheExpiry    time.Duration // 节点缓存过期时间
}

// DefaultDiscoveryConfig 默认发现配置
func DefaultDiscoveryConfig(nodeID string) *DiscoveryConfig {
	return &DiscoveryConfig{
		NodeID:            nodeID,
		EnableDiscovery:   true,
		AnnounceInterval:  5 * time.Minute,
		QueryInterval:     10 * time.Minute,
		MaxHops:           3,
		MaxPeers:          50,
		MinReputation:     10.0,
		ConnectionTimeout: 30 * time.Second,
		PeerCacheExpiry:   1 * time.Hour,
	}
}

// AutoDiscovery 自动发现服务
type AutoDiscovery struct {
	config *DiscoveryConfig
	
	connector   PeerConnector
	signer      MessageSigner
	neighbors   NeighborProvider
	reputation  ReputationChecker
	
	// 发现的节点缓存
	discovered map[string]*DiscoveredPeer
	
	// 已连接节点
	connected map[string]bool
	
	// 本节点信息
	selfInfo *PeerInfo
	
	// 回调
	onPeerDiscovered func(*DiscoveredPeer)
	onPeerConnected  func(string)
	onPeerLost       func(string)
	
	mu     sync.RWMutex
	ctx    context.Context
	cancel context.CancelFunc
	wg     sync.WaitGroup
}

// NewAutoDiscovery 创建自动发现服务
func NewAutoDiscovery(config *DiscoveryConfig) *AutoDiscovery {
	if config == nil {
		config = DefaultDiscoveryConfig("")
	}
	
	ctx, cancel := context.WithCancel(context.Background())
	
	return &AutoDiscovery{
		config:     config,
		discovered: make(map[string]*DiscoveredPeer),
		connected:  make(map[string]bool),
		ctx:        ctx,
		cancel:     cancel,
	}
}

// SetPeerConnector 设置节点连接器
func (d *AutoDiscovery) SetPeerConnector(c PeerConnector) {
	d.connector = c
}

// SetSigner 设置签名器
func (d *AutoDiscovery) SetSigner(s MessageSigner) {
	d.signer = s
}

// SetNeighborProvider 设置邻居提供者
func (d *AutoDiscovery) SetNeighborProvider(n NeighborProvider) {
	d.neighbors = n
}

// SetReputationChecker 设置声誉检查器
func (d *AutoDiscovery) SetReputationChecker(rc ReputationChecker) {
	d.reputation = rc
}

// SetSelfInfo 设置本节点信息
func (d *AutoDiscovery) SetSelfInfo(info *PeerInfo) {
	d.selfInfo = info
}

// SetOnPeerDiscovered 设置节点发现回调
func (d *AutoDiscovery) SetOnPeerDiscovered(fn func(*DiscoveredPeer)) {
	d.onPeerDiscovered = fn
}

// SetOnPeerConnected 设置节点连接回调
func (d *AutoDiscovery) SetOnPeerConnected(fn func(string)) {
	d.onPeerConnected = fn
}

// SetOnPeerLost 设置节点丢失回调
func (d *AutoDiscovery) SetOnPeerLost(fn func(string)) {
	d.onPeerLost = fn
}

// Start 启动自动发现
func (d *AutoDiscovery) Start() error {
	if !d.config.EnableDiscovery {
		return ErrDiscoveryDisabled
	}
	
	// 启动广播循环
	d.wg.Add(1)
	go d.announceLoop()
	
	// 启动查询循环
	d.wg.Add(1)
	go d.queryLoop()
	
	// 启动清理循环
	d.wg.Add(1)
	go d.cleanupLoop()
	
	// 启动自动连接循环
	d.wg.Add(1)
	go d.autoConnectLoop()
	
	return nil
}

// Stop 停止自动发现
func (d *AutoDiscovery) Stop() {
	d.cancel()
	d.wg.Wait()
}

// announceLoop 广播循环
func (d *AutoDiscovery) announceLoop() {
	defer d.wg.Done()
	
	// 立即广播一次
	d.announce()
	
	ticker := time.NewTicker(d.config.AnnounceInterval)
	defer ticker.Stop()
	
	for {
		select {
		case <-d.ctx.Done():
			return
		case <-ticker.C:
			d.announce()
		}
	}
}

// announce 广播自己的存在
func (d *AutoDiscovery) announce() {
	if d.selfInfo == nil || d.neighbors == nil {
		return
	}
	
	// 创建广播消息
	announcement := &PeerAnnouncement{
		Info:      *d.selfInfo,
		Neighbors: d.neighbors.GetNeighbors(),
		Services:  []string{"mail", "bulletin", "discovery"},
	}
	
	payloadBytes, _ := json.Marshal(announcement)
	
	msg := &SyncMessage{
		ID:        generateID(),
		Type:      TypePeerAnnounce,
		Sender:    d.config.NodeID,
		Timestamp: time.Now(),
		TTL:       d.config.MaxHops,
		Nonce:     generateNonce(),
		Payload:   payloadBytes,
	}
	
	// 签名
	if d.signer != nil {
		signData := d.getSignData(msg)
		sig, _ := d.signer.Sign(signData)
		msg.Signature = string(sig)
	}
	
	// 发送给所有邻居
	d.broadcastToNeighbors(msg)
}

// queryLoop 查询循环
func (d *AutoDiscovery) queryLoop() {
	defer d.wg.Done()
	
	ticker := time.NewTicker(d.config.QueryInterval)
	defer ticker.Stop()
	
	for {
		select {
		case <-d.ctx.Done():
			return
		case <-ticker.C:
			d.queryPeers()
		}
	}
}

// queryPeers 查询更多节点
func (d *AutoDiscovery) queryPeers() {
	if d.neighbors == nil {
		return
	}
	
	query := &PeerQuery{
		QueryID:   generateID(),
		MaxHops:   d.config.MaxHops,
		RequestBy: d.config.NodeID,
	}
	
	payloadBytes, _ := json.Marshal(query)
	
	msg := &SyncMessage{
		ID:        generateID(),
		Type:      TypePeerQuery,
		Sender:    d.config.NodeID,
		Timestamp: time.Now(),
		TTL:       d.config.MaxHops,
		Nonce:     generateNonce(),
		Payload:   payloadBytes,
	}
	
	d.broadcastToNeighbors(msg)
}

// cleanupLoop 清理循环
func (d *AutoDiscovery) cleanupLoop() {
	defer d.wg.Done()
	
	ticker := time.NewTicker(10 * time.Minute)
	defer ticker.Stop()
	
	for {
		select {
		case <-d.ctx.Done():
			return
		case <-ticker.C:
			d.cleanup()
		}
	}
}

// cleanup 清理过期节点
func (d *AutoDiscovery) cleanup() {
	d.mu.Lock()
	defer d.mu.Unlock()
	
	now := time.Now()
	for id, peer := range d.discovered {
		if now.Sub(peer.LastContact) > d.config.PeerCacheExpiry {
			delete(d.discovered, id)
		}
	}
}

// autoConnectLoop 自动连接循环
func (d *AutoDiscovery) autoConnectLoop() {
	defer d.wg.Done()
	
	ticker := time.NewTicker(1 * time.Minute)
	defer ticker.Stop()
	
	for {
		select {
		case <-d.ctx.Done():
			return
		case <-ticker.C:
			d.autoConnect()
		}
	}
}

// autoConnect 自动连接高质量节点
func (d *AutoDiscovery) autoConnect() {
	d.mu.RLock()
	// 获取当前连接数
	connectedCount := len(d.connected)
	
	// 获取候选节点
	candidates := make([]*DiscoveredPeer, 0)
	for _, peer := range d.discovered {
		if !d.connected[peer.Info.NodeID] {
			candidates = append(candidates, peer)
		}
	}
	d.mu.RUnlock()
	
	if connectedCount >= d.config.MaxPeers {
		return
	}
	
	// 按评分排序
	for i := range candidates {
		for j := i + 1; j < len(candidates); j++ {
			if candidates[i].Score < candidates[j].Score {
				candidates[i], candidates[j] = candidates[j], candidates[i]
			}
		}
	}
	
	// 尝试连接
	needed := d.config.MaxPeers - connectedCount
	for i := 0; i < needed && i < len(candidates); i++ {
		peer := candidates[i]
		go d.tryConnect(peer)
	}
}

// tryConnect 尝试连接节点
func (d *AutoDiscovery) tryConnect(peer *DiscoveredPeer) {
	if d.connector == nil {
		return
	}
	
	// 检查声誉
	if d.reputation != nil {
		rep := d.reputation.GetReputation(peer.Info.NodeID)
		if rep < d.config.MinReputation {
			return
		}
	}
	
	ctx, cancel := context.WithTimeout(d.ctx, d.config.ConnectionTimeout)
	defer cancel()
	
	if err := d.connector.Connect(ctx, peer.Info.NodeID); err != nil {
		return
	}
	
	d.mu.Lock()
	d.connected[peer.Info.NodeID] = true
	d.mu.Unlock()
	
	if d.onPeerConnected != nil {
		d.onPeerConnected(peer.Info.NodeID)
	}
}

// HandleMessage 处理收到的发现消息
func (d *AutoDiscovery) HandleMessage(data []byte) error {
	var msg SyncMessage
	if err := json.Unmarshal(data, &msg); err != nil {
		return err
	}
	
	// 忽略自己发的消息
	if msg.Sender == d.config.NodeID {
		return nil
	}
	
	switch msg.Type {
	case TypePeerAnnounce:
		return d.handleAnnounce(&msg)
	case TypePeerQuery:
		return d.handleQuery(&msg)
	case TypePeerResponse:
		return d.handleResponse(&msg)
	}
	
	return nil
}

// handleAnnounce 处理节点广播
func (d *AutoDiscovery) handleAnnounce(msg *SyncMessage) error {
	var announcement PeerAnnouncement
	if err := json.Unmarshal(msg.Payload, &announcement); err != nil {
		return err
	}
	
	// 验证声誉
	if d.reputation != nil && d.reputation.IsBlacklisted(announcement.Info.NodeID) {
		return nil
	}
	
	// 添加到发现列表
	d.addDiscoveredPeer(&announcement.Info, msg.Sender, d.config.MaxHops-msg.TTL+1)
	
	// 继续转发
	if msg.TTL > 1 {
		msg.TTL--
		d.broadcastToNeighbors(msg)
	}
	
	return nil
}

// handleQuery 处理节点查询
func (d *AutoDiscovery) handleQuery(msg *SyncMessage) error {
	var query PeerQuery
	if err := json.Unmarshal(msg.Payload, &query); err != nil {
		return err
	}
	
	// 准备响应
	d.mu.RLock()
	peers := make([]PeerInfo, 0)
	for _, peer := range d.discovered {
		if peer.Info.NodeID != query.RequestBy {
			peers = append(peers, peer.Info)
		}
	}
	// 添加自己
	if d.selfInfo != nil {
		peers = append(peers, *d.selfInfo)
	}
	d.mu.RUnlock()
	
	response := &PeerQueryResponse{
		QueryID: query.QueryID,
		Peers:   peers,
		HopPath: []string{d.config.NodeID},
	}
	
	payloadBytes, _ := json.Marshal(response)
	
	respMsg := &SyncMessage{
		ID:        generateID(),
		Type:      TypePeerResponse,
		Sender:    d.config.NodeID,
		Receiver:  msg.Sender,
		Timestamp: time.Now(),
		TTL:       1,
		Payload:   payloadBytes,
	}
	
	// 发送响应
	data, _ := json.Marshal(respMsg)
	ctx, cancel := context.WithTimeout(d.ctx, 10*time.Second)
	defer cancel()
	
	if d.connector != nil && d.connector.IsConnected(msg.Sender) {
		d.connector.Send(ctx, msg.Sender, data)
	}
	
	// 转发查询
	if msg.TTL > 1 {
		msg.TTL--
		d.broadcastToNeighbors(msg)
	}
	
	return nil
}

// handleResponse 处理查询响应
func (d *AutoDiscovery) handleResponse(msg *SyncMessage) error {
	var response PeerQueryResponse
	if err := json.Unmarshal(msg.Payload, &response); err != nil {
		return err
	}
	
	// 添加发现的节点
	for _, peer := range response.Peers {
		if peer.NodeID != d.config.NodeID {
			d.addDiscoveredPeer(&peer, msg.Sender, len(response.HopPath))
		}
	}
	
	return nil
}

// addDiscoveredPeer 添加发现的节点
func (d *AutoDiscovery) addDiscoveredPeer(info *PeerInfo, source string, hopCount int) {
	d.mu.Lock()
	defer d.mu.Unlock()
	
	existing, ok := d.discovered[info.NodeID]
	if ok {
		// 更新信息
		existing.LastContact = time.Now()
		if hopCount < existing.HopCount {
			existing.HopCount = hopCount
			existing.Source = source
		}
		return
	}
	
	// 计算评分
	score := d.calculateScore(info, hopCount)
	
	peer := &DiscoveredPeer{
		Info:         *info,
		DiscoveredAt: time.Now(),
		LastContact:  time.Now(),
		HopCount:     hopCount,
		Source:       source,
		Score:        score,
	}
	
	d.discovered[info.NodeID] = peer
	
	// 触发回调
	if d.onPeerDiscovered != nil {
		go d.onPeerDiscovered(peer)
	}
}

// calculateScore 计算节点评分
func (d *AutoDiscovery) calculateScore(info *PeerInfo, hopCount int) float64 {
	score := 50.0 // 基础分
	
	// 声誉加分
	score += info.Reputation * 0.3
	
	// 跳数减分
	score -= float64(hopCount) * 5
	
	// 最近在线加分
	if time.Since(info.LastSeen) < 5*time.Minute {
		score += 10
	}
	
	return score
}

// broadcastToNeighbors 广播给邻居
func (d *AutoDiscovery) broadcastToNeighbors(msg *SyncMessage) {
	if d.neighbors == nil || d.connector == nil {
		return
	}
	
	data, _ := json.Marshal(msg)
	ctx, cancel := context.WithTimeout(d.ctx, 10*time.Second)
	defer cancel()
	
	neighbors := d.neighbors.GetNeighbors()
	for _, neighbor := range neighbors {
		if neighbor != msg.Sender && d.connector.IsConnected(neighbor) {
			go d.connector.Send(ctx, neighbor, data)
		}
	}
}

// getSignData 获取签名数据
func (d *AutoDiscovery) getSignData(msg *SyncMessage) []byte {
	return []byte(msg.ID + msg.Sender + string(msg.Type))
}

// GetDiscoveredPeers 获取发现的节点列表
func (d *AutoDiscovery) GetDiscoveredPeers() []*DiscoveredPeer {
	d.mu.RLock()
	defer d.mu.RUnlock()
	
	peers := make([]*DiscoveredPeer, 0, len(d.discovered))
	for _, peer := range d.discovered {
		peers = append(peers, peer)
	}
	return peers
}

// GetConnectedPeers 获取已连接节点
func (d *AutoDiscovery) GetConnectedPeers() []string {
	d.mu.RLock()
	defer d.mu.RUnlock()
	
	peers := make([]string, 0, len(d.connected))
	for id := range d.connected {
		peers = append(peers, id)
	}
	return peers
}

// MarkDisconnected 标记节点断开
func (d *AutoDiscovery) MarkDisconnected(nodeID string) {
	d.mu.Lock()
	delete(d.connected, nodeID)
	d.mu.Unlock()
	
	if d.onPeerLost != nil {
		d.onPeerLost(nodeID)
	}
}

// GetStats 获取统计信息
func (d *AutoDiscovery) GetStats() map[string]int {
	d.mu.RLock()
	defer d.mu.RUnlock()
	
	return map[string]int{
		"discovered": len(d.discovered),
		"connected":  len(d.connected),
	}
}
