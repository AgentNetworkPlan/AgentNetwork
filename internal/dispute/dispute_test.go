package dispute

import (
	"os"
	"testing"
	"time"
)

func TestNewDisputeManager(t *testing.T) {
	config := &DisputeConfig{
		DataDir:          t.TempDir(),
		AutoResolveRules: true,
	}

	dm := NewDisputeManager(config)
	if dm == nil {
		t.Fatal("NewDisputeManager returned nil")
	}
}

func TestCreateDispute(t *testing.T) {
	dm := NewDisputeManager(&DisputeConfig{
		DataDir:           t.TempDir(),
		ExpirationPeriod:  7 * 24 * time.Hour,
		MinEvidenceCount:  1,
		MinVotesRequired:  3,
	})

	dispute, err := dm.CreateDispute(
		"task1",
		"complainant1",
		"defendant1",
		DisputeNonDelivery,
		"Did not deliver the work",
		100.0,
	)

	if err != nil {
		t.Fatalf("CreateDispute failed: %v", err)
	}

	if dispute.ID == "" {
		t.Error("Dispute ID should be generated")
	}

	if dispute.Status != DisputePending {
		t.Errorf("Status should be pending, got %s", dispute.Status)
	}

	if dispute.Type != DisputeNonDelivery {
		t.Errorf("Type should be non_delivery, got %s", dispute.Type)
	}

	// Duplicate dispute for same task
	_, err = dm.CreateDispute(
		"task1",
		"complainant1",
		"defendant1",
		DisputeNonDelivery,
		"Another dispute",
		50.0,
	)
	if err == nil {
		t.Error("Should not allow duplicate dispute for same task")
	}
}

func TestSubmitEvidence(t *testing.T) {
	dm := NewDisputeManager(&DisputeConfig{
		DataDir: t.TempDir(),
	})

	dispute, _ := dm.CreateDispute(
		"task1",
		"complainant1",
		"defendant1",
		DisputeQualityIssue,
		"Quality below standard",
		50.0,
	)

	// Submit evidence from complainant
	err := dm.SubmitEvidence(
		dispute.ID,
		"complainant1",
		"text",
		"The delivered work had multiple errors",
		"evidence_hash",
	)
	if err != nil {
		t.Errorf("SubmitEvidence failed: %v", err)
	}

	// Verify evidence was added
	updatedDispute, _ := dm.GetDispute(dispute.ID)
	if len(updatedDispute.Evidence) != 1 {
		t.Errorf("Expected 1 evidence, got %d", len(updatedDispute.Evidence))
	}

	// Submit evidence from defendant
	err = dm.SubmitEvidence(
		dispute.ID,
		"defendant1",
		"hash",
		"delivery_proof_hash",
		"proof_hash",
	)
	if err != nil {
		t.Errorf("SubmitEvidence from defendant failed: %v", err)
	}

	// Stranger cannot submit evidence
	err = dm.SubmitEvidence(
		dispute.ID,
		"stranger",
		"text",
		"Some content",
		"hash",
	)
	if err != ErrUnauthorized {
		t.Errorf("Expected unauthorized error, got %v", err)
	}
}

func TestStartReview(t *testing.T) {
	dm := NewDisputeManager(&DisputeConfig{
		DataDir:          t.TempDir(),
		MinEvidenceCount: 1,
	})

	dispute, _ := dm.CreateDispute(
		"task1",
		"complainant1",
		"defendant1",
		DisputeNonDelivery,
		"Issue",
		100.0,
	)

	// Cannot start review without evidence
	err := dm.StartReview(dispute.ID)
	if err == nil {
		t.Error("Should not start review without evidence")
	}

	// Add evidence
	dm.SubmitEvidence(dispute.ID, "complainant1", "text", "Evidence", "hash")

	// Now can start review
	err = dm.StartReview(dispute.ID)
	if err != nil {
		t.Errorf("StartReview failed: %v", err)
	}

	updatedDispute, _ := dm.GetDispute(dispute.ID)
	if updatedDispute.Status != DisputeInReview {
		t.Errorf("Status should be in_review, got %s", updatedDispute.Status)
	}
}

