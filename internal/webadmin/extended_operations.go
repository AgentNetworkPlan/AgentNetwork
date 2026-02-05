package webadmin

import (
	"encoding/json"
	"net/http"
	"strconv"
	"strings"
)

// ExtendedOperationsProvider 扩展操作接口 - 支持完整的 Task09 API
type ExtendedOperationsProvider interface {
	OperationsProvider // 嵌入基础接口

	// 声誉扩展
	UpdateReputation(nodeID string, delta int, reason string) (*ReputationUpdateResult, error)
	GetReputationHistory(nodeID string, limit int) ([]*ReputationHistoryEntry, error)

	// 任务管理
	CreateTask(taskType, description string, deadline int64) (*TaskCreateResult, error)
	GetTaskStatus(taskID string) (*TaskStatus, error)
	AcceptTask(taskID string) error
	SubmitTaskResult(taskID, result string) error
	ListTasks(status string, limit int) ([]*TaskInfo, error)

	// 指责系统
	CreateAccusation(accused, accusationType, reason string) (*AccusationCreateResult, error)
	ListAccusations(accused string, limit int) ([]*AccusationInfo, error)
	GetAccusationDetail(id string) (*AccusationDetail, error)
	AnalyzeAccusations(nodeID string) (*AccusationAnalysis, error)

	// 激励系统
	AwardIncentive(nodeID, taskType string) (*IncentiveAward, error)
	PropagateReputation(target string, delta int) (*PropagateResult, error)
	GetIncentiveHistory(nodeID string, limit int) ([]*IncentiveRecord, error)
	GetTolerance(nodeID string) (*ToleranceInfo, error)

	// 投票系统
	CreateProposal(title, proposalType, target string) (*ProposalCreateResult, error)
	ListProposals(status string) ([]*ProposalInfo, error)
	GetProposal(id string) (*ProposalDetail, error)
	Vote(proposalID, vote string) error
	FinalizeProposal(proposalID string) (*ProposalResult, error)

	// 超级节点
	ListSupernodes() ([]*SupernodeInfo, error)
	ListCandidates() ([]*CandidateInfo, error)
	ApplyCandidate(stake int64) error
	WithdrawCandidate() error
	VoteCandidate(candidate string) error
	StartElection() (*ElectionInfo, error)
	FinalizeElection(electionID string) (*ElectionResult, error)
	SubmitAudit(target string, passed bool) (*AuditSubmitResult, error)
	GetAuditResult(target string) (*AuditResultInfo, error)

	// 创世节点
	GetGenesisInfo() (*GenesisInfo, error)
	CreateInvite(forPubkey string) (*InviteCreateResult, error)
	VerifyInvite(invitation string) (*InviteVerifyResult, error)
	JoinNetwork(invitation, pubkey string) (*JoinResult, error)

	// 日志系统
	SubmitLog(eventType string, data map[string]interface{}) (*LogSubmitResult, error)
	QueryLogs(nodeID, eventType string, limit int) ([]*LogEntryInfo, error)
	ExportLogs(format string, start, end int64) (*LogExportResult, error)

	// 审计集成 (Task44)
	GetAuditDeviations(limit int) ([]*AuditDeviationInfo, error)
	GetAuditPenaltyConfig() (*AuditPenaltyConfigInfo, error)
	SetAuditPenaltyConfig(severity string, repPenalty int, slashRatio float64) error
	ManualPenalty(nodeID, severity, reason string) (*ManualPenaltyResult, error)

	// 抵押物管理 (Task44)
	ListCollaterals(status string) ([]*CollateralInfo, error)
	GetCollateralByNode(nodeID, purpose string) (*CollateralInfo, error)
	SlashByNode(nodeID, purpose, reason, evidence string, ratio float64) (*SlashResult, error)
	GetSlashHistory(nodeID string, limit int) ([]*SlashRecord, error)

	// 争议预审 (Task44)
	ListDisputes(status string) ([]*DisputeInfo, error)
	GetDisputeSuggestion(id string) (*DisputeSuggestionInfo, error)
	VerifyEvidence(disputeID, evidenceID, verifierID string) error
	ApplyDisputeSuggestion(disputeID, approverID string) (*ApplySuggestionResult, error)
	GetDisputeDetail(id string) (*DisputeDetail, error)

	// 托管多签 (Task44)
	ListEscrows(status string) ([]*EscrowInfo, error)
	GetEscrowDetail(id string) (*EscrowDetail, error)
	SubmitArbitratorSignature(escrowID, arbitratorID, signature string) (*SignatureSubmitResult, error)
	GetSignatureCount(escrowID string) (*SignatureCountInfo, error)
	ResolveEscrow(escrowID, winner string, signatures map[string]string) (*EscrowResolveResult, error)
}

// ========== 数据结构定义 ==========

// ReputationUpdateResult 声誉更新结果
type ReputationUpdateResult struct {
	NewReputation float64 `json:"new_reputation"`
}

// ReputationHistoryEntry 声誉历史条目
type ReputationHistoryEntry struct {
	Timestamp string  `json:"timestamp"`
	Delta     int     `json:"delta"`
	Reason    string  `json:"reason"`
	NewValue  float64 `json:"new_value"`
}

// TaskCreateResult 任务创建结果
type TaskCreateResult struct {
	TaskID string `json:"task_id"`
}

// TaskStatus 任务状态
type TaskStatus struct {
	TaskID   string `json:"task_id"`
	Status   string `json:"status"`
	Progress int    `json:"progress"`
	Worker   string `json:"worker,omitempty"`
}

// TaskInfo 任务信息
type TaskInfo struct {
	TaskID      string `json:"task_id"`
	Type        string `json:"type"`
	Description string `json:"description"`
	Status      string `json:"status"`
	Creator     string `json:"creator"`
	Worker      string `json:"worker,omitempty"`
	Deadline    int64  `json:"deadline"`
	CreatedAt   string `json:"created_at"`
}

// AccusationCreateResult 指责创建结果
type AccusationCreateResult struct {
	AccusationID string `json:"accusation_id"`
}

// AccusationInfo 指责信息
type AccusationInfo struct {
	ID        string `json:"id"`
	Accuser   string `json:"accuser"`
	Accused   string `json:"accused"`
	Type      string `json:"type"`
	Reason    string `json:"reason"`
	Status    string `json:"status"`
	Timestamp string `json:"timestamp"`
}

// AccusationDetail 指责详情
type AccusationDetail struct {
	AccusationInfo
	Evidence   []string `json:"evidence"`
	Resolution string   `json:"resolution,omitempty"`
	ResolvedAt string   `json:"resolved_at,omitempty"`
}

// AccusationAnalysis 指责分析
type AccusationAnalysis struct {
	NodeID        string  `json:"node_id"`
	TotalReceived int     `json:"total_received"`
	Credibility   float64 `json:"credibility"`
	TrendScore    float64 `json:"trend_score"`
}

