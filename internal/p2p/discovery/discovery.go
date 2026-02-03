package discovery

import (
	"context"
	"fmt"
	"sync"
	"time"

	dht "github.com/libp2p/go-libp2p-kad-dht"
	"github.com/libp2p/go-libp2p/core/host"
	"github.com/libp2p/go-libp2p/core/peer"
	drouting "github.com/libp2p/go-libp2p/p2p/discovery/routing"
	dutil "github.com/libp2p/go-libp2p/p2p/discovery/util"
)

const (
	// DiscoveryNamespace 发现命名空间
	DiscoveryNamespace = "/daan/1.0.0"
	// DiscoveryInterval 发现间隔
	DiscoveryInterval = time.Minute * 5
)

// Service 节点发现服务
type Service struct {
	host       host.Host
	dht        *dht.IpfsDHT
	routingDsc *drouting.RoutingDiscovery

	ctx    context.Context
	cancel context.CancelFunc

	peerChan chan peer.AddrInfo
	mu       sync.RWMutex
	peers    map[peer.ID]peer.AddrInfo
}

// NewService 创建节点发现服务
func NewService(h host.Host, kadDHT *dht.IpfsDHT) *Service {
	ctx, cancel := context.WithCancel(context.Background())

	routingDsc := drouting.NewRoutingDiscovery(kadDHT)

	return &Service{
		host:       h,
		dht:        kadDHT,
		routingDsc: routingDsc,
		ctx:        ctx,
		cancel:     cancel,
		peerChan:   make(chan peer.AddrInfo, 100),
		peers:      make(map[peer.ID]peer.AddrInfo),
	}
}

// Start 启动发现服务
func (s *Service) Start() error {
	fmt.Printf("🔍 节点发现服务启动\n")

	// 广播自己
	go s.advertise()

	// 发现其他节点
	go s.discover()

	return nil
}

// Stop 停止发现服务
func (s *Service) Stop() {
	s.cancel()
	close(s.peerChan)
}

// advertise 广播自己的存在
func (s *Service) advertise() {
	for {
		select {
		case <-s.ctx.Done():
			return
		default:
		}

		dutil.Advertise(s.ctx, s.routingDsc, DiscoveryNamespace)
		fmt.Printf("   📢 已广播节点信息到 DHT\n")

		select {
		case <-s.ctx.Done():
			return
		case <-time.After(DiscoveryInterval):
		}
	}
}

// discover 发现其他节点
func (s *Service) discover() {
	for {
		select {
		case <-s.ctx.Done():
			return
		default:
		}

		peerChan, err := s.routingDsc.FindPeers(s.ctx, DiscoveryNamespace)
		if err != nil {
			fmt.Printf("   ⚠️  发现节点失败: %v\n", err)
			time.Sleep(DiscoveryInterval)
			continue
		}

		for p := range peerChan {
			if p.ID == s.host.ID() {
				continue // 跳过自己
			}

			s.mu.Lock()
			if _, exists := s.peers[p.ID]; !exists {
				s.peers[p.ID] = p
				fmt.Printf("   🔗 发现新节点: %s\n", p.ID.String()[:12])

				// 尝试连接
				go s.connectPeer(p)
			}
			s.mu.Unlock()
		}

		select {
		case <-s.ctx.Done():
			return
		case <-time.After(DiscoveryInterval):
		}
	}
}

// connectPeer 连接到节点
func (s *Service) connectPeer(p peer.AddrInfo) {
	ctx, cancel := context.WithTimeout(s.ctx, 30*time.Second)
	defer cancel()

	if err := s.host.Connect(ctx, p); err != nil {
		fmt.Printf("   ⚠️  连接节点失败 %s: %v\n", p.ID.String()[:12], err)
		return
	}

	fmt.Printf("   ✅ 已连接节点: %s\n", p.ID.String()[:12])

	select {
	case s.peerChan <- p:
	default:
	}
}

// PeerChan 返回发现的节点通道
func (s *Service) PeerChan() <-chan peer.AddrInfo {
	return s.peerChan
}

// GetPeers 获取已发现的节点列表
func (s *Service) GetPeers() []peer.AddrInfo {
	s.mu.RLock()
	defer s.mu.RUnlock()

	peers := make([]peer.AddrInfo, 0, len(s.peers))
	for _, p := range s.peers {
		peers = append(peers, p)
	}
	return peers
}

// FindPeer 查找指定节点
func (s *Service) FindPeer(ctx context.Context, id peer.ID) (peer.AddrInfo, error) {
	return s.dht.FindPeer(ctx, id)
}
