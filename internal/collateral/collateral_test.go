package collateral

import (
	"testing"
	"time"
)

func TestNewCollateralManager(t *testing.T) {
	cm := NewCollateralManager()
	if cm == nil {
		t.Fatal("expected non-nil CollateralManager")
	}
	if cm.collaterals == nil {
		t.Error("collaterals map not initialized")
	}
}

func TestCreateCollateral(t *testing.T) {
	cm := NewCollateralManager()

	tests := []struct {
		name       string
		owner      string
		colType    string
		purpose    string
		amount     float64
		duration   time.Duration
		wantErr    bool
		errType    error
	}{
		{"valid token", "nodeA", CollateralTypeToken, "guarantee", 100.0, 24 * time.Hour, false, nil},
		{"valid stake", "nodeA", CollateralTypeStake, "service", 50.0, 48 * time.Hour, false, nil},
		{"insufficient amount", "nodeA", CollateralTypeToken, "test", 0.5, 24 * time.Hour, true, ErrInsufficientAmount},
		{"invalid type", "nodeA", "invalid", "test", 100.0, 24 * time.Hour, true, ErrInvalidCollateralType},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			c, err := cm.CreateCollateral(tt.owner, tt.colType, tt.purpose, tt.amount, tt.duration)
			if tt.wantErr {
				if err == nil {
					t.Error("expected error, got nil")
				}
				if tt.errType != nil && err != tt.errType {
					t.Errorf("expected error %v, got %v", tt.errType, err)
				}
			} else {
				if err != nil {
					t.Errorf("unexpected error: %v", err)
				}
				if c == nil {
					t.Error("expected collateral, got nil")
				}
				if c != nil && c.Status != CollateralStatusPending {
					t.Errorf("expected pending status, got %s", c.Status)
				}
			}
		})
	}
}

func TestActivateCollateral(t *testing.T) {
	cm := NewCollateralManager()

	c, _ := cm.CreateCollateral("nodeA", CollateralTypeToken, "test", 100.0, 24*time.Hour)

	// 激活
	err := cm.ActivateCollateral(c.ID)
	if err != nil {
		t.Fatalf("ActivateCollateral failed: %v", err)
	}

	// 检查状态
	updated, _ := cm.GetCollateral(c.ID)
	if updated.Status != CollateralStatusActive {
		t.Errorf("expected active status, got %s", updated.Status)
	}

	// 再次激活应失败
	err = cm.ActivateCollateral(c.ID)
	if err == nil {
		t.Error("expected error when activating already active collateral")
	}

	// 激活不存在的
	err = cm.ActivateCollateral("nonexistent")
	if err != ErrCollateralNotFound {
		t.Errorf("expected ErrCollateralNotFound, got %v", err)
	}
}

func TestLockCollateral(t *testing.T) {
	cm := NewCollateralManager()

	c, _ := cm.CreateCollateral("nodeA", CollateralTypeToken, "test", 100.0, 24*time.Hour)
	cm.ActivateCollateral(c.ID)

	// 锁定
	err := cm.LockCollateral(c.ID, "nodeB", "dispute")
	if err != nil {
		t.Fatalf("LockCollateral failed: %v", err)
	}

	// 检查状态
	updated, _ := cm.GetCollateral(c.ID)
	if updated.Status != CollateralStatusLocked {
		t.Errorf("expected locked status, got %s", updated.Status)
	}
	if updated.Beneficiary != "nodeB" {
		t.Errorf("expected beneficiary nodeB, got %s", updated.Beneficiary)
	}
}

func TestSlashCollateral(t *testing.T) {
	cm := NewCollateralManager()

	c, _ := cm.CreateCollateral("nodeA", CollateralTypeToken, "test", 100.0, 24*time.Hour)
	cm.ActivateCollateral(c.ID)

	// 惩罚
	event, err := cm.SlashCollateral(c.ID, "violation", []string{"evidence1"}, 0.5)
	if err != nil {
		t.Fatalf("SlashCollateral failed: %v", err)
	}

	if event.Amount != 50.0 {
		t.Errorf("expected slash amount 50.0, got %.2f", event.Amount)
	}
	if event.Owner != "nodeA" {
		t.Errorf("expected owner nodeA, got %s", event.Owner)
	}

	// 检查状态
	updated, _ := cm.GetCollateral(c.ID)
	if updated.Status != CollateralStatusSlashed {
		t.Errorf("expected slashed status, got %s", updated.Status)
	}

	// 再次惩罚应失败
	_, err = cm.SlashCollateral(c.ID, "violation2", nil, 0.5)
	if err != ErrAlreadySlashed {
		t.Errorf("expected ErrAlreadySlashed, got %v", err)
	}

	// 检查惩罚历史
	history := cm.GetSlashHistory("nodeA")
	if len(history) != 1 {
		t.Errorf("expected 1 slash event, got %d", len(history))
	}

	totalSlashed := cm.GetTotalSlashed("nodeA")
	if totalSlashed != 50.0 {
		t.Errorf("expected total slashed 50.0, got %.2f", totalSlashed)
	}
}

