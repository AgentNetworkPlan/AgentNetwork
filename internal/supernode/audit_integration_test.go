package supernode

import (
	"testing"
	"time"

	"github.com/AgentNetworkPlan/AgentNetwork/internal/collateral"
	"github.com/AgentNetworkPlan/AgentNetwork/internal/ledger"
)

func TestNewAuditIntegration(t *testing.T) {
	config := DefaultAuditPenaltyConfig()
	l, _ := ledger.NewLedger("")
	cm := collateral.NewCollateralManager()

	smConfig := DefaultConfig("node1")
	sm, _ := NewSuperNodeManager(smConfig)

	ai := NewAuditIntegration(config, l, cm, sm, "system")

	if ai == nil {
		t.Fatal("AuditIntegration should not be nil")
	}

	if ai.config != config {
		t.Error("Config should be set")
	}
}

func TestDefaultAuditPenaltyConfig(t *testing.T) {
	config := DefaultAuditPenaltyConfig()

	if config.MinorDeviationPenalty <= 0 {
		t.Error("MinorDeviationPenalty should be positive")
	}

	if config.SevereDeviationPenalty <= config.MinorDeviationPenalty {
		t.Error("SevereDeviationPenalty should be greater than MinorDeviationPenalty")
	}

	if config.MinorSlashRatio <= 0 || config.MinorSlashRatio > 1 {
		t.Error("MinorSlashRatio should be between 0 and 1")
	}

	if config.SevereSlashRatio <= config.MinorSlashRatio {
		t.Error("SevereSlashRatio should be greater than MinorSlashRatio")
	}
}

func TestAuditIntegrationStart(t *testing.T) {
	config := DefaultAuditPenaltyConfig()
	l, _ := ledger.NewLedger("")
	cm := collateral.NewCollateralManager()

	smConfig := DefaultConfig("node1")
	sm, _ := NewSuperNodeManager(smConfig)

	ai := NewAuditIntegration(config, l, cm, sm, "system")
	ai.Start()

	// Verify callback is registered by checking if sm.onAuditorDeviation is set
	// (We can't directly check the private field, but we can verify through behavior)
}

func TestHandleAuditorDeviation_LedgerEvent(t *testing.T) {
	config := DefaultAuditPenaltyConfig()
	config.EnableAutoSlash = false // Disable slashing for this test
	l, _ := ledger.NewLedger(t.TempDir())

	smConfig := DefaultConfig("node1")
	sm, _ := NewSuperNodeManager(smConfig)

	ai := NewAuditIntegration(config, l, nil, sm, "system")

	deviation := &AuditDeviation{
		AuditID:        "audit123",
		AuditorID:      "auditor1",
		ExpectedResult: ResultPass,
		ActualResult:   ResultFail,
		Severity:       "severe",
		DetectedAt:     time.Now(),
	}

	// Use ManualPenalty to test
	event, _, err := ai.ManualPenalty(deviation)

	if err != nil {
		t.Fatalf("ManualPenalty should not error: %v", err)
	}

	if event == nil {
		t.Fatal("Should have created a violation event")
	}

	if event.Type != ledger.EventViolation {
		t.Errorf("Event type should be VIOLATION, got %s", event.Type)
	}

	if event.NodeID != "auditor1" {
		t.Errorf("Event should be for auditor1, got %s", event.NodeID)
	}
}

func TestHandleAuditorDeviation_CollateralSlash(t *testing.T) {
	config := DefaultAuditPenaltyConfig()
	config.EnableAutoSlash = true
	config.AuditorCollateralPurpose = "supernode_auditor"

	cm := collateral.NewCollateralManager()

	// Create collateral for auditor
	c, err := cm.CreateCollateral("auditor1", "stake", "supernode_auditor", 100.0, 24*time.Hour)
	if err != nil {
		t.Fatalf("Failed to create collateral: %v", err)
	}
	cm.ActivateCollateral(c.ID)

	smConfig := DefaultConfig("node1")
	sm, _ := NewSuperNodeManager(smConfig)

	ai := NewAuditIntegration(config, nil, cm, sm, "system")

	deviation := &AuditDeviation{
		AuditID:        "audit123",
		AuditorID:      "auditor1",
		ExpectedResult: ResultPass,
		ActualResult:   ResultFail,
		Severity:       "severe",
		DetectedAt:     time.Now(),
	}

	_, slashEvent, err := ai.ManualPenalty(deviation)

	if err != nil {
		t.Fatalf("ManualPenalty should not error: %v", err)
	}

	if slashEvent == nil {
		t.Fatal("Should have created a slash event")
	}

	if slashEvent.Owner != "auditor1" {
		t.Errorf("Slash event should be for auditor1, got %s", slashEvent.Owner)
	}

	expectedSlash := 100.0 * config.SevereSlashRatio
	if slashEvent.Amount != expectedSlash {
		t.Errorf("Slash amount should be %.2f, got %.2f", expectedSlash, slashEvent.Amount)
	}
}

