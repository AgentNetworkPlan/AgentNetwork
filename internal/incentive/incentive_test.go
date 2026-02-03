package incentive

import (
	"sync"
	"testing"
	"time"
)

func createTestManager(t *testing.T) *IncentiveManager {
	t.Helper()
	
	tmpDir := t.TempDir()
	config := &IncentiveConfig{
		NodeID:              "test-node-001",
		DataDir:             tmpDir,
		DefaultDecayFactor:  0.7,
		DefaultTolerance:    50.0,
		ToleranceResetPeriod: 24 * time.Hour,
		MinPropagationScore: 0.1,
		MaxPropagationDepth: 5,
		TaskWeights: map[TaskType]*TaskWeightConfig{
			TaskTypeGeneral: {TaskType: TaskTypeGeneral, Weight: 1.0, MinScore: 1, MaxScore: 10},
			TaskTypeRelay:   {TaskType: TaskTypeRelay, Weight: 1.2, MinScore: 1, MaxScore: 15},
			TaskTypeAudit:   {TaskType: TaskTypeAudit, Weight: 1.5, MinScore: 5, MaxScore: 20},
		},
	}
	
	im, err := NewIncentiveManager(config)
	if err != nil {
		t.Fatalf("Failed to create incentive manager: %v", err)
	}
	
	return im
}

func TestNewIncentiveManager(t *testing.T) {
	t.Run("nil config", func(t *testing.T) {
		_, err := NewIncentiveManager(nil)
		if err != ErrNilConfig {
			t.Errorf("expected ErrNilConfig, got %v", err)
		}
	})
	
	t.Run("empty node ID", func(t *testing.T) {
		config := &IncentiveConfig{}
		_, err := NewIncentiveManager(config)
		if err != ErrEmptyNodeID {
			t.Errorf("expected ErrEmptyNodeID, got %v", err)
		}
	})
	
	t.Run("valid config", func(t *testing.T) {
		im := createTestManager(t)
		if im == nil {
			t.Fatal("expected non-nil manager")
		}
	})
}

func TestDefaultConfig(t *testing.T) {
	config := DefaultIncentiveConfig("test-node")
	
	if config.NodeID != "test-node" {
		t.Errorf("NodeID = %s, want test-node", config.NodeID)
	}
	if config.DefaultDecayFactor != 0.7 {
		t.Errorf("DefaultDecayFactor = %f, want 0.7", config.DefaultDecayFactor)
	}
	if config.DefaultTolerance != 50.0 {
		t.Errorf("DefaultTolerance = %f, want 50.0", config.DefaultTolerance)
	}
	if len(config.TaskWeights) == 0 {
		t.Error("expected TaskWeights to be populated")
	}
}

func TestAwardTaskCompletion(t *testing.T) {
	im := createTestManager(t)
	
	reward, err := im.AwardTaskCompletion("node-001", "task-001", TaskTypeGeneral, 10.0, "Test task")
	if err != nil {
		t.Fatalf("AwardTaskCompletion failed: %v", err)
	}
	
	if reward.RewardID == "" {
		t.Error("expected non-empty reward ID")
	}
	if reward.NodeID != "node-001" {
		t.Errorf("NodeID = %s, want node-001", reward.NodeID)
	}
	if reward.TaskID != "task-001" {
		t.Errorf("TaskID = %s, want task-001", reward.TaskID)
	}
	if reward.BaseScore != 10.0 {
		t.Errorf("BaseScore = %f, want 10.0", reward.BaseScore)
	}
	if reward.TaskWeight != 1.0 {
		t.Errorf("TaskWeight = %f, want 1.0", reward.TaskWeight)
	}
	if reward.FinalScore != 10.0 {
		t.Errorf("FinalScore = %f, want 10.0", reward.FinalScore)
	}
	if reward.Status != RewardStatusConfirmed {
		t.Errorf("Status = %s, want confirmed", reward.Status)
	}
}

