// Package supernode 实现超级节点管理
// 包括超级节点选举、任务审计、网络监督和动态轮换
package supernode

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"math/rand"
	"os"
	"path/filepath"
	"sort"
	"sync"
	"time"
)

// NodeRole 节点角色
type NodeRole string

const (
	RoleNormal NodeRole = "normal" // 普通节点
	RoleSuper  NodeRole = "super"  // 超级节点
	RoleCandidate NodeRole = "candidate" // 候选超级节点
)

// AuditType 审计类型
type AuditType string

const (
	AuditTask       AuditType = "task"       // 任务审计
	AuditReputation AuditType = "reputation" // 声誉审计
	AuditBehavior   AuditType = "behavior"   // 行为审计
)

// AuditResult 审计结果
type AuditResult string

const (
	ResultPass   AuditResult = "pass"   // 通过
	ResultFail   AuditResult = "fail"   // 失败
	ResultPending AuditResult = "pending" // 待定
)

// SuperNode 超级节点信息
type SuperNode struct {
	NodeID      string    `json:"node_id"`
	Reputation  float64   `json:"reputation"`     // 信誉值
	Stake       float64   `json:"stake"`          // 抵押值
	ElectedAt   time.Time `json:"elected_at"`     // 当选时间
	TermEndsAt  time.Time `json:"term_ends_at"`   // 任期结束时间
	VotesReceived float64 `json:"votes_received"` // 获得票数
	AuditCount  int       `json:"audit_count"`    // 审计次数
	PassRate    float64   `json:"pass_rate"`      // 审计通过率
	IsActive    bool      `json:"is_active"`      // 是否活跃
}

// Candidate 候选超级节点
type Candidate struct {
	NodeID     string    `json:"node_id"`
	Reputation float64   `json:"reputation"`
	Stake      float64   `json:"stake"`
	AppliedAt  time.Time `json:"applied_at"`
	Votes      float64   `json:"votes"` // 获得投票权重
	Supporters map[string]float64 `json:"supporters"` // 支持者: voterID -> weight
}

// AuditRecord 审计记录
type AuditRecord struct {
	ID          string      `json:"id"`
	Type        AuditType   `json:"type"`
	TargetID    string      `json:"target_id"`    // 被审计对象ID
	AuditorID   string      `json:"auditor_id"`   // 审计者（超级节点）ID
	Result      AuditResult `json:"result"`
	Evidence    string      `json:"evidence"`     // 审计证据/说明
	Timestamp   time.Time   `json:"timestamp"`
	Signature   []byte      `json:"signature"`
}

// MultiAudit 多节点交叉审计
type MultiAudit struct {
	ID          string                   `json:"id"`
	Type        AuditType                `json:"type"`
	TargetID    string                   `json:"target_id"`
	CreatedAt   time.Time                `json:"created_at"`
	ExpiresAt   time.Time                `json:"expires_at"`
	Auditors    []string                 `json:"auditors"`      // 被分配的审计者
	Results     map[string]*AuditRecord  `json:"results"`       // auditorID -> result
	FinalResult AuditResult              `json:"final_result"`
	Finalized   bool                     `json:"finalized"`
	Deviations  []AuditDeviation         `json:"deviations"`    // 偏离共识的审计者记录
}

// AuditDeviation 审计偏离记录（用于惩罚闭环）
type AuditDeviation struct {
	AuditID       string      `json:"audit_id"`
	AuditorID     string      `json:"auditor_id"`
	ExpectedResult AuditResult `json:"expected_result"` // 应该的结果（FinalResult）
	ActualResult   AuditResult `json:"actual_result"`   // 实际提交的结果
	Severity       string      `json:"severity"`        // minor/severe
	DetectedAt     time.Time   `json:"detected_at"`
}

// Election 选举周期
type Election struct {
	ID          string               `json:"id"`
	StartAt     time.Time            `json:"start_at"`
	EndAt       time.Time            `json:"end_at"`
	Candidates  map[string]*Candidate `json:"candidates"` // nodeID -> Candidate
	Winners     []string             `json:"winners"`
	Status      ElectionStatus       `json:"status"`
}

// ElectionStatus 选举状态
type ElectionStatus string

