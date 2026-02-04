// Package incentive 实现节点声誉激励与传播机制
// 包括任务奖励、声誉传播、耐受值控制等功能
package incentive

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"time"
)

// 错误定义
var (
	ErrNilConfig           = errors.New("config cannot be nil")
	ErrEmptyNodeID         = errors.New("node ID cannot be empty")
	ErrEmptyTaskID         = errors.New("task ID cannot be empty")
	ErrInvalidScore        = errors.New("score must be positive")
	ErrToleranceExceeded   = errors.New("tolerance exceeded for this source")
	ErrSelfPropagation     = errors.New("cannot propagate to self")
	ErrInvalidDecayFactor  = errors.New("decay factor must be between 0 and 1")
	ErrRewardNotFound      = errors.New("reward not found")
	ErrPropagationNotFound = errors.New("propagation not found")
	ErrDuplicateReward     = errors.New("duplicate reward for task")
)

// TaskType 任务类型
type TaskType string

const (
	TaskTypeGeneral    TaskType = "general"    // 一般任务
	TaskTypeRelay      TaskType = "relay"      // 中继任务
	TaskTypeAudit      TaskType = "audit"      // 审计任务
	TaskTypeVoting     TaskType = "voting"     // 投票任务
	TaskTypeStorage    TaskType = "storage"    // 存储任务
	TaskTypeCompute    TaskType = "compute"    // 计算任务
	TaskTypeValidation TaskType = "validation" // 验证任务
)

// ReputationSource 声誉来源类型
type ReputationSource string

const (
	// 有效的声誉来源（可验证）
	SourceTaskCompletion  ReputationSource = "task_completion"   // 任务完成（主要来源）
	SourceRelayService    ReputationSource = "relay_service"     // 中继服务
	SourceStorageService  ReputationSource = "storage_service"   // 存储服务
	SourceAuditPass       ReputationSource = "audit_pass"        // 审计通过
	SourceVotingParticipation ReputationSource = "voting"        // 投票参与
	
	// 禁用的声誉来源
	SourcePeerRating      ReputationSource = "peer_rating"       // 节点互评（已禁用）
	SourceDirectTransfer  ReputationSource = "direct_transfer"   // 直接转移（已禁用）
)

// ValidReputationSources 有效的声誉来源列表
var ValidReputationSources = map[ReputationSource]bool{
	SourceTaskCompletion:      true,
	SourceRelayService:        true,
	SourceStorageService:      true,
	SourceAuditPass:           true,
	SourceVotingParticipation: true,
}

// IsValidReputationSource 检查声誉来源是否有效
func IsValidReputationSource(source ReputationSource) bool {
	return ValidReputationSources[source]
}

// ErrInvalidReputationSource 无效的声誉来源错误
var ErrInvalidReputationSource = errors.New("invalid or disabled reputation source")

// RewardStatus 奖励状态
type RewardStatus string

const (
	RewardStatusPending   RewardStatus = "pending"   // 待确认
	RewardStatusConfirmed RewardStatus = "confirmed" // 已确认
	RewardStatusPropagated RewardStatus = "propagated" // 已传播
	RewardStatusExpired   RewardStatus = "expired"   // 已过期
)

// TaskReward 任务奖励记录
type TaskReward struct {
	RewardID     string           `json:"reward_id"`     // 奖励唯一ID
	NodeID       string           `json:"node_id"`       // 节点ID
	TaskID       string           `json:"task_id"`       // 任务ID
	TaskType     TaskType         `json:"task_type"`     // 任务类型
	Source       ReputationSource `json:"source"`        // 声誉来源
	BaseScore    float64          `json:"base_score"`    // 基础分
	TaskWeight   float64          `json:"task_weight"`   // 任务权重
	FinalScore   float64          `json:"final_score"`   // 最终得分
	Timestamp    time.Time        `json:"timestamp"`     // 时间戳
	Status       RewardStatus     `json:"status"`        // 状态
	Description  string           `json:"description"`   // 描述
	PropagatedTo []string         `json:"propagated_to"` // 已传播到的节点
}