func TestAwardTaskCompletionWithWeight(t *testing.T) {
	im := createTestManager(t)
	
	// 使用有权重的任务类型
	reward, err := im.AwardTaskCompletion("node-001", "task-002", TaskTypeAudit, 10.0, "Audit task")
	if err != nil {
		t.Fatalf("AwardTaskCompletion failed: %v", err)
	}
	
	// Weight = 1.5
	expectedScore := 10.0 * 1.5
	if reward.FinalScore != expectedScore {
		t.Errorf("FinalScore = %f, want %f", reward.FinalScore, expectedScore)
	}
}

func TestAwardTaskCompletionErrors(t *testing.T) {
	im := createTestManager(t)
	
	t.Run("empty node ID", func(t *testing.T) {
		_, err := im.AwardTaskCompletion("", "task", TaskTypeGeneral, 10, "")
		if err != ErrEmptyNodeID {
			t.Errorf("expected ErrEmptyNodeID, got %v", err)
		}
	})
	
	t.Run("empty task ID", func(t *testing.T) {
		_, err := im.AwardTaskCompletion("node", "", TaskTypeGeneral, 10, "")
		if err != ErrEmptyTaskID {
			t.Errorf("expected ErrEmptyTaskID, got %v", err)
		}
	})
	
	t.Run("invalid score", func(t *testing.T) {
		_, err := im.AwardTaskCompletion("node", "task", TaskTypeGeneral, 0, "")
		if err != ErrInvalidScore {
			t.Errorf("expected ErrInvalidScore, got %v", err)
		}
	})
	
	t.Run("duplicate reward", func(t *testing.T) {
		im.AwardTaskCompletion("node", "dup-task", TaskTypeGeneral, 10, "")
		_, err := im.AwardTaskCompletion("node", "dup-task", TaskTypeGeneral, 10, "")
		if err != ErrDuplicateReward {
			t.Errorf("expected ErrDuplicateReward, got %v", err)
		}
	})
}

func TestAwardTaskCompletionScoreLimit(t *testing.T) {
	im := createTestManager(t)
	
	// 超过最大分数
	reward, _ := im.AwardTaskCompletion("node", "task-max", TaskTypeGeneral, 100, "")
	
	// MaxScore for General = 10, Weight = 1.0
	if reward.FinalScore != 10.0 {
		t.Errorf("FinalScore = %f, want 10.0 (max)", reward.FinalScore)
	}
	
	// 低于最小分数
	reward, _ = im.AwardTaskCompletion("node", "task-min", TaskTypeAudit, 1, "")
	
	// MinScore for Audit = 5, Weight = 1.5
	expectedMin := 5.0 * 1.5
	if reward.FinalScore != expectedMin {
		t.Errorf("FinalScore = %f, want %f (min)", reward.FinalScore, expectedMin)
	}
}

func TestGetReward(t *testing.T) {
	im := createTestManager(t)
	
	reward, _ := im.AwardTaskCompletion("node", "task", TaskTypeGeneral, 10, "")
	
	t.Run("found", func(t *testing.T) {
		found, err := im.GetReward(reward.RewardID)
		if err != nil {
			t.Fatalf("GetReward failed: %v", err)
		}
		if found.TaskID != "task" {
			t.Errorf("TaskID = %s, want task", found.TaskID)
		}
	})
	
	t.Run("not found", func(t *testing.T) {
		_, err := im.GetReward("nonexistent")
		if err != ErrRewardNotFound {
			t.Errorf("expected ErrRewardNotFound, got %v", err)
		}
	})
}

func TestGetRewardByTask(t *testing.T) {
	im := createTestManager(t)
	
	im.AwardTaskCompletion("node", "find-task", TaskTypeGeneral, 10, "")
	
	reward, err := im.GetRewardByTask("find-task")
	if err != nil {
		t.Fatalf("GetRewardByTask failed: %v", err)
	}
	if reward.TaskID != "find-task" {
		t.Errorf("TaskID = %s, want find-task", reward.TaskID)
	}
	
	_, err = im.GetRewardByTask("nonexistent")
	if err != ErrRewardNotFound {
		t.Errorf("expected ErrRewardNotFound, got %v", err)
	}
}

