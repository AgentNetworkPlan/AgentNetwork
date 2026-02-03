package accusation

import (
	"math"
	"os"
	"path/filepath"
	"testing"
	"time"
)

// 测试工具函数
func tempDir(t *testing.T) string {
	dir := filepath.Join(os.TempDir(), "accusation_test_"+time.Now().Format("20060102150405"))
	t.Cleanup(func() {
		os.RemoveAll(dir)
	})
	return dir
}

func TestNewAccusationManager(t *testing.T) {
	t.Run("nil config", func(t *testing.T) {
		_, err := NewAccusationManager(nil)
		if err != ErrNilConfig {
			t.Errorf("expected ErrNilConfig, got %v", err)
		}
	})
	
	t.Run("empty node ID", func(t *testing.T) {
		config := &AccusationConfig{}
		_, err := NewAccusationManager(config)
		if err != ErrEmptyNodeID {
			t.Errorf("expected ErrEmptyNodeID, got %v", err)
		}
	})
	
	t.Run("valid config", func(t *testing.T) {
		config := DefaultAccusationConfig("node1")
		config.DataDir = tempDir(t)
		
		am, err := NewAccusationManager(config)
		if err != nil {
			t.Fatalf("failed to create manager: %v", err)
		}
		if am == nil {
			t.Fatal("manager is nil")
		}
	})
}

func TestDefaultAccusationConfig(t *testing.T) {
	config := DefaultAccusationConfig("node1")
	
	if config.NodeID != "node1" {
		t.Errorf("expected NodeID 'node1', got %s", config.NodeID)
	}
	if config.DecayFactor != 0.7 {
		t.Errorf("expected DecayFactor 0.7, got %f", config.DecayFactor)
	}
	if config.DefaultTolerance != 50.0 {
		t.Errorf("expected DefaultTolerance 50.0, got %f", config.DefaultTolerance)
	}
	if config.BasePenalty != 10.0 {
		t.Errorf("expected BasePenalty 10.0, got %f", config.BasePenalty)
	}
	if config.MaxPropagationDepth != 5 {
		t.Errorf("expected MaxPropagationDepth 5, got %d", config.MaxPropagationDepth)
	}
}

func TestCreateAccusation(t *testing.T) {
	config := DefaultAccusationConfig("accuser1")
	config.DataDir = tempDir(t)
	config.GetReputationFunc = func(nodeID string) float64 {
		return 50.0
	}
	
	am, err := NewAccusationManager(config)
	if err != nil {
		t.Fatalf("failed to create manager: %v", err)
	}
	
	t.Run("empty accused", func(t *testing.T) {
		_, err := am.CreateAccusation("", TypeTaskCheating, "test reason", "")
		if err != ErrEmptyAccused {
			t.Errorf("expected ErrEmptyAccused, got %v", err)
		}
	})
	
	t.Run("self accusation", func(t *testing.T) {
		_, err := am.CreateAccusation("accuser1", TypeTaskCheating, "test reason", "")
		if err != ErrSelfAccusation {
			t.Errorf("expected ErrSelfAccusation, got %v", err)
		}
	})
	
	t.Run("low reputation", func(t *testing.T) {
		config2 := DefaultAccusationConfig("accuser2")
		config2.DataDir = tempDir(t)
		config2.GetReputationFunc = func(nodeID string) float64 {
			return 10.0 // 低于最低要求
		}
		
		am2, _ := NewAccusationManager(config2)
		_, err := am2.CreateAccusation("accused1", TypeTaskCheating, "test reason", "")
		if err != ErrLowReputation {
			t.Errorf("expected ErrLowReputation, got %v", err)
		}
	})
	
	t.Run("valid accusation", func(t *testing.T) {
		acc, err := am.CreateAccusation("accused1", TypeTaskCheating, "test reason", "evidence data")
		if err != nil {
			t.Fatalf("failed to create accusation: %v", err)
		}
		
		if acc.Accuser != "accuser1" {
			t.Errorf("expected accuser 'accuser1', got %s", acc.Accuser)
		}
		if acc.Accused != "accused1" {
			t.Errorf("expected accused 'accused1', got %s", acc.Accused)
		}
		if acc.Type != TypeTaskCheating {
			t.Errorf("expected type TypeTaskCheating, got %s", acc.Type)
		}
		if acc.Reason != "test reason" {
			t.Errorf("expected reason 'test reason', got %s", acc.Reason)
		}
		if acc.Evidence != "evidence data" {
			t.Errorf("expected evidence 'evidence data', got %s", acc.Evidence)
		}
		if acc.Status != StatusPending {
			t.Errorf("expected status Pending, got %s", acc.Status)
		}
		if acc.PropagationDepth != 0 {
			t.Errorf("expected depth 0, got %d", acc.PropagationDepth)
		}
	})
}

