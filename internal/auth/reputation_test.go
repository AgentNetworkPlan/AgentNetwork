package auth

import (
	"testing"
	"time"
)

func TestNewReputationSystem(t *testing.T) {
	rs := NewReputationSystem()
	if rs == nil {
		t.Fatal("信誉系统不应为 nil")
	}
}

func TestReputationSystem_RegisterNode(t *testing.T) {
	rs := NewReputationSystem()

	err := rs.RegisterNode("node-001")
	if err != nil {
		t.Fatalf("注册节点失败: %v", err)
	}

	// 重复注册应失败
	err = rs.RegisterNode("node-001")
	if err == nil {
		t.Error("重复注册应该失败")
	}
}

func TestReputationSystem_GetReputation(t *testing.T) {
	rs := NewReputationSystem()
	rs.RegisterNode("node-001")

	rep, err := rs.GetReputation("node-001")
	if err != nil {
		t.Fatalf("获取信誉失败: %v", err)
	}

	if rep.Score != InitialReputation {
		t.Errorf("初始信誉应为 %f，实际为 %f", InitialReputation, rep.Score)
	}
}

func TestReputationSystem_OnTaskCompleted(t *testing.T) {
	rs := NewReputationSystem()
	rs.RegisterNode("node-001")

	initialScore, _ := rs.GetScore("node-001")

	err := rs.OnTaskCompleted("node-001", "task-001", 5)
	if err != nil {
		t.Fatalf("任务完成事件处理失败: %v", err)
	}

	newScore, _ := rs.GetScore("node-001")
	if newScore <= initialScore {
		t.Error("完成任务后信誉分应增加")
	}

	rep, _ := rs.GetReputation("node-001")
	if rep.TotalTasksCompleted != 1 {
		t.Errorf("完成任务数应为 1，实际为 %d", rep.TotalTasksCompleted)
	}
}

func TestReputationSystem_OnTaskFailed(t *testing.T) {
	rs := NewReputationSystem()
	rs.RegisterNode("node-001")

	initialScore, _ := rs.GetScore("node-001")

	err := rs.OnTaskFailed("node-001", "task-001")
	if err != nil {
		t.Fatalf("任务失败事件处理失败: %v", err)
	}

	newScore, _ := rs.GetScore("node-001")
	if newScore >= initialScore {
		t.Error("任务失败后信誉分应减少")
	}

	rep, _ := rs.GetReputation("node-001")
	if rep.TotalTasksFailed != 1 {
		t.Errorf("失败任务数应为 1，实际为 %d", rep.TotalTasksFailed)
	}
}

func TestReputationSystem_OnVerificationResult(t *testing.T) {
	rs := NewReputationSystem()
	rs.RegisterNode("node-001")

	// 正确验证
	initialScore, _ := rs.GetScore("node-001")
	rs.OnVerificationResult("node-001", "task-001", true)
	newScore, _ := rs.GetScore("node-001")
	if newScore <= initialScore {
		t.Error("正确验证后信誉分应增加")
	}

	rep, _ := rs.GetReputation("node-001")
	if rep.CorrectVerifications != 1 {
		t.Errorf("正确验证数应为 1，实际为 %d", rep.CorrectVerifications)
	}

	// 错误验证
	initialScore = newScore
	rs.OnVerificationResult("node-001", "task-002", false)
	newScore, _ = rs.GetScore("node-001")
	if newScore >= initialScore {
		t.Error("错误验证后信誉分应减少")
	}
}

func TestReputationSystem_OnSybilDetected(t *testing.T) {
	rs := NewReputationSystem()
	rs.RegisterNode("node-001")

	initialScore, _ := rs.GetScore("node-001")

	err := rs.OnSybilDetected("node-001", "可疑行为")
	if err != nil {
		t.Fatalf("Sybil 检测事件处理失败: %v", err)
	}

	newScore, _ := rs.GetScore("node-001")
	if newScore >= initialScore {
		t.Error("Sybil 检测后信誉分应大幅减少")
	}
}

func TestReputationSystem_Ban(t *testing.T) {
	rs := NewReputationSystem()
	rs.RegisterNode("node-001")

	// 多次 Sybil 检测导致封禁
	for i := 0; i < 3; i++ {
		rs.OnSybilDetected("node-001", "可疑行为")
	}

	rep, _ := rs.GetReputation("node-001")
	if !rep.IsBanned {
		t.Error("信誉过低的节点应被封禁")
	}
}

func TestReputationSystem_ApplyDailyDecay(t *testing.T) {
	rs := NewReputationSystem()
	rs.RegisterNode("node-001")

	// 增加信誉
	for i := 0; i < 5; i++ {
		rs.OnTaskCompleted("node-001", "task", 5)
	}

	initialScore, _ := rs.GetScore("node-001")

	// 应用衰减
	rs.ApplyDailyDecay()

	newScore, _ := rs.GetScore("node-001")
	if newScore >= initialScore {
		t.Error("高于基准值的信誉应该衰减")
	}
}