func TestGetNodeRewards(t *testing.T) {
	im := createTestManager(t)
	
	im.AwardTaskCompletion("node-a", "task-1", TaskTypeGeneral, 10, "")
	im.AwardTaskCompletion("node-a", "task-2", TaskTypeGeneral, 10, "")
	im.AwardTaskCompletion("node-b", "task-3", TaskTypeGeneral, 10, "")
	
	rewards := im.GetNodeRewards("node-a")
	if len(rewards) != 2 {
		t.Errorf("rewards count = %d, want 2", len(rewards))
	}
	
	rewards = im.GetNodeRewards("node-b")
	if len(rewards) != 1 {
		t.Errorf("rewards count = %d, want 1", len(rewards))
	}
}

func TestPropagateReputation(t *testing.T) {
	tmpDir := t.TempDir()
	config := &IncentiveConfig{
		NodeID:              "test-node",
		DataDir:             tmpDir,
		DefaultDecayFactor:  0.7,
		DefaultTolerance:    100.0,
		ToleranceResetPeriod: 24 * time.Hour,
		MinPropagationScore: 0.1,
		MaxPropagationDepth: 5,
		TaskWeights: map[TaskType]*TaskWeightConfig{
			TaskTypeGeneral: {Weight: 1.0, MinScore: 1, MaxScore: 100},
		},
		GetNeighborsFunc: func(nodeID string) []string {
			return []string{"neighbor-1", "neighbor-2", "neighbor-3"}
		},
	}
	
	im, _ := NewIncentiveManager(config)
	
	// 初始化邻居的耐受值记录
	im.mu.Lock()
	im.tolerances["neighbor-1"] = make(map[string]*ToleranceRecord)
	im.tolerances["neighbor-2"] = make(map[string]*ToleranceRecord)
	im.tolerances["neighbor-3"] = make(map[string]*ToleranceRecord)
	im.mu.Unlock()
	
	reward, _ := im.AwardTaskCompletion("source-node", "prop-task", TaskTypeGeneral, 10, "")
	
	propagatedTo, err := im.PropagateReputation(reward.RewardID)
	if err != nil {
		t.Fatalf("PropagateReputation failed: %v", err)
	}
	
	if len(propagatedTo) != 3 {
		t.Errorf("propagatedTo count = %d, want 3", len(propagatedTo))
	}
	
	// 检查状态
	reward, _ = im.GetReward(reward.RewardID)
	if reward.Status != RewardStatusPropagated {
		t.Errorf("Status = %s, want propagated", reward.Status)
	}
}

func TestPropagateReputationNotFound(t *testing.T) {
	im := createTestManager(t)
	
	_, err := im.PropagateReputation("nonexistent")
	if err != ErrRewardNotFound {
		t.Errorf("expected ErrRewardNotFound, got %v", err)
	}
}

func TestReceivePropagation(t *testing.T) {
	im := createTestManager(t)
	
	err := im.ReceivePropagation("source-node", 10.0, 1, "reward-001")
	if err != nil {
		t.Fatalf("ReceivePropagation failed: %v", err)
	}
	
	// 检查传播记录
	records := im.GetPropagationRecords(im.config.NodeID)
	if len(records) != 1 {
		t.Errorf("records count = %d, want 1", len(records))
	}
	
	// 检查耐受值更新
	tolerance := im.GetToleranceRecord("source-node")
	if tolerance == nil {
		t.Fatal("expected tolerance record to exist")
	}
	
	// 传播分数 = 10 * 0.7 = 7
	expectedReceived := 7.0
	if tolerance.TotalReceived != expectedReceived {
		t.Errorf("TotalReceived = %f, want %f", tolerance.TotalReceived, expectedReceived)
	}
}

func TestReceivePropagationErrors(t *testing.T) {
	im := createTestManager(t)
	
	t.Run("empty source", func(t *testing.T) {
		err := im.ReceivePropagation("", 10, 1, "reward")
		if err != ErrEmptyNodeID {
			t.Errorf("expected ErrEmptyNodeID, got %v", err)
		}
	})
	
	t.Run("invalid score", func(t *testing.T) {
		err := im.ReceivePropagation("source", 0, 1, "reward")
		if err != ErrInvalidScore {
			t.Errorf("expected ErrInvalidScore, got %v", err)
		}
	})
	
	t.Run("self propagation", func(t *testing.T) {
		err := im.ReceivePropagation("test-node-001", 10, 1, "reward") // same as config.NodeID
		if err != ErrSelfPropagation {
			t.Errorf("expected ErrSelfPropagation, got %v", err)
		}
	})
}