func TestReceiveAccusation(t *testing.T) {
	config := DefaultAccusationConfig("receiver1")
	config.DataDir = tempDir(t)
	
	am, _ := NewAccusationManager(config)
	
	t.Run("nil accusation", func(t *testing.T) {
		err := am.ReceiveAccusation(nil, "node2")
		if err == nil {
			t.Error("expected error for nil accusation")
		}
	})
	
	t.Run("expired accusation", func(t *testing.T) {
		acc := &Accusation{
			AccusationID: "exp1",
			Accuser:      "accuser1",
			Accused:      "accused1",
			ExpiresAt:    time.Now().Add(-time.Hour),
		}
		err := am.ReceiveAccusation(acc, "node2")
		if err != ErrAccusationExpired {
			t.Errorf("expected ErrAccusationExpired, got %v", err)
		}
	})
	
	t.Run("valid accusation", func(t *testing.T) {
		acc := &Accusation{
			AccusationID:  "acc1",
			Accuser:       "accuser1",
			Accused:       "accused1",
			Type:          TypeMessageSpam,
			Reason:        "spam reason",
			Timestamp:     time.Now(),
			ExpiresAt:     time.Now().Add(24 * time.Hour),
			Status:        StatusPending,
			BasePenalty:   10.0,
		}
		
		err := am.ReceiveAccusation(acc, "node2")
		if err != nil {
			t.Fatalf("failed to receive accusation: %v", err)
		}
		
		// 检查是否存储
		stored, err := am.GetAccusation("acc1")
		if err != nil {
			t.Fatalf("failed to get accusation: %v", err)
		}
		if stored.PropagationDepth != 1 {
			t.Errorf("expected depth 1, got %d", stored.PropagationDepth)
		}
	})
	
	t.Run("duplicate accusation", func(t *testing.T) {
		acc := &Accusation{
			AccusationID: "acc1",
			Accuser:      "accuser1",
			Accused:      "accused1",
			ExpiresAt:    time.Now().Add(24 * time.Hour),
		}
		err := am.ReceiveAccusation(acc, "node3")
		if err != ErrDuplicateAccusation {
			t.Errorf("expected ErrDuplicateAccusation, got %v", err)
		}
	})
}

