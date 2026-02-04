package guarantee

import (
	"testing"
	"time"
)

func TestCreateGuarantee(t *testing.T) {
	gm, err := NewGuaranteeManager("")
	if err != nil {
		t.Fatalf("Failed to create manager: %v", err)
	}

	// Set reputation function
	gm.SetReputationFunc(func(nodeID string) float64 {
		return 50.0 // Enough reputation
	})

	g, err := gm.CreateGuarantee(
		"sponsor1", "pubkey_sponsor",
		"newnode1", "pubkey_newnode",
		nil,
	)
	if err != nil {
		t.Fatalf("Failed to create guarantee: %v", err)
	}

	if g.ID == "" {
		t.Error("Guarantee ID should not be empty")
	}
	if g.SponsorID != "sponsor1" {
		t.Errorf("Expected sponsor sponsor1, got %s", g.SponsorID)
	}
	if g.NewNodeID != "newnode1" {
		t.Errorf("Expected new node newnode1, got %s", g.NewNodeID)
	}
	if g.Status != GuaranteeStatusPending {
		t.Errorf("Expected status pending, got %s", g.Status)
	}
}

func TestCreateGuaranteeLowReputation(t *testing.T) {
	gm, _ := NewGuaranteeManager("")
	gm.SetReputationFunc(func(nodeID string) float64 {
		return 10.0 // Not enough reputation
	})

	_, err := gm.CreateGuarantee(
		"sponsor1", "pubkey_sponsor",
		"newnode1", "pubkey_newnode",
		nil,
	)
	if err == nil {
		t.Error("Should fail with low reputation")
	}
}

func TestCreateGuaranteeMaxSponsorships(t *testing.T) {
	gm, _ := NewGuaranteeManager("")
	gm.SetReputationFunc(func(nodeID string) float64 {
		return 50.0
	})

	// Note: MaxSponsoredNodes (5) is larger than MaxInvitesPerDay (2)
	// So we need to use different sponsors or manipulate invitations
	// For this test, we manually add guarantees to bypass daily limit

	// Directly add guarantees to the manager
	for i := 0; i < MaxSponsoredNodes; i++ {
		g := &Guarantee{
			ID:                generateID(),
			SponsorID:         "sponsor1",
			NewNodeID:         "existingnode" + string(rune('0'+i)),
			SponsorReputation: 50.0,
			Status:            GuaranteeStatusActive,
			LiabilityRatio:    0.5,
		}
		gm.guarantees[g.ID] = g
		gm.bySponsor["sponsor1"] = append(gm.bySponsor["sponsor1"], g.ID)
	}

	// Now try to create another one - should fail due to max sponsorships
	_, err := gm.CreateGuarantee(
		"sponsor1", "pubkey",
		"newnodeX", "pubkey",
		nil,
	)
	if err == nil {
		t.Error("Should fail when max sponsorships reached")
	}
}

func TestCreateGuaranteeDailyLimit(t *testing.T) {
	gm, _ := NewGuaranteeManager("")
	gm.SetReputationFunc(func(nodeID string) float64 {
		return 50.0
	})

	// Create max daily invitations
	for i := 0; i < MaxInvitesPerDay; i++ {
		_, err := gm.CreateGuarantee(
			"sponsor1", "pubkey",
			"newnode"+string(rune('A'+i)), "pubkey",
			nil,
		)
		if err != nil {
			t.Fatalf("Failed to create guarantee %d: %v", i, err)
		}
	}

	// Next one should fail (daily limit)
	_, err := gm.CreateGuarantee(
		"sponsor1", "pubkey",
		"newnodeZ", "pubkey",
		nil,
	)
	if err == nil {
		t.Error("Should fail when daily limit reached")
	}
}

func TestActivateGuarantee(t *testing.T) {
	gm, _ := NewGuaranteeManager("")
	gm.SetReputationFunc(func(nodeID string) float64 {
		return 50.0
	})

	g, _ := gm.CreateGuarantee(
		"sponsor1", "pubkey",
		"newnode1", "pubkey",
		nil,
	)

	if g.Status != GuaranteeStatusPending {
		t.Errorf("Initial status should be pending")
	}

	err := gm.ActivateGuarantee(g.ID)
	if err != nil {
		t.Fatalf("Failed to activate: %v", err)
	}

	g = gm.GetGuarantee(g.ID)
	if g.Status != GuaranteeStatusActive {
		t.Errorf("Status should be active after activation")
	}
}

