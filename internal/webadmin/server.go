// Package webadmin provides a web-based administration interface for AgentNetwork nodes.
// It offers a Vue.js-based dashboard with real-time network topology visualization,
// node management, API exploration, and log viewing capabilities.
package webadmin

import (
	"context"
	"embed"
	"encoding/json"
	"fmt"
	"io/fs"
	"net/http"
	"strings"
	"sync"
	"time"

	"github.com/gorilla/websocket"
)

//go:embed static/*
var staticFiles embed.FS

// Config holds the configuration for the web admin server.
type Config struct {
	// ListenAddr is the address to listen on (default: 127.0.0.1:18080)
	ListenAddr string `json:"listen_addr"`

	// AdminToken is the authentication token for admin access
	AdminToken string `json:"admin_token"`

	// SessionDuration is how long a session cookie is valid (default: 24h)
	SessionDuration time.Duration `json:"session_duration"`

	// EnableCORS enables CORS headers for development
	EnableCORS bool `json:"enable_cors"`

	// StaticPath is an optional path to serve static files from disk (for development)
	StaticPath string `json:"static_path"`
}

// DefaultConfig returns the default configuration.
func DefaultConfig() *Config {
	return &Config{
		ListenAddr:      "127.0.0.1:18080",
		AdminToken:      "",
		SessionDuration: 24 * time.Hour,
		EnableCORS:      false,
		StaticPath:      "",
	}
}

// Server is the web administration server.
type Server struct {
	config     *Config
	httpServer *http.Server
	mux        *http.ServeMux
	auth       *AuthManager
	wsHub      *WebSocketHub
	topology   *TopologyManager
	handlers   *Handlers
	opHandlers *OperationHandlers
	extHandlers *ExtendedOperationHandlers
	nodeInfo   NodeInfoProvider
	opsProvider OperationsProvider
	extProvider ExtendedOperationsProvider

	mu      sync.RWMutex
	running bool
}

// NodeInfoProvider is the interface for getting node information.
type NodeInfoProvider interface {
	// GetNodeID returns the current node's ID
	GetNodeID() string

	// GetPeerCount returns the number of connected peers
	GetPeerCount() int

	// GetPeers returns the list of connected peer IDs
	GetPeers() []string

	// GetNodeStatus returns the node's current status
	GetNodeStatus() *NodeStatus

	// GetHTTPAPIEndpoints returns the list of HTTP API endpoints
	GetHTTPAPIEndpoints() []APIEndpoint

	// GetRecentLogs returns recent log entries
	GetRecentLogs(limit int) []LogEntry

	// GetNetworkStats returns network statistics
	GetNetworkStats() *NetworkStats

	// ConnectToPeer connects to a peer by multiaddr
	ConnectToPeer(multiaddr string) error

	// DisconnectPeer disconnects from a peer by ID
	DisconnectPeer(peerID string) error

	// GetSystemInfo returns system information
	GetSystemInfo() *SystemInfo

	// GetBootstrapNodes returns the list of bootstrap nodes
	GetBootstrapNodes() []string

	// AddBootstrapNode adds a bootstrap node
	AddBootstrapNode(addr string) error

	// RemoveBootstrapNode removes a bootstrap node
	RemoveBootstrapNode(addr string) error
}

// SystemInfo represents system information.
type SystemInfo struct {
	OS           string  `json:"os"`
	Arch         string  `json:"arch"`
	NumCPU       int     `json:"num_cpu"`
	NumGoroutine int     `json:"num_goroutine"`
	MemAlloc     uint64  `json:"mem_alloc"`
	MemTotal     uint64  `json:"mem_total"`
	MemSys       uint64  `json:"mem_sys"`
	GoVersion    string  `json:"go_version"`
	Hostname     string  `json:"hostname"`
}

// NodeStatus represents the current status of a node.
type NodeStatus struct {
	NodeID      string    `json:"node_id"`
	PublicKey   string    `json:"public_key"`
	StartTime   time.Time `json:"start_time"`
	Uptime      string    `json:"uptime"`
	Version     string    `json:"version"`
	P2PPort     int       `json:"p2p_port"`
	HTTPPort    int       `json:"http_port"`
	GRPCPort    int       `json:"grpc_port"`
	AdminPort   int       `json:"admin_port"`
	IsGenesis   bool      `json:"is_genesis"`
	IsSupernode bool      `json:"is_supernode"`
	Reputation  float64   `json:"reputation"`
	TokenCount  int64     `json:"token_count"`
}

// APIEndpoint represents an HTTP API endpoint.
type APIEndpoint struct {
	Method      string `json:"method"`
	Path        string `json:"path"`
	Description string `json:"description"`
	Category    string `json:"category"`
}

// LogEntry represents a log entry.
type LogEntry struct {
	Timestamp time.Time `json:"timestamp"`
	Level     string    `json:"level"`
	Module    string    `json:"module"`
	Message   string    `json:"message"`
}

