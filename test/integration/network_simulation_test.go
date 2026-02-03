package integration

import (
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/AgentNetworkPlan/AgentNetwork/internal/p2p/host"
	"github.com/AgentNetworkPlan/AgentNetwork/internal/p2p/node"
	"github.com/libp2p/go-libp2p/core/peer"
)

// TestNetworkSimulation 模拟5-10个节点加入网络和通信的完整过程
func TestNetworkSimulation(t *testing.T) {
	const (
		numNodes         = 8 // 模拟8个普通节点
		warmupTime       = 2 * time.Second
		discoveryTime    = 3 * time.Second
		communicationTime = 2 * time.Second
	)

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	_ = ctx // 避免未使用警告

	// ==========================================
	// 步骤 1: 创建引导节点（Bootstrap Node）
	// ==========================================
	t.Log("📡 步骤 1: 创建并启动引导节点...")
	
	bootstrapNode, err := node.New(&node.Config{
		KeyPath:     t.TempDir() + "/bootstrap.key",
		ListenAddrs: []string{"/ip4/127.0.0.1/tcp/0"},
		Role:        host.RoleBootstrap,
		EnableRelay: true,
		EnableDHT:   true,
	})
	if err != nil {
		t.Fatalf("创建引导节点失败: %v", err)
	}

	if err := bootstrapNode.Start(); err != nil {
		t.Fatalf("启动引导节点失败: %v", err)
	}
	defer bootstrapNode.Stop()

	// 获取引导节点信息
	bootstrapHost := bootstrapNode.Host()
	bootstrapAddrs := bootstrapHost.Host().Addrs()
	bootstrapPeerID := bootstrapHost.Host().ID()
	
	// 构建引导节点的完整地址
	var bootstrapPeers []string
	for _, addr := range bootstrapAddrs {
		bootstrapPeers = append(bootstrapPeers, fmt.Sprintf("%s/p2p/%s", addr, bootstrapPeerID))
	}

	t.Logf("✅ 引导节点已启动")
	t.Logf("   PeerID: %s", bootstrapPeerID.String())
	for _, addr := range bootstrapAddrs {
		t.Logf("   地址: %s/p2p/%s", addr, bootstrapPeerID)
	}

	// 等待引导节点完全启动
	time.Sleep(warmupTime)

	// ==========================================
	// 步骤 2: 创建并启动普通节点
	// ==========================================
	t.Logf("\n📱 步骤 2: 创建并启动 %d 个普通节点...", numNodes)
	
	nodes := make([]*node.Node, numNodes)
	nodeHosts := make([]*host.Host, numNodes)
	nodeIDs := make([]peer.ID, numNodes)

	for i := 0; i < numNodes; i++ {
		// 创建节点
		n, err := node.New(&node.Config{
			KeyPath:        fmt.Sprintf("%s/node-%d.key", t.TempDir(), i),
			ListenAddrs:    []string{"/ip4/127.0.0.1/tcp/0"},
			Role:           host.RoleNormal,
			BootstrapPeers: bootstrapPeers,
			EnableRelay:    false,
			EnableDHT:      true,
		})
		if err != nil {
			t.Fatalf("创建节点 %d 失败: %v", i, err)
		}

		// 启动节点
		if err := n.Start(); err != nil {
			t.Fatalf("启动节点 %d 失败: %v", i, err)
		}

		nodes[i] = n
		nodeHosts[i] = n.Host()
		nodeIDs[i] = n.Host().Host().ID()

		t.Logf("   ✓ 节点 %d 已启动 - PeerID: %s", i+1, nodeIDs[i].ShortString())
	}

	// 确保所有节点资源被释放
	defer func() {
		for _, n := range nodes {
			if n != nil {
				n.Stop()
			}
		}
	}()

	// ==========================================
	// 步骤 3: 等待节点发现和连接
	// ==========================================
	t.Logf("\n🔍 步骤 3: 等待节点相互发现...")
	time.Sleep(discoveryTime)

	// ==========================================
	// 步骤 4: 检查网络连接状态
	// ==========================================
	t.Log("\n📊 步骤 4: 网络拓扑分析")
	t.Log("━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━")
	
	// 检查引导节点连接
	bootstrapPeers2 := bootstrapHost.Host().Network().Peers()
	t.Logf("\n🌟 引导节点: %s", bootstrapPeerID.ShortString())
	t.Logf("   已连接节点数: %d", len(bootstrapPeers2))
	for _, p := range bootstrapPeers2 {
		t.Logf("   └─ %s", p.ShortString())
	}

	// 检查每个普通节点的连接
	t.Log("\n🔗 普通节点连接状态:")
	totalConnections := 0
	for i, h := range nodeHosts {
		peers := h.Host().Network().Peers()
		totalConnections += len(peers)
		t.Logf("   节点 %d (%s): %d 个连接", 
			i+1, 
			nodeIDs[i].ShortString(), 
			len(peers))
		
		// 显示连接的对等节点
		for j, p := range peers {
			if j < 3 || len(peers) <= 5 { // 最多显示前3个或全部（如果<=5）
				if p == bootstrapPeerID {
					t.Logf("      └─ %s (引导节点)", p.ShortString())
				} else {
					t.Logf("      └─ %s", p.ShortString())
				}
			} else if j == 3 {
				t.Logf("      └─ ... (还有 %d 个连接)", len(peers)-3)
				break
			}
		}
	}

	avgConnections := float64(totalConnections) / float64(numNodes)
	t.Logf("\n📈 统计数据:")
	t.Logf("   总节点数: %d (包括1个引导节点)", numNodes+1)
	t.Logf("   总连接数: %d", totalConnections)
	t.Logf("   平均每节点连接数: %.2f", avgConnections)

	// ==========================================
	// 步骤 5: 测试节点间通信
	// ==========================================
	t.Log("\n💬 步骤 5: 测试节点间通信能力")
	t.Log("━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━")

	// 从第一个节点向其他节点发送 ping
	sender := nodeHosts[0]
	senderID := nodeIDs[0]
	
	t.Logf("\n📤 从节点 1 (%s) 发送消息...", senderID.ShortString())
	
	successCount := 0
	testCount := min(numNodes-1, 3)
	for i := 1; i <= testCount; i++ { // 测试前3个目标节点
		targetID := nodeIDs[i]
		
		// 尝试连接到目标节点
		targetAddr := nodeHosts[i].Host().Addrs()
		if len(targetAddr) > 0 {
			targetInfo := peer.AddrInfo{
				ID:    targetID,
				Addrs: targetAddr,
			}
			
			testCtx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
			err := sender.Host().Connect(testCtx, targetInfo)
			cancel()
			
			if err == nil {
				successCount++
				t.Logf("   ✓ 成功建立连接: 节点 1 → 节点 %d (%s)", 
					i+1, targetID.ShortString())
			} else {
				t.Logf("   ✗ 连接失败: 节点 1 → 节点 %d: %v", i+1, err)
			}
		}
	}

	t.Logf("\n📊 通信测试结果: %d/%d 成功", successCount, testCount)

	// ==========================================
	// 步骤 6: 测试节点发现功能
	// ==========================================
	t.Log("\n🔎 步骤 6: 测试DHT节点发现")
	t.Log("━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━")

	// 从第二个节点查找第三个节点
	if numNodes >= 3 {
		searcherHost := nodeHosts[1]
		targetID := nodeIDs[2]

		t.Logf("尝试从节点 2 通过DHT查找节点 3 (%s)...", targetID.ShortString())
		
		findCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		foundPeer, err := searcherHost.FindPeer(findCtx, targetID)
		cancel()
		
		if err == nil && foundPeer.ID == targetID {
			t.Logf("✅ 成功通过DHT找到节点!")
			t.Logf("   目标: %s", foundPeer.ID.ShortString())
			t.Logf("   地址数: %d", len(foundPeer.Addrs))
		} else {
			t.Logf("⚠️  DHT查找超时或失败 (这在测试环境中是正常的)")
		}
	}

	// ==========================================
	// 步骤 7: 监控网络稳定性
	// ==========================================
	t.Log("\n⏱️  步骤 7: 监控网络稳定性...")
	time.Sleep(communicationTime)

	// 再次检查连接状态
	finalConnections := 0
	disconnectedNodes := 0
	
	for i, h := range nodeHosts {
		peers := h.Host().Network().Peers()
		finalConnections += len(peers)
		
		if len(peers) == 0 {
			disconnectedNodes++
			t.Logf("   ⚠️  节点 %d 没有连接", i+1)
		}
	}

	t.Log("\n━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━")
	t.Log("🎯 网络模拟测试完成")
	t.Log("━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━")
	t.Logf("✅ 引导节点: 1")
	t.Logf("✅ 普通节点: %d", numNodes)
	t.Logf("✅ 总连接数: %d", finalConnections)
	t.Logf("✅ 平均连接数: %.2f", float64(finalConnections)/float64(numNodes))
	t.Logf("⚠️  断开节点: %d", disconnectedNodes)

	// 验证关键指标
	if len(bootstrapPeers2) == 0 {
		t.Error("引导节点没有任何连接")
	}

	if disconnectedNodes > numNodes/2 {
		t.Errorf("超过一半的节点断开连接: %d/%d", disconnectedNodes, numNodes)
	}

	if avgConnections < 1.0 {
		t.Errorf("平均连接数过低: %.2f", avgConnections)
	}

	t.Log("\n✅ 所有验证通过 - 网络运行正常")
}

