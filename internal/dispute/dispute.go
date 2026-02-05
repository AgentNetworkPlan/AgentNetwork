// Package dispute 提供争议处理功能
// Task 27: 委托任务与文件传输
package dispute

import (
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"time"
)

var (
	ErrDisputeNotFound  = errors.New("dispute not found")
	ErrDisputeResolved  = errors.New("dispute already resolved")
	ErrInvalidEvidence  = errors.New("invalid evidence")
	ErrUnauthorized     = errors.New("unauthorized")
	ErrVotingClosed     = errors.New("voting is closed")
	ErrAlreadyVoted     = errors.New("already voted")
)

// DisputeStatus 争议状态
type DisputeStatus string

const (
	DisputePending     DisputeStatus = "pending"      // 等待处理
	DisputeInReview    DisputeStatus = "in_review"    // 审核中
	DisputeArbitration DisputeStatus = "arbitration"  // 仲裁中
	DisputeResolved    DisputeStatus = "resolved"     // 已解决
	DisputeDismissed   DisputeStatus = "dismissed"    // 已驳回
	DisputeExpired     DisputeStatus = "expired"      // 已过期
)

// DisputeType 争议类型
type DisputeType string

const (
	DisputeNonDelivery   DisputeType = "non_delivery"   // 未交付
	DisputeQualityIssue  DisputeType = "quality_issue"  // 质量问题
	DisputeNonPayment    DisputeType = "non_payment"    // 未支付
	DisputeFalseDelivery DisputeType = "false_delivery" // 虚假交付
	DisputeTimeout       DisputeType = "timeout"        // 超时
	DisputeOther         DisputeType = "other"          // 其他
)

// ResolutionType 解决方式
type ResolutionType string

const (
	ResolutionAutomatic ResolutionType = "automatic" // 自动仲裁（规则匹配）
	ResolutionCommittee ResolutionType = "committee" // 委员会裁决
	ResolutionMutual    ResolutionType = "mutual"    // 双方协商
)

// Dispute 争议记录
type Dispute struct {
	ID     string `json:"id"`
	TaskID string `json:"task_id"`

	// 参与方
	ComplainantID string `json:"complainant_id"` // 申诉方
	DefendantID   string `json:"defendant_id"`   // 被诉方

	// 争议信息
	Type        DisputeType `json:"type"`
	Description string      `json:"description"`
	Amount      float64     `json:"amount"` // 争议金额

	// 证据
	Evidence []Evidence `json:"evidence"`

	// 状态
	Status DisputeStatus `json:"status"`

	// 解决方案
	Resolution       *Resolution `json:"resolution,omitempty"`
	ResolutionType   ResolutionType `json:"resolution_type,omitempty"`

	// 仲裁投票
	Votes      []ArbitrationVote `json:"votes,omitempty"`
	VoteDeadline int64           `json:"vote_deadline,omitempty"`

	// 时间
	CreatedAt  int64 `json:"created_at"`
	UpdatedAt  int64 `json:"updated_at"`
	ResolvedAt int64 `json:"resolved_at,omitempty"`
	ExpiresAt  int64 `json:"expires_at"`
}

// Evidence 证据
type Evidence struct {
	ID          string `json:"id"`
	DisputeID   string `json:"dispute_id"`
	SubmitterID string `json:"submitter_id"`
	Type        string `json:"type"` // "text", "hash", "signature", "screenshot"
	Content     string `json:"content"`
	Hash        string `json:"hash"`
	SubmittedAt int64  `json:"submitted_at"`
	Verified    bool   `json:"verified"`
}

// Resolution 解决方案
type Resolution struct {
	Winner        string  `json:"winner"`          // 胜出方
	Loser         string  `json:"loser"`           // 败诉方
	AmountToWinner float64 `json:"amount_to_winner"` // 判给胜出方的金额
	Penalty       float64 `json:"penalty"`         // 对败诉方的惩罚
	Reason        string  `json:"reason"`
	ResolvedBy    string  `json:"resolved_by"` // 解决者（system/committee/mutual）
}