func TestAnalyzeAccusation(t *testing.T) {
	config := DefaultAccusationConfig("analyzer1")
	config.DataDir = tempDir(t)
	
	var reputationUpdates []struct {
		nodeID string
		delta  float64
	}
	
	config.UpdateReputationFunc = func(nodeID string, delta float64) error {
		reputationUpdates = append(reputationUpdates, struct {
			nodeID string
			delta  float64
		}{nodeID, delta})
		return nil
	}
	
	am, _ := NewAccusationManager(config)
	
	// 接收一个指责
	acc := &Accusation{
		AccusationID:  "analyze1",
		Accuser:       "accuser1",
		Accused:       "accused1",
		Type:          TypeTaskCheating,
		Reason:        "cheating",
		Timestamp:     time.Now(),
		ExpiresAt:     time.Now().Add(24 * time.Hour),
		Status:        StatusPending,
		BasePenalty:   10.0,
		PropagationDepth: 0,
	}
	am.ReceiveAccusation(acc, "node2")
	
	t.Run("analyze not found", func(t *testing.T) {
		_, err := am.AnalyzeAccusation("notfound", true, "")
		if err != ErrAccusationNotFound {
			t.Errorf("expected ErrAccusationNotFound, got %v", err)
		}
	})
	
	t.Run("accept accusation", func(t *testing.T) {
		reputationUpdates = nil
		
		analysis, err := am.AnalyzeAccusation("analyze1", true, "evidence verified")
		if err != nil {
			t.Fatalf("failed to analyze: %v", err)
		}
		
		if !analysis.Accepted {
			t.Error("expected accepted=true")
		}
		if analysis.Reason != "evidence verified" {
			t.Errorf("expected reason 'evidence verified', got %s", analysis.Reason)
		}
		
		// 检查声誉更新被调用
		if len(reputationUpdates) == 0 {
			t.Error("expected reputation update")
		} else {
			found := false
			for _, u := range reputationUpdates {
				if u.nodeID == "accused1" && u.delta < 0 {
					found = true
					break
				}
			}
			if !found {
				t.Error("expected reputation penalty for accused1")
			}
		}
		
		// 检查状态
		stored, _ := am.GetAccusation("analyze1")
		if stored.Status != StatusVerified {
			t.Errorf("expected status Verified, got %s", stored.Status)
		}
	})
}

func TestTolerance(t *testing.T) {
	config := DefaultAccusationConfig("receiver1")
	config.DataDir = tempDir(t)
	config.DefaultTolerance = 30.0
	
	am, _ := NewAccusationManager(config)
	
	// 设置耐受值
	am.SetTolerance("accuser1", 30.0)
	
	// 接收多个指责，直到超过耐受值
	for i := 0; i < 3; i++ {
		acc := &Accusation{
			AccusationID: "tol" + string(rune('A'+i)),
			Accuser:      "accuser1",
			Accused:      "accused1",
			ExpiresAt:    time.Now().Add(24 * time.Hour),
			BasePenalty:  12.0,
		}
		err := am.ReceiveAccusation(acc, "node2")
		
		if i < 2 {
			if err != nil {
				t.Errorf("iteration %d: unexpected error: %v", i, err)
			}
		} else {
			// 第三个应该超过耐受值
			if err != ErrToleranceExceeded {
				t.Errorf("iteration %d: expected ErrToleranceExceeded, got %v", i, err)
			}
		}
	}
	
	// 检查耐受值记录
	record := am.GetToleranceRecord("accuser1")
	if record == nil {
		t.Fatal("expected tolerance record")
	}
	if record.TotalPenaltyReceived != 24.0 {
		t.Errorf("expected total 24.0, got %f", record.TotalPenaltyReceived)
	}
}

func TestResetTolerance(t *testing.T) {
	config := DefaultAccusationConfig("receiver1")
	config.DataDir = tempDir(t)
	
	am, _ := NewAccusationManager(config)
	
	am.SetTolerance("accuser1", 50.0)
	
	// 接收一些指责
	acc := &Accusation{
		AccusationID: "reset1",
		Accuser:      "accuser1",
		Accused:      "accused1",
		ExpiresAt:    time.Now().Add(24 * time.Hour),
		BasePenalty:  20.0,
	}
	am.ReceiveAccusation(acc, "node2")
	
	record := am.GetToleranceRecord("accuser1")
	if record.RemainingTolerance != 30.0 {
		t.Errorf("expected remaining 30.0, got %f", record.RemainingTolerance)
	}
	
	// 重置
	err := am.ResetTolerance("accuser1")
	if err != nil {
		t.Fatalf("failed to reset tolerance: %v", err)
	}
	
	record = am.GetToleranceRecord("accuser1")
	if record.RemainingTolerance != 50.0 {
		t.Errorf("expected remaining 50.0 after reset, got %f", record.RemainingTolerance)
	}
	if record.TotalPenaltyReceived != 0 {
		t.Errorf("expected total 0 after reset, got %f", record.TotalPenaltyReceived)
	}
}

