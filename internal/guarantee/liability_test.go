package guarantee

import (
	"testing"
)

func TestProcessViolation(t *testing.T) {
	gm, _ := NewGuaranteeManager("")
	gm.SetReputationFunc(func(nodeID string) float64 {
		return 50.0
	})

	// Create and activate guarantee
	g, _ := gm.CreateGuarantee("sponsor1", "pk", "violator1", "pk", nil)
	gm.ActivateGuarantee(g.ID)

	// Process violation
	records, err := gm.ProcessViolation("violator1", ViolationMinor, 10.0)
	if err != nil {
		t.Fatalf("Failed to process violation: %v", err)
	}

	if len(records) != 1 {
		t.Errorf("Expected 1 liability record, got %d", len(records))
	}

	record := records[0]
	if record.SponsorID != "sponsor1" {
		t.Errorf("Expected sponsor sponsor1, got %s", record.SponsorID)
	}
	if record.ViolatorID != "violator1" {
		t.Errorf("Expected violator violator1, got %s", record.ViolatorID)
	}
	if record.OriginalPenalty != 10.0 {
		t.Errorf("Expected original penalty 10.0, got %f", record.OriginalPenalty)
	}

	// Minor violation = 30% liability
	expectedPenalty := 10.0 * 0.3
	if record.SponsorPenalty != expectedPenalty {
		t.Errorf("Expected sponsor penalty %f, got %f", expectedPenalty, record.SponsorPenalty)
	}
}

func TestProcessViolationNoSponsor(t *testing.T) {
	gm, _ := NewGuaranteeManager("")

	// Process violation for node without sponsor
	records, err := gm.ProcessViolation("unknown", ViolationMinor, 10.0)
	if err != nil {
		t.Fatalf("Should not fail: %v", err)
	}

	if records != nil && len(records) > 0 {
		t.Error("Should return empty records for node without sponsor")
	}
}

func TestProcessViolationInactiveGuarantee(t *testing.T) {
	gm, _ := NewGuaranteeManager("")
	gm.SetReputationFunc(func(nodeID string) float64 {
		return 50.0
	})

	// Create guarantee but don't activate
	gm.CreateGuarantee("sponsor1", "pk", "violator1", "pk", nil)

	// Process violation - should not generate liability
	records, err := gm.ProcessViolation("violator1", ViolationMinor, 10.0)
	if err != nil {
		t.Fatalf("Should not fail: %v", err)
	}

	if len(records) != 0 {
		t.Error("Should not generate liability for inactive guarantee")
	}
}

func TestViolationSeverityLevels(t *testing.T) {
	// Note: The liability penalty is min(liabilityRatio, guaranteeLiabilityRatio) * penalty
	// Default guarantee liability ratio is 0.5 (50%)
	// So even if violation type has higher ratio, it's capped at 0.5
	tests := []struct {
		vType    ViolationType
		penalty  float64
		expected float64
	}{
		{ViolationMinor, 10.0, 3.0},    // 30% < 50%, so use 30%
		{ViolationModerate, 10.0, 5.0}, // 50% == 50%, so use 50%
		{ViolationSevere, 10.0, 5.0},   // 70% > 50%, capped at 50%
		{ViolationCritical, 10.0, 5.0}, // 100% > 50%, capped at 50%
	}

	for _, tt := range tests {
		gm, _ := NewGuaranteeManager("")
		gm.SetReputationFunc(func(nodeID string) float64 {
			return 50.0
		})

		g, _ := gm.CreateGuarantee("sponsor", "pk", "violator", "pk", nil)
		gm.ActivateGuarantee(g.ID)

		records, _ := gm.ProcessViolation("violator", tt.vType, tt.penalty)
		if len(records) == 0 {
			t.Errorf("Expected liability for %s", tt.vType)
			continue
		}

		actual := records[0].SponsorPenalty
		if actual != tt.expected {
			t.Errorf("For %s: expected penalty %f, got %f", tt.vType, tt.expected, actual)
		}
	}
}

func TestSettleLiability(t *testing.T) {
	gm, _ := NewGuaranteeManager("")
	gm.SetReputationFunc(func(nodeID string) float64 {
		return 50.0
	})

	g, _ := gm.CreateGuarantee("sponsor1", "pk", "violator1", "pk", nil)
	gm.ActivateGuarantee(g.ID)

	records, _ := gm.ProcessViolation("violator1", ViolationMinor, 10.0)
	record := records[0]

	if record.Status != "pending" {
		t.Errorf("Initial status should be pending")
	}

	err := gm.SettleLiability(record.ID)
	if err != nil {
		t.Fatalf("Failed to settle: %v", err)
	}

	settled := gm.GetLiability(record.ID)
	if settled.Status != "settled" {
		t.Errorf("Status should be settled")
	}
	if settled.SettledAt == 0 {
		t.Error("SettledAt should be set")
	}
}

func TestSettleLiabilityAlreadySettled(t *testing.T) {
	gm, _ := NewGuaranteeManager("")
	gm.SetReputationFunc(func(nodeID string) float64 {
		return 50.0
	})

	g, _ := gm.CreateGuarantee("sponsor1", "pk", "violator1", "pk", nil)
	gm.ActivateGuarantee(g.ID)

	records, _ := gm.ProcessViolation("violator1", ViolationMinor, 10.0)
	record := records[0]

	gm.SettleLiability(record.ID)

	// Try to settle again
	err := gm.SettleLiability(record.ID)
	if err == nil {
		t.Error("Should fail when already settled")
	}
}