func TestValidateGuarantee(t *testing.T) {
	gm, _ := NewGuaranteeManager("")

	// Valid guarantee
	validG := &Guarantee{
		ID:                "test1",
		SponsorID:         "sponsor1",
		NewNodeID:         "newnode1",
		SponsorReputation: 50.0,
		LiabilityRatio:    0.5,
		ValidUntil:        time.Now().Add(24 * time.Hour).Unix(),
	}

	if err := gm.ValidateGuarantee(validG); err != nil {
		t.Errorf("Valid guarantee should pass: %v", err)
	}

	// Test nil
	if err := gm.ValidateGuarantee(nil); err == nil {
		t.Error("Nil guarantee should fail")
	}

	// Test empty ID
	badG := *validG
	badG.ID = ""
	if err := gm.ValidateGuarantee(&badG); err == nil {
		t.Error("Empty ID should fail")
	}

	// Test self-guarantee
	badG = *validG
	badG.SponsorID = "same"
	badG.NewNodeID = "same"
	if err := gm.ValidateGuarantee(&badG); err == nil {
		t.Error("Self-guarantee should fail")
	}

	// Test low reputation
	badG = *validG
	badG.SponsorReputation = 10.0
	if err := gm.ValidateGuarantee(&badG); err == nil {
		t.Error("Low reputation should fail")
	}

	// Test invalid liability ratio
	badG = *validG
	badG.LiabilityRatio = 1.5
	if err := gm.ValidateGuarantee(&badG); err == nil {
		t.Error("Invalid liability ratio should fail")
	}

	// Test expired
	badG = *validG
	badG.ValidUntil = time.Now().Add(-24 * time.Hour).Unix()
	if err := gm.ValidateGuarantee(&badG); err == nil {
		t.Error("Expired guarantee should fail")
	}
}

func TestGetGuaranteesByNode(t *testing.T) {
	gm, _ := NewGuaranteeManager("")
	gm.SetReputationFunc(func(nodeID string) float64 {
		return 50.0
	})

	// Create guarantees for different nodes
	gm.CreateGuarantee("sponsor1", "pk", "node1", "pk", nil)
	gm.CreateGuarantee("sponsor2", "pk", "node1", "pk", nil)
	gm.CreateGuarantee("sponsor1", "pk", "node2", "pk", nil)

	// Get by node
	guarantees := gm.GetGuaranteesByNode("node1")
	if len(guarantees) != 2 {
		t.Errorf("Expected 2 guarantees for node1, got %d", len(guarantees))
	}

	guarantees = gm.GetGuaranteesByNode("node2")
	if len(guarantees) != 1 {
		t.Errorf("Expected 1 guarantee for node2, got %d", len(guarantees))
	}
}

func TestGetGuaranteesBySponsor(t *testing.T) {
	gm, _ := NewGuaranteeManager("")
	gm.SetReputationFunc(func(nodeID string) float64 {
		return 50.0
	})

	gm.CreateGuarantee("sponsor1", "pk", "node1", "pk", nil)
	gm.CreateGuarantee("sponsor1", "pk", "node2", "pk", nil)
	gm.CreateGuarantee("sponsor2", "pk", "node3", "pk", nil)

	guarantees := gm.GetGuaranteesBySponsor("sponsor1")
	if len(guarantees) != 2 {
		t.Errorf("Expected 2 guarantees by sponsor1, got %d", len(guarantees))
	}
}

func TestExpireGuarantees(t *testing.T) {
	gm, _ := NewGuaranteeManager("")
	gm.SetReputationFunc(func(nodeID string) float64 {
		return 50.0
	})

	// Create guarantee with past expiry
	g, _ := gm.CreateGuarantee("sponsor1", "pk", "node1", "pk", nil)
	gm.ActivateGuarantee(g.ID)

	// Manually set expiry to past
	g = gm.GetGuarantee(g.ID)
	g.ValidUntil = time.Now().Add(-1 * time.Hour).Unix()

	// Run expiration
	count := gm.ExpireGuarantees()
	if count != 1 {
		t.Errorf("Expected 1 expired, got %d", count)
	}

	g = gm.GetGuarantee(g.ID)
	if g.Status != GuaranteeStatusExpired {
		t.Errorf("Status should be expired")
	}
}

func TestGuaranteeCount(t *testing.T) {
	gm, _ := NewGuaranteeManager("")
	gm.SetReputationFunc(func(nodeID string) float64 {
		return 50.0
	})

	if gm.GuaranteeCount() != 0 {
		t.Error("Initial count should be 0")
	}

	gm.CreateGuarantee("s1", "pk", "n1", "pk", nil)
	gm.CreateGuarantee("s2", "pk", "n2", "pk", nil)

	if gm.GuaranteeCount() != 2 {
		t.Errorf("Expected 2 guarantees, got %d", gm.GuaranteeCount())
	}
}

func TestActiveGuaranteeCount(t *testing.T) {
	gm, _ := NewGuaranteeManager("")
	gm.SetReputationFunc(func(nodeID string) float64 {
		return 50.0
	})

	g1, _ := gm.CreateGuarantee("s1", "pk", "n1", "pk", nil)
	g2, _ := gm.CreateGuarantee("s2", "pk", "n2", "pk", nil)

	// Only activate one
	gm.ActivateGuarantee(g1.ID)

	if gm.ActiveGuaranteeCount() != 1 {
		t.Errorf("Expected 1 active, got %d", gm.ActiveGuaranteeCount())
	}

	gm.ActivateGuarantee(g2.ID)
	if gm.ActiveGuaranteeCount() != 2 {
		t.Errorf("Expected 2 active, got %d", gm.ActiveGuaranteeCount())
	}
}

func TestReset(t *testing.T) {
	gm, _ := NewGuaranteeManager("")
	gm.SetReputationFunc(func(nodeID string) float64 {
		return 50.0
	})

	gm.CreateGuarantee("s1", "pk", "n1", "pk", nil)
	gm.CreateGuarantee("s2", "pk", "n2", "pk", nil)

	gm.Reset()

	if gm.GuaranteeCount() != 0 {
		t.Errorf("Expected 0 after reset, got %d", gm.GuaranteeCount())
	}
}
