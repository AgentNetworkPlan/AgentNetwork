package escrow

import (
	"os"
	"testing"
)

func TestNewEscrowManager(t *testing.T) {
	config := &EscrowConfig{
		DataDir:    t.TempDir(),
		MinDeposit: 0.1,
		MaxDeposit: 1000.0,
	}

	em := NewEscrowManager(config)
	if em == nil {
		t.Fatal("NewEscrowManager returned nil")
	}
}

func TestCreateEscrow(t *testing.T) {
	em := NewEscrowManager(&EscrowConfig{
		DataDir:    t.TempDir(),
		MinDeposit: 0.1,
		MaxDeposit: 1000.0,
	})

	requiredDeposits := map[string]float64{
		"requester": 10.0,
		"executor":  5.0,
	}

	escrow, err := em.CreateEscrow("task1", requiredDeposits)
	if err != nil {
		t.Fatalf("CreateEscrow failed: %v", err)
	}

	if escrow.ID == "" {
		t.Error("Escrow ID should be generated")
	}

	if escrow.Status != EscrowPending {
		t.Errorf("Status should be pending, got %s", escrow.Status)
	}

	if len(escrow.Participants) != 2 {
		t.Errorf("Should have 2 participants, got %d", len(escrow.Participants))
	}
}

func TestEscrowDeposit(t *testing.T) {
	em := NewEscrowManager(&EscrowConfig{
		DataDir:    t.TempDir(),
		MinDeposit: 0.1,
		MaxDeposit: 1000.0,
	})

	escrow, _ := em.CreateEscrow("task1", map[string]float64{
		"requester": 10.0,
		"executor":  5.0,
	})

	// Deposit from requester
	err := em.Deposit(escrow.ID, "requester", 10.0, "sig1")
	if err != nil {
		t.Errorf("Deposit failed: %v", err)
	}

	// Status should still be pending (waiting for executor)
	updatedEscrow, _ := em.GetEscrow(escrow.ID)
	if updatedEscrow.Status != EscrowPending {
		t.Errorf("Status should still be pending, got %s", updatedEscrow.Status)
	}

	// Deposit from executor
	err = em.Deposit(escrow.ID, "executor", 5.0, "sig2")
	if err != nil {
		t.Errorf("Deposit failed: %v", err)
	}

	// Status should now be locked
	updatedEscrow, _ = em.GetEscrow(escrow.ID)
	if updatedEscrow.Status != EscrowLocked {
		t.Errorf("Status should be locked, got %s", updatedEscrow.Status)
	}

	if updatedEscrow.TotalAmount != 15.0 {
		t.Errorf("Total amount should be 15.0, got %.1f", updatedEscrow.TotalAmount)
	}
}

func TestEscrowDepositValidation(t *testing.T) {
	em := NewEscrowManager(&EscrowConfig{
		DataDir:    t.TempDir(),
		MinDeposit: 0.1,
		MaxDeposit: 1000.0,
	})

	escrow, _ := em.CreateEscrow("task1", map[string]float64{
		"requester": 10.0,
		"executor":  5.0,  // 添加第二参与者，避免单人存款后直接锁定
	})

	// Insufficient funds
	err := em.Deposit(escrow.ID, "requester", 5.0, "sig")
	if err == nil {
		t.Error("Should fail with insufficient funds")
	}

	// Unauthorized deposit
	err = em.Deposit(escrow.ID, "stranger", 10.0, "sig")
	if err != ErrUnauthorized {
		t.Errorf("Expected unauthorized error, got %v", err)
	}

	// Double deposit
	em.Deposit(escrow.ID, "requester", 10.0, "sig1")
	err = em.Deposit(escrow.ID, "requester", 10.0, "sig2")
	if err != ErrAlreadyDeposited {
		t.Errorf("Expected already deposited error, got %v", err)
	}
}

func TestEscrowRelease(t *testing.T) {
	em := NewEscrowManager(&EscrowConfig{
		DataDir:    t.TempDir(),
		MinDeposit: 0.1,
		MaxDeposit: 1000.0,
	})

	escrow, _ := em.CreateEscrow("task1", map[string]float64{
		"requester": 10.0,
		"executor":  5.0,
	})

	// Deposit both
	em.Deposit(escrow.ID, "requester", 10.0, "sig1")
	em.Deposit(escrow.ID, "executor", 5.0, "sig2")

	// Release to executor
	signatures := map[string]string{
		"requester": "unlock_sig1",
	}

	err := em.Release(escrow.ID, "executor", 15.0, signatures)
	if err != nil {
		t.Errorf("Release failed: %v", err)
	}

	updatedEscrow, _ := em.GetEscrow(escrow.ID)
	if updatedEscrow.Status != EscrowReleased {
		t.Errorf("Status should be released, got %s", updatedEscrow.Status)
	}

	if updatedEscrow.ReleasedTo != "executor" {
		t.Errorf("Released to should be executor, got %s", updatedEscrow.ReleasedTo)
	}

	if updatedEscrow.ReleasedAmount != 15.0 {
		t.Errorf("Released amount should be 15.0, got %.1f", updatedEscrow.ReleasedAmount)
	}
}