// IncentiveAward 激励奖励
type IncentiveAward struct {
	Reward float64 `json:"reward"`
}

// PropagateResult 传播结果
type PropagateResult struct {
	PropagatedTo int `json:"propagated_to"`
}

// IncentiveRecord 激励记录
type IncentiveRecord struct {
	Timestamp string  `json:"timestamp"`
	Type      string  `json:"type"`
	Amount    float64 `json:"amount"`
	Reason    string  `json:"reason"`
}

// ToleranceInfo 耐受值信息
type ToleranceInfo struct {
	Tolerance int `json:"tolerance"`
	Max       int `json:"max"`
}

// ProposalCreateResult 提案创建结果
type ProposalCreateResult struct {
	ProposalID string `json:"proposal_id"`
}

// ProposalInfo 提案信息
type ProposalInfo struct {
	ID        string `json:"id"`
	Title     string `json:"title"`
	Type      string `json:"type"`
	Target    string `json:"target,omitempty"`
	Status    string `json:"status"`
	Creator   string `json:"creator"`
	CreatedAt string `json:"created_at"`
	VotesYes  int    `json:"votes_yes"`
	VotesNo   int    `json:"votes_no"`
}

// ProposalDetail 提案详情
type ProposalDetail struct {
	ProposalInfo
	Description string            `json:"description"`
	Votes       map[string]string `json:"votes"`
	ExpiresAt   string            `json:"expires_at"`
}

// ProposalResult 提案结果
type ProposalResult struct {
	Result string `json:"result"`
}

// SupernodeInfo 超级节点信息
type SupernodeInfo struct {
	NodeID    string `json:"node_id"`
	Term      int    `json:"term"`
	ElectedAt string `json:"elected_at"`
	Status    string `json:"status"`
}

// CandidateInfo 候选人信息
type CandidateInfo struct {
	NodeID    string  `json:"node_id"`
	Stake     int64   `json:"stake"`
	Votes     int     `json:"votes"`
	AppliedAt string  `json:"applied_at"`
	Score     float64 `json:"score"`
}

// ElectionInfo 选举信息
type ElectionInfo struct {
	ElectionID string `json:"election_id"`
	Status     string `json:"status"`
	StartedAt  string `json:"started_at"`
}

// ElectionResult 选举结果
type ElectionResult struct {
	Elected []string `json:"elected"`
}

// AuditSubmitResult 审计提交结果
type AuditSubmitResult struct {
	AuditID string `json:"audit_id"`
}

// AuditResultInfo 审计结果信息
type AuditResultInfo struct {
	Target   string  `json:"target"`
	PassRate float64 `json:"pass_rate"`
	Total    int     `json:"total"`
	Passed   int     `json:"passed"`
}

// GenesisInfo 创世节点信息
type GenesisInfo struct {
	GenesisID   string `json:"genesis_id"`
	CreatedAt   string `json:"created_at"`
	NetworkName string `json:"network_name"`
}

// InviteCreateResult 邀请创建结果
type InviteCreateResult struct {
	InvitationID string `json:"invitation_id"`
}

// InviteVerifyResult 邀请验证结果
type InviteVerifyResult struct {
	Valid   bool   `json:"valid"`
	Inviter string `json:"inviter"`
}

// JoinResult 加入网络结果
type JoinResult struct {
	NodeID    string   `json:"node_id"`
	Neighbors []string `json:"neighbors"`
}

// LogSubmitResult 日志提交结果
type LogSubmitResult struct {
	LogID string `json:"log_id"`
}

// LogEntryInfo 日志条目信息
type LogEntryInfo struct {
	ID        string                 `json:"id"`
	NodeID    string                 `json:"node_id"`
	EventType string                 `json:"event_type"`
	Data      map[string]interface{} `json:"data"`
	Timestamp string                 `json:"timestamp"`
}

// LogExportResult 日志导出结果
type LogExportResult struct {
	File string `json:"file"`
}

// AuditDeviationInfo 审计偏离信息
type AuditDeviationInfo struct {
	AuditID        string `json:"audit_id"`
	AuditorID      string `json:"auditor_id"`
	ExpectedResult bool   `json:"expected_result"`
	ActualResult   bool   `json:"actual_result"`
	Severity       string `json:"severity"`
	Timestamp      string `json:"timestamp"`
}

// AuditPenaltyConfigInfo 审计惩罚配置信息
type AuditPenaltyConfigInfo struct {
	Minor  PenaltyConfig `json:"minor"`
	Severe PenaltyConfig `json:"severe"`
}

// PenaltyConfig 惩罚配置
type PenaltyConfig struct {
	RepPenalty int     `json:"rep_penalty"`
	SlashRatio float64 `json:"slash_ratio"`
}

// ManualPenaltyResult 手动惩罚结果
type ManualPenaltyResult struct {
	PenaltyApplied bool    `json:"penalty_applied"`
	RepDelta       int     `json:"rep_delta"`
	Slashed        float64 `json:"slashed"`
}

// CollateralInfo 抵押物信息
type CollateralInfo struct {
	CollateralID string  `json:"collateral_id"`
	NodeID       string  `json:"node_id"`
	Purpose      string  `json:"purpose"`
	Amount       float64 `json:"amount"`
	Slashed      float64 `json:"slashed"`
	Status       string  `json:"status"`
	CreatedAt    string  `json:"created_at"`
}

// SlashResult 罚没结果
type SlashResult struct {
	SlashedAmount float64 `json:"slashed_amount"`
	Remaining     float64 `json:"remaining"`
}

// SlashRecord 罚没记录
type SlashRecord struct {
	ID        string  `json:"id"`
	NodeID    string  `json:"node_id"`
	Purpose   string  `json:"purpose"`
	Amount    float64 `json:"amount"`
	Reason    string  `json:"reason"`
	Timestamp string  `json:"timestamp"`
}

// DisputeInfo 争议信息
type DisputeInfo struct {
	ID         string `json:"id"`
	Plaintiff  string `json:"plaintiff"`
	Defendant  string `json:"defendant"`
	Status     string `json:"status"`
	CreatedAt  string `json:"created_at"`
	EscrowID   string `json:"escrow_id,omitempty"`
}

// DisputeSuggestionInfo 争议建议信息
type DisputeSuggestionInfo struct {
	DisputeID           string   `json:"dispute_id"`
	SuggestedResolution string   `json:"suggested_resolution"`
	Confidence          float64  `json:"confidence"`
	CanAutoExecute      bool     `json:"can_auto_execute"`
	MissingEvidence     []string `json:"missing_evidence"`
	Warnings            []string `json:"warnings"`
}

// ApplySuggestionResult 应用建议结果
type ApplySuggestionResult struct {
	Applied    bool   `json:"applied"`
	Resolution string `json:"resolution"`
}

// DisputeDetail 争议详情
type DisputeDetail struct {
	DisputeInfo
	Description string         `json:"description"`
	Evidence    []EvidenceInfo `json:"evidence"`
	Resolution  string         `json:"resolution,omitempty"`
	ResolvedAt  string         `json:"resolved_at,omitempty"`
}