func TestAutoResolve(t *testing.T) {
	dm := NewDisputeManager(&DisputeConfig{
		DataDir:          t.TempDir(),
		AutoResolveRules: true,
		MinEvidenceCount: 1,
	})

	// Create non-delivery dispute
	dispute, _ := dm.CreateDispute(
		"task1",
		"complainant1",
		"defendant1",
		DisputeNonDelivery,
		"Never received the work",
		100.0,
	)

	// Submit evidence (no delivery proof from defendant)
	dm.SubmitEvidence(dispute.ID, "complainant1", "text", "I waited but nothing was delivered", "hash")
	dm.StartReview(dispute.ID)

	// Try auto-resolve
	resolution, err := dm.TryAutoResolve(dispute.ID)
	if err != nil {
		t.Errorf("TryAutoResolve failed: %v", err)
	}

	if resolution == nil {
		t.Fatal("Resolution should not be nil")
	}

	if resolution.Winner != "complainant1" {
		t.Errorf("Winner should be complainant, got %s", resolution.Winner)
	}

	updatedDispute, _ := dm.GetDispute(dispute.ID)
	if updatedDispute.Status != DisputeResolved {
		t.Errorf("Status should be resolved, got %s", updatedDispute.Status)
	}

	if updatedDispute.ResolutionType != ResolutionAutomatic {
		t.Errorf("Resolution type should be automatic, got %s", updatedDispute.ResolutionType)
	}
}

func TestStartArbitration(t *testing.T) {
	dm := NewDisputeManager(&DisputeConfig{
		DataDir:           t.TempDir(),
		MinEvidenceCount:  1,
		MinVotesRequired:  3,
		ArbitrationPeriod: 72 * time.Hour,
	})

	dispute, _ := dm.CreateDispute(
		"task1",
		"complainant1",
		"defendant1",
		DisputeOther,
		"Complex issue",
		100.0,
	)

	dm.SubmitEvidence(dispute.ID, "complainant1", "text", "Evidence", "hash")
	dm.StartReview(dispute.ID)

	// Start arbitration with arbitrators
	arbitrators := []string{"arb1", "arb2", "arb3", "arb4", "arb5"}
	err := dm.StartArbitration(dispute.ID, arbitrators)
	if err != nil {
		t.Errorf("StartArbitration failed: %v", err)
	}

	updatedDispute, _ := dm.GetDispute(dispute.ID)
	if updatedDispute.Status != DisputeArbitration {
		t.Errorf("Status should be arbitration, got %s", updatedDispute.Status)
	}

	if updatedDispute.VoteDeadline == 0 {
		t.Error("Vote deadline should be set")
	}
}

func TestArbitrationVoting(t *testing.T) {
	dm := NewDisputeManager(&DisputeConfig{
		DataDir:           t.TempDir(),
		MinEvidenceCount:  1,
		MinVotesRequired:  3,
		ArbitrationPeriod: 72 * time.Hour,
	})

	dispute, _ := dm.CreateDispute(
		"task1",
		"complainant1",
		"defendant1",
		DisputeOther,
		"Issue",
		100.0,
	)

	dm.SubmitEvidence(dispute.ID, "complainant1", "text", "Evidence", "hash")
	dm.StartReview(dispute.ID)
	dm.StartArbitration(dispute.ID, []string{"arb1", "arb2", "arb3", "arb4", "arb5"})

	// Submit votes
	err := dm.SubmitVote(dispute.ID, "arb1", "complainant1", "Compelling evidence", "sig1")
	if err != nil {
		t.Errorf("SubmitVote failed: %v", err)
	}

	err = dm.SubmitVote(dispute.ID, "arb2", "complainant1", "Agree with complainant", "sig2")
	if err != nil {
		t.Errorf("SubmitVote failed: %v", err)
	}

	// Double vote should fail
	err = dm.SubmitVote(dispute.ID, "arb1", "defendant1", "Changed mind", "sig3")
	if err != ErrAlreadyVoted {
		t.Errorf("Expected already voted error, got %v", err)
	}

	// Invalid vote target
	err = dm.SubmitVote(dispute.ID, "arb3", "stranger", "Vote for stranger", "sig4")
	if err == nil {
		t.Error("Should not allow voting for non-participant")
	}

	// Third vote
	err = dm.SubmitVote(dispute.ID, "arb3", "defendant1", "Support defendant", "sig5")
	if err != nil {
		t.Errorf("SubmitVote failed: %v", err)
	}

	updatedDispute, _ := dm.GetDispute(dispute.ID)
	if len(updatedDispute.Votes) != 3 {
		t.Errorf("Expected 3 votes, got %d", len(updatedDispute.Votes))
	}
}

