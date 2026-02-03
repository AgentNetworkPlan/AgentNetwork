package integration

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"testing"
	"time"

	"github.com/AgentNetworkPlan/AgentNetwork/internal/httpapi"
	"github.com/AgentNetworkPlan/AgentNetwork/internal/p2p/host"
	"github.com/AgentNetworkPlan/AgentNetwork/internal/p2p/node"
	"github.com/libp2p/go-libp2p/core/peer"
)

// TestEnhancedNetworkBehaviors 增强版网络协作测试 - 覆盖更多节点行为和API接口
func TestEnhancedNetworkBehaviors(t *testing.T) {
	const (
		numNodes      = 6 // 6个普通节点
		warmupTime    = 2 * time.Second
		discoveryTime = 3 * time.Second
	)

	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()

	_ = ctx

	t.Log("━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━")
	t.Log("🚀 增强版网络协作测试 - 多行为场景覆盖")
	t.Log("━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━\n")

	// ==========================================
	// 步骤 1: 创建引导节点和HTTP API服务器
	// ==========================================
	t.Log("📡 步骤 1: 创建引导节点与HTTP API...")
	
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

	// 启动引导节点的HTTP API
	bootstrapHTTP, err := httpapi.NewServer(httpapi.DefaultConfig(bootstrapNode.Host().Host().ID().String()))
	if err != nil {
		t.Fatalf("创建HTTP API失败: %v", err)
	}
	go bootstrapHTTP.Start()
	defer bootstrapHTTP.Stop()
	time.Sleep(500 * time.Millisecond)

	bootstrapHost := bootstrapNode.Host()
	bootstrapPeerID := bootstrapHost.Host().ID()
	
	var bootstrapPeers []string
	for _, addr := range bootstrapHost.Host().Addrs() {
		bootstrapPeers = append(bootstrapPeers, fmt.Sprintf("%s/p2p/%s", addr, bootstrapPeerID))
	}

	t.Logf("✅ 引导节点已启动 - PeerID: %s", bootstrapPeerID.ShortString())
	
	time.Sleep(warmupTime)

	// ==========================================
	// 步骤 2: 创建普通节点和HTTP API服务器
	// ==========================================
	t.Logf("\n📱 步骤 2: 创建 %d 个普通节点与HTTP API服务器...", numNodes)
	
	nodes := make([]*node.Node, numNodes)
	nodeHosts := make([]*host.Host, numNodes)
	nodeIDs := make([]peer.ID, numNodes)
	httpAPIs := make([]*httpapi.Server, numNodes)
	httpPorts := make([]int, numNodes)

	for i := 0; i < numNodes; i++ {
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

		if err := n.Start(); err != nil {
			t.Fatalf("启动节点 %d 失败: %v", i, err)
		}

		nodes[i] = n
		nodeHosts[i] = n.Host()
		nodeIDs[i] = n.Host().Host().ID()

		// 启动HTTP API服务器
		httpPort := 18100 + i
		httpCfg := httpapi.DefaultConfig(nodeIDs[i].String())
		httpCfg.ListenAddr = fmt.Sprintf(":%d", httpPort)
		httpAPI, err := httpapi.NewServer(httpCfg)
		if err != nil {
			t.Fatalf("创建节点 %d 的HTTP API失败: %v", i, err)
		}
		go httpAPI.Start()
		
		httpAPIs[i] = httpAPI
		httpPorts[i] = httpPort

		t.Logf("   ✓ 节点 %d 已启动 - PeerID: %s, HTTP: :%d", 
			i+1, nodeIDs[i].ShortString(), httpPort)
	}

	defer func() {
		for i, n := range nodes {
			if n != nil {
				n.Stop()
			}
			if httpAPIs[i] != nil {
				httpAPIs[i].Stop()
			}
		}
	}()

	time.Sleep(discoveryTime)

	// ==========================================
	// 步骤 3: 测试 HTTP API - 节点信息接口
	// ==========================================
	t.Log("\n🔍 步骤 3: 测试节点信息API...")
	t.Log("━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━")

	testCases := []struct {
		name     string
		endpoint string
		method   string
	}{
		{"健康检查", "/health", "GET"},
		{"节点状态", "/status", "GET"},
		{"节点信息", "/api/v1/node/info", "GET"},
		{"对等节点列表", "/api/v1/node/peers", "GET"},
	}

	successCount := 0
	for _, tc := range testCases {
		url := fmt.Sprintf("http://localhost:%d%s", httpPorts[0], tc.endpoint)
		resp, err := http.Get(url)
		if err == nil && resp.StatusCode == http.StatusOK {
			successCount++
			t.Logf("   ✓ %s - %s", tc.name, tc.endpoint)
			resp.Body.Close()
		} else {
			t.Logf("   ✗ %s - 失败", tc.name)
			if resp != nil {
				resp.Body.Close()
			}
		}
	}

	t.Logf("\n   节点信息API测试: %d/%d 通过\n", successCount, len(testCases))

	// ==========================================
	// 步骤 4: 测试邻居管理API
	// ==========================================
	t.Log("🤝 步骤 4: 测试邻居管理API...")
	t.Log("━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━")

	neighborTests := []struct {
		name     string
		endpoint string
		method   string
	}{
		{"邻居列表", "/api/v1/neighbor/list", "GET"},
		{"最佳邻居", "/api/v1/neighbor/best", "GET"},
	}

	neighborSuccess := 0
	for _, tc := range neighborTests {
		url := fmt.Sprintf("http://localhost:%d%s", httpPorts[0], tc.endpoint)
		resp, err := http.Get(url)
		if err == nil && resp.StatusCode == http.StatusOK {
			neighborSuccess++
			t.Logf("   ✓ %s", tc.name)
			resp.Body.Close()
		} else {
			t.Logf("   ✗ %s - 失败", tc.name)
			if resp != nil {
				resp.Body.Close()
			}
		}
	}

	t.Logf("\n   邻居管理API测试: %d/%d 通过\n", neighborSuccess, len(neighborTests))

	// ==========================================
	// 步骤 5: 测试消息传递API
	// ==========================================
	t.Log("💬 步骤 5: 测试消息传递...")
	t.Log("━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━")

	// 节点1向节点2发送消息
	if numNodes >= 2 {
		msgReq := httpapi.MessageRequest{
			To:      nodeIDs[1].String(),
			Type:    "test",
			Content: "Hello from node 1",
			Metadata: map[string]interface{}{
				"timestamp": time.Now().Unix(),
				"test":      true,
			},
		}

		msgBody, _ := json.Marshal(msgReq)
		url := fmt.Sprintf("http://localhost:%d/api/v1/message/send", httpPorts[0])
		resp, err := http.Post(url, "application/json", bytes.NewReader(msgBody))
		
		if err == nil && resp.StatusCode == http.StatusOK {
			t.Log("   ✓ 消息发送成功: 节点 1 → 节点 2")
			resp.Body.Close()
		} else {
			t.Log("   ✗ 消息发送失败")
			if resp != nil {
				resp.Body.Close()
			}
		}
	}

	// ==========================================
	// 步骤 6: 测试邮箱系统API
	// ==========================================
	t.Log("\n📬 步骤 6: 测试邮箱系统...")
	t.Log("━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━")

	mailboxTests := []struct {
		name     string
		endpoint string
	}{
		{"收件箱", "/api/v1/mailbox/inbox"},
		{"发件箱", "/api/v1/mailbox/outbox"},
	}

	mailboxSuccess := 0
	for _, tc := range mailboxTests {
		url := fmt.Sprintf("http://localhost:%d%s", httpPorts[0], tc.endpoint)
		resp, err := http.Get(url)
		if err == nil && resp.StatusCode == http.StatusOK {
			mailboxSuccess++
			t.Logf("   ✓ %s", tc.name)
			resp.Body.Close()
		} else {
			t.Logf("   ✗ %s - 失败", tc.name)
			if resp != nil {
				resp.Body.Close()
			}
		}
	}

	t.Logf("\n   邮箱系统API测试: %d/%d 通过\n", mailboxSuccess, len(mailboxTests))

	// ==========================================
	// 步骤 7: 测试任务系统API
	// ==========================================
	t.Log("📋 步骤 7: 测试任务系统...")
	t.Log("━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━")

	// 创建任务
	taskReq := httpapi.TaskRequest{
		TaskID:      "test-task-001",
		Type:        "computation",
		Description: "Test task from integration test",
		Target:      nodeIDs[1].String(),
		Payload: map[string]interface{}{
			"operation": "sum",
			"values":    []int{1, 2, 3, 4, 5},
		},
	}

	taskBody, _ := json.Marshal(taskReq)
	url := fmt.Sprintf("http://localhost:%d/api/v1/task/create", httpPorts[0])
	resp, err := http.Post(url, "application/json", bytes.NewReader(taskBody))
	
	taskCreated := false
	if err == nil && resp.StatusCode == http.StatusOK {
		t.Log("   ✓ 任务创建成功")
		taskCreated = true
		resp.Body.Close()
	} else {
		t.Log("   ✗ 任务创建失败")
		if resp != nil {
			resp.Body.Close()
		}
	}

	// 查询任务列表
	url = fmt.Sprintf("http://localhost:%d/api/v1/task/list", httpPorts[0])
	resp, err = http.Get(url)
	if err == nil && resp.StatusCode == http.StatusOK {
		t.Log("   ✓ 任务列表查询成功")
		resp.Body.Close()
	} else {
		t.Log("   ✗ 任务列表查询失败")
		if resp != nil {
			resp.Body.Close()
		}
	}

	// ==========================================
	// 步骤 8: 测试信誉系统API
	// ==========================================
	t.Log("\n⭐ 步骤 8: 测试信誉系统...")
	t.Log("━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━")

	reputationTests := []struct {
		name     string
		endpoint string
	}{
		{"信誉查询", "/api/v1/reputation/query"},
		{"信誉排名", "/api/v1/reputation/ranking"},
	}

	reputationSuccess := 0
	for _, tc := range reputationTests {
		url := fmt.Sprintf("http://localhost:%d%s", httpPorts[0], tc.endpoint)
		resp, err := http.Get(url)
		if err == nil && resp.StatusCode == http.StatusOK {
			reputationSuccess++
			t.Logf("   ✓ %s", tc.name)
			resp.Body.Close()
		} else {
			t.Logf("   ✗ %s - 失败", tc.name)
			if resp != nil {
				resp.Body.Close()
			}
		}
	}

	t.Logf("\n   信誉系统API测试: %d/%d 通过\n", reputationSuccess, len(reputationTests))

	// ==========================================
	// 步骤 9: 测试公告板API
	// ==========================================
	t.Log("📢 步骤 9: 测试公告板系统...")
	t.Log("━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━")

	// 发布公告
	bulletinReq := map[string]interface{}{
		"topic":   "test",
		"content": "Test bulletin from integration test",
		"author":  nodeIDs[0].String(),
	}

	bulletinBody, _ := json.Marshal(bulletinReq)
	url = fmt.Sprintf("http://localhost:%d/api/v1/bulletin/publish", httpPorts[0])
	resp, err = http.Post(url, "application/json", bytes.NewReader(bulletinBody))
	
	if err == nil && resp.StatusCode == http.StatusOK {
		t.Log("   ✓ 公告发布成功")
		resp.Body.Close()
	} else {
		t.Log("   ✗ 公告发布失败")
		if resp != nil {
			resp.Body.Close()
		}
	}

	// 搜索公告
	url = fmt.Sprintf("http://localhost:%d/api/v1/bulletin/search?keyword=test", httpPorts[0])
	resp, err = http.Get(url)
	if err == nil && resp.StatusCode == http.StatusOK {
		t.Log("   ✓ 公告搜索成功")
		resp.Body.Close()
	} else {
		t.Log("   ✗ 公告搜索失败")
		if resp != nil {
			resp.Body.Close()
		}
	}

	// ==========================================
	// 步骤 10: 测试投票系统API
	// ==========================================
	t.Log("\n🗳️  步骤 10: 测试投票系统...")
	t.Log("━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━")

	votingTests := []struct {
		name     string
		endpoint string
	}{
		{"提案列表", "/api/v1/voting/proposal/list"},
	}

	votingSuccess := 0
	for _, tc := range votingTests {
		url := fmt.Sprintf("http://localhost:%d%s", httpPorts[0], tc.endpoint)
		resp, err := http.Get(url)
		if err == nil && resp.StatusCode == http.StatusOK {
			votingSuccess++
			t.Logf("   ✓ %s", tc.name)
			resp.Body.Close()
		} else {
			t.Logf("   ✗ %s - 失败", tc.name)
			if resp != nil {
				resp.Body.Close()
			}
		}
	}

	t.Logf("\n   投票系统API测试: %d/%d 通过\n", votingSuccess, len(votingTests))

	// ==========================================
	// 步骤 11: 网络拓扑验证
	// ==========================================
	t.Log("🌐 步骤 11: 网络拓扑验证...")
	t.Log("━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━")

	totalConnections := 0
	for i, h := range nodeHosts {
		peers := h.Host().Network().Peers()
		totalConnections += len(peers)
		t.Logf("   节点 %d: %d 个连接", i+1, len(peers))
	}

	avgConnections := float64(totalConnections) / float64(numNodes)
	t.Logf("\n   平均连接数: %.2f", avgConnections)

	// ==========================================
	// 总结报告
	// ==========================================
	t.Log("\n━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━")
	t.Log("📊 增强版网络测试总结")
	t.Log("━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━")

	totalTests := len(testCases) + len(neighborTests) + len(mailboxTests) + 
		len(reputationTests) + len(votingTests) + 3 // 消息、任务、公告
	passedTests := successCount + neighborSuccess + mailboxSuccess + 
		reputationSuccess + votingSuccess
	if taskCreated {
		passedTests++
	}

	t.Logf("✅ 节点总数: %d", numNodes+1)
	t.Logf("✅ API测试通过: %d/%d (%.1f%%)", 
		passedTests, totalTests, float64(passedTests)/float64(totalTests)*100)
	t.Logf("✅ 网络连接: 平均 %.2f 连接/节点", avgConnections)
	t.Log("━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━\n")

	// 基本验证
	if avgConnections < 1.0 {
		t.Errorf("网络连接性不足: 平均 %.2f 连接/节点", avgConnections)
	}

	if float64(passedTests)/float64(totalTests) < 0.5 {
		t.Errorf("API测试通过率过低: %.1f%%", float64(passedTests)/float64(totalTests)*100)
	}

	t.Log("✅ 增强版网络协作测试完成")
}