// NetworkStats represents network statistics.
type NetworkStats struct {
	TotalPeers       int     `json:"total_peers"`
	ActivePeers      int     `json:"active_peers"`
	MessagesSent     int64   `json:"messages_sent"`
	MessagesReceived int64   `json:"messages_received"`
	BytesSent        int64   `json:"bytes_sent"`
	BytesReceived    int64   `json:"bytes_received"`
	AvgLatency       float64 `json:"avg_latency_ms"`
}

// New creates a new web admin server.
func New(config *Config, nodeInfo NodeInfoProvider) *Server {
	if config == nil {
		config = DefaultConfig()
	}

	s := &Server{
		config:   config,
		nodeInfo: nodeInfo,
		mux:      http.NewServeMux(),
	}

	s.auth = NewAuthManager(config.AdminToken, config.SessionDuration)
	s.wsHub = NewWebSocketHub()
	s.topology = NewTopologyManager(nodeInfo)
	s.handlers = NewHandlers(s)
	s.opHandlers = NewOperationHandlers(s, nil) // 初始化时没有操作提供者
	s.extHandlers = nil // 初始化时没有扩展操作提供者

	s.setupRoutes()

	return s
}

// SetOperationsProvider sets the operations provider for node control APIs.
func (s *Server) SetOperationsProvider(provider OperationsProvider) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.opsProvider = provider
	s.opHandlers = NewOperationHandlers(s, provider)
}

// SetExtendedOperationsProvider sets the extended operations provider for full API support.
func (s *Server) SetExtendedOperationsProvider(provider ExtendedOperationsProvider) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.extProvider = provider
	s.extHandlers = NewExtendedOperationHandlers(s, provider)
}

