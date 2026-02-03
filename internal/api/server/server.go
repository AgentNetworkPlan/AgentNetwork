package server

import (
	"context"
	"fmt"
	"net"
	"sync"
	"time"

	"github.com/libp2p/go-libp2p/core/peer"
	"google.golang.org/grpc"

	"github.com/AgentNetworkPlan/AgentNetwork/internal/p2p/node"
)

// NodeStatus 节点状态
type NodeStatus string

const (
	StatusOnline  NodeStatus = "online"
	StatusOffline NodeStatus = "offline"
	StatusBusy    NodeStatus = "busy"
)

// NodeEntry 节点条目
type NodeEntry struct {
	NodeID       string
	PeerID       peer.ID
	Addresses    []string
	Status       NodeStatus
	Capabilities []string
	ConnectedAt  time.Time
	LastSeen     time.Time
}

// Server gRPC 服务器
type Server struct {
	UnimplementedToolNetworkServer

	node       *node.Node
	grpcServer *grpc.Server
	listenAddr string

	mu    sync.RWMutex
	nodes map[string]*NodeEntry
}

// NewServer 创建 gRPC 服务器
func NewServer(n *node.Node, listenAddr string) *Server {
	return &Server{
		node:       n,
		listenAddr: listenAddr,
		nodes:      make(map[string]*NodeEntry),
	}
}

// Start 启动 gRPC 服务器
func (s *Server) Start() error {
	lis, err := net.Listen("tcp", s.listenAddr)
	if err != nil {
		return fmt.Errorf("监听失败: %w", err)
	}

	s.grpcServer = grpc.NewServer()
	RegisterToolNetworkServer(s.grpcServer, s)

	fmt.Printf("🌐 gRPC 服务启动: %s\n", s.listenAddr)

	go func() {
		if err := s.grpcServer.Serve(lis); err != nil {
			fmt.Printf("gRPC 服务错误: %v\n", err)
		}
	}()

	return nil
}

// Stop 停止 gRPC 服务器
func (s *Server) Stop() {
	if s.grpcServer != nil {
		s.grpcServer.GracefulStop()
	}
}

// GetNodeList 获取节点列表
func (s *Server) GetNodeList(ctx context.Context, filter *NodeFilter) (*NodeList, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	var nodes []*NodeInfo
	limit := int(filter.Limit)
	if limit <= 0 {
		limit = 100
	}

	// 从 P2P 网络获取连接的节点
	connectedPeers := s.node.Host().Peers()

	for _, peerID := range connectedPeers {
		if len(nodes) >= limit {
			break
		}

		// 应用过滤器
		if filter.Status != "" && filter.Status != string(StatusOnline) {
			continue
		}

		addrs := s.node.Host().Host().Peerstore().Addrs(peerID)
		addrStrs := make([]string, len(addrs))
		for i, addr := range addrs {
			addrStrs[i] = addr.String()
		}

		nodes = append(nodes, &NodeInfo{
			NodeId:      peerID.String(),
			PeerId:      peerID.String(),
			Addresses:   addrStrs,
			Status:      string(StatusOnline),
			ConnectedAt: time.Now().Unix(),
			LastSeen:    time.Now().Unix(),
		})
	}

	return &NodeList{
		Nodes: nodes,
		Total: int32(len(nodes)),
	}, nil
}

// GetNodeInfo 获取节点信息
func (s *Server) GetNodeInfo(ctx context.Context, req *NodeInfoRequest) (*NodeInfoResponse, error) {
	peerID, err := peer.Decode(req.NodeId)
	if err != nil {
		return &NodeInfoResponse{Found: false}, nil
	}

	// 检查是否已连接
	conns := s.node.Host().Host().Network().ConnsToPeer(peerID)
	if len(conns) == 0 {
		return &NodeInfoResponse{Found: false}, nil
	}

	addrs := s.node.Host().Host().Peerstore().Addrs(peerID)
	addrStrs := make([]string, len(addrs))
	for i, addr := range addrs {
		addrStrs[i] = addr.String()
	}

	return &NodeInfoResponse{
		Found: true,
		Node: &NodeInfo{
			NodeId:    peerID.String(),
			PeerId:    peerID.String(),
			Addresses: addrStrs,
			Status:    string(StatusOnline),
			LastSeen:  time.Now().Unix(),
		},
	}, nil
}

// SendTask 发送任务
func (s *Server) SendTask(ctx context.Context, req *TaskRequest) (*TaskResponse, error) {
	startTime := time.Now()

	// TODO: 实现任务分发逻辑
	// 1. 选择目标节点
	// 2. 通过 libp2p stream 发送任务
	// 3. 等待结果

	return &TaskResponse{
		TaskId:     req.TaskId,
		Success:    true,
		Result:     []byte("Task received"),
		ExecutedBy: s.node.ID(),
		DurationMs: time.Since(startTime).Milliseconds(),
	}, nil
}

// StoreData 存储数据
func (s *Server) StoreData(ctx context.Context, req *DataRequest) (*StoreResponse, error) {
	// TODO: 使用 DHT 存储数据
	// 目前返回成功作为占位

	return &StoreResponse{
		Success: true,
		Key:     req.Key,
	}, nil
}

// FetchData 获取数据
func (s *Server) FetchData(ctx context.Context, req *FetchRequest) (*FetchResponse, error) {
	// TODO: 从 DHT 获取数据

	return &FetchResponse{
		Found: false,
		Error: "数据未找到",
	}, nil
}

// Heartbeat 心跳
func (s *Server) Heartbeat(ctx context.Context, req *HeartbeatRequest) (*HeartbeatResponse, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	// 更新节点信息
	entry, exists := s.nodes[req.NodeId]
	if !exists {
		entry = &NodeEntry{
			NodeID:      req.NodeId,
			ConnectedAt: time.Now(),
		}
		s.nodes[req.NodeId] = entry
	}

	entry.Status = NodeStatus(req.Status)
	entry.Capabilities = req.Capabilities
	entry.LastSeen = time.Now()

	return &HeartbeatResponse{
		Success:    true,
		ServerTime: time.Now().Unix(),
	}, nil
}