func TestEscrowRefund(t *testing.T) {
	em := NewEscrowManager(&EscrowConfig{
		DataDir:    t.TempDir(),
		MinDeposit: 0.1,
		MaxDeposit: 1000.0,
	})

	escrow, _ := em.CreateEscrow("task1", map[string]float64{
		"requester": 10.0,
		"executor":  5.0,
	})

	em.Deposit(escrow.ID, "requester", 10.0, "sig1")
	em.Deposit(escrow.ID, "executor", 5.0, "sig2")

	signatures := map[string]string{
		"requester": "refund_sig",
	}

	err := em.Refund(escrow.ID, signatures)
	if err != nil {
		t.Errorf("Refund failed: %v", err)
	}

	updatedEscrow, _ := em.GetEscrow(escrow.ID)
	if updatedEscrow.Status != EscrowRefunded {
		t.Errorf("Status should be refunded, got %s", updatedEscrow.Status)
	}
}

func TestEscrowDispute(t *testing.T) {
	em := NewEscrowManager(&EscrowConfig{
		DataDir:    t.TempDir(),
		MinDeposit: 0.1,
		MaxDeposit: 1000.0,
	})

	escrow, _ := em.CreateEscrow("task1", map[string]float64{
		"requester": 10.0,
		"executor":  5.0,
	})

	em.Deposit(escrow.ID, "requester", 10.0, "sig1")
	em.Deposit(escrow.ID, "executor", 5.0, "sig2")

	// Stranger cannot dispute (test before any dispute)
	escrow2, _ := em.CreateEscrow("task2", map[string]float64{
		"requester": 10.0,
		"executor":  5.0,
	})
	em.Deposit(escrow2.ID, "requester", 10.0, "sig3")
	em.Deposit(escrow2.ID, "executor", 5.0, "sig4")
	err := em.Dispute(escrow2.ID, "stranger", "Test")
	if err != ErrUnauthorized {
		t.Errorf("Stranger should not be able to dispute: %v", err)
	}

	// Dispute by requester
	err = em.Dispute(escrow.ID, "requester", "Quality issue")
	if err != nil {
		t.Errorf("Dispute failed: %v", err)
	}

	updatedEscrow, _ := em.GetEscrow(escrow.ID)
	if updatedEscrow.Status != EscrowDisputed {
		t.Errorf("Status should be disputed, got %s", updatedEscrow.Status)
	}

	if updatedEscrow.DisputeReason != "Quality issue" {
		t.Errorf("Dispute reason should be 'Quality issue', got '%s'", updatedEscrow.DisputeReason)
	}
}

func TestEscrowResolveDispute(t *testing.T) {
	em := NewEscrowManager(&EscrowConfig{
		DataDir:           t.TempDir(),
		MinDeposit:        0.1,
		MaxDeposit:        1000.0,
		MinArbitratorSigs: 2, // Task44: 需要2个仲裁签名
	})

	escrow, _ := em.CreateEscrow("task1", map[string]float64{
		"requester": 10.0,
		"executor":  5.0,
	})

	em.Deposit(escrow.ID, "requester", 10.0, "sig1")
	em.Deposit(escrow.ID, "executor", 5.0, "sig2")
	em.Dispute(escrow.ID, "requester", "Issue")

	// Task44: Resolve dispute with multiple arbitrator signatures
	arbitratorSigs := map[string]string{
		"arb1": "arbitrator_sig_1",
		"arb2": "arbitrator_sig_2",
	}
	err := em.ResolveDispute(escrow.ID, "executor", 10.0, arbitratorSigs)
	if err != nil {
		t.Errorf("ResolveDispute failed: %v", err)
	}

	updatedEscrow, _ := em.GetEscrow(escrow.ID)
	if updatedEscrow.Status != EscrowReleased {
		t.Errorf("Status should be released, got %s", updatedEscrow.Status)
	}

	if updatedEscrow.ReleaseCondition != "dispute_resolution_multisig" {
		t.Errorf("Release condition should be dispute_resolution_multisig, got %s", updatedEscrow.ReleaseCondition)
	}
}

func TestEscrowForfeit(t *testing.T) {
	em := NewEscrowManager(&EscrowConfig{
		DataDir:    t.TempDir(),
		MinDeposit: 0.1,
		MaxDeposit: 1000.0,
	})

	escrow, _ := em.CreateEscrow("task1", map[string]float64{
		"requester": 10.0,
		"executor":  5.0,
	})

	em.Deposit(escrow.ID, "requester", 10.0, "sig1")
	em.Deposit(escrow.ID, "executor", 5.0, "sig2")

	// Forfeit violator's deposit
	err := em.Forfeit(escrow.ID, "executor", "Violation detected")
	if err != nil {
		t.Errorf("Forfeit failed: %v", err)
	}

	updatedEscrow, _ := em.GetEscrow(escrow.ID)
	if updatedEscrow.Status != EscrowForfeited {
		t.Errorf("Status should be forfeited, got %s", updatedEscrow.Status)
	}

	if updatedEscrow.ReleasedAmount != 5.0 {
		t.Errorf("Forfeited amount should be 5.0 (executor's deposit), got %.1f", updatedEscrow.ReleasedAmount)
	}
}