// setupRoutes configures all HTTP routes.
func (s *Server) setupRoutes() {
	// API routes
	s.mux.HandleFunc("/api/auth/login", s.wrapHandler(s.handlers.HandleLogin, false))
	s.mux.HandleFunc("/api/health", s.wrapHandler(s.handlers.HandleHealth, false))
	
	// Protected routes
	s.mux.HandleFunc("/api/node/status", s.wrapHandler(s.handlers.HandleNodeStatus, true))
	s.mux.HandleFunc("/api/node/peers", s.wrapHandler(s.handlers.HandlePeers, true))
	s.mux.HandleFunc("/api/node/config", s.wrapHandler(s.handlers.HandleConfig, true))
	s.mux.HandleFunc("/api/topology", s.wrapHandler(s.handlers.HandleTopology, true))
	s.mux.HandleFunc("/api/endpoints", s.wrapHandler(s.handlers.HandleEndpoints, true))
	s.mux.HandleFunc("/api/logs", s.wrapHandler(s.handlers.HandleLogs, true))
	s.mux.HandleFunc("/api/stats", s.wrapHandler(s.handlers.HandleStats, true))
	s.mux.HandleFunc("/api/auth/token/refresh", s.wrapHandler(s.handlers.HandleTokenRefresh, true))
	s.mux.HandleFunc("/api/auth/logout", s.wrapHandler(s.handlers.HandleLogout, true))

	// ========== 节点操作 API ==========
	// 邻居管理
	s.mux.HandleFunc("/api/neighbor/list", s.wrapOperationHandler(s.opHandlers.HandleNeighborList, true))
	s.mux.HandleFunc("/api/neighbor/best", s.wrapOperationHandler(s.opHandlers.HandleNeighborBest, true))
	s.mux.HandleFunc("/api/neighbor/add", s.wrapOperationHandler(s.opHandlers.HandleNeighborAdd, true))
	s.mux.HandleFunc("/api/neighbor/remove", s.wrapOperationHandler(s.opHandlers.HandleNeighborRemove, true))
	s.mux.HandleFunc("/api/neighbor/ping", s.wrapOperationHandler(s.opHandlers.HandleNeighborPing, true))

	// 邮箱操作
	s.mux.HandleFunc("/api/mailbox/send", s.wrapOperationHandler(s.opHandlers.HandleMailboxSend, true))
	s.mux.HandleFunc("/api/mailbox/inbox", s.wrapOperationHandler(s.opHandlers.HandleMailboxInbox, true))
	s.mux.HandleFunc("/api/mailbox/outbox", s.wrapOperationHandler(s.opHandlers.HandleMailboxOutbox, true))
	s.mux.HandleFunc("/api/mailbox/read/", s.wrapOperationHandler(s.opHandlers.HandleMailboxRead, true))
	s.mux.HandleFunc("/api/mailbox/mark-read", s.wrapOperationHandler(s.opHandlers.HandleMailboxMarkRead, true))
	s.mux.HandleFunc("/api/mailbox/delete", s.wrapOperationHandler(s.opHandlers.HandleMailboxDelete, true))

	// 留言板操作
	s.mux.HandleFunc("/api/bulletin/publish", s.wrapOperationHandler(s.opHandlers.HandleBulletinPublish, true))
	s.mux.HandleFunc("/api/bulletin/topic/", s.wrapOperationHandler(s.opHandlers.HandleBulletinByTopic, true))
	s.mux.HandleFunc("/api/bulletin/author/", s.wrapOperationHandler(s.opHandlers.HandleBulletinByAuthor, true))
	s.mux.HandleFunc("/api/bulletin/search", s.wrapOperationHandler(s.opHandlers.HandleBulletinSearch, true))
	s.mux.HandleFunc("/api/bulletin/subscribe", s.wrapOperationHandler(s.opHandlers.HandleBulletinSubscribe, true))
	s.mux.HandleFunc("/api/bulletin/unsubscribe", s.wrapOperationHandler(s.opHandlers.HandleBulletinUnsubscribe, true))
	s.mux.HandleFunc("/api/bulletin/revoke", s.wrapOperationHandler(s.opHandlers.HandleBulletinRevoke, true))
	s.mux.HandleFunc("/api/bulletin/subscriptions", s.wrapOperationHandler(s.opHandlers.HandleBulletinSubscriptions, true))

	// 声誉查询
	s.mux.HandleFunc("/api/reputation/query", s.wrapOperationHandler(s.opHandlers.HandleReputationQuery, true))
	s.mux.HandleFunc("/api/reputation/ranking", s.wrapOperationHandler(s.opHandlers.HandleReputationRanking, true))

	// 消息发送
	s.mux.HandleFunc("/api/message/send", s.wrapOperationHandler(s.opHandlers.HandleMessageSend, true))
	s.mux.HandleFunc("/api/message/broadcast", s.wrapOperationHandler(s.opHandlers.HandleMessageBroadcast, true))
	
	// 安全相关
	s.mux.HandleFunc("/api/security/status", s.wrapOperationHandler(s.opHandlers.HandleSecurityStatus, true))
	s.mux.HandleFunc("/api/security/report", s.wrapOperationHandler(s.opHandlers.HandleSecurityReport, true))
	
	// WebSocket routes
	s.mux.HandleFunc("/ws/topology", s.wsAuthMiddleware(s.handlers.HandleWSTopology))
	s.mux.HandleFunc("/ws/logs", s.wsAuthMiddleware(s.handlers.HandleWSLogs))
	s.mux.HandleFunc("/ws/stats", s.wsAuthMiddleware(s.handlers.HandleWSStats))

	// ========== 扩展 API (Task09 完整支持) ==========
	// 声誉扩展
	s.mux.HandleFunc("/api/reputation/update", s.wrapExtendedHandler(func(w http.ResponseWriter, r *http.Request) {
		if s.extHandlers != nil {
			s.extHandlers.HandleReputationUpdate(w, r)
		} else {
			WriteError(w, http.StatusServiceUnavailable, "Extended operations not configured")
		}
	}, true))
	s.mux.HandleFunc("/api/reputation/history", s.wrapExtendedHandler(func(w http.ResponseWriter, r *http.Request) {
		if s.extHandlers != nil {
			s.extHandlers.HandleReputationHistory(w, r)
		} else {
			WriteError(w, http.StatusServiceUnavailable, "Extended operations not configured")
		}
	}, true))

	// 任务管理
	s.mux.HandleFunc("/api/task/create", s.wrapExtendedHandler(func(w http.ResponseWriter, r *http.Request) {
		if s.extHandlers != nil { s.extHandlers.HandleTaskCreate(w, r) } else { WriteError(w, http.StatusServiceUnavailable, "Extended operations not configured") }
	}, true))
	s.mux.HandleFunc("/api/task/status", s.wrapExtendedHandler(func(w http.ResponseWriter, r *http.Request) {
		if s.extHandlers != nil { s.extHandlers.HandleTaskStatus(w, r) } else { WriteError(w, http.StatusServiceUnavailable, "Extended operations not configured") }
	}, true))
	s.mux.HandleFunc("/api/task/accept", s.wrapExtendedHandler(func(w http.ResponseWriter, r *http.Request) {
		if s.extHandlers != nil { s.extHandlers.HandleTaskAccept(w, r) } else { WriteError(w, http.StatusServiceUnavailable, "Extended operations not configured") }
	}, true))
	s.mux.HandleFunc("/api/task/submit", s.wrapExtendedHandler(func(w http.ResponseWriter, r *http.Request) {
		if s.extHandlers != nil { s.extHandlers.HandleTaskSubmit(w, r) } else { WriteError(w, http.StatusServiceUnavailable, "Extended operations not configured") }
	}, true))
	s.mux.HandleFunc("/api/task/list", s.wrapExtendedHandler(func(w http.ResponseWriter, r *http.Request) {
		if s.extHandlers != nil { s.extHandlers.HandleTaskList(w, r) } else { WriteError(w, http.StatusServiceUnavailable, "Extended operations not configured") }
	}, true))

	// 指责系统
	s.mux.HandleFunc("/api/accusation/create", s.wrapExtendedHandler(func(w http.ResponseWriter, r *http.Request) {
		if s.extHandlers != nil { s.extHandlers.HandleAccusationCreate(w, r) } else { WriteError(w, http.StatusServiceUnavailable, "Extended operations not configured") }
	}, true))
	s.mux.HandleFunc("/api/accusation/list", s.wrapExtendedHandler(func(w http.ResponseWriter, r *http.Request) {
		if s.extHandlers != nil { s.extHandlers.HandleAccusationList(w, r) } else { WriteError(w, http.StatusServiceUnavailable, "Extended operations not configured") }
	}, true))
	s.mux.HandleFunc("/api/accusation/detail/", s.wrapExtendedHandler(func(w http.ResponseWriter, r *http.Request) {
		if s.extHandlers != nil { s.extHandlers.HandleAccusationDetail(w, r) } else { WriteError(w, http.StatusServiceUnavailable, "Extended operations not configured") }
	}, true))
	s.mux.HandleFunc("/api/accusation/analyze", s.wrapExtendedHandler(func(w http.ResponseWriter, r *http.Request) {
		if s.extHandlers != nil { s.extHandlers.HandleAccusationAnalyze(w, r) } else { WriteError(w, http.StatusServiceUnavailable, "Extended operations not configured") }
	}, true))

	// 激励系统
	s.mux.HandleFunc("/api/incentive/award", s.wrapExtendedHandler(func(w http.ResponseWriter, r *http.Request) {
		if s.extHandlers != nil { s.extHandlers.HandleIncentiveAward(w, r) } else { WriteError(w, http.StatusServiceUnavailable, "Extended operations not configured") }
	}, true))
	s.mux.HandleFunc("/api/incentive/propagate", s.wrapExtendedHandler(func(w http.ResponseWriter, r *http.Request) {
		if s.extHandlers != nil { s.extHandlers.HandleIncentivePropagate(w, r) } else { WriteError(w, http.StatusServiceUnavailable, "Extended operations not configured") }
	}, true))
	s.mux.HandleFunc("/api/incentive/history", s.wrapExtendedHandler(func(w http.ResponseWriter, r *http.Request) {
		if s.extHandlers != nil { s.extHandlers.HandleIncentiveHistory(w, r) } else { WriteError(w, http.StatusServiceUnavailable, "Extended operations not configured") }
	}, true))
	s.mux.HandleFunc("/api/incentive/tolerance", s.wrapExtendedHandler(func(w http.ResponseWriter, r *http.Request) {
		if s.extHandlers != nil { s.extHandlers.HandleIncentiveTolerance(w, r) } else { WriteError(w, http.StatusServiceUnavailable, "Extended operations not configured") }
	}, true))

	// 投票系统
	s.mux.HandleFunc("/api/voting/proposal/create", s.wrapExtendedHandler(func(w http.ResponseWriter, r *http.Request) {
		if s.extHandlers != nil { s.extHandlers.HandleProposalCreate(w, r) } else { WriteError(w, http.StatusServiceUnavailable, "Extended operations not configured") }
	}, true))
	s.mux.HandleFunc("/api/voting/proposal/list", s.wrapExtendedHandler(func(w http.ResponseWriter, r *http.Request) {
		if s.extHandlers != nil { s.extHandlers.HandleProposalList(w, r) } else { WriteError(w, http.StatusServiceUnavailable, "Extended operations not configured") }
	}, true))
	s.mux.HandleFunc("/api/voting/proposal/finalize", s.wrapExtendedHandler(func(w http.ResponseWriter, r *http.Request) {
		if s.extHandlers != nil { s.extHandlers.HandleProposalFinalize(w, r) } else { WriteError(w, http.StatusServiceUnavailable, "Extended operations not configured") }
	}, true))
	s.mux.HandleFunc("/api/voting/proposal/", s.wrapExtendedHandler(func(w http.ResponseWriter, r *http.Request) {
		if s.extHandlers != nil { s.extHandlers.HandleProposalDetail(w, r) } else { WriteError(w, http.StatusServiceUnavailable, "Extended operations not configured") }
	}, true))
	s.mux.HandleFunc("/api/voting/vote", s.wrapExtendedHandler(func(w http.ResponseWriter, r *http.Request) {
		if s.extHandlers != nil { s.extHandlers.HandleVotingVote(w, r) } else { WriteError(w, http.StatusServiceUnavailable, "Extended operations not configured") }
	}, true))

	// 超级节点
	s.mux.HandleFunc("/api/supernode/list", s.wrapExtendedHandler(func(w http.ResponseWriter, r *http.Request) {
		if s.extHandlers != nil { s.extHandlers.HandleSupernodeList(w, r) } else { WriteError(w, http.StatusServiceUnavailable, "Extended operations not configured") }
	}, true))
	s.mux.HandleFunc("/api/supernode/candidates", s.wrapExtendedHandler(func(w http.ResponseWriter, r *http.Request) {
		if s.extHandlers != nil { s.extHandlers.HandleCandidatesList(w, r) } else { WriteError(w, http.StatusServiceUnavailable, "Extended operations not configured") }
	}, true))
	s.mux.HandleFunc("/api/supernode/apply", s.wrapExtendedHandler(func(w http.ResponseWriter, r *http.Request) {
		if s.extHandlers != nil { s.extHandlers.HandleSupernodeApply(w, r) } else { WriteError(w, http.StatusServiceUnavailable, "Extended operations not configured") }
	}, true))
	s.mux.HandleFunc("/api/supernode/withdraw", s.wrapExtendedHandler(func(w http.ResponseWriter, r *http.Request) {
		if s.extHandlers != nil { s.extHandlers.HandleSupernodeWithdraw(w, r) } else { WriteError(w, http.StatusServiceUnavailable, "Extended operations not configured") }
	}, true))
	s.mux.HandleFunc("/api/supernode/vote", s.wrapExtendedHandler(func(w http.ResponseWriter, r *http.Request) {
		if s.extHandlers != nil { s.extHandlers.HandleSupernodeVote(w, r) } else { WriteError(w, http.StatusServiceUnavailable, "Extended operations not configured") }
	}, true))
	s.mux.HandleFunc("/api/supernode/election/start", s.wrapExtendedHandler(func(w http.ResponseWriter, r *http.Request) {
		if s.extHandlers != nil { s.extHandlers.HandleElectionStart(w, r) } else { WriteError(w, http.StatusServiceUnavailable, "Extended operations not configured") }
	}, true))
	s.mux.HandleFunc("/api/supernode/election/finalize", s.wrapExtendedHandler(func(w http.ResponseWriter, r *http.Request) {
		if s.extHandlers != nil { s.extHandlers.HandleElectionFinalize(w, r) } else { WriteError(w, http.StatusServiceUnavailable, "Extended operations not configured") }
	}, true))
	s.mux.HandleFunc("/api/supernode/audit/submit", s.wrapExtendedHandler(func(w http.ResponseWriter, r *http.Request) {
		if s.extHandlers != nil { s.extHandlers.HandleAuditSubmit(w, r) } else { WriteError(w, http.StatusServiceUnavailable, "Extended operations not configured") }
	}, true))
	s.mux.HandleFunc("/api/supernode/audit/result", s.wrapExtendedHandler(func(w http.ResponseWriter, r *http.Request) {
		if s.extHandlers != nil { s.extHandlers.HandleAuditResult(w, r) } else { WriteError(w, http.StatusServiceUnavailable, "Extended operations not configured") }
	}, true))

	// 创世节点
	s.mux.HandleFunc("/api/genesis/info", s.wrapExtendedHandler(func(w http.ResponseWriter, r *http.Request) {
		if s.extHandlers != nil { s.extHandlers.HandleGenesisInfo(w, r) } else { WriteError(w, http.StatusServiceUnavailable, "Extended operations not configured") }
	}, true))
	s.mux.HandleFunc("/api/genesis/invite/create", s.wrapExtendedHandler(func(w http.ResponseWriter, r *http.Request) {
		if s.extHandlers != nil { s.extHandlers.HandleGenesisInviteCreate(w, r) } else { WriteError(w, http.StatusServiceUnavailable, "Extended operations not configured") }
	}, true))
	s.mux.HandleFunc("/api/genesis/invite/verify", s.wrapExtendedHandler(func(w http.ResponseWriter, r *http.Request) {
		if s.extHandlers != nil { s.extHandlers.HandleGenesisInviteVerify(w, r) } else { WriteError(w, http.StatusServiceUnavailable, "Extended operations not configured") }
	}, true))
	s.mux.HandleFunc("/api/genesis/join", s.wrapExtendedHandler(func(w http.ResponseWriter, r *http.Request) {
		if s.extHandlers != nil { s.extHandlers.HandleGenesisJoin(w, r) } else { WriteError(w, http.StatusServiceUnavailable, "Extended operations not configured") }
	}, true))

	// 日志系统
	s.mux.HandleFunc("/api/log/submit", s.wrapExtendedHandler(func(w http.ResponseWriter, r *http.Request) {
		if s.extHandlers != nil { s.extHandlers.HandleLogSubmit(w, r) } else { WriteError(w, http.StatusServiceUnavailable, "Extended operations not configured") }
	}, true))
	s.mux.HandleFunc("/api/log/query", s.wrapExtendedHandler(func(w http.ResponseWriter, r *http.Request) {
		if s.extHandlers != nil { s.extHandlers.HandleLogQuery(w, r) } else { WriteError(w, http.StatusServiceUnavailable, "Extended operations not configured") }
	}, true))
	s.mux.HandleFunc("/api/log/export", s.wrapExtendedHandler(func(w http.ResponseWriter, r *http.Request) {
		if s.extHandlers != nil { s.extHandlers.HandleLogExport(w, r) } else { WriteError(w, http.StatusServiceUnavailable, "Extended operations not configured") }
	}, true))

	// 审计集成 (Task44)
	s.mux.HandleFunc("/api/audit/deviations", s.wrapExtendedHandler(func(w http.ResponseWriter, r *http.Request) {
		if s.extHandlers != nil { s.extHandlers.HandleAuditDeviations(w, r) } else { WriteError(w, http.StatusServiceUnavailable, "Extended operations not configured") }
	}, true))
	s.mux.HandleFunc("/api/audit/penalty-config", s.wrapExtendedHandler(func(w http.ResponseWriter, r *http.Request) {
		if s.extHandlers != nil { s.extHandlers.HandleAuditPenaltyConfig(w, r) } else { WriteError(w, http.StatusServiceUnavailable, "Extended operations not configured") }
	}, true))
	s.mux.HandleFunc("/api/audit/manual-penalty", s.wrapExtendedHandler(func(w http.ResponseWriter, r *http.Request) {
		if s.extHandlers != nil { s.extHandlers.HandleManualPenalty(w, r) } else { WriteError(w, http.StatusServiceUnavailable, "Extended operations not configured") }
	}, true))

	// 抵押物管理 (Task44)
	s.mux.HandleFunc("/api/collateral/list", s.wrapExtendedHandler(func(w http.ResponseWriter, r *http.Request) {
		if s.extHandlers != nil { s.extHandlers.HandleCollateralList(w, r) } else { WriteError(w, http.StatusServiceUnavailable, "Extended operations not configured") }
	}, true))
	s.mux.HandleFunc("/api/collateral/by-node", s.wrapExtendedHandler(func(w http.ResponseWriter, r *http.Request) {
		if s.extHandlers != nil { s.extHandlers.HandleCollateralByNode(w, r) } else { WriteError(w, http.StatusServiceUnavailable, "Extended operations not configured") }
	}, true))
	s.mux.HandleFunc("/api/collateral/slash-by-node", s.wrapExtendedHandler(func(w http.ResponseWriter, r *http.Request) {
		if s.extHandlers != nil { s.extHandlers.HandleSlashByNode(w, r) } else { WriteError(w, http.StatusServiceUnavailable, "Extended operations not configured") }
	}, true))
	s.mux.HandleFunc("/api/collateral/slash-history", s.wrapExtendedHandler(func(w http.ResponseWriter, r *http.Request) {
		if s.extHandlers != nil { s.extHandlers.HandleSlashHistory(w, r) } else { WriteError(w, http.StatusServiceUnavailable, "Extended operations not configured") }
	}, true))

	// 争议预审 (Task44)
	s.mux.HandleFunc("/api/dispute/list", s.wrapExtendedHandler(func(w http.ResponseWriter, r *http.Request) {
		if s.extHandlers != nil { s.extHandlers.HandleDisputeList(w, r) } else { WriteError(w, http.StatusServiceUnavailable, "Extended operations not configured") }
	}, true))
	s.mux.HandleFunc("/api/dispute/suggestion/", s.wrapExtendedHandler(func(w http.ResponseWriter, r *http.Request) {
		if s.extHandlers != nil { s.extHandlers.HandleDisputeSuggestion(w, r) } else { WriteError(w, http.StatusServiceUnavailable, "Extended operations not configured") }
	}, true))
	s.mux.HandleFunc("/api/dispute/verify-evidence", s.wrapExtendedHandler(func(w http.ResponseWriter, r *http.Request) {
		if s.extHandlers != nil { s.extHandlers.HandleVerifyEvidence(w, r) } else { WriteError(w, http.StatusServiceUnavailable, "Extended operations not configured") }
	}, true))
	s.mux.HandleFunc("/api/dispute/apply-suggestion", s.wrapExtendedHandler(func(w http.ResponseWriter, r *http.Request) {
		if s.extHandlers != nil { s.extHandlers.HandleApplySuggestion(w, r) } else { WriteError(w, http.StatusServiceUnavailable, "Extended operations not configured") }
	}, true))
	s.mux.HandleFunc("/api/dispute/detail/", s.wrapExtendedHandler(func(w http.ResponseWriter, r *http.Request) {
		if s.extHandlers != nil { s.extHandlers.HandleDisputeDetail(w, r) } else { WriteError(w, http.StatusServiceUnavailable, "Extended operations not configured") }
	}, true))

	// 托管多签 (Task44)
	s.mux.HandleFunc("/api/escrow/list", s.wrapExtendedHandler(func(w http.ResponseWriter, r *http.Request) {
		if s.extHandlers != nil { s.extHandlers.HandleEscrowList(w, r) } else { WriteError(w, http.StatusServiceUnavailable, "Extended operations not configured") }
	}, true))
	s.mux.HandleFunc("/api/escrow/detail/", s.wrapExtendedHandler(func(w http.ResponseWriter, r *http.Request) {
		if s.extHandlers != nil { s.extHandlers.HandleEscrowDetail(w, r) } else { WriteError(w, http.StatusServiceUnavailable, "Extended operations not configured") }
	}, true))
	s.mux.HandleFunc("/api/escrow/arbitrator-signature", s.wrapExtendedHandler(func(w http.ResponseWriter, r *http.Request) {
		if s.extHandlers != nil { s.extHandlers.HandleArbitratorSignature(w, r) } else { WriteError(w, http.StatusServiceUnavailable, "Extended operations not configured") }
	}, true))
	s.mux.HandleFunc("/api/escrow/signature-count/", s.wrapExtendedHandler(func(w http.ResponseWriter, r *http.Request) {
		if s.extHandlers != nil { s.extHandlers.HandleSignatureCount(w, r) } else { WriteError(w, http.StatusServiceUnavailable, "Extended operations not configured") }
	}, true))
	s.mux.HandleFunc("/api/escrow/resolve", s.wrapExtendedHandler(func(w http.ResponseWriter, r *http.Request) {
		if s.extHandlers != nil { s.extHandlers.HandleEscrowResolve(w, r) } else { WriteError(w, http.StatusServiceUnavailable, "Extended operations not configured") }
	}, true))

	// Static files (Vue.js app)
	s.setupStaticFiles()
}