func TestPropagate(t *testing.T) {
	config := DefaultAccusationConfig("node1")
	config.DataDir = tempDir(t)
	config.GetReputationFunc = func(nodeID string) float64 {
		return 50.0
	}
	config.GetNeighborsFunc = func(nodeID string) []string {
		return []string{"neighbor1", "neighbor2", "neighbor3"}
	}
	
	am, _ := NewAccusationManager(config)
	
	// 创建指责
	acc, _ := am.CreateAccusation("accused1", TypeDataCorruption, "data issue", "")
	
	// 传播
	propagated, err := am.PropagateAccusation(acc.AccusationID)
	if err != nil {
		t.Fatalf("failed to propagate: %v", err)
	}
	
	if len(propagated) != 3 {
		t.Errorf("expected 3 propagations, got %d", len(propagated))
	}
	
	// 检查状态
	stored, _ := am.GetAccusation(acc.AccusationID)
	if stored.Status != StatusDelivered {
		t.Errorf("expected status Delivered, got %s", stored.Status)
	}
}

func TestContinuePropagation(t *testing.T) {
	config := DefaultAccusationConfig("node1")
	config.DataDir = tempDir(t)
	config.MaxPropagationDepth = 5
	config.GetNeighborsFunc = func(nodeID string) []string {
		return []string{"n1", "n2"}
	}
	
	am, _ := NewAccusationManager(config)
	
	// 接收一个深度为0的指责（接收后会变成1）
	acc := &Accusation{
		AccusationID:     "continue1",
		Accuser:          "original",
		Accused:          "accused1",
		ExpiresAt:        time.Now().Add(24 * time.Hour),
		PropagationDepth: 0,
		BasePenalty:      10.0,
	}
	am.ReceiveAccusation(acc, "sender")
	
	// 接收后 depth=1，继续传播
	propagated, err := am.ContinuePropagation("continue1")
	if err != nil {
		t.Fatalf("failed to continue: %v", err)
	}
	if len(propagated) != 2 {
		t.Errorf("expected 2 propagations, got %d", len(propagated))
	}
	
	// 深度达到限制后不再传播
	am.mu.Lock()
	stored := am.accusations["continue1"]
	stored.PropagationDepth = 5
	am.mu.Unlock()
	
	propagated, _ = am.ContinuePropagation("continue1")
	if len(propagated) != 0 {
		t.Errorf("expected 0 propagations at max depth, got %d", len(propagated))
	}
}

func TestCalculatePenalty(t *testing.T) {
	config := DefaultAccusationConfig("node1")
	config.DataDir = tempDir(t)
	config.BasePenalty = 10.0
	config.DecayFactor = 0.7
	
	am, _ := NewAccusationManager(config)
	
	tests := []struct {
		reputation float64
		depth      int
		expected   float64
	}{
		{50.0, 0, 12.5},   // 0.5 + (50/100)*1.5 = 1.25, 10 * 1.25 = 12.5
		{100.0, 0, 20.0},  // 0.5 + 1*1.5 = 2.0, 10 * 2.0 = 20.0
		{0.0, 0, 5.0},     // 0.5 + 0 = 0.5, 10 * 0.5 = 5.0
		{50.0, 1, 8.75},   // 12.5 * 0.7 = 8.75
		{50.0, 2, 6.125},  // 12.5 * 0.7^2 = 6.125
	}
	
	const epsilon = 0.001
	
	for _, tt := range tests {
		penalty := am.CalculatePenalty(tt.reputation, tt.depth)
		if math.Abs(penalty-tt.expected) > epsilon {
			t.Errorf("rep=%.1f, depth=%d: expected %.3f, got %.3f",
				tt.reputation, tt.depth, tt.expected, penalty)
		}
	}
}