// EvidenceInfo 证据信息
type EvidenceInfo struct {
	ID         string `json:"id"`
	Type       string `json:"type"`
	Content    string `json:"content"`
	SubmittedBy string `json:"submitted_by"`
	Verified   bool   `json:"verified"`
	VerifiedBy string `json:"verified_by,omitempty"`
	Timestamp  string `json:"timestamp"`
}

// EscrowInfo 托管信息
type EscrowInfo struct {
	ID          string  `json:"id"`
	Amount      float64 `json:"amount"`
	Depositor   string  `json:"depositor"`
	Beneficiary string  `json:"beneficiary"`
	Status      string  `json:"status"`
	CreatedAt   string  `json:"created_at"`
}

// EscrowDetail 托管详情
type EscrowDetail struct {
	EscrowInfo
	Description     string   `json:"description,omitempty"`
	Arbitrators     []string `json:"arbitrators"`
	MinSignatures   int      `json:"min_signatures"`
	CurrentSigners  []string `json:"current_signers"`
	ExpiresAt       string   `json:"expires_at,omitempty"`
}

// SignatureSubmitResult 签名提交结果
type SignatureSubmitResult struct {
	Submitted    bool `json:"submitted"`
	CurrentCount int  `json:"current_count"`
	Required     int  `json:"required"`
}

// SignatureCountInfo 签名数量信息
type SignatureCountInfo struct {
	EscrowID     string   `json:"escrow_id"`
	CurrentCount int      `json:"current_count"`
	Required     int      `json:"required"`
	Signers      []string `json:"signers"`
}

// EscrowResolveResult 托管解决结果
type EscrowResolveResult struct {
	Resolved bool    `json:"resolved"`
	Winner   string  `json:"winner"`
	Amount   float64 `json:"amount"`
}

// ========== 扩展操作处理器 ==========

// ExtendedOperationHandlers 扩展操作处理器
type ExtendedOperationHandlers struct {
	server   *Server
	provider ExtendedOperationsProvider
}

// NewExtendedOperationHandlers 创建扩展操作处理器
func NewExtendedOperationHandlers(server *Server, provider ExtendedOperationsProvider) *ExtendedOperationHandlers {
	return &ExtendedOperationHandlers{
		server:   server,
		provider: provider,
	}
}

// getProvider 获取 provider
func (h *ExtendedOperationHandlers) getProvider() ExtendedOperationsProvider {
	return h.provider
}

// ========== 声誉扩展处理器 ==========

// HandleReputationUpdate 更新声誉
func (h *ExtendedOperationHandlers) HandleReputationUpdate(w http.ResponseWriter, r *http.Request) {
	provider := h.getProvider()
	if provider == nil {
		WriteError(w, http.StatusServiceUnavailable, "Provider not available")
		return
	}

	var req struct {
		NodeID string `json:"node_id"`
		Delta  int    `json:"delta"`
		Reason string `json:"reason"`
	}

	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		WriteError(w, http.StatusBadRequest, "Invalid request body")
		return
	}

	result, err := provider.UpdateReputation(req.NodeID, req.Delta, req.Reason)
	if err != nil {
		WriteError(w, http.StatusInternalServerError, err.Error())
		return
	}

	WriteJSON(w, http.StatusOK, result)
}

// HandleReputationHistory 获取声誉历史
func (h *ExtendedOperationHandlers) HandleReputationHistory(w http.ResponseWriter, r *http.Request) {
	provider := h.getProvider()
	if provider == nil {
		WriteError(w, http.StatusServiceUnavailable, "Provider not available")
		return
	}

	nodeID := r.URL.Query().Get("node_id")
	limit := parseIntParam(r, "limit", 20)

	history, err := provider.GetReputationHistory(nodeID, limit)
	if err != nil {
		WriteError(w, http.StatusInternalServerError, err.Error())
		return
	}

	WriteJSON(w, http.StatusOK, map[string]interface{}{
		"history": history,
		"count":   len(history),
	})
}

// ========== 任务管理处理器 ==========

// HandleTaskCreate 创建任务
func (h *ExtendedOperationHandlers) HandleTaskCreate(w http.ResponseWriter, r *http.Request) {
	provider := h.getProvider()
	if provider == nil {
		WriteError(w, http.StatusServiceUnavailable, "Provider not available")
		return
	}

	var req struct {
		Type        string `json:"type"`
		Description string `json:"description"`
		Deadline    int64  `json:"deadline"`
	}

	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		WriteError(w, http.StatusBadRequest, "Invalid request body")
		return
	}

	result, err := provider.CreateTask(req.Type, req.Description, req.Deadline)
	if err != nil {
		WriteError(w, http.StatusInternalServerError, err.Error())
		return
	}

	WriteJSON(w, http.StatusOK, result)
}

// HandleTaskStatus 获取任务状态
func (h *ExtendedOperationHandlers) HandleTaskStatus(w http.ResponseWriter, r *http.Request) {
	provider := h.getProvider()
	if provider == nil {
		WriteError(w, http.StatusServiceUnavailable, "Provider not available")
		return
	}

	taskID := r.URL.Query().Get("task_id")
	if taskID == "" {
		WriteError(w, http.StatusBadRequest, "task_id is required")
		return
	}

	status, err := provider.GetTaskStatus(taskID)
	if err != nil {
		WriteError(w, http.StatusInternalServerError, err.Error())
		return
	}

	WriteJSON(w, http.StatusOK, status)
}

// HandleTaskAccept 接受任务
func (h *ExtendedOperationHandlers) HandleTaskAccept(w http.ResponseWriter, r *http.Request) {
	provider := h.getProvider()
	if provider == nil {
		WriteError(w, http.StatusServiceUnavailable, "Provider not available")
		return
	}

	var req struct {
		TaskID string `json:"task_id"`
	}

	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		WriteError(w, http.StatusBadRequest, "Invalid request body")
		return
	}

	if err := provider.AcceptTask(req.TaskID); err != nil {
		WriteError(w, http.StatusInternalServerError, err.Error())
		return
	}

	WriteJSON(w, http.StatusOK, map[string]string{"status": "accepted"})
}

// HandleTaskSubmit 提交任务结果
func (h *ExtendedOperationHandlers) HandleTaskSubmit(w http.ResponseWriter, r *http.Request) {
	provider := h.getProvider()
	if provider == nil {
		WriteError(w, http.StatusServiceUnavailable, "Provider not available")
		return
	}

	var req struct {
		TaskID string `json:"task_id"`
		Result string `json:"result"`
	}

	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		WriteError(w, http.StatusBadRequest, "Invalid request body")
		return
	}

	if err := provider.SubmitTaskResult(req.TaskID, req.Result); err != nil {
		WriteError(w, http.StatusInternalServerError, err.Error())
		return
	}

	WriteJSON(w, http.StatusOK, map[string]string{"status": "submitted"})
}