func TestHandleAuditorDeviation_SeverityLevels(t *testing.T) {
	config := DefaultAuditPenaltyConfig()

	ai := &AuditIntegration{config: config}

	// Test minor severity
	repPenalty, slashRatio := ai.GetPenaltyForSeverity("minor")
	if repPenalty != config.MinorDeviationPenalty {
		t.Errorf("Minor reputation penalty mismatch: got %.2f, want %.2f",
			repPenalty, config.MinorDeviationPenalty)
	}
	if slashRatio != config.MinorSlashRatio {
		t.Errorf("Minor slash ratio mismatch: got %.2f, want %.2f",
			slashRatio, config.MinorSlashRatio)
	}

	// Test severe severity
	repPenalty, slashRatio = ai.GetPenaltyForSeverity("severe")
	if repPenalty != config.SevereDeviationPenalty {
		t.Errorf("Severe reputation penalty mismatch: got %.2f, want %.2f",
			repPenalty, config.SevereDeviationPenalty)
	}
	if slashRatio != config.SevereSlashRatio {
		t.Errorf("Severe slash ratio mismatch: got %.2f, want %.2f",
			slashRatio, config.SevereSlashRatio)
	}

	// Test unknown severity (should default to minor)
	repPenalty, slashRatio = ai.GetPenaltyForSeverity("unknown")
	if repPenalty != config.MinorDeviationPenalty {
		t.Errorf("Unknown severity should default to minor penalty")
	}
}

func TestAuditIntegrationCallback(t *testing.T) {
	config := DefaultAuditPenaltyConfig()
	config.EnableAutoSlash = false

	l, _ := ledger.NewLedger(t.TempDir())
	cm := collateral.NewCollateralManager()
	smConfig := DefaultConfig("node1")
	sm, _ := NewSuperNodeManager(smConfig)

	ai := NewAuditIntegration(config, l, cm, sm, "system")

	callbackCalled := false
	var receivedDeviation *AuditDeviation
	var receivedEvent *ledger.Event

	ai.SetOnPenaltyApplied(func(d *AuditDeviation, e *ledger.Event, s *collateral.SlashEvent) {
		callbackCalled = true
		receivedDeviation = d
		receivedEvent = e
	})

	ai.Start()

	// Simulate deviation by directly calling the handler
	deviation := &AuditDeviation{
		AuditID:        "audit456",
		AuditorID:      "auditor2",
		ExpectedResult: ResultPass,
		ActualResult:   ResultFail,
		Severity:       "minor",
		DetectedAt:     time.Now(),
	}

	ai.handleAuditorDeviation(deviation)

	if !callbackCalled {
		t.Error("Callback should have been called")
	}

	if receivedDeviation != deviation {
		t.Error("Callback should receive the same deviation")
	}

	if receivedEvent == nil {
		t.Error("Callback should receive the violation event")
	}
}

func TestIntegrationWithSupernodeAudit(t *testing.T) {
	// Create all components
	config := DefaultAuditPenaltyConfig()
	config.EnableAutoSlash = true
	config.AuditorCollateralPurpose = "supernode_auditor"

	l, _ := ledger.NewLedger(t.TempDir())
	cm := collateral.NewCollateralManager()

	smConfig := DefaultConfig("node1")
	smConfig.AuditThreshold = 0.5 // 50% threshold
	smConfig.AuditorsPerTask = 3
	sm, _ := NewSuperNodeManager(smConfig)

	ai := NewAuditIntegration(config, l, cm, sm, "system")

	// Track penalties
	penalties := make([]string, 0)
	ai.SetOnPenaltyApplied(func(d *AuditDeviation, e *ledger.Event, s *collateral.SlashEvent) {
		penalties = append(penalties, d.AuditorID)
	})

	ai.Start()

	// Add super nodes (auditors) directly to map (same pattern as other tests)
	for _, nodeID := range []string{"sn1", "sn2", "sn3"} {
		sm.superNodes[nodeID] = &SuperNode{
			NodeID:     nodeID,
			IsActive:   true,
			Reputation: 100,
			Stake:      50,
			AuditCount: 0,
			PassRate:   1.0,
		}

		// Create collateral for each auditor
		c, _ := cm.CreateCollateral(nodeID, "stake", "supernode_auditor", 100.0, 24*time.Hour)
		cm.ActivateCollateral(c.ID)
	}

	// Create audit
	audit, err := sm.CreateAudit(AuditTask, "target1")
	if err != nil {
		t.Fatalf("Failed to create audit: %v", err)
	}

	// sn1 and sn2 submit PASS, sn3 submits FAIL (deviation)
	for _, auditor := range audit.Auditors {
		result := ResultPass
		if auditor == "sn3" {
			result = ResultFail // This will deviate from consensus
		}
		sm.SubmitAuditResult(audit.ID, auditor, result, "evidence")
	}

	// Wait for async callback processing
	time.Sleep(100 * time.Millisecond)

	// Verify sn3 was penalized
	found := false
	for _, id := range penalties {
		if id == "sn3" {
			found = true
			break
		}
	}

	if !found {
		t.Error("sn3 should have been penalized for deviating from consensus")
	}
}