// ArbitrationVote 仲裁投票
type ArbitrationVote struct {
	ArbitratorID string `json:"arbitrator_id"`
	VoteFor      string `json:"vote_for"` // 支持谁
	Reason       string `json:"reason"`
	VotedAt      int64  `json:"voted_at"`
	Signature    string `json:"signature"`
}

// DisputeConfig 争议处理配置
type DisputeConfig struct {
	DataDir           string        // 数据目录
	AutoResolveRules  bool          // 是否启用自动解决规则
	ReviewPeriod      time.Duration // 审核期
	ArbitrationPeriod time.Duration // 仲裁期
	ExpirationPeriod  time.Duration // 过期期
	MinEvidenceCount  int           // 最少证据数
	MinVotesRequired  int           // 最少仲裁票数
}

// DefaultDisputeConfig 返回默认配置
func DefaultDisputeConfig() *DisputeConfig {
	return &DisputeConfig{
		DataDir:           "data/dispute",
		AutoResolveRules:  true,
		ReviewPeriod:      24 * time.Hour,
		ArbitrationPeriod: 72 * time.Hour,
		ExpirationPeriod:  7 * 24 * time.Hour,
		MinEvidenceCount:  1,
		MinVotesRequired:  3,
	}
}

// DisputeManager 争议管理器
type DisputeManager struct {
	mu     sync.RWMutex
	config *DisputeConfig

	// 争议记录
	disputes map[string]*Dispute // disputeID -> dispute

	// 索引
	disputesByTask   map[string]string   // taskID -> disputeID
	disputesByNode   map[string][]string // nodeID -> []disputeID
	disputesByStatus map[DisputeStatus][]string

	// 自动解决规则
	autoRules []AutoResolveRule
}

// AutoResolveRule 自动解决规则
type AutoResolveRule struct {
	Type        DisputeType
	Condition   func(*Dispute) bool
	Resolution  func(*Dispute) *Resolution
	Description string
}

// AutoResolveSuggestion Task44: 自动裁决建议（降级为预审，不再直接执行）
type AutoResolveSuggestion struct {
	DisputeID      string       `json:"dispute_id"`
	MatchedRule    string       `json:"matched_rule"`    // 匹配的规则描述
	Suggestion     *Resolution  `json:"suggestion"`      // 建议的裁决
	Confidence     float64      `json:"confidence"`      // 置信度 (0-1)
	MissingEvidence []string    `json:"missing_evidence"` // 缺失的关键证据
	Warnings       []string     `json:"warnings"`        // 风险警告
	CanAutoExecute bool         `json:"can_auto_execute"` // 是否可自动执行（仅当证据Verified时）
}

// NewDisputeManager 创建争议管理器
func NewDisputeManager(config *DisputeConfig) *DisputeManager {
	if config == nil {
		config = DefaultDisputeConfig()
	}

	dm := &DisputeManager{
		config:           config,
		disputes:         make(map[string]*Dispute),
		disputesByTask:   make(map[string]string),
		disputesByNode:   make(map[string][]string),
		disputesByStatus: make(map[DisputeStatus][]string),
		autoRules:        defaultAutoRules(),
	}

	dm.load()
	return dm
}

// RegisterAutoRule 注册自动解决规则
func (dm *DisputeManager) RegisterAutoRule(rule AutoResolveRule) {
	dm.mu.Lock()
	defer dm.mu.Unlock()
	dm.autoRules = append(dm.autoRules, rule)
}

