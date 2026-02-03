// Package voting 实现节点投票机制
// 包括投票发起、权重计算、阈值判断和剔除状态管理
package voting

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"sync"
	"time"
)

// VoteType 投票类型
type VoteType string

const (
	VoteKick      VoteType = "kick"      // 剔除投票
	VoteRestore   VoteType = "restore"   // 恢复投票
	VotePromote   VoteType = "promote"   // 晋升投票（如超级节点）
	VoteDemote    VoteType = "demote"    // 降级投票
	VoteProposal  VoteType = "proposal"  // 提案投票
)

// VoteChoice 投票选择
type VoteChoice string

const (
	ChoiceYes     VoteChoice = "yes"
	ChoiceNo      VoteChoice = "no"
	ChoiceAbstain VoteChoice = "abstain"
)

// NodeStatus 节点状态
type NodeStatus string

const (
	StatusActive    NodeStatus = "active"    // 活跃
	StatusSuspended NodeStatus = "suspended" // 暂停
	StatusRemoved   NodeStatus = "removed"   // 已剔除
	StatusPending   NodeStatus = "pending"   // 待审核
)

// Vote 单个投票记录
type Vote struct {
	ID          string     `json:"id"`           // 投票ID
	ProposalID  string     `json:"proposal_id"`  // 提案ID
	VoterID     string     `json:"voter_id"`     // 投票者节点ID
	Choice      VoteChoice `json:"choice"`       // 投票选择
	Weight      float64    `json:"weight"`       // 投票权重
	Timestamp   time.Time  `json:"timestamp"`    // 投票时间
	Reason      string     `json:"reason"`       // 投票理由（可选）
	Signature   []byte     `json:"signature"`    // 投票签名
}

// Proposal 投票提案
type Proposal struct {
	ID           string           `json:"id"`             // 提案ID
	Type         VoteType         `json:"type"`           // 投票类型
	TargetNodeID string           `json:"target_node_id"` // 目标节点ID
	ProposerID   string           `json:"proposer_id"`    // 发起者节点ID
	Reason       string           `json:"reason"`         // 提案理由
	CreatedAt    time.Time        `json:"created_at"`     // 创建时间
	ExpiresAt    time.Time        `json:"expires_at"`     // 过期时间
	Votes        map[string]*Vote `json:"votes"`          // 投票记录: voterID -> Vote
	Status       ProposalStatus   `json:"status"`         // 提案状态
	Result       *ProposalResult  `json:"result,omitempty"` // 提案结果
}

// ProposalStatus 提案状态
type ProposalStatus string

const (
	ProposalPending  ProposalStatus = "pending"  // 进行中
	ProposalPassed   ProposalStatus = "passed"   // 已通过
	ProposalRejected ProposalStatus = "rejected" // 已拒绝
	ProposalExpired  ProposalStatus = "expired"  // 已过期
)

// ProposalResult 提案结果
type ProposalResult struct {
	TotalWeight   float64   `json:"total_weight"`
	YesWeight     float64   `json:"yes_weight"`
	NoWeight      float64   `json:"no_weight"`
	AbstainWeight float64   `json:"abstain_weight"`
	YesRatio      float64   `json:"yes_ratio"`
	Passed        bool      `json:"passed"`
	FinalizedAt   time.Time `json:"finalized_at"`
}

// NodeTrust 节点信任信息
type NodeTrust struct {
	NodeID     string     `json:"node_id"`
	Reputation float64    `json:"reputation"`  // 信誉分 [0, 100]
	Stake      float64    `json:"stake"`       // 抵押分 [0, 100]
	Status     NodeStatus `json:"status"`
	JoinedAt   time.Time  `json:"joined_at"`
	LastActive time.Time  `json:"last_active"`
	VoteCount  int        `json:"vote_count"`  // 参与投票次数
}

// SignFunc 签名函数类型
type SignFunc func(data []byte) ([]byte, error)

// VerifyFunc 验签函数类型
type VerifyFunc func(pubKey string, data, signature []byte) (bool, error)

// GetReputationFunc 获取信誉分函数类型
type GetReputationFunc func(nodeID string) float64

// VotingConfig 投票配置
type VotingConfig struct {
	NodeID            string        // 当前节点ID
	DataDir           string        // 数据目录
	PassThreshold     float64       // 通过阈值 (0-1)
	QuorumThreshold   float64       // 法定人数阈值 (0-1)
	ProposalDuration  time.Duration // 提案持续时间
	BufferPeriod      time.Duration // 缓冲期（防止突发操纵）
	ReputationWeight  float64       // 信誉权重系数 α
	StakeWeight       float64       // 抵押权重系数 β
	MinRepToVote      float64       // 最低投票信誉要求
	MinRepToPropose   float64       // 最低提案信誉要求
	CleanupInterval   time.Duration // 清理间隔
}