func TestReputationSystem_GetTopNodes(t *testing.T) {
	rs := NewReputationSystem()

	// 注册多个节点
	nodes := []string{"node-a", "node-b", "node-c", "node-d", "node-e"}
	for _, nodeID := range nodes {
		rs.RegisterNode(nodeID)
	}

	// 给不同节点不同的信誉
	rs.OnTaskCompleted("node-a", "task", 5)
	rs.OnTaskCompleted("node-a", "task", 5)
	rs.OnTaskCompleted("node-b", "task", 5)
	rs.OnTaskFailed("node-c", "task")

	// 获取前 3 名
	topNodes := rs.GetTopNodes(3)
	if len(topNodes) != 3 {
		t.Errorf("应返回 3 个节点，实际返回 %d", len(topNodes))
	}

	// 验证排序
	if topNodes[0].NodeID != "node-a" {
		t.Error("node-a 应该排第一")
	}
}

func TestReputationSystem_GetQualifiedVerifiers(t *testing.T) {
	rs := NewReputationSystem()

	// 注册多个节点
	nodes := []string{"node-a", "node-b", "node-c"}
	for _, nodeID := range nodes {
		rs.RegisterNode(nodeID)
	}

	// 给部分节点高信誉
	rs.OnTaskCompleted("node-a", "task", 5)
	rs.OnTaskCompleted("node-a", "task", 5)
	rs.OnTaskFailed("node-c", "task")
	rs.OnTaskFailed("node-c", "task")

	// 获取合格验证者 (信誉 > 0.5)
	qualified := rs.GetQualifiedVerifiers(0.5)
	
	// 检查 node-c 不在列表中 (信誉低于阈值)
	hasNodeC := false
	for _, rep := range qualified {
		if rep.NodeID == "node-c" {
			hasNodeC = true
		}
	}
	if hasNodeC {
		t.Error("node-c 信誉过低，不应在合格验证者列表中")
	}
}

func TestReputationSystem_CalculateTrustScore(t *testing.T) {
	rs := NewReputationSystem()
	rs.RegisterNode("node-001")

	// 完成一些任务和验证
	rs.OnTaskCompleted("node-001", "task-1", 5)
	rs.OnTaskCompleted("node-001", "task-2", 5)
	rs.OnVerificationResult("node-001", "task-3", true)
	rs.OnVerificationResult("node-001", "task-4", true)

	trustScore, err := rs.CalculateTrustScore("node-001")
	if err != nil {
		t.Fatalf("计算信任分数失败: %v", err)
	}

	if trustScore <= 0 || trustScore > 1 {
		t.Errorf("信任分数应在 0-1 之间，实际为 %f", trustScore)
	}
}

func TestReputationSystem_GetNodeRecords(t *testing.T) {
	rs := NewReputationSystem()
	rs.RegisterNode("node-001")

	rs.OnTaskCompleted("node-001", "task-1", 5)
	rs.OnTaskFailed("node-001", "task-2")

	records := rs.GetNodeRecords("node-001")
	
	// 应有 3 条记录：注册、完成、失败
	if len(records) != 3 {
		t.Errorf("应有 3 条记录，实际有 %d 条", len(records))
	}
}

func TestReputationSystem_ExportImportState(t *testing.T) {
	rs := NewReputationSystem()
	rs.RegisterNode("node-001")
	rs.OnTaskCompleted("node-001", "task-1", 5)

	// 导出状态
	data, err := rs.ExportState()
	if err != nil {
		t.Fatalf("导出状态失败: %v", err)
	}

	// 创建新系统并导入
	rs2 := NewReputationSystem()
	err = rs2.ImportState(data)
	if err != nil {
		t.Fatalf("导入状态失败: %v", err)
	}

	// 验证状态
	rep, _ := rs2.GetReputation("node-001")
	if rep.TotalTasksCompleted != 1 {
		t.Error("导入后状态不匹配")
	}
}

func TestReputationClamp(t *testing.T) {
	// 测试信誉限制
	result := clampReputation(1.5)
	if result != MaxReputation {
		t.Errorf("超过最大值应被限制为 %f，实际为 %f", MaxReputation, result)
	}

	result = clampReputation(-1.5)
	if result != MinReputation {
		t.Errorf("低于最小值应被限制为 %f，实际为 %f", MinReputation, result)
	}

	result = clampReputation(0.5)
	if result != 0.5 {
		t.Errorf("正常值不应被修改，期望 0.5，实际为 %f", result)
	}
}

// 测试时间衰减
func TestReputationSystem_ActivityDecay(t *testing.T) {
	rs := NewReputationSystem()
	rs.RegisterNode("node-001")

	// 完成任务增加信誉
	rs.OnTaskCompleted("node-001", "task", 5)

	// 手动设置最后活动时间为30天前
	rep, _ := rs.GetReputation("node-001")
	rep.LastActivityAt = time.Now().Add(-30 * 24 * time.Hour)

	// 计算信任分数（会考虑活跃度）
	trustScore, _ := rs.CalculateTrustScore("node-001")

	// 长时间不活跃应该降低信任分数
	if trustScore >= 0.8 {
		t.Error("长时间不活跃的节点信任分数应该较低")
	}
}