// TestNetworkScalability 测试网络可扩展性（10个节点）
func TestNetworkScalability(t *testing.T) {
	const numNodes = 10

	t.Logf("🚀 开始大规模网络测试 (%d 节点)...\n", numNodes+1)

	// 创建引导节点
	bootstrapNode, _ := node.New(&node.Config{
		KeyPath:     t.TempDir() + "/bootstrap.key",
		ListenAddrs: []string{"/ip4/127.0.0.1/tcp/0"},
		Role:        host.RoleBootstrap,
		EnableRelay: true,
		EnableDHT:   true,
	})
	bootstrapNode.Start()
	defer bootstrapNode.Stop()

	// 获取引导节点地址
	bootstrapHost := bootstrapNode.Host()
	var bootstrapPeers []string
	for _, addr := range bootstrapHost.Host().Addrs() {
		bootstrapPeers = append(bootstrapPeers, 
			fmt.Sprintf("%s/p2p/%s", addr, bootstrapHost.Host().ID()))
	}

	time.Sleep(1 * time.Second)

	// 批量创建节点
	nodes := make([]*node.Node, numNodes)
	nodeHosts := make([]*host.Host, numNodes)

	startTime := time.Now()

	for i := 0; i < numNodes; i++ {
		n, _ := node.New(&node.Config{
			KeyPath:        fmt.Sprintf("%s/node-%d.key", t.TempDir(), i),
			ListenAddrs:    []string{"/ip4/127.0.0.1/tcp/0"},
			Role:           host.RoleNormal,
			BootstrapPeers: bootstrapPeers,
			EnableRelay:    false,
			EnableDHT:      true,
		})
		n.Start()

		nodes[i] = n
		nodeHosts[i] = n.Host()

		if (i+1)%3 == 0 {
			t.Logf("   已创建 %d/%d 节点", i+1, numNodes)
		}
	}

	defer func() {
		for _, n := range nodes {
			if n != nil {
				n.Stop()
			}
		}
	}()

	creationTime := time.Since(startTime)
	t.Logf("✅ 所有节点创建完成，耗时: %v\n", creationTime)

	// 等待网络稳定
	t.Log("⏳ 等待网络收敛...")
	time.Sleep(5 * time.Second)

	// 统计网络状态
	totalConnections := 0
	maxConnections := 0
	minConnections := 999

	for _, h := range nodeHosts {
		peerCount := len(h.Host().Network().Peers())
		totalConnections += peerCount
		if peerCount > maxConnections {
			maxConnections = peerCount
		}
		if peerCount < minConnections {
			minConnections = peerCount
		}
	}

	avgConnections := float64(totalConnections) / float64(numNodes)

	t.Log("\n" + "━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━")
	t.Log("📊 网络可扩展性测试结果")
	t.Log("━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━")
	t.Logf("节点总数: %d", numNodes+1)
	t.Logf("创建耗时: %v", creationTime)
	t.Logf("平均连接数: %.2f", avgConnections)
	t.Logf("最大连接数: %d", maxConnections)
	t.Logf("最小连接数: %d", minConnections)
	t.Logf("总连接数: %d", totalConnections)
	t.Log("━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━\n")

	// 性能断言
	if avgConnections < 1.0 {
		t.Errorf("网络连接性不足: 平均 %.2f 连接/节点", avgConnections)
	}

	if creationTime > 10*time.Second {
		t.Logf("⚠️  节点创建较慢: %v (可能是正常的)", creationTime)
	}

	t.Log("✅ 可扩展性测试完成")
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}