// DefaultConfig 返回默认配置
func DefaultConfig(nodeID string) *VotingConfig {
	return &VotingConfig{
		NodeID:            nodeID,
		DataDir:           "./data/voting",
		PassThreshold:     0.6,         // 60%通过
		QuorumThreshold:   0.3,         // 30%参与
		ProposalDuration:  30 * time.Minute,
		BufferPeriod:      5 * time.Minute,
		ReputationWeight:  0.7,         // α = 0.7
		StakeWeight:       0.3,         // β = 0.3
		MinRepToVote:      10,          // 最低10分可投票
		MinRepToPropose:   30,          // 最低30分可发起提案
		CleanupInterval:   1 * time.Hour,
	}
}

// VotingManager 投票管理器
type VotingManager struct {
	config    *VotingConfig
	proposals map[string]*Proposal // proposalID -> Proposal
	nodes     map[string]*NodeTrust // nodeID -> NodeTrust
	mu        sync.RWMutex

	signFunc      SignFunc
	verifyFunc    VerifyFunc
	getReputation GetReputationFunc

	// 回调
	onProposalCreated func(*Proposal)
	onVoteCast        func(*Vote)
	onProposalFinalized func(*Proposal)
	onNodeKicked      func(nodeID string)
	onNodeRestored    func(nodeID string)

	stopCh chan struct{}
	wg     sync.WaitGroup
}

// NewVotingManager 创建投票管理器
func NewVotingManager(config *VotingConfig) (*VotingManager, error) {
	if config == nil {
		return nil, errors.New("config is required")
	}
	if config.NodeID == "" {
		return nil, errors.New("node ID is required")
	}
	if config.PassThreshold <= 0 || config.PassThreshold > 1 {
		return nil, errors.New("pass threshold must be between 0 and 1")
	}

	if config.DataDir != "" {
		if err := os.MkdirAll(config.DataDir, 0755); err != nil {
			return nil, fmt.Errorf("failed to create data dir: %w", err)
		}
	}

	vm := &VotingManager{
		config:    config,
		proposals: make(map[string]*Proposal),
		nodes:     make(map[string]*NodeTrust),
		stopCh:    make(chan struct{}),
	}

	return vm, nil
}

// SetSignFunc 设置签名函数
func (v *VotingManager) SetSignFunc(fn SignFunc) {
	v.mu.Lock()
	defer v.mu.Unlock()
	v.signFunc = fn
}

// SetVerifyFunc 设置验签函数
func (v *VotingManager) SetVerifyFunc(fn VerifyFunc) {
	v.mu.Lock()
	defer v.mu.Unlock()
	v.verifyFunc = fn
}

// SetGetReputationFunc 设置获取信誉分函数
func (v *VotingManager) SetGetReputationFunc(fn GetReputationFunc) {
	v.mu.Lock()
	defer v.mu.Unlock()
	v.getReputation = fn
}

// SetOnProposalCreated 设置提案创建回调
func (v *VotingManager) SetOnProposalCreated(fn func(*Proposal)) {
	v.mu.Lock()
	defer v.mu.Unlock()
	v.onProposalCreated = fn
}

// SetOnVoteCast 设置投票回调
func (v *VotingManager) SetOnVoteCast(fn func(*Vote)) {
	v.mu.Lock()
	defer v.mu.Unlock()
	v.onVoteCast = fn
}

// SetOnProposalFinalized 设置提案结束回调
func (v *VotingManager) SetOnProposalFinalized(fn func(*Proposal)) {
	v.mu.Lock()
	defer v.mu.Unlock()
	v.onProposalFinalized = fn
}

// SetOnNodeKicked 设置节点剔除回调
func (v *VotingManager) SetOnNodeKicked(fn func(string)) {
	v.mu.Lock()
	defer v.mu.Unlock()
	v.onNodeKicked = fn
}

// SetOnNodeRestored 设置节点恢复回调
func (v *VotingManager) SetOnNodeRestored(fn func(string)) {
	v.mu.Lock()
	defer v.mu.Unlock()
	v.onNodeRestored = fn
}

