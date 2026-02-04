package webadmin

import (
	"fmt"
	"sync"
	"time"
)

// DefaultNodeInfoProvider provides a basic implementation of NodeInfoProvider
// that can be configured by the node at runtime.
type DefaultNodeInfoProvider struct {
	nodeID      string
	publicKey   string
	startTime   time.Time
	version     string
	p2pPort     int
	httpPort    int
	grpcPort    int
	adminPort   int
	isGenesis   bool
	isSupernode bool
	reputation  float64
	tokenCount  int64

	peers        []string
	endpoints    []APIEndpoint
	logs         []LogEntry
	maxLogs      int
	stats        *NetworkStats
	getPeersFunc func() []string

	mu sync.RWMutex
}

// NewDefaultNodeInfoProvider creates a new DefaultNodeInfoProvider.
func NewDefaultNodeInfoProvider() *DefaultNodeInfoProvider {
	return &DefaultNodeInfoProvider{
		startTime: time.Now(),
		version:   "0.1.0",
		maxLogs:   1000,
		logs:      make([]LogEntry, 0),
		stats: &NetworkStats{
			TotalPeers:  0,
			ActivePeers: 0,
		},
	}
}

// SetNodeInfo sets the basic node information.
func (p *DefaultNodeInfoProvider) SetNodeInfo(nodeID, publicKey, version string) {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.nodeID = nodeID
	p.publicKey = publicKey
	p.version = version
}

// SetPorts sets the port configuration.
func (p *DefaultNodeInfoProvider) SetPorts(p2p, http, grpc, admin int) {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.p2pPort = p2p
	p.httpPort = http
	p.grpcPort = grpc
	p.adminPort = admin
}

// SetRole sets the node role information.
func (p *DefaultNodeInfoProvider) SetRole(isGenesis, isSupernode bool) {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.isGenesis = isGenesis
	p.isSupernode = isSupernode
}

// SetReputation sets the node's reputation.
func (p *DefaultNodeInfoProvider) SetReputation(rep float64) {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.reputation = rep
}

// SetTokenCount sets the node's token count.
func (p *DefaultNodeInfoProvider) SetTokenCount(count int64) {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.tokenCount = count
}

// SetPeersFunc sets a function to dynamically get peers.
func (p *DefaultNodeInfoProvider) SetPeersFunc(fn func() []string) {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.getPeersFunc = fn
}

// SetPeers sets the list of peers directly.
func (p *DefaultNodeInfoProvider) SetPeers(peers []string) {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.peers = peers
}

// SetEndpoints sets the list of HTTP API endpoints.
func (p *DefaultNodeInfoProvider) SetEndpoints(endpoints []APIEndpoint) {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.endpoints = endpoints
}

// AddLog adds a log entry.
func (p *DefaultNodeInfoProvider) AddLog(level, module, message string) {
	p.mu.Lock()
	defer p.mu.Unlock()

	entry := LogEntry{
		Timestamp: time.Now(),
		Level:     level,
		Module:    module,
		Message:   message,
	}

	p.logs = append(p.logs, entry)
	if len(p.logs) > p.maxLogs {
		p.logs = p.logs[len(p.logs)-p.maxLogs:]
	}
}

// UpdateStats updates network statistics.
func (p *DefaultNodeInfoProvider) UpdateStats(stats *NetworkStats) {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.stats = stats
}

// --- NodeInfoProvider interface implementation ---

func (p *DefaultNodeInfoProvider) GetNodeID() string {
	p.mu.RLock()
	defer p.mu.RUnlock()
	return p.nodeID
}

func (p *DefaultNodeInfoProvider) GetPeerCount() int {
	return len(p.GetPeers())
}

func (p *DefaultNodeInfoProvider) GetPeers() []string {
	p.mu.RLock()
	defer p.mu.RUnlock()

	if p.getPeersFunc != nil {
		return p.getPeersFunc()
	}
	return p.peers
}