func TestReturnCollateral(t *testing.T) {
	cm := NewCollateralManager()

	c, _ := cm.CreateCollateral("nodeA", CollateralTypeToken, "test", 100.0, 24*time.Hour)
	cm.ActivateCollateral(c.ID)

	// 归还
	err := cm.ReturnCollateral(c.ID)
	if err != nil {
		t.Fatalf("ReturnCollateral failed: %v", err)
	}

	updated, _ := cm.GetCollateral(c.ID)
	if updated.Status != CollateralStatusReturned {
		t.Errorf("expected returned status, got %s", updated.Status)
	}

	// 锁定的不能归还
	c2, _ := cm.CreateCollateral("nodeB", CollateralTypeToken, "test", 100.0, 24*time.Hour)
	cm.ActivateCollateral(c2.ID)
	cm.LockCollateral(c2.ID, "nodeC", "dispute")
	
	err = cm.ReturnCollateral(c2.ID)
	if err != ErrCollateralLocked {
		t.Errorf("expected ErrCollateralLocked, got %v", err)
	}
}

func TestGetOwnerCollaterals(t *testing.T) {
	cm := NewCollateralManager()

	cm.CreateCollateral("nodeA", CollateralTypeToken, "test1", 100.0, 24*time.Hour)
	cm.CreateCollateral("nodeA", CollateralTypeStake, "test2", 50.0, 24*time.Hour)
	cm.CreateCollateral("nodeB", CollateralTypeToken, "test3", 75.0, 24*time.Hour)

	colsA := cm.GetOwnerCollaterals("nodeA")
	if len(colsA) != 2 {
		t.Errorf("expected 2 collaterals for nodeA, got %d", len(colsA))
	}

	colsB := cm.GetOwnerCollaterals("nodeB")
	if len(colsB) != 1 {
		t.Errorf("expected 1 collateral for nodeB, got %d", len(colsB))
	}
}

func TestGetActiveCollateral(t *testing.T) {
	cm := NewCollateralManager()

	c1, _ := cm.CreateCollateral("nodeA", CollateralTypeToken, "test1", 100.0, 48*time.Hour)
	c2, _ := cm.CreateCollateral("nodeA", CollateralTypeStake, "test2", 50.0, 48*time.Hour)
	cm.ActivateCollateral(c1.ID)
	cm.ActivateCollateral(c2.ID)

	// 第三个保持pending状态
	cm.CreateCollateral("nodeA", CollateralTypeToken, "test3", 25.0, 48*time.Hour)

	active := cm.GetActiveCollateral("nodeA")
	if active != 150.0 {
		t.Errorf("expected active collateral 150.0, got %.2f", active)
	}
}

func TestVerifyCollateral(t *testing.T) {
	cm := NewCollateralManager()

	c, _ := cm.CreateCollateral("nodeA", CollateralTypeToken, "test", 100.0, 24*time.Hour)
	
	// Pending 状态应为无效
	proof, err := cm.VerifyCollateral(c.ID)
	if err != nil {
		t.Fatalf("VerifyCollateral failed: %v", err)
	}
	if proof.IsValid {
		t.Error("expected pending collateral to be invalid")
	}

	// 激活后应为有效
	cm.ActivateCollateral(c.ID)
	proof, _ = cm.VerifyCollateral(c.ID)
	if !proof.IsValid {
		t.Error("expected active collateral to be valid")
	}
	if proof.Owner != "nodeA" {
		t.Errorf("expected owner nodeA, got %s", proof.Owner)
	}
}