// HandleTaskList 获取任务列表
func (h *ExtendedOperationHandlers) HandleTaskList(w http.ResponseWriter, r *http.Request) {
	provider := h.getProvider()
	if provider == nil {
		WriteError(w, http.StatusServiceUnavailable, "Provider not available")
		return
	}

	status := r.URL.Query().Get("status")
	limit := parseIntParam(r, "limit", 20)

	tasks, err := provider.ListTasks(status, limit)
	if err != nil {
		WriteError(w, http.StatusInternalServerError, err.Error())
		return
	}

	WriteJSON(w, http.StatusOK, map[string]interface{}{
		"tasks": tasks,
		"count": len(tasks),
	})
}

// ========== 指责系统处理器 ==========

// HandleAccusationCreate 创建指责
func (h *ExtendedOperationHandlers) HandleAccusationCreate(w http.ResponseWriter, r *http.Request) {
	provider := h.getProvider()
	if provider == nil {
		WriteError(w, http.StatusServiceUnavailable, "Provider not available")
		return
	}

	var req struct {
		Accused string `json:"accused"`
		Type    string `json:"type"`
		Reason  string `json:"reason"`
	}

	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		WriteError(w, http.StatusBadRequest, "Invalid request body")
		return
	}

	result, err := provider.CreateAccusation(req.Accused, req.Type, req.Reason)
	if err != nil {
		WriteError(w, http.StatusInternalServerError, err.Error())
		return
	}

	WriteJSON(w, http.StatusOK, result)
}

// HandleAccusationList 获取指责列表
func (h *ExtendedOperationHandlers) HandleAccusationList(w http.ResponseWriter, r *http.Request) {
	provider := h.getProvider()
	if provider == nil {
		WriteError(w, http.StatusServiceUnavailable, "Provider not available")
		return
	}

	accused := r.URL.Query().Get("accused")
	limit := parseIntParam(r, "limit", 20)

	accusations, err := provider.ListAccusations(accused, limit)
	if err != nil {
		WriteError(w, http.StatusInternalServerError, err.Error())
		return
	}

	WriteJSON(w, http.StatusOK, map[string]interface{}{
		"accusations": accusations,
		"count":       len(accusations),
	})
}

// HandleAccusationDetail 获取指责详情
func (h *ExtendedOperationHandlers) HandleAccusationDetail(w http.ResponseWriter, r *http.Request) {
	provider := h.getProvider()
	if provider == nil {
		WriteError(w, http.StatusServiceUnavailable, "Provider not available")
		return
	}

	id := extractPathParam(r.URL.Path, "detail")
	if id == "" {
		WriteError(w, http.StatusBadRequest, "id is required")
		return
	}

	detail, err := provider.GetAccusationDetail(id)
	if err != nil {
		WriteError(w, http.StatusInternalServerError, err.Error())
		return
	}

	WriteJSON(w, http.StatusOK, detail)
}

// HandleAccusationAnalyze 分析指责
func (h *ExtendedOperationHandlers) HandleAccusationAnalyze(w http.ResponseWriter, r *http.Request) {
	provider := h.getProvider()
	if provider == nil {
		WriteError(w, http.StatusServiceUnavailable, "Provider not available")
		return
	}

	nodeID := r.URL.Query().Get("node_id")
	if nodeID == "" {
		WriteError(w, http.StatusBadRequest, "node_id is required")
		return
	}

	analysis, err := provider.AnalyzeAccusations(nodeID)
	if err != nil {
		WriteError(w, http.StatusInternalServerError, err.Error())
		return
	}

	WriteJSON(w, http.StatusOK, analysis)
}

// ========== 激励系统处理器 ==========

// HandleIncentiveAward 奖励任务
func (h *ExtendedOperationHandlers) HandleIncentiveAward(w http.ResponseWriter, r *http.Request) {
	provider := h.getProvider()
	if provider == nil {
		WriteError(w, http.StatusServiceUnavailable, "Provider not available")
		return
	}

	var req struct {
		NodeID   string `json:"node_id"`
		TaskType string `json:"task_type"`
	}

	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		WriteError(w, http.StatusBadRequest, "Invalid request body")
		return
	}

	result, err := provider.AwardIncentive(req.NodeID, req.TaskType)
	if err != nil {
		WriteError(w, http.StatusInternalServerError, err.Error())
		return
	}

	WriteJSON(w, http.StatusOK, result)
}

// HandleIncentivePropagate 传播声誉
func (h *ExtendedOperationHandlers) HandleIncentivePropagate(w http.ResponseWriter, r *http.Request) {
	provider := h.getProvider()
	if provider == nil {
		WriteError(w, http.StatusServiceUnavailable, "Provider not available")
		return
	}

	var req struct {
		Target string `json:"target"`
		Delta  int    `json:"delta"`
	}

	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		WriteError(w, http.StatusBadRequest, "Invalid request body")
		return
	}

	result, err := provider.PropagateReputation(req.Target, req.Delta)
	if err != nil {
		WriteError(w, http.StatusInternalServerError, err.Error())
		return
	}

	WriteJSON(w, http.StatusOK, result)
}

// HandleIncentiveHistory 获取奖励历史
func (h *ExtendedOperationHandlers) HandleIncentiveHistory(w http.ResponseWriter, r *http.Request) {
	provider := h.getProvider()
	if provider == nil {
		WriteError(w, http.StatusServiceUnavailable, "Provider not available")
		return
	}

	nodeID := r.URL.Query().Get("node_id")
	limit := parseIntParam(r, "limit", 20)

	history, err := provider.GetIncentiveHistory(nodeID, limit)
	if err != nil {
		WriteError(w, http.StatusInternalServerError, err.Error())
		return
	}

	WriteJSON(w, http.StatusOK, map[string]interface{}{
		"rewards": history,
		"count":   len(history),
	})
}

// HandleIncentiveTolerance 获取耐受值
func (h *ExtendedOperationHandlers) HandleIncentiveTolerance(w http.ResponseWriter, r *http.Request) {
	provider := h.getProvider()
	if provider == nil {
		WriteError(w, http.StatusServiceUnavailable, "Provider not available")
		return
	}

	nodeID := r.URL.Query().Get("node_id")
	if nodeID == "" {
		WriteError(w, http.StatusBadRequest, "node_id is required")
		return
	}

	tolerance, err := provider.GetTolerance(nodeID)
	if err != nil {
		WriteError(w, http.StatusInternalServerError, err.Error())
		return
	}

	WriteJSON(w, http.StatusOK, tolerance)
}

// ========== 投票系统处理器 ==========

