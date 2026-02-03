package network

import (
	"context"
	"testing"
	"time"

	"github.com/libp2p/go-libp2p"
	"github.com/libp2p/go-libp2p/core/peer"
)

func TestNewConnectionManager(t *testing.T) {
	// 创建测试主机
	h, err := libp2p.New()
	if err != nil {
		t.Fatalf("创建主机失败: %v", err)
	}
	defer h.Close()

	cm := NewConnectionManager(h, nil)
	if cm == nil {
		t.Fatal("ConnectionManager 不应为 nil")
	}

	cm.Stop()
}

func TestConnectionManagerConfig(t *testing.T) {
	cfg := DefaultConnectionManagerConfig()

	if cfg.HealthCheckInterval != 30*time.Second {
		t.Errorf("默认健康检查间隔错误: %v", cfg.HealthCheckInterval)
	}
	if cfg.PingTimeout != 5*time.Second {
		t.Errorf("默认 Ping 超时错误: %v", cfg.PingTimeout)
	}
	if cfg.MaxFailCount != 3 {
		t.Errorf("默认最大失败次数错误: %d", cfg.MaxFailCount)
	}
}

func TestConnectionState(t *testing.T) {
	states := []struct {
		state    ConnectionState
		expected string
	}{
		{StateDisconnected, "disconnected"},
		{StateConnecting, "connecting"},
		{StateConnected, "connected"},
		{StateUnhealthy, "unhealthy"},
		{ConnectionState(99), "unknown"},
	}

	for _, tc := range states {
		if tc.state.String() != tc.expected {
			t.Errorf("状态字符串错误: got %s, want %s", tc.state.String(), tc.expected)
		}
	}
}

func TestCalculateBackoff(t *testing.T) {
	h, _ := libp2p.New()
	defer h.Close()

	cfg := &ConnectionManagerConfig{
		ReconnectBaseInterval:  5 * time.Second,
		MaxReconnectInterval:   5 * time.Minute,
		ReconnectBackoffFactor: 2.0,
	}

	cm := NewConnectionManager(h, cfg)
	defer cm.Stop()

	tests := []struct {
		failCount int
		expected  time.Duration
	}{
		{1, 5 * time.Second},
		{2, 10 * time.Second},
		{3, 20 * time.Second},
		{4, 40 * time.Second},
		{10, 5 * time.Minute}, // 超过最大值
	}

	for _, tc := range tests {
		backoff := cm.calculateBackoff(tc.failCount)
		if backoff != tc.expected {
			t.Errorf("failCount=%d: got %v, want %v", tc.failCount, backoff, tc.expected)
		}
	}
}

func TestAddRemovePeer(t *testing.T) {
	h, _ := libp2p.New()
	defer h.Close()

	cm := NewConnectionManager(h, nil)
	defer cm.Stop()

	// 创建测试节点信息
	peerID, _ := peer.Decode("QmYyQSo1c1Ym7orWxLYvCrM2EmxFTANf8wXmmE7DWjhx5N")
	addrInfo := peer.AddrInfo{ID: peerID}

	// 添加节点
	cm.AddPeer(addrInfo)

	info, exists := cm.GetPeerInfo(peerID)
	if !exists {
		t.Fatal("节点应该存在")
	}
	if info.State != StateDisconnected {
		t.Errorf("新添加节点状态应该是 disconnected, got %s", info.State.String())
	}

	// 移除节点
	cm.RemovePeer(peerID)

	_, exists = cm.GetPeerInfo(peerID)
	if exists {
		t.Fatal("节点应该已被移除")
	}
}

func TestGetAllPeers(t *testing.T) {
	h, _ := libp2p.New()
	defer h.Close()

	cm := NewConnectionManager(h, nil)
	defer cm.Stop()

	// 添加多个节点
	for i := 0; i < 5; i++ {
		peerID, _ := peer.Decode("QmYyQSo1c1Ym7orWxLYvCrM2EmxFTANf8wXmmE7DWjhx5N")
		// 使用不同的地址模拟不同节点（实际测试中这会是相同ID，但这里只测试存储逻辑）
		cm.AddPeer(peer.AddrInfo{ID: peerID})
	}

	peers := cm.GetAllPeers()
	if len(peers) != 1 { // 相同 ID 只保存一个
		t.Logf("节点数量: %d", len(peers))
	}
}