// wrapExtendedHandler wraps extended operation handler with CORS and auth middleware.
func (s *Server) wrapExtendedHandler(handler http.HandlerFunc, requireAuth bool) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		// CORS headers
		if s.config.EnableCORS {
			w.Header().Set("Access-Control-Allow-Origin", "*")
			w.Header().Set("Access-Control-Allow-Methods", "GET, POST, PUT, DELETE, OPTIONS")
			w.Header().Set("Access-Control-Allow-Headers", "Content-Type, Authorization")
		}

		if r.Method == "OPTIONS" {
			w.WriteHeader(http.StatusOK)
			return
		}

		// Auth check
		if requireAuth && !s.checkAuth(r) {
			http.Error(w, "Unauthorized", http.StatusUnauthorized)
			return
		}

		handler(w, r)
	}
}

// wrapOperationHandler wraps operation handler with CORS and auth middleware.
func (s *Server) wrapOperationHandler(handler http.HandlerFunc, requireAuth bool) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		// CORS headers
		if s.config.EnableCORS {
			w.Header().Set("Access-Control-Allow-Origin", "*")
			w.Header().Set("Access-Control-Allow-Methods", "GET, POST, PUT, DELETE, OPTIONS")
			w.Header().Set("Access-Control-Allow-Headers", "Content-Type, Authorization")
		}

		if r.Method == "OPTIONS" {
			w.WriteHeader(http.StatusOK)
			return
		}

		// Auth check
		if requireAuth && !s.checkAuth(r) {
			http.Error(w, "Unauthorized", http.StatusUnauthorized)
			return
		}

		// 确保使用最新的操作处理器
		s.mu.RLock()
		opHandler := s.opHandlers
		s.mu.RUnlock()

		if opHandler != nil && opHandler.provider != nil {
			handler(w, r)
		} else {
			WriteError(w, http.StatusServiceUnavailable, "Operations provider not configured")
		}
	}
}

