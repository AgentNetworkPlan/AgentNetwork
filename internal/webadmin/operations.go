package webadmin

import (
	"encoding/json"
	"net/http"
	"strconv"
	"strings"
)

// OperationsProvider 节点操作接口
type OperationsProvider interface {
	// 邻居管理
	GetNeighbors() ([]*NeighborInfo, error)
	GetBestNeighbors(count int) ([]*NeighborInfo, error)
	AddNeighbor(nodeID string, addresses []string) error
	RemoveNeighbor(nodeID string) error
	PingNeighbor(nodeID string) (*PingResult, error)

	// 邮箱操作
	SendMail(to, subject, content string) (*SendMailResult, error)
	GetInbox(limit, offset int) (*MailboxResponse, error)
	GetOutbox(limit, offset int) (*MailboxResponse, error)
	ReadMail(messageID string) (*MailMessage, error)
	MarkMailRead(messageID string) error
	DeleteMail(messageID string) error

	// 留言板操作
	PublishBulletin(topic, content string, ttl int) (*PublishResult, error)
	GetBulletinByTopic(topic string, limit int) ([]*BulletinMessage, error)
	GetBulletinByAuthor(author string, limit int) ([]*BulletinMessage, error)
	SearchBulletin(keyword string, limit int) ([]*BulletinMessage, error)
	SubscribeTopic(topic string) error
	UnsubscribeTopic(topic string) error
	RevokeBulletin(messageID string) error
	GetSubscriptions() ([]string, error)

	// 声誉查询
	GetReputation(nodeID string) (*ReputationInfo, error)
	GetReputationRanking(limit int) ([]*ReputationInfo, error)

	// 消息发送
	SendDirectMessage(to, msgType, content string) (*SendMessageResult, error)
	BroadcastMessage(content string) (*BroadcastResult, error)
}

// NeighborInfo 邻居信息
type NeighborInfo struct {
	NodeID       string  `json:"node_id"`
	PublicKey    string  `json:"public_key,omitempty"`
	Type         string  `json:"type"`
	Reputation   int64   `json:"reputation"`
	TrustScore   float64 `json:"trust_score"`
	Status       string  `json:"status"`
	LastSeen     string  `json:"last_seen,omitempty"`
	Addresses    []string `json:"addresses,omitempty"`
	SuccessPings int     `json:"success_pings"`
	FailedPings  int     `json:"failed_pings"`
}

// PingResult ping结果
type PingResult struct {
	NodeID    string `json:"node_id"`
	Online    bool   `json:"online"`
	LatencyMs int64  `json:"latency_ms"`
	Error     string `json:"error,omitempty"`
}

// SendMailResult 发送邮件结果
type SendMailResult struct {
	MessageID string `json:"message_id"`
	Status    string `json:"status"`
}

// MailboxResponse 邮箱响应
type MailboxResponse struct {
	Messages []*MailSummary `json:"messages"`
	Total    int            `json:"total"`
	Offset   int            `json:"offset"`
	Limit    int            `json:"limit"`
}

// MailSummary 邮件摘要
type MailSummary struct {
	ID        string `json:"id"`
	From      string `json:"from"`
	To        string `json:"to"`
	Subject   string `json:"subject"`
	Timestamp string `json:"timestamp"`
	Status    string `json:"status"`
	Encrypted bool   `json:"encrypted"`
}

// MailMessage 完整邮件
type MailMessage struct {
	ID        string `json:"id"`
	From      string `json:"from"`
	To        string `json:"to"`
	Subject   string `json:"subject"`
	Content   string `json:"content"`
	Timestamp string `json:"timestamp"`
	Status    string `json:"status"`
	Encrypted bool   `json:"encrypted"`
	ReadAt    string `json:"read_at,omitempty"`
}

// PublishResult 发布结果
type PublishResult struct {
	MessageID string `json:"message_id"`
	Topic     string `json:"topic"`
	Status    string `json:"status"`
}

// BulletinMessage 公告消息
type BulletinMessage struct {
	MessageID   string   `json:"message_id"`
	Author      string   `json:"author"`
	Topic       string   `json:"topic"`
	Content     string   `json:"content"`
	Preview     string   `json:"preview,omitempty"`
	Timestamp   string   `json:"timestamp"`
	ExpiresAt   string   `json:"expires_at,omitempty"`
	Status      string   `json:"status"`
	Tags        []string `json:"tags,omitempty"`
	ReplyTo     string   `json:"reply_to,omitempty"`
	Reputation  float64  `json:"reputation"`
}

