// Package httpapi 提供 HTTP REST API 接口
// 支持节点管理、消息、任务、声誉、日志等功能的 HTTP 访问
package httpapi

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strconv"
	"strings"
	"sync"
	"time"
)

// 错误定义
var (
	ErrNilConfig       = errors.New("config cannot be nil")
	ErrEmptyNodeID     = errors.New("node ID cannot be empty")
	ErrInvalidRequest  = errors.New("invalid request")
	ErrUnauthorized    = errors.New("unauthorized")
	ErrNotFound        = errors.New("not found")
	ErrMethodNotAllowed = errors.New("method not allowed")
)

// Config HTTP API 配置
type Config struct {
	NodeID        string
	ListenAddr    string        // 监听地址 (e.g., ":18345")
	ReadTimeout   time.Duration
	WriteTimeout  time.Duration
	EnableCORS    bool
	MaxBodySize   int64
	
	// Token 认证配置
	APIToken       string // API Token（为空则自动生成）
	AuthEnabled    bool   // 是否启用 Token 认证（默认启用）
	
	// 签名函数（用于验证请求）
	VerifyFunc func(nodeID string, data []byte, signature string) bool
}

// DefaultConfig 返回默认配置
func DefaultConfig(nodeID string) *Config {
	return &Config{
		NodeID:       nodeID,
		ListenAddr:   ":18345",
		ReadTimeout:  30 * time.Second,
		WriteTimeout: 30 * time.Second,
		EnableCORS:   true,
		MaxBodySize:  10 * 1024 * 1024, // 10MB
		AuthEnabled:  true,             // 默认启用认证
	}
}

// Response 通用响应
type Response struct {
	Success bool        `json:"success"`
	Data    interface{} `json:"data,omitempty"`
	Error   string      `json:"error,omitempty"`
	Code    int         `json:"code"`
}

// NodeInfoResponse 节点信息响应
type NodeInfoResponse struct {
	NodeID    string   `json:"node_id"`
	Addresses []string `json:"addresses"`
	Status    string   `json:"status"`
	Uptime    int64    `json:"uptime"`
	Version   string   `json:"version"`
}

// PeerInfo 节点信息
type PeerInfo struct {
	NodeID      string    `json:"node_id"`
	Addresses   []string  `json:"addresses"`
	Status      string    `json:"status"`
	ConnectedAt time.Time `json:"connected_at"`
	LastSeen    time.Time `json:"last_seen"`
}

// MessageRequest 消息请求
type MessageRequest struct {
	To        string                 `json:"to"`
	Type      string                 `json:"type"`
	Content   string                 `json:"content"`
	Metadata  map[string]interface{} `json:"metadata,omitempty"`
	Signature string                 `json:"signature,omitempty"`
}

// TaskRequest 任务请求
type TaskRequest struct {
	TaskID      string                 `json:"task_id"`
	Type        string                 `json:"type"`
	Description string                 `json:"description"`
	Target      string                 `json:"target,omitempty"`
	Payload     map[string]interface{} `json:"payload,omitempty"`
	Signature   string                 `json:"signature,omitempty"`
}

// ReputationRequest 声誉请求
type ReputationRequest struct {
	NodeID    string  `json:"node_id"`
	Delta     float64 `json:"delta,omitempty"`
	Reason    string  `json:"reason,omitempty"`
	Signature string  `json:"signature,omitempty"`
}

// AccusationRequest 指责请求
type AccusationRequest struct {
	Accused   string `json:"accused"`
	Type      string `json:"type"`
	Reason    string `json:"reason"`
	Evidence  string `json:"evidence,omitempty"`
	Signature string `json:"signature,omitempty"`
}

// NeighborRequest 邻居请求
type NeighborRequest struct {
	NodeID    string   `json:"node_id"`
	Addresses []string `json:"addresses,omitempty"`
}

// MailboxMessage 邮箱消息
type MailboxMessage struct {
	ID        string `json:"id"`
	From      string `json:"from"`
	To        string `json:"to"`
	Subject   string `json:"subject"`
	Content   string `json:"content"`
	Timestamp int64  `json:"timestamp"`
	Read      bool   `json:"read"`
}

// MailboxSendRequest 邮箱发送请求
type MailboxSendRequest struct {
	To        string `json:"to"`
	Subject   string `json:"subject"`
	Content   string `json:"content"`
	Encrypted bool   `json:"encrypted,omitempty"`
}

// BulletinMessage 留言板消息
type BulletinMessage struct {
	ID        string `json:"id"`
	Author    string `json:"author"`
	Topic     string `json:"topic"`
	Content   string `json:"content"`
	Timestamp int64  `json:"timestamp"`
	TTL       int64  `json:"ttl"`
}

// BulletinPublishRequest 留言发布请求
type BulletinPublishRequest struct {
	Topic     string `json:"topic"`
	Content   string `json:"content"`
	TTL       int64  `json:"ttl,omitempty"`
	Signature string `json:"signature,omitempty"`
}

// ProposalRequest 提案请求
type ProposalRequest struct {
	Title       string `json:"title"`
	Type        string `json:"type"`
	Description string `json:"description,omitempty"`
	Target      string `json:"target,omitempty"`
}

// VoteRequest 投票请求
type VoteRequest struct {
	ProposalID string `json:"proposal_id"`
	Vote       string `json:"vote"` // yes/no/abstain
}

// SuperNodeApplyRequest 超级节点申请请求
type SuperNodeApplyRequest struct {
	Stake int64 `json:"stake"`
}

// SuperNodeVoteRequest 超级节点投票请求
type SuperNodeVoteRequest struct {
	Candidate string `json:"candidate"`
}

// AuditSubmitRequest 审计提交请求
type AuditSubmitRequest struct {
	Target  string `json:"target"`
	Passed  bool   `json:"passed"`
	Details string `json:"details,omitempty"`
}

// GenesisInviteRequest 创世邀请请求
type GenesisInviteRequest struct {
	ForPubkey string `json:"for_pubkey"`
}

// GenesisJoinRequest 加入网络请求
type GenesisJoinRequest struct {
	Invitation string `json:"invitation"`
	Pubkey     string `json:"pubkey"`
}

// IncentiveAwardRequest 激励奖励请求
type IncentiveAwardRequest struct {
	NodeID   string `json:"node_id"`
	TaskType string `json:"task_type"`
}

// IncentivePropagateRequest 声誉传播请求
type IncentivePropagateRequest struct {
	Target string  `json:"target"`
	Delta  float64 `json:"delta"`
}

// Server HTTP API 服务器
type Server struct {
	mu         sync.RWMutex
	config     *Config
	httpServer *http.Server
	running    bool
	startTime  time.Time
	
	// 处理函数（由外部模块注入）
	handlers   map[string]http.HandlerFunc
	
	// 回调函数
	OnMessageReceived  func(from string, msg *MessageRequest)
	OnTaskReceived     func(from string, task *TaskRequest)
	OnReputationQuery  func(nodeID string) float64
	OnAccusationCreate func(from string, acc *AccusationRequest)
	
	// 数据获取函数
	GetPeersFunc       func() []*PeerInfo
	GetReputationFunc  func(nodeID string) float64
	SendMessageFunc    func(to string, msg *MessageRequest) error
	CreateTaskFunc     func(task *TaskRequest) (string, error)
	CreateAccusation   func(acc *AccusationRequest) (string, error)
	
	// 邻居管理
	GetNeighborsFunc    func(limit int) []*PeerInfo
	GetBestNeighbors    func(count int) []*PeerInfo
	AddNeighborFunc     func(nodeID string, addrs []string) error
	RemoveNeighborFunc  func(nodeID string) error
	PingNeighborFunc    func(nodeID string) (int64, bool)
	
	// 邮箱功能
	MailboxSendFunc     func(to, subject, content string, encrypted bool) (string, error)
	MailboxInboxFunc    func(limit, offset int) ([]*MailboxMessage, int)
	MailboxOutboxFunc   func(limit, offset int) ([]*MailboxMessage, int)
	MailboxReadFunc     func(messageID string) (*MailboxMessage, error)
	MailboxMarkReadFunc func(messageID string) error
	MailboxDeleteFunc   func(messageID string) error
	
	// 留言板功能
	BulletinPublishFunc   func(topic, content string, ttl int64) (string, error)
	BulletinGetFunc       func(messageID string) (*BulletinMessage, error)
	BulletinByTopicFunc   func(topic string, limit int) []*BulletinMessage
	BulletinByAuthorFunc  func(author string, limit int) []*BulletinMessage
	BulletinSearchFunc    func(keyword string, limit int) []*BulletinMessage
	BulletinSubscribeFunc func(topic string) error
	BulletinUnsubscribe   func(topic string) error
	BulletinRevokeFunc    func(messageID string) error
	
	// 投票功能
	VotingCreateFunc    func(title, voteType, desc, target string) (string, error)
	VotingListFunc      func(status string) []map[string]interface{}
	VotingGetFunc       func(proposalID string) (map[string]interface{}, error)
	VotingVoteFunc      func(proposalID, vote string) error
	VotingFinalizeFunc  func(proposalID string) (string, error)
	
	// 超级节点
	SuperNodeListFunc       func() []map[string]interface{}
	SuperNodeCandidatesFunc func() []map[string]interface{}
	SuperNodeApplyFunc      func(stake int64) error
	SuperNodeWithdrawFunc   func() error
	SuperNodeVoteFunc       func(candidate string) error
	SuperNodeStartElection  func() (string, error)
	SuperNodeFinalizeFunc   func(electionID string) ([]string, error)
	SuperNodeAuditSubmit    func(target string, passed bool, details string) (string, error)
	SuperNodeAuditResult    func(target string) (float64, error)
	
	// 创世节点
	GenesisInfoFunc         func() map[string]interface{}
	GenesisCreateInviteFunc func(forPubkey string) (string, error)
	GenesisVerifyInviteFunc func(invitation string) (bool, string, error)
	GenesisJoinFunc         func(invitation, pubkey string) (string, []string, error)
	
	// 激励系统
	IncentiveAwardFunc     func(nodeID, taskType string) (float64, error)
	IncentivePropagateFunc func(target string, delta float64) (int, error)
	IncentiveHistoryFunc   func(nodeID string, limit int) []map[string]interface{}
	IncentiveToleranceFunc func(nodeID string) (int, int)
	
	// 声誉扩展
	ReputationRankingFunc func(limit int) []map[string]interface{}
	ReputationHistoryFunc func(nodeID string, limit int) []map[string]interface{}
	
	// 指责扩展
	AccusationDetailFunc  func(accID string) (map[string]interface{}, error)
	AccusationAnalyzeFunc func(nodeID string) map[string]interface{}
	
	// Token 认证管理器
	tokenManager *TokenManager
}

