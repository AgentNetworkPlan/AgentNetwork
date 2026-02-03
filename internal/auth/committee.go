package auth

import (
	"crypto/rand"
	"encoding/hex"
	"errors"
	"sort"
	"sync"
	"time"

	"github.com/tjfoc/gmsm/sm3"
)

// CommitteeMember 委员会成员
type CommitteeMember struct {
	NodeID        string    `json:"node_id"`
	PublicKey     string    `json:"public_key"`
	ReputationScore float64 `json:"reputation_score"`
	JoinedAt      time.Time `json:"joined_at"`
	IsActive      bool      `json:"is_active"`
	VotingPower   float64   `json:"voting_power"`  // 投票权重
}

// VerificationCommittee 验证委员会
type VerificationCommittee struct {
	members        map[string]*CommitteeMember
	minMembers     int     // 最小委员会规模
	quorum         float64 // 法定人数比例 (0-1)
	selectionRatio float64 // 选择比例
	mu             sync.RWMutex
}

// NewVerificationCommittee 创建验证委员会
func NewVerificationCommittee(minMembers int, quorum float64) *VerificationCommittee {
	return &VerificationCommittee{
		members:        make(map[string]*CommitteeMember),
		minMembers:     minMembers,
		quorum:         quorum,
		selectionRatio: 0.3, // 默认选择 30% 的成员进行验证
	}
}

// AddMember 添加委员会成员
func (vc *VerificationCommittee) AddMember(nodeID, publicKey string, reputationScore float64) error {
	vc.mu.Lock()
	defer vc.mu.Unlock()

	if _, exists := vc.members[nodeID]; exists {
		return errors.New("成员已存在")
	}

	// 投票权重基于信誉分
	votingPower := calculateVotingPower(reputationScore)

	vc.members[nodeID] = &CommitteeMember{
		NodeID:          nodeID,
		PublicKey:       publicKey,
		ReputationScore: reputationScore,
		JoinedAt:        time.Now(),
		IsActive:        true,
		VotingPower:     votingPower,
	}

	return nil
}

// RemoveMember 移除委员会成员
func (vc *VerificationCommittee) RemoveMember(nodeID string) error {
	vc.mu.Lock()
	defer vc.mu.Unlock()

	if _, exists := vc.members[nodeID]; !exists {
		return errors.New("成员不存在")
	}

	delete(vc.members, nodeID)
	return nil
}

// UpdateMemberReputation 更新成员信誉分
func (vc *VerificationCommittee) UpdateMemberReputation(nodeID string, newScore float64) error {
	vc.mu.Lock()
	defer vc.mu.Unlock()

	member, exists := vc.members[nodeID]
	if !exists {
		return errors.New("成员不存在")
	}

	member.ReputationScore = newScore
	member.VotingPower = calculateVotingPower(newScore)
	return nil
}

// SetMemberActive 设置成员活跃状态
func (vc *VerificationCommittee) SetMemberActive(nodeID string, active bool) error {
	vc.mu.Lock()
	defer vc.mu.Unlock()

	member, exists := vc.members[nodeID]
	if !exists {
		return errors.New("成员不存在")
	}

	member.IsActive = active
	return nil
}

// GetMember 获取成员信息
func (vc *VerificationCommittee) GetMember(nodeID string) (*CommitteeMember, error) {
	vc.mu.RLock()
	defer vc.mu.RUnlock()

	member, exists := vc.members[nodeID]
	if !exists {
		return nil, errors.New("成员不存在")
	}
	return member, nil
}

// GetActiveMembers 获取所有活跃成员
func (vc *VerificationCommittee) GetActiveMembers() []*CommitteeMember {
	vc.mu.RLock()
	defer vc.mu.RUnlock()

	var active []*CommitteeMember
	for _, member := range vc.members {
		if member.IsActive {
			active = append(active, member)
		}
	}
	return active
}