const (
	ElectionOpen     ElectionStatus = "open"     // 开放投票
	ElectionClosed   ElectionStatus = "closed"   // 已结束
	ElectionFinalized ElectionStatus = "finalized" // 已确认
)

// SignFunc 签名函数
type SignFunc func(data []byte) ([]byte, error)

// VerifyFunc 验签函数
type VerifyFunc func(pubKey string, data, signature []byte) (bool, error)

// SuperNodeConfig 超级节点配置
type SuperNodeConfig struct {
	NodeID              string        // 当前节点ID
	DataDir             string        // 数据目录
	MaxSuperNodes       int           // 最大超级节点数量
	TermDuration        time.Duration // 任期时长
	ElectionDuration    time.Duration // 选举周期
	MinReputation       float64       // 候选最低信誉
	MinStake            float64       // 候选最低抵押
	AuditThreshold      float64       // 审计通过阈值 (0-1)
	AuditorsPerTask     int           // 每个任务的审计者数量
	CleanupInterval     time.Duration // 清理间隔
}

// DefaultConfig 返回默认配置
func DefaultConfig(nodeID string) *SuperNodeConfig {
	return &SuperNodeConfig{
		NodeID:           nodeID,
		DataDir:          "./data/supernode",
		MaxSuperNodes:    5,
		TermDuration:     7 * 24 * time.Hour, // 1周
		ElectionDuration: 24 * time.Hour,     // 1天投票期
		MinReputation:    50,
		MinStake:         30,
		AuditThreshold:   0.6, // 60%审计者通过才算通过
		AuditorsPerTask:  3,
		CleanupInterval:  1 * time.Hour,
	}
}

// SuperNodeManager 超级节点管理器
type SuperNodeManager struct {
	config      *SuperNodeConfig
	superNodes  map[string]*SuperNode  // nodeID -> SuperNode
	candidates  map[string]*Candidate  // nodeID -> Candidate
	audits      map[string]*MultiAudit // auditID -> MultiAudit
	elections   map[string]*Election   // electionID -> Election
	currentElection *Election
	mu          sync.RWMutex

	signFunc   SignFunc
	verifyFunc VerifyFunc

	// 回调
	onSuperNodeElected   func(*SuperNode)
	onSuperNodeRemoved   func(nodeID string)
	onAuditCompleted     func(*MultiAudit)
	onAuditorDeviation   func(*AuditDeviation) // Task44: 审计偏离惩罚回调
	onElectionStarted    func(*Election)
	onElectionFinalized  func(*Election)

	stopCh chan struct{}
	wg     sync.WaitGroup
	rng    *rand.Rand
}

// NewSuperNodeManager 创建超级节点管理器
func NewSuperNodeManager(config *SuperNodeConfig) (*SuperNodeManager, error) {
	if config == nil {
		return nil, errors.New("config is required")
	}
	if config.NodeID == "" {
		return nil, errors.New("node ID is required")
	}
	if config.MaxSuperNodes <= 0 {
		return nil, errors.New("max super nodes must be positive")
	}

	if config.DataDir != "" {
		if err := os.MkdirAll(config.DataDir, 0755); err != nil {
			return nil, fmt.Errorf("failed to create data dir: %w", err)
		}
	}

	sm := &SuperNodeManager{
		config:     config,
		superNodes: make(map[string]*SuperNode),
		candidates: make(map[string]*Candidate),
		audits:     make(map[string]*MultiAudit),
		elections:  make(map[string]*Election),
		stopCh:     make(chan struct{}),
		rng:        rand.New(rand.NewSource(time.Now().UnixNano())),
	}

	return sm, nil
}

// SetSignFunc 设置签名函数
func (s *SuperNodeManager) SetSignFunc(fn SignFunc) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.signFunc = fn
}

// SetVerifyFunc 设置验签函数
func (s *SuperNodeManager) SetVerifyFunc(fn VerifyFunc) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.verifyFunc = fn
}

// SetOnSuperNodeElected 设置超级节点当选回调
func (s *SuperNodeManager) SetOnSuperNodeElected(fn func(*SuperNode)) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.onSuperNodeElected = fn
}