func TestGetAccusationsByAccuser(t *testing.T) {
	config := DefaultAccusationConfig("node1")
	config.DataDir = tempDir(t)
	
	am, _ := NewAccusationManager(config)
	
	// 添加指责
	am.mu.Lock()
	am.accusations["a1"] = &Accusation{AccusationID: "a1", Accuser: "accuser1", Accused: "accused1"}
	am.accusations["a2"] = &Accusation{AccusationID: "a2", Accuser: "accuser1", Accused: "accused2"}
	am.accusations["a3"] = &Accusation{AccusationID: "a3", Accuser: "accuser2", Accused: "accused1"}
	am.mu.Unlock()
	
	results := am.GetAccusationsByAccuser("accuser1")
	if len(results) != 2 {
		t.Errorf("expected 2 accusations, got %d", len(results))
	}
	
	results = am.GetAccusationsByAccuser("accuser2")
	if len(results) != 1 {
		t.Errorf("expected 1 accusation, got %d", len(results))
	}
}

func TestGetAccusationsByAccused(t *testing.T) {
	config := DefaultAccusationConfig("node1")
	config.DataDir = tempDir(t)
	
	am, _ := NewAccusationManager(config)
	
	// 添加指责
	am.mu.Lock()
	am.accusations["a1"] = &Accusation{AccusationID: "a1", Accuser: "accuser1", Accused: "accused1"}
	am.accusations["a2"] = &Accusation{AccusationID: "a2", Accuser: "accuser2", Accused: "accused1"}
	am.accusations["a3"] = &Accusation{AccusationID: "a3", Accuser: "accuser1", Accused: "accused2"}
	am.mu.Unlock()
	
	results := am.GetAccusationsByAccused("accused1")
	if len(results) != 2 {
		t.Errorf("expected 2 accusations, got %d", len(results))
	}
	
	results = am.GetAccusationsByAccused("accused2")
	if len(results) != 1 {
		t.Errorf("expected 1 accusation, got %d", len(results))
	}
}

func TestGetPendingAccusations(t *testing.T) {
	config := DefaultAccusationConfig("node1")
	config.DataDir = tempDir(t)
	
	am, _ := NewAccusationManager(config)
	
	now := time.Now()
	
	am.mu.Lock()
	am.accusations["p1"] = &Accusation{AccusationID: "p1", Status: StatusPending, Timestamp: now}
	am.accusations["p2"] = &Accusation{AccusationID: "p2", Status: StatusDelivered, Timestamp: now.Add(-time.Hour)}
	am.accusations["p3"] = &Accusation{AccusationID: "p3", Status: StatusVerified, Timestamp: now}
	am.accusations["p4"] = &Accusation{AccusationID: "p4", Status: StatusPending, Timestamp: now.Add(time.Hour)}
	am.mu.Unlock()
	
	pending := am.GetPendingAccusations()
	if len(pending) != 3 {
		t.Errorf("expected 3 pending, got %d", len(pending))
	}
	
	// 检查排序（最新的在前）
	if pending[0].AccusationID != "p4" {
		t.Errorf("expected p4 first, got %s", pending[0].AccusationID)
	}
}