func TestCheckRequirement(t *testing.T) {
	cm := NewCollateralManager()

	c, _ := cm.CreateCollateral("nodeA", CollateralTypeToken, "test", 100.0, 48*time.Hour)
	cm.ActivateCollateral(c.ID)

	req := &CollateralRequirement{
		MinAmount:     50.0,
		AcceptedTypes: []string{CollateralTypeToken},
		LockPeriod:    24 * time.Hour,
		Purpose:       "guarantee",
	}

	met, msg := cm.CheckRequirement("nodeA", req)
	if !met {
		t.Errorf("expected requirement met, got: %s", msg)
	}

	// 金额不足
	req2 := &CollateralRequirement{
		MinAmount:     200.0,
		AcceptedTypes: []string{CollateralTypeToken},
		LockPeriod:    24 * time.Hour,
	}
	met, _ = cm.CheckRequirement("nodeA", req2)
	if met {
		t.Error("expected requirement not met for insufficient amount")
	}
}

func TestExpireCollaterals(t *testing.T) {
	cm := NewCollateralManager()

	// 创建一个已经过期的抵押物（需要直接修改）
	c, _ := cm.CreateCollateral("nodeA", CollateralTypeToken, "test", 100.0, -1*time.Hour)
	cm.collaterals[c.ID].Status = CollateralStatusActive // 直接设置为 active

	count := cm.ExpireCollaterals()
	if count != 1 {
		t.Errorf("expected 1 expired, got %d", count)
	}

	updated, _ := cm.GetCollateral(c.ID)
	if updated.Status != CollateralStatusExpired {
		t.Errorf("expected expired status, got %s", updated.Status)
	}
}

func TestGetStats(t *testing.T) {
	cm := NewCollateralManager()

	c1, _ := cm.CreateCollateral("nodeA", CollateralTypeToken, "test1", 100.0, 48*time.Hour)
	c2, _ := cm.CreateCollateral("nodeA", CollateralTypeStake, "test2", 50.0, 48*time.Hour)
	cm.ActivateCollateral(c1.ID)
	cm.ActivateCollateral(c2.ID)
	cm.SlashCollateral(c1.ID, "test", nil, 0.5)

	stats := cm.GetStats()

	if stats["total_collaterals"].(int) != 2 {
		t.Errorf("expected 2 total collaterals, got %v", stats["total_collaterals"])
	}
	if stats["active_count"].(int) != 1 {
		t.Errorf("expected 1 active, got %v", stats["active_count"])
	}
	if stats["slashed_count"].(int) != 1 {
		t.Errorf("expected 1 slashed, got %v", stats["slashed_count"])
	}
}

func TestGuaranteePool_AddGuarantee(t *testing.T) {
	cm := NewCollateralManager()
	gp := NewGuaranteePool(cm)

	// 创建担保人的抵押物
	c, _ := cm.CreateCollateral("guarantor", CollateralTypeToken, "guarantee", 150.0, 48*time.Hour)
	cm.ActivateCollateral(c.ID)

	// 添加担保
	err := gp.AddGuarantee("guarantor", "newNode", c.ID)
	if err != nil {
		t.Fatalf("AddGuarantee failed: %v", err)
	}

	// 检查关系
	guaranteed := gp.GetGuaranteed("guarantor")
	if len(guaranteed) != 1 || guaranteed[0] != "newNode" {
		t.Error("expected newNode in guaranteed list")
	}

	guarantors := gp.GetGuarantors("newNode")
	if len(guarantors) != 1 || guarantors[0] != "guarantor" {
		t.Error("expected guarantor in guarantors list")
	}

	// 自己担保自己
	err = gp.AddGuarantee("nodeA", "nodeA", c.ID)
	if err != ErrSelfCollateral {
		t.Errorf("expected ErrSelfCollateral, got %v", err)
	}
}

func TestGuaranteePool_SlashGuarantor(t *testing.T) {
	cm := NewCollateralManager()
	gp := NewGuaranteePool(cm)

	c, _ := cm.CreateCollateral("guarantor", CollateralTypeToken, "guarantee", 200.0, 48*time.Hour)
	cm.ActivateCollateral(c.ID)
	gp.AddGuarantee("guarantor", "badNode", c.ID)

	// 惩罚担保人
	event, err := gp.SlashGuarantor("guarantor", "badNode", "guaranteed node violated", []string{"evidence"})
	if err != nil {
		t.Fatalf("SlashGuarantor failed: %v", err)
	}

	// 担保人被没收 25% (0.5 * 0.5 = 0.25)
	expectedSlash := 200.0 * SlashRatio * 0.5
	if event.Amount != expectedSlash {
		t.Errorf("expected slash %.2f, got %.2f", expectedSlash, event.Amount)
	}
}