// NewServer 创建 HTTP API 服务器
func NewServer(config *Config) (*Server, error) {
	if config == nil {
		return nil, ErrNilConfig
	}
	if config.NodeID == "" {
		return nil, ErrEmptyNodeID
	}
	
	// 创建 Token 管理器
	authConfig := &AuthConfig{
		APIToken:       config.APIToken,
		TokenGenerated: config.APIToken != "", // 如果传入了 token，标记为已生成
		AuthEnabled:    config.AuthEnabled,
	}
	tokenManager := NewTokenManager(authConfig)
	
	s := &Server{
		config:       config,
		handlers:     make(map[string]http.HandlerFunc),
		startTime:    time.Now(),
		tokenManager: tokenManager,
	}
	
	return s, nil
}

// Start 启动服务器
func (s *Server) Start() error {
	s.mu.Lock()
	if s.running {
		s.mu.Unlock()
		return nil
	}
	s.running = true
	s.startTime = time.Now()
	s.mu.Unlock()
	
	// 确保 Token 存在
	if s.tokenManager.IsAuthEnabled() {
		token, isNew, err := s.tokenManager.EnsureToken()
		if err != nil {
			return fmt.Errorf("初始化 API Token 失败: %w", err)
		}
		if isNew {
			// 首次生成，打印到控制台
			PrintTokenInfo(token, s.config.ListenAddr)
		}
	}
	
	mux := http.NewServeMux()
	
	// 注册路由
	s.registerRoutes(mux)
	
	// 创建 HTTP 服务器
	s.httpServer = &http.Server{
		Addr:         s.config.ListenAddr,
		Handler:      s.middleware(mux),
		ReadTimeout:  s.config.ReadTimeout,
		WriteTimeout: s.config.WriteTimeout,
	}
	
	go func() {
		if err := s.httpServer.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			fmt.Printf("HTTP server error: %v\n", err)
		}
	}()
	
	return nil
}

// Stop 停止服务器
func (s *Server) Stop() error {
	s.mu.Lock()
	if !s.running {
		s.mu.Unlock()
		return nil
	}
	s.running = false
	s.mu.Unlock()
	
	if s.httpServer != nil {
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		return s.httpServer.Shutdown(ctx)
	}
	
	return nil
}

// GetAPIToken 获取当前 API Token
func (s *Server) GetAPIToken() string {
	if s.tokenManager == nil {
		return ""
	}
	return s.tokenManager.GetToken()
}

// SetAPIToken 设置 API Token
func (s *Server) SetAPIToken(token string) {
	if s.tokenManager != nil {
		s.tokenManager.SetToken(token)
	}
}

// RegenerateAPIToken 重新生成 API Token
func (s *Server) RegenerateAPIToken() (string, error) {
	if s.tokenManager == nil {
		return "", errors.New("token manager not initialized")
	}
	return s.tokenManager.RegenerateToken()
}

// RevokeAPIToken 撤销 API Token
func (s *Server) RevokeAPIToken() {
	if s.tokenManager != nil {
		s.tokenManager.RevokeToken()
	}
}

// GetAuthConfig 获取认证配置
func (s *Server) GetAuthConfig() *AuthConfig {
	if s.tokenManager == nil {
		return nil
	}
	return s.tokenManager.GetConfig()
}

// registerRoutes 注册路由
func (s *Server) registerRoutes(mux *http.ServeMux) {
	// 健康检查
	mux.HandleFunc("/health", s.handleHealth)
	mux.HandleFunc("/status", s.handleStatus)
	
	// 节点管理
	mux.HandleFunc("/api/v1/node/info", s.handleNodeInfo)
	mux.HandleFunc("/api/v1/node/peers", s.handlePeers)
	mux.HandleFunc("/api/v1/node/register", s.handleNodeRegister)
	
	// 邻居管理
	mux.HandleFunc("/api/v1/neighbor/list", s.handleNeighborList)
	mux.HandleFunc("/api/v1/neighbor/best", s.handleNeighborBest)
	mux.HandleFunc("/api/v1/neighbor/add", s.handleNeighborAdd)
	mux.HandleFunc("/api/v1/neighbor/remove", s.handleNeighborRemove)
	mux.HandleFunc("/api/v1/neighbor/ping", s.handleNeighborPing)
	
	// 消息
	mux.HandleFunc("/api/v1/message/send", s.handleSendMessage)
	mux.HandleFunc("/api/v1/message/receive", s.handleReceiveMessage)
	
	// 邮箱
	mux.HandleFunc("/api/v1/mailbox/send", s.handleMailboxSend)
	mux.HandleFunc("/api/v1/mailbox/inbox", s.handleMailboxInbox)
	mux.HandleFunc("/api/v1/mailbox/outbox", s.handleMailboxOutbox)
	mux.HandleFunc("/api/v1/mailbox/read/", s.handleMailboxRead)
	mux.HandleFunc("/api/v1/mailbox/mark-read", s.handleMailboxMarkRead)
	mux.HandleFunc("/api/v1/mailbox/delete", s.handleMailboxDelete)
	
	// 留言板
	mux.HandleFunc("/api/v1/bulletin/publish", s.handleBulletinPublish)
	mux.HandleFunc("/api/v1/bulletin/message/", s.handleBulletinGet)
	mux.HandleFunc("/api/v1/bulletin/topic/", s.handleBulletinByTopic)
	mux.HandleFunc("/api/v1/bulletin/author/", s.handleBulletinByAuthor)
	mux.HandleFunc("/api/v1/bulletin/search", s.handleBulletinSearch)
	mux.HandleFunc("/api/v1/bulletin/subscribe", s.handleBulletinSubscribe)
	mux.HandleFunc("/api/v1/bulletin/unsubscribe", s.handleBulletinUnsubscribe)
	mux.HandleFunc("/api/v1/bulletin/revoke", s.handleBulletinRevoke)
	
	// 任务
	mux.HandleFunc("/api/v1/task/create", s.handleCreateTask)
	mux.HandleFunc("/api/v1/task/status", s.handleTaskStatus)
	mux.HandleFunc("/api/v1/task/accept", s.handleTaskAccept)
	mux.HandleFunc("/api/v1/task/submit", s.handleTaskSubmit)
	mux.HandleFunc("/api/v1/task/list", s.handleTaskList)
	
	// 声誉
	mux.HandleFunc("/api/v1/reputation/query", s.handleReputationQuery)
	mux.HandleFunc("/api/v1/reputation/update", s.handleReputationUpdate)
	mux.HandleFunc("/api/v1/reputation/ranking", s.handleReputationRanking)
	mux.HandleFunc("/api/v1/reputation/history", s.handleReputationHistory)
	
	// 指责
	mux.HandleFunc("/api/v1/accusation/create", s.handleAccusationCreate)
	mux.HandleFunc("/api/v1/accusation/list", s.handleAccusationList)
	mux.HandleFunc("/api/v1/accusation/detail/", s.handleAccusationDetail)
	mux.HandleFunc("/api/v1/accusation/analyze", s.handleAccusationAnalyze)
	
	// 激励
	mux.HandleFunc("/api/v1/incentive/award", s.handleIncentiveAward)
	mux.HandleFunc("/api/v1/incentive/propagate", s.handleIncentivePropagate)
	mux.HandleFunc("/api/v1/incentive/history", s.handleIncentiveHistory)
	mux.HandleFunc("/api/v1/incentive/tolerance", s.handleIncentiveTolerance)
	
	// 投票
	mux.HandleFunc("/api/v1/voting/proposal/create", s.handleVotingCreate)
	mux.HandleFunc("/api/v1/voting/proposal/list", s.handleVotingList)
	mux.HandleFunc("/api/v1/voting/proposal/", s.handleVotingGet)
	mux.HandleFunc("/api/v1/voting/vote", s.handleVotingVote)
	mux.HandleFunc("/api/v1/voting/proposal/finalize", s.handleVotingFinalize)
	
	// 超级节点
	mux.HandleFunc("/api/v1/supernode/list", s.handleSuperNodeList)
	mux.HandleFunc("/api/v1/supernode/candidates", s.handleSuperNodeCandidates)
	mux.HandleFunc("/api/v1/supernode/apply", s.handleSuperNodeApply)
	mux.HandleFunc("/api/v1/supernode/withdraw", s.handleSuperNodeWithdraw)
	mux.HandleFunc("/api/v1/supernode/vote", s.handleSuperNodeVote)
	mux.HandleFunc("/api/v1/supernode/election/start", s.handleSuperNodeElectionStart)
	mux.HandleFunc("/api/v1/supernode/election/finalize", s.handleSuperNodeElectionFinalize)
	mux.HandleFunc("/api/v1/supernode/audit/submit", s.handleSuperNodeAuditSubmit)
	mux.HandleFunc("/api/v1/supernode/audit/result", s.handleSuperNodeAuditResult)
	
	// 创世节点
	mux.HandleFunc("/api/v1/genesis/info", s.handleGenesisInfo)
	mux.HandleFunc("/api/v1/genesis/invite/create", s.handleGenesisInviteCreate)
	mux.HandleFunc("/api/v1/genesis/invite/verify", s.handleGenesisInviteVerify)
	mux.HandleFunc("/api/v1/genesis/join", s.handleGenesisJoin)
	
	// 日志
	mux.HandleFunc("/api/v1/log/submit", s.handleLogSubmit)
	mux.HandleFunc("/api/v1/log/query", s.handleLogQuery)
	mux.HandleFunc("/api/v1/log/export", s.handleLogExport)
	
	// 审计集成
	mux.HandleFunc("/api/v1/audit/deviations", s.handleAuditDeviations)
	mux.HandleFunc("/api/v1/audit/penalty-config", s.handleAuditPenaltyConfig)
	mux.HandleFunc("/api/v1/audit/manual-penalty", s.handleAuditManualPenalty)
	
	// 抵押物管理
	mux.HandleFunc("/api/v1/collateral/list", s.handleCollateralList)
	mux.HandleFunc("/api/v1/collateral/by-node", s.handleCollateralByNode)
	mux.HandleFunc("/api/v1/collateral/slash-by-node", s.handleCollateralSlashByNode)
	mux.HandleFunc("/api/v1/collateral/slash-history", s.handleCollateralSlashHistory)
	
	// 争议预审
	mux.HandleFunc("/api/v1/dispute/list", s.handleDisputeList)
	mux.HandleFunc("/api/v1/dispute/suggestion/", s.handleDisputeSuggestion)
	mux.HandleFunc("/api/v1/dispute/verify-evidence", s.handleDisputeVerifyEvidence)
	mux.HandleFunc("/api/v1/dispute/apply-suggestion", s.handleDisputeApplySuggestion)
	mux.HandleFunc("/api/v1/dispute/detail/", s.handleDisputeDetail)
	
	// 托管多签
	mux.HandleFunc("/api/v1/escrow/list", s.handleEscrowList)
	mux.HandleFunc("/api/v1/escrow/detail/", s.handleEscrowDetail)
	mux.HandleFunc("/api/v1/escrow/arbitrator-signature", s.handleEscrowArbitratorSignature)
	mux.HandleFunc("/api/v1/escrow/signature-count/", s.handleEscrowSignatureCount)
	mux.HandleFunc("/api/v1/escrow/resolve", s.handleEscrowResolve)
	
	// 注册自定义处理函数
	for path, handler := range s.handlers {
		mux.HandleFunc(path, handler)
	}
}