// CreateDispute 创建争议
func (dm *DisputeManager) CreateDispute(taskID, complainantID, defendantID string, disputeType DisputeType, description string, amount float64) (*Dispute, error) {
	dm.mu.Lock()
	defer dm.mu.Unlock()

	// 检查是否已存在
	if _, exists := dm.disputesByTask[taskID]; exists {
		return nil, errors.New("dispute already exists for this task")
	}

	dispute := &Dispute{
		ID:            dm.generateID(),
		TaskID:        taskID,
		ComplainantID: complainantID,
		DefendantID:   defendantID,
		Type:          disputeType,
		Description:   description,
		Amount:        amount,
		Evidence:      make([]Evidence, 0),
		Status:        DisputePending,
		CreatedAt:     time.Now().Unix(),
		UpdatedAt:     time.Now().Unix(),
		ExpiresAt:     time.Now().Add(dm.config.ExpirationPeriod).Unix(),
	}

	dm.disputes[dispute.ID] = dispute
	dm.disputesByTask[taskID] = dispute.ID
	dm.disputesByNode[complainantID] = append(dm.disputesByNode[complainantID], dispute.ID)
	dm.disputesByNode[defendantID] = append(dm.disputesByNode[defendantID], dispute.ID)
	dm.disputesByStatus[DisputePending] = append(dm.disputesByStatus[DisputePending], dispute.ID)

	dm.save()
	return dispute, nil
}

// SubmitEvidence 提交证据
func (dm *DisputeManager) SubmitEvidence(disputeID, submitterID, evidenceType, content, hash string) error {
	dm.mu.Lock()
	defer dm.mu.Unlock()

	dispute, exists := dm.disputes[disputeID]
	if !exists {
		return ErrDisputeNotFound
	}

	if dispute.Status == DisputeResolved || dispute.Status == DisputeDismissed {
		return ErrDisputeResolved
	}

	// 验证是否是参与方
	if submitterID != dispute.ComplainantID && submitterID != dispute.DefendantID {
		return ErrUnauthorized
	}

	evidence := Evidence{
		ID:          dm.generateID(),
		DisputeID:   disputeID,
		SubmitterID: submitterID,
		Type:        evidenceType,
		Content:     content,
		Hash:        hash,
		SubmittedAt: time.Now().Unix(),
		Verified:    false,
	}

	dispute.Evidence = append(dispute.Evidence, evidence)
	dispute.UpdatedAt = time.Now().Unix()

	dm.save()
	return nil
}

// VerifyEvidence Task44: 验证证据（将证据标记为已验证）
func (dm *DisputeManager) VerifyEvidence(disputeID, evidenceID, verifierID string) error {
	dm.mu.Lock()
	defer dm.mu.Unlock()

	dispute, exists := dm.disputes[disputeID]
	if !exists {
		return ErrDisputeNotFound
	}

	for i := range dispute.Evidence {
		if dispute.Evidence[i].ID == evidenceID {
			dispute.Evidence[i].Verified = true
			dispute.UpdatedAt = time.Now().Unix()
			dm.save()
			return nil
		}
	}

	return ErrInvalidEvidence
}

// StartReview 开始审核
func (dm *DisputeManager) StartReview(disputeID string) error {
	dm.mu.Lock()
	defer dm.mu.Unlock()

	dispute, exists := dm.disputes[disputeID]
	if !exists {
		return ErrDisputeNotFound
	}

	if dispute.Status != DisputePending {
		return fmt.Errorf("cannot start review: dispute status is %s", dispute.Status)
	}

	// 检查证据数量
	if len(dispute.Evidence) < dm.config.MinEvidenceCount {
		return fmt.Errorf("insufficient evidence: need at least %d, got %d", dm.config.MinEvidenceCount, len(dispute.Evidence))
	}

	oldStatus := dispute.Status
	dispute.Status = DisputeInReview
	dispute.UpdatedAt = time.Now().Unix()

	dm.updateStatusIndex(disputeID, oldStatus, DisputeInReview)
	dm.save()

	return nil
}