// HandleProposalCreate 创建提案
func (h *ExtendedOperationHandlers) HandleProposalCreate(w http.ResponseWriter, r *http.Request) {
	provider := h.getProvider()
	if provider == nil {
		WriteError(w, http.StatusServiceUnavailable, "Provider not available")
		return
	}

	var req struct {
		Title  string `json:"title"`
		Type   string `json:"type"`
		Target string `json:"target"`
	}

	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		WriteError(w, http.StatusBadRequest, "Invalid request body")
		return
	}

	result, err := provider.CreateProposal(req.Title, req.Type, req.Target)
	if err != nil {
		WriteError(w, http.StatusInternalServerError, err.Error())
		return
	}

	WriteJSON(w, http.StatusOK, result)
}

// HandleProposalList 获取提案列表
func (h *ExtendedOperationHandlers) HandleProposalList(w http.ResponseWriter, r *http.Request) {
	provider := h.getProvider()
	if provider == nil {
		WriteError(w, http.StatusServiceUnavailable, "Provider not available")
		return
	}

	status := r.URL.Query().Get("status")

	proposals, err := provider.ListProposals(status)
	if err != nil {
		WriteError(w, http.StatusInternalServerError, err.Error())
		return
	}

	WriteJSON(w, http.StatusOK, map[string]interface{}{
		"proposals": proposals,
		"count":     len(proposals),
	})
}

// HandleProposalDetail 获取提案详情
func (h *ExtendedOperationHandlers) HandleProposalDetail(w http.ResponseWriter, r *http.Request) {
	provider := h.getProvider()
	if provider == nil {
		WriteError(w, http.StatusServiceUnavailable, "Provider not available")
		return
	}

	id := extractLastPathSegment(r.URL.Path)
	if id == "" {
		WriteError(w, http.StatusBadRequest, "id is required")
		return
	}

	proposal, err := provider.GetProposal(id)
	if err != nil {
		WriteError(w, http.StatusInternalServerError, err.Error())
		return
	}

	WriteJSON(w, http.StatusOK, proposal)
}

// HandleVotingVote 投票
func (h *ExtendedOperationHandlers) HandleVotingVote(w http.ResponseWriter, r *http.Request) {
	provider := h.getProvider()
	if provider == nil {
		WriteError(w, http.StatusServiceUnavailable, "Provider not available")
		return
	}

	var req struct {
		ProposalID string `json:"proposal_id"`
		Vote       string `json:"vote"`
	}

	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		WriteError(w, http.StatusBadRequest, "Invalid request body")
		return
	}

	if err := provider.Vote(req.ProposalID, req.Vote); err != nil {
		WriteError(w, http.StatusInternalServerError, err.Error())
		return
	}

	WriteJSON(w, http.StatusOK, map[string]string{"status": "voted"})
}

// HandleProposalFinalize 结束提案
func (h *ExtendedOperationHandlers) HandleProposalFinalize(w http.ResponseWriter, r *http.Request) {
	provider := h.getProvider()
	if provider == nil {
		WriteError(w, http.StatusServiceUnavailable, "Provider not available")
		return
	}

	var req struct {
		ProposalID string `json:"proposal_id"`
	}

	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		WriteError(w, http.StatusBadRequest, "Invalid request body")
		return
	}

	result, err := provider.FinalizeProposal(req.ProposalID)
	if err != nil {
		WriteError(w, http.StatusInternalServerError, err.Error())
		return
	}

	WriteJSON(w, http.StatusOK, result)
}

// ========== 超级节点处理器 ==========

// HandleSupernodeList 获取超级节点列表
func (h *ExtendedOperationHandlers) HandleSupernodeList(w http.ResponseWriter, r *http.Request) {
	provider := h.getProvider()
	if provider == nil {
		WriteError(w, http.StatusServiceUnavailable, "Provider not available")
		return
	}

	supernodes, err := provider.ListSupernodes()
	if err != nil {
		WriteError(w, http.StatusInternalServerError, err.Error())
		return
	}

	WriteJSON(w, http.StatusOK, map[string]interface{}{
		"supernodes": supernodes,
		"count":      len(supernodes),
	})
}

// HandleCandidatesList 获取候选人列表
func (h *ExtendedOperationHandlers) HandleCandidatesList(w http.ResponseWriter, r *http.Request) {
	provider := h.getProvider()
	if provider == nil {
		WriteError(w, http.StatusServiceUnavailable, "Provider not available")
		return
	}

	candidates, err := provider.ListCandidates()
	if err != nil {
		WriteError(w, http.StatusInternalServerError, err.Error())
		return
	}

	WriteJSON(w, http.StatusOK, map[string]interface{}{
		"candidates": candidates,
		"count":      len(candidates),
	})
}

// HandleSupernodeApply 申请候选
func (h *ExtendedOperationHandlers) HandleSupernodeApply(w http.ResponseWriter, r *http.Request) {
	provider := h.getProvider()
	if provider == nil {
		WriteError(w, http.StatusServiceUnavailable, "Provider not available")
		return
	}

	var req struct {
		Stake int64 `json:"stake"`
	}

	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		WriteError(w, http.StatusBadRequest, "Invalid request body")
		return
	}

	if err := provider.ApplyCandidate(req.Stake); err != nil {
		WriteError(w, http.StatusInternalServerError, err.Error())
		return
	}

	WriteJSON(w, http.StatusOK, map[string]string{"status": "applied"})
}

// HandleSupernodeWithdraw 撤销候选
func (h *ExtendedOperationHandlers) HandleSupernodeWithdraw(w http.ResponseWriter, r *http.Request) {
	provider := h.getProvider()
	if provider == nil {
		WriteError(w, http.StatusServiceUnavailable, "Provider not available")
		return
	}

	if err := provider.WithdrawCandidate(); err != nil {
		WriteError(w, http.StatusInternalServerError, err.Error())
		return
	}

	WriteJSON(w, http.StatusOK, map[string]string{"status": "withdrawn"})
}

// HandleSupernodeVote 投票候选人
func (h *ExtendedOperationHandlers) HandleSupernodeVote(w http.ResponseWriter, r *http.Request) {
	provider := h.getProvider()
	if provider == nil {
		WriteError(w, http.StatusServiceUnavailable, "Provider not available")
		return
	}

	var req struct {
		Candidate string `json:"candidate"`
	}

	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		WriteError(w, http.StatusBadRequest, "Invalid request body")
		return
	}

	if err := provider.VoteCandidate(req.Candidate); err != nil {
		WriteError(w, http.StatusInternalServerError, err.Error())
		return
	}

	WriteJSON(w, http.StatusOK, map[string]string{"status": "voted"})
}

// HandleElectionStart 启动选举
func (h *ExtendedOperationHandlers) HandleElectionStart(w http.ResponseWriter, r *http.Request) {
	provider := h.getProvider()
	if provider == nil {
		WriteError(w, http.StatusServiceUnavailable, "Provider not available")
		return
	}

	result, err := provider.StartElection()
	if err != nil {
		WriteError(w, http.StatusInternalServerError, err.Error())
		return
	}

	WriteJSON(w, http.StatusOK, result)
}