func TestToleranceExceeded(t *testing.T) {
	tmpDir := t.TempDir()
	config := &IncentiveConfig{
		NodeID:              "test-node",
		DataDir:             tmpDir,
		DefaultDecayFactor:  0.7,
		DefaultTolerance:    10.0, // 低耐受值
		ToleranceResetPeriod: 24 * time.Hour,
		MinPropagationScore: 0.1,
		MaxPropagationDepth: 5,
		TaskWeights: map[TaskType]*TaskWeightConfig{
			TaskTypeGeneral: {Weight: 1.0, MinScore: 1, MaxScore: 100},
		},
	}
	
	im, _ := NewIncentiveManager(config)
	
	// 第一次传播成功 (10 * 0.7 = 7)
	err := im.ReceivePropagation("source", 10, 1, "reward-1")
	if err != nil {
		t.Fatalf("first propagation should succeed: %v", err)
	}
	
	// 第二次超过耐受值 (7 + 7 = 14 > 10)
	err = im.ReceivePropagation("source", 10, 1, "reward-2")
	if err != ErrToleranceExceeded {
		t.Errorf("expected ErrToleranceExceeded, got %v", err)
	}
}

func TestToleranceCallback(t *testing.T) {
	tmpDir := t.TempDir()
	config := &IncentiveConfig{
		NodeID:              "test-node",
		DataDir:             tmpDir,
		DefaultDecayFactor:  0.7,
		DefaultTolerance:    5.0,
		ToleranceResetPeriod: 24 * time.Hour,
		MinPropagationScore: 0.1,
		MaxPropagationDepth: 5,
		TaskWeights: map[TaskType]*TaskWeightConfig{
			TaskTypeGeneral: {Weight: 1.0, MinScore: 1, MaxScore: 100},
		},
	}
	
	im, _ := NewIncentiveManager(config)
	
	var exceededSource, exceededTarget string
	im.OnToleranceExceeded = func(source, target string, score float64) {
		exceededSource = source
		exceededTarget = target
	}
	
	im.ReceivePropagation("source", 10, 1, "reward-1")
	im.ReceivePropagation("source", 10, 1, "reward-2")
	
	if exceededSource != "source" {
		t.Errorf("exceededSource = %s, want source", exceededSource)
	}
	if exceededTarget != "test-node" {
		t.Errorf("exceededTarget = %s, want test-node", exceededTarget)
	}
}

func TestResetTolerance(t *testing.T) {
	im := createTestManager(t)
	
	// 先接收一些传播
	im.ReceivePropagation("source", 10, 1, "reward")
	
	tolerance := im.GetToleranceRecord("source")
	if tolerance.TotalReceived == 0 {
		t.Error("expected TotalReceived > 0")
	}
	
	// 重置
	err := im.ResetTolerance("source")
	if err != nil {
		t.Fatalf("ResetTolerance failed: %v", err)
	}
	
	tolerance = im.GetToleranceRecord("source")
	if tolerance.TotalReceived != 0 {
		t.Errorf("TotalReceived = %f, want 0", tolerance.TotalReceived)
	}
	if tolerance.RemainingTolerance != tolerance.MaxTolerance {
		t.Errorf("RemainingTolerance = %f, want %f", tolerance.RemainingTolerance, tolerance.MaxTolerance)
	}
}

func TestSetTolerance(t *testing.T) {
	im := createTestManager(t)
	
	im.SetTolerance("new-source", 100.0)
	
	tolerance := im.GetToleranceRecord("new-source")
	if tolerance == nil {
		t.Fatal("expected tolerance record to exist")
	}
	if tolerance.MaxTolerance != 100.0 {
		t.Errorf("MaxTolerance = %f, want 100.0", tolerance.MaxTolerance)
	}
}