// SelectVerifiers 选择验证者 (基于权益加权随机)
func (vc *VerificationCommittee) SelectVerifiers(taskID string, count int) ([]*CommitteeMember, error) {
	vc.mu.RLock()
	defer vc.mu.RUnlock()

	activeMembers := vc.GetActiveMembers()
	if len(activeMembers) < vc.minMembers {
		return nil, errors.New("活跃成员不足")
	}

	if count > len(activeMembers) {
		count = len(activeMembers)
	}

	// 权益加权随机选择
	selected := weightedRandomSelection(activeMembers, count, taskID)
	return selected, nil
}

// VerificationRequest 验证请求
type VerificationRequest struct {
	TaskID     string              `json:"task_id"`
	Proof      *ProofOfTask        `json:"proof"`
	Verifiers  []*CommitteeMember  `json:"verifiers"`
	CreatedAt  time.Time           `json:"created_at"`
	Deadline   time.Time           `json:"deadline"`
}

// VerificationSession 验证会话
type VerificationSession struct {
	Request       *VerificationRequest
	Votes         map[string]*TaskVerification // nodeID -> verification
	FinalResult   *bool
	CompletedAt   *time.Time
	mu            sync.RWMutex
}

// NewVerificationSession 创建验证会话
func NewVerificationSession(request *VerificationRequest) *VerificationSession {
	return &VerificationSession{
		Request: request,
		Votes:   make(map[string]*TaskVerification),
	}
}

// AddVote 添加投票
func (vs *VerificationSession) AddVote(verification *TaskVerification) error {
	vs.mu.Lock()
	defer vs.mu.Unlock()

	if vs.FinalResult != nil {
		return errors.New("验证已完成")
	}

	// 检查是否是指定的验证者
	isVerifier := false
	for _, v := range vs.Request.Verifiers {
		if v.NodeID == verification.VerifierID {
			isVerifier = true
			break
		}
	}
	if !isVerifier {
		return errors.New("不是指定的验证者")
	}

	// 检查是否已投票
	if _, exists := vs.Votes[verification.VerifierID]; exists {
		return errors.New("已经投票")
	}

	vs.Votes[verification.VerifierID] = verification
	return nil
}

// CheckConsensus 检查是否达成共识
func (vs *VerificationSession) CheckConsensus(quorum float64) (*bool, error) {
	vs.mu.Lock()
	defer vs.mu.Unlock()

	if vs.FinalResult != nil {
		return vs.FinalResult, nil
	}

	totalVerifiers := len(vs.Request.Verifiers)
	votesNeeded := int(float64(totalVerifiers) * quorum)

	if len(vs.Votes) < votesNeeded {
		return nil, nil // 还未达到法定人数
	}

	// 计算加权投票
	var validWeight, invalidWeight float64
	for verifierID, vote := range vs.Votes {
		// 找到验证者的权重
		var weight float64 = 1.0
		for _, v := range vs.Request.Verifiers {
			if v.NodeID == verifierID {
				weight = v.VotingPower
				break
			}
		}

		if vote.IsValid {
			validWeight += weight
		} else {
			invalidWeight += weight
		}
	}

	// 判断结果
	totalWeight := validWeight + invalidWeight
	if totalWeight > 0 {
		validRatio := validWeight / totalWeight
		result := validRatio > 0.5
		vs.FinalResult = &result
		now := time.Now()
		vs.CompletedAt = &now
		return vs.FinalResult, nil
	}

	return nil, nil
}

// GetResult 获取验证结果
func (vs *VerificationSession) GetResult() *bool {
	vs.mu.RLock()
	defer vs.mu.RUnlock()
	return vs.FinalResult
}

// ========== 委员会管理器 ==========

// CommitteeManager 委员会管理器
type CommitteeManager struct {
	committee *VerificationCommittee
	sessions  map[string]*VerificationSession // taskID -> session
	mu        sync.RWMutex
}

// NewCommitteeManager 创建委员会管理器
func NewCommitteeManager(minMembers int, quorum float64) *CommitteeManager {
	return &CommitteeManager{
		committee: NewVerificationCommittee(minMembers, quorum),
		sessions:  make(map[string]*VerificationSession),
	}
}