// TryAutoResolve Task44: 尝试自动解决（降级为预审建议）
// 返回 AutoResolveSuggestion 而不是直接裁决，需要调用 ApplyAutoResolution 执行
func (dm *DisputeManager) TryAutoResolve(disputeID string) (*AutoResolveSuggestion, error) {
	dm.mu.Lock()
	defer dm.mu.Unlock()

	if !dm.config.AutoResolveRules {
		return nil, errors.New("auto resolve is disabled")
	}

	dispute, exists := dm.disputes[disputeID]
	if !exists {
		return nil, ErrDisputeNotFound
	}

	if dispute.Status != DisputeInReview {
		return nil, fmt.Errorf("cannot auto resolve: dispute status is %s", dispute.Status)
	}

	// Task44: 检查证据是否已验证
	var warnings []string
	var missingEvidence []string
	hasVerifiedEvidence := false
	for _, e := range dispute.Evidence {
		if e.Verified {
			hasVerifiedEvidence = true
		} else {
			warnings = append(warnings, fmt.Sprintf("evidence '%s' (type=%s) is not verified", e.ID, e.Type))
		}
	}
	if !hasVerifiedEvidence {
		missingEvidence = append(missingEvidence, "at least one verified evidence required")
	}

	// 尝试匹配规则
	for _, rule := range dm.autoRules {
		if rule.Type == dispute.Type && rule.Condition(dispute) {
			resolution := rule.Resolution(dispute)
			resolution.ResolvedBy = "system_suggestion" // Task44: 标记为建议而非最终裁决

			// Task44: 计算置信度（基于已验证证据比例）
			confidence := 0.5 // 基础置信度
			verifiedCount := 0
			for _, e := range dispute.Evidence {
				if e.Verified {
					verifiedCount++
				}
			}
			if len(dispute.Evidence) > 0 {
				confidence = 0.5 + 0.5*float64(verifiedCount)/float64(len(dispute.Evidence))
			}

			// Task44: 仅当所有关键证据已验证时才允许自动执行
			canAutoExecute := hasVerifiedEvidence && len(warnings) == 0

			return &AutoResolveSuggestion{
				DisputeID:       disputeID,
				MatchedRule:     rule.Description,
				Suggestion:      resolution,
				Confidence:      confidence,
				MissingEvidence: missingEvidence,
				Warnings:        warnings,
				CanAutoExecute:  canAutoExecute,
			}, nil
		}
	}

	return nil, errors.New("no matching auto-resolve rule")
}

// ApplyAutoResolution Task44: 应用自动裁决建议（需要明确批准）
func (dm *DisputeManager) ApplyAutoResolution(disputeID string, suggestion *AutoResolveSuggestion, approverID string) (*Resolution, error) {
	dm.mu.Lock()
	defer dm.mu.Unlock()

	if suggestion == nil || suggestion.Suggestion == nil {
		return nil, errors.New("invalid suggestion")
	}

	dispute, exists := dm.disputes[disputeID]
	if !exists {
		return nil, ErrDisputeNotFound
	}

	if dispute.Status != DisputeInReview {
		return nil, fmt.Errorf("cannot apply resolution: dispute status is %s", dispute.Status)
	}

	// Task44: 仅当满足条件时才允许执行
	if !suggestion.CanAutoExecute {
		return nil, errors.New("suggestion cannot be auto-executed: missing verified evidence")
	}

	resolution := suggestion.Suggestion
	resolution.ResolvedBy = fmt.Sprintf("system_approved_by_%s", approverID)

	dispute.Resolution = resolution
	dispute.ResolutionType = ResolutionAutomatic
	dispute.Status = DisputeResolved
	dispute.ResolvedAt = time.Now().Unix()
	dispute.UpdatedAt = time.Now().Unix()

	dm.updateStatusIndex(disputeID, DisputeInReview, DisputeResolved)
	dm.save()

	return resolution, nil
}

// StartArbitration 开始仲裁
func (dm *DisputeManager) StartArbitration(disputeID string, arbitrators []string) error {
	dm.mu.Lock()
	defer dm.mu.Unlock()

	dispute, exists := dm.disputes[disputeID]
	if !exists {
		return ErrDisputeNotFound
	}

	if dispute.Status != DisputeInReview {
		return fmt.Errorf("cannot start arbitration: dispute status is %s", dispute.Status)
	}

	if len(arbitrators) < dm.config.MinVotesRequired {
		return fmt.Errorf("need at least %d arbitrators", dm.config.MinVotesRequired)
	}

	oldStatus := dispute.Status
	dispute.Status = DisputeArbitration
	dispute.Votes = make([]ArbitrationVote, 0)
	dispute.VoteDeadline = time.Now().Add(dm.config.ArbitrationPeriod).Unix()
	dispute.UpdatedAt = time.Now().Unix()

	dm.updateStatusIndex(disputeID, oldStatus, DisputeArbitration)
	dm.save()

	return nil
}