func TestFinalizeArbitration(t *testing.T) {
	dm := NewDisputeManager(&DisputeConfig{
		DataDir:           t.TempDir(),
		MinEvidenceCount:  1,
		MinVotesRequired:  3,
		ArbitrationPeriod: 72 * time.Hour,
	})

	dispute, _ := dm.CreateDispute(
		"task1",
		"complainant1",
		"defendant1",
		DisputeOther,
		"Issue",
		100.0,
	)

	dm.SubmitEvidence(dispute.ID, "complainant1", "text", "Evidence", "hash")
	dm.StartReview(dispute.ID)
	dm.StartArbitration(dispute.ID, []string{"arb1", "arb2", "arb3", "arb4", "arb5"})

	// Submit 3 votes for complainant, 2 for defendant
	dm.SubmitVote(dispute.ID, "arb1", "complainant1", "Reason", "sig1")
	dm.SubmitVote(dispute.ID, "arb2", "complainant1", "Reason", "sig2")
	dm.SubmitVote(dispute.ID, "arb3", "complainant1", "Reason", "sig3")
	dm.SubmitVote(dispute.ID, "arb4", "defendant1", "Reason", "sig4")
	dm.SubmitVote(dispute.ID, "arb5", "defendant1", "Reason", "sig5")

	// Finalize
	resolution, err := dm.FinalizeArbitration(dispute.ID)
	if err != nil {
		t.Errorf("FinalizeArbitration failed: %v", err)
	}

	if resolution.Winner != "complainant1" {
		t.Errorf("Complainant should win with 3 votes, got winner %s", resolution.Winner)
	}

	updatedDispute, _ := dm.GetDispute(dispute.ID)
	if updatedDispute.Status != DisputeResolved {
		t.Errorf("Status should be resolved, got %s", updatedDispute.Status)
	}

	if updatedDispute.ResolutionType != ResolutionCommittee {
		t.Errorf("Resolution type should be committee, got %s", updatedDispute.ResolutionType)
	}
}

func TestDismissDispute(t *testing.T) {
	dm := NewDisputeManager(&DisputeConfig{
		DataDir: t.TempDir(),
	})

	dispute, _ := dm.CreateDispute(
		"task1",
		"complainant1",
		"defendant1",
		DisputeOther,
		"Frivolous claim",
		10.0,
	)

	err := dm.DismissDispute(dispute.ID, "Insufficient evidence, frivolous claim")
	if err != nil {
		t.Errorf("DismissDispute failed: %v", err)
	}

	updatedDispute, _ := dm.GetDispute(dispute.ID)
	if updatedDispute.Status != DisputeDismissed {
		t.Errorf("Status should be dismissed, got %s", updatedDispute.Status)
	}

	// Cannot dismiss already resolved dispute
	dispute2, _ := dm.CreateDispute(
		"task2",
		"complainant2",
		"defendant2",
		DisputeOther,
		"Issue",
		50.0,
	)
	dm.DismissDispute(dispute2.ID, "Test")

	err = dm.DismissDispute(dispute2.ID, "Try again")
	if err != ErrDisputeResolved {
		t.Errorf("Expected dispute resolved error, got %v", err)
	}
}

func TestDisputeQueries(t *testing.T) {
	dm := NewDisputeManager(&DisputeConfig{
		DataDir: t.TempDir(),
	})

	// Create multiple disputes
	dm.CreateDispute("task1", "node1", "node2", DisputeNonDelivery, "Issue 1", 100.0)
	dm.CreateDispute("task2", "node1", "node3", DisputeQualityIssue, "Issue 2", 50.0)
	dm.CreateDispute("task3", "node4", "node1", DisputeNonPayment, "Issue 3", 75.0)

	// Query by node
	disputes := dm.GetDisputesByNode("node1")
	if len(disputes) != 3 {
		t.Errorf("Expected 3 disputes for node1, got %d", len(disputes))
	}

	disputes = dm.GetDisputesByNode("node2")
	if len(disputes) != 1 {
		t.Errorf("Expected 1 dispute for node2, got %d", len(disputes))
	}

	// Query by task
	dispute, err := dm.GetDisputeByTask("task1")
	if err != nil {
		t.Errorf("GetDisputeByTask failed: %v", err)
	}
	if dispute.Amount != 100.0 {
		t.Errorf("Wrong dispute returned")
	}

	// Query by status
	pendingDisputes := dm.GetDisputesByStatus(DisputePending)
	if len(pendingDisputes) != 3 {
		t.Errorf("Expected 3 pending disputes, got %d", len(pendingDisputes))
	}
}