// wrapHandler wraps a handler with CORS and optional auth middleware.
func (s *Server) wrapHandler(handler http.HandlerFunc, requireAuth bool) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		// CORS headers
		if s.config.EnableCORS {
			w.Header().Set("Access-Control-Allow-Origin", "*")
			w.Header().Set("Access-Control-Allow-Methods", "GET, POST, PUT, DELETE, OPTIONS")
			w.Header().Set("Access-Control-Allow-Headers", "Content-Type, Authorization")
		}

		if r.Method == "OPTIONS" {
			w.WriteHeader(http.StatusOK)
			return
		}

		// Auth check
		if requireAuth && !s.checkAuth(r) {
			http.Error(w, "Unauthorized", http.StatusUnauthorized)
			return
		}

		handler(w, r)
	}
}

// checkAuth checks if the request is authenticated.
func (s *Server) checkAuth(r *http.Request) bool {
	// Check URL token parameter (quick access with admin token)
	token := r.URL.Query().Get("token")
	if token != "" && s.auth.ValidateToken(token) {
		return true
	}

	// Check session cookie (session ID)
	cookie, err := r.Cookie(TokenCookieName)
	if err == nil {
		// First try as session ID
		if s.auth.ValidateSession(cookie.Value) {
			return true
		}
		// Fallback to direct token validation for backward compatibility
		if s.auth.ValidateToken(cookie.Value) {
			return true
		}
	}

	// Check Authorization header (admin token)
	authHeader := r.Header.Get("Authorization")
	if strings.HasPrefix(authHeader, "Bearer ") {
		token := strings.TrimPrefix(authHeader, "Bearer ")
		if s.auth.ValidateToken(token) {
			return true
		}
	}

	return false
}

