package guarantee

import (
	"fmt"
	"time"
)

// Liability ratios for different violation types
var liabilityRatios = map[ViolationType]float64{
	ViolationMinor:    0.3,  // 30% of penalty passed to sponsor
	ViolationModerate: 0.5,  // 50% of penalty passed to sponsor
	ViolationSevere:   0.7,  // 70% of penalty passed to sponsor
	ViolationCritical: 1.0,  // 100% of penalty passed to sponsor (kicked together)
}

// Liability periods - how long after joining the sponsor is still liable
var liabilityPeriods = map[ViolationType]time.Duration{
	ViolationMinor:    30 * 24 * time.Hour,  // 30 days
	ViolationModerate: 60 * 24 * time.Hour,  // 60 days
	ViolationSevere:   90 * 24 * time.Hour,  // 90 days
	ViolationCritical: 365 * 24 * time.Hour, // Permanent (1 year)
}

// ProcessViolation calculates and records the liability when a sponsored node violates
func (gm *GuaranteeManager) ProcessViolation(
	violatorID string,
	violationType ViolationType,
	originalPenalty float64,
) ([]*LiabilityRecord, error) {
	gm.mu.Lock()
	defer gm.mu.Unlock()

	// Find active guarantees for this violator
	guaranteeIDs := gm.byNode[violatorID]
	if len(guaranteeIDs) == 0 {
		return nil, nil // No sponsor, no liability
	}

	now := time.Now().Unix()
	liabilityPeriod := liabilityPeriods[violationType]
	var records []*LiabilityRecord

	for _, gid := range guaranteeIDs {
		g := gm.guarantees[gid]
		if g == nil || g.Status != GuaranteeStatusActive {
			continue
		}

		// Check if still within liability period
		joinedAt := g.CreatedAt
		if now-joinedAt > int64(liabilityPeriod.Seconds()) {
			continue // Past liability period
		}

		// Calculate sponsor penalty
		// Use the minimum of:
		// 1. Default liability ratio for this violation type
		// 2. The guarantee's specified liability ratio
		// 3. Remaining guarantee amount
		liabilityRatio := liabilityRatios[violationType]
		if g.LiabilityRatio < liabilityRatio {
			liabilityRatio = g.LiabilityRatio
		}

		sponsorPenalty := originalPenalty * liabilityRatio
		if sponsorPenalty > g.GuaranteeAmount {
			sponsorPenalty = g.GuaranteeAmount
		}

		// Create liability record
		record := &LiabilityRecord{
			ID:              generateID(),
			SponsorID:       g.SponsorID,
			ViolatorID:      violatorID,
			GuaranteeID:     gid,
			ViolationType:   violationType,
			OriginalPenalty: originalPenalty,
			SponsorPenalty:  sponsorPenalty,
			Status:          "pending",
			CreatedAt:       now,
		}

		gm.liabilities[record.ID] = record
		records = append(records, record)

		// Update guarantee status for severe violations
		if violationType == ViolationSevere || violationType == ViolationCritical {
			g.Status = GuaranteeStatusSettled
			g.UpdatedAt = now
		}
	}

	if len(records) > 0 {
		gm.saveAsync()
	}

	return records, nil
}

// SettleLiability marks a liability as settled
func (gm *GuaranteeManager) SettleLiability(liabilityID string) error {
	gm.mu.Lock()
	defer gm.mu.Unlock()

	l, ok := gm.liabilities[liabilityID]
	if !ok {
		return fmt.Errorf("liability not found: %s", liabilityID)
	}

	if l.Status == "settled" {
		return fmt.Errorf("liability already settled")
	}

	l.Status = "settled"
	l.SettledAt = time.Now().Unix()

	gm.saveAsync()
	return nil
}

// GetLiability returns a liability by ID
func (gm *GuaranteeManager) GetLiability(id string) *LiabilityRecord {
	gm.mu.RLock()
	defer gm.mu.RUnlock()
	return gm.liabilities[id]
}

// GetPendingLiabilities returns all pending liabilities
func (gm *GuaranteeManager) GetPendingLiabilities() []*LiabilityRecord {
	gm.mu.RLock()
	defer gm.mu.RUnlock()

	var result []*LiabilityRecord
	for _, l := range gm.liabilities {
		if l.Status == "pending" {
			result = append(result, l)
		}
	}
	return result
}

// GetLiabilitiesBySponsor returns all liabilities for a sponsor
func (gm *GuaranteeManager) GetLiabilitiesBySponsor(sponsorID string) []*LiabilityRecord {
	gm.mu.RLock()
	defer gm.mu.RUnlock()

	var result []*LiabilityRecord
	for _, l := range gm.liabilities {
		if l.SponsorID == sponsorID {
			result = append(result, l)
		}
	}
	return result
}

// GetLiabilitiesByViolator returns all liabilities for a violator
func (gm *GuaranteeManager) GetLiabilitiesByViolator(violatorID string) []*LiabilityRecord {
	gm.mu.RLock()
	defer gm.mu.RUnlock()

	var result []*LiabilityRecord
	for _, l := range gm.liabilities {
		if l.ViolatorID == violatorID {
			result = append(result, l)
		}
	}
	return result
}

// CalculateTotalLiability calculates total pending liability for a sponsor
func (gm *GuaranteeManager) CalculateTotalLiability(sponsorID string) float64 {
	gm.mu.RLock()
	defer gm.mu.RUnlock()

	var total float64
	for _, l := range gm.liabilities {
		if l.SponsorID == sponsorID && l.Status == "pending" {
			total += l.SponsorPenalty
		}
	}
	return total
}

// LiabilityCount returns the total number of liabilities
func (gm *GuaranteeManager) LiabilityCount() int {
	gm.mu.RLock()
	defer gm.mu.RUnlock()
	return len(gm.liabilities)
}

// PendingLiabilityCount returns the number of pending liabilities
func (gm *GuaranteeManager) PendingLiabilityCount() int {
	gm.mu.RLock()
	defer gm.mu.RUnlock()

	count := 0
	for _, l := range gm.liabilities {
		if l.Status == "pending" {
			count++
		}
	}
	return count
}