// middleware 中间件
func (s *Server) middleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// CORS
		if s.config.EnableCORS {
			w.Header().Set("Access-Control-Allow-Origin", "*")
			w.Header().Set("Access-Control-Allow-Methods", "GET, POST, PUT, DELETE, OPTIONS")
			w.Header().Set("Access-Control-Allow-Headers", "Content-Type, X-NodeID, X-Signature, X-API-Token")
		}
		
		// 预检请求
		if r.Method == http.MethodOptions {
			w.WriteHeader(http.StatusOK)
			return
		}
		
		// 限制请求体大小
		r.Body = http.MaxBytesReader(w, r.Body, s.config.MaxBodySize)
		
		// 设置 JSON 响应头
		w.Header().Set("Content-Type", "application/json")
		
		// Token 认证（健康检查端点除外）
		if r.URL.Path != "/health" && r.URL.Path != "/status" {
			if s.tokenManager != nil && s.tokenManager.IsAuthEnabled() {
				token := r.Header.Get(TokenHeader)
				if token == "" {
					token = r.URL.Query().Get(TokenQueryParam)
				}
				if !s.tokenManager.ValidateToken(token) {
					w.WriteHeader(http.StatusUnauthorized)
					json.NewEncoder(w).Encode(Response{
						Success: false,
						Error:   "invalid or missing API token",
						Code:    http.StatusUnauthorized,
					})
					return
				}
			}
		}
		
		next.ServeHTTP(w, r)
	})
}

// 响应辅助函数
func (s *Server) writeJSON(w http.ResponseWriter, status int, data interface{}) {
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(Response{
		Success: status >= 200 && status < 300,
		Data:    data,
		Code:    status,
	})
}

func (s *Server) writeError(w http.ResponseWriter, status int, err string) {
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(Response{
		Success: false,
		Error:   err,
		Code:    status,
	})
}

// handleHealth 健康检查
func (s *Server) handleHealth(w http.ResponseWriter, r *http.Request) {
	s.writeJSON(w, http.StatusOK, map[string]interface{}{
		"status":   "ok",
		"node_id":  s.config.NodeID,
		"timestamp": time.Now().Unix(),
	})
}

// handleStatus 状态信息
func (s *Server) handleStatus(w http.ResponseWriter, r *http.Request) {
	s.mu.RLock()
	uptime := time.Since(s.startTime).Seconds()
	s.mu.RUnlock()
	
	s.writeJSON(w, http.StatusOK, map[string]interface{}{
		"node_id":   s.config.NodeID,
		"running":   s.running,
		"uptime_sec": uptime,
		"listen_addr": s.config.ListenAddr,
	})
}

// handleNodeInfo 节点信息
func (s *Server) handleNodeInfo(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		s.writeError(w, http.StatusMethodNotAllowed, "method not allowed")
		return
	}
	
	s.mu.RLock()
	uptime := int64(time.Since(s.startTime).Seconds())
	s.mu.RUnlock()
	
	info := &NodeInfoResponse{
		NodeID:    s.config.NodeID,
		Addresses: []string{s.config.ListenAddr},
		Status:    "online",
		Uptime:    uptime,
		Version:   "1.0.0",
	}
	
	s.writeJSON(w, http.StatusOK, info)
}

// handlePeers 获取对等节点
func (s *Server) handlePeers(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		s.writeError(w, http.StatusMethodNotAllowed, "method not allowed")
		return
	}
	
	var peers []*PeerInfo
	if s.GetPeersFunc != nil {
		peers = s.GetPeersFunc()
	}
	
	if peers == nil {
		peers = []*PeerInfo{}
	}
	
	s.writeJSON(w, http.StatusOK, map[string]interface{}{
		"peers": peers,
		"count": len(peers),
	})
}

// handleSendMessage 发送消息
func (s *Server) handleSendMessage(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		s.writeError(w, http.StatusMethodNotAllowed, "method not allowed")
		return
	}
	
	var req MessageRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		s.writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}
	
	if req.To == "" {
		s.writeError(w, http.StatusBadRequest, "recipient is required")
		return
	}
	
	if s.SendMessageFunc != nil {
		if err := s.SendMessageFunc(req.To, &req); err != nil {
			s.writeError(w, http.StatusInternalServerError, err.Error())
			return
		}
	}
	
	s.writeJSON(w, http.StatusOK, map[string]interface{}{
		"sent": true,
		"to":   req.To,
	})
}

// handleReceiveMessage 接收消息回调
func (s *Server) handleReceiveMessage(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		s.writeError(w, http.StatusMethodNotAllowed, "method not allowed")
		return
	}
	
	var req MessageRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		s.writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}
	
	from := r.Header.Get("X-NodeID")
	if from == "" {
		from = "unknown"
	}
	
	if s.OnMessageReceived != nil {
		s.OnMessageReceived(from, &req)
	}
	
	s.writeJSON(w, http.StatusOK, map[string]interface{}{
		"received": true,
	})
}

// handleCreateTask 创建任务
func (s *Server) handleCreateTask(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		s.writeError(w, http.StatusMethodNotAllowed, "method not allowed")
		return
	}
	
	var req TaskRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		s.writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}
	
	var taskID string
	var err error
	
	if s.CreateTaskFunc != nil {
		taskID, err = s.CreateTaskFunc(&req)
		if err != nil {
			s.writeError(w, http.StatusInternalServerError, err.Error())
			return
		}
	} else {
		taskID = req.TaskID
	}
	
	s.writeJSON(w, http.StatusOK, map[string]interface{}{
		"task_id": taskID,
		"created": true,
	})
}

// handleTaskStatus 任务状态
func (s *Server) handleTaskStatus(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		s.writeError(w, http.StatusMethodNotAllowed, "method not allowed")
		return
	}
	
	taskID := r.URL.Query().Get("task_id")
	if taskID == "" {
		s.writeError(w, http.StatusBadRequest, "task_id is required")
		return
	}
	
	// TODO: 查询实际任务状态
	s.writeJSON(w, http.StatusOK, map[string]interface{}{
		"task_id": taskID,
		"status":  "pending",
	})
}

// handleReputationQuery 查询声誉
func (s *Server) handleReputationQuery(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		s.writeError(w, http.StatusMethodNotAllowed, "method not allowed")
		return
	}
	
	nodeID := r.URL.Query().Get("node_id")
	if nodeID == "" {
		nodeID = s.config.NodeID
	}
	
	var reputation float64 = 50.0
	if s.GetReputationFunc != nil {
		reputation = s.GetReputationFunc(nodeID)
	}
	
	s.writeJSON(w, http.StatusOK, map[string]interface{}{
		"node_id":    nodeID,
		"reputation": reputation,
	})
}

// handleReputationUpdate 更新声誉
func (s *Server) handleReputationUpdate(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		s.writeError(w, http.StatusMethodNotAllowed, "method not allowed")
		return
	}
	
	var req ReputationRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		s.writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}
	
	// TODO: 实际更新声誉
	s.writeJSON(w, http.StatusOK, map[string]interface{}{
		"node_id": req.NodeID,
		"updated": true,
	})
}