func TestGetStats(t *testing.T) {
	config := DefaultAccusationConfig("node1")
	config.DataDir = tempDir(t)
	
	am, _ := NewAccusationManager(config)
	
	am.mu.Lock()
	am.accusations["s1"] = &Accusation{AccusationID: "s1", Status: StatusPending, BasePenalty: 10, AccuserCost: 2}
	am.accusations["s2"] = &Accusation{AccusationID: "s2", Status: StatusVerified, BasePenalty: 15, AccuserCost: 3}
	am.accusations["s3"] = &Accusation{AccusationID: "s3", Status: StatusRejected, BasePenalty: 5, AccuserCost: 1}
	am.tolerances["t1"] = &ToleranceRecord{}
	am.tolerances["t2"] = &ToleranceRecord{}
	am.mu.Unlock()
	
	stats := am.GetStats()
	
	if stats.TotalAccusations != 3 {
		t.Errorf("expected 3 total, got %d", stats.TotalAccusations)
	}
	if stats.PendingAccusations != 1 {
		t.Errorf("expected 1 pending, got %d", stats.PendingAccusations)
	}
	if stats.VerifiedAccusations != 1 {
		t.Errorf("expected 1 verified, got %d", stats.VerifiedAccusations)
	}
	if stats.RejectedAccusations != 1 {
		t.Errorf("expected 1 rejected, got %d", stats.RejectedAccusations)
	}
	if stats.TotalPenaltyApplied != 15.0 {
		t.Errorf("expected penalty 15.0, got %f", stats.TotalPenaltyApplied)
	}
	if stats.TotalAccuserCost != 6.0 {
		t.Errorf("expected cost 6.0, got %f", stats.TotalAccuserCost)
	}
	if stats.ActiveTolerances != 2 {
		t.Errorf("expected 2 tolerances, got %d", stats.ActiveTolerances)
	}
}

func TestPersistence(t *testing.T) {
	dir := tempDir(t)
	
	// 创建并保存
	config1 := DefaultAccusationConfig("node1")
	config1.DataDir = dir
	
	am1, _ := NewAccusationManager(config1)
	
	am1.mu.Lock()
	am1.accusations["persist1"] = &Accusation{
		AccusationID: "persist1",
		Accuser:      "a",
		Accused:      "b",
		Status:       StatusVerified,
	}
	am1.tolerances["tol1"] = &ToleranceRecord{
		AccuserNodeID:      "tol1",
		MaxTolerance:       100,
		RemainingTolerance: 80,
	}
	am1.mu.Unlock()
	
	am1.save()
	
	// 重新加载
	config2 := DefaultAccusationConfig("node1")
	config2.DataDir = dir
	
	am2, _ := NewAccusationManager(config2)
	
	// 检查数据恢复
	acc, err := am2.GetAccusation("persist1")
	if err != nil {
		t.Fatalf("failed to get accusation: %v", err)
	}
	if acc.Status != StatusVerified {
		t.Errorf("expected status Verified, got %s", acc.Status)
	}
	
	tol := am2.GetToleranceRecord("tol1")
	if tol == nil {
		t.Fatal("expected tolerance record")
	}
	if tol.MaxTolerance != 100 {
		t.Errorf("expected max 100, got %f", tol.MaxTolerance)
	}
}

func TestSignatureValidation(t *testing.T) {
	config := DefaultAccusationConfig("node1")
	config.DataDir = tempDir(t)
	config.GetReputationFunc = func(nodeID string) float64 {
		return 50.0
	}
	
	signed := false
	config.SignFunc = func(data []byte) (string, error) {
		signed = true
		return "test_signature", nil
	}
	
	am, _ := NewAccusationManager(config)
	
	acc, err := am.CreateAccusation("accused1", TypeTaskCheating, "test", "")
	if err != nil {
		t.Fatalf("failed to create: %v", err)
	}
	
	if !signed {
		t.Error("expected signature function to be called")
	}
	if acc.Signature != "test_signature" {
		t.Errorf("expected signature 'test_signature', got %s", acc.Signature)
	}
}