func TestContinuePropagation(t *testing.T) {
	tmpDir := t.TempDir()
	config := &IncentiveConfig{
		NodeID:              "test-node",
		DataDir:             tmpDir,
		DefaultDecayFactor:  0.7,
		DefaultTolerance:    100.0,
		ToleranceResetPeriod: 24 * time.Hour,
		MinPropagationScore: 0.1,
		MaxPropagationDepth: 5,
		TaskWeights: map[TaskType]*TaskWeightConfig{
			TaskTypeGeneral: {Weight: 1.0, MinScore: 1, MaxScore: 100},
		},
		GetNeighborsFunc: func(nodeID string) []string {
			return []string{"neighbor-a", "neighbor-b"}
		},
	}
	
	im, _ := NewIncentiveManager(config)
	
	// 初始化邻居的耐受值
	im.mu.Lock()
	im.tolerances["neighbor-a"] = make(map[string]*ToleranceRecord)
	im.tolerances["neighbor-b"] = make(map[string]*ToleranceRecord)
	im.mu.Unlock()
	
	propagatedTo, err := im.ContinuePropagation("source", 10.0, 1, "origin-reward")
	if err != nil {
		t.Fatalf("ContinuePropagation failed: %v", err)
	}
	
	// 应该传播到2个邻居
	if len(propagatedTo) != 2 {
		t.Errorf("propagatedTo count = %d, want 2", len(propagatedTo))
	}
}

func TestMaxPropagationDepth(t *testing.T) {
	tmpDir := t.TempDir()
	config := &IncentiveConfig{
		NodeID:              "test-node",
		DataDir:             tmpDir,
		DefaultDecayFactor:  0.7,
		DefaultTolerance:    100.0,
		ToleranceResetPeriod: 24 * time.Hour,
		MinPropagationScore: 0.1,
		MaxPropagationDepth: 3,
		TaskWeights: map[TaskType]*TaskWeightConfig{
			TaskTypeGeneral: {Weight: 1.0, MinScore: 1, MaxScore: 100},
		},
	}
	
	im, _ := NewIncentiveManager(config)
	
	// 深度超过限制
	err := im.ReceivePropagation("source", 100, 4, "reward")
	if err == nil {
		t.Error("expected error for exceeded depth")
	}
}

func TestMinPropagationScore(t *testing.T) {
	tmpDir := t.TempDir()
	config := &IncentiveConfig{
		NodeID:              "test-node",
		DataDir:             tmpDir,
		DefaultDecayFactor:  0.7,
		DefaultTolerance:    100.0,
		ToleranceResetPeriod: 24 * time.Hour,
		MinPropagationScore: 1.0, // 高最小值
		MaxPropagationDepth: 5,
		TaskWeights: map[TaskType]*TaskWeightConfig{
			TaskTypeGeneral: {Weight: 1.0, MinScore: 1, MaxScore: 100},
		},
	}
	
	im, _ := NewIncentiveManager(config)
	
	// 分数太小 (0.5 * 0.7 = 0.35 < 1.0)
	err := im.ReceivePropagation("source", 0.5, 1, "reward")
	if err == nil {
		t.Error("expected error for small propagation score")
	}
}

func TestCalculateTaskScore(t *testing.T) {
	im := createTestManager(t)
	
	score := im.CalculateTaskScore(TaskTypeGeneral, 5)
	if score != 5.0 {
		t.Errorf("score = %f, want 5.0", score)
	}
	
	score = im.CalculateTaskScore(TaskTypeAudit, 10)
	if score != 15.0 { // 10 * 1.5
		t.Errorf("score = %f, want 15.0", score)
	}
	
	// 超过最大值
	score = im.CalculateTaskScore(TaskTypeGeneral, 100)
	if score != 10.0 { // MaxScore = 10
		t.Errorf("score = %f, want 10.0 (max)", score)
	}
}

func TestCalculatePropagatedScore(t *testing.T) {
	im := createTestManager(t)
	
	// Depth 0
	score := im.CalculatePropagatedScore(10, 0)
	if score != 10.0 {
		t.Errorf("score = %f, want 10.0", score)
	}
	
	// Depth 1: 10 * 0.7 = 7
	score = im.CalculatePropagatedScore(10, 1)
	if score != 7.0 {
		t.Errorf("score = %f, want 7.0", score)
	}
	
	// Depth 2: 10 * 0.7 * 0.7 = 4.9
	score = im.CalculatePropagatedScore(10, 2)
	expected := 10.0 * 0.7 * 0.7
	epsilon := 0.0001
	if score < expected-epsilon || score > expected+epsilon {
		t.Errorf("score = %f, want %f (±%f)", score, expected, epsilon)
	}
}