// SubmitVote 提交仲裁投票
func (dm *DisputeManager) SubmitVote(disputeID, arbitratorID, voteFor, reason, signature string) error {
	dm.mu.Lock()
	defer dm.mu.Unlock()

	dispute, exists := dm.disputes[disputeID]
	if !exists {
		return ErrDisputeNotFound
	}

	if dispute.Status != DisputeArbitration {
		return ErrVotingClosed
	}

	// 检查投票截止时间
	if time.Now().Unix() > dispute.VoteDeadline {
		return ErrVotingClosed
	}

	// 检查是否已投票
	for _, vote := range dispute.Votes {
		if vote.ArbitratorID == arbitratorID {
			return ErrAlreadyVoted
		}
	}

	// 验证投票对象
	if voteFor != dispute.ComplainantID && voteFor != dispute.DefendantID {
		return fmt.Errorf("vote_for must be either complainant or defendant")
	}

	vote := ArbitrationVote{
		ArbitratorID: arbitratorID,
		VoteFor:      voteFor,
		Reason:       reason,
		VotedAt:      time.Now().Unix(),
		Signature:    signature,
	}

	dispute.Votes = append(dispute.Votes, vote)
	dispute.UpdatedAt = time.Now().Unix()

	dm.save()
	return nil
}

// FinalizeArbitration 完成仲裁
func (dm *DisputeManager) FinalizeArbitration(disputeID string) (*Resolution, error) {
	dm.mu.Lock()
	defer dm.mu.Unlock()

	dispute, exists := dm.disputes[disputeID]
	if !exists {
		return nil, ErrDisputeNotFound
	}

	if dispute.Status != DisputeArbitration {
		return nil, fmt.Errorf("dispute is not in arbitration: %s", dispute.Status)
	}

	// 检查票数
	if len(dispute.Votes) < dm.config.MinVotesRequired {
		return nil, fmt.Errorf("insufficient votes: need %d, got %d", dm.config.MinVotesRequired, len(dispute.Votes))
	}

	// 统计投票
	voteCounts := make(map[string]int)
	for _, vote := range dispute.Votes {
		voteCounts[vote.VoteFor]++
	}

	// 确定胜者
	var winner, loser string
	var maxVotes int
	for nodeID, count := range voteCounts {
		if count > maxVotes {
			maxVotes = count
			winner = nodeID
		}
	}

	if winner == dispute.ComplainantID {
		loser = dispute.DefendantID
	} else {
		loser = dispute.ComplainantID
	}

	resolution := &Resolution{
		Winner:         winner,
		Loser:          loser,
		AmountToWinner: dispute.Amount,
		Penalty:        dispute.Amount * 0.1, // 10% 惩罚
		Reason:         fmt.Sprintf("Committee arbitration: %d votes for winner", maxVotes),
		ResolvedBy:     "committee",
	}

	dispute.Resolution = resolution
	dispute.ResolutionType = ResolutionCommittee
	dispute.Status = DisputeResolved
	dispute.ResolvedAt = time.Now().Unix()
	dispute.UpdatedAt = time.Now().Unix()

	dm.updateStatusIndex(disputeID, DisputeArbitration, DisputeResolved)
	dm.save()

	return resolution, nil
}

// DismissDispute 驳回争议
func (dm *DisputeManager) DismissDispute(disputeID, reason string) error {
	dm.mu.Lock()
	defer dm.mu.Unlock()

	dispute, exists := dm.disputes[disputeID]
	if !exists {
		return ErrDisputeNotFound
	}

	if dispute.Status == DisputeResolved || dispute.Status == DisputeDismissed {
		return ErrDisputeResolved
	}

	oldStatus := dispute.Status
	dispute.Status = DisputeDismissed
	dispute.Resolution = &Resolution{
		Reason:     reason,
		ResolvedBy: "system",
	}
	dispute.ResolvedAt = time.Now().Unix()
	dispute.UpdatedAt = time.Now().Unix()

	dm.updateStatusIndex(disputeID, oldStatus, DisputeDismissed)
	dm.save()

	return nil
}