func TestVerifySignature(t *testing.T) {
	config := DefaultAccusationConfig("receiver1")
	config.DataDir = tempDir(t)
	
	verifyResult := true
	config.VerifyFunc = func(publicKey string, data []byte, signature string) bool {
		return verifyResult
	}
	
	am, _ := NewAccusationManager(config)
	
	t.Run("valid signature", func(t *testing.T) {
		verifyResult = true
		acc := &Accusation{
			AccusationID: "verify1",
			Accuser:      "accuser1",
			Accused:      "accused1",
			Type:         TypeOther,
			Reason:       "test",
			Timestamp:    time.Now(),
			ExpiresAt:    time.Now().Add(24 * time.Hour),
			Signature:    "valid_sig",
		}
		
		err := am.ReceiveAccusation(acc, "node2")
		if err != nil {
			t.Errorf("expected success, got error: %v", err)
		}
	})
	
	t.Run("invalid signature", func(t *testing.T) {
		verifyResult = false
		acc := &Accusation{
			AccusationID: "verify2",
			Accuser:      "accuser2",
			Accused:      "accused2",
			Type:         TypeOther,
			Reason:       "test",
			Timestamp:    time.Now(),
			ExpiresAt:    time.Now().Add(24 * time.Hour),
			Signature:    "invalid_sig",
		}
		
		err := am.ReceiveAccusation(acc, "node2")
		if err != ErrInvalidSignature {
			t.Errorf("expected ErrInvalidSignature, got %v", err)
		}
	})
}

func TestCallbacks(t *testing.T) {
	config := DefaultAccusationConfig("node1")
	config.DataDir = tempDir(t)
	config.GetReputationFunc = func(nodeID string) float64 {
		return 50.0
	}
	
	var createdAcc *Accusation
	var receivedAcc *Accusation
	var verifiedAcc *Accusation
	
	am, _ := NewAccusationManager(config)
	
	am.OnAccusationCreated = func(acc *Accusation) {
		createdAcc = acc
	}
	am.OnAccusationReceived = func(acc *Accusation, from string) {
		receivedAcc = acc
	}
	am.OnAccusationVerified = func(acc *Accusation, analysis *AccusationAnalysis) {
		verifiedAcc = acc
	}
	
	// 测试创建回调
	acc, _ := am.CreateAccusation("accused1", TypeOther, "test", "")
	if createdAcc == nil || createdAcc.AccusationID != acc.AccusationID {
		t.Error("OnAccusationCreated not called correctly")
	}
	
	// 测试接收回调
	am.ReceiveAccusation(&Accusation{
		AccusationID: "cb2",
		Accuser:      "a",
		Accused:      "b",
		ExpiresAt:    time.Now().Add(24 * time.Hour),
	}, "sender")
	if receivedAcc == nil || receivedAcc.AccusationID != "cb2" {
		t.Error("OnAccusationReceived not called correctly")
	}
	
	// 测试验证回调
	am.AnalyzeAccusation("cb2", true, "accepted")
	if verifiedAcc == nil || verifiedAcc.AccusationID != "cb2" {
		t.Error("OnAccusationVerified not called correctly")
	}
}

func TestStartStop(t *testing.T) {
	config := DefaultAccusationConfig("node1")
	config.DataDir = tempDir(t)
	config.CleanupInterval = 100 * time.Millisecond
	
	am, _ := NewAccusationManager(config)
	
	am.Start()
	time.Sleep(50 * time.Millisecond)
	
	// 再次启动不应有问题
	am.Start()
	
	am.Stop()
	time.Sleep(50 * time.Millisecond)
	
	// 再次停止不应有问题
	am.Stop()
}

func TestClear(t *testing.T) {
	config := DefaultAccusationConfig("node1")
	config.DataDir = tempDir(t)
	
	am, _ := NewAccusationManager(config)
	
	am.mu.Lock()
	am.accusations["c1"] = &Accusation{}
	am.tolerances["t1"] = &ToleranceRecord{}
	am.mu.Unlock()
	
	am.Clear()
	
	am.mu.RLock()
	if len(am.accusations) != 0 {
		t.Error("accusations not cleared")
	}
	if len(am.tolerances) != 0 {
		t.Error("tolerances not cleared")
	}
	am.mu.RUnlock()
}

