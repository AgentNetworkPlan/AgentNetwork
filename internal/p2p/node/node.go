package node

import (
	"context"
	"fmt"
	"os"
	"os/signal"
	"syscall"

	"github.com/AgentNetworkPlan/AgentNetwork/internal/p2p/discovery"
	"github.com/AgentNetworkPlan/AgentNetwork/internal/p2p/host"
	"github.com/AgentNetworkPlan/AgentNetwork/internal/p2p/identity"
)

// Config 节点配置
type Config struct {
	// 身份相关
	KeyPath string

	// 网络相关
	ListenAddrs    []string
	BootstrapPeers []string
	Role           host.NodeRole

	// 功能开关
	EnableRelay bool
	EnableDHT   bool
}

// DefaultConfig 返回默认配置
func DefaultConfig() *Config {
	return &Config{
		KeyPath: "keys/node.key",
		ListenAddrs: []string{
			"/ip4/0.0.0.0/tcp/0",
			"/ip4/0.0.0.0/udp/0/quic-v1",
		},
		Role:        host.RoleNormal,
		EnableRelay: true,
		EnableDHT:   true,
	}
}

// Node P2P 网络节点
type Node struct {
	config    *Config
	identity  *identity.Identity
	host      *host.Host
	discovery *discovery.Service

	ctx    context.Context
	cancel context.CancelFunc
}

// New 创建新节点
func New(cfg *Config) (*Node, error) {
	if cfg == nil {
		cfg = DefaultConfig()
	}

	ctx, cancel := context.WithCancel(context.Background())

	// 加载或创建身份
	id, err := identity.LoadOrCreate(cfg.KeyPath)
	if err != nil {
		cancel()
		return nil, fmt.Errorf("加载身份失败: %w", err)
	}

	// 创建 P2P 主机
	hostCfg := &host.Config{
		Identity:       id,
		ListenAddrs:    cfg.ListenAddrs,
		BootstrapPeers: cfg.BootstrapPeers,
		Role:           cfg.Role,
		EnableRelay:    cfg.EnableRelay,
		EnableDHT:      cfg.EnableDHT,
	}

	h, err := host.New(hostCfg)
	if err != nil {
		cancel()
		return nil, fmt.Errorf("创建 P2P 主机失败: %w", err)
	}

	n := &Node{
		config:   cfg,
		identity: id,
		host:     h,
		ctx:      ctx,
		cancel:   cancel,
	}

	return n, nil
}

// Start 启动节点
func (n *Node) Start() error {
	fmt.Println("═══════════════════════════════════════════")
	fmt.Println("🔗 DAAN P2P 节点")
	fmt.Println("═══════════════════════════════════════════")

	// 启动 P2P 主机
	if err := n.host.Start(); err != nil {
		return fmt.Errorf("启动 P2P 主机失败: %w", err)
	}

	// 如果 DHT 可用，启动发现服务
	if n.host.DHT() != nil {
		n.discovery = discovery.NewService(n.host.Host(), n.host.DHT())
		if err := n.discovery.Start(); err != nil {
			fmt.Printf("⚠️  启动发现服务失败: %v\n", err)
		}
	}

	fmt.Println("═══════════════════════════════════════════")
	fmt.Printf("📊 当前连接节点数: %d\n", n.host.ConnectedPeers())
	fmt.Println("═══════════════════════════════════════════")

	return nil
}

// Run 运行节点（阻塞直到收到停止信号）
func (n *Node) Run() error {
	if err := n.Start(); err != nil {
		return err
	}

	// 等待停止信号
	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)

	fmt.Println("按 Ctrl+C 停止节点...")

	<-sigCh
	fmt.Println("\n正在停止节点...")

	return n.Stop()
}

// Stop 停止节点
func (n *Node) Stop() error {
	n.cancel()

	if n.discovery != nil {
		n.discovery.Stop()
	}

	if n.host != nil {
		return n.host.Stop()
	}

	return nil
}

// Identity 返回节点身份
func (n *Node) Identity() *identity.Identity {
	return n.identity
}

// Host 返回 P2P 主机
func (n *Node) Host() *host.Host {
	return n.host
}

// Discovery 返回发现服务
func (n *Node) Discovery() *discovery.Service {
	return n.discovery
}

// ID 返回节点 ID
func (n *Node) ID() string {
	return n.identity.PeerID.String()
}

// ShortID 返回短格式节点 ID
func (n *Node) ShortID() string {
	return n.identity.ShortID()
}