// GetDispute 获取争议信息
func (dm *DisputeManager) GetDispute(disputeID string) (*Dispute, error) {
	dm.mu.RLock()
	defer dm.mu.RUnlock()

	dispute, exists := dm.disputes[disputeID]
	if !exists {
		return nil, ErrDisputeNotFound
	}
	return dispute, nil
}

// GetDisputeByTask 根据任务获取争议
func (dm *DisputeManager) GetDisputeByTask(taskID string) (*Dispute, error) {
	dm.mu.RLock()
	defer dm.mu.RUnlock()

	disputeID, exists := dm.disputesByTask[taskID]
	if !exists {
		return nil, ErrDisputeNotFound
	}

	dispute, exists := dm.disputes[disputeID]
	if !exists {
		return nil, ErrDisputeNotFound
	}

	return dispute, nil
}

// GetDisputesByNode 获取节点的所有争议
func (dm *DisputeManager) GetDisputesByNode(nodeID string) []*Dispute {
	dm.mu.RLock()
	defer dm.mu.RUnlock()

	ids := dm.disputesByNode[nodeID]
	disputes := make([]*Dispute, 0, len(ids))
	for _, id := range ids {
		if d, exists := dm.disputes[id]; exists {
			disputes = append(disputes, d)
		}
	}
	return disputes
}

// GetDisputesByStatus 获取指定状态的争议
func (dm *DisputeManager) GetDisputesByStatus(status DisputeStatus) []*Dispute {
	dm.mu.RLock()
	defer dm.mu.RUnlock()

	ids := dm.disputesByStatus[status]
	disputes := make([]*Dispute, 0, len(ids))
	for _, id := range ids {
		if d, exists := dm.disputes[id]; exists {
			disputes = append(disputes, d)
		}
	}
	return disputes
}

// CheckExpiredDisputes 检查过期争议
func (dm *DisputeManager) CheckExpiredDisputes() []string {
	dm.mu.Lock()
	defer dm.mu.Unlock()

	var expired []string
	now := time.Now().Unix()

	for id, dispute := range dm.disputes {
		if dispute.Status != DisputeResolved && dispute.Status != DisputeDismissed && dispute.Status != DisputeExpired {
			if now > dispute.ExpiresAt {
				oldStatus := dispute.Status
				dispute.Status = DisputeExpired
				dispute.UpdatedAt = now
				dm.updateStatusIndex(id, oldStatus, DisputeExpired)
				expired = append(expired, id)
			}
		}
	}

	if len(expired) > 0 {
		dm.save()
	}

	return expired
}

// GetStatistics 获取统计信息
func (dm *DisputeManager) GetStatistics() *DisputeStatistics {
	dm.mu.RLock()
	defer dm.mu.RUnlock()

	stats := &DisputeStatistics{
		TotalDisputes: len(dm.disputes),
		ByStatus:      make(map[DisputeStatus]int),
		ByType:        make(map[DisputeType]int),
	}

	for _, dispute := range dm.disputes {
		stats.ByStatus[dispute.Status]++
		stats.ByType[dispute.Type]++

		if dispute.Status == DisputeResolved {
			if dispute.Resolution != nil {
				if dispute.Resolution.Winner == dispute.ComplainantID {
					stats.ComplainantWins++
				} else {
					stats.DefendantWins++
				}
			}
		}
	}

	return stats
}

// DisputeStatistics 争议统计
type DisputeStatistics struct {
	TotalDisputes   int
	ByStatus        map[DisputeStatus]int
	ByType          map[DisputeType]int
	ComplainantWins int
	DefendantWins   int
}

// 内部方法

func (dm *DisputeManager) generateID() string {
	bytes := make([]byte, 16)
	rand.Read(bytes)
	return "dispute_" + hex.EncodeToString(bytes)
}