// setupStaticFiles configures static file serving with SPA fallback.
func (s *Server) setupStaticFiles() {
	if s.config.StaticPath != "" {
		// Serve from disk (development mode) with SPA fallback
		s.mux.Handle("/", &spaHandler{
			staticPath: s.config.StaticPath,
			indexPath:  "index.html",
		})
	} else {
		// Serve embedded files (production mode)
		subFS, err := fs.Sub(staticFiles, "static")
		if err != nil {
			// If no embedded files, serve a simple placeholder
			s.mux.HandleFunc("/", s.handlers.HandleIndex)
			return
		}
		s.mux.Handle("/", &spaEmbedHandler{
			fs:        http.FS(subFS),
			indexHTML: mustReadIndex(subFS),
		})
	}
}

// spaHandler serves static files with SPA fallback for disk-based files.
type spaHandler struct {
	staticPath string
	indexPath  string
}

func (h *spaHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	// Get the absolute path to prevent directory traversal
	path := r.URL.Path
	if path == "/" {
		path = "/index.html"
	}

	// Try to serve the static file
	if _, err := http.Dir(h.staticPath).Open(path); err == nil {
		http.FileServer(http.Dir(h.staticPath)).ServeHTTP(w, r)
		return
	}

	// For SPA: serve index.html for all non-file routes
	http.ServeFile(w, r, h.staticPath+"/"+h.indexPath)
}