func TestGetPendingLiabilities(t *testing.T) {
	gm, _ := NewGuaranteeManager("")
	gm.SetReputationFunc(func(nodeID string) float64 {
		return 50.0
	})

	g1, _ := gm.CreateGuarantee("sponsor1", "pk", "violator1", "pk", nil)
	gm.ActivateGuarantee(g1.ID)
	g2, _ := gm.CreateGuarantee("sponsor2", "pk", "violator2", "pk", nil)
	gm.ActivateGuarantee(g2.ID)

	records1, _ := gm.ProcessViolation("violator1", ViolationMinor, 10.0)
	gm.ProcessViolation("violator2", ViolationMinor, 10.0)

	// Settle one
	gm.SettleLiability(records1[0].ID)

	pending := gm.GetPendingLiabilities()
	if len(pending) != 1 {
		t.Errorf("Expected 1 pending, got %d", len(pending))
	}
}

func TestGetLiabilitiesBySponsor(t *testing.T) {
	gm, _ := NewGuaranteeManager("")
	gm.SetReputationFunc(func(nodeID string) float64 {
		return 50.0
	})

	g1, _ := gm.CreateGuarantee("sponsor1", "pk", "v1", "pk", nil)
	gm.ActivateGuarantee(g1.ID)
	g2, _ := gm.CreateGuarantee("sponsor1", "pk", "v2", "pk", nil)
	gm.ActivateGuarantee(g2.ID)
	g3, _ := gm.CreateGuarantee("sponsor2", "pk", "v3", "pk", nil)
	gm.ActivateGuarantee(g3.ID)

	gm.ProcessViolation("v1", ViolationMinor, 10.0)
	gm.ProcessViolation("v2", ViolationMinor, 10.0)
	gm.ProcessViolation("v3", ViolationMinor, 10.0)

	sponsor1Liabilities := gm.GetLiabilitiesBySponsor("sponsor1")
	if len(sponsor1Liabilities) != 2 {
		t.Errorf("Expected 2 liabilities for sponsor1, got %d", len(sponsor1Liabilities))
	}

	sponsor2Liabilities := gm.GetLiabilitiesBySponsor("sponsor2")
	if len(sponsor2Liabilities) != 1 {
		t.Errorf("Expected 1 liability for sponsor2, got %d", len(sponsor2Liabilities))
	}
}

func TestCalculateTotalLiability(t *testing.T) {
	gm, _ := NewGuaranteeManager("")
	gm.SetReputationFunc(func(nodeID string) float64 {
		return 50.0
	})

	g1, _ := gm.CreateGuarantee("sponsor1", "pk", "v1", "pk", nil)
	gm.ActivateGuarantee(g1.ID)
	g2, _ := gm.CreateGuarantee("sponsor1", "pk", "v2", "pk", nil)
	gm.ActivateGuarantee(g2.ID)

	gm.ProcessViolation("v1", ViolationMinor, 10.0) // 3.0 liability
	gm.ProcessViolation("v2", ViolationMinor, 20.0) // 6.0 liability

	total := gm.CalculateTotalLiability("sponsor1")
	expected := 3.0 + 6.0
	if total != expected {
		t.Errorf("Expected total liability %f, got %f", expected, total)
	}
}

func TestLiabilityCount(t *testing.T) {
	gm, _ := NewGuaranteeManager("")
	gm.SetReputationFunc(func(nodeID string) float64 {
		return 50.0
	})

	if gm.LiabilityCount() != 0 {
		t.Error("Initial count should be 0")
	}

	g, _ := gm.CreateGuarantee("sponsor1", "pk", "v1", "pk", nil)
	gm.ActivateGuarantee(g.ID)
	gm.ProcessViolation("v1", ViolationMinor, 10.0)

	if gm.LiabilityCount() != 1 {
		t.Errorf("Expected 1 liability, got %d", gm.LiabilityCount())
	}
}

func TestPendingLiabilityCount(t *testing.T) {
	gm, _ := NewGuaranteeManager("")
	gm.SetReputationFunc(func(nodeID string) float64 {
		return 50.0
	})

	g1, _ := gm.CreateGuarantee("s1", "pk", "v1", "pk", nil)
	gm.ActivateGuarantee(g1.ID)
	g2, _ := gm.CreateGuarantee("s2", "pk", "v2", "pk", nil)
	gm.ActivateGuarantee(g2.ID)

	r1, _ := gm.ProcessViolation("v1", ViolationMinor, 10.0)
	gm.ProcessViolation("v2", ViolationMinor, 10.0)

	if gm.PendingLiabilityCount() != 2 {
		t.Errorf("Expected 2 pending, got %d", gm.PendingLiabilityCount())
	}

	gm.SettleLiability(r1[0].ID)

	if gm.PendingLiabilityCount() != 1 {
		t.Errorf("Expected 1 pending after settle, got %d", gm.PendingLiabilityCount())
	}
}