// SetOnSuperNodeRemoved 设置超级节点移除回调
func (s *SuperNodeManager) SetOnSuperNodeRemoved(fn func(string)) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.onSuperNodeRemoved = fn
}

// SetOnAuditCompleted 设置审计完成回调
func (s *SuperNodeManager) SetOnAuditCompleted(fn func(*MultiAudit)) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.onAuditCompleted = fn
}

// SetOnAuditorDeviation 设置审计偏离回调（Task44: 用于触发Violation事件）
func (s *SuperNodeManager) SetOnAuditorDeviation(fn func(*AuditDeviation)) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.onAuditorDeviation = fn
}

// SetOnElectionStarted 设置选举开始回调
func (s *SuperNodeManager) SetOnElectionStarted(fn func(*Election)) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.onElectionStarted = fn
}

// SetOnElectionFinalized 设置选举结束回调
func (s *SuperNodeManager) SetOnElectionFinalized(fn func(*Election)) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.onElectionFinalized = fn
}

// Start 启动服务
func (s *SuperNodeManager) Start() error {
	if err := s.loadFromDisk(); err != nil {
		fmt.Printf("Warning: failed to load supernode data: %v\n", err)
	}

	s.wg.Add(1)
	go s.mainLoop()

	return nil
}

// Stop 停止服务
func (s *SuperNodeManager) Stop() error {
	close(s.stopCh)
	s.wg.Wait()
	return s.saveToDisk()
}

// === 候选与选举 ===

// ApplyCandidate 申请成为候选超级节点
func (s *SuperNodeManager) ApplyCandidate(nodeID string, reputation, stake float64) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if nodeID == "" {
		return errors.New("node ID is required")
	}

	// 检查是否已是超级节点
	if _, exists := s.superNodes[nodeID]; exists {
		return errors.New("node is already a super node")
	}

	// 检查是否已是候选
	if _, exists := s.candidates[nodeID]; exists {
		return errors.New("node is already a candidate")
	}

	// 检查资格
	if reputation < s.config.MinReputation {
		return fmt.Errorf("reputation too low: %.2f < %.2f", reputation, s.config.MinReputation)
	}
	if stake < s.config.MinStake {
		return fmt.Errorf("stake too low: %.2f < %.2f", stake, s.config.MinStake)
	}

	s.candidates[nodeID] = &Candidate{
		NodeID:     nodeID,
		Reputation: reputation,
		Stake:      stake,
		AppliedAt:  time.Now(),
		Supporters: make(map[string]float64),
	}

	return nil
}

// WithdrawCandidate 撤回候选资格
func (s *SuperNodeManager) WithdrawCandidate(nodeID string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if _, exists := s.candidates[nodeID]; !exists {
		return errors.New("node is not a candidate")
	}

	delete(s.candidates, nodeID)
	return nil
}

// VoteForCandidate 为候选人投票
func (s *SuperNodeManager) VoteForCandidate(voterID, candidateID string, weight float64) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	candidate, exists := s.candidates[candidateID]
	if !exists {
		return errors.New("candidate not found")
	}

	// 检查是否已投票
	if _, voted := candidate.Supporters[voterID]; voted {
		return errors.New("already voted for this candidate")
	}

	candidate.Supporters[voterID] = weight
	candidate.Votes += weight

	return nil
}

// StartElection 开始新一轮选举
func (s *SuperNodeManager) StartElection() (*Election, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	// 检查是否有进行中的选举
	if s.currentElection != nil && s.currentElection.Status == ElectionOpen {
		return nil, errors.New("election already in progress")
	}

	now := time.Now()
	election := &Election{
		ID:         s.generateElectionID(),
		StartAt:    now,
		EndAt:      now.Add(s.config.ElectionDuration),
		Candidates: make(map[string]*Candidate),
		Status:     ElectionOpen,
	}

	// 复制当前候选人到选举
	for id, c := range s.candidates {
		election.Candidates[id] = &Candidate{
			NodeID:     c.NodeID,
			Reputation: c.Reputation,
			Stake:      c.Stake,
			AppliedAt:  c.AppliedAt,
			Votes:      c.Votes,
			Supporters: make(map[string]float64),
		}
		for k, v := range c.Supporters {
			election.Candidates[id].Supporters[k] = v
		}
	}

	s.currentElection = election
	s.elections[election.ID] = election

	if s.onElectionStarted != nil {
		go s.onElectionStarted(election)
	}

	return election, nil
}