// PropagationRecord 声誉传播记录
type PropagationRecord struct {
	PropagationID   string    `json:"propagation_id"`   // 传播ID
	SourceNodeID    string    `json:"source_node_id"`   // 来源节点
	TargetNodeID    string    `json:"target_node_id"`   // 目标节点
	OriginalScore   float64   `json:"original_score"`   // 原始分数
	DecayFactor     float64   `json:"decay_factor"`     // 衰减因子
	PropagatedScore float64   `json:"propagated_score"` // 传播后分数
	Depth           int       `json:"depth"`            // 传播深度
	Timestamp       time.Time `json:"timestamp"`        // 时间戳
	OriginRewardID  string    `json:"origin_reward_id"` // 原始奖励ID
}

// ToleranceRecord 耐受值记录
type ToleranceRecord struct {
	SourceNodeID      string    `json:"source_node_id"`      // 来源节点
	TargetNodeID      string    `json:"target_node_id"`      // 目标节点（本节点）
	TotalReceived     float64   `json:"total_received"`      // 累计接收声誉
	MaxTolerance      float64   `json:"max_tolerance"`       // 最大耐受值
	RemainingTolerance float64  `json:"remaining_tolerance"` // 剩余耐受值
	LastResetTime     time.Time `json:"last_reset_time"`     // 上次重置时间
	NextResetTime     time.Time `json:"next_reset_time"`     // 下次重置时间
}

// TaskWeightConfig 任务权重配置
type TaskWeightConfig struct {
	TaskType   TaskType `json:"task_type"`
	Weight     float64  `json:"weight"`
	MinScore   float64  `json:"min_score"`
	MaxScore   float64  `json:"max_score"`
}

// IncentiveConfig 激励系统配置
type IncentiveConfig struct {
	NodeID            string                       // 本节点ID
	DataDir           string                       // 数据目录
	DefaultDecayFactor float64                     // 默认衰减因子
	DefaultTolerance   float64                     // 默认耐受值
	ToleranceResetPeriod time.Duration             // 耐受值重置周期
	MinPropagationScore  float64                   // 最小传播分数
	MaxPropagationDepth  int                       // 最大传播深度
	TaskWeights         map[TaskType]*TaskWeightConfig // 任务权重配置
	
	// 获取邻居函数
	GetNeighborsFunc func(nodeID string) []string
	
	// 更新声誉函数
	UpdateReputationFunc func(nodeID string, delta float64) error
	
	// 获取当前声誉函数
	GetReputationFunc func(nodeID string) float64
}

// DefaultIncentiveConfig 返回默认配置
func DefaultIncentiveConfig(nodeID string) *IncentiveConfig {
	return &IncentiveConfig{
		NodeID:              nodeID,
		DataDir:             "./data/incentive",
		DefaultDecayFactor:  0.7,
		DefaultTolerance:    50.0,
		ToleranceResetPeriod: 24 * time.Hour,
		MinPropagationScore: 0.1,
		MaxPropagationDepth: 5,
		TaskWeights: map[TaskType]*TaskWeightConfig{
			TaskTypeGeneral:    {TaskType: TaskTypeGeneral, Weight: 1.0, MinScore: 1, MaxScore: 10},
			TaskTypeRelay:      {TaskType: TaskTypeRelay, Weight: 1.2, MinScore: 1, MaxScore: 15},
			TaskTypeAudit:      {TaskType: TaskTypeAudit, Weight: 1.5, MinScore: 5, MaxScore: 20},
			TaskTypeVoting:     {TaskType: TaskTypeVoting, Weight: 1.1, MinScore: 1, MaxScore: 10},
			TaskTypeStorage:    {TaskType: TaskTypeStorage, Weight: 1.3, MinScore: 2, MaxScore: 15},
			TaskTypeCompute:    {TaskType: TaskTypeCompute, Weight: 1.4, MinScore: 3, MaxScore: 20},
			TaskTypeValidation: {TaskType: TaskTypeValidation, Weight: 1.2, MinScore: 2, MaxScore: 12},
		},
	}
}

// IncentiveManager 激励系统管理器
type IncentiveManager struct {
	mu           sync.RWMutex
	config       *IncentiveConfig
	rewards      map[string]*TaskReward                    // RewardID -> TaskReward
	taskRewards  map[string]string                         // TaskID -> RewardID (防止重复)
	propagations map[string]*PropagationRecord             // PropagationID -> Record
	tolerances   map[string]map[string]*ToleranceRecord    // TargetNodeID -> SourceNodeID -> Record
	running      bool
	stopCh       chan struct{}
	
	// 回调
	OnRewardCreated    func(*TaskReward)
	OnRewardPropagated func(*TaskReward, []string)
	OnToleranceExceeded func(sourceNodeID, targetNodeID string, score float64)
	OnToleranceReset   func(targetNodeID string)
}