// ReputationInfo 声誉信息
type ReputationInfo struct {
	NodeID     string  `json:"node_id"`
	Reputation float64 `json:"reputation"`
	Rank       int     `json:"rank,omitempty"`
}

// SendMessageResult 发送消息结果
type SendMessageResult struct {
	MessageID string `json:"message_id"`
	Status    string `json:"status"`
}

// BroadcastResult 广播结果
type BroadcastResult struct {
	MessageID   string `json:"message_id"`
	ReachedCount int   `json:"reached_count"`
}

// OperationHandlers 操作处理器
type OperationHandlers struct {
	server   *Server
	provider OperationsProvider
}

// NewOperationHandlers 创建操作处理器
func NewOperationHandlers(server *Server, provider OperationsProvider) *OperationHandlers {
	return &OperationHandlers{
		server:   server,
		provider: provider,
	}
}

// SetProvider 设置操作提供者
func (h *OperationHandlers) SetProvider(provider OperationsProvider) {
	h.provider = provider
}

// getProvider 获取当前的操作提供者（优先使用 server 的）
func (h *OperationHandlers) getProvider() OperationsProvider {
	if h.server != nil {
		h.server.mu.RLock()
		p := h.server.opsProvider
		h.server.mu.RUnlock()
		if p != nil {
			return p
		}
	}
	return h.provider
}

// ======================== 邻居管理处理器 ========================

// HandleNeighborList 获取邻居列表
func (h *OperationHandlers) HandleNeighborList(w http.ResponseWriter, r *http.Request) {
	provider := h.getProvider()
	if provider == nil {
		WriteError(w, http.StatusServiceUnavailable, "Operations provider not available")
		return
	}

	neighbors, err := provider.GetNeighbors()
	if err != nil {
		WriteError(w, http.StatusInternalServerError, err.Error())
		return
	}

	WriteJSON(w, http.StatusOK, map[string]interface{}{
		"neighbors": neighbors,
		"count":     len(neighbors),
	})
}

// HandleNeighborBest 获取最佳邻居
func (h *OperationHandlers) HandleNeighborBest(w http.ResponseWriter, r *http.Request) {
	provider := h.getProvider()
	if provider == nil {
		WriteError(w, http.StatusServiceUnavailable, "Operations provider not available")
		return
	}

	count := 5
	if c := r.URL.Query().Get("count"); c != "" {
		if n, err := strconv.Atoi(c); err == nil && n > 0 {
			count = n
		}
	}

	neighbors, err := provider.GetBestNeighbors(count)
	if err != nil {
		WriteError(w, http.StatusInternalServerError, err.Error())
		return
	}

	WriteJSON(w, http.StatusOK, map[string]interface{}{
		"neighbors": neighbors,
		"count":     len(neighbors),
	})
}

// HandleNeighborAdd 添加邻居
func (h *OperationHandlers) HandleNeighborAdd(w http.ResponseWriter, r *http.Request) {
	provider := h.getProvider()
	if provider == nil {
		WriteError(w, http.StatusServiceUnavailable, "Operations provider not available")
		return
	}

	var req struct {
		NodeID    string   `json:"node_id"`
		Addresses []string `json:"addresses"`
	}

	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		WriteError(w, http.StatusBadRequest, "Invalid request body")
		return
	}

	if req.NodeID == "" {
		WriteError(w, http.StatusBadRequest, "node_id is required")
		return
	}

	if err := provider.AddNeighbor(req.NodeID, req.Addresses); err != nil {
		WriteError(w, http.StatusInternalServerError, err.Error())
		return
	}

	WriteJSON(w, http.StatusOK, map[string]string{"status": "ok"})
}

// HandleNeighborRemove 移除邻居
func (h *OperationHandlers) HandleNeighborRemove(w http.ResponseWriter, r *http.Request) {
	provider := h.getProvider()
	if provider == nil {
		WriteError(w, http.StatusServiceUnavailable, "Operations provider not available")
		return
	}

	var req struct {
		NodeID string `json:"node_id"`
	}

	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		WriteError(w, http.StatusBadRequest, "Invalid request body")
		return
	}

	if req.NodeID == "" {
		WriteError(w, http.StatusBadRequest, "node_id is required")
		return
	}

	if err := provider.RemoveNeighbor(req.NodeID); err != nil {
		WriteError(w, http.StatusInternalServerError, err.Error())
		return
	}

	WriteJSON(w, http.StatusOK, map[string]string{"status": "ok"})
}