// handleAccusationCreate 创建指责
func (s *Server) handleAccusationCreate(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		s.writeError(w, http.StatusMethodNotAllowed, "method not allowed")
		return
	}
	
	var req AccusationRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		s.writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}
	
	if req.Accused == "" {
		s.writeError(w, http.StatusBadRequest, "accused is required")
		return
	}
	
	from := r.Header.Get("X-NodeID")
	if from == "" {
		from = s.config.NodeID
	}
	
	var accusationID string
	if s.CreateAccusation != nil {
		var err error
		accusationID, err = s.CreateAccusation(&req)
		if err != nil {
			s.writeError(w, http.StatusInternalServerError, err.Error())
			return
		}
	}
	
	if s.OnAccusationCreate != nil {
		s.OnAccusationCreate(from, &req)
	}
	
	s.writeJSON(w, http.StatusOK, map[string]interface{}{
		"accusation_id": accusationID,
		"created":       true,
	})
}

// handleAccusationList 列出指责
func (s *Server) handleAccusationList(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		s.writeError(w, http.StatusMethodNotAllowed, "method not allowed")
		return
	}
	
	// TODO: 查询实际指责列表
	s.writeJSON(w, http.StatusOK, map[string]interface{}{
		"accusations": []interface{}{},
		"count":       0,
	})
}

// handleLogSubmit 提交日志
func (s *Server) handleLogSubmit(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		s.writeError(w, http.StatusMethodNotAllowed, "method not allowed")
		return
	}
	
	var logEntry map[string]interface{}
	if err := json.NewDecoder(r.Body).Decode(&logEntry); err != nil {
		s.writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}
	
	// TODO: 存储日志
	s.writeJSON(w, http.StatusOK, map[string]interface{}{
		"submitted": true,
	})
}

// handleLogQuery 查询日志
func (s *Server) handleLogQuery(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		s.writeError(w, http.StatusMethodNotAllowed, "method not allowed")
		return
	}
	
	nodeID := r.URL.Query().Get("node_id")
	eventType := r.URL.Query().Get("event_type")
	limitStr := r.URL.Query().Get("limit")
	
	limit := 100
	if limitStr != "" {
		if l, err := strconv.Atoi(limitStr); err == nil && l > 0 {
			limit = l
		}
	}
	
	// TODO: 查询实际日志
	_ = nodeID
	_ = eventType
	_ = limit
	
	s.writeJSON(w, http.StatusOK, map[string]interface{}{
		"logs":  []interface{}{},
		"count": 0,
	})
}

// RegisterHandler 注册自定义处理函数
func (s *Server) RegisterHandler(path string, handler http.HandlerFunc) {
	s.mu.Lock()
	s.handlers[path] = handler
	s.mu.Unlock()
}

// IsRunning 检查是否运行中
func (s *Server) IsRunning() bool {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.running
}

// GetListenAddr 获取监听地址
func (s *Server) GetListenAddr() string {
	return s.config.ListenAddr
}

// parseBody 解析请求体
func parseBody(r *http.Request, v interface{}) error {
	body, err := io.ReadAll(r.Body)
	if err != nil {
		return err
	}
	return json.Unmarshal(body, v)
}

// getQueryParam 获取查询参数
func getQueryParam(r *http.Request, key string, defaultValue string) string {
	value := r.URL.Query().Get(key)
	if value == "" {
		return defaultValue
	}
	return value
}

// getIntQueryParam 获取整数查询参数
func getIntQueryParam(r *http.Request, key string, defaultValue int) int {
	value := r.URL.Query().Get(key)
	if value == "" {
		return defaultValue
	}
	if i, err := strconv.Atoi(value); err == nil {
		return i
	}
	return defaultValue
}

// validateSignature 验证签名
func (s *Server) validateSignature(r *http.Request, body []byte) bool {
	if s.config.VerifyFunc == nil {
		return true
	}
	
	nodeID := r.Header.Get("X-NodeID")
	signature := r.Header.Get("X-Signature")
	
	if nodeID == "" || signature == "" {
		return false
	}
	
	return s.config.VerifyFunc(nodeID, body, signature)
}

// extractNodeID 从请求中提取节点ID
func extractNodeID(r *http.Request) string {
	nodeID := r.Header.Get("X-NodeID")
	if nodeID != "" {
		return nodeID
	}
	
	// 尝试从 URL 参数获取
	nodeID = r.URL.Query().Get("node_id")
	if nodeID != "" {
		return nodeID
	}
	
	// 尝试从 IP 获取（简化）
	remoteAddr := r.RemoteAddr
	if idx := strings.LastIndex(remoteAddr, ":"); idx > 0 {
		return remoteAddr[:idx]
	}
	
	return remoteAddr
}

// extractPathParam 从URL路径中提取参数
func extractPathParam(r *http.Request, prefix string) string {
	path := r.URL.Path
	if strings.HasPrefix(path, prefix) {
		return strings.TrimPrefix(path, prefix)
	}
	return ""
}

// ============== 节点注册 ==============

func (s *Server) handleNodeRegister(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		s.writeError(w, http.StatusMethodNotAllowed, "method not allowed")
		return
	}
	
	var req struct {
		Pubkey    string `json:"pubkey"`
		Signature string `json:"signature"`
	}
	if err := parseBody(r, &req); err != nil {
		s.writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}
	
	if req.Pubkey == "" {
		s.writeError(w, http.StatusBadRequest, "pubkey required")
		return
	}
	
	// TODO: 实际注册逻辑
	s.writeJSON(w, http.StatusOK, map[string]interface{}{
		"node_id": req.Pubkey[:16] + "...",
		"status":  "registered",
	})
}

// ============== 邻居管理 ==============

func (s *Server) handleNeighborList(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		s.writeError(w, http.StatusMethodNotAllowed, "method not allowed")
		return
	}
	
	limit := getIntQueryParam(r, "limit", 20)
	
	var neighbors []*PeerInfo
	if s.GetNeighborsFunc != nil {
		neighbors = s.GetNeighborsFunc(limit)
	}
	if neighbors == nil {
		neighbors = []*PeerInfo{}
	}
	
	s.writeJSON(w, http.StatusOK, map[string]interface{}{
		"neighbors": neighbors,
		"count":     len(neighbors),
	})
}

func (s *Server) handleNeighborBest(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		s.writeError(w, http.StatusMethodNotAllowed, "method not allowed")
		return
	}
	
	count := getIntQueryParam(r, "count", 3)
	
	var neighbors []*PeerInfo
	if s.GetBestNeighbors != nil {
		neighbors = s.GetBestNeighbors(count)
	}
	if neighbors == nil {
		neighbors = []*PeerInfo{}
	}
	
	s.writeJSON(w, http.StatusOK, map[string]interface{}{
		"neighbors": neighbors,
		"count":     len(neighbors),
	})
}

func (s *Server) handleNeighborAdd(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		s.writeError(w, http.StatusMethodNotAllowed, "method not allowed")
		return
	}
	
	var req NeighborRequest
	if err := parseBody(r, &req); err != nil {
		s.writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}
	
	if req.NodeID == "" {
		s.writeError(w, http.StatusBadRequest, "node_id required")
		return
	}
	
	if s.AddNeighborFunc != nil {
		if err := s.AddNeighborFunc(req.NodeID, req.Addresses); err != nil {
			s.writeError(w, http.StatusInternalServerError, err.Error())
			return
		}
	}
	
	s.writeJSON(w, http.StatusOK, map[string]interface{}{
		"status": "ok",
	})
}

func (s *Server) handleNeighborRemove(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		s.writeError(w, http.StatusMethodNotAllowed, "method not allowed")
		return
	}
	
	var req NeighborRequest
	if err := parseBody(r, &req); err != nil {
		s.writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}
	
	if req.NodeID == "" {
		s.writeError(w, http.StatusBadRequest, "node_id required")
		return
	}
	
	if s.RemoveNeighborFunc != nil {
		if err := s.RemoveNeighborFunc(req.NodeID); err != nil {
			s.writeError(w, http.StatusInternalServerError, err.Error())
			return
		}
	}
	
	s.writeJSON(w, http.StatusOK, map[string]interface{}{
		"status": "ok",
	})
}

func (s *Server) handleNeighborPing(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		s.writeError(w, http.StatusMethodNotAllowed, "method not allowed")
		return
	}
	
	var req NeighborRequest
	if err := parseBody(r, &req); err != nil {
		s.writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}
	
	if req.NodeID == "" {
		s.writeError(w, http.StatusBadRequest, "node_id required")
		return
	}
	
	latency := int64(0)
	online := false
	if s.PingNeighborFunc != nil {
		latency, online = s.PingNeighborFunc(req.NodeID)
	}
	
	s.writeJSON(w, http.StatusOK, map[string]interface{}{
		"latency_ms": latency,
		"online":     online,
	})
}

// ============== 邮箱功能 ==============

func (s *Server) handleMailboxSend(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		s.writeError(w, http.StatusMethodNotAllowed, "method not allowed")
		return
	}
	
	var req MailboxSendRequest
	if err := parseBody(r, &req); err != nil {
		s.writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}
	
	if req.To == "" {
		s.writeError(w, http.StatusBadRequest, "recipient required")
		return
	}
	
	messageID := fmt.Sprintf("msg_%d", time.Now().UnixNano())
	if s.MailboxSendFunc != nil {
		var err error
		messageID, err = s.MailboxSendFunc(req.To, req.Subject, req.Content, req.Encrypted)
		if err != nil {
			s.writeError(w, http.StatusInternalServerError, err.Error())
			return
		}
	}
	
	s.writeJSON(w, http.StatusOK, map[string]interface{}{
		"message_id": messageID,
		"status":     "sent",
	})
}

func (s *Server) handleMailboxInbox(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		s.writeError(w, http.StatusMethodNotAllowed, "method not allowed")
		return
	}
	
	limit := getIntQueryParam(r, "limit", 20)
	offset := getIntQueryParam(r, "offset", 0)
	
	var messages []*MailboxMessage
	total := 0
	if s.MailboxInboxFunc != nil {
		messages, total = s.MailboxInboxFunc(limit, offset)
	}
	if messages == nil {
		messages = []*MailboxMessage{}
	}
	
	s.writeJSON(w, http.StatusOK, map[string]interface{}{
		"messages": messages,
		"total":    total,
	})
}

