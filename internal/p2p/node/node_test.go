package node

import (
	"testing"
	"time"

	"github.com/AgentNetworkPlan/AgentNetwork/internal/p2p/host"
)

func TestNew(t *testing.T) {
	tmpDir := t.TempDir()

	cfg := &Config{
		KeyPath: tmpDir + "/test.key",
		ListenAddrs: []string{
			"/ip4/127.0.0.1/tcp/0",
		},
		Role:        host.RoleNormal,
		EnableRelay: false,
		EnableDHT:   true,
	}

	n, err := New(cfg)
	if err != nil {
		t.Fatalf("创建节点失败: %v", err)
	}
	defer n.Stop()

	if n.ID() == "" {
		t.Error("节点 ID 为空")
	}

	if n.Identity() == nil {
		t.Error("节点身份为空")
	}

	t.Logf("节点 ID: %s", n.ShortID())
}

func TestNode_Start(t *testing.T) {
	tmpDir := t.TempDir()

	cfg := &Config{
		KeyPath: tmpDir + "/test.key",
		ListenAddrs: []string{
			"/ip4/127.0.0.1/tcp/0",
		},
		Role:        host.RoleNormal,
		EnableRelay: false,
		EnableDHT:   true,
	}

	n, err := New(cfg)
	if err != nil {
		t.Fatalf("创建节点失败: %v", err)
	}
	defer n.Stop()

	if err := n.Start(); err != nil {
		t.Fatalf("启动节点失败: %v", err)
	}

	// 等待服务启动
	time.Sleep(200 * time.Millisecond)

	if n.Host() == nil {
		t.Error("P2P 主机为空")
	}

	if n.Discovery() == nil {
		t.Error("发现服务为空")
	}
}

func TestNode_TwoNodes_Discovery(t *testing.T) {
	tmpDir := t.TempDir()

	// 创建 Bootstrap 节点
	cfg1 := &Config{
		KeyPath: tmpDir + "/bootstrap.key",
		ListenAddrs: []string{
			"/ip4/127.0.0.1/tcp/0",
		},
		Role:        host.RoleBootstrap,
		EnableRelay: true,
		EnableDHT:   true,
	}

	n1, err := New(cfg1)
	if err != nil {
		t.Fatalf("创建 Bootstrap 节点失败: %v", err)
	}
	defer n1.Stop()

	if err := n1.Start(); err != nil {
		t.Fatalf("启动 Bootstrap 节点失败: %v", err)
	}

	// 获取 Bootstrap 地址
	addrs := n1.Host().Addrs()
	if len(addrs) == 0 {
		t.Fatal("Bootstrap 节点没有地址")
	}

	bootstrapAddr := addrs[0].String() + "/p2p/" + n1.ID()
	t.Logf("Bootstrap 地址: %s", bootstrapAddr)

	// 创建普通节点
	cfg2 := &Config{
		KeyPath: tmpDir + "/normal.key",
		ListenAddrs: []string{
			"/ip4/127.0.0.1/tcp/0",
		},
		BootstrapPeers: []string{bootstrapAddr},
		Role:           host.RoleNormal,
		EnableRelay:    true,
		EnableDHT:      true,
	}

	n2, err := New(cfg2)
	if err != nil {
		t.Fatalf("创建普通节点失败: %v", err)
	}
	defer n2.Stop()

	if err := n2.Start(); err != nil {
		t.Fatalf("启动普通节点失败: %v", err)
	}

	// 等待连接建立
	time.Sleep(2 * time.Second)

	// 检查连接
	n1Peers := n1.Host().ConnectedPeers()
	n2Peers := n2.Host().ConnectedPeers()

	t.Logf("Bootstrap 节点连接数: %d", n1Peers)
	t.Logf("普通节点连接数: %d", n2Peers)

	if n1Peers == 0 || n2Peers == 0 {
		t.Error("节点未能建立连接")
	}
}

func TestNode_MultipleNodes(t *testing.T) {
	if testing.Short() {
		t.Skip("跳过长时间测试")
	}

	tmpDir := t.TempDir()
	nodeCount := 5

	// 创建 Bootstrap 节点
	bootstrapCfg := &Config{
		KeyPath: tmpDir + "/bootstrap.key",
		ListenAddrs: []string{
			"/ip4/127.0.0.1/tcp/0",
		},
		Role:        host.RoleBootstrap,
		EnableRelay: true,
		EnableDHT:   true,
	}

	bootstrap, err := New(bootstrapCfg)
	if err != nil {
		t.Fatalf("创建 Bootstrap 节点失败: %v", err)
	}
	defer bootstrap.Stop()

	if err := bootstrap.Start(); err != nil {
		t.Fatalf("启动 Bootstrap 节点失败: %v", err)
	}

	addrs := bootstrap.Host().Addrs()
	bootstrapAddr := addrs[0].String() + "/p2p/" + bootstrap.ID()

	// 创建多个普通节点
	nodes := make([]*Node, nodeCount)
	for i := 0; i < nodeCount; i++ {
		cfg := &Config{
			KeyPath: tmpDir + "/node" + string(rune('0'+i)) + ".key",
			ListenAddrs: []string{
				"/ip4/127.0.0.1/tcp/0",
			},
			BootstrapPeers: []string{bootstrapAddr},
			Role:           host.RoleNormal,
			EnableRelay:    true,
			EnableDHT:      true,
		}

		n, err := New(cfg)
		if err != nil {
			t.Fatalf("创建节点 %d 失败: %v", i, err)
		}
		nodes[i] = n
		defer n.Stop()

		if err := n.Start(); err != nil {
			t.Fatalf("启动节点 %d 失败: %v", i, err)
		}
	}

	// 等待网络稳定
	time.Sleep(3 * time.Second)

	// 检查连接
	t.Logf("Bootstrap 节点连接数: %d", bootstrap.Host().ConnectedPeers())

	for i, n := range nodes {
		peers := n.Host().ConnectedPeers()
		t.Logf("节点 %d 连接数: %d", i, peers)
		if peers == 0 {
			t.Errorf("节点 %d 没有连接", i)
		}
	}
}