// spaEmbedHandler serves embedded static files with SPA fallback.
type spaEmbedHandler struct {
	fs        http.FileSystem
	indexHTML []byte
}

func (h *spaEmbedHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	path := r.URL.Path

	// Try to open the file
	f, err := h.fs.Open(path)
	if err == nil {
		defer f.Close()
		stat, err := f.Stat()
		if err == nil && !stat.IsDir() {
			// File exists, serve it
			http.FileServer(h.fs).ServeHTTP(w, r)
			return
		}
	}

	// For SPA routes (e.g., /login, /dashboard), serve index.html
	// Don't fallback for API or WS routes
	if strings.HasPrefix(path, "/api/") || strings.HasPrefix(path, "/ws/") {
		http.NotFound(w, r)
		return
	}

	// Serve index.html for SPA routing
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.WriteHeader(http.StatusOK)
	w.Write(h.indexHTML)
}

// mustReadIndex reads index.html from the embedded filesystem.
func mustReadIndex(fsys fs.FS) []byte {
	data, err := fs.ReadFile(fsys, "index.html")
	if err != nil {
		return []byte("<!DOCTYPE html><html><body><h1>DAAN Admin</h1><p>Frontend not built.</p></body></html>")
	}
	return data
}

// wsAuthMiddleware wraps a WebSocket handler with authentication.
func (s *Server) wsAuthMiddleware(handler http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		// Check URL token parameter
		token := r.URL.Query().Get("token")
		if token != "" && s.auth.ValidateToken(token) {
			handler(w, r)
			return
		}

		// Check token cookie
		cookie, err := r.Cookie(TokenCookieName)
		if err == nil && s.auth.ValidateToken(cookie.Value) {
			handler(w, r)
			return
		}

		http.Error(w, "Unauthorized", http.StatusUnauthorized)
	}
}

