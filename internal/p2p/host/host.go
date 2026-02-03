package host

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/libp2p/go-libp2p"
	dht "github.com/libp2p/go-libp2p-kad-dht"
	"github.com/libp2p/go-libp2p/core/host"
	"github.com/libp2p/go-libp2p/core/network"
	"github.com/libp2p/go-libp2p/core/peer"
	"github.com/libp2p/go-libp2p/core/routing"
	"github.com/libp2p/go-libp2p/p2p/net/connmgr"
	"github.com/libp2p/go-libp2p/p2p/protocol/circuitv2/relay"
	"github.com/libp2p/go-libp2p/p2p/security/noise"
	libp2ptls "github.com/libp2p/go-libp2p/p2p/security/tls"
	"github.com/multiformats/go-multiaddr"

	"github.com/AgentNetworkPlan/AgentNetwork/internal/p2p/identity"
)

// NodeRole 节点角色
type NodeRole string

const (
	RoleBootstrap NodeRole = "bootstrap" // 引导节点
	RoleRelay     NodeRole = "relay"     // 中转节点
	RoleNormal    NodeRole = "normal"    // 普通节点
)

// Config P2P 主机配置
type Config struct {
	Identity       *identity.Identity
	ListenAddrs    []string
	BootstrapPeers []string
	Role           NodeRole
	EnableRelay    bool
	EnableDHT      bool
}

// DefaultConfig 返回默认配置
func DefaultConfig() *Config {
	return &Config{
		ListenAddrs: []string{
			"/ip4/0.0.0.0/tcp/0",
			"/ip4/0.0.0.0/udp/0/quic-v1",
		},
		Role:        RoleNormal,
		EnableRelay: true,
		EnableDHT:   true,
	}
}

// Host P2P 主机
type Host struct {
	config   *Config
	host     host.Host
	dht      *dht.IpfsDHT
	ctx      context.Context
	cancel   context.CancelFunc
	mu       sync.RWMutex
	connChan chan peer.AddrInfo
}

// New 创建新的 P2P 主机
func New(cfg *Config) (*Host, error) {
	if cfg == nil {
		cfg = DefaultConfig()
	}

	// 如果没有身份，创建一个
	if cfg.Identity == nil {
		id, err := identity.NewIdentity()
		if err != nil {
			return nil, fmt.Errorf("创建身份失败: %w", err)
		}
		cfg.Identity = id
	}

	ctx, cancel := context.WithCancel(context.Background())

	h := &Host{
		config:   cfg,
		ctx:      ctx,
		cancel:   cancel,
		connChan: make(chan peer.AddrInfo, 100),
	}

	if err := h.init(); err != nil {
		cancel()
		return nil, err
	}

	return h, nil
}

// init 初始化 libp2p 主机
func (h *Host) init() error {
	// 解析监听地址
	listenAddrs := make([]multiaddr.Multiaddr, 0, len(h.config.ListenAddrs))
	for _, addr := range h.config.ListenAddrs {
		ma, err := multiaddr.NewMultiaddr(addr)
		if err != nil {
			return fmt.Errorf("解析监听地址失败 %s: %w", addr, err)
		}
		listenAddrs = append(listenAddrs, ma)
	}

	// 创建连接管理器
	connMgr, err := connmgr.NewConnManager(
		100, // 最小连接数
		400, // 最大连接数
		connmgr.WithGracePeriod(time.Minute),
	)
	if err != nil {
		return fmt.Errorf("创建连接管理器失败: %w", err)
	}

	// 构建 libp2p 选项
	opts := []libp2p.Option{
		libp2p.Identity(h.config.Identity.PrivKey),
		libp2p.ListenAddrs(listenAddrs...),
		libp2p.Security(libp2ptls.ID, libp2ptls.New),
		libp2p.Security(noise.ID, noise.New),
		libp2p.ConnectionManager(connMgr),
		libp2p.NATPortMap(),
		libp2p.EnableNATService(),
	}

	// 根据角色配置
	if h.config.Role == RoleRelay || h.config.Role == RoleBootstrap {
		// 作为 Relay 服务器
		opts = append(opts, libp2p.EnableRelayService())
	}

	if h.config.EnableRelay {
		// 启用 Relay 客户端（使用 Relay 中转）
		opts = append(opts, libp2p.EnableRelay())
	}

	// DHT 路由
	var kadDHT *dht.IpfsDHT
	if h.config.EnableDHT {
		opts = append(opts, libp2p.Routing(func(h host.Host) (routing.PeerRouting, error) {
			var dhtOpts []dht.Option

			// 根据角色配置 DHT 模式
			switch h.Network().LocalPeer() {
			default:
				if len(h.Network().ListenAddresses()) > 0 {
					dhtOpts = append(dhtOpts, dht.Mode(dht.ModeAutoServer))
				} else {
					dhtOpts = append(dhtOpts, dht.Mode(dht.ModeClient))
				}
			}

			var err error
			kadDHT, err = dht.New(context.Background(), h, dhtOpts...)
			return kadDHT, err
		}))
	}

	// 创建主机
	libp2pHost, err := libp2p.New(opts...)
	if err != nil {
		return fmt.Errorf("创建 libp2p 主机失败: %w", err)
	}

	h.host = libp2pHost
	h.dht = kadDHT

	// 设置连接通知
	h.host.Network().Notify(&network.NotifyBundle{
		ConnectedF: func(n network.Network, c network.Conn) {
			select {
			case h.connChan <- peer.AddrInfo{ID: c.RemotePeer(), Addrs: []multiaddr.Multiaddr{c.RemoteMultiaddr()}}:
			default:
			}
		},
	})

	return nil
}

