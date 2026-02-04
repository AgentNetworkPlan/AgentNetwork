package reputation

import (
	"math"
	"sync"
	"time"
)

// 信誉系统参数
const (
	Alpha  = 0.8  // 历史信誉衰减系数
	Lambda = 0.1  // 惩罚权重
	Delta  = 0.2  // Owner 信任传递系数
	
	// 时间衰减参数
	DefaultHalfLifeDays = 30  // 默认半衰期（天）
	MinDecayFactor      = 0.1 // 最小衰减因子
)

// Rating 评价
type Rating struct {
	FromAgentID string    // 评价者 Agent ID
	ToAgentID   string    // 被评价者 Agent ID
	Score       float64   // 评分 [-1, 1]
	Weight      float64   // 评价者权重
	Timestamp   time.Time // 评价时间
}

// ReputationRecord 声誉记录（带时间戳）
type ReputationRecord struct {
	Score      float64   `json:"score"`      // 声誉分数
	Timestamp  time.Time `json:"timestamp"`  // 获得时间
	Source     string    `json:"source"`     // 来源
	SourceNode string    `json:"source_node"`// 来源节点
}

// Agent 代理信誉数据
type Agent struct {
	ID           string
	Score        float64 // 当前信誉值 [-1, 1]
	OwnerTrust   float64 // Owner 信任值
	Penalty      float64 // 惩罚项
	Ratings      []Rating
	Records      []ReputationRecord // 声誉记录（用于时间衰减计算）
	LastUpdated  time.Time          // 最后更新时间
}

// System 信誉系统
type System struct {
	agents       map[string]*Agent
	mu           sync.RWMutex
	halfLifeDays int // 半衰期（天）
}

// NewSystem 创建信誉系统
func NewSystem() *System {
	return &System{
		agents:       make(map[string]*Agent),
		halfLifeDays: DefaultHalfLifeDays,
	}
}

// NewSystemWithHalfLife 创建带自定义半衰期的信誉系统
func NewSystemWithHalfLife(halfLifeDays int) *System {
	if halfLifeDays <= 0 {
		halfLifeDays = DefaultHalfLifeDays
	}
	return &System{
		agents:       make(map[string]*Agent),
		halfLifeDays: halfLifeDays,
	}
}

// RegisterAgent 注册 Agent
func (s *System) RegisterAgent(id string, ownerTrust float64) {
	s.mu.Lock()
	defer s.mu.Unlock()

	s.agents[id] = &Agent{
		ID:          id,
		Score:       0, // 初始信誉为 0
		OwnerTrust:  ownerTrust,
		Ratings:     make([]Rating, 0),
		Records:     make([]ReputationRecord, 0),
		LastUpdated: time.Now(),
	}
}

// AddRating 添加评价
func (s *System) AddRating(rating Rating) {
	s.mu.Lock()
	defer s.mu.Unlock()

	agent, exists := s.agents[rating.ToAgentID]
	if !exists {
		return
	}

	// 设置时间戳
	if rating.Timestamp.IsZero() {
		rating.Timestamp = time.Now()
	}

	agent.Ratings = append(agent.Ratings, rating)
}

// AddReputationRecord 添加声誉记录（带时间戳，用于时间衰减）
func (s *System) AddReputationRecord(agentID string, score float64, source, sourceNode string) {
	s.mu.Lock()
	defer s.mu.Unlock()

	agent, exists := s.agents[agentID]
	if !exists {
		return
	}

	record := ReputationRecord{
		Score:      score,
		Timestamp:  time.Now(),
		Source:     source,
		SourceNode: sourceNode,
	}
	agent.Records = append(agent.Records, record)
}

// AddPenalty 添加惩罚
func (s *System) AddPenalty(agentID string, penalty float64) {
	s.mu.Lock()
	defer s.mu.Unlock()

	agent, exists := s.agents[agentID]
	if !exists {
		return
	}

	agent.Penalty += penalty
}