// FinalizeElection 结束选举并确定超级节点
func (s *SuperNodeManager) FinalizeElection() (*Election, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	if s.currentElection == nil {
		return nil, errors.New("no current election")
	}

	if s.currentElection.Status != ElectionOpen {
		return nil, errors.New("election is not open")
	}

	election := s.currentElection

	// 按票数排序候选人
	var sortedCandidates []*Candidate
	for _, c := range election.Candidates {
		sortedCandidates = append(sortedCandidates, c)
	}
	sort.Slice(sortedCandidates, func(i, j int) bool {
		return sortedCandidates[i].Votes > sortedCandidates[j].Votes
	})

	// 选出前N名
	winnerCount := s.config.MaxSuperNodes
	if len(sortedCandidates) < winnerCount {
		winnerCount = len(sortedCandidates)
	}

	now := time.Now()
	for i := 0; i < winnerCount; i++ {
		c := sortedCandidates[i]
		if c.Votes <= 0 {
			continue // 没有票的不能当选
		}

		superNode := &SuperNode{
			NodeID:        c.NodeID,
			Reputation:    c.Reputation,
			Stake:         c.Stake,
			ElectedAt:     now,
			TermEndsAt:    now.Add(s.config.TermDuration),
			VotesReceived: c.Votes,
			IsActive:      true,
		}

		s.superNodes[c.NodeID] = superNode
		election.Winners = append(election.Winners, c.NodeID)

		// 从候选人中移除
		delete(s.candidates, c.NodeID)

		if s.onSuperNodeElected != nil {
			go s.onSuperNodeElected(superNode)
		}
	}

	election.Status = ElectionFinalized
	s.currentElection = nil

	if s.onElectionFinalized != nil {
		go s.onElectionFinalized(election)
	}

	return election, nil
}

// GetCurrentElection 获取当前选举
func (s *SuperNodeManager) GetCurrentElection() *Election {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.currentElection
}

// === 超级节点管理 ===

// IsSuperNode 检查是否为超级节点
func (s *SuperNodeManager) IsSuperNode(nodeID string) bool {
	s.mu.RLock()
	defer s.mu.RUnlock()
	
	sn, exists := s.superNodes[nodeID]
	return exists && sn.IsActive
}

// GetSuperNode 获取超级节点信息
func (s *SuperNodeManager) GetSuperNode(nodeID string) (*SuperNode, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	sn, exists := s.superNodes[nodeID]
	if !exists {
		return nil, errors.New("super node not found")
	}

	copy := *sn
	return &copy, nil
}

// GetActiveSuperNodes 获取所有活跃超级节点
func (s *SuperNodeManager) GetActiveSuperNodes() []*SuperNode {
	s.mu.RLock()
	defer s.mu.RUnlock()

	var result []*SuperNode
	for _, sn := range s.superNodes {
		if sn.IsActive {
			copy := *sn
			result = append(result, &copy)
		}
	}
	return result
}

// RemoveSuperNode 移除超级节点
func (s *SuperNodeManager) RemoveSuperNode(nodeID string, reason string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	sn, exists := s.superNodes[nodeID]
	if !exists {
		return errors.New("super node not found")
	}

	sn.IsActive = false

	if s.onSuperNodeRemoved != nil {
		go s.onSuperNodeRemoved(nodeID)
	}

	return nil
}

// GetNodeRole 获取节点角色
func (s *SuperNodeManager) GetNodeRole(nodeID string) NodeRole {
	s.mu.RLock()
	defer s.mu.RUnlock()

	if sn, exists := s.superNodes[nodeID]; exists && sn.IsActive {
		return RoleSuper
	}
	if _, exists := s.candidates[nodeID]; exists {
		return RoleCandidate
	}
	return RoleNormal
}

// === 审计功能 ===