// HandleNeighborPing ping邻居
func (h *OperationHandlers) HandleNeighborPing(w http.ResponseWriter, r *http.Request) {
	provider := h.getProvider()
	if provider == nil {
		WriteError(w, http.StatusServiceUnavailable, "Operations provider not available")
		return
	}

	var req struct {
		NodeID string `json:"node_id"`
	}

	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		WriteError(w, http.StatusBadRequest, "Invalid request body")
		return
	}

	if req.NodeID == "" {
		WriteError(w, http.StatusBadRequest, "node_id is required")
		return
	}

	result, err := provider.PingNeighbor(req.NodeID)
	if err != nil {
		WriteError(w, http.StatusInternalServerError, err.Error())
		return
	}

	WriteJSON(w, http.StatusOK, result)
}

// ======================== 邮箱处理器 ========================

// HandleMailboxSend 发送邮件
func (h *OperationHandlers) HandleMailboxSend(w http.ResponseWriter, r *http.Request) {
	provider := h.getProvider()
	if provider == nil {
		WriteError(w, http.StatusServiceUnavailable, "Operations provider not available")
		return
	}

	var req struct {
		To      string `json:"to"`
		Subject string `json:"subject"`
		Content string `json:"content"`
	}

	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		WriteError(w, http.StatusBadRequest, "Invalid request body")
		return
	}

	if req.To == "" || req.Content == "" {
		WriteError(w, http.StatusBadRequest, "to and content are required")
		return
	}

	result, err := provider.SendMail(req.To, req.Subject, req.Content)
	if err != nil {
		WriteError(w, http.StatusInternalServerError, err.Error())
		return
	}

	WriteJSON(w, http.StatusOK, result)
}

// HandleMailboxInbox 获取收件箱
func (h *OperationHandlers) HandleMailboxInbox(w http.ResponseWriter, r *http.Request) {
	provider := h.getProvider()
	if provider == nil {
		WriteError(w, http.StatusServiceUnavailable, "Operations provider not available")
		return
	}

	limit, offset := parsePagination(r)

	result, err := provider.GetInbox(limit, offset)
	if err != nil {
		WriteError(w, http.StatusInternalServerError, err.Error())
		return
	}

	WriteJSON(w, http.StatusOK, result)
}

// HandleMailboxOutbox 获取发件箱
func (h *OperationHandlers) HandleMailboxOutbox(w http.ResponseWriter, r *http.Request) {
	provider := h.getProvider()
	if provider == nil {
		WriteError(w, http.StatusServiceUnavailable, "Operations provider not available")
		return
	}

	limit, offset := parsePagination(r)

	result, err := provider.GetOutbox(limit, offset)
	if err != nil {
		WriteError(w, http.StatusInternalServerError, err.Error())
		return
	}

	WriteJSON(w, http.StatusOK, result)
}

// HandleMailboxRead 读取邮件
func (h *OperationHandlers) HandleMailboxRead(w http.ResponseWriter, r *http.Request) {
	provider := h.getProvider()
	if provider == nil {
		WriteError(w, http.StatusServiceUnavailable, "Operations provider not available")
		return
	}

	// 从URL路径中提取消息ID: /api/mailbox/read/{id}
	path := r.URL.Path
	parts := strings.Split(path, "/")
	if len(parts) < 5 {
		WriteError(w, http.StatusBadRequest, "message_id is required")
		return
	}
	messageID := parts[len(parts)-1]

	message, err := provider.ReadMail(messageID)
	if err != nil {
		WriteError(w, http.StatusInternalServerError, err.Error())
		return
	}

	WriteJSON(w, http.StatusOK, message)
}

// HandleMailboxMarkRead 标记已读
func (h *OperationHandlers) HandleMailboxMarkRead(w http.ResponseWriter, r *http.Request) {
	provider := h.getProvider()
	if provider == nil {
		WriteError(w, http.StatusServiceUnavailable, "Operations provider not available")
		return
	}

	var req struct {
		MessageID string `json:"message_id"`
	}

	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		WriteError(w, http.StatusBadRequest, "Invalid request body")
		return
	}

	if req.MessageID == "" {
		WriteError(w, http.StatusBadRequest, "message_id is required")
		return
	}

	if err := provider.MarkMailRead(req.MessageID); err != nil {
		WriteError(w, http.StatusInternalServerError, err.Error())
		return
	}

	WriteJSON(w, http.StatusOK, map[string]string{"status": "ok"})
}