// HandleElectionFinalize 完成选举
func (h *ExtendedOperationHandlers) HandleElectionFinalize(w http.ResponseWriter, r *http.Request) {
	provider := h.getProvider()
	if provider == nil {
		WriteError(w, http.StatusServiceUnavailable, "Provider not available")
		return
	}

	var req struct {
		ElectionID string `json:"election_id"`
	}

	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		WriteError(w, http.StatusBadRequest, "Invalid request body")
		return
	}

	result, err := provider.FinalizeElection(req.ElectionID)
	if err != nil {
		WriteError(w, http.StatusInternalServerError, err.Error())
		return
	}

	WriteJSON(w, http.StatusOK, result)
}

// HandleAuditSubmit 提交审计
func (h *ExtendedOperationHandlers) HandleAuditSubmit(w http.ResponseWriter, r *http.Request) {
	provider := h.getProvider()
	if provider == nil {
		WriteError(w, http.StatusServiceUnavailable, "Provider not available")
		return
	}

	var req struct {
		Target string `json:"target"`
		Passed bool   `json:"passed"`
	}

	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		WriteError(w, http.StatusBadRequest, "Invalid request body")
		return
	}

	result, err := provider.SubmitAudit(req.Target, req.Passed)
	if err != nil {
		WriteError(w, http.StatusInternalServerError, err.Error())
		return
	}

	WriteJSON(w, http.StatusOK, result)
}

// HandleAuditResult 获取审计结果
func (h *ExtendedOperationHandlers) HandleAuditResult(w http.ResponseWriter, r *http.Request) {
	provider := h.getProvider()
	if provider == nil {
		WriteError(w, http.StatusServiceUnavailable, "Provider not available")
		return
	}

	target := r.URL.Query().Get("target")
	if target == "" {
		WriteError(w, http.StatusBadRequest, "target is required")
		return
	}

	result, err := provider.GetAuditResult(target)
	if err != nil {
		WriteError(w, http.StatusInternalServerError, err.Error())
		return
	}

	WriteJSON(w, http.StatusOK, result)
}

// ========== 创世节点处理器 ==========

// HandleGenesisInfo 获取创世信息
func (h *ExtendedOperationHandlers) HandleGenesisInfo(w http.ResponseWriter, r *http.Request) {
	provider := h.getProvider()
	if provider == nil {
		WriteError(w, http.StatusServiceUnavailable, "Provider not available")
		return
	}

	info, err := provider.GetGenesisInfo()
	if err != nil {
		WriteError(w, http.StatusInternalServerError, err.Error())
		return
	}

	WriteJSON(w, http.StatusOK, info)
}

// HandleGenesisInviteCreate 创建邀请
func (h *ExtendedOperationHandlers) HandleGenesisInviteCreate(w http.ResponseWriter, r *http.Request) {
	provider := h.getProvider()
	if provider == nil {
		WriteError(w, http.StatusServiceUnavailable, "Provider not available")
		return
	}

	var req struct {
		ForPubkey string `json:"for_pubkey"`
	}

	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		WriteError(w, http.StatusBadRequest, "Invalid request body")
		return
	}

	result, err := provider.CreateInvite(req.ForPubkey)
	if err != nil {
		WriteError(w, http.StatusInternalServerError, err.Error())
		return
	}

	WriteJSON(w, http.StatusOK, result)
}

// HandleGenesisInviteVerify 验证邀请
func (h *ExtendedOperationHandlers) HandleGenesisInviteVerify(w http.ResponseWriter, r *http.Request) {
	provider := h.getProvider()
	if provider == nil {
		WriteError(w, http.StatusServiceUnavailable, "Provider not available")
		return
	}

	var req struct {
		Invitation string `json:"invitation"`
	}

	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		WriteError(w, http.StatusBadRequest, "Invalid request body")
		return
	}

	result, err := provider.VerifyInvite(req.Invitation)
	if err != nil {
		WriteError(w, http.StatusInternalServerError, err.Error())
		return
	}

	WriteJSON(w, http.StatusOK, result)
}

// HandleGenesisJoin 加入网络
func (h *ExtendedOperationHandlers) HandleGenesisJoin(w http.ResponseWriter, r *http.Request) {
	provider := h.getProvider()
	if provider == nil {
		WriteError(w, http.StatusServiceUnavailable, "Provider not available")
		return
	}

	var req struct {
		Invitation string `json:"invitation"`
		Pubkey     string `json:"pubkey"`
	}

	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		WriteError(w, http.StatusBadRequest, "Invalid request body")
		return
	}

	result, err := provider.JoinNetwork(req.Invitation, req.Pubkey)
	if err != nil {
		WriteError(w, http.StatusInternalServerError, err.Error())
		return
	}

	WriteJSON(w, http.StatusOK, result)
}

// ========== 日志系统处理器 ==========

// HandleLogSubmit 提交日志
func (h *ExtendedOperationHandlers) HandleLogSubmit(w http.ResponseWriter, r *http.Request) {
	provider := h.getProvider()
	if provider == nil {
		WriteError(w, http.StatusServiceUnavailable, "Provider not available")
		return
	}

	var req struct {
		EventType string                 `json:"event_type"`
		Data      map[string]interface{} `json:"data"`
	}

	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		WriteError(w, http.StatusBadRequest, "Invalid request body")
		return
	}

	result, err := provider.SubmitLog(req.EventType, req.Data)
	if err != nil {
		WriteError(w, http.StatusInternalServerError, err.Error())
		return
	}

	WriteJSON(w, http.StatusOK, result)
}

// HandleLogQuery 查询日志
func (h *ExtendedOperationHandlers) HandleLogQuery(w http.ResponseWriter, r *http.Request) {
	provider := h.getProvider()
	if provider == nil {
		WriteError(w, http.StatusServiceUnavailable, "Provider not available")
		return
	}

	nodeID := r.URL.Query().Get("node_id")
	eventType := r.URL.Query().Get("event_type")
	limit := parseIntParam(r, "limit", 50)

	logs, err := provider.QueryLogs(nodeID, eventType, limit)
	if err != nil {
		WriteError(w, http.StatusInternalServerError, err.Error())
		return
	}

	WriteJSON(w, http.StatusOK, map[string]interface{}{
		"logs":  logs,
		"count": len(logs),
	})
}

// HandleLogExport 导出日志
func (h *ExtendedOperationHandlers) HandleLogExport(w http.ResponseWriter, r *http.Request) {
	provider := h.getProvider()
	if provider == nil {
		WriteError(w, http.StatusServiceUnavailable, "Provider not available")
		return
	}

	format := r.URL.Query().Get("format")
	if format == "" {
		format = "json"
	}
	start := parseIntParam(r, "start", 0)
	end := parseIntParam(r, "end", 0)

	result, err := provider.ExportLogs(format, int64(start), int64(end))
	if err != nil {
		WriteError(w, http.StatusInternalServerError, err.Error())
		return
	}

	WriteJSON(w, http.StatusOK, result)
}

// ========== 审计集成处理器 (Task44) ==========