func (s *Server) handleMailboxOutbox(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		s.writeError(w, http.StatusMethodNotAllowed, "method not allowed")
		return
	}
	
	limit := getIntQueryParam(r, "limit", 20)
	offset := getIntQueryParam(r, "offset", 0)
	
	var messages []*MailboxMessage
	total := 0
	if s.MailboxOutboxFunc != nil {
		messages, total = s.MailboxOutboxFunc(limit, offset)
	}
	if messages == nil {
		messages = []*MailboxMessage{}
	}
	
	s.writeJSON(w, http.StatusOK, map[string]interface{}{
		"messages": messages,
		"total":    total,
	})
}

func (s *Server) handleMailboxRead(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		s.writeError(w, http.StatusMethodNotAllowed, "method not allowed")
		return
	}
	
	messageID := extractPathParam(r, "/api/v1/mailbox/read/")
	if messageID == "" {
		s.writeError(w, http.StatusBadRequest, "message_id required")
		return
	}
	
	if s.MailboxReadFunc != nil {
		msg, err := s.MailboxReadFunc(messageID)
		if err != nil {
			s.writeError(w, http.StatusNotFound, err.Error())
			return
		}
		s.writeJSON(w, http.StatusOK, msg)
		return
	}
	
	s.writeError(w, http.StatusNotFound, "message not found")
}

func (s *Server) handleMailboxMarkRead(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		s.writeError(w, http.StatusMethodNotAllowed, "method not allowed")
		return
	}
	
	var req struct {
		MessageID string `json:"message_id"`
	}
	if err := parseBody(r, &req); err != nil {
		s.writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}
	
	if s.MailboxMarkReadFunc != nil {
		if err := s.MailboxMarkReadFunc(req.MessageID); err != nil {
			s.writeError(w, http.StatusInternalServerError, err.Error())
			return
		}
	}
	
	s.writeJSON(w, http.StatusOK, map[string]interface{}{
		"status": "ok",
	})
}

func (s *Server) handleMailboxDelete(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		s.writeError(w, http.StatusMethodNotAllowed, "method not allowed")
		return
	}
	
	var req struct {
		MessageID string `json:"message_id"`
	}
	if err := parseBody(r, &req); err != nil {
		s.writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}
	
	if s.MailboxDeleteFunc != nil {
		if err := s.MailboxDeleteFunc(req.MessageID); err != nil {
			s.writeError(w, http.StatusInternalServerError, err.Error())
			return
		}
	}
	
	s.writeJSON(w, http.StatusOK, map[string]interface{}{
		"status": "ok",
	})
}

// ============== 留言板功能 ==============

func (s *Server) handleBulletinPublish(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		s.writeError(w, http.StatusMethodNotAllowed, "method not allowed")
		return
	}
	
	var req BulletinPublishRequest
	if err := parseBody(r, &req); err != nil {
		s.writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}
	
	if req.Content == "" {
		s.writeError(w, http.StatusBadRequest, "content required")
		return
	}
	
	messageID := fmt.Sprintf("blt_%d", time.Now().UnixNano())
	if s.BulletinPublishFunc != nil {
		var err error
		messageID, err = s.BulletinPublishFunc(req.Topic, req.Content, req.TTL)
		if err != nil {
			s.writeError(w, http.StatusInternalServerError, err.Error())
			return
		}
	}
	
	s.writeJSON(w, http.StatusOK, map[string]interface{}{
		"message_id": messageID,
		"status":     "published",
	})
}

func (s *Server) handleBulletinGet(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		s.writeError(w, http.StatusMethodNotAllowed, "method not allowed")
		return
	}
	
	messageID := extractPathParam(r, "/api/v1/bulletin/message/")
	if messageID == "" {
		s.writeError(w, http.StatusBadRequest, "message_id required")
		return
	}
	
	if s.BulletinGetFunc != nil {
		msg, err := s.BulletinGetFunc(messageID)
		if err != nil {
			s.writeError(w, http.StatusNotFound, err.Error())
			return
		}
		s.writeJSON(w, http.StatusOK, msg)
		return
	}
	
	s.writeError(w, http.StatusNotFound, "message not found")
}

func (s *Server) handleBulletinByTopic(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		s.writeError(w, http.StatusMethodNotAllowed, "method not allowed")
		return
	}
	
	topic := extractPathParam(r, "/api/v1/bulletin/topic/")
	limit := getIntQueryParam(r, "limit", 20)
	
	var messages []*BulletinMessage
	if s.BulletinByTopicFunc != nil {
		messages = s.BulletinByTopicFunc(topic, limit)
	}
	if messages == nil {
		messages = []*BulletinMessage{}
	}
	
	s.writeJSON(w, http.StatusOK, map[string]interface{}{
		"messages": messages,
		"count":    len(messages),
	})
}

func (s *Server) handleBulletinByAuthor(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		s.writeError(w, http.StatusMethodNotAllowed, "method not allowed")
		return
	}
	
	author := extractPathParam(r, "/api/v1/bulletin/author/")
	limit := getIntQueryParam(r, "limit", 20)
	
	var messages []*BulletinMessage
	if s.BulletinByAuthorFunc != nil {
		messages = s.BulletinByAuthorFunc(author, limit)
	}
	if messages == nil {
		messages = []*BulletinMessage{}
	}
	
	s.writeJSON(w, http.StatusOK, map[string]interface{}{
		"messages": messages,
		"count":    len(messages),
	})
}

func (s *Server) handleBulletinSearch(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		s.writeError(w, http.StatusMethodNotAllowed, "method not allowed")
		return
	}
	
	keyword := getQueryParam(r, "keyword", "")
	limit := getIntQueryParam(r, "limit", 20)
	
	var messages []*BulletinMessage
	if s.BulletinSearchFunc != nil {
		messages = s.BulletinSearchFunc(keyword, limit)
	}
	if messages == nil {
		messages = []*BulletinMessage{}
	}
	
	s.writeJSON(w, http.StatusOK, map[string]interface{}{
		"messages": messages,
		"count":    len(messages),
	})
}

func (s *Server) handleBulletinSubscribe(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		s.writeError(w, http.StatusMethodNotAllowed, "method not allowed")
		return
	}
	
	var req struct {
		Topic string `json:"topic"`
	}
	if err := parseBody(r, &req); err != nil {
		s.writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}
	
	if s.BulletinSubscribeFunc != nil {
		if err := s.BulletinSubscribeFunc(req.Topic); err != nil {
			s.writeError(w, http.StatusInternalServerError, err.Error())
			return
		}
	}
	
	s.writeJSON(w, http.StatusOK, map[string]interface{}{
		"status": "subscribed",
		"topic":  req.Topic,
	})
}

func (s *Server) handleBulletinUnsubscribe(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		s.writeError(w, http.StatusMethodNotAllowed, "method not allowed")
		return
	}
	
	var req struct {
		Topic string `json:"topic"`
	}
	if err := parseBody(r, &req); err != nil {
		s.writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}
	
	if s.BulletinUnsubscribe != nil {
		if err := s.BulletinUnsubscribe(req.Topic); err != nil {
			s.writeError(w, http.StatusInternalServerError, err.Error())
			return
		}
	}
	
	s.writeJSON(w, http.StatusOK, map[string]interface{}{
		"status": "unsubscribed",
		"topic":  req.Topic,
	})
}

func (s *Server) handleBulletinRevoke(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		s.writeError(w, http.StatusMethodNotAllowed, "method not allowed")
		return
	}
	
	var req struct {
		MessageID string `json:"message_id"`
	}
	if err := parseBody(r, &req); err != nil {
		s.writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}
	
	if s.BulletinRevokeFunc != nil {
		if err := s.BulletinRevokeFunc(req.MessageID); err != nil {
			s.writeError(w, http.StatusInternalServerError, err.Error())
			return
		}
	}
	
	s.writeJSON(w, http.StatusOK, map[string]interface{}{
		"status": "revoked",
	})
}

// ============== 任务扩展 ==============

func (s *Server) handleTaskAccept(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		s.writeError(w, http.StatusMethodNotAllowed, "method not allowed")
		return
	}
	
	var req struct {
		TaskID string `json:"task_id"`
	}
	if err := parseBody(r, &req); err != nil {
		s.writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}
	
	s.writeJSON(w, http.StatusOK, map[string]interface{}{
		"status":  "accepted",
		"task_id": req.TaskID,
	})
}

func (s *Server) handleTaskSubmit(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		s.writeError(w, http.StatusMethodNotAllowed, "method not allowed")
		return
	}
	
	var req struct {
		TaskID string `json:"task_id"`
		Result string `json:"result"`
	}
	if err := parseBody(r, &req); err != nil {
		s.writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}
	
	s.writeJSON(w, http.StatusOK, map[string]interface{}{
		"status":  "submitted",
		"task_id": req.TaskID,
	})
}

func (s *Server) handleTaskList(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		s.writeError(w, http.StatusMethodNotAllowed, "method not allowed")
		return
	}
	
	status := getQueryParam(r, "status", "")
	limit := getIntQueryParam(r, "limit", 20)
	_ = status
	_ = limit
	
	s.writeJSON(w, http.StatusOK, map[string]interface{}{
		"tasks": []interface{}{},
		"count": 0,
	})
}

// ============== 声誉扩展 ==============

func (s *Server) handleReputationRanking(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		s.writeError(w, http.StatusMethodNotAllowed, "method not allowed")
		return
	}
	
	limit := getIntQueryParam(r, "limit", 10)
	
	var rankings []map[string]interface{}
	if s.ReputationRankingFunc != nil {
		rankings = s.ReputationRankingFunc(limit)
	}
	if rankings == nil {
		rankings = []map[string]interface{}{}
	}
	
	s.writeJSON(w, http.StatusOK, map[string]interface{}{
		"rankings": rankings,
	})
}