// HandleMailboxDelete 删除邮件
func (h *OperationHandlers) HandleMailboxDelete(w http.ResponseWriter, r *http.Request) {
	provider := h.getProvider()
	if provider == nil {
		WriteError(w, http.StatusServiceUnavailable, "Operations provider not available")
		return
	}

	var req struct {
		MessageID string `json:"message_id"`
	}

	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		WriteError(w, http.StatusBadRequest, "Invalid request body")
		return
	}

	if req.MessageID == "" {
		WriteError(w, http.StatusBadRequest, "message_id is required")
		return
	}

	if err := provider.DeleteMail(req.MessageID); err != nil {
		WriteError(w, http.StatusInternalServerError, err.Error())
		return
	}

	WriteJSON(w, http.StatusOK, map[string]string{"status": "ok"})
}

// ======================== 留言板处理器 ========================

// HandleBulletinPublish 发布留言
func (h *OperationHandlers) HandleBulletinPublish(w http.ResponseWriter, r *http.Request) {
	provider := h.getProvider()
	if provider == nil {
		WriteError(w, http.StatusServiceUnavailable, "Operations provider not available")
		return
	}

	var req struct {
		Topic   string `json:"topic"`
		Content string `json:"content"`
		TTL     int    `json:"ttl"`
	}

	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		WriteError(w, http.StatusBadRequest, "Invalid request body")
		return
	}

	if req.Topic == "" || req.Content == "" {
		WriteError(w, http.StatusBadRequest, "topic and content are required")
		return
	}

	result, err := provider.PublishBulletin(req.Topic, req.Content, req.TTL)
	if err != nil {
		WriteError(w, http.StatusInternalServerError, err.Error())
		return
	}

	WriteJSON(w, http.StatusOK, result)
}

// HandleBulletinByTopic 按话题获取留言
func (h *OperationHandlers) HandleBulletinByTopic(w http.ResponseWriter, r *http.Request) {
	provider := h.getProvider()
	if provider == nil {
		WriteError(w, http.StatusServiceUnavailable, "Operations provider not available")
		return
	}

	// 从URL路径中提取topic: /api/bulletin/topic/{topic}
	path := r.URL.Path
	parts := strings.Split(path, "/")
	if len(parts) < 5 {
		WriteError(w, http.StatusBadRequest, "topic is required")
		return
	}
	topic := parts[len(parts)-1]

	limit := 20
	if l := r.URL.Query().Get("limit"); l != "" {
		if n, err := strconv.Atoi(l); err == nil && n > 0 {
			limit = n
		}
	}

	messages, err := provider.GetBulletinByTopic(topic, limit)
	if err != nil {
		WriteError(w, http.StatusInternalServerError, err.Error())
		return
	}

	WriteJSON(w, http.StatusOK, map[string]interface{}{
		"messages": messages,
		"count":    len(messages),
		"topic":    topic,
	})
}

// HandleBulletinByAuthor 按作者获取留言
func (h *OperationHandlers) HandleBulletinByAuthor(w http.ResponseWriter, r *http.Request) {
	provider := h.getProvider()
	if provider == nil {
		WriteError(w, http.StatusServiceUnavailable, "Operations provider not available")
		return
	}

	// 从URL路径中提取author: /api/bulletin/author/{author}
	path := r.URL.Path
	parts := strings.Split(path, "/")
	if len(parts) < 5 {
		WriteError(w, http.StatusBadRequest, "author is required")
		return
	}
	author := parts[len(parts)-1]

	limit := 20
	if l := r.URL.Query().Get("limit"); l != "" {
		if n, err := strconv.Atoi(l); err == nil && n > 0 {
			limit = n
		}
	}

	messages, err := provider.GetBulletinByAuthor(author, limit)
	if err != nil {
		WriteError(w, http.StatusInternalServerError, err.Error())
		return
	}

	WriteJSON(w, http.StatusOK, map[string]interface{}{
		"messages": messages,
		"count":    len(messages),
		"author":   author,
	})
}