// Start 启动投票服务
func (v *VotingManager) Start() error {
	if err := v.loadFromDisk(); err != nil {
		fmt.Printf("Warning: failed to load voting data: %v\n", err)
	}

	v.wg.Add(1)
	go v.mainLoop()

	return nil
}

// Stop 停止投票服务
func (v *VotingManager) Stop() error {
	close(v.stopCh)
	v.wg.Wait()
	return v.saveToDisk()
}

// RegisterNode 注册节点
func (v *VotingManager) RegisterNode(nodeID string, reputation, stake float64) error {
	v.mu.Lock()
	defer v.mu.Unlock()

	if nodeID == "" {
		return errors.New("node ID is required")
	}

	if _, exists := v.nodes[nodeID]; exists {
		return errors.New("node already registered")
	}

	v.nodes[nodeID] = &NodeTrust{
		NodeID:     nodeID,
		Reputation: reputation,
		Stake:      stake,
		Status:     StatusActive,
		JoinedAt:   time.Now(),
		LastActive: time.Now(),
	}

	return nil
}

// UpdateNodeTrust 更新节点信任信息
func (v *VotingManager) UpdateNodeTrust(nodeID string, reputation, stake float64) error {
	v.mu.Lock()
	defer v.mu.Unlock()

	node, exists := v.nodes[nodeID]
	if !exists {
		return errors.New("node not found")
	}

	node.Reputation = reputation
	node.Stake = stake
	node.LastActive = time.Now()

	return nil
}

// GetNodeTrust 获取节点信任信息
func (v *VotingManager) GetNodeTrust(nodeID string) (*NodeTrust, error) {
	v.mu.RLock()
	defer v.mu.RUnlock()

	node, exists := v.nodes[nodeID]
	if !exists {
		return nil, errors.New("node not found")
	}

	// 返回副本
	copy := *node
	return &copy, nil
}

// GetNodeStatus 获取节点状态
func (v *VotingManager) GetNodeStatus(nodeID string) NodeStatus {
	v.mu.RLock()
	defer v.mu.RUnlock()

	if node, exists := v.nodes[nodeID]; exists {
		return node.Status
	}
	return StatusPending
}

// CreateProposal 创建投票提案
func (v *VotingManager) CreateProposal(voteType VoteType, targetNodeID, reason string) (*Proposal, error) {
	v.mu.Lock()
	defer v.mu.Unlock()

	// 检查发起者信誉
	proposerRep := v.getNodeReputation(v.config.NodeID)
	if proposerRep < v.config.MinRepToPropose {
		return nil, fmt.Errorf("reputation too low to propose: %.2f < %.2f", proposerRep, v.config.MinRepToPropose)
	}

	// 检查目标节点
	if targetNodeID == "" {
		return nil, errors.New("target node ID is required")
	}

	// 检查是否已有进行中的相同提案
	for _, p := range v.proposals {
		if p.Status == ProposalPending && 
		   p.Type == voteType && 
		   p.TargetNodeID == targetNodeID {
			return nil, errors.New("similar proposal already pending")
		}
	}

	now := time.Now()
	proposal := &Proposal{
		Type:         voteType,
		TargetNodeID: targetNodeID,
		ProposerID:   v.config.NodeID,
		Reason:       reason,
		CreatedAt:    now,
		ExpiresAt:    now.Add(v.config.ProposalDuration),
		Votes:        make(map[string]*Vote),
		Status:       ProposalPending,
	}

	// 生成提案ID
	proposal.ID = v.generateProposalID(proposal)

	// 存储提案
	v.proposals[proposal.ID] = proposal

	// 触发回调
	if v.onProposalCreated != nil {
		go v.onProposalCreated(proposal)
	}

	return proposal, nil
}