// CreateAudit 创建多节点审计任务
func (s *SuperNodeManager) CreateAudit(auditType AuditType, targetID string) (*MultiAudit, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	if targetID == "" {
		return nil, errors.New("target ID is required")
	}

	// 获取活跃超级节点
	var activeSuperNodes []string
	for id, sn := range s.superNodes {
		if sn.IsActive {
			activeSuperNodes = append(activeSuperNodes, id)
		}
	}

	if len(activeSuperNodes) == 0 {
		return nil, errors.New("no active super nodes available")
	}

	// 随机选择审计者
	auditorCount := s.config.AuditorsPerTask
	if len(activeSuperNodes) < auditorCount {
		auditorCount = len(activeSuperNodes)
	}

	// 打乱顺序
	s.rng.Shuffle(len(activeSuperNodes), func(i, j int) {
		activeSuperNodes[i], activeSuperNodes[j] = activeSuperNodes[j], activeSuperNodes[i]
	})

	auditors := activeSuperNodes[:auditorCount]

	now := time.Now()
	audit := &MultiAudit{
		ID:        s.generateAuditID(auditType, targetID),
		Type:      auditType,
		TargetID:  targetID,
		CreatedAt: now,
		ExpiresAt: now.Add(24 * time.Hour), // 24小时内完成审计
		Auditors:  auditors,
		Results:   make(map[string]*AuditRecord),
		FinalResult: ResultPending,
	}

	s.audits[audit.ID] = audit
	return audit, nil
}

// SubmitAuditResult 提交审计结果
func (s *SuperNodeManager) SubmitAuditResult(auditID, auditorID string, result AuditResult, evidence string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	audit, exists := s.audits[auditID]
	if !exists {
		return errors.New("audit not found")
	}

	if audit.Finalized {
		return errors.New("audit already finalized")
	}

	// 检查是否是指定的审计者
	isAuditor := false
	for _, a := range audit.Auditors {
		if a == auditorID {
			isAuditor = true
			break
		}
	}
	if !isAuditor {
		return errors.New("not an assigned auditor")
	}

	// 检查是否已提交
	if _, submitted := audit.Results[auditorID]; submitted {
		return errors.New("audit result already submitted")
	}

	// 创建审计记录
	record := &AuditRecord{
		ID:        s.generateRecordID(auditID, auditorID),
		Type:      audit.Type,
		TargetID:  audit.TargetID,
		AuditorID: auditorID,
		Result:    result,
		Evidence:  evidence,
		Timestamp: time.Now(),
	}

	// 签名
	if s.signFunc != nil {
		signData := s.getAuditSignData(record)
		sig, err := s.signFunc(signData)
		if err != nil {
			return fmt.Errorf("failed to sign: %w", err)
		}
		record.Signature = sig
	}

	audit.Results[auditorID] = record

	// 更新审计者统计
	if sn, exists := s.superNodes[auditorID]; exists {
		sn.AuditCount++
	}

	// 检查是否可以结束
	s.tryFinalizeAudit(audit)

	return nil
}

// GetAudit 获取审计任务
func (s *SuperNodeManager) GetAudit(auditID string) (*MultiAudit, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	audit, exists := s.audits[auditID]
	if !exists {
		return nil, errors.New("audit not found")
	}
	return audit, nil
}

// GetPendingAudits 获取待处理的审计任务
func (s *SuperNodeManager) GetPendingAudits(auditorID string) []*MultiAudit {
	s.mu.RLock()
	defer s.mu.RUnlock()

	var result []*MultiAudit
	for _, audit := range s.audits {
		if audit.Finalized {
			continue
		}
		// 检查是否是该审计者的任务
		for _, a := range audit.Auditors {
			if a == auditorID {
				// 检查是否已提交
				if _, submitted := audit.Results[auditorID]; !submitted {
					result = append(result, audit)
				}
				break
			}
		}
	}
	return result
}

// === 内部方法 ===