// HandleAuditDeviations 获取审计偏离列表
func (h *ExtendedOperationHandlers) HandleAuditDeviations(w http.ResponseWriter, r *http.Request) {
	provider := h.getProvider()
	if provider == nil {
		WriteError(w, http.StatusServiceUnavailable, "Provider not available")
		return
	}

	limit := parseIntParam(r, "limit", 20)

	deviations, err := provider.GetAuditDeviations(limit)
	if err != nil {
		WriteError(w, http.StatusInternalServerError, err.Error())
		return
	}

	WriteJSON(w, http.StatusOK, map[string]interface{}{
		"deviations": deviations,
		"count":      len(deviations),
	})
}

// HandleAuditPenaltyConfig 获取/设置惩罚配置
func (h *ExtendedOperationHandlers) HandleAuditPenaltyConfig(w http.ResponseWriter, r *http.Request) {
	provider := h.getProvider()
	if provider == nil {
		WriteError(w, http.StatusServiceUnavailable, "Provider not available")
		return
	}

	if r.Method == http.MethodGet {
		config, err := provider.GetAuditPenaltyConfig()
		if err != nil {
			WriteError(w, http.StatusInternalServerError, err.Error())
			return
		}
		WriteJSON(w, http.StatusOK, config)
		return
	}

	// POST - 设置配置
	var req struct {
		Severity   string  `json:"severity"`
		RepPenalty int     `json:"rep_penalty"`
		SlashRatio float64 `json:"slash_ratio"`
	}

	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		WriteError(w, http.StatusBadRequest, "Invalid request body")
		return
	}

	if err := provider.SetAuditPenaltyConfig(req.Severity, req.RepPenalty, req.SlashRatio); err != nil {
		WriteError(w, http.StatusInternalServerError, err.Error())
		return
	}

	WriteJSON(w, http.StatusOK, map[string]string{"status": "updated"})
}

// HandleManualPenalty 手动惩罚
func (h *ExtendedOperationHandlers) HandleManualPenalty(w http.ResponseWriter, r *http.Request) {
	provider := h.getProvider()
	if provider == nil {
		WriteError(w, http.StatusServiceUnavailable, "Provider not available")
		return
	}

	var req struct {
		NodeID   string `json:"node_id"`
		Severity string `json:"severity"`
		Reason   string `json:"reason"`
	}

	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		WriteError(w, http.StatusBadRequest, "Invalid request body")
		return
	}

	result, err := provider.ManualPenalty(req.NodeID, req.Severity, req.Reason)
	if err != nil {
		WriteError(w, http.StatusInternalServerError, err.Error())
		return
	}

	WriteJSON(w, http.StatusOK, result)
}

// ========== 抵押物管理处理器 (Task44) ==========

// HandleCollateralList 获取抵押物列表
func (h *ExtendedOperationHandlers) HandleCollateralList(w http.ResponseWriter, r *http.Request) {
	provider := h.getProvider()
	if provider == nil {
		WriteError(w, http.StatusServiceUnavailable, "Provider not available")
		return
	}

	status := r.URL.Query().Get("status")

	collaterals, err := provider.ListCollaterals(status)
	if err != nil {
		WriteError(w, http.StatusInternalServerError, err.Error())
		return
	}

	WriteJSON(w, http.StatusOK, map[string]interface{}{
		"collaterals": collaterals,
		"count":       len(collaterals),
	})
}

// HandleCollateralByNode 按节点查询抵押物
func (h *ExtendedOperationHandlers) HandleCollateralByNode(w http.ResponseWriter, r *http.Request) {
	provider := h.getProvider()
	if provider == nil {
		WriteError(w, http.StatusServiceUnavailable, "Provider not available")
		return
	}

	nodeID := r.URL.Query().Get("node_id")
	purpose := r.URL.Query().Get("purpose")

	if nodeID == "" {
		WriteError(w, http.StatusBadRequest, "node_id is required")
		return
	}

	collateral, err := provider.GetCollateralByNode(nodeID, purpose)
	if err != nil {
		WriteError(w, http.StatusInternalServerError, err.Error())
		return
	}

	WriteJSON(w, http.StatusOK, collateral)
}

// HandleSlashByNode 按节点罚没
func (h *ExtendedOperationHandlers) HandleSlashByNode(w http.ResponseWriter, r *http.Request) {
	provider := h.getProvider()
	if provider == nil {
		WriteError(w, http.StatusServiceUnavailable, "Provider not available")
		return
	}

	var req struct {
		NodeID   string  `json:"node_id"`
		Purpose  string  `json:"purpose"`
		Reason   string  `json:"reason"`
		Evidence string  `json:"evidence"`
		Ratio    float64 `json:"ratio"`
	}

	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		WriteError(w, http.StatusBadRequest, "Invalid request body")
		return
	}

	result, err := provider.SlashByNode(req.NodeID, req.Purpose, req.Reason, req.Evidence, req.Ratio)
	if err != nil {
		WriteError(w, http.StatusInternalServerError, err.Error())
		return
	}

	WriteJSON(w, http.StatusOK, result)
}

// HandleSlashHistory 获取罚没历史
func (h *ExtendedOperationHandlers) HandleSlashHistory(w http.ResponseWriter, r *http.Request) {
	provider := h.getProvider()
	if provider == nil {
		WriteError(w, http.StatusServiceUnavailable, "Provider not available")
		return
	}

	nodeID := r.URL.Query().Get("node_id")
	limit := parseIntParam(r, "limit", 20)

	history, err := provider.GetSlashHistory(nodeID, limit)
	if err != nil {
		WriteError(w, http.StatusInternalServerError, err.Error())
		return
	}

	WriteJSON(w, http.StatusOK, map[string]interface{}{
		"history": history,
		"count":   len(history),
	})
}

// ========== 争议预审处理器 (Task44) ==========

// HandleDisputeList 获取争议列表
func (h *ExtendedOperationHandlers) HandleDisputeList(w http.ResponseWriter, r *http.Request) {
	provider := h.getProvider()
	if provider == nil {
		WriteError(w, http.StatusServiceUnavailable, "Provider not available")
		return
	}

	status := r.URL.Query().Get("status")

	disputes, err := provider.ListDisputes(status)
	if err != nil {
		WriteError(w, http.StatusInternalServerError, err.Error())
		return
	}

	WriteJSON(w, http.StatusOK, map[string]interface{}{
		"disputes": disputes,
		"count":    len(disputes),
	})
}

// HandleDisputeSuggestion 获取争议建议
func (h *ExtendedOperationHandlers) HandleDisputeSuggestion(w http.ResponseWriter, r *http.Request) {
	provider := h.getProvider()
	if provider == nil {
		WriteError(w, http.StatusServiceUnavailable, "Provider not available")
		return
	}

	id := extractLastPathSegment(r.URL.Path)
	if id == "" {
		WriteError(w, http.StatusBadRequest, "id is required")
		return
	}

	suggestion, err := provider.GetDisputeSuggestion(id)
	if err != nil {
		WriteError(w, http.StatusInternalServerError, err.Error())
		return
	}

	WriteJSON(w, http.StatusOK, suggestion)
}