// CastVote 投票
func (v *VotingManager) CastVote(proposalID string, choice VoteChoice, reason string) (*Vote, error) {
	v.mu.Lock()
	defer v.mu.Unlock()

	// 检查投票者信誉
	voterRep := v.getNodeReputation(v.config.NodeID)
	if voterRep < v.config.MinRepToVote {
		return nil, fmt.Errorf("reputation too low to vote: %.2f < %.2f", voterRep, v.config.MinRepToVote)
	}

	// 检查提案
	proposal, exists := v.proposals[proposalID]
	if !exists {
		return nil, errors.New("proposal not found")
	}

	// 检查提案状态
	if proposal.Status != ProposalPending {
		return nil, fmt.Errorf("proposal is not pending: %s", proposal.Status)
	}

	// 检查是否过期
	if time.Now().After(proposal.ExpiresAt) {
		proposal.Status = ProposalExpired
		return nil, errors.New("proposal has expired")
	}

	// 检查是否已投票
	if _, voted := proposal.Votes[v.config.NodeID]; voted {
		return nil, errors.New("already voted")
	}

	// 检查缓冲期（防止刚创建就投票）
	if time.Since(proposal.CreatedAt) < v.config.BufferPeriod {
		return nil, fmt.Errorf("proposal is in buffer period, please wait %v", 
			v.config.BufferPeriod - time.Since(proposal.CreatedAt))
	}

	// 计算投票权重
	weight := v.calculateVoteWeight(v.config.NodeID)

	// 创建投票
	vote := &Vote{
		ProposalID: proposalID,
		VoterID:    v.config.NodeID,
		Choice:     choice,
		Weight:     weight,
		Timestamp:  time.Now(),
		Reason:     reason,
	}

	// 生成投票ID
	vote.ID = v.generateVoteID(vote)

	// 签名
	if v.signFunc != nil {
		signData := v.getVoteSignData(vote)
		sig, err := v.signFunc(signData)
		if err != nil {
			return nil, fmt.Errorf("failed to sign vote: %w", err)
		}
		vote.Signature = sig
	}

	// 记录投票
	proposal.Votes[v.config.NodeID] = vote

	// 更新投票者统计
	if node, exists := v.nodes[v.config.NodeID]; exists {
		node.VoteCount++
	}

	// 触发回调
	if v.onVoteCast != nil {
		go v.onVoteCast(vote)
	}

	// 检查是否可以提前结束
	v.tryFinalizeProposal(proposal)

	return vote, nil
}

// ReceiveVote 接收其他节点的投票（用于投票传播）
func (v *VotingManager) ReceiveVote(vote *Vote) error {
	if vote == nil {
		return errors.New("vote is nil")
	}

	v.mu.Lock()
	defer v.mu.Unlock()

	// 检查提案
	proposal, exists := v.proposals[vote.ProposalID]
	if !exists {
		return errors.New("proposal not found")
	}

	// 检查提案状态
	if proposal.Status != ProposalPending {
		return fmt.Errorf("proposal is not pending: %s", proposal.Status)
	}

	// 检查是否已有此节点的投票
	if _, voted := proposal.Votes[vote.VoterID]; voted {
		return errors.New("vote already recorded")
	}

	// 验证签名
	if v.verifyFunc != nil && len(vote.Signature) > 0 {
		signData := v.getVoteSignData(vote)
		valid, err := v.verifyFunc(vote.VoterID, signData, vote.Signature)
		if err != nil {
			return fmt.Errorf("failed to verify signature: %w", err)
		}
		if !valid {
			return errors.New("invalid vote signature")
		}
	}

	// 验证投票者信誉
	voterRep := v.getNodeReputation(vote.VoterID)
	if voterRep < v.config.MinRepToVote {
		return fmt.Errorf("voter reputation too low: %.2f < %.2f", voterRep, v.config.MinRepToVote)
	}

	// 记录投票
	proposal.Votes[vote.VoterID] = vote

	// 尝试结束
	v.tryFinalizeProposal(proposal)

	return nil
}

// GetProposal 获取提案
func (v *VotingManager) GetProposal(proposalID string) (*Proposal, error) {
	v.mu.RLock()
	defer v.mu.RUnlock()

	proposal, exists := v.proposals[proposalID]
	if !exists {
		return nil, errors.New("proposal not found")
	}

	return proposal, nil
}

// ListProposals 列出提案
func (v *VotingManager) ListProposals(status ProposalStatus, limit, offset int) []*Proposal {
	v.mu.RLock()
	defer v.mu.RUnlock()

	var result []*Proposal
	for _, p := range v.proposals {
		if status == "" || p.Status == status {
			result = append(result, p)
		}
	}

	// 按创建时间倒序
	sort.Slice(result, func(i, j int) bool {
		return result[i].CreatedAt.After(result[j].CreatedAt)
	})

	// 分页
	if offset >= len(result) {
		return nil
	}
	end := offset + limit
	if end > len(result) || limit <= 0 {
		end = len(result)
	}

	return result[offset:end]
}

// GetActiveProposals 获取进行中的提案
func (v *VotingManager) GetActiveProposals() []*Proposal {
	return v.ListProposals(ProposalPending, 0, 0)
}

// === 内部方法 ===