func (p *DefaultNodeInfoProvider) GetNodeStatus() *NodeStatus {
	p.mu.RLock()
	defer p.mu.RUnlock()

	uptime := time.Since(p.startTime)
	hours := int(uptime.Hours())
	minutes := int(uptime.Minutes()) % 60
	seconds := int(uptime.Seconds()) % 60

	uptimeStr := ""
	if hours > 0 {
		uptimeStr = fmt.Sprintf("%dh %dm %ds", hours, minutes, seconds)
	} else if minutes > 0 {
		uptimeStr = fmt.Sprintf("%dm %ds", minutes, seconds)
	} else {
		uptimeStr = fmt.Sprintf("%ds", seconds)
	}

	return &NodeStatus{
		NodeID:      p.nodeID,
		PublicKey:   p.publicKey,
		StartTime:   p.startTime,
		Uptime:      uptimeStr,
		Version:     p.version,
		P2PPort:     p.p2pPort,
		HTTPPort:    p.httpPort,
		GRPCPort:    p.grpcPort,
		AdminPort:   p.adminPort,
		IsGenesis:   p.isGenesis,
		IsSupernode: p.isSupernode,
		Reputation:  p.reputation,
		TokenCount:  p.tokenCount,
	}
}

func (p *DefaultNodeInfoProvider) GetHTTPAPIEndpoints() []APIEndpoint {
	p.mu.RLock()
	defer p.mu.RUnlock()

	if p.endpoints == nil {
		return defaultHTTPEndpoints()
	}
	return p.endpoints
}

func (p *DefaultNodeInfoProvider) GetRecentLogs(limit int) []LogEntry {
	p.mu.RLock()
	defer p.mu.RUnlock()

	if limit <= 0 || limit > len(p.logs) {
		limit = len(p.logs)
	}

	// Return most recent logs
	start := len(p.logs) - limit
	if start < 0 {
		start = 0
	}

	result := make([]LogEntry, limit)
	copy(result, p.logs[start:])
	return result
}

func (p *DefaultNodeInfoProvider) GetNetworkStats() *NetworkStats {
	p.mu.RLock()
	defer p.mu.RUnlock()

	if p.stats == nil {
		return &NetworkStats{}
	}

	// Update peer count
	stats := *p.stats
	stats.TotalPeers = len(p.GetPeers())
	stats.ActivePeers = stats.TotalPeers

	return &stats
}

// defaultHTTPEndpoints returns the default list of HTTP API endpoints.
func defaultHTTPEndpoints() []APIEndpoint {
	return []APIEndpoint{
		// Health
		{Method: "GET", Path: "/v1/health", Description: "健康检查", Category: "System"},
		{Method: "GET", Path: "/v1/info", Description: "节点信息", Category: "System"},

		// Peers
		{Method: "GET", Path: "/v1/peers", Description: "获取连接的节点列表", Category: "Network"},
		{Method: "POST", Path: "/v1/peers/connect", Description: "连接到指定节点", Category: "Network"},
		{Method: "DELETE", Path: "/v1/peers/{id}", Description: "断开与节点的连接", Category: "Network"},

		// Messages
		{Method: "POST", Path: "/v1/messages/send", Description: "发送消息给指定节点", Category: "Messaging"},
		{Method: "GET", Path: "/v1/messages/inbox", Description: "获取收件箱消息", Category: "Messaging"},
		{Method: "POST", Path: "/v1/messages/broadcast", Description: "广播消息", Category: "Messaging"},

		// Bulletin
		{Method: "GET", Path: "/v1/bulletin", Description: "获取留言板消息", Category: "Bulletin"},
		{Method: "POST", Path: "/v1/bulletin", Description: "发布留言", Category: "Bulletin"},

		// Reputation
		{Method: "GET", Path: "/v1/reputation/{id}", Description: "获取节点声誉", Category: "Reputation"},
		{Method: "POST", Path: "/v1/reputation/rate", Description: "评价节点", Category: "Reputation"},

		// Voting
		{Method: "GET", Path: "/v1/votes", Description: "获取投票列表", Category: "Voting"},
		{Method: "POST", Path: "/v1/votes", Description: "创建投票", Category: "Voting"},
		{Method: "POST", Path: "/v1/votes/{id}/cast", Description: "投票", Category: "Voting"},

		// Tasks
		{Method: "GET", Path: "/v1/tasks", Description: "获取任务列表", Category: "Tasks"},
		{Method: "POST", Path: "/v1/tasks", Description: "创建任务", Category: "Tasks"},
		{Method: "POST", Path: "/v1/tasks/{id}/accept", Description: "接受任务", Category: "Tasks"},
		{Method: "POST", Path: "/v1/tasks/{id}/complete", Description: "完成任务", Category: "Tasks"},
	}
}