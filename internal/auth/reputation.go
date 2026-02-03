package auth

import (
	"encoding/json"
	"errors"
	"math"
	"sync"
	"time"
)

// ReputationConstants 信誉系统常量
const (
	InitialReputation   = 0.5  // 初始信誉分
	MaxReputation       = 1.0  // 最大信誉分
	MinReputation       = -1.0 // 最小信誉分
	DecayFactor         = 0.95 // 衰减因子 (每天)
	TaskCompletionBonus = 0.05 // 完成任务奖励
	TaskFailurePenalty  = 0.1  // 任务失败惩罚
	VerificationBonus   = 0.02 // 验证正确奖励
	VerificationPenalty = 0.05 // 验证错误惩罚
	SybilPenalty        = 0.5  // Sybil 攻击惩罚
)

// ReputationEvent 信誉事件类型
type ReputationEvent string

const (
	EventTaskCompleted     ReputationEvent = "task_completed"
	EventTaskFailed        ReputationEvent = "task_failed"
	EventTaskExpired       ReputationEvent = "task_expired"
	EventVerificationCorrect ReputationEvent = "verification_correct"
	EventVerificationWrong  ReputationEvent = "verification_wrong"
	EventSybilDetected     ReputationEvent = "sybil_detected"
	EventNodeJoined        ReputationEvent = "node_joined"
	EventDailyDecay        ReputationEvent = "daily_decay"
)

// ReputationRecord 信誉记录
type ReputationRecord struct {
	NodeID      string          `json:"node_id"`
	Event       ReputationEvent `json:"event"`
	Delta       float64         `json:"delta"`       // 变化量
	OldScore    float64         `json:"old_score"`
	NewScore    float64         `json:"new_score"`
	RelatedTask string          `json:"related_task,omitempty"`
	Timestamp   time.Time       `json:"timestamp"`
	Signature   []byte          `json:"signature,omitempty"` // 记录签名
}

// NodeReputation 节点信誉
type NodeReputation struct {
	NodeID            string    `json:"node_id"`
	Score             float64   `json:"score"`
	TotalTasksCompleted int     `json:"total_tasks_completed"`
	TotalTasksFailed   int      `json:"total_tasks_failed"`
	TotalVerifications int      `json:"total_verifications"`
	CorrectVerifications int    `json:"correct_verifications"`
	LastActivityAt    time.Time `json:"last_activity_at"`
	JoinedAt          time.Time `json:"joined_at"`
	IsBanned          bool      `json:"is_banned"`
	BanReason         string    `json:"ban_reason,omitempty"`
}

// ReputationSystem 信誉系统
type ReputationSystem struct {
	nodes   map[string]*NodeReputation
	records []ReputationRecord
	mu      sync.RWMutex
}

// NewReputationSystem 创建信誉系统
func NewReputationSystem() *ReputationSystem {
	return &ReputationSystem{
		nodes:   make(map[string]*NodeReputation),
		records: make([]ReputationRecord, 0),
	}
}

// RegisterNode 注册新节点
func (rs *ReputationSystem) RegisterNode(nodeID string) error {
	rs.mu.Lock()
	defer rs.mu.Unlock()

	if _, exists := rs.nodes[nodeID]; exists {
		return errors.New("节点已注册")
	}

	now := time.Now()
	rs.nodes[nodeID] = &NodeReputation{
		NodeID:         nodeID,
		Score:          InitialReputation,
		LastActivityAt: now,
		JoinedAt:       now,
	}

	rs.addRecord(nodeID, EventNodeJoined, 0, 0, InitialReputation, "")
	return nil
}

// GetReputation 获取节点信誉
func (rs *ReputationSystem) GetReputation(nodeID string) (*NodeReputation, error) {
	rs.mu.RLock()
	defer rs.mu.RUnlock()

	rep, exists := rs.nodes[nodeID]
	if !exists {
		return nil, errors.New("节点未注册")
	}
	return rep, nil
}

// GetScore 获取信誉分
func (rs *ReputationSystem) GetScore(nodeID string) (float64, error) {
	rs.mu.RLock()
	defer rs.mu.RUnlock()

	rep, exists := rs.nodes[nodeID]
	if !exists {
		return 0, errors.New("节点未注册")
	}
	return rep.Score, nil
}