// TestAPICompleteness 测试API接口覆盖完整性
func TestAPICompleteness(t *testing.T) {
	t.Log("━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━")
	t.Log("📋 API接口覆盖完整性评估")
	t.Log("━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━\n")

	// 所有已实现的HTTP API接口
	apiEndpoints := map[string][]string{
		"节点管理": {
			"/health",
			"/status",
			"/api/v1/node/info",
			"/api/v1/node/peers",
			"/api/v1/node/register",
		},
		"邻居管理": {
			"/api/v1/neighbor/list",
			"/api/v1/neighbor/best",
			"/api/v1/neighbor/add",
			"/api/v1/neighbor/remove",
			"/api/v1/neighbor/ping",
		},
		"消息传递": {
			"/api/v1/message/send",
			"/api/v1/message/receive",
		},
		"邮箱系统": {
			"/api/v1/mailbox/send",
			"/api/v1/mailbox/inbox",
			"/api/v1/mailbox/outbox",
			"/api/v1/mailbox/read/",
			"/api/v1/mailbox/mark-read",
			"/api/v1/mailbox/delete",
		},
		"公告板": {
			"/api/v1/bulletin/publish",
			"/api/v1/bulletin/message/",
			"/api/v1/bulletin/topic/",
			"/api/v1/bulletin/author/",
			"/api/v1/bulletin/search",
			"/api/v1/bulletin/subscribe",
			"/api/v1/bulletin/unsubscribe",
			"/api/v1/bulletin/revoke",
		},
		"任务系统": {
			"/api/v1/task/create",
			"/api/v1/task/status",
			"/api/v1/task/accept",
			"/api/v1/task/submit",
			"/api/v1/task/list",
		},
		"信誉系统": {
			"/api/v1/reputation/query",
			"/api/v1/reputation/update",
			"/api/v1/reputation/ranking",
			"/api/v1/reputation/history",
		},
		"指控系统": {
			"/api/v1/accusation/create",
			"/api/v1/accusation/list",
			"/api/v1/accusation/detail/",
			"/api/v1/accusation/analyze",
		},
		"激励机制": {
			"/api/v1/incentive/award",
			"/api/v1/incentive/propagate",
			"/api/v1/incentive/history",
			"/api/v1/incentive/tolerance",
		},
		"投票治理": {
			"/api/v1/voting/proposal/create",
			"/api/v1/voting/proposal/list",
			"/api/v1/voting/proposal/",
			"/api/v1/voting/vote",
			"/api/v1/voting/proposal/finalize",
		},
		"超级节点": {
			"/api/v1/supernode/list",
			"/api/v1/supernode/candidates",
			"/api/v1/supernode/apply",
			"/api/v1/supernode/heartbeat",
		},
		"存储系统": {
			"/api/v1/storage/put",
			"/api/v1/storage/get",
			"/api/v1/storage/delete",
			"/api/v1/storage/list",
			"/api/v1/storage/has",
		},
		"日志系统": {
			"/api/v1/log/tail",
			"/api/v1/log/stream",
		},
	}

	// 统计接口数量
	totalAPIs := 0
	for category, endpoints := range apiEndpoints {
		count := len(endpoints)
		totalAPIs += count
		t.Logf("📂 %s: %d 个接口", category, count)
		for i, endpoint := range endpoints {
			if i < 3 || len(endpoints) <= 5 {
				t.Logf("   └─ %s", endpoint)
			} else if i == 3 {
				t.Logf("   └─ ... (还有 %d 个)", len(endpoints)-3)
				break
			}
		}
	}

	t.Logf("\n━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━")
	t.Logf("📊 API接口统计:")
	t.Logf("   总模块数: %d", len(apiEndpoints))
	t.Logf("   总接口数: %d", totalAPIs)
	t.Log("━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━")

	// 标记已覆盖的接口
	testedEndpoints := map[string]bool{
		"/health":                    true,
		"/status":                    true,
		"/api/v1/node/info":          true,
		"/api/v1/node/peers":         true,
		"/api/v1/neighbor/list":      true,
		"/api/v1/neighbor/best":      true,
		"/api/v1/message/send":       true,
		"/api/v1/mailbox/inbox":      true,
		"/api/v1/mailbox/outbox":     true,
		"/api/v1/task/create":        true,
		"/api/v1/task/list":          true,
		"/api/v1/reputation/query":   true,
		"/api/v1/reputation/ranking": true,
		"/api/v1/bulletin/publish":   true,
		"/api/v1/bulletin/search":    true,
		"/api/v1/voting/proposal/list": true,
	}

	testedCount := len(testedEndpoints)
	coverage := float64(testedCount) / float64(totalAPIs) * 100

	t.Logf("\n✅ 已测试接口: %d/%d (%.1f%%)", testedCount, totalAPIs, coverage)
	t.Logf("⚠️  未测试接口: %d (%.1f%%)", totalAPIs-testedCount, 100-coverage)

	t.Log("\n💡 建议:")
	if coverage < 50 {
		t.Log("   ⚠️  测试覆盖率较低，建议增加更多API测试")
	} else if coverage < 80 {
		t.Log("   ⚡ 测试覆盖率中等，可继续改进")
	} else {
		t.Log("   ✨ 测试覆盖率良好")
	}

	t.Log("\n🎯 未覆盖的核心接口:")
	uncoveredCore := []string{
		"/api/v1/storage/put", "/api/v1/storage/get",
		"/api/v1/accusation/create", "/api/v1/accusation/list",
		"/api/v1/voting/vote", "/api/v1/supernode/list",
	}
	for _, endpoint := range uncoveredCore {
		t.Logf("   • %s", endpoint)
	}

	t.Log("\n━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━\n")
}
