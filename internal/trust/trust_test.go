package trust

import (
	"testing"
	"time"
)

func TestNewTrustNetwork(t *testing.T) {
	tn := NewTrustNetwork()
	if tn == nil {
		t.Fatal("expected non-nil TrustNetwork")
	}
	if tn.relations == nil {
		t.Error("relations map not initialized")
	}
}

func TestSetDirectTrust(t *testing.T) {
	tn := NewTrustNetwork()

	tests := []struct {
		name    string
		from    string
		to      string
		trust   float64
		wantErr error
	}{
		{"valid positive trust", "A", "B", 0.5, nil},
		{"valid negative trust", "A", "C", -0.3, nil},
		{"max trust", "A", "D", TrustMax, nil},
		{"min trust", "A", "E", TrustMin, nil},
		{"self trust", "A", "A", 0.5, ErrSelfTrust},
		{"trust too high", "A", "F", 1.5, ErrInvalidTrust},
		{"trust too low", "A", "G", -1.5, ErrInvalidTrust},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tn.SetDirectTrust(tt.from, tt.to, tt.trust, "test")
			if err != tt.wantErr {
				t.Errorf("SetDirectTrust() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func TestGetDirectTrust(t *testing.T) {
	tn := NewTrustNetwork()

	// 设置信任
	tn.SetDirectTrust("A", "B", 0.7, "test")

	// 获取存在的信任
	trust, exists := tn.GetDirectTrust("A", "B")
	if !exists {
		t.Error("expected trust to exist")
	}
	if trust != 0.7 {
		t.Errorf("expected trust 0.7, got %v", trust)
	}

	// 获取不存在的信任
	trust, exists = tn.GetDirectTrust("A", "C")
	if exists {
		t.Error("expected trust not to exist")
	}
	if trust != TrustNeutral {
		t.Errorf("expected neutral trust, got %v", trust)
	}
}

func TestUpdateTrustFromInteraction(t *testing.T) {
	tn := NewTrustNetwork()

	// 初始交互
	err := tn.UpdateTrustFromInteraction("A", "B", InteractionSuccess, "success1")
	if err != nil {
		t.Fatalf("UpdateTrustFromInteraction failed: %v", err)
	}

	trust, _ := tn.GetDirectTrust("A", "B")
	expected := TrustInitial + TrustGainPerSuccess
	// 允许浮点数精度误差
	if diff := trust - expected; diff < -0.001 || diff > 0.001 {
		t.Errorf("expected trust %.3f after success, got %.3f", expected, trust)
	}

	// 失败交互
	err = tn.UpdateTrustFromInteraction("A", "B", InteractionFailure, "failure1")
	if err != nil {
		t.Fatalf("UpdateTrustFromInteraction failed: %v", err)
	}

	trust, _ = tn.GetDirectTrust("A", "B")
	expected = expected - TrustLossPerFailure
	// 允许浮点数精度误差
	if diff := trust - expected; diff < -0.001 || diff > 0.001 {
		t.Errorf("expected trust %.3f after failure, got %.3f", expected, trust)
	}
}

func TestCalculatePropagatedTrust(t *testing.T) {
	tn := NewTrustNetwork()

	// 建立信任链 A -> B -> C
	tn.SetDirectTrust("A", "B", 0.8, "")
	tn.SetDirectTrust("B", "C", 0.6, "")

	// 直接信任
	trust, path := tn.CalculatePropagatedTrust("A", "B")
	if trust != 0.8 {
		t.Errorf("expected direct trust 0.8, got %.3f", trust)
	}
	if !path.IsDirectPath {
		t.Error("expected direct path")
	}

	// 传播信任 A -> B -> C
	trust, path = tn.CalculatePropagatedTrust("A", "C")
	if path == nil {
		t.Fatal("expected trust path, got nil")
	}
	// 传播信任 = 0.8 * 0.6 * 0.5 (衰减) = 0.24
	expectedTrust := 0.8 * 0.6 * PropagationDecay
	if trust != expectedTrust {
		t.Errorf("expected propagated trust %.3f, got %.3f", expectedTrust, trust)
	}
	if path.PathLength != 2 {
		t.Errorf("expected path length 2, got %d", path.PathLength)
	}
}

func TestAddGuarantee(t *testing.T) {
	tn := NewTrustNetwork()

	err := tn.AddGuarantee("G", "A")
	if err != nil {
		t.Fatalf("AddGuarantee failed: %v", err)
	}

	// 检查自动建立的信任
	trust, exists := tn.GetDirectTrust("G", "A")
	if !exists {
		t.Error("expected trust relation from guarantee")
	}
	if trust != TrustInitial+DirectInteractionBonus {
		t.Errorf("expected trust %.3f, got %.3f", TrustInitial+DirectInteractionBonus, trust)
	}

	// 测试自担保
	err = tn.AddGuarantee("A", "A")
	if err != ErrSelfTrust {
		t.Errorf("expected ErrSelfTrust, got %v", err)
	}
}

func TestAddWitness(t *testing.T) {
	tn := NewTrustNetwork()

	err := tn.AddWitness("W", "A")
	if err != nil {
		t.Fatalf("AddWitness failed: %v", err)
	}

	// 检查自动建立的信任
	trust, exists := tn.GetDirectTrust("W", "A")
	if !exists {
		t.Error("expected trust relation from witness")
	}
	if trust != TrustInitial {
		t.Errorf("expected trust %.3f, got %.3f", TrustInitial, trust)
	}
}

func TestCalculateCompositeTrust(t *testing.T) {
	tn := NewTrustNetwork()

	// 设置直接信任
	tn.SetDirectTrust("A", "B", 0.6, "")
	// 设置担保
	tn.AddGuarantee("G", "B")
	tn.SetDirectTrust("A", "G", 0.8, "")
	// 设置见证
	tn.AddWitness("W", "B")
	tn.SetDirectTrust("A", "W", 0.5, "")
	// 设置声誉
	tn.SetReputation("B", 700)

	trust := tn.CalculateCompositeTrust("A", "B")
	if trust <= 0 || trust > TrustMax {
		t.Errorf("unexpected composite trust: %.3f", trust)
	}
}

func TestApplyDailyDecay(t *testing.T) {
	tn := NewTrustNetwork()

	tn.SetDirectTrust("A", "B", 0.5, "")
	tn.SetDirectTrust("A", "C", -0.3, "")

	updatedCount := tn.ApplyDailyDecay()
	if updatedCount != 2 {
		t.Errorf("expected 2 updates, got %d", updatedCount)
	}

	trustB, _ := tn.GetDirectTrust("A", "B")
	if trustB >= 0.5 {
		t.Error("expected positive trust to decay")
	}

	trustC, _ := tn.GetDirectTrust("A", "C")
	if trustC <= -0.3 {
		t.Error("expected negative trust to decay toward neutral")
	}
}

func TestGetTrustRelations(t *testing.T) {
	tn := NewTrustNetwork()

	tn.SetDirectTrust("A", "B", 0.5, "")
	tn.SetDirectTrust("A", "C", 0.3, "")

	relations := tn.GetTrustRelations("A")
	if len(relations) != 2 {
		t.Errorf("expected 2 relations, got %d", len(relations))
	}
}

func TestGetTrustedBy(t *testing.T) {
	tn := NewTrustNetwork()

	tn.SetDirectTrust("A", "X", 0.5, "")
	tn.SetDirectTrust("B", "X", 0.7, "")
	tn.SetDirectTrust("C", "X", 0.1, "")

	trusters := tn.GetTrustedBy("X", 0.5)
	if len(trusters) != 2 { // A 和 B
		t.Errorf("expected 2 trusters with trust >= 0.5, got %d", len(trusters))
	}
}

func TestGetNetworkStats(t *testing.T) {
	tn := NewTrustNetwork()

	tn.SetDirectTrust("A", "B", 0.5, "")
	tn.SetDirectTrust("A", "C", -0.3, "")
	tn.AddGuarantee("G", "A")

	stats := tn.GetNetworkStats()

	if stats["total_relations"].(int) != 3 {
		t.Errorf("expected 3 relations, got %v", stats["total_relations"])
	}
	if stats["total_guarantees"].(int) != 1 {
		t.Errorf("expected 1 guarantee, got %v", stats["total_guarantees"])
	}
}

func TestExportTrustGraph(t *testing.T) {
	tn := NewTrustNetwork()

	tn.SetDirectTrust("A", "B", 0.5, "")
	tn.AddGuarantee("G", "A")

	edges := tn.ExportTrustGraph()
	if len(edges) < 2 {
		t.Errorf("expected at least 2 edges, got %d", len(edges))
	}
}

func TestTrustPathString(t *testing.T) {
	path := &TrustPath{
		Nodes:        []string{"A", "B", "C"},
		TrustValues:  []float64{0.8, 0.6},
		FinalTrust:   0.24,
		PathLength:   2,
		Confidence:   0.5,
		ComputedAt:   time.Now(),
		IsDirectPath: false,
	}

	str := path.String()
	if str == "" {
		t.Error("expected non-empty string")
	}

	// nil path
	var nilPath *TrustPath
	if nilPath.String() != "no path" {
		t.Error("expected 'no path' for nil")
	}
}

func TestTrustUpdate_EvidenceAccumulation(t *testing.T) {
	tn := NewTrustNetwork()

	// 多次更新积累证据
	for i := 0; i < 5; i++ {
		tn.SetDirectTrust("A", "B", 0.5, "evidence")
	}

	rel := tn.relations["A"]["B"]
	if len(rel.Evidence) != 5 {
		t.Errorf("expected 5 evidence entries, got %d", len(rel.Evidence))
	}
}