// NewIncentiveManager 创建激励管理器
func NewIncentiveManager(config *IncentiveConfig) (*IncentiveManager, error) {
	if config == nil {
		return nil, ErrNilConfig
	}
	if config.NodeID == "" {
		return nil, ErrEmptyNodeID
	}
	
	// 创建数据目录
	if config.DataDir != "" {
		if err := os.MkdirAll(config.DataDir, 0755); err != nil {
			return nil, fmt.Errorf("failed to create data directory: %w", err)
		}
	}
	
	im := &IncentiveManager{
		config:       config,
		rewards:      make(map[string]*TaskReward),
		taskRewards:  make(map[string]string),
		propagations: make(map[string]*PropagationRecord),
		tolerances:   make(map[string]map[string]*ToleranceRecord),
		stopCh:       make(chan struct{}),
	}
	
	// 初始化本节点的耐受值表
	im.tolerances[config.NodeID] = make(map[string]*ToleranceRecord)
	
	// 加载持久化数据
	if err := im.load(); err != nil {
		// 忽略加载错误
	}
	
	return im, nil
}

// Start 启动激励系统
func (im *IncentiveManager) Start() {
	im.mu.Lock()
	if im.running {
		im.mu.Unlock()
		return
	}
	im.running = true
	im.stopCh = make(chan struct{})
	im.mu.Unlock()
	
	go im.toleranceResetLoop()
}

// Stop 停止激励系统
func (im *IncentiveManager) Stop() {
	im.mu.Lock()
	if !im.running {
		im.mu.Unlock()
		return
	}
	im.running = false
	close(im.stopCh)
	im.mu.Unlock()
	
	im.save()
}

// toleranceResetLoop 耐受值重置循环
func (im *IncentiveManager) toleranceResetLoop() {
	ticker := time.NewTicker(time.Hour) // 每小时检查
	defer ticker.Stop()
	
	for {
		select {
		case <-ticker.C:
			im.checkAndResetTolerances()
		case <-im.stopCh:
			return
		}
	}
}

// checkAndResetTolerances 检查并重置过期的耐受值
func (im *IncentiveManager) checkAndResetTolerances() {
	im.mu.Lock()
	defer im.mu.Unlock()
	
	now := time.Now()
	
	for targetID, sourceMap := range im.tolerances {
		for sourceID, record := range sourceMap {
			if now.After(record.NextResetTime) {
				// 重置耐受值
				record.TotalReceived = 0
				record.RemainingTolerance = record.MaxTolerance
				record.LastResetTime = now
				record.NextResetTime = now.Add(im.config.ToleranceResetPeriod)
				
				// 触发回调
				if im.OnToleranceReset != nil && targetID == im.config.NodeID {
					go im.OnToleranceReset(sourceID)
				}
			}
		}
	}
}

// AwardTaskCompletion 奖励任务完成
func (im *IncentiveManager) AwardTaskCompletion(nodeID, taskID string, taskType TaskType, baseScore float64, description string) (*TaskReward, error) {
	return im.AwardTaskCompletionWithSource(nodeID, taskID, taskType, SourceTaskCompletion, baseScore, description)
}

