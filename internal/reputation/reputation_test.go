package reputation

import (
	"testing"
)

func TestNewSystem(t *testing.T) {
	sys := NewSystem()
	if sys == nil {
		t.Fatal("系统为空")
	}

	if sys.agents == nil {
		t.Error("agents map 未初始化")
	}
}

func TestSystem_RegisterAgent(t *testing.T) {
	sys := NewSystem()

	sys.RegisterAgent("agent-1", 0.5)

	score := sys.GetScore("agent-1")
	if score != 0 {
		t.Errorf("初始信誉值错误: %f (应该是 0)", score)
	}
}

func TestSystem_RegisterMultipleAgents(t *testing.T) {
	sys := NewSystem()

	agents := []string{"agent-1", "agent-2", "agent-3"}
	for _, id := range agents {
		sys.RegisterAgent(id, 0.5)
	}

	scores := sys.GetAllScores()
	if len(scores) != 3 {
		t.Errorf("注册的 Agent 数量错误: %d", len(scores))
	}
}

func TestSystem_AddRating(t *testing.T) {
	sys := NewSystem()
	sys.RegisterAgent("agent-1", 0.5)
	sys.RegisterAgent("agent-2", 0.5)

	// agent-2 给 agent-1 评分
	rating := Rating{
		FromAgentID: "agent-2",
		ToAgentID:   "agent-1",
		Score:       0.8,
		Weight:      1.0,
	}

	sys.AddRating(rating)

	// 更新分数
	newScore := sys.UpdateScore("agent-1")
	if newScore <= 0 {
		t.Errorf("更新后的分数应该大于 0: %f", newScore)
	}

	t.Logf("更新后的信誉值: %f", newScore)
}

func TestSystem_AddPenalty(t *testing.T) {
	sys := NewSystem()
	sys.RegisterAgent("agent-1", 0.5)

	// 先给一些正面评价
	sys.AddRating(Rating{
		FromAgentID: "agent-2",
		ToAgentID:   "agent-1",
		Score:       0.5,
		Weight:      1.0,
	})

	// 添加惩罚
	sys.AddPenalty("agent-1", 0.5)

	// 更新分数
	score := sys.UpdateScore("agent-1")
	t.Logf("添加惩罚后的信誉值: %f", score)

	// 分数应该受到惩罚影响
}

func TestSystem_UpdateScore_Algorithm(t *testing.T) {
	sys := NewSystem()
	sys.RegisterAgent("agent-1", 0.5) // ownerTrust = 0.5

	// 添加多个评价
	ratings := []Rating{
		{FromAgentID: "a", ToAgentID: "agent-1", Score: 0.9, Weight: 1.0},
		{FromAgentID: "b", ToAgentID: "agent-1", Score: 0.8, Weight: 0.8},
		{FromAgentID: "c", ToAgentID: "agent-1", Score: 0.7, Weight: 0.6},
	}

	for _, r := range ratings {
		sys.AddRating(r)
	}

	score := sys.UpdateScore("agent-1")

	// 验证分数在 [-1, 1] 范围内
	if score < -1 || score > 1 {
		t.Errorf("分数超出范围: %f", score)
	}

	t.Logf("计算得到的信誉值: %f", score)
}

func TestSystem_UpdateScore_NegativeRatings(t *testing.T) {
	sys := NewSystem()
	sys.RegisterAgent("agent-1", 0.0)

	// 添加负面评价
	ratings := []Rating{
		{FromAgentID: "a", ToAgentID: "agent-1", Score: -0.8, Weight: 1.0},
		{FromAgentID: "b", ToAgentID: "agent-1", Score: -0.9, Weight: 1.0},
	}

	for _, r := range ratings {
		sys.AddRating(r)
	}

	score := sys.UpdateScore("agent-1")

	// 分数应该是负的
	if score >= 0 {
		t.Errorf("负面评价后分数应该为负: %f", score)
	}

	// 验证分数在 [-1, 1] 范围内
	if score < -1 {
		t.Errorf("分数低于 -1: %f", score)
	}

	t.Logf("负面评价后的信誉值: %f", score)
}

func TestSystem_GetScore_NonExistent(t *testing.T) {
	sys := NewSystem()

	score := sys.GetScore("nonexistent")
	if score != 0 {
		t.Errorf("不存在的 Agent 分数应该为 0: %f", score)
	}
}

func TestSystem_AddRating_NonExistent(t *testing.T) {
	sys := NewSystem()

	// 给不存在的 Agent 评分不应该 panic
	rating := Rating{
		FromAgentID: "a",
		ToAgentID:   "nonexistent",
		Score:       0.5,
		Weight:      1.0,
	}

	sys.AddRating(rating) // 不应该 panic
}

func TestSystem_UpdateScore_NonExistent(t *testing.T) {
	sys := NewSystem()

	score := sys.UpdateScore("nonexistent")
	if score != 0 {
		t.Errorf("不存在的 Agent 更新分数应该为 0: %f", score)
	}
}

func TestSystem_ScoreDecay(t *testing.T) {
	sys := NewSystem()
	sys.RegisterAgent("agent-1", 0.5)

	// 第一次评价
	sys.AddRating(Rating{
		FromAgentID: "a",
		ToAgentID:   "agent-1",
		Score:       1.0,
		Weight:      1.0,
	})
	score1 := sys.UpdateScore("agent-1")

	// 第二次评价（较低）
	sys.AddRating(Rating{
		FromAgentID: "b",
		ToAgentID:   "agent-1",
		Score:       0.5,
		Weight:      1.0,
	})
	score2 := sys.UpdateScore("agent-1")

	t.Logf("第一次更新: %f, 第二次更新: %f", score1, score2)

	// 由于 alpha=0.8 的衰减，分数会考虑历史
	// 第二次分数应该受第一次的影响
}

func TestClip(t *testing.T) {
	tests := []struct {
		value, min, max, expected float64
	}{
		{0.5, -1, 1, 0.5},
		{-2.0, -1, 1, -1.0},
		{2.0, -1, 1, 1.0},
		{0, -1, 1, 0},
		{-1, -1, 1, -1},
		{1, -1, 1, 1},
	}

	for _, tt := range tests {
		result := clip(tt.value, tt.min, tt.max)
		if result != tt.expected {
			t.Errorf("clip(%f, %f, %f) = %f, 期望 %f",
				tt.value, tt.min, tt.max, result, tt.expected)
		}
	}
}

func TestConstants(t *testing.T) {
	if Alpha != 0.8 {
		t.Errorf("Alpha 常量错误: %f", Alpha)
	}
	if Lambda != 0.1 {
		t.Errorf("Lambda 常量错误: %f", Lambda)
	}
	if Delta != 0.2 {
		t.Errorf("Delta 常量错误: %f", Delta)
	}
}