// GetCommittee 获取委员会
func (cm *CommitteeManager) GetCommittee() *VerificationCommittee {
	return cm.committee
}

// InitiateVerification 发起验证
func (cm *CommitteeManager) InitiateVerification(proof *ProofOfTask, verifierCount int, deadline time.Duration) (*VerificationRequest, error) {
	cm.mu.Lock()
	defer cm.mu.Unlock()

	// 选择验证者
	verifiers, err := cm.committee.SelectVerifiers(proof.TaskID, verifierCount)
	if err != nil {
		return nil, err
	}

	request := &VerificationRequest{
		TaskID:    proof.TaskID,
		Proof:     proof,
		Verifiers: verifiers,
		CreatedAt: time.Now(),
		Deadline:  time.Now().Add(deadline),
	}

	// 创建会话
	session := NewVerificationSession(request)
	cm.sessions[proof.TaskID] = session

	return request, nil
}

// SubmitVerification 提交验证结果
func (cm *CommitteeManager) SubmitVerification(verification *TaskVerification) error {
	cm.mu.Lock()
	defer cm.mu.Unlock()

	session, exists := cm.sessions[verification.TaskID]
	if !exists {
		return errors.New("验证会话不存在")
	}

	if err := session.AddVote(verification); err != nil {
		return err
	}

	// 检查共识
	session.CheckConsensus(cm.committee.quorum)
	return nil
}

// GetVerificationResult 获取验证结果
func (cm *CommitteeManager) GetVerificationResult(taskID string) (*bool, error) {
	cm.mu.RLock()
	defer cm.mu.RUnlock()

	session, exists := cm.sessions[taskID]
	if !exists {
		return nil, errors.New("验证会话不存在")
	}

	return session.GetResult(), nil
}

// GetSession 获取验证会话
func (cm *CommitteeManager) GetSession(taskID string) (*VerificationSession, error) {
	cm.mu.RLock()
	defer cm.mu.RUnlock()

	session, exists := cm.sessions[taskID]
	if !exists {
		return nil, errors.New("验证会话不存在")
	}
	return session, nil
}

// ========== 辅助函数 ==========

// calculateVotingPower 计算投票权重
func calculateVotingPower(reputationScore float64) float64 {
	// 基于信誉分的非线性权重
	// 信誉分 0-1，权重 0.5-2.0
	if reputationScore < 0 {
		return 0.5
	}
	if reputationScore > 1 {
		reputationScore = 1
	}
	return 0.5 + reputationScore*1.5
}

// weightedRandomSelection 权益加权随机选择
func weightedRandomSelection(members []*CommitteeMember, count int, seed string) []*CommitteeMember {
	if len(members) <= count {
		return members
	}

	// 计算总权重
	var totalWeight float64
	for _, m := range members {
		totalWeight += m.VotingPower
	}

	// 使用任务 ID 作为随机种子
	seedHash := sm3.Sm3Sum([]byte(seed))
	
	// 按权重排序
	type weightedMember struct {
		member *CommitteeMember
		score  float64
	}

	weighted := make([]weightedMember, len(members))
	for i, m := range members {
		// 为每个成员计算一个分数 = 权重 * 随机因子
		memberHash := sm3.Sm3Sum(append(seedHash[:], []byte(m.NodeID)...))
		randomFactor := float64(memberHash[0]) / 255.0
		weighted[i] = weightedMember{
			member: m,
			score:  m.VotingPower * randomFactor,
		}
	}

	// 按分数排序
	sort.Slice(weighted, func(i, j int) bool {
		return weighted[i].score > weighted[j].score
	})

	// 选择前 count 个
	selected := make([]*CommitteeMember, count)
	for i := 0; i < count; i++ {
		selected[i] = weighted[i].member
	}

	return selected
}

// generateID 生成唯一 ID
func generateID() string {
	b := make([]byte, 16)
	rand.Read(b)
	return hex.EncodeToString(b)
}

// computeHash 计算数据哈希
func computeHash(data []byte) string {
	hash := sm3.Sm3Sum(data)
	return hex.EncodeToString(hash[:])
}