func (dm *DisputeManager) updateStatusIndex(disputeID string, oldStatus, newStatus DisputeStatus) {
	// 从旧状态列表中移除
	oldList := dm.disputesByStatus[oldStatus]
	for i, id := range oldList {
		if id == disputeID {
			dm.disputesByStatus[oldStatus] = append(oldList[:i], oldList[i+1:]...)
			break
		}
	}

	// 添加到新状态列表
	dm.disputesByStatus[newStatus] = append(dm.disputesByStatus[newStatus], disputeID)
}

func (dm *DisputeManager) load() {
	filePath := filepath.Join(dm.config.DataDir, "disputes.json")
	data, err := os.ReadFile(filePath)
	if err != nil {
		return
	}

	var stored struct {
		Disputes map[string]*Dispute `json:"disputes"`
	}

	if err := json.Unmarshal(data, &stored); err != nil {
		return
	}

	if stored.Disputes != nil {
		dm.disputes = stored.Disputes
		// 重建索引
		for id, d := range dm.disputes {
			dm.disputesByTask[d.TaskID] = id
			dm.disputesByNode[d.ComplainantID] = append(dm.disputesByNode[d.ComplainantID], id)
			dm.disputesByNode[d.DefendantID] = append(dm.disputesByNode[d.DefendantID], id)
			dm.disputesByStatus[d.Status] = append(dm.disputesByStatus[d.Status], id)
		}
	}
}

func (dm *DisputeManager) save() {
	if err := os.MkdirAll(dm.config.DataDir, 0755); err != nil {
		return
	}

	stored := struct {
		Disputes map[string]*Dispute `json:"disputes"`
	}{
		Disputes: dm.disputes,
	}

	data, err := json.MarshalIndent(stored, "", "  ")
	if err != nil {
		return
	}

	filePath := filepath.Join(dm.config.DataDir, "disputes.json")
	os.WriteFile(filePath, data, 0644)
}

// 默认自动解决规则
func defaultAutoRules() []AutoResolveRule {
	return []AutoResolveRule{
		{
			Type: DisputeTimeout,
			Condition: func(d *Dispute) bool {
				// 超时争议：如果有交付证明，申诉方获胜
				for _, e := range d.Evidence {
					if e.Type == "delivery_proof" && e.SubmitterID == d.ComplainantID {
						return true
					}
				}
				return false
			},
			Resolution: func(d *Dispute) *Resolution {
				return &Resolution{
					Winner:         d.ComplainantID,
					Loser:          d.DefendantID,
					AmountToWinner: d.Amount,
					Penalty:        d.Amount * 0.1,
					Reason:         "Auto-resolved: Timeout with valid delivery proof",
				}
			},
			Description: "Timeout with delivery proof - complainant wins",
		},
		{
			Type: DisputeNonDelivery,
			Condition: func(d *Dispute) bool {
				// 未交付争议：如果没有交付证明，申诉方获胜
				hasDeliveryProof := false
				for _, e := range d.Evidence {
					if e.Type == "delivery_proof" && e.SubmitterID == d.DefendantID {
						hasDeliveryProof = true
						break
					}
				}
				return !hasDeliveryProof
			},
			Resolution: func(d *Dispute) *Resolution {
				return &Resolution{
					Winner:         d.ComplainantID,
					Loser:          d.DefendantID,
					AmountToWinner: d.Amount,
					Penalty:        d.Amount * 0.2,
					Reason:         "Auto-resolved: Non-delivery without proof",
				}
			},
			Description: "Non-delivery without proof - complainant wins",
		},
		{
			Type: DisputeNonPayment,
			Condition: func(d *Dispute) bool {
				// 未支付争议：如果有完成证明，被诉方获胜
				hasCompletionProof := false
				for _, e := range d.Evidence {
					if e.Type == "completion_proof" && e.SubmitterID == d.DefendantID {
						hasCompletionProof = true
						break
					}
				}
				return hasCompletionProof
			},
			Resolution: func(d *Dispute) *Resolution {
				return &Resolution{
					Winner:         d.DefendantID,
					Loser:          d.ComplainantID,
					AmountToWinner: d.Amount,
					Penalty:        d.Amount * 0.15,
					Reason:         "Auto-resolved: Non-payment with completion proof",
				}
			},
			Description: "Non-payment with completion proof - defendant wins",
		},
	}
}