// OnTaskCompleted 任务完成事件
func (rs *ReputationSystem) OnTaskCompleted(workerID, taskID string, difficulty int) error {
	rs.mu.Lock()
	defer rs.mu.Unlock()

	rep, exists := rs.nodes[workerID]
	if !exists {
		return errors.New("节点未注册")
	}

	if rep.IsBanned {
		return errors.New("节点已被封禁")
	}

	// 根据难度计算奖励
	bonus := TaskCompletionBonus * float64(difficulty) / 5.0
	oldScore := rep.Score
	rep.Score = clampReputation(rep.Score + bonus)
	rep.TotalTasksCompleted++
	rep.LastActivityAt = time.Now()

	rs.addRecord(workerID, EventTaskCompleted, bonus, oldScore, rep.Score, taskID)
	return nil
}

// OnTaskFailed 任务失败事件
func (rs *ReputationSystem) OnTaskFailed(workerID, taskID string) error {
	rs.mu.Lock()
	defer rs.mu.Unlock()

	rep, exists := rs.nodes[workerID]
	if !exists {
		return errors.New("节点未注册")
	}

	oldScore := rep.Score
	rep.Score = clampReputation(rep.Score - TaskFailurePenalty)
	rep.TotalTasksFailed++
	rep.LastActivityAt = time.Now()

	rs.addRecord(workerID, EventTaskFailed, -TaskFailurePenalty, oldScore, rep.Score, taskID)
	return nil
}

// OnTaskExpired 任务过期事件
func (rs *ReputationSystem) OnTaskExpired(workerID, taskID string) error {
	rs.mu.Lock()
	defer rs.mu.Unlock()

	rep, exists := rs.nodes[workerID]
	if !exists {
		return errors.New("节点未注册")
	}

	penalty := TaskFailurePenalty * 0.5 // 过期惩罚较轻
	oldScore := rep.Score
	rep.Score = clampReputation(rep.Score - penalty)
	rep.LastActivityAt = time.Now()

	rs.addRecord(workerID, EventTaskExpired, -penalty, oldScore, rep.Score, taskID)
	return nil
}

// OnVerificationResult 验证结果事件
func (rs *ReputationSystem) OnVerificationResult(verifierID, taskID string, wasCorrect bool) error {
	rs.mu.Lock()
	defer rs.mu.Unlock()

	rep, exists := rs.nodes[verifierID]
	if !exists {
		return errors.New("节点未注册")
	}

	rep.TotalVerifications++
	rep.LastActivityAt = time.Now()

	var delta float64
	var event ReputationEvent
	oldScore := rep.Score

	if wasCorrect {
		delta = VerificationBonus
		event = EventVerificationCorrect
		rep.CorrectVerifications++
	} else {
		delta = -VerificationPenalty
		event = EventVerificationWrong
	}

	rep.Score = clampReputation(rep.Score + delta)
	rs.addRecord(verifierID, event, delta, oldScore, rep.Score, taskID)
	return nil
}

// OnSybilDetected Sybil 攻击检测事件
func (rs *ReputationSystem) OnSybilDetected(nodeID string, reason string) error {
	rs.mu.Lock()
	defer rs.mu.Unlock()

	rep, exists := rs.nodes[nodeID]
	if !exists {
		return errors.New("节点未注册")
	}

	oldScore := rep.Score
	rep.Score = clampReputation(rep.Score - SybilPenalty)

	// 如果信誉过低，封禁节点
	if rep.Score < -0.5 {
		rep.IsBanned = true
		rep.BanReason = reason
	}

	rs.addRecord(nodeID, EventSybilDetected, -SybilPenalty, oldScore, rep.Score, "")
	return nil
}

// ApplyDailyDecay 应用每日衰减
func (rs *ReputationSystem) ApplyDailyDecay() {
	rs.mu.Lock()
	defer rs.mu.Unlock()

	for nodeID, rep := range rs.nodes {
		if rep.IsBanned {
			continue
		}

		// 只对正信誉衰减
		if rep.Score > 0 {
			oldScore := rep.Score
			// 向基准值 0.5 衰减
			rep.Score = 0.5 + (rep.Score-0.5)*DecayFactor
			delta := rep.Score - oldScore
			rs.addRecord(nodeID, EventDailyDecay, delta, oldScore, rep.Score, "")
		}
	}
}

// GetTopNodes 获取信誉最高的节点
func (rs *ReputationSystem) GetTopNodes(count int) []*NodeReputation {
	rs.mu.RLock()
	defer rs.mu.RUnlock()

	nodes := make([]*NodeReputation, 0, len(rs.nodes))
	for _, rep := range rs.nodes {
		if !rep.IsBanned {
			nodes = append(nodes, rep)
		}
	}

	// 排序
	for i := 0; i < len(nodes)-1; i++ {
		for j := i + 1; j < len(nodes); j++ {
			if nodes[j].Score > nodes[i].Score {
				nodes[i], nodes[j] = nodes[j], nodes[i]
			}
		}
	}

	if count > len(nodes) {
		count = len(nodes)
	}

	return nodes[:count]
}