// calculateVoteWeight 计算投票权重
func (v *VotingManager) calculateVoteWeight(nodeID string) float64 {
	// VoteWeight = α * Reputation + β * Stake
	node, exists := v.nodes[nodeID]
	if !exists {
		// 使用外部获取信誉函数
		rep := v.getNodeReputation(nodeID)
		return v.config.ReputationWeight * rep
	}

	return v.config.ReputationWeight * node.Reputation + 
	       v.config.StakeWeight * node.Stake
}

// getNodeReputation 获取节点信誉
func (v *VotingManager) getNodeReputation(nodeID string) float64 {
	// 优先使用本地存储
	if node, exists := v.nodes[nodeID]; exists {
		return node.Reputation
	}

	// 使用外部函数
	if v.getReputation != nil {
		return v.getReputation(nodeID)
	}

	// 默认值
	return 0
}

// tryFinalizeProposal 尝试结束提案
func (v *VotingManager) tryFinalizeProposal(proposal *Proposal) {
	if proposal.Status != ProposalPending {
		return
	}

	// 计算投票结果
	result := v.calculateResult(proposal)

	// 计算总权重（所有已注册节点）
	totalPossibleWeight := v.calculateTotalPossibleWeight()
	if totalPossibleWeight <= 0 {
		return
	}

	// 检查法定人数
	quorum := result.TotalWeight / totalPossibleWeight
	if quorum < v.config.QuorumThreshold {
		return // 法定人数不足，继续等待
	}

	// 计算通过率
	result.YesRatio = 0
	if result.TotalWeight > 0 {
		result.YesRatio = result.YesWeight / result.TotalWeight
	}

	// 检查是否通过
	result.Passed = result.YesRatio >= v.config.PassThreshold
	result.FinalizedAt = time.Now()

	// 更新提案状态
	proposal.Result = result
	if result.Passed {
		proposal.Status = ProposalPassed
		v.applyProposalResult(proposal)
	} else {
		proposal.Status = ProposalRejected
	}

	// 触发回调
	if v.onProposalFinalized != nil {
		go v.onProposalFinalized(proposal)
	}
}

// calculateResult 计算投票结果
func (v *VotingManager) calculateResult(proposal *Proposal) *ProposalResult {
	result := &ProposalResult{}

	for _, vote := range proposal.Votes {
		result.TotalWeight += vote.Weight
		switch vote.Choice {
		case ChoiceYes:
			result.YesWeight += vote.Weight
		case ChoiceNo:
			result.NoWeight += vote.Weight
		case ChoiceAbstain:
			result.AbstainWeight += vote.Weight
		}
	}

	return result
}

// calculateTotalPossibleWeight 计算总可能权重
func (v *VotingManager) calculateTotalPossibleWeight() float64 {
	var total float64
	for nodeID, node := range v.nodes {
		if node.Status == StatusActive {
			total += v.calculateVoteWeight(nodeID)
		}
	}
	return total
}

// applyProposalResult 应用提案结果
func (v *VotingManager) applyProposalResult(proposal *Proposal) {
	switch proposal.Type {
	case VoteKick:
		if node, exists := v.nodes[proposal.TargetNodeID]; exists {
			node.Status = StatusRemoved
		}
		if v.onNodeKicked != nil {
			go v.onNodeKicked(proposal.TargetNodeID)
		}

	case VoteRestore:
		if node, exists := v.nodes[proposal.TargetNodeID]; exists {
			node.Status = StatusActive
		}
		if v.onNodeRestored != nil {
			go v.onNodeRestored(proposal.TargetNodeID)
		}

	case VotePromote, VoteDemote, VoteProposal:
		// 这些类型由外部处理
	}
}

// generateProposalID 生成提案ID
func (v *VotingManager) generateProposalID(proposal *Proposal) string {
	data := fmt.Sprintf("%s|%s|%s|%d",
		proposal.Type,
		proposal.TargetNodeID,
		proposal.ProposerID,
		proposal.CreatedAt.UnixNano(),
	)
	hash := sha256.Sum256([]byte(data))
	return hex.EncodeToString(hash[:16])
}

// generateVoteID 生成投票ID
func (v *VotingManager) generateVoteID(vote *Vote) string {
	data := fmt.Sprintf("%s|%s|%s|%d",
		vote.ProposalID,
		vote.VoterID,
		vote.Choice,
		vote.Timestamp.UnixNano(),
	)
	hash := sha256.Sum256([]byte(data))
	return hex.EncodeToString(hash[:16])
}