// Start 启动 P2P 主机
func (h *Host) Start() error {
	fmt.Printf("🚀 P2P 节点启动\n")
	fmt.Printf("   PeerID: %s\n", h.host.ID())
	fmt.Printf("   角色: %s\n", h.config.Role)

	// 打印监听地址
	fmt.Printf("   监听地址:\n")
	for _, addr := range h.host.Addrs() {
		fmt.Printf("      %s/p2p/%s\n", addr, h.host.ID())
	}

	// 如果是 Relay 节点，启动 Relay 服务
	if h.config.Role == RoleRelay || h.config.Role == RoleBootstrap {
		_, err := relay.New(h.host)
		if err != nil {
			fmt.Printf("   ⚠️  启动 Relay 服务失败: %v\n", err)
		} else {
			fmt.Printf("   ✅ Relay 服务已启动\n")
		}
	}

	// 引导 DHT
	if h.dht != nil {
		if err := h.dht.Bootstrap(h.ctx); err != nil {
			return fmt.Errorf("DHT 引导失败: %w", err)
		}
		fmt.Printf("   ✅ DHT 已启动\n")
	}

	// 连接到引导节点
	if len(h.config.BootstrapPeers) > 0 {
		go h.connectBootstrapPeers()
	}

	return nil
}

// connectBootstrapPeers 连接到引导节点
func (h *Host) connectBootstrapPeers() {
	for _, addrStr := range h.config.BootstrapPeers {
		ma, err := multiaddr.NewMultiaddr(addrStr)
		if err != nil {
			fmt.Printf("   ⚠️  解析引导节点地址失败 %s: %v\n", addrStr, err)
			continue
		}

		peerInfo, err := peer.AddrInfoFromP2pAddr(ma)
		if err != nil {
			fmt.Printf("   ⚠️  解析引导节点信息失败 %s: %v\n", addrStr, err)
			continue
		}

		// 不连接自己
		if peerInfo.ID == h.host.ID() {
			continue
		}

		ctx, cancel := context.WithTimeout(h.ctx, 10*time.Second)
		if err := h.host.Connect(ctx, *peerInfo); err != nil {
			fmt.Printf("   ⚠️  连接引导节点失败 %s: %v\n", peerInfo.ID.String()[:12], err)
		} else {
			fmt.Printf("   ✅ 已连接引导节点: %s\n", peerInfo.ID.String()[:12])
		}
		cancel()
	}
}

// Stop 停止 P2P 主机
func (h *Host) Stop() error {
	h.cancel()

	if h.dht != nil {
		if err := h.dht.Close(); err != nil {
			fmt.Printf("关闭 DHT 失败: %v\n", err)
		}
	}

	return h.host.Close()
}

// Host 返回底层 libp2p 主机
func (h *Host) Host() host.Host {
	return h.host
}

// DHT 返回 DHT 实例
func (h *Host) DHT() *dht.IpfsDHT {
	return h.dht
}

// ID 返回节点 ID
func (h *Host) ID() peer.ID {
	return h.host.ID()
}

// Addrs 返回监听地址
func (h *Host) Addrs() []multiaddr.Multiaddr {
	return h.host.Addrs()
}

// Connect 连接到指定节点
func (h *Host) Connect(ctx context.Context, peerInfo peer.AddrInfo) error {
	return h.host.Connect(ctx, peerInfo)
}

// Peers 返回已连接的节点列表
func (h *Host) Peers() []peer.ID {
	return h.host.Network().Peers()
}

// ConnectedPeers 返回已连接的节点数量
func (h *Host) ConnectedPeers() int {
	return len(h.host.Network().Peers())
}

// FindPeer 通过 DHT 查找节点
func (h *Host) FindPeer(ctx context.Context, id peer.ID) (peer.AddrInfo, error) {
	if h.dht == nil {
		return peer.AddrInfo{}, fmt.Errorf("DHT 未启用")
	}
	return h.dht.FindPeer(ctx, id)
}

// Advertise 在 DHT 中广播服务
func (h *Host) Advertise(ctx context.Context, ns string) error {
	if h.dht == nil {
		return fmt.Errorf("DHT 未启用")
	}
	// 使用 DHT 的 Provide 功能
	// 这里简化处理，实际应该使用 discovery 包
	return nil
}

// ConnectionEvents 返回连接事件通道
func (h *Host) ConnectionEvents() <-chan peer.AddrInfo {
	return h.connChan
}
