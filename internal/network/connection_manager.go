package network

import (
	"context"
	"encoding/json"
	"fmt"
	"sync"
	"time"

	"github.com/libp2p/go-libp2p/core/host"
	"github.com/libp2p/go-libp2p/core/network"
	"github.com/libp2p/go-libp2p/core/peer"
	"github.com/multiformats/go-multiaddr"
)

// ConnectionState 连接状态
type ConnectionState int

const (
	StateDisconnected ConnectionState = iota
	StateConnecting
	StateConnected
	StateUnhealthy
)

func (s ConnectionState) String() string {
	switch s {
	case StateDisconnected:
		return "disconnected"
	case StateConnecting:
		return "connecting"
	case StateConnected:
		return "connected"
	case StateUnhealthy:
		return "unhealthy"
	default:
		return "unknown"
	}
}

// PeerInfo 节点信息
type PeerInfo struct {
	ID            peer.ID              `json:"id"`
	Addrs         []multiaddr.Multiaddr `json:"-"`
	State         ConnectionState      `json:"state"`
	LastSeen      time.Time            `json:"last_seen"`
	LastPing      time.Time            `json:"last_ping"`
	PingLatency   time.Duration        `json:"ping_latency"`
	FailCount     int                  `json:"fail_count"`
	ReconnectAt   time.Time            `json:"reconnect_at"`
	ConnectedAt   time.Time            `json:"connected_at"`
	DisconnectedAt time.Time           `json:"disconnected_at"`
}

// ConnectionManagerConfig 连接管理器配置
type ConnectionManagerConfig struct {
	// 健康检查间隔
	HealthCheckInterval time.Duration
	// Ping 超时
	PingTimeout time.Duration
	// 最大失败次数（超过后标记为不健康）
	MaxFailCount int
	// 重连基础间隔
	ReconnectBaseInterval time.Duration
	// 最大重连间隔
	MaxReconnectInterval time.Duration
	// 重连退避因子
	ReconnectBackoffFactor float64
	// 不健康阈值（Ping 延迟超过此值）
	UnhealthyLatencyThreshold time.Duration
	// 最大并发重连数
	MaxConcurrentReconnects int
}

// DefaultConnectionManagerConfig 默认配置
func DefaultConnectionManagerConfig() *ConnectionManagerConfig {
	return &ConnectionManagerConfig{
		HealthCheckInterval:       30 * time.Second,
		PingTimeout:              5 * time.Second,
		MaxFailCount:             3,
		ReconnectBaseInterval:    5 * time.Second,
		MaxReconnectInterval:     5 * time.Minute,
		ReconnectBackoffFactor:   2.0,
		UnhealthyLatencyThreshold: 3 * time.Second,
		MaxConcurrentReconnects:  5,
	}
}

// ConnectionManager 连接管理器
type ConnectionManager struct {
	host   host.Host
	config *ConnectionManagerConfig

	peers    map[peer.ID]*PeerInfo
	mu       sync.RWMutex

	ctx    context.Context
	cancel context.CancelFunc

	// 事件通道
	eventChan chan ConnectionEvent

	// 重连信号量
	reconnectSem chan struct{}
}

// ConnectionEvent 连接事件
type ConnectionEvent struct {
	Type      string    `json:"type"`
	PeerID    peer.ID   `json:"peer_id"`
	Timestamp time.Time `json:"timestamp"`
	Details   string    `json:"details,omitempty"`
}

// NewConnectionManager 创建连接管理器
func NewConnectionManager(h host.Host, cfg *ConnectionManagerConfig) *ConnectionManager {
	if cfg == nil {
		cfg = DefaultConnectionManagerConfig()
	}

	ctx, cancel := context.WithCancel(context.Background())

	cm := &ConnectionManager{
		host:         h,
		config:       cfg,
		peers:        make(map[peer.ID]*PeerInfo),
		ctx:          ctx,
		cancel:       cancel,
		eventChan:    make(chan ConnectionEvent, 100),
		reconnectSem: make(chan struct{}, cfg.MaxConcurrentReconnects),
	}

	// 注册连接通知
	h.Network().Notify(&network.NotifyBundle{
		ConnectedF:    cm.onConnected,
		DisconnectedF: cm.onDisconnected,
	})

	return cm
}