func TestEscrowQueries(t *testing.T) {
	em := NewEscrowManager(&EscrowConfig{
		DataDir:    t.TempDir(),
		MinDeposit: 0.1,
		MaxDeposit: 1000.0,
	})

	// Create multiple escrows
	em.CreateEscrow("task1", map[string]float64{"node1": 10.0})
	em.CreateEscrow("task2", map[string]float64{"node1": 5.0, "node2": 5.0})

	// Query by node
	escrows := em.GetEscrowsByNode("node1")
	if len(escrows) != 2 {
		t.Errorf("Expected 2 escrows for node1, got %d", len(escrows))
	}

	escrows = em.GetEscrowsByNode("node2")
	if len(escrows) != 1 {
		t.Errorf("Expected 1 escrow for node2, got %d", len(escrows))
	}

	// Query by task
	escrow, err := em.GetEscrowByTask("task1")
	if err != nil {
		t.Errorf("GetEscrowByTask failed: %v", err)
	}
	if escrow.TaskID != "task1" {
		t.Errorf("Wrong task ID: %s", escrow.TaskID)
	}

	// Query by status
	pendingEscrows := em.GetEscrowsByStatus(EscrowPending)
	if len(pendingEscrows) != 2 {
		t.Errorf("Expected 2 pending escrows, got %d", len(pendingEscrows))
	}
}

func TestGetLockedAmount(t *testing.T) {
	em := NewEscrowManager(&EscrowConfig{
		DataDir:    t.TempDir(),
		MinDeposit: 0.1,
		MaxDeposit: 1000.0,
	})

	escrow, _ := em.CreateEscrow("task1", map[string]float64{"node1": 10.0})
	em.Deposit(escrow.ID, "node1", 10.0, "sig")

	locked := em.GetLockedAmount("node1")
	if locked != 10.0 {
		t.Errorf("Expected 10.0 locked, got %.1f", locked)
	}

	// No locked amount for stranger
	locked = em.GetLockedAmount("stranger")
	if locked != 0 {
		t.Errorf("Expected 0 locked for stranger, got %.1f", locked)
	}
}

func TestEscrowStatistics(t *testing.T) {
	em := NewEscrowManager(&EscrowConfig{
		DataDir:    t.TempDir(),
		MinDeposit: 0.1,
		MaxDeposit: 1000.0,
	})

	escrow1, _ := em.CreateEscrow("task1", map[string]float64{"node1": 10.0})
	em.Deposit(escrow1.ID, "node1", 10.0, "sig1")

	escrow2, _ := em.CreateEscrow("task2", map[string]float64{"node2": 5.0})
	em.Deposit(escrow2.ID, "node2", 5.0, "sig2")
	em.Release(escrow2.ID, "node2", 5.0, map[string]string{"node2": "unlock"})

	stats := em.GetStatistics()
	if stats.TotalEscrows != 2 {
		t.Errorf("Expected 2 escrows, got %d", stats.TotalEscrows)
	}

	if stats.TotalLocked != 10.0 {
		t.Errorf("Expected 10.0 locked, got %.1f", stats.TotalLocked)
	}

	if stats.TotalReleased != 5.0 {
		t.Errorf("Expected 5.0 released, got %.1f", stats.TotalReleased)
	}
}

func TestEscrowPersistence(t *testing.T) {
	tempDir := t.TempDir()
	config := &EscrowConfig{
		DataDir:    tempDir,
		MinDeposit: 0.1,
		MaxDeposit: 1000.0,
	}

	// Create escrow
	em1 := NewEscrowManager(config)
	escrow, _ := em1.CreateEscrow("task1", map[string]float64{"node1": 10.0})
	escrowID := escrow.ID

	// Load in new manager
	em2 := NewEscrowManager(config)
	loadedEscrow, err := em2.GetEscrow(escrowID)
	if err != nil {
		t.Fatalf("Failed to load escrow: %v", err)
	}

	if loadedEscrow.TaskID != "task1" {
		t.Errorf("Expected task ID 'task1', got '%s'", loadedEscrow.TaskID)
	}

	os.RemoveAll(tempDir)
}

func TestEscrowDepositAmountLimits(t *testing.T) {
	em := NewEscrowManager(&EscrowConfig{
		DataDir:    t.TempDir(),
		MinDeposit: 1.0,
		MaxDeposit: 100.0,
	})

	// Below minimum
	_, err := em.CreateEscrow("task1", map[string]float64{"node1": 0.5})
	if err == nil {
		t.Error("Should fail with deposit below minimum")
	}

	// Above maximum
	_, err = em.CreateEscrow("task2", map[string]float64{"node1": 200.0})
	if err == nil {
		t.Error("Should fail with deposit above maximum")
	}

	// Within limits
	_, err = em.CreateEscrow("task3", map[string]float64{"node1": 50.0})
	if err != nil {
		t.Errorf("Should succeed with valid deposit: %v", err)
	}
}