func (s *Server) handleReputationHistory(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		s.writeError(w, http.StatusMethodNotAllowed, "method not allowed")
		return
	}
	
	nodeID := getQueryParam(r, "node_id", s.config.NodeID)
	limit := getIntQueryParam(r, "limit", 20)
	
	var history []map[string]interface{}
	if s.ReputationHistoryFunc != nil {
		history = s.ReputationHistoryFunc(nodeID, limit)
	}
	if history == nil {
		history = []map[string]interface{}{}
	}
	
	s.writeJSON(w, http.StatusOK, map[string]interface{}{
		"node_id": nodeID,
		"history": history,
	})
}

// ============== 指责扩展 ==============

func (s *Server) handleAccusationDetail(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		s.writeError(w, http.StatusMethodNotAllowed, "method not allowed")
		return
	}
	
	accID := extractPathParam(r, "/api/v1/accusation/detail/")
	if accID == "" {
		s.writeError(w, http.StatusBadRequest, "accusation_id required")
		return
	}
	
	if s.AccusationDetailFunc != nil {
		detail, err := s.AccusationDetailFunc(accID)
		if err != nil {
			s.writeError(w, http.StatusNotFound, err.Error())
			return
		}
		s.writeJSON(w, http.StatusOK, detail)
		return
	}
	
	s.writeError(w, http.StatusNotFound, "accusation not found")
}

func (s *Server) handleAccusationAnalyze(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		s.writeError(w, http.StatusMethodNotAllowed, "method not allowed")
		return
	}
	
	nodeID := getQueryParam(r, "node_id", "")
	if nodeID == "" {
		s.writeError(w, http.StatusBadRequest, "node_id required")
		return
	}
	
	var analysis map[string]interface{}
	if s.AccusationAnalyzeFunc != nil {
		analysis = s.AccusationAnalyzeFunc(nodeID)
	}
	if analysis == nil {
		analysis = map[string]interface{}{
			"node_id":        nodeID,
			"total_received": 0,
			"credibility":    1.0,
		}
	}
	
	s.writeJSON(w, http.StatusOK, analysis)
}

// ============== 激励系统 ==============

func (s *Server) handleIncentiveAward(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		s.writeError(w, http.StatusMethodNotAllowed, "method not allowed")
		return
	}
	
	var req IncentiveAwardRequest
	if err := parseBody(r, &req); err != nil {
		s.writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}
	
	reward := 0.0
	if s.IncentiveAwardFunc != nil {
		var err error
		reward, err = s.IncentiveAwardFunc(req.NodeID, req.TaskType)
		if err != nil {
			s.writeError(w, http.StatusInternalServerError, err.Error())
			return
		}
	}
	
	s.writeJSON(w, http.StatusOK, map[string]interface{}{
		"reward": reward,
	})
}

func (s *Server) handleIncentivePropagate(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		s.writeError(w, http.StatusMethodNotAllowed, "method not allowed")
		return
	}
	
	var req IncentivePropagateRequest
	if err := parseBody(r, &req); err != nil {
		s.writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}
	
	propagatedTo := 0
	if s.IncentivePropagateFunc != nil {
		var err error
		propagatedTo, err = s.IncentivePropagateFunc(req.Target, req.Delta)
		if err != nil {
			s.writeError(w, http.StatusInternalServerError, err.Error())
			return
		}
	}
	
	s.writeJSON(w, http.StatusOK, map[string]interface{}{
		"propagated_to": propagatedTo,
	})
}

func (s *Server) handleIncentiveHistory(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		s.writeError(w, http.StatusMethodNotAllowed, "method not allowed")
		return
	}
	
	nodeID := getQueryParam(r, "node_id", s.config.NodeID)
	limit := getIntQueryParam(r, "limit", 20)
	
	var rewards []map[string]interface{}
	if s.IncentiveHistoryFunc != nil {
		rewards = s.IncentiveHistoryFunc(nodeID, limit)
	}
	if rewards == nil {
		rewards = []map[string]interface{}{}
	}
	
	s.writeJSON(w, http.StatusOK, map[string]interface{}{
		"rewards": rewards,
	})
}

func (s *Server) handleIncentiveTolerance(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		s.writeError(w, http.StatusMethodNotAllowed, "method not allowed")
		return
	}
	
	nodeID := getQueryParam(r, "node_id", s.config.NodeID)
	
	tolerance, maxTolerance := 0, 10
	if s.IncentiveToleranceFunc != nil {
		tolerance, maxTolerance = s.IncentiveToleranceFunc(nodeID)
	}
	
	s.writeJSON(w, http.StatusOK, map[string]interface{}{
		"node_id":   nodeID,
		"tolerance": tolerance,
		"max":       maxTolerance,
	})
}

// ============== 投票系统 ==============

func (s *Server) handleVotingCreate(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		s.writeError(w, http.StatusMethodNotAllowed, "method not allowed")
		return
	}
	
	var req ProposalRequest
	if err := parseBody(r, &req); err != nil {
		s.writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}
	
	if req.Title == "" {
		s.writeError(w, http.StatusBadRequest, "title required")
		return
	}
	
	proposalID := fmt.Sprintf("prop_%d", time.Now().UnixNano())
	if s.VotingCreateFunc != nil {
		var err error
		proposalID, err = s.VotingCreateFunc(req.Title, req.Type, req.Description, req.Target)
		if err != nil {
			s.writeError(w, http.StatusInternalServerError, err.Error())
			return
		}
	}
	
	s.writeJSON(w, http.StatusOK, map[string]interface{}{
		"proposal_id": proposalID,
		"status":      "created",
	})
}

func (s *Server) handleVotingList(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		s.writeError(w, http.StatusMethodNotAllowed, "method not allowed")
		return
	}
	
	status := getQueryParam(r, "status", "")
	
	var proposals []map[string]interface{}
	if s.VotingListFunc != nil {
		proposals = s.VotingListFunc(status)
	}
	if proposals == nil {
		proposals = []map[string]interface{}{}
	}
	
	s.writeJSON(w, http.StatusOK, map[string]interface{}{
		"proposals": proposals,
	})
}

func (s *Server) handleVotingGet(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		s.writeError(w, http.StatusMethodNotAllowed, "method not allowed")
		return
	}
	
	proposalID := extractPathParam(r, "/api/v1/voting/proposal/")
	if proposalID == "" || proposalID == "create" || proposalID == "list" || proposalID == "finalize" {
		s.writeError(w, http.StatusBadRequest, "proposal_id required")
		return
	}
	
	if s.VotingGetFunc != nil {
		proposal, err := s.VotingGetFunc(proposalID)
		if err != nil {
			s.writeError(w, http.StatusNotFound, err.Error())
			return
		}
		s.writeJSON(w, http.StatusOK, proposal)
		return
	}
	
	s.writeError(w, http.StatusNotFound, "proposal not found")
}

func (s *Server) handleVotingVote(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		s.writeError(w, http.StatusMethodNotAllowed, "method not allowed")
		return
	}
	
	var req VoteRequest
	if err := parseBody(r, &req); err != nil {
		s.writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}
	
	if req.ProposalID == "" {
		s.writeError(w, http.StatusBadRequest, "proposal_id required")
		return
	}
	
	if s.VotingVoteFunc != nil {
		if err := s.VotingVoteFunc(req.ProposalID, req.Vote); err != nil {
			s.writeError(w, http.StatusInternalServerError, err.Error())
			return
		}
	}
	
	s.writeJSON(w, http.StatusOK, map[string]interface{}{
		"status": "voted",
	})
}

func (s *Server) handleVotingFinalize(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		s.writeError(w, http.StatusMethodNotAllowed, "method not allowed")
		return
	}
	
	var req struct {
		ProposalID string `json:"proposal_id"`
	}
	if err := parseBody(r, &req); err != nil {
		s.writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}
	
	result := "unknown"
	if s.VotingFinalizeFunc != nil {
		var err error
		result, err = s.VotingFinalizeFunc(req.ProposalID)
		if err != nil {
			s.writeError(w, http.StatusInternalServerError, err.Error())
			return
		}
	}
	
	s.writeJSON(w, http.StatusOK, map[string]interface{}{
		"result": result,
	})
}

// ============== 超级节点 ==============

func (s *Server) handleSuperNodeList(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		s.writeError(w, http.StatusMethodNotAllowed, "method not allowed")
		return
	}
	
	var supernodes []map[string]interface{}
	if s.SuperNodeListFunc != nil {
		supernodes = s.SuperNodeListFunc()
	}
	if supernodes == nil {
		supernodes = []map[string]interface{}{}
	}
	
	s.writeJSON(w, http.StatusOK, map[string]interface{}{
		"supernodes": supernodes,
	})
}

func (s *Server) handleSuperNodeCandidates(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		s.writeError(w, http.StatusMethodNotAllowed, "method not allowed")
		return
	}
	
	var candidates []map[string]interface{}
	if s.SuperNodeCandidatesFunc != nil {
		candidates = s.SuperNodeCandidatesFunc()
	}
	if candidates == nil {
		candidates = []map[string]interface{}{}
	}
	
	s.writeJSON(w, http.StatusOK, map[string]interface{}{
		"candidates": candidates,
	})
}

func (s *Server) handleSuperNodeApply(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		s.writeError(w, http.StatusMethodNotAllowed, "method not allowed")
		return
	}
	
	var req SuperNodeApplyRequest
	if err := parseBody(r, &req); err != nil {
		s.writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}
	
	if s.SuperNodeApplyFunc != nil {
		if err := s.SuperNodeApplyFunc(req.Stake); err != nil {
			s.writeError(w, http.StatusInternalServerError, err.Error())
			return
		}
	}
	
	s.writeJSON(w, http.StatusOK, map[string]interface{}{
		"status": "applied",
	})
}