// HandleBulletinSearch 搜索留言
func (h *OperationHandlers) HandleBulletinSearch(w http.ResponseWriter, r *http.Request) {
	provider := h.getProvider()
	if provider == nil {
		WriteError(w, http.StatusServiceUnavailable, "Operations provider not available")
		return
	}

	keyword := r.URL.Query().Get("keyword")
	if keyword == "" {
		WriteError(w, http.StatusBadRequest, "keyword is required")
		return
	}

	limit := 20
	if l := r.URL.Query().Get("limit"); l != "" {
		if n, err := strconv.Atoi(l); err == nil && n > 0 {
			limit = n
		}
	}

	messages, err := provider.SearchBulletin(keyword, limit)
	if err != nil {
		WriteError(w, http.StatusInternalServerError, err.Error())
		return
	}

	WriteJSON(w, http.StatusOK, map[string]interface{}{
		"messages": messages,
		"count":    len(messages),
		"keyword":  keyword,
	})
}

// HandleBulletinSubscribe 订阅话题
func (h *OperationHandlers) HandleBulletinSubscribe(w http.ResponseWriter, r *http.Request) {
	provider := h.getProvider()
	if provider == nil {
		WriteError(w, http.StatusServiceUnavailable, "Operations provider not available")
		return
	}

	var req struct {
		Topic string `json:"topic"`
	}

	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		WriteError(w, http.StatusBadRequest, "Invalid request body")
		return
	}

	if req.Topic == "" {
		WriteError(w, http.StatusBadRequest, "topic is required")
		return
	}

	if err := provider.SubscribeTopic(req.Topic); err != nil {
		WriteError(w, http.StatusInternalServerError, err.Error())
		return
	}

	WriteJSON(w, http.StatusOK, map[string]string{"status": "subscribed"})
}

// HandleBulletinUnsubscribe 取消订阅话题
func (h *OperationHandlers) HandleBulletinUnsubscribe(w http.ResponseWriter, r *http.Request) {
	provider := h.getProvider()
	if provider == nil {
		WriteError(w, http.StatusServiceUnavailable, "Operations provider not available")
		return
	}

	var req struct {
		Topic string `json:"topic"`
	}

	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		WriteError(w, http.StatusBadRequest, "Invalid request body")
		return
	}

	if req.Topic == "" {
		WriteError(w, http.StatusBadRequest, "topic is required")
		return
	}

	if err := provider.UnsubscribeTopic(req.Topic); err != nil {
		WriteError(w, http.StatusInternalServerError, err.Error())
		return
	}

	WriteJSON(w, http.StatusOK, map[string]string{"status": "unsubscribed"})
}

// HandleBulletinRevoke 撤回留言
func (h *OperationHandlers) HandleBulletinRevoke(w http.ResponseWriter, r *http.Request) {
	provider := h.getProvider()
	if provider == nil {
		WriteError(w, http.StatusServiceUnavailable, "Operations provider not available")
		return
	}

	var req struct {
		MessageID string `json:"message_id"`
	}

	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		WriteError(w, http.StatusBadRequest, "Invalid request body")
		return
	}

	if req.MessageID == "" {
		WriteError(w, http.StatusBadRequest, "message_id is required")
		return
	}

	if err := provider.RevokeBulletin(req.MessageID); err != nil {
		WriteError(w, http.StatusInternalServerError, err.Error())
		return
	}

	WriteJSON(w, http.StatusOK, map[string]string{"status": "revoked"})
}

// HandleBulletinSubscriptions 获取订阅列表
func (h *OperationHandlers) HandleBulletinSubscriptions(w http.ResponseWriter, r *http.Request) {
	provider := h.getProvider()
	if provider == nil {
		WriteError(w, http.StatusServiceUnavailable, "Operations provider not available")
		return
	}

	topics, err := provider.GetSubscriptions()
	if err != nil {
		WriteError(w, http.StatusInternalServerError, err.Error())
		return
	}

	WriteJSON(w, http.StatusOK, map[string]interface{}{
		"topics": topics,
		"count":  len(topics),
	})
}

// ======================== 声誉处理器 ========================

// HandleReputationQuery 查询声誉
func (h *OperationHandlers) HandleReputationQuery(w http.ResponseWriter, r *http.Request) {
	provider := h.getProvider()
	if provider == nil {
		WriteError(w, http.StatusServiceUnavailable, "Operations provider not available")
		return
	}

	nodeID := r.URL.Query().Get("node_id")
	if nodeID == "" {
		WriteError(w, http.StatusBadRequest, "node_id is required")
		return
	}

	rep, err := provider.GetReputation(nodeID)
	if err != nil {
		WriteError(w, http.StatusInternalServerError, err.Error())
		return
	}

	WriteJSON(w, http.StatusOK, rep)
}