// getVoteSignData 获取投票签名数据
func (v *VotingManager) getVoteSignData(vote *Vote) []byte {
	data := fmt.Sprintf("%s|%s|%s|%.6f|%d",
		vote.ProposalID,
		vote.VoterID,
		vote.Choice,
		vote.Weight,
		vote.Timestamp.UnixNano(),
	)
	return []byte(data)
}

// mainLoop 主循环
func (v *VotingManager) mainLoop() {
	defer v.wg.Done()

	ticker := time.NewTicker(v.config.CleanupInterval)
	checkTicker := time.NewTicker(1 * time.Minute) // 检查过期提案
	defer ticker.Stop()
	defer checkTicker.Stop()

	for {
		select {
		case <-ticker.C:
			v.cleanup()
		case <-checkTicker.C:
			v.checkExpiredProposals()
		case <-v.stopCh:
			return
		}
	}
}

// checkExpiredProposals 检查过期提案
func (v *VotingManager) checkExpiredProposals() {
	v.mu.Lock()
	defer v.mu.Unlock()

	now := time.Now()
	for _, proposal := range v.proposals {
		if proposal.Status == ProposalPending && now.After(proposal.ExpiresAt) {
			proposal.Status = ProposalExpired
			proposal.Result = v.calculateResult(proposal)
			proposal.Result.FinalizedAt = now

			if v.onProposalFinalized != nil {
				go v.onProposalFinalized(proposal)
			}
		}
	}
}

// cleanup 清理旧数据
func (v *VotingManager) cleanup() {
	v.mu.Lock()
	defer v.mu.Unlock()

	// 清理7天前已结束的提案
	cutoff := time.Now().Add(-7 * 24 * time.Hour)
	for id, proposal := range v.proposals {
		if proposal.Status != ProposalPending && 
		   proposal.Result != nil && 
		   proposal.Result.FinalizedAt.Before(cutoff) {
			delete(v.proposals, id)
		}
	}
}

// === 持久化 ===

type votingData struct {
	Proposals map[string]*Proposal  `json:"proposals"`
	Nodes     map[string]*NodeTrust `json:"nodes"`
}

func (v *VotingManager) saveToDisk() error {
	if v.config.DataDir == "" {
		return nil
	}

	v.mu.RLock()
	data := &votingData{
		Proposals: v.proposals,
		Nodes:     v.nodes,
	}
	v.mu.RUnlock()

	jsonData, err := json.MarshalIndent(data, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to marshal: %w", err)
	}

	filePath := filepath.Join(v.config.DataDir, "voting.json")
	return os.WriteFile(filePath, jsonData, 0644)
}

func (v *VotingManager) loadFromDisk() error {
	if v.config.DataDir == "" {
		return nil
	}

	filePath := filepath.Join(v.config.DataDir, "voting.json")
	if _, err := os.Stat(filePath); os.IsNotExist(err) {
		return nil
	}

	jsonData, err := os.ReadFile(filePath)
	if err != nil {
		return fmt.Errorf("failed to read: %w", err)
	}

	var data votingData
	if err := json.Unmarshal(jsonData, &data); err != nil {
		return fmt.Errorf("failed to unmarshal: %w", err)
	}

	v.mu.Lock()
	defer v.mu.Unlock()

	if data.Proposals != nil {
		v.proposals = data.Proposals
	}
	if data.Nodes != nil {
		v.nodes = data.Nodes
	}

	return nil
}

// === 统计信息 ===

// VotingStats 投票统计
type VotingStats struct {
	TotalProposals   int `json:"total_proposals"`
	PendingProposals int `json:"pending_proposals"`
	PassedProposals  int `json:"passed_proposals"`
	RejectedProposals int `json:"rejected_proposals"`
	TotalNodes       int `json:"total_nodes"`
	ActiveNodes      int `json:"active_nodes"`
	RemovedNodes     int `json:"removed_nodes"`
}

// GetStats 获取统计信息
func (v *VotingManager) GetStats() *VotingStats {
	v.mu.RLock()
	defer v.mu.RUnlock()

	stats := &VotingStats{
		TotalProposals: len(v.proposals),
		TotalNodes:     len(v.nodes),
	}

	for _, p := range v.proposals {
		switch p.Status {
		case ProposalPending:
			stats.PendingProposals++
		case ProposalPassed:
			stats.PassedProposals++
		case ProposalRejected:
			stats.RejectedProposals++
		}
	}

	for _, n := range v.nodes {
		switch n.Status {
		case StatusActive:
			stats.ActiveNodes++
		case StatusRemoved:
			stats.RemovedNodes++
		}
	}

	return stats
}