// tryFinalizeAudit 尝试结束审计
func (s *SuperNodeManager) tryFinalizeAudit(audit *MultiAudit) {
	if audit.Finalized {
		return
	}

	// 检查是否所有审计者都已提交
	if len(audit.Results) < len(audit.Auditors) {
		return
	}

	// 统计结果
	passCount := 0
	failCount := 0
	for _, record := range audit.Results {
		switch record.Result {
		case ResultPass:
			passCount++
		case ResultFail:
			failCount++
		}
	}

	total := len(audit.Results)
	passRatio := float64(passCount) / float64(total)

	if passRatio >= s.config.AuditThreshold {
		audit.FinalResult = ResultPass
	} else {
		audit.FinalResult = ResultFail
	}

	audit.Finalized = true

	// 更新审计者通过率 & 检测偏离（Task44: 审计偏离惩罚闭环）
	audit.Deviations = make([]AuditDeviation, 0)
	for auditorID, record := range audit.Results {
		if sn, exists := s.superNodes[auditorID]; exists {
			// 如果审计者的结果与最终结果一致，增加其通过率
			if record.Result == audit.FinalResult {
				sn.PassRate = (sn.PassRate*float64(sn.AuditCount-1) + 1) / float64(sn.AuditCount)
			} else {
				sn.PassRate = sn.PassRate * float64(sn.AuditCount-1) / float64(sn.AuditCount)
				// Task44: 记录偏离并触发回调
				severity := "minor"
				// 严重偏离: 结果完全相反 (pass vs fail)
				if (record.Result == ResultPass && audit.FinalResult == ResultFail) ||
					(record.Result == ResultFail && audit.FinalResult == ResultPass) {
					severity = "severe"
				}
				deviation := AuditDeviation{
					AuditID:        audit.ID,
					AuditorID:      auditorID,
					ExpectedResult: audit.FinalResult,
					ActualResult:   record.Result,
					Severity:       severity,
					DetectedAt:     time.Now(),
				}
				audit.Deviations = append(audit.Deviations, deviation)
				// 触发偏离回调（可用于Emit EventViolation / SlashCollateral）
				if s.onAuditorDeviation != nil {
					go s.onAuditorDeviation(&deviation)
				}
			}
		}
	}

	if s.onAuditCompleted != nil {
		go s.onAuditCompleted(audit)
	}
}

// checkTermExpiry 检查任期过期
func (s *SuperNodeManager) checkTermExpiry() {
	s.mu.Lock()
	defer s.mu.Unlock()

	now := time.Now()
	for nodeID, sn := range s.superNodes {
		if sn.IsActive && now.After(sn.TermEndsAt) {
			sn.IsActive = false
			if s.onSuperNodeRemoved != nil {
				go s.onSuperNodeRemoved(nodeID)
			}
		}
	}
}

// generateElectionID 生成选举ID
func (s *SuperNodeManager) generateElectionID() string {
	data := fmt.Sprintf("election|%d", time.Now().UnixNano())
	hash := sha256.Sum256([]byte(data))
	return hex.EncodeToString(hash[:16])
}

// generateAuditID 生成审计ID
func (s *SuperNodeManager) generateAuditID(auditType AuditType, targetID string) string {
	data := fmt.Sprintf("%s|%s|%d", auditType, targetID, time.Now().UnixNano())
	hash := sha256.Sum256([]byte(data))
	return hex.EncodeToString(hash[:16])
}

// generateRecordID 生成审计记录ID
func (s *SuperNodeManager) generateRecordID(auditID, auditorID string) string {
	data := fmt.Sprintf("%s|%s|%d", auditID, auditorID, time.Now().UnixNano())
	hash := sha256.Sum256([]byte(data))
	return hex.EncodeToString(hash[:16])
}

// getAuditSignData 获取审计签名数据
func (s *SuperNodeManager) getAuditSignData(record *AuditRecord) []byte {
	data := fmt.Sprintf("%s|%s|%s|%s|%d",
		record.ID,
		record.AuditorID,
		record.TargetID,
		record.Result,
		record.Timestamp.UnixNano(),
	)
	return []byte(data)
}

// mainLoop 主循环
func (s *SuperNodeManager) mainLoop() {
	defer s.wg.Done()

	ticker := time.NewTicker(s.config.CleanupInterval)
	termTicker := time.NewTicker(1 * time.Hour) // 每小时检查任期
	defer ticker.Stop()
	defer termTicker.Stop()

	for {
		select {
		case <-ticker.C:
			s.cleanup()
		case <-termTicker.C:
			s.checkTermExpiry()
		case <-s.stopCh:
			return
		}
	}
}