func (s *Server) handleSuperNodeWithdraw(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		s.writeError(w, http.StatusMethodNotAllowed, "method not allowed")
		return
	}
	
	if s.SuperNodeWithdrawFunc != nil {
		if err := s.SuperNodeWithdrawFunc(); err != nil {
			s.writeError(w, http.StatusInternalServerError, err.Error())
			return
		}
	}
	
	s.writeJSON(w, http.StatusOK, map[string]interface{}{
		"status": "withdrawn",
	})
}

func (s *Server) handleSuperNodeVote(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		s.writeError(w, http.StatusMethodNotAllowed, "method not allowed")
		return
	}
	
	var req SuperNodeVoteRequest
	if err := parseBody(r, &req); err != nil {
		s.writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}
	
	if req.Candidate == "" {
		s.writeError(w, http.StatusBadRequest, "candidate required")
		return
	}
	
	if s.SuperNodeVoteFunc != nil {
		if err := s.SuperNodeVoteFunc(req.Candidate); err != nil {
			s.writeError(w, http.StatusInternalServerError, err.Error())
			return
		}
	}
	
	s.writeJSON(w, http.StatusOK, map[string]interface{}{
		"status": "voted",
	})
}

func (s *Server) handleSuperNodeElectionStart(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		s.writeError(w, http.StatusMethodNotAllowed, "method not allowed")
		return
	}
	
	electionID := fmt.Sprintf("elec_%d", time.Now().UnixNano())
	if s.SuperNodeStartElection != nil {
		var err error
		electionID, err = s.SuperNodeStartElection()
		if err != nil {
			s.writeError(w, http.StatusInternalServerError, err.Error())
			return
		}
	}
	
	s.writeJSON(w, http.StatusOK, map[string]interface{}{
		"election_id": electionID,
		"status":      "started",
	})
}

func (s *Server) handleSuperNodeElectionFinalize(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		s.writeError(w, http.StatusMethodNotAllowed, "method not allowed")
		return
	}
	
	var req struct {
		ElectionID string `json:"election_id"`
	}
	if err := parseBody(r, &req); err != nil {
		s.writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}
	
	var elected []string
	if s.SuperNodeFinalizeFunc != nil {
		var err error
		elected, err = s.SuperNodeFinalizeFunc(req.ElectionID)
		if err != nil {
			s.writeError(w, http.StatusInternalServerError, err.Error())
			return
		}
	}
	if elected == nil {
		elected = []string{}
	}
	
	s.writeJSON(w, http.StatusOK, map[string]interface{}{
		"elected": elected,
		"status":  "finalized",
	})
}

func (s *Server) handleSuperNodeAuditSubmit(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		s.writeError(w, http.StatusMethodNotAllowed, "method not allowed")
		return
	}
	
	var req AuditSubmitRequest
	if err := parseBody(r, &req); err != nil {
		s.writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}
	
	if req.Target == "" {
		s.writeError(w, http.StatusBadRequest, "target required")
		return
	}
	
	auditID := fmt.Sprintf("audit_%d", time.Now().UnixNano())
	if s.SuperNodeAuditSubmit != nil {
		var err error
		auditID, err = s.SuperNodeAuditSubmit(req.Target, req.Passed, req.Details)
		if err != nil {
			s.writeError(w, http.StatusInternalServerError, err.Error())
			return
		}
	}
	
	s.writeJSON(w, http.StatusOK, map[string]interface{}{
		"audit_id": auditID,
		"status":   "submitted",
	})
}

func (s *Server) handleSuperNodeAuditResult(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		s.writeError(w, http.StatusMethodNotAllowed, "method not allowed")
		return
	}
	
	target := getQueryParam(r, "target", "")
	if target == "" {
		s.writeError(w, http.StatusBadRequest, "target required")
		return
	}
	
	passRate := 0.0
	if s.SuperNodeAuditResult != nil {
		var err error
		passRate, err = s.SuperNodeAuditResult(target)
		if err != nil {
			s.writeError(w, http.StatusInternalServerError, err.Error())
			return
		}
	}
	
	s.writeJSON(w, http.StatusOK, map[string]interface{}{
		"target":    target,
		"pass_rate": passRate,
	})
}

// ============== 创世节点 ==============

func (s *Server) handleGenesisInfo(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		s.writeError(w, http.StatusMethodNotAllowed, "method not allowed")
		return
	}
	
	var info map[string]interface{}
	if s.GenesisInfoFunc != nil {
		info = s.GenesisInfoFunc()
	}
	if info == nil {
		info = map[string]interface{}{
			"genesis_id": "unknown",
			"created_at": time.Now().Format(time.RFC3339),
		}
	}
	
	s.writeJSON(w, http.StatusOK, info)
}

func (s *Server) handleGenesisInviteCreate(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		s.writeError(w, http.StatusMethodNotAllowed, "method not allowed")
		return
	}
	
	var req GenesisInviteRequest
	if err := parseBody(r, &req); err != nil {
		s.writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}
	
	invitationID := fmt.Sprintf("inv_%d", time.Now().UnixNano())
	if s.GenesisCreateInviteFunc != nil {
		var err error
		invitationID, err = s.GenesisCreateInviteFunc(req.ForPubkey)
		if err != nil {
			s.writeError(w, http.StatusInternalServerError, err.Error())
			return
		}
	}
	
	s.writeJSON(w, http.StatusOK, map[string]interface{}{
		"invitation_id": invitationID,
		"status":        "created",
	})
}

func (s *Server) handleGenesisInviteVerify(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		s.writeError(w, http.StatusMethodNotAllowed, "method not allowed")
		return
	}
	
	var req struct {
		Invitation string `json:"invitation"`
	}
	if err := parseBody(r, &req); err != nil {
		s.writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}
	
	valid := false
	inviter := ""
	if s.GenesisVerifyInviteFunc != nil {
		var err error
		valid, inviter, err = s.GenesisVerifyInviteFunc(req.Invitation)
		if err != nil {
			s.writeError(w, http.StatusInternalServerError, err.Error())
			return
		}
	}
	
	s.writeJSON(w, http.StatusOK, map[string]interface{}{
		"valid":   valid,
		"inviter": inviter,
	})
}

func (s *Server) handleGenesisJoin(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		s.writeError(w, http.StatusMethodNotAllowed, "method not allowed")
		return
	}
	
	var req GenesisJoinRequest
	if err := parseBody(r, &req); err != nil {
		s.writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}
	
	if req.Invitation == "" || req.Pubkey == "" {
		s.writeError(w, http.StatusBadRequest, "invitation and pubkey required")
		return
	}
	
	nodeID := ""
	var neighbors []string
	if s.GenesisJoinFunc != nil {
		var err error
		nodeID, neighbors, err = s.GenesisJoinFunc(req.Invitation, req.Pubkey)
		if err != nil {
			s.writeError(w, http.StatusInternalServerError, err.Error())
			return
		}
	}
	if neighbors == nil {
		neighbors = []string{}
	}
	
	s.writeJSON(w, http.StatusOK, map[string]interface{}{
		"node_id":   nodeID,
		"neighbors": neighbors,
		"status":    "joined",
	})
}

// ============== 日志扩展 ==============

func (s *Server) handleLogExport(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		s.writeError(w, http.StatusMethodNotAllowed, "method not allowed")
		return
	}
	
	format := getQueryParam(r, "format", "json")
	_ = format
	
	s.writeJSON(w, http.StatusOK, map[string]interface{}{
		"file":   "logs_export.json",
		"status": "exported",
	})
}

// ============== 审计集成 ==============

// AuditDeviation 审计偏离记录
type AuditDeviation struct {
	AuditID    string `json:"audit_id"`
	AuditorID  string `json:"auditor_id"`
	TargetID   string `json:"target_id"`
	Expected   bool   `json:"expected"`
	Actual     bool   `json:"actual"`
	Severity   string `json:"severity"`
	Timestamp  int64  `json:"timestamp"`
}

// PenaltyConfig 惩罚配置
type PenaltyConfig struct {
	Severity    string  `json:"severity"`
	RepPenalty  float64 `json:"rep_penalty"`
	SlashRatio  float64 `json:"slash_ratio"`
}

func (s *Server) handleAuditDeviations(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		s.writeError(w, http.StatusMethodNotAllowed, "method not allowed")
		return
	}
	
	limit := getIntQueryParam(r, "limit", 20)
	
	// 返回模拟数据，实际应从审计模块获取
	deviations := []AuditDeviation{}
	_ = limit
	
	s.writeJSON(w, http.StatusOK, map[string]interface{}{
		"deviations": deviations,
		"total":      0,
	})
}

func (s *Server) handleAuditPenaltyConfig(w http.ResponseWriter, r *http.Request) {
	if r.Method == http.MethodGet {
		// 获取惩罚配置
		config := map[string]PenaltyConfig{
			"minor":  {Severity: "minor", RepPenalty: 5, SlashRatio: 0.1},
			"severe": {Severity: "severe", RepPenalty: 20, SlashRatio: 0.3},
		}
		s.writeJSON(w, http.StatusOK, config)
		return
	}
	
	if r.Method == http.MethodPost {
		var req PenaltyConfig
		if err := parseBody(r, &req); err != nil {
			s.writeError(w, http.StatusBadRequest, "invalid request body")
			return
		}
		
		s.writeJSON(w, http.StatusOK, map[string]interface{}{
			"status":  "updated",
			"config":  req,
		})
		return
	}
	
	s.writeError(w, http.StatusMethodNotAllowed, "method not allowed")
}

func (s *Server) handleAuditManualPenalty(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		s.writeError(w, http.StatusMethodNotAllowed, "method not allowed")
		return
	}
	
	var req struct {
		NodeID   string `json:"node_id"`
		Severity string `json:"severity"`
		Reason   string `json:"reason"`
	}
	if err := parseBody(r, &req); err != nil {
		s.writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}
	
	if req.NodeID == "" {
		s.writeError(w, http.StatusBadRequest, "node_id required")
		return
	}
	
	s.writeJSON(w, http.StatusOK, map[string]interface{}{
		"penalty_applied": true,
		"node_id":         req.NodeID,
		"rep_delta":       -5.0,
		"slashed":         100,
	})
}

