package host

import (
	"context"
	"testing"
	"time"

	"github.com/AgentNetworkPlan/AgentNetwork/internal/p2p/identity"
)

func TestNew(t *testing.T) {
	id, err := identity.NewIdentity()
	if err != nil {
		t.Fatalf("创建身份失败: %v", err)
	}

	cfg := &Config{
		Identity: id,
		ListenAddrs: []string{
			"/ip4/127.0.0.1/tcp/0",
		},
		Role:        RoleNormal,
		EnableRelay: false,
		EnableDHT:   true,
	}

	h, err := New(cfg)
	if err != nil {
		t.Fatalf("创建主机失败: %v", err)
	}
	defer h.Stop()

	if h.ID() == "" {
		t.Error("主机 ID 为空")
	}

	if len(h.Addrs()) == 0 {
		t.Error("主机没有监听地址")
	}

	t.Logf("主机 ID: %s", h.ID())
	t.Logf("监听地址: %v", h.Addrs())
}

func TestHost_Start(t *testing.T) {
	id, err := identity.NewIdentity()
	if err != nil {
		t.Fatalf("创建身份失败: %v", err)
	}

	cfg := &Config{
		Identity: id,
		ListenAddrs: []string{
			"/ip4/127.0.0.1/tcp/0",
		},
		Role:        RoleNormal,
		EnableRelay: false,
		EnableDHT:   true,
	}

	h, err := New(cfg)
	if err != nil {
		t.Fatalf("创建主机失败: %v", err)
	}
	defer h.Stop()

	if err := h.Start(); err != nil {
		t.Fatalf("启动主机失败: %v", err)
	}

	// 等待 DHT 启动
	time.Sleep(100 * time.Millisecond)

	if h.DHT() == nil {
		t.Error("DHT 未启动")
	}
}

func TestHost_TwoNodes_Connect(t *testing.T) {
	// 创建节点 1
	id1, _ := identity.NewIdentity()
	cfg1 := &Config{
		Identity: id1,
		ListenAddrs: []string{
			"/ip4/127.0.0.1/tcp/0",
		},
		Role:        RoleNormal,
		EnableRelay: false,
		EnableDHT:   true,
	}

	h1, err := New(cfg1)
	if err != nil {
		t.Fatalf("创建主机1失败: %v", err)
	}
	defer h1.Stop()

	if err := h1.Start(); err != nil {
		t.Fatalf("启动主机1失败: %v", err)
	}

	// 创建节点 2
	id2, _ := identity.NewIdentity()
	cfg2 := &Config{
		Identity: id2,
		ListenAddrs: []string{
			"/ip4/127.0.0.1/tcp/0",
		},
		Role:        RoleNormal,
		EnableRelay: false,
		EnableDHT:   true,
	}

	h2, err := New(cfg2)
	if err != nil {
		t.Fatalf("创建主机2失败: %v", err)
	}
	defer h2.Stop()

	if err := h2.Start(); err != nil {
		t.Fatalf("启动主机2失败: %v", err)
	}

	// 节点2 连接到节点1
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	h1Addrs := h1.Host().Addrs()
	if len(h1Addrs) == 0 {
		t.Fatal("主机1没有地址")
	}

	peerInfo := h1.Host().Peerstore().PeerInfo(h1.ID())
	peerInfo.Addrs = h1Addrs

	if err := h2.Connect(ctx, peerInfo); err != nil {
		t.Fatalf("连接失败: %v", err)
	}

	// 验证连接
	time.Sleep(100 * time.Millisecond)

	if h1.ConnectedPeers() == 0 {
		t.Error("主机1 没有连接的节点")
	}

	if h2.ConnectedPeers() == 0 {
		t.Error("主机2 没有连接的节点")
	}

	t.Logf("主机1 连接数: %d", h1.ConnectedPeers())
	t.Logf("主机2 连接数: %d", h2.ConnectedPeers())
}

func TestHost_Roles(t *testing.T) {
	roles := []NodeRole{RoleBootstrap, RoleRelay, RoleNormal}

	for _, role := range roles {
		t.Run(string(role), func(t *testing.T) {
			id, _ := identity.NewIdentity()
			cfg := &Config{
				Identity: id,
				ListenAddrs: []string{
					"/ip4/127.0.0.1/tcp/0",
				},
				Role:        role,
				EnableRelay: role == RoleRelay || role == RoleBootstrap,
				EnableDHT:   true,
			}

			h, err := New(cfg)
			if err != nil {
				t.Fatalf("创建 %s 主机失败: %v", role, err)
			}
			defer h.Stop()

			if err := h.Start(); err != nil {
				t.Fatalf("启动 %s 主机失败: %v", role, err)
			}

			t.Logf("%s 主机启动成功, ID: %s", role, h.ID().String()[:12])
		})
	}
}