// GetQualifiedVerifiers 获取合格的验证者 (信誉 > 阈值)
func (rs *ReputationSystem) GetQualifiedVerifiers(minScore float64) []*NodeReputation {
	rs.mu.RLock()
	defer rs.mu.RUnlock()

	var qualified []*NodeReputation
	for _, rep := range rs.nodes {
		if !rep.IsBanned && rep.Score >= minScore {
			qualified = append(qualified, rep)
		}
	}
	return qualified
}

// GetNodeRecords 获取节点的信誉记录
func (rs *ReputationSystem) GetNodeRecords(nodeID string) []ReputationRecord {
	rs.mu.RLock()
	defer rs.mu.RUnlock()

	var records []ReputationRecord
	for _, r := range rs.records {
		if r.NodeID == nodeID {
			records = append(records, r)
		}
	}
	return records
}

// GetAllRecords 获取所有记录
func (rs *ReputationSystem) GetAllRecords() []ReputationRecord {
	rs.mu.RLock()
	defer rs.mu.RUnlock()
	return append([]ReputationRecord{}, rs.records...)
}

// CalculateTrustScore 计算综合信任分数
func (rs *ReputationSystem) CalculateTrustScore(nodeID string) (float64, error) {
	rs.mu.RLock()
	defer rs.mu.RUnlock()

	rep, exists := rs.nodes[nodeID]
	if !exists {
		return 0, errors.New("节点未注册")
	}

	if rep.IsBanned {
		return 0, nil
	}

	// 综合考虑多个因素
	baseScore := rep.Score

	// 任务完成率
	var taskSuccessRate float64 = 0.5
	totalTasks := rep.TotalTasksCompleted + rep.TotalTasksFailed
	if totalTasks > 0 {
		taskSuccessRate = float64(rep.TotalTasksCompleted) / float64(totalTasks)
	}

	// 验证准确率
	var verificationAccuracy float64 = 0.5
	if rep.TotalVerifications > 0 {
		verificationAccuracy = float64(rep.CorrectVerifications) / float64(rep.TotalVerifications)
	}

	// 活跃度 (基于最后活动时间)
	activityScore := 1.0
	daysSinceActivity := time.Since(rep.LastActivityAt).Hours() / 24
	if daysSinceActivity > 7 {
		activityScore = math.Max(0.5, 1.0-daysSinceActivity/30)
	}

	// 综合评分
	trustScore := baseScore*0.4 + taskSuccessRate*0.3 + verificationAccuracy*0.2 + activityScore*0.1

	return clampReputation(trustScore), nil
}

// ExportState 导出系统状态
func (rs *ReputationSystem) ExportState() ([]byte, error) {
	rs.mu.RLock()
	defer rs.mu.RUnlock()

	state := struct {
		Nodes   map[string]*NodeReputation `json:"nodes"`
		Records []ReputationRecord         `json:"records"`
	}{
		Nodes:   rs.nodes,
		Records: rs.records,
	}

	return json.Marshal(state)
}

// ImportState 导入系统状态
func (rs *ReputationSystem) ImportState(data []byte) error {
	rs.mu.Lock()
	defer rs.mu.Unlock()

	var state struct {
		Nodes   map[string]*NodeReputation `json:"nodes"`
		Records []ReputationRecord         `json:"records"`
	}

	if err := json.Unmarshal(data, &state); err != nil {
		return err
	}

	rs.nodes = state.Nodes
	rs.records = state.Records
	return nil
}

// addRecord 添加记录 (内部方法，需要持有锁)
func (rs *ReputationSystem) addRecord(nodeID string, event ReputationEvent, delta, oldScore, newScore float64, taskID string) {
	record := ReputationRecord{
		NodeID:      nodeID,
		Event:       event,
		Delta:       delta,
		OldScore:    oldScore,
		NewScore:    newScore,
		RelatedTask: taskID,
		Timestamp:   time.Now(),
	}
	rs.records = append(rs.records, record)
}

// clampReputation 限制信誉分在有效范围内
func clampReputation(score float64) float64 {
	if score > MaxReputation {
		return MaxReputation
	}
	if score < MinReputation {
		return MinReputation
	}
	return score
}