// Start 启动连接管理器
func (cm *ConnectionManager) Start() {
	go cm.healthCheckLoop()
	go cm.reconnectLoop()
}

// Stop 停止连接管理器
func (cm *ConnectionManager) Stop() {
	cm.cancel()
	// 不关闭 eventChan，避免 panic
}

// onConnected 连接建立回调
func (cm *ConnectionManager) onConnected(n network.Network, c network.Conn) {
	peerID := c.RemotePeer()

	cm.mu.Lock()
	info, exists := cm.peers[peerID]
	if !exists {
		info = &PeerInfo{
			ID:    peerID,
			Addrs: []multiaddr.Multiaddr{c.RemoteMultiaddr()},
		}
		cm.peers[peerID] = info
	}

	info.State = StateConnected
	info.LastSeen = time.Now()
	info.ConnectedAt = time.Now()
	info.FailCount = 0
	if c.RemoteMultiaddr() != nil {
		// 添加新地址（如果不存在）
		addrExists := false
		for _, addr := range info.Addrs {
			if addr.Equal(c.RemoteMultiaddr()) {
				addrExists = true
				break
			}
		}
		if !addrExists {
			info.Addrs = append(info.Addrs, c.RemoteMultiaddr())
		}
	}
	cm.mu.Unlock()

	cm.emitEvent("connected", peerID, "")
}

// onDisconnected 连接断开回调
func (cm *ConnectionManager) onDisconnected(n network.Network, c network.Conn) {
	peerID := c.RemotePeer()

	cm.mu.Lock()
	info, exists := cm.peers[peerID]
	if exists {
		info.State = StateDisconnected
		info.DisconnectedAt = time.Now()
		info.FailCount++
		// 计算下次重连时间（指数退避）
		backoff := cm.calculateBackoff(info.FailCount)
		info.ReconnectAt = time.Now().Add(backoff)
	}
	cm.mu.Unlock()

	cm.emitEvent("disconnected", peerID, "")
}

// calculateBackoff 计算退避时间
func (cm *ConnectionManager) calculateBackoff(failCount int) time.Duration {
	backoff := cm.config.ReconnectBaseInterval
	for i := 1; i < failCount; i++ {
		backoff = time.Duration(float64(backoff) * cm.config.ReconnectBackoffFactor)
		if backoff > cm.config.MaxReconnectInterval {
			backoff = cm.config.MaxReconnectInterval
			break
		}
	}
	return backoff
}

// healthCheckLoop 健康检查循环
func (cm *ConnectionManager) healthCheckLoop() {
	ticker := time.NewTicker(cm.config.HealthCheckInterval)
	defer ticker.Stop()

	for {
		select {
		case <-cm.ctx.Done():
			return
		case <-ticker.C:
			cm.performHealthCheck()
		}
	}
}

// performHealthCheck 执行健康检查
func (cm *ConnectionManager) performHealthCheck() {
	cm.mu.RLock()
	connectedPeers := make([]*PeerInfo, 0)
	for _, info := range cm.peers {
		if info.State == StateConnected {
			infoCopy := *info
			connectedPeers = append(connectedPeers, &infoCopy)
		}
	}
	cm.mu.RUnlock()

	for _, info := range connectedPeers {
		go cm.pingPeer(info.ID)
	}
}

// pingPeer Ping 指定节点
func (cm *ConnectionManager) pingPeer(peerID peer.ID) {
	ctx, cancel := context.WithTimeout(cm.ctx, cm.config.PingTimeout)
	defer cancel()

	start := time.Now()

	// 检查连接状态
	if cm.host.Network().Connectedness(peerID) != network.Connected {
		cm.handlePingFailure(peerID, "not connected")
		return
	}

	// 尝试打开一个流来检测连接健康
	stream, err := cm.host.NewStream(ctx, peerID, "/ping/1.0.0")
	if err != nil {
		cm.handlePingFailure(peerID, err.Error())
		return
	}
	stream.Close()

	latency := time.Since(start)

	cm.mu.Lock()
	info, exists := cm.peers[peerID]
	if exists {
		info.LastPing = time.Now()
		info.LastSeen = time.Now()
		info.PingLatency = latency
		info.FailCount = 0

		if latency > cm.config.UnhealthyLatencyThreshold {
			info.State = StateUnhealthy
			cm.mu.Unlock()
			cm.emitEvent("unhealthy", peerID, fmt.Sprintf("high latency: %v", latency))
			return
		}

		info.State = StateConnected
	}
	cm.mu.Unlock()
}

// handlePingFailure 处理 Ping 失败
func (cm *ConnectionManager) handlePingFailure(peerID peer.ID, reason string) {
	cm.mu.Lock()
	info, exists := cm.peers[peerID]
	if exists {
		info.FailCount++
		if info.FailCount >= cm.config.MaxFailCount {
			info.State = StateUnhealthy
			cm.mu.Unlock()
			cm.emitEvent("unhealthy", peerID, reason)
			return
		}
	}
	cm.mu.Unlock()
}

// reconnectLoop 重连循环
func (cm *ConnectionManager) reconnectLoop() {
	ticker := time.NewTicker(5 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-cm.ctx.Done():
			return
		case <-ticker.C:
			cm.performReconnects()
		}
	}
}

// performReconnects 执行重连
func (cm *ConnectionManager) performReconnects() {
	cm.mu.RLock()
	needReconnect := make([]*PeerInfo, 0)
	now := time.Now()

	for _, info := range cm.peers {
		if info.State == StateDisconnected && now.After(info.ReconnectAt) {
			infoCopy := *info
			needReconnect = append(needReconnect, &infoCopy)
		}
	}
	cm.mu.RUnlock()

	for _, info := range needReconnect {
		select {
		case cm.reconnectSem <- struct{}{}:
			go func(p *PeerInfo) {
				defer func() { <-cm.reconnectSem }()
				cm.reconnectPeer(p)
			}(info)
		default:
			// 已达到最大并发重连数
		}
	}
}

// reconnectPeer 重连指定节点
func (cm *ConnectionManager) reconnectPeer(info *PeerInfo) {
	cm.mu.Lock()
	peerInfo, exists := cm.peers[info.ID]
	if !exists {
		cm.mu.Unlock()
		return
	}
	peerInfo.State = StateConnecting
	addrs := make([]multiaddr.Multiaddr, len(peerInfo.Addrs))
	copy(addrs, peerInfo.Addrs)
	cm.mu.Unlock()

	cm.emitEvent("reconnecting", info.ID, "")

	ctx, cancel := context.WithTimeout(cm.ctx, 10*time.Second)
	defer cancel()

	addrInfo := peer.AddrInfo{
		ID:    info.ID,
		Addrs: addrs,
	}

	if err := cm.host.Connect(ctx, addrInfo); err != nil {
		cm.mu.Lock()
		if p, ok := cm.peers[info.ID]; ok {
			p.State = StateDisconnected
			p.FailCount++
			p.ReconnectAt = time.Now().Add(cm.calculateBackoff(p.FailCount))
		}
		cm.mu.Unlock()
		cm.emitEvent("reconnect_failed", info.ID, err.Error())
	}
	// 成功连接会触发 onConnected 回调
}

// AddPeer 添加节点（用于主动添加需要维护的节点）
func (cm *ConnectionManager) AddPeer(addrInfo peer.AddrInfo) {
	cm.mu.Lock()
	defer cm.mu.Unlock()

	if _, exists := cm.peers[addrInfo.ID]; !exists {
		cm.peers[addrInfo.ID] = &PeerInfo{
			ID:          addrInfo.ID,
			Addrs:       addrInfo.Addrs,
			State:       StateDisconnected,
			ReconnectAt: time.Now(),
		}
	}
}

// RemovePeer 移除节点
func (cm *ConnectionManager) RemovePeer(peerID peer.ID) {
	cm.mu.Lock()
	delete(cm.peers, peerID)
	cm.mu.Unlock()
}

// GetPeerInfo 获取节点信息
func (cm *ConnectionManager) GetPeerInfo(peerID peer.ID) (*PeerInfo, bool) {
	cm.mu.RLock()
	defer cm.mu.RUnlock()

	info, exists := cm.peers[peerID]
	if !exists {
		return nil, false
	}

	// 返回副本
	infoCopy := *info
	return &infoCopy, true
}

// GetAllPeers 获取所有节点信息
func (cm *ConnectionManager) GetAllPeers() []*PeerInfo {
	cm.mu.RLock()
	defer cm.mu.RUnlock()

	result := make([]*PeerInfo, 0, len(cm.peers))
	for _, info := range cm.peers {
		infoCopy := *info
		result = append(result, &infoCopy)
	}
	return result
}

// GetConnectedPeers 获取已连接的节点
func (cm *ConnectionManager) GetConnectedPeers() []*PeerInfo {
	cm.mu.RLock()
	defer cm.mu.RUnlock()

	result := make([]*PeerInfo, 0)
	for _, info := range cm.peers {
		if info.State == StateConnected {
			infoCopy := *info
			result = append(result, &infoCopy)
		}
	}
	return result
}

// GetUnhealthyPeers 获取不健康的节点
func (cm *ConnectionManager) GetUnhealthyPeers() []*PeerInfo {
	cm.mu.RLock()
	defer cm.mu.RUnlock()

	result := make([]*PeerInfo, 0)
	for _, info := range cm.peers {
		if info.State == StateUnhealthy {
			infoCopy := *info
			result = append(result, &infoCopy)
		}
	}
	return result
}

// Events 返回事件通道
func (cm *ConnectionManager) Events() <-chan ConnectionEvent {
	return cm.eventChan
}

// emitEvent 发送事件
func (cm *ConnectionManager) emitEvent(eventType string, peerID peer.ID, details string) {
	event := ConnectionEvent{
		Type:      eventType,
		PeerID:    peerID,
		Timestamp: time.Now(),
		Details:   details,
	}

	select {
	case <-cm.ctx.Done():
		// 已停止，忽略事件
		return
	default:
	}

	select {
	case cm.eventChan <- event:
	default:
		// 通道满，丢弃事件
	}
}

// Stats 连接统计
type ConnectionStats struct {
	TotalPeers       int           `json:"total_peers"`
	ConnectedPeers   int           `json:"connected_peers"`
	DisconnectedPeers int          `json:"disconnected_peers"`
	UnhealthyPeers   int           `json:"unhealthy_peers"`
	AverageLatency   time.Duration `json:"average_latency"`
}

// GetStats 获取统计信息
func (cm *ConnectionManager) GetStats() *ConnectionStats {
	cm.mu.RLock()
	defer cm.mu.RUnlock()

	stats := &ConnectionStats{
		TotalPeers: len(cm.peers),
	}

	var totalLatency time.Duration
	var latencyCount int

	for _, info := range cm.peers {
		switch info.State {
		case StateConnected:
			stats.ConnectedPeers++
		case StateDisconnected:
			stats.DisconnectedPeers++
		case StateUnhealthy:
			stats.UnhealthyPeers++
		}

		if info.PingLatency > 0 {
			totalLatency += info.PingLatency
			latencyCount++
		}
	}

	if latencyCount > 0 {
		stats.AverageLatency = totalLatency / time.Duration(latencyCount)
	}

	return stats
}

// MarshalJSON 自定义 JSON 序列化
func (p *PeerInfo) MarshalJSON() ([]byte, error) {
	type Alias PeerInfo
	return json.Marshal(&struct {
		*Alias
		ID      string   `json:"id"`
		Addrs   []string `json:"addrs"`
		State   string   `json:"state"`
	}{
		Alias: (*Alias)(p),
		ID:    p.ID.String(),
		Addrs: func() []string {
			addrs := make([]string, len(p.Addrs))
			for i, a := range p.Addrs {
				addrs[i] = a.String()
			}
			return addrs
		}(),
		State: p.State.String(),
	})
}
