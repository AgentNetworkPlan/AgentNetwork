package reputation

import (
	"math"
	"sync"
)

// 信誉系统参数
const (
	Alpha  = 0.8  // 历史信誉衰减系数
	Lambda = 0.1  // 惩罚权重
	Delta  = 0.2  // Owner 信任传递系数
)

// Rating 评价
type Rating struct {
	FromAgentID string  // 评价者 Agent ID
	ToAgentID   string  // 被评价者 Agent ID
	Score       float64 // 评分 [-1, 1]
	Weight      float64 // 评价者权重
}

// Agent 代理信誉数据
type Agent struct {
	ID          string
	Score       float64 // 当前信誉值 [-1, 1]
	OwnerTrust  float64 // Owner 信任值
	Penalty     float64 // 惩罚项
	Ratings     []Rating
}

// System 信誉系统
type System struct {
	agents map[string]*Agent
	mu     sync.RWMutex
}

// NewSystem 创建信誉系统
func NewSystem() *System {
	return &System{
		agents: make(map[string]*Agent),
	}
}

// RegisterAgent 注册 Agent
func (s *System) RegisterAgent(id string, ownerTrust float64) {
	s.mu.Lock()
	defer s.mu.Unlock()

	s.agents[id] = &Agent{
		ID:         id,
		Score:      0, // 初始信誉为 0
		OwnerTrust: ownerTrust,
		Ratings:    make([]Rating, 0),
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

	agent.Ratings = append(agent.Ratings, rating)
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

	// 计算加权评分
	var weightedSum, weightSum float64
	for _, r := range agent.Ratings {
		weightedSum += r.Weight * r.Score
		weightSum += r.Weight
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

	// 清空已处理的评价
	agent.Ratings = make([]Rating, 0)

	return agent.Score
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