// HandleVerifyEvidence 验证证据
func (h *ExtendedOperationHandlers) HandleVerifyEvidence(w http.ResponseWriter, r *http.Request) {
	provider := h.getProvider()
	if provider == nil {
		WriteError(w, http.StatusServiceUnavailable, "Provider not available")
		return
	}

	var req struct {
		DisputeID  string `json:"dispute_id"`
		EvidenceID string `json:"evidence_id"`
		VerifierID string `json:"verifier_id"`
	}

	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		WriteError(w, http.StatusBadRequest, "Invalid request body")
		return
	}

	if err := provider.VerifyEvidence(req.DisputeID, req.EvidenceID, req.VerifierID); err != nil {
		WriteError(w, http.StatusInternalServerError, err.Error())
		return
	}

	WriteJSON(w, http.StatusOK, map[string]bool{"verified": true})
}

// HandleApplySuggestion 应用建议
func (h *ExtendedOperationHandlers) HandleApplySuggestion(w http.ResponseWriter, r *http.Request) {
	provider := h.getProvider()
	if provider == nil {
		WriteError(w, http.StatusServiceUnavailable, "Provider not available")
		return
	}

	var req struct {
		DisputeID  string `json:"dispute_id"`
		ApproverID string `json:"approver_id"`
	}

	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		WriteError(w, http.StatusBadRequest, "Invalid request body")
		return
	}

	result, err := provider.ApplyDisputeSuggestion(req.DisputeID, req.ApproverID)
	if err != nil {
		WriteError(w, http.StatusInternalServerError, err.Error())
		return
	}

	WriteJSON(w, http.StatusOK, result)
}

// HandleDisputeDetail 获取争议详情
func (h *ExtendedOperationHandlers) HandleDisputeDetail(w http.ResponseWriter, r *http.Request) {
	provider := h.getProvider()
	if provider == nil {
		WriteError(w, http.StatusServiceUnavailable, "Provider not available")
		return
	}

	id := extractLastPathSegment(r.URL.Path)
	if id == "" {
		WriteError(w, http.StatusBadRequest, "id is required")
		return
	}

	detail, err := provider.GetDisputeDetail(id)
	if err != nil {
		WriteError(w, http.StatusInternalServerError, err.Error())
		return
	}

	WriteJSON(w, http.StatusOK, detail)
}

// ========== 托管多签处理器 (Task44) ==========

// HandleEscrowList 获取托管列表
func (h *ExtendedOperationHandlers) HandleEscrowList(w http.ResponseWriter, r *http.Request) {
	provider := h.getProvider()
	if provider == nil {
		WriteError(w, http.StatusServiceUnavailable, "Provider not available")
		return
	}

	status := r.URL.Query().Get("status")

	escrows, err := provider.ListEscrows(status)
	if err != nil {
		WriteError(w, http.StatusInternalServerError, err.Error())
		return
	}

	WriteJSON(w, http.StatusOK, map[string]interface{}{
		"escrows": escrows,
		"count":   len(escrows),
	})
}

// HandleEscrowDetail 获取托管详情
func (h *ExtendedOperationHandlers) HandleEscrowDetail(w http.ResponseWriter, r *http.Request) {
	provider := h.getProvider()
	if provider == nil {
		WriteError(w, http.StatusServiceUnavailable, "Provider not available")
		return
	}

	id := extractLastPathSegment(r.URL.Path)
	if id == "" {
		WriteError(w, http.StatusBadRequest, "id is required")
		return
	}

	detail, err := provider.GetEscrowDetail(id)
	if err != nil {
		WriteError(w, http.StatusInternalServerError, err.Error())
		return
	}

	WriteJSON(w, http.StatusOK, detail)
}

// HandleArbitratorSignature 提交仲裁者签名
func (h *ExtendedOperationHandlers) HandleArbitratorSignature(w http.ResponseWriter, r *http.Request) {
	provider := h.getProvider()
	if provider == nil {
		WriteError(w, http.StatusServiceUnavailable, "Provider not available")
		return
	}

	var req struct {
		EscrowID     string `json:"escrow_id"`
		ArbitratorID string `json:"arbitrator_id"`
		Signature    string `json:"signature"`
	}

	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		WriteError(w, http.StatusBadRequest, "Invalid request body")
		return
	}

	result, err := provider.SubmitArbitratorSignature(req.EscrowID, req.ArbitratorID, req.Signature)
	if err != nil {
		WriteError(w, http.StatusInternalServerError, err.Error())
		return
	}

	WriteJSON(w, http.StatusOK, result)
}

// HandleSignatureCount 获取签名数量
func (h *ExtendedOperationHandlers) HandleSignatureCount(w http.ResponseWriter, r *http.Request) {
	provider := h.getProvider()
	if provider == nil {
		WriteError(w, http.StatusServiceUnavailable, "Provider not available")
		return
	}

	id := extractLastPathSegment(r.URL.Path)
	if id == "" {
		WriteError(w, http.StatusBadRequest, "id is required")
		return
	}

	count, err := provider.GetSignatureCount(id)
	if err != nil {
		WriteError(w, http.StatusInternalServerError, err.Error())
		return
	}

	WriteJSON(w, http.StatusOK, count)
}

// HandleEscrowResolve 解决托管
func (h *ExtendedOperationHandlers) HandleEscrowResolve(w http.ResponseWriter, r *http.Request) {
	provider := h.getProvider()
	if provider == nil {
		WriteError(w, http.StatusServiceUnavailable, "Provider not available")
		return
	}

	var req struct {
		EscrowID   string            `json:"escrow_id"`
		Winner     string            `json:"winner"`
		Signatures map[string]string `json:"signatures"`
	}

	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		WriteError(w, http.StatusBadRequest, "Invalid request body")
		return
	}

	result, err := provider.ResolveEscrow(req.EscrowID, req.Winner, req.Signatures)
	if err != nil {
		WriteError(w, http.StatusInternalServerError, err.Error())
		return
	}

	WriteJSON(w, http.StatusOK, result)
}

// ========== 辅助函数 ==========

func parseIntParam(r *http.Request, key string, defaultValue int) int {
	if v := r.URL.Query().Get(key); v != "" {
		if n, err := strconv.Atoi(v); err == nil && n > 0 {
			return n
		}
	}
	return defaultValue
}

func extractPathParam(path, segment string) string {
	parts := strings.Split(path, "/")
	for i, p := range parts {
		if p == segment && i+1 < len(parts) {
			return parts[i+1]
		}
	}
	return ""
}

func extractLastPathSegment(path string) string {
	parts := strings.Split(strings.TrimSuffix(path, "/"), "/")
	if len(parts) > 0 {
		return parts[len(parts)-1]
	}
	return ""
}