func TestSetters(t *testing.T) {
	config := DefaultAccusationConfig("node1")
	config.DataDir = tempDir(t)
	
	am, _ := NewAccusationManager(config)
	
	am.SetDecayFactor(0.8)
	if am.config.DecayFactor != 0.8 {
		t.Errorf("expected decay 0.8, got %f", am.config.DecayFactor)
	}
	
	am.SetBasePenalty(15.0)
	if am.config.BasePenalty != 15.0 {
		t.Errorf("expected penalty 15.0, got %f", am.config.BasePenalty)
	}
	
	am.SetNaturalDecayAmount(2.0)
	if am.config.NaturalDecayAmount != 2.0 {
		t.Errorf("expected decay amount 2.0, got %f", am.config.NaturalDecayAmount)
	}
}

func TestAccusationTypes(t *testing.T) {
	config := DefaultAccusationConfig("node1")
	config.DataDir = tempDir(t)
	config.GetReputationFunc = func(nodeID string) float64 {
		return 50.0
	}
	
	am, _ := NewAccusationManager(config)
	
	types := []AccusationType{
		TypeTaskCheating,
		TypeMessageSpam,
		TypeServiceDenial,
		TypeDataCorruption,
		TypeProtocolViolation,
		TypeOther,
	}
	
	for i, at := range types {
		accused := "accused" + string(rune('0'+i))
		acc, err := am.CreateAccusation(accused, at, "reason", "")
		if err != nil {
			t.Errorf("failed to create accusation type %s: %v", at, err)
			continue
		}
		if acc.Type != at {
			t.Errorf("expected type %s, got %s", at, acc.Type)
		}
	}
}

func TestGetAnalyses(t *testing.T) {
	config := DefaultAccusationConfig("node1")
	config.DataDir = tempDir(t)
	
	am, _ := NewAccusationManager(config)
	
	// 接收并分析
	am.ReceiveAccusation(&Accusation{
		AccusationID: "ana1",
		Accuser:      "a",
		Accused:      "b",
		ExpiresAt:    time.Now().Add(24 * time.Hour),
		BasePenalty:  10,
	}, "sender")
	
	am.AnalyzeAccusation("ana1", true, "reason1")
	
	analyses := am.GetAnalyses("ana1")
	if len(analyses) != 1 {
		t.Errorf("expected 1 analysis, got %d", len(analyses))
	}
	
	// 不存在的指责
	analyses = am.GetAnalyses("notfound")
	if len(analyses) != 0 {
		t.Errorf("expected 0 analyses, got %d", len(analyses))
	}
}

func TestGetAllTolerances(t *testing.T) {
	config := DefaultAccusationConfig("node1")
	config.DataDir = tempDir(t)
	
	am, _ := NewAccusationManager(config)
	
	am.SetTolerance("t1", 50)
	am.SetTolerance("t2", 60)
	am.SetTolerance("t3", 70)
	
	tolerances := am.GetAllTolerances()
	if len(tolerances) != 3 {
		t.Errorf("expected 3 tolerances, got %d", len(tolerances))
	}
}

func TestPropagateNotFound(t *testing.T) {
	config := DefaultAccusationConfig("node1")
	config.DataDir = tempDir(t)
	
	am, _ := NewAccusationManager(config)
	
	_, err := am.PropagateAccusation("notfound")
	if err != ErrAccusationNotFound {
		t.Errorf("expected ErrAccusationNotFound, got %v", err)
	}
}

func TestResetToleranceNotFound(t *testing.T) {
	config := DefaultAccusationConfig("node1")
	config.DataDir = tempDir(t)
	
	am, _ := NewAccusationManager(config)
	
	err := am.ResetTolerance("notfound")
	if err == nil {
		t.Error("expected error for not found")
	}
}

func TestContinuePropagationNotFound(t *testing.T) {
	config := DefaultAccusationConfig("node1")
	config.DataDir = tempDir(t)
	
	am, _ := NewAccusationManager(config)
	
	_, err := am.ContinuePropagation("notfound")
	if err != ErrAccusationNotFound {
		t.Errorf("expected ErrAccusationNotFound, got %v", err)
	}
}