func TestDisputeStatistics(t *testing.T) {
	dm := NewDisputeManager(&DisputeConfig{
		DataDir:          t.TempDir(),
		MinEvidenceCount: 1,
		AutoResolveRules: true, // 启用自动解决规则
	})

	// 注册自动解决规则
	dm.RegisterAutoRule(AutoResolveRule{
		Type: DisputeNonDelivery,
		Condition: func(d *Dispute) bool {
			return len(d.Evidence) > 0
		},
		Resolution: func(d *Dispute) *Resolution {
			return &Resolution{
				Winner:         d.ComplainantID,
				Loser:          d.DefendantID,
				AmountToWinner: d.Amount,
				Penalty:        d.Amount * 0.5,
				Reason:         "Complainant wins due to non-delivery evidence",
			}
		},
	})

	// Create and resolve some disputes
	dispute1, _ := dm.CreateDispute("task1", "comp1", "def1", DisputeNonDelivery, "Issue", 100.0)
	dm.SubmitEvidence(dispute1.ID, "comp1", "text", "Proof", "hash")
	dm.StartReview(dispute1.ID)
	dm.TryAutoResolve(dispute1.ID) // Complainant wins

	dispute2, _ := dm.CreateDispute("task2", "comp2", "def2", DisputeOther, "Issue", 50.0)
	dm.DismissDispute(dispute2.ID, "Frivolous")

	dm.CreateDispute("task3", "comp3", "def3", DisputeQualityIssue, "Pending", 75.0)

	stats := dm.GetStatistics()
	if stats.TotalDisputes != 3 {
		t.Errorf("Expected 3 disputes, got %d", stats.TotalDisputes)
	}

	if stats.ByStatus[DisputeResolved] != 1 {
		t.Errorf("Expected 1 resolved, got %d", stats.ByStatus[DisputeResolved])
	}

	if stats.ByStatus[DisputeDismissed] != 1 {
		t.Errorf("Expected 1 dismissed, got %d", stats.ByStatus[DisputeDismissed])
	}

	if stats.ComplainantWins != 1 {
		t.Errorf("Expected 1 complainant win, got %d", stats.ComplainantWins)
	}
}

func TestCheckExpiredDisputes(t *testing.T) {
	dm := NewDisputeManager(&DisputeConfig{
		DataDir:          t.TempDir(),
		ExpirationPeriod: 1 * time.Hour, // 设置较长的时间
	})

	dispute, _ := dm.CreateDispute("task1", "comp1", "def1", DisputeOther, "Issue", 100.0)

	// 手动设置过期时间为过去，模拟已过期
	dm.mu.Lock()
	dm.disputes[dispute.ID].ExpiresAt = time.Now().Unix() - 10 // 过去10秒
	dm.mu.Unlock()

	expired := dm.CheckExpiredDisputes()
	if len(expired) != 1 {
		t.Errorf("Expected 1 expired dispute, got %d", len(expired))
	}

	updatedDispute, _ := dm.GetDisputeByTask("task1")
	if updatedDispute.Status != DisputeExpired {
		t.Errorf("Status should be expired, got %s", updatedDispute.Status)
	}
}

func TestDisputePersistence(t *testing.T) {
	tempDir := t.TempDir()
	config := &DisputeConfig{
		DataDir: tempDir,
	}

	// Create dispute
	dm1 := NewDisputeManager(config)
	dispute, _ := dm1.CreateDispute("task1", "comp1", "def1", DisputeNonDelivery, "Test issue", 100.0)
	disputeID := dispute.ID

	// Load in new manager
	dm2 := NewDisputeManager(config)
	loadedDispute, err := dm2.GetDispute(disputeID)
	if err != nil {
		t.Fatalf("Failed to load dispute: %v", err)
	}

	if loadedDispute.Description != "Test issue" {
		t.Errorf("Expected description 'Test issue', got '%s'", loadedDispute.Description)
	}

	if loadedDispute.Amount != 100.0 {
		t.Errorf("Expected amount 100.0, got %.1f", loadedDispute.Amount)
	}

	os.RemoveAll(tempDir)
}

func TestResolvedDisputeCannotAcceptEvidence(t *testing.T) {
	dm := NewDisputeManager(&DisputeConfig{
		DataDir: t.TempDir(),
	})

	dispute, _ := dm.CreateDispute("task1", "comp1", "def1", DisputeOther, "Issue", 100.0)
	dm.DismissDispute(dispute.ID, "Dismissed")

	err := dm.SubmitEvidence(dispute.ID, "comp1", "text", "More evidence", "hash")
	if err != ErrDisputeResolved {
		t.Errorf("Expected dispute resolved error, got %v", err)
	}
}

func TestDisputeTypes(t *testing.T) {
	dm := NewDisputeManager(&DisputeConfig{
		DataDir: t.TempDir(),
	})

	types := []DisputeType{
		DisputeNonDelivery,
		DisputeQualityIssue,
		DisputeNonPayment,
		DisputeFalseDelivery,
		DisputeTimeout,
		DisputeOther,
	}

	for i, dt := range types {
		taskID := "task" + string(rune('a'+i))
		dispute, err := dm.CreateDispute(taskID, "comp", "def", dt, "Issue", 50.0)
		if err != nil {
			t.Errorf("Failed to create dispute with type %s: %v", dt, err)
		}
		if dispute.Type != dt {
			t.Errorf("Expected type %s, got %s", dt, dispute.Type)
		}
	}

	stats := dm.GetStatistics()
	if stats.TotalDisputes != len(types) {
		t.Errorf("Expected %d disputes, got %d", len(types), stats.TotalDisputes)
	}
}