func TestGetStats(t *testing.T) {
	im := createTestManager(t)
	
	im.AwardTaskCompletion("node-1", "task-1", TaskTypeGeneral, 10, "")
	im.AwardTaskCompletion("node-2", "task-2", TaskTypeGeneral, 20, "")
	
	// 模拟传播
	im.ReceivePropagation("source", 10, 1, "reward")
	
	stats := im.GetStats()
	
	if stats.TotalRewards != 2 {
		t.Errorf("TotalRewards = %d, want 2", stats.TotalRewards)
	}
	
	// 分数被限制为最大值10
	if stats.TotalScore != 20.0 { // 10 + 10 (both capped at 10)
		t.Errorf("TotalScore = %f, want 20.0", stats.TotalScore)
	}
	
	if stats.TotalPropagations != 1 {
		t.Errorf("TotalPropagations = %d, want 1", stats.TotalPropagations)
	}
	
	if stats.AverageRewardScore != 10.0 { // 20 / 2
		t.Errorf("AverageRewardScore = %f, want 10.0", stats.AverageRewardScore)
	}
}

func TestStartStop(t *testing.T) {
	im := createTestManager(t)
	
	im.Start()
	
	// 重复启动
	im.Start()
	
	im.Stop()
	
	// 重复停止
	im.Stop()
}

func TestPersistence(t *testing.T) {
	tmpDir := t.TempDir()
	
	config := &IncentiveConfig{
		NodeID:              "persist-node",
		DataDir:             tmpDir,
		DefaultDecayFactor:  0.7,
		DefaultTolerance:    50.0,
		ToleranceResetPeriod: 24 * time.Hour,
		MinPropagationScore: 0.1,
		MaxPropagationDepth: 5,
		TaskWeights: map[TaskType]*TaskWeightConfig{
			TaskTypeGeneral: {Weight: 1.0, MinScore: 1, MaxScore: 100},
		},
	}
	
	im1, _ := NewIncentiveManager(config)
	im1.AwardTaskCompletion("node", "persist-task", TaskTypeGeneral, 10, "")
	im1.save()
	
	// 重新加载
	im2, _ := NewIncentiveManager(config)
	
	reward, err := im2.GetRewardByTask("persist-task")
	if err != nil {
		t.Fatalf("GetRewardByTask failed: %v", err)
	}
	if reward.NodeID != "node" {
		t.Errorf("NodeID = %s, want node", reward.NodeID)
	}
}

func TestCallbacks(t *testing.T) {
	im := createTestManager(t)
	
	var createdReward *TaskReward
	im.OnRewardCreated = func(reward *TaskReward) {
		createdReward = reward
	}
	
	im.AwardTaskCompletion("node", "callback-task", TaskTypeGeneral, 10, "")
	
	if createdReward == nil {
		t.Error("OnRewardCreated not called")
	}
}

func TestSetDecayFactor(t *testing.T) {
	im := createTestManager(t)
	
	err := im.SetDecayFactor(0.5)
	if err != nil {
		t.Fatalf("SetDecayFactor failed: %v", err)
	}
	
	if im.config.DefaultDecayFactor != 0.5 {
		t.Errorf("DecayFactor = %f, want 0.5", im.config.DefaultDecayFactor)
	}
	
	// 无效值
	err = im.SetDecayFactor(0)
	if err != ErrInvalidDecayFactor {
		t.Errorf("expected ErrInvalidDecayFactor, got %v", err)
	}
	
	err = im.SetDecayFactor(1.0)
	if err != ErrInvalidDecayFactor {
		t.Errorf("expected ErrInvalidDecayFactor, got %v", err)
	}
}

func TestSetDefaultTolerance(t *testing.T) {
	im := createTestManager(t)
	
	im.SetDefaultTolerance(100.0)
	
	if im.config.DefaultTolerance != 100.0 {
		t.Errorf("DefaultTolerance = %f, want 100.0", im.config.DefaultTolerance)
	}
}