// cleanup 清理旧数据
func (s *SuperNodeManager) cleanup() {
	s.mu.Lock()
	defer s.mu.Unlock()

	// 清理30天前已结束的审计
	cutoff := time.Now().Add(-30 * 24 * time.Hour)
	for id, audit := range s.audits {
		if audit.Finalized && audit.CreatedAt.Before(cutoff) {
			delete(s.audits, id)
		}
	}

	// 清理过期的选举
	for id, election := range s.elections {
		if election.Status == ElectionFinalized && election.EndAt.Before(cutoff) {
			delete(s.elections, id)
		}
	}
}

// === 持久化 ===

type supernodeData struct {
	SuperNodes map[string]*SuperNode  `json:"super_nodes"`
	Candidates map[string]*Candidate  `json:"candidates"`
	Audits     map[string]*MultiAudit `json:"audits"`
	Elections  map[string]*Election   `json:"elections"`
}

func (s *SuperNodeManager) saveToDisk() error {
	if s.config.DataDir == "" {
		return nil
	}

	s.mu.RLock()
	data := &supernodeData{
		SuperNodes: s.superNodes,
		Candidates: s.candidates,
		Audits:     s.audits,
		Elections:  s.elections,
	}
	s.mu.RUnlock()

	jsonData, err := json.MarshalIndent(data, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to marshal: %w", err)
	}

	filePath := filepath.Join(s.config.DataDir, "supernode.json")
	return os.WriteFile(filePath, jsonData, 0644)
}

func (s *SuperNodeManager) loadFromDisk() error {
	if s.config.DataDir == "" {
		return nil
	}

	filePath := filepath.Join(s.config.DataDir, "supernode.json")
	if _, err := os.Stat(filePath); os.IsNotExist(err) {
		return nil
	}

	jsonData, err := os.ReadFile(filePath)
	if err != nil {
		return fmt.Errorf("failed to read: %w", err)
	}

	var data supernodeData
	if err := json.Unmarshal(jsonData, &data); err != nil {
		return fmt.Errorf("failed to unmarshal: %w", err)
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	if data.SuperNodes != nil {
		s.superNodes = data.SuperNodes
	}
	if data.Candidates != nil {
		s.candidates = data.Candidates
	}
	if data.Audits != nil {
		s.audits = data.Audits
	}
	if data.Elections != nil {
		s.elections = data.Elections
	}

	return nil
}

// === 统计信息 ===

// SuperNodeStats 超级节点统计
type SuperNodeStats struct {
	TotalSuperNodes   int     `json:"total_super_nodes"`
	ActiveSuperNodes  int     `json:"active_super_nodes"`
	TotalCandidates   int     `json:"total_candidates"`
	TotalAudits       int     `json:"total_audits"`
	CompletedAudits   int     `json:"completed_audits"`
	AveragePassRate   float64 `json:"average_pass_rate"`
}

// GetStats 获取统计信息
func (s *SuperNodeManager) GetStats() *SuperNodeStats {
	s.mu.RLock()
	defer s.mu.RUnlock()

	stats := &SuperNodeStats{
		TotalSuperNodes: len(s.superNodes),
		TotalCandidates: len(s.candidates),
		TotalAudits:     len(s.audits),
	}

	var totalPassRate float64
	for _, sn := range s.superNodes {
		if sn.IsActive {
			stats.ActiveSuperNodes++
			totalPassRate += sn.PassRate
		}
	}

	if stats.ActiveSuperNodes > 0 {
		stats.AveragePassRate = totalPassRate / float64(stats.ActiveSuperNodes)
	}

	for _, audit := range s.audits {
		if audit.Finalized {
			stats.CompletedAudits++
		}
	}

	return stats
}

// GetCandidates 获取所有候选人
func (s *SuperNodeManager) GetCandidates() []*Candidate {
	s.mu.RLock()
	defer s.mu.RUnlock()

	var result []*Candidate
	for _, c := range s.candidates {
		copy := *c
		result = append(result, &copy)
	}

	// 按票数排序
	sort.Slice(result, func(i, j int) bool {
		return result[i].Votes > result[j].Votes
	})

	return result
}