// ============== 抵押物管理 ==============

// Collateral 抵押物
type Collateral struct {
	ID        string  `json:"id"`
	NodeID    string  `json:"node_id"`
	Purpose   string  `json:"purpose"`
	Amount    float64 `json:"amount"`
	Slashed   float64 `json:"slashed"`
	Status    string  `json:"status"`
	CreatedAt int64   `json:"created_at"`
}

func (s *Server) handleCollateralList(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		s.writeError(w, http.StatusMethodNotAllowed, "method not allowed")
		return
	}
	
	status := getQueryParam(r, "status", "")
	_ = status
	
	collaterals := []Collateral{}
	
	s.writeJSON(w, http.StatusOK, map[string]interface{}{
		"collaterals": collaterals,
		"total":       0,
	})
}

func (s *Server) handleCollateralByNode(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		s.writeError(w, http.StatusMethodNotAllowed, "method not allowed")
		return
	}
	
	nodeID := getQueryParam(r, "node_id", "")
	purpose := getQueryParam(r, "purpose", "")
	
	if nodeID == "" {
		s.writeError(w, http.StatusBadRequest, "node_id required")
		return
	}
	
	s.writeJSON(w, http.StatusOK, map[string]interface{}{
		"collateral_id": "coll-" + nodeID[:8],
		"node_id":       nodeID,
		"purpose":       purpose,
		"amount":        1000.0,
		"slashed":       0.0,
		"status":        "active",
	})
}

func (s *Server) handleCollateralSlashByNode(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		s.writeError(w, http.StatusMethodNotAllowed, "method not allowed")
		return
	}
	
	var req struct {
		NodeID   string  `json:"node_id"`
		Purpose  string  `json:"purpose"`
		Ratio    float64 `json:"ratio"`
		Reason   string  `json:"reason"`
		Evidence string  `json:"evidence"`
	}
	if err := parseBody(r, &req); err != nil {
		s.writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}
	
	if req.NodeID == "" || req.Purpose == "" {
		s.writeError(w, http.StatusBadRequest, "node_id and purpose required")
		return
	}
	
	slashedAmount := 1000.0 * req.Ratio
	
	s.writeJSON(w, http.StatusOK, map[string]interface{}{
		"slashed_amount": slashedAmount,
		"remaining":      1000.0 - slashedAmount,
		"status":         "slashed",
	})
}

func (s *Server) handleCollateralSlashHistory(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		s.writeError(w, http.StatusMethodNotAllowed, "method not allowed")
		return
	}
	
	nodeID := getQueryParam(r, "node_id", "")
	limit := getIntQueryParam(r, "limit", 20)
	_, _ = nodeID, limit
	
	history := []map[string]interface{}{}
	
	s.writeJSON(w, http.StatusOK, map[string]interface{}{
		"history": history,
		"total":   0,
	})
}

// ============== 争议预审 ==============

// Dispute 争议记录
type Dispute struct {
	ID         string   `json:"id"`
	Plaintiff  string   `json:"plaintiff"`
	Defendant  string   `json:"defendant"`
	Status     string   `json:"status"`
	Evidence   []string `json:"evidence"`
	CreatedAt  int64    `json:"created_at"`
}

// DisputeSuggestion 争议解决建议
type DisputeSuggestion struct {
	Resolution      string   `json:"suggested_resolution"`
	Confidence      float64  `json:"confidence"`
	CanAutoExecute  bool     `json:"can_auto_execute"`
	MissingEvidence []string `json:"missing_evidence"`
	Warnings        []string `json:"warnings"`
}

func (s *Server) handleDisputeList(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		s.writeError(w, http.StatusMethodNotAllowed, "method not allowed")
		return
	}
	
	status := getQueryParam(r, "status", "")
	_ = status
	
	disputes := []Dispute{}
	
	s.writeJSON(w, http.StatusOK, map[string]interface{}{
		"disputes": disputes,
		"total":    0,
	})
}

func (s *Server) handleDisputeSuggestion(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		s.writeError(w, http.StatusMethodNotAllowed, "method not allowed")
		return
	}
	
	// 从URL提取争议ID
	disputeID := strings.TrimPrefix(r.URL.Path, "/api/v1/dispute/suggestion/")
	if disputeID == "" {
		s.writeError(w, http.StatusBadRequest, "dispute_id required")
		return
	}
	
	suggestion := DisputeSuggestion{
		Resolution:      "favor_plaintiff",
		Confidence:      0.85,
		CanAutoExecute:  false,
		MissingEvidence: []string{"delivery_proof"},
		Warnings:        []string{"证据未全部验证"},
	}
	
	s.writeJSON(w, http.StatusOK, suggestion)
}

func (s *Server) handleDisputeVerifyEvidence(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		s.writeError(w, http.StatusMethodNotAllowed, "method not allowed")
		return
	}
	
	var req struct {
		DisputeID  string `json:"dispute_id"`
		EvidenceID string `json:"evidence_id"`
		VerifierID string `json:"verifier_id"`
	}
	if err := parseBody(r, &req); err != nil {
		s.writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}
	
	s.writeJSON(w, http.StatusOK, map[string]interface{}{
		"verified":    true,
		"dispute_id":  req.DisputeID,
		"evidence_id": req.EvidenceID,
	})
}

func (s *Server) handleDisputeApplySuggestion(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		s.writeError(w, http.StatusMethodNotAllowed, "method not allowed")
		return
	}
	
	var req struct {
		DisputeID  string `json:"dispute_id"`
		ApproverID string `json:"approver_id"`
	}
	if err := parseBody(r, &req); err != nil {
		s.writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}
	
	s.writeJSON(w, http.StatusOK, map[string]interface{}{
		"applied":    true,
		"resolution": "favor_plaintiff",
		"dispute_id": req.DisputeID,
	})
}

func (s *Server) handleDisputeDetail(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		s.writeError(w, http.StatusMethodNotAllowed, "method not allowed")
		return
	}
	
	disputeID := strings.TrimPrefix(r.URL.Path, "/api/v1/dispute/detail/")
	if disputeID == "" {
		s.writeError(w, http.StatusBadRequest, "dispute_id required")
		return
	}
	
	dispute := Dispute{
		ID:        disputeID,
		Plaintiff: "node-A",
		Defendant: "node-B",
		Status:    "pending",
		Evidence:  []string{},
		CreatedAt: time.Now().Unix(),
	}
	
	s.writeJSON(w, http.StatusOK, dispute)
}

// ============== 托管多签 ==============

// Escrow 托管记录
type Escrow struct {
	ID          string  `json:"id"`
	Amount      float64 `json:"amount"`
	Depositor   string  `json:"depositor"`
	Beneficiary string  `json:"beneficiary"`
	Status      string  `json:"status"`
	CreatedAt   int64   `json:"created_at"`
}

func (s *Server) handleEscrowList(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		s.writeError(w, http.StatusMethodNotAllowed, "method not allowed")
		return
	}
	
	status := getQueryParam(r, "status", "")
	_ = status
	
	escrows := []Escrow{}
	
	s.writeJSON(w, http.StatusOK, map[string]interface{}{
		"escrows": escrows,
		"total":   0,
	})
}

func (s *Server) handleEscrowDetail(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		s.writeError(w, http.StatusMethodNotAllowed, "method not allowed")
		return
	}
	
	escrowID := strings.TrimPrefix(r.URL.Path, "/api/v1/escrow/detail/")
	if escrowID == "" {
		s.writeError(w, http.StatusBadRequest, "escrow_id required")
		return
	}
	
	escrow := Escrow{
		ID:          escrowID,
		Amount:      1000,
		Depositor:   "node-A",
		Beneficiary: "node-B",
		Status:      "active",
		CreatedAt:   time.Now().Unix(),
	}
	
	s.writeJSON(w, http.StatusOK, escrow)
}

func (s *Server) handleEscrowArbitratorSignature(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		s.writeError(w, http.StatusMethodNotAllowed, "method not allowed")
		return
	}
	
	var req struct {
		EscrowID    string `json:"escrow_id"`
		ArbitratorID string `json:"arbitrator_id"`
		Signature   string `json:"signature"`
	}
	if err := parseBody(r, &req); err != nil {
		s.writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}
	
	s.writeJSON(w, http.StatusOK, map[string]interface{}{
		"submitted":     true,
		"current_count": 1,
		"required":      2,
	})
}

func (s *Server) handleEscrowSignatureCount(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		s.writeError(w, http.StatusMethodNotAllowed, "method not allowed")
		return
	}
	
	escrowID := strings.TrimPrefix(r.URL.Path, "/api/v1/escrow/signature-count/")
	if escrowID == "" {
		s.writeError(w, http.StatusBadRequest, "escrow_id required")
		return
	}
	
	s.writeJSON(w, http.StatusOK, map[string]interface{}{
		"escrow_id":     escrowID,
		"current_count": 1,
		"required":      2,
		"signers":       []string{"arb-001"},
	})
}

func (s *Server) handleEscrowResolve(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		s.writeError(w, http.StatusMethodNotAllowed, "method not allowed")
		return
	}
	
	var req struct {
		EscrowID   string            `json:"escrow_id"`
		Winner     string            `json:"winner"`
		Signatures map[string]string `json:"signatures"`
	}
	if err := parseBody(r, &req); err != nil {
		s.writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}
	
	if req.EscrowID == "" || req.Winner == "" {
		s.writeError(w, http.StatusBadRequest, "escrow_id and winner required")
		return
	}
	
	s.writeJSON(w, http.StatusOK, map[string]interface{}{
		"resolved":  true,
		"winner":    req.Winner,
		"amount":    1000.0,
		"escrow_id": req.EscrowID,
	})
}