func TestSetTaskWeightConfig(t *testing.T) {
	im := createTestManager(t)
	
	im.SetTaskWeightConfig(TaskTypeStorage, 2.0, 5, 50)
	
	wc := im.GetTaskWeightConfig(TaskTypeStorage)
	if wc == nil {
		t.Fatal("expected TaskWeightConfig to exist")
	}
	if wc.Weight != 2.0 {
		t.Errorf("Weight = %f, want 2.0", wc.Weight)
	}
	if wc.MinScore != 5 {
		t.Errorf("MinScore = %f, want 5", wc.MinScore)
	}
	if wc.MaxScore != 50 {
		t.Errorf("MaxScore = %f, want 50", wc.MaxScore)
	}
}

func TestGetPropagationRecords(t *testing.T) {
	im := createTestManager(t)
	
	im.ReceivePropagation("source-1", 10, 1, "reward-1")
	im.ReceivePropagation("source-2", 10, 1, "reward-2")
	
	records := im.GetPropagationRecords(im.config.NodeID)
	if len(records) != 2 {
		t.Errorf("records count = %d, want 2", len(records))
	}
}

func TestGetAllTolerances(t *testing.T) {
	im := createTestManager(t)
	
	im.ReceivePropagation("source-1", 10, 1, "reward-1")
	im.ReceivePropagation("source-2", 10, 1, "reward-2")
	
	tolerances := im.GetAllTolerances()
	if len(tolerances) != 2 {
		t.Errorf("tolerances count = %d, want 2", len(tolerances))
	}
}

func TestClear(t *testing.T) {
	im := createTestManager(t)
	
	im.AwardTaskCompletion("node", "task", TaskTypeGeneral, 10, "")
	im.ReceivePropagation("source", 10, 1, "reward")
	
	im.Clear()
	
	stats := im.GetStats()
	if stats.TotalRewards != 0 {
		t.Errorf("TotalRewards = %d, want 0", stats.TotalRewards)
	}
	if stats.TotalPropagations != 0 {
		t.Errorf("TotalPropagations = %d, want 0", stats.TotalPropagations)
	}
}

func TestConcurrentAccess(t *testing.T) {
	im := createTestManager(t)
	
	var wg sync.WaitGroup
	
	// 并发奖励
	for i := 0; i < 10; i++ {
		wg.Add(1)
		go func(n int) {
			defer wg.Done()
			taskID := time.Now().Format("20060102150405.000000000") + string(rune('A'+n))
			im.AwardTaskCompletion("node", taskID, TaskTypeGeneral, 5, "")
		}(i)
	}
	
	// 并发读取
	for i := 0; i < 10; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			im.GetStats()
			im.GetNodeRewards("node")
		}()
	}
	
	wg.Wait()
}

func TestUpdateReputationCallback(t *testing.T) {
	tmpDir := t.TempDir()
	
	var updatedNodeID string
	var updatedDelta float64
	
	config := &IncentiveConfig{
		NodeID:              "test-node",
		DataDir:             tmpDir,
		DefaultDecayFactor:  0.7,
		DefaultTolerance:    50.0,
		ToleranceResetPeriod: 24 * time.Hour,
		MinPropagationScore: 0.1,
		MaxPropagationDepth: 5,
		TaskWeights: map[TaskType]*TaskWeightConfig{
			TaskTypeGeneral: {Weight: 1.0, MinScore: 1, MaxScore: 100},
		},
		UpdateReputationFunc: func(nodeID string, delta float64) error {
			updatedNodeID = nodeID
			updatedDelta = delta
			return nil
		},
	}
	
	im, _ := NewIncentiveManager(config)
	
	im.AwardTaskCompletion("node-x", "task-x", TaskTypeGeneral, 10, "")
	
	if updatedNodeID != "node-x" {
		t.Errorf("updatedNodeID = %s, want node-x", updatedNodeID)
	}
	if updatedDelta != 10.0 {
		t.Errorf("updatedDelta = %f, want 10.0", updatedDelta)
	}
}