// UpdateScore 更新信誉值
// S_i = clip(α·S_i + (1-α)·(Σw_j·r_j/Σw_j) - λ·p_i + δ·T_owner, -1, 1)
func (s *System) UpdateScore(agentID string) float64 {
	s.mu.Lock()
	defer s.mu.Unlock()

	agent, exists := s.agents[agentID]
	if !exists {
		return 0
	}

	// 计算加权评分（带时间衰减）
	var weightedSum, weightSum float64
	now := time.Now()
	for _, r := range agent.Ratings {
		// 计算时间衰减因子
		decayFactor := s.calculateTimeDecay(r.Timestamp, now)
		adjustedWeight := r.Weight * decayFactor
		weightedSum += adjustedWeight * r.Score
		weightSum += adjustedWeight
	}

	var avgRating float64
	if weightSum > 0 {
		avgRating = weightedSum / weightSum
	}

	// 计算新信誉值
	newScore := Alpha*agent.Score +
		(1-Alpha)*avgRating -
		Lambda*agent.Penalty +
		Delta*agent.OwnerTrust

	// clip 到 [-1, 1]
	agent.Score = clip(newScore, -1, 1)
	agent.LastUpdated = now

	// 清空已处理的评价
	agent.Ratings = make([]Rating, 0)

	return agent.Score
}

// GetScoreWithDecay 获取带时间衰减的声誉值
// 基于声誉记录计算，越旧的记录贡献越小
func (s *System) GetScoreWithDecay(agentID string) float64 {
	s.mu.RLock()
	defer s.mu.RUnlock()

	agent, exists := s.agents[agentID]
	if !exists {
		return 0
	}

	if len(agent.Records) == 0 {
		return agent.Score
	}

	// 计算带时间衰减的声誉总分
	now := time.Now()
	var totalScore float64
	for _, record := range agent.Records {
		decayFactor := s.calculateTimeDecay(record.Timestamp, now)
		totalScore += record.Score * decayFactor
	}

	// 加上基础分数
	return clip(agent.Score+totalScore, -1, 1)
}

// calculateTimeDecay 计算时间衰减因子
// 使用指数衰减公式: decay = 0.5^(age/halfLife)
func (s *System) calculateTimeDecay(recordTime, now time.Time) float64 {
	if recordTime.IsZero() {
		return 1.0
	}
	
	age := now.Sub(recordTime)
	halfLife := time.Duration(s.halfLifeDays) * 24 * time.Hour
	
	// 指数衰减
	decay := math.Pow(0.5, float64(age)/float64(halfLife))
	
	// 确保不会低于最小值
	if decay < MinDecayFactor {
		return MinDecayFactor
	}
	return decay
}

// CalculateTimeDecay 公开的时间衰减计算函数
func CalculateTimeDecay(recordTime time.Time, halfLifeDays int) float64 {
	if recordTime.IsZero() {
		return 1.0
	}
	
	age := time.Since(recordTime)
	halfLife := time.Duration(halfLifeDays) * 24 * time.Hour
	
	decay := math.Pow(0.5, float64(age)/float64(halfLife))
	
	if decay < MinDecayFactor {
		return MinDecayFactor
	}
	return decay
}

// GetScore 获取信誉值
func (s *System) GetScore(agentID string) float64 {
	s.mu.RLock()
	defer s.mu.RUnlock()

	agent, exists := s.agents[agentID]
	if !exists {
		return 0
	}

	return agent.Score
}

// GetAllScores 获取所有 Agent 的信誉值
func (s *System) GetAllScores() map[string]float64 {
	s.mu.RLock()
	defer s.mu.RUnlock()

	scores := make(map[string]float64)
	for id, agent := range s.agents {
		scores[id] = agent.Score
	}

	return scores
}

// clip 将值限制在 [min, max] 范围内
func clip(value, min, max float64) float64 {
	return math.Max(min, math.Min(max, value))
}