// AwardTaskCompletionWithSource 带声誉来源的任务奖励
func (im *IncentiveManager) AwardTaskCompletionWithSource(nodeID, taskID string, taskType TaskType, source ReputationSource, baseScore float64, description string) (*TaskReward, error) {
	if nodeID == "" {
		return nil, ErrEmptyNodeID
	}
	if taskID == "" {
		return nil, ErrEmptyTaskID
	}
	if baseScore <= 0 {
		return nil, ErrInvalidScore
	}
	
	// 验证声誉来源是否有效
	if !IsValidReputationSource(source) {
		return nil, ErrInvalidReputationSource
	}
	
	im.mu.Lock()
	
	// 检查是否已奖励过此任务
	if _, exists := im.taskRewards[taskID]; exists {
		im.mu.Unlock()
		return nil, ErrDuplicateReward
	}
	
	// 获取任务权重
	weight := 1.0
	if wc, ok := im.config.TaskWeights[taskType]; ok {
		weight = wc.Weight
		// 限制分数范围
		if baseScore < wc.MinScore {
			baseScore = wc.MinScore
		}
		if baseScore > wc.MaxScore {
			baseScore = wc.MaxScore
		}
	}
	
	now := time.Now()
	
	// 生成奖励ID
	idData := fmt.Sprintf("%s%s%d", nodeID, taskID, now.UnixNano())
	hash := sha256.Sum256([]byte(idData))
	rewardID := hex.EncodeToString(hash[:16])
	
	finalScore := baseScore * weight
	
	reward := &TaskReward{
		RewardID:     rewardID,
		NodeID:       nodeID,
		TaskID:       taskID,
		TaskType:     taskType,
		Source:       source,
		BaseScore:    baseScore,
		TaskWeight:   weight,
		FinalScore:   finalScore,
		Timestamp:    now,
		Status:       RewardStatusPending,
		Description:  description,
		PropagatedTo: make([]string, 0),
	}
	
	im.rewards[rewardID] = reward
	im.taskRewards[taskID] = rewardID
	
	im.mu.Unlock()
	
	// 更新节点声誉
	if im.config.UpdateReputationFunc != nil {
		if err := im.config.UpdateReputationFunc(nodeID, finalScore); err == nil {
			im.mu.Lock()
			reward.Status = RewardStatusConfirmed
			im.mu.Unlock()
		}
	} else {
		im.mu.Lock()
		reward.Status = RewardStatusConfirmed
		im.mu.Unlock()
	}
	
	// 保存
	im.save()
	
	// 触发回调
	if im.OnRewardCreated != nil {
		im.OnRewardCreated(reward)
	}
	
	return reward, nil
}

// PropagateReputation 传播声誉到邻居节点
func (im *IncentiveManager) PropagateReputation(rewardID string) ([]string, error) {
	im.mu.Lock()
	
	reward, ok := im.rewards[rewardID]
	if !ok {
		im.mu.Unlock()
		return nil, ErrRewardNotFound
	}
	
	if reward.Status != RewardStatusConfirmed {
		im.mu.Unlock()
		return nil, errors.New("reward not confirmed yet")
	}
	
	im.mu.Unlock()
	
	// 获取邻居节点
	var neighbors []string
	if im.config.GetNeighborsFunc != nil {
		neighbors = im.config.GetNeighborsFunc(reward.NodeID)
	}
	
	if len(neighbors) == 0 {
		return []string{}, nil
	}
	
	propagatedTo := make([]string, 0)
	
	for _, neighborID := range neighbors {
		if neighborID == reward.NodeID {
			continue
		}
		
		err := im.propagateToNode(reward.NodeID, neighborID, reward.FinalScore, 1, rewardID)
		if err == nil {
			propagatedTo = append(propagatedTo, neighborID)
		}
	}
	
	im.mu.Lock()
	reward.PropagatedTo = propagatedTo
	reward.Status = RewardStatusPropagated
	im.mu.Unlock()
	
	// 保存
	im.save()
	
	// 触发回调
	if im.OnRewardPropagated != nil && len(propagatedTo) > 0 {
		im.OnRewardPropagated(reward, propagatedTo)
	}
	
	return propagatedTo, nil
}

// propagateToNode 传播声誉到单个节点
func (im *IncentiveManager) propagateToNode(sourceNodeID, targetNodeID string, score float64, depth int, originRewardID string) error {
	if targetNodeID == sourceNodeID {
		return ErrSelfPropagation
	}
	
	// 检查传播深度
	if depth > im.config.MaxPropagationDepth {
		return errors.New("max propagation depth exceeded")
	}
	
	// 计算衰减后的分数
	propagatedScore := score * im.config.DefaultDecayFactor
	
	// 检查最小传播分数
	if propagatedScore < im.config.MinPropagationScore {
		return errors.New("propagated score too small")
	}
	
	im.mu.Lock()
	
	// 检查耐受值
	if tolerances, ok := im.tolerances[targetNodeID]; ok {
		if record, ok := tolerances[sourceNodeID]; ok {
			if record.RemainingTolerance < propagatedScore {
				im.mu.Unlock()
				
				// 触发回调
				if im.OnToleranceExceeded != nil {
					im.OnToleranceExceeded(sourceNodeID, targetNodeID, propagatedScore)
				}
				
				return ErrToleranceExceeded
			}
			// 更新耐受值
			record.TotalReceived += propagatedScore
			record.RemainingTolerance -= propagatedScore
		} else {
			// 创建新的耐受值记录
			now := time.Now()
			tolerances[sourceNodeID] = &ToleranceRecord{
				SourceNodeID:       sourceNodeID,
				TargetNodeID:       targetNodeID,
				TotalReceived:      propagatedScore,
				MaxTolerance:       im.config.DefaultTolerance,
				RemainingTolerance: im.config.DefaultTolerance - propagatedScore,
				LastResetTime:      now,
				NextResetTime:      now.Add(im.config.ToleranceResetPeriod),
			}
		}
	}
	
	// 记录传播
	now := time.Now()
	propID := fmt.Sprintf("%s-%s-%d", sourceNodeID, targetNodeID, now.UnixNano())
	hash := sha256.Sum256([]byte(propID))
	propagationID := hex.EncodeToString(hash[:16])
	
	record := &PropagationRecord{
		PropagationID:   propagationID,
		SourceNodeID:    sourceNodeID,
		TargetNodeID:    targetNodeID,
		OriginalScore:   score,
		DecayFactor:     im.config.DefaultDecayFactor,
		PropagatedScore: propagatedScore,
		Depth:           depth,
		Timestamp:       now,
		OriginRewardID:  originRewardID,
	}
	
	im.propagations[propagationID] = record
	
	im.mu.Unlock()
	
	// 更新目标节点声誉
	if im.config.UpdateReputationFunc != nil {
		im.config.UpdateReputationFunc(targetNodeID, propagatedScore)
	}
	
	return nil
}

// ReceivePropagation 接收传播的声誉（从其他节点）
func (im *IncentiveManager) ReceivePropagation(sourceNodeID string, score float64, depth int, originRewardID string) error {
	if sourceNodeID == "" {
		return ErrEmptyNodeID
	}
	if score <= 0 {
		return ErrInvalidScore
	}
	
	return im.propagateToNode(sourceNodeID, im.config.NodeID, score, depth, originRewardID)
}

// ContinuePropagation 继续传播到自己的邻居
func (im *IncentiveManager) ContinuePropagation(sourceNodeID string, score float64, depth int, originRewardID string) ([]string, error) {
	// 先接收
	err := im.ReceivePropagation(sourceNodeID, score, depth, originRewardID)
	if err != nil {
		return nil, err
	}
	
	// 获取邻居
	var neighbors []string
	if im.config.GetNeighborsFunc != nil {
		neighbors = im.config.GetNeighborsFunc(im.config.NodeID)
	}
	
	if len(neighbors) == 0 {
		return []string{}, nil
	}
	
	// 计算传播分数
	propagatedScore := score * im.config.DefaultDecayFactor
	nextDepth := depth + 1
	
	propagatedTo := make([]string, 0)
	
	for _, neighborID := range neighbors {
		if neighborID == sourceNodeID || neighborID == im.config.NodeID {
			continue
		}
		
		err := im.propagateToNode(im.config.NodeID, neighborID, propagatedScore, nextDepth, originRewardID)
		if err == nil {
			propagatedTo = append(propagatedTo, neighborID)
		}
	}
	
	return propagatedTo, nil
}

// GetReward 获取奖励信息
func (im *IncentiveManager) GetReward(rewardID string) (*TaskReward, error) {
	im.mu.RLock()
	defer im.mu.RUnlock()
	
	reward, ok := im.rewards[rewardID]
	if !ok {
		return nil, ErrRewardNotFound
	}
	
	return reward, nil
}

// GetRewardByTask 根据任务ID获取奖励
func (im *IncentiveManager) GetRewardByTask(taskID string) (*TaskReward, error) {
	im.mu.RLock()
	defer im.mu.RUnlock()
	
	rewardID, ok := im.taskRewards[taskID]
	if !ok {
		return nil, ErrRewardNotFound
	}
	
	reward, ok := im.rewards[rewardID]
	if !ok {
		return nil, ErrRewardNotFound
	}
	
	return reward, nil
}

// GetNodeRewards 获取节点的所有奖励
func (im *IncentiveManager) GetNodeRewards(nodeID string) []*TaskReward {
	im.mu.RLock()
	defer im.mu.RUnlock()
	
	rewards := make([]*TaskReward, 0)
	for _, reward := range im.rewards {
		if reward.NodeID == nodeID {
			rewards = append(rewards, reward)
		}
	}
	return rewards
}

// GetPropagationRecords 获取传播记录
func (im *IncentiveManager) GetPropagationRecords(nodeID string) []*PropagationRecord {
	im.mu.RLock()
	defer im.mu.RUnlock()
	
	records := make([]*PropagationRecord, 0)
	for _, record := range im.propagations {
		if record.SourceNodeID == nodeID || record.TargetNodeID == nodeID {
			records = append(records, record)
		}
	}
	return records
}

// GetToleranceRecord 获取耐受值记录
func (im *IncentiveManager) GetToleranceRecord(sourceNodeID string) *ToleranceRecord {
	im.mu.RLock()
	defer im.mu.RUnlock()
	
	if tolerances, ok := im.tolerances[im.config.NodeID]; ok {
		if record, ok := tolerances[sourceNodeID]; ok {
			return record
		}
	}
	return nil
}

// GetAllTolerances 获取所有耐受值记录
func (im *IncentiveManager) GetAllTolerances() []*ToleranceRecord {
	im.mu.RLock()
	defer im.mu.RUnlock()
	
	records := make([]*ToleranceRecord, 0)
	if tolerances, ok := im.tolerances[im.config.NodeID]; ok {
		for _, record := range tolerances {
			records = append(records, record)
		}
	}
	return records
}

// ResetTolerance 手动重置某来源的耐受值
func (im *IncentiveManager) ResetTolerance(sourceNodeID string) error {
	im.mu.Lock()
	defer im.mu.Unlock()
	
	tolerances, ok := im.tolerances[im.config.NodeID]
	if !ok {
		return errors.New("no tolerance records found")
	}
	
	record, ok := tolerances[sourceNodeID]
	if !ok {
		return errors.New("tolerance record not found for source")
	}
	
	now := time.Now()
	record.TotalReceived = 0
	record.RemainingTolerance = record.MaxTolerance
	record.LastResetTime = now
	record.NextResetTime = now.Add(im.config.ToleranceResetPeriod)
	
	return nil
}

// SetTolerance 设置某来源的耐受值
func (im *IncentiveManager) SetTolerance(sourceNodeID string, tolerance float64) {
	im.mu.Lock()
	defer im.mu.Unlock()
	
	tolerances, ok := im.tolerances[im.config.NodeID]
	if !ok {
		tolerances = make(map[string]*ToleranceRecord)
		im.tolerances[im.config.NodeID] = tolerances
	}
	
	now := time.Now()
	
	if record, ok := tolerances[sourceNodeID]; ok {
		record.MaxTolerance = tolerance
		record.RemainingTolerance = tolerance - record.TotalReceived
		if record.RemainingTolerance < 0 {
			record.RemainingTolerance = 0
		}
	} else {
		tolerances[sourceNodeID] = &ToleranceRecord{
			SourceNodeID:       sourceNodeID,
			TargetNodeID:       im.config.NodeID,
			TotalReceived:      0,
			MaxTolerance:       tolerance,
			RemainingTolerance: tolerance,
			LastResetTime:      now,
			NextResetTime:      now.Add(im.config.ToleranceResetPeriod),
		}
	}
}

// CalculateTaskScore 计算任务分数
func (im *IncentiveManager) CalculateTaskScore(taskType TaskType, baseScore float64) float64 {
	if wc, ok := im.config.TaskWeights[taskType]; ok {
		// 限制范围
		if baseScore < wc.MinScore {
			baseScore = wc.MinScore
		}
		if baseScore > wc.MaxScore {
			baseScore = wc.MaxScore
		}
		return baseScore * wc.Weight
	}
	return baseScore
}

// CalculatePropagatedScore 计算传播后的分数
func (im *IncentiveManager) CalculatePropagatedScore(score float64, depth int) float64 {
	if depth < 1 {
		return score
	}
	
	decayed := score
	for i := 0; i < depth; i++ {
		decayed *= im.config.DefaultDecayFactor
	}
	return decayed
}

// IncentiveStats 激励系统统计
type IncentiveStats struct {
	TotalRewards          int64   `json:"total_rewards"`
	TotalScore            float64 `json:"total_score"`
	TotalPropagations     int64   `json:"total_propagations"`
	TotalPropagatedScore  float64 `json:"total_propagated_score"`
	ActiveTolerances      int     `json:"active_tolerances"`
	ExceededTolerances    int     `json:"exceeded_tolerances"`
	AverageRewardScore    float64 `json:"average_reward_score"`
}

// GetStats 获取统计信息
func (im *IncentiveManager) GetStats() *IncentiveStats {
	im.mu.RLock()
	defer im.mu.RUnlock()
	
	stats := &IncentiveStats{
		TotalRewards:      int64(len(im.rewards)),
		TotalPropagations: int64(len(im.propagations)),
	}
	
	for _, reward := range im.rewards {
		stats.TotalScore += reward.FinalScore
	}
	
	for _, record := range im.propagations {
		stats.TotalPropagatedScore += record.PropagatedScore
	}
	
	if tolerances, ok := im.tolerances[im.config.NodeID]; ok {
		stats.ActiveTolerances = len(tolerances)
		for _, record := range tolerances {
			if record.RemainingTolerance <= 0 {
				stats.ExceededTolerances++
			}
		}
	}
	
	if stats.TotalRewards > 0 {
		stats.AverageRewardScore = stats.TotalScore / float64(stats.TotalRewards)
	}
	
	return stats
}

// GetTaskWeightConfig 获取任务权重配置
func (im *IncentiveManager) GetTaskWeightConfig(taskType TaskType) *TaskWeightConfig {
	im.mu.RLock()
	defer im.mu.RUnlock()
	
	if wc, ok := im.config.TaskWeights[taskType]; ok {
		return wc
	}
	return nil
}

// SetTaskWeightConfig 设置任务权重配置
func (im *IncentiveManager) SetTaskWeightConfig(taskType TaskType, weight, minScore, maxScore float64) {
	im.mu.Lock()
	defer im.mu.Unlock()
	
	im.config.TaskWeights[taskType] = &TaskWeightConfig{
		TaskType: taskType,
		Weight:   weight,
		MinScore: minScore,
		MaxScore: maxScore,
	}
}

// SetDecayFactor 设置衰减因子
func (im *IncentiveManager) SetDecayFactor(factor float64) error {
	if factor <= 0 || factor >= 1 {
		return ErrInvalidDecayFactor
	}
	
	im.mu.Lock()
	im.config.DefaultDecayFactor = factor
	im.mu.Unlock()
	
	return nil
}

// SetDefaultTolerance 设置默认耐受值
func (im *IncentiveManager) SetDefaultTolerance(tolerance float64) {
	im.mu.Lock()
	im.config.DefaultTolerance = tolerance
	im.mu.Unlock()
}

// persistState 持久化状态
type persistState struct {
	Rewards      map[string]*TaskReward                 `json:"rewards"`
	TaskRewards  map[string]string                      `json:"task_rewards"`
	Propagations map[string]*PropagationRecord          `json:"propagations"`
	Tolerances   map[string]map[string]*ToleranceRecord `json:"tolerances"`
}

// save 保存数据
func (im *IncentiveManager) save() error {
	if im.config.DataDir == "" {
		return nil
	}
	
	im.mu.RLock()
	state := &persistState{
		Rewards:      im.rewards,
		TaskRewards:  im.taskRewards,
		Propagations: im.propagations,
		Tolerances:   im.tolerances,
	}
	im.mu.RUnlock()
	
	data, err := json.MarshalIndent(state, "", "  ")
	if err != nil {
		return err
	}
	
	filePath := filepath.Join(im.config.DataDir, "incentive.json")
	return os.WriteFile(filePath, data, 0644)
}

// load 加载数据
func (im *IncentiveManager) load() error {
	if im.config.DataDir == "" {
		return nil
	}
	
	filePath := filepath.Join(im.config.DataDir, "incentive.json")
	data, err := os.ReadFile(filePath)
	if err != nil {
		if os.IsNotExist(err) {
			return nil
		}
		return err
	}
	
	var state persistState
	if err := json.Unmarshal(data, &state); err != nil {
		return err
	}
	
	im.mu.Lock()
	defer im.mu.Unlock()
	
	if state.Rewards != nil {
		im.rewards = state.Rewards
	}
	if state.TaskRewards != nil {
		im.taskRewards = state.TaskRewards
	}
	if state.Propagations != nil {
		im.propagations = state.Propagations
	}
	if state.Tolerances != nil {
		im.tolerances = state.Tolerances
	}
	
	return nil
}

// Clear 清空所有数据
func (im *IncentiveManager) Clear() {
	im.mu.Lock()
	defer im.mu.Unlock()
	
	im.rewards = make(map[string]*TaskReward)
	im.taskRewards = make(map[string]string)
	im.propagations = make(map[string]*PropagationRecord)
	im.tolerances = make(map[string]map[string]*ToleranceRecord)
	im.tolerances[im.config.NodeID] = make(map[string]*ToleranceRecord)
}