// HandleReputationRanking 获取声誉排行
func (h *OperationHandlers) HandleReputationRanking(w http.ResponseWriter, r *http.Request) {
	provider := h.getProvider()
	if provider == nil {
		WriteError(w, http.StatusServiceUnavailable, "Operations provider not available")
		return
	}

	limit := 10
	if l := r.URL.Query().Get("limit"); l != "" {
		if n, err := strconv.Atoi(l); err == nil && n > 0 {
			limit = n
		}
	}

	rankings, err := provider.GetReputationRanking(limit)
	if err != nil {
		WriteError(w, http.StatusInternalServerError, err.Error())
		return
	}

	WriteJSON(w, http.StatusOK, map[string]interface{}{
		"rankings": rankings,
		"count":    len(rankings),
	})
}

// ======================== 消息处理器 ========================

// HandleMessageSend 发送直接消息
func (h *OperationHandlers) HandleMessageSend(w http.ResponseWriter, r *http.Request) {
	provider := h.getProvider()
	if provider == nil {
		WriteError(w, http.StatusServiceUnavailable, "Operations provider not available")
		return
	}

	var req struct {
		To      string `json:"to"`
		Type    string `json:"type"`
		Content string `json:"content"`
	}

	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		WriteError(w, http.StatusBadRequest, "Invalid request body")
		return
	}

	if req.To == "" || req.Content == "" {
		WriteError(w, http.StatusBadRequest, "to and content are required")
		return
	}

	if req.Type == "" {
		req.Type = "text"
	}

	result, err := provider.SendDirectMessage(req.To, req.Type, req.Content)
	if err != nil {
		WriteError(w, http.StatusInternalServerError, err.Error())
		return
	}

	WriteJSON(w, http.StatusOK, result)
}

// HandleMessageBroadcast 广播消息
func (h *OperationHandlers) HandleMessageBroadcast(w http.ResponseWriter, r *http.Request) {
	provider := h.getProvider()
	if provider == nil {
		WriteError(w, http.StatusServiceUnavailable, "Operations provider not available")
		return
	}

	var req struct {
		Content string `json:"content"`
	}

	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		WriteError(w, http.StatusBadRequest, "Invalid request body")
		return
	}

	if req.Content == "" {
		WriteError(w, http.StatusBadRequest, "content is required")
		return
	}

	result, err := provider.BroadcastMessage(req.Content)
	if err != nil {
		WriteError(w, http.StatusInternalServerError, err.Error())
		return
	}

	WriteJSON(w, http.StatusOK, result)
}

// ======================== 安全相关处理器 ========================

// HandleSecurityStatus 获取安全状态
func (h *OperationHandlers) HandleSecurityStatus(w http.ResponseWriter, r *http.Request) {
	provider := h.getProvider()
	if provider == nil {
		WriteError(w, http.StatusServiceUnavailable, "Operations provider not available")
		return
	}

	// 检查是否是 RealOperationsProvider
	realProvider, ok := provider.(*RealOperationsProvider)
	if !ok {
		WriteJSON(w, http.StatusOK, map[string]interface{}{
			"enabled": false,
			"message": "Security manager not available in mock mode",
		})
		return
	}

	status := realProvider.GetRateLimitStatus()
	WriteJSON(w, http.StatusOK, status)
}

// HandleSecurityReport 获取安全报告
func (h *OperationHandlers) HandleSecurityReport(w http.ResponseWriter, r *http.Request) {
	provider := h.getProvider()
	if provider == nil {
		WriteError(w, http.StatusServiceUnavailable, "Operations provider not available")
		return
	}

	// 检查是否是 RealOperationsProvider
	realProvider, ok := provider.(*RealOperationsProvider)
	if !ok {
		WriteJSON(w, http.StatusOK, map[string]interface{}{
			"enabled": false,
			"message": "Security manager not available in mock mode",
		})
		return
	}

	report := realProvider.GetSecurityReport()
	if report == nil {
		WriteJSON(w, http.StatusOK, map[string]interface{}{
			"enabled": false,
			"message": "Security manager not configured",
		})
		return
	}

	WriteJSON(w, http.StatusOK, report)
}

// ======================== 辅助函数 ========================

func parsePagination(r *http.Request) (limit, offset int) {
	limit = 20
	offset = 0

	if l := r.URL.Query().Get("limit"); l != "" {
		if n, err := strconv.Atoi(l); err == nil && n > 0 && n <= 100 {
			limit = n
		}
	}

	if o := r.URL.Query().Get("offset"); o != "" {
		if n, err := strconv.Atoi(o); err == nil && n >= 0 {
			offset = n
		}
	}

	return
}