// Start starts the web admin server.
func (s *Server) Start() error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if s.running {
		return fmt.Errorf("server already running")
	}

	s.httpServer = &http.Server{
		Addr:         s.config.ListenAddr,
		Handler:      s.mux,
		ReadTimeout:  15 * time.Second,
		WriteTimeout: 15 * time.Second,
		IdleTimeout:  60 * time.Second,
	}

	// Start WebSocket hub
	go s.wsHub.Run()

	// Start topology updates
	go s.topology.StartUpdates(s.wsHub)

	s.running = true

	go func() {
		if err := s.httpServer.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			// Log error
			fmt.Printf("Web admin server error: %v\n", err)
		}
	}()

	fmt.Printf("🌐 Web Admin server started at http://%s\n", s.config.ListenAddr)
	return nil
}

// Stop stops the web admin server.
func (s *Server) Stop() error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if !s.running {
		return nil
	}

	s.topology.StopUpdates()
	s.wsHub.Close()

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	if err := s.httpServer.Shutdown(ctx); err != nil {
		return fmt.Errorf("shutdown error: %w", err)
	}

	s.running = false
	return nil
}

// IsRunning returns whether the server is running.
func (s *Server) IsRunning() bool {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.running
}

// GetAdminURL returns the admin panel URL with token.
func (s *Server) GetAdminURL() string {
	addr := s.config.ListenAddr
	if addr[0] == ':' {
		addr = "localhost" + addr
	}
	if s.config.AdminToken != "" {
		return fmt.Sprintf("http://%s/login?token=%s", addr, s.config.AdminToken)
	}
	return fmt.Sprintf("http://%s", addr)
}

// WriteJSON writes a JSON response.
func WriteJSON(w http.ResponseWriter, status int, data interface{}) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(data)
}

// WriteError writes a JSON error response.
func WriteError(w http.ResponseWriter, status int, message string) {
	WriteJSON(w, status, map[string]string{"error": message})
}

// WebSocket upgrader
var wsUpgrader = websocket.Upgrader{
	ReadBufferSize:  1024,
	WriteBufferSize: 1024,
	CheckOrigin: func(r *http.Request) bool {
		return true // Allow all origins for now
	},
}