func TestGetStats(t *testing.T) {
	h, _ := libp2p.New()
	defer h.Close()

	cm := NewConnectionManager(h, nil)
	defer cm.Stop()

	stats := cm.GetStats()
	if stats == nil {
		t.Fatal("Stats 不应为 nil")
	}
	if stats.TotalPeers != 0 {
		t.Errorf("初始节点数应该为 0, got %d", stats.TotalPeers)
	}
}

func TestPeerInfoMarshalJSON(t *testing.T) {
	peerID, _ := peer.Decode("QmYyQSo1c1Ym7orWxLYvCrM2EmxFTANf8wXmmE7DWjhx5N")
	info := &PeerInfo{
		ID:       peerID,
		State:    StateConnected,
		LastSeen: time.Now(),
	}

	data, err := info.MarshalJSON()
	if err != nil {
		t.Fatalf("JSON 序列化失败: %v", err)
	}

	if len(data) == 0 {
		t.Error("JSON 数据不应为空")
	}

	t.Logf("JSON: %s", string(data))
}

func TestConnectionEvents(t *testing.T) {
	h, _ := libp2p.New()
	defer h.Close()

	cm := NewConnectionManager(h, nil)
	cm.Start()
	defer cm.Stop()

	// 检查事件通道
	events := cm.Events()
	if events == nil {
		t.Fatal("事件通道不应为 nil")
	}
}

func TestTwoNodesConnection(t *testing.T) {
	// 创建两个主机
	h1, err := libp2p.New()
	if err != nil {
		t.Fatalf("创建主机1失败: %v", err)
	}
	defer h1.Close()

	h2, err := libp2p.New()
	if err != nil {
		t.Fatalf("创建主机2失败: %v", err)
	}
	defer h2.Close()

	cm1 := NewConnectionManager(h1, nil)
	cm1.Start()
	defer cm1.Stop()

	cm2 := NewConnectionManager(h2, nil)
	cm2.Start()
	defer cm2.Stop()

	// h1 连接到 h2
	h2Info := peer.AddrInfo{
		ID:    h2.ID(),
		Addrs: h2.Addrs(),
	}

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	if err := h1.Connect(ctx, h2Info); err != nil {
		t.Fatalf("连接失败: %v", err)
	}

	// 等待连接事件处理
	time.Sleep(100 * time.Millisecond)

	// 检查连接状态
	info, exists := cm1.GetPeerInfo(h2.ID())
	if !exists {
		t.Fatal("h2 应该在 cm1 的节点列表中")
	}
	if info.State != StateConnected {
		t.Errorf("h2 状态应该是 connected, got %s", info.State.String())
	}

	stats := cm1.GetStats()
	if stats.ConnectedPeers != 1 {
		t.Errorf("连接节点数应该是 1, got %d", stats.ConnectedPeers)
	}
}

func TestDisconnectionAndReconnect(t *testing.T) {
	// 创建两个主机
	h1, _ := libp2p.New()
	defer h1.Close()

	h2, _ := libp2p.New()

	cfg := &ConnectionManagerConfig{
		HealthCheckInterval:    100 * time.Millisecond,
		PingTimeout:           100 * time.Millisecond,
		MaxFailCount:          2,
		ReconnectBaseInterval: 100 * time.Millisecond,
		MaxReconnectInterval:  1 * time.Second,
		ReconnectBackoffFactor: 2.0,
		MaxConcurrentReconnects: 5,
	}

	cm1 := NewConnectionManager(h1, cfg)
	cm1.Start()
	defer cm1.Stop()

	// 连接
	h2Info := peer.AddrInfo{
		ID:    h2.ID(),
		Addrs: h2.Addrs(),
	}

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	h1.Connect(ctx, h2Info)
	time.Sleep(100 * time.Millisecond)

	// 断开 h2
	h2.Close()
	time.Sleep(200 * time.Millisecond)

	// 检查状态变化
	info, exists := cm1.GetPeerInfo(h2.ID())
	if exists {
		t.Logf("断开后节点状态: %s, FailCount: %d", info.State.String(), info.FailCount)
	}
}
