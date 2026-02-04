// Package guarantee provides guarantee (sponsorship) management for node network joining.
// It implements the guarantee mechanism where existing nodes sponsor new nodes.
package guarantee

import (
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"time"
)

// Constants for guarantee parameters
const (
	// Reputation requirements
	MinSponsorReputation   = 30.0  // Minimum reputation to be a sponsor
	DefaultGuaranteeAmount = 10.0  // Default guarantee amount (reputation at stake)
	DefaultLiabilityRatio  = 0.5   // Default liability ratio (50%)
	GuaranteeValidDays     = 90    // Guarantee validity period in days
	MaxSponsoredNodes      = 5     // Maximum active sponsorships per node

	// Invitation limits
	MaxInvitesPerDay       = 2     // Maximum invitations per day
	InvitationCost         = 5.0   // Reputation cost for invitation (locked)
	CooldownAfterViolation = 7 * 24 * time.Hour // Cooldown after sponsored node violates
)

// GuaranteeStatus represents the status of a guarantee
type GuaranteeStatus string

const (
	GuaranteeStatusPending   GuaranteeStatus = "pending"   // Waiting for committee approval
	GuaranteeStatusActive    GuaranteeStatus = "active"    // Active guarantee
	GuaranteeStatusExpired   GuaranteeStatus = "expired"   // Guarantee expired
	GuaranteeStatusRevoked   GuaranteeStatus = "revoked"   // Guarantee revoked
	GuaranteeStatusSettled   GuaranteeStatus = "settled"   // Liability settled
	GuaranteeStatusCompleted GuaranteeStatus = "completed" // Successfully completed (no violations)
)

// Guarantee represents a guarantee/sponsorship record
type Guarantee struct {
	// Basic information
	ID      string `json:"id"`      // Guarantee ID
	Version int    `json:"version"` // Version number

	// Sponsor information
	SponsorID         string  `json:"sponsor_id"`         // Sponsor node ID
	SponsorPubKey     string  `json:"sponsor_pubkey"`     // Sponsor public key
	SponsorReputation float64 `json:"sponsor_reputation"` // Sponsor's reputation at time of guarantee

	// Sponsored node information
	NewNodeID     string `json:"new_node_id"`     // New node ID
	NewNodePubKey string `json:"new_node_pubkey"` // New node public key

	// Guarantee terms
	InitialReputation float64 `json:"initial_reputation"` // Initial reputation for new node
	GuaranteeAmount   float64 `json:"guarantee_amount"`   // Amount at stake
	LiabilityRatio    float64 `json:"liability_ratio"`    // Liability ratio (0-1)
	ValidUntil        int64   `json:"valid_until"`        // Validity timestamp

	// Status
	Status    GuaranteeStatus `json:"status"`
	CreatedAt int64           `json:"created_at"` // Creation timestamp
	UpdatedAt int64           `json:"updated_at"` // Last update timestamp

	// Signatures
	SponsorSignature string `json:"sponsor_signature"` // Sponsor's signature
	Nonce            string `json:"nonce"`             // Anti-replay nonce
}

// ViolationType defines types of violations
type ViolationType string

const (
	ViolationMinor    ViolationType = "minor"    // Minor violation
	ViolationModerate ViolationType = "moderate" // Moderate violation
	ViolationSevere   ViolationType = "severe"   // Severe violation
	ViolationCritical ViolationType = "critical" // Critical/malicious violation
)

// LiabilityRecord represents a liability settlement record
type LiabilityRecord struct {
	ID          string `json:"id"`
	SponsorID   string `json:"sponsor_id"`    // Sponsor who bears liability
	ViolatorID  string `json:"violator_id"`   // Node that violated
	GuaranteeID string `json:"guarantee_id"`  // Related guarantee

	// Violation details
	ViolationType   ViolationType `json:"violation_type"`
	OriginalPenalty float64       `json:"original_penalty"` // Penalty for violator
	SponsorPenalty  float64       `json:"sponsor_penalty"`  // Penalty for sponsor

	// Status
	Status    string `json:"status"` // pending, settled
	CreatedAt int64  `json:"created_at"`
	SettledAt int64  `json:"settled_at"`
}

// InvitationRecord tracks invitation history for rate limiting
type InvitationRecord struct {
	SponsorID   string  `json:"sponsor_id"`
	NewNodeID   string  `json:"new_node_id"`
	GuaranteeID string  `json:"guarantee_id"`
	InvitedAt   int64   `json:"invited_at"`
	Status      string  `json:"status"` // pending, approved, rejected
}

// GuaranteeManager manages guarantees and liabilities
type GuaranteeManager struct {
	dataDir    string
	guarantees map[string]*Guarantee          // guaranteeID -> Guarantee
	byNode     map[string][]string            // newNodeID -> []guaranteeID
	bySponsor  map[string][]string            // sponsorID -> []guaranteeID
	liabilities map[string]*LiabilityRecord   // liabilityID -> LiabilityRecord
	invitations []InvitationRecord            // Invitation history

	// Callbacks
	getReputation func(nodeID string) float64 // Get node reputation

	mu sync.RWMutex
}

// NewGuaranteeManager creates a new guarantee manager
func NewGuaranteeManager(dataDir string) (*GuaranteeManager, error) {
	gm := &GuaranteeManager{
		dataDir:     dataDir,
		guarantees:  make(map[string]*Guarantee),
		byNode:      make(map[string][]string),
		bySponsor:   make(map[string][]string),
		liabilities: make(map[string]*LiabilityRecord),
		invitations: make([]InvitationRecord, 0),
	}

	// Create data directory if not exists
	if dataDir != "" {
		if err := os.MkdirAll(dataDir, 0755); err != nil {
			return nil, fmt.Errorf("failed to create data directory: %w", err)
		}

		// Load existing data
		if err := gm.load(); err != nil {
			// If file doesn't exist, that's OK
			if !os.IsNotExist(err) {
				return nil, fmt.Errorf("failed to load guarantees: %w", err)
			}
		}
	}

	return gm, nil
}

// SetReputationFunc sets the callback for getting node reputation
func (gm *GuaranteeManager) SetReputationFunc(fn func(nodeID string) float64) {
	gm.mu.Lock()
	defer gm.mu.Unlock()
	gm.getReputation = fn
}

// generateID generates a random ID
func generateID() string {
	bytes := make([]byte, 16)
	rand.Read(bytes)
	return hex.EncodeToString(bytes)
}

// CreateGuarantee creates a new guarantee
func (gm *GuaranteeManager) CreateGuarantee(
	sponsorID, sponsorPubKey string,
	newNodeID, newNodePubKey string,
	options *GuaranteeOptions,
) (*Guarantee, error) {
	gm.mu.Lock()
	defer gm.mu.Unlock()

	// Check if sponsor can invite
	if err := gm.checkCanInvite(sponsorID); err != nil {
		return nil, err
	}

	// Apply default options
	if options == nil {
		options = DefaultGuaranteeOptions()
	}

	// Get sponsor reputation
	var sponsorRep float64
	if gm.getReputation != nil {
		sponsorRep = gm.getReputation(sponsorID)
	} else {
		sponsorRep = MinSponsorReputation + 10 // Default for testing
	}

	// Validate sponsor reputation
	if sponsorRep < MinSponsorReputation {
		return nil, fmt.Errorf("sponsor reputation %.1f is below minimum %.1f",
			sponsorRep, MinSponsorReputation)
	}

	// Create guarantee
	now := time.Now()
	validUntil := now.Add(time.Duration(options.ValidDays) * 24 * time.Hour)

	nonce := generateID()
	guarantee := &Guarantee{
		ID:                generateID(),
		Version:           1,
		SponsorID:         sponsorID,
		SponsorPubKey:     sponsorPubKey,
		SponsorReputation: sponsorRep,
		NewNodeID:         newNodeID,
		NewNodePubKey:     newNodePubKey,
		InitialReputation: options.InitialReputation,
		GuaranteeAmount:   options.GuaranteeAmount,
		LiabilityRatio:    options.LiabilityRatio,
		ValidUntil:        validUntil.Unix(),
		Status:            GuaranteeStatusPending,
		CreatedAt:         now.Unix(),
		UpdatedAt:         now.Unix(),
		Nonce:             nonce,
	}

	// Store guarantee
	gm.guarantees[guarantee.ID] = guarantee
	gm.byNode[newNodeID] = append(gm.byNode[newNodeID], guarantee.ID)
	gm.bySponsor[sponsorID] = append(gm.bySponsor[sponsorID], guarantee.ID)

	// Record invitation
	gm.invitations = append(gm.invitations, InvitationRecord{
		SponsorID:   sponsorID,
		NewNodeID:   newNodeID,
		GuaranteeID: guarantee.ID,
		InvitedAt:   now.Unix(),
		Status:      "pending",
	})

	// Persist
	gm.saveAsync()

	return guarantee, nil
}

// GuaranteeOptions configures guarantee creation
type GuaranteeOptions struct {
	InitialReputation float64 // Initial reputation for new node
	GuaranteeAmount   float64 // Amount at stake
	LiabilityRatio    float64 // Liability ratio (0-1)
	ValidDays         int     // Validity period in days
}

// DefaultGuaranteeOptions returns default options
func DefaultGuaranteeOptions() *GuaranteeOptions {
	return &GuaranteeOptions{
		InitialReputation: 1.0,
		GuaranteeAmount:   DefaultGuaranteeAmount,
		LiabilityRatio:    DefaultLiabilityRatio,
		ValidDays:         GuaranteeValidDays,
	}
}

// checkCanInvite checks if a sponsor can create a new invitation
func (gm *GuaranteeManager) checkCanInvite(sponsorID string) error {
	// Check active sponsorships count
	activeCount := 0
	for _, gid := range gm.bySponsor[sponsorID] {
		if g := gm.guarantees[gid]; g != nil && g.Status == GuaranteeStatusActive {
			activeCount++
		}
	}
	if activeCount >= MaxSponsoredNodes {
		return fmt.Errorf("sponsor has reached maximum active sponsorships (%d)", MaxSponsoredNodes)
	}

	// Check daily invitation limit
	now := time.Now()
	startOfDay := time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, now.Location())
	todayCount := 0
	for _, inv := range gm.invitations {
		if inv.SponsorID == sponsorID && inv.InvitedAt >= startOfDay.Unix() {
			todayCount++
		}
	}
	if todayCount >= MaxInvitesPerDay {
		return fmt.Errorf("sponsor has reached daily invitation limit (%d)", MaxInvitesPerDay)
	}

	// Check cooldown
	for _, inv := range gm.invitations {
		if inv.SponsorID == sponsorID {
			// Check if any sponsored node had a violation that triggered cooldown
			for _, lid := range gm.getLiabilitiesBySponsor(sponsorID) {
				if l := gm.liabilities[lid]; l != nil {
					cooldownEnd := l.SettledAt + int64(CooldownAfterViolation.Seconds())
					if now.Unix() < cooldownEnd {
						return fmt.Errorf("sponsor is in cooldown period until %s",
							time.Unix(cooldownEnd, 0).Format(time.RFC3339))
					}
				}
			}
		}
	}

	return nil
}

func (gm *GuaranteeManager) getLiabilitiesBySponsor(sponsorID string) []string {
	var result []string
	for id, l := range gm.liabilities {
		if l.SponsorID == sponsorID {
			result = append(result, id)
		}
	}
	return result
}

// ActivateGuarantee activates a pending guarantee after committee approval
func (gm *GuaranteeManager) ActivateGuarantee(guaranteeID string) error {
	gm.mu.Lock()
	defer gm.mu.Unlock()

	g, ok := gm.guarantees[guaranteeID]
	if !ok {
		return fmt.Errorf("guarantee not found: %s", guaranteeID)
	}

	if g.Status != GuaranteeStatusPending {
		return fmt.Errorf("guarantee is not pending: %s", g.Status)
	}

	g.Status = GuaranteeStatusActive
	g.UpdatedAt = time.Now().Unix()

	// Update invitation status
	for i := range gm.invitations {
		if gm.invitations[i].GuaranteeID == guaranteeID {
			gm.invitations[i].Status = "approved"
		}
	}

	gm.saveAsync()
	return nil
}

// GetGuarantee returns a guarantee by ID
func (gm *GuaranteeManager) GetGuarantee(id string) *Guarantee {
	gm.mu.RLock()
	defer gm.mu.RUnlock()
	return gm.guarantees[id]
}

// GetGuaranteesByNode returns all guarantees for a node
func (gm *GuaranteeManager) GetGuaranteesByNode(nodeID string) []*Guarantee {
	gm.mu.RLock()
	defer gm.mu.RUnlock()

	var result []*Guarantee
	for _, gid := range gm.byNode[nodeID] {
		if g := gm.guarantees[gid]; g != nil {
			result = append(result, g)
		}
	}
	return result
}

// GetGuaranteesBySponsor returns all guarantees created by a sponsor
func (gm *GuaranteeManager) GetGuaranteesBySponsor(sponsorID string) []*Guarantee {
	gm.mu.RLock()
	defer gm.mu.RUnlock()

	var result []*Guarantee
	for _, gid := range gm.bySponsor[sponsorID] {
		if g := gm.guarantees[gid]; g != nil {
			result = append(result, g)
		}
	}
	return result
}

// GetActiveGuaranteesBySponsor returns active guarantees by sponsor
func (gm *GuaranteeManager) GetActiveGuaranteesBySponsor(sponsorID string) []*Guarantee {
	gm.mu.RLock()
	defer gm.mu.RUnlock()

	var result []*Guarantee
	for _, gid := range gm.bySponsor[sponsorID] {
		if g := gm.guarantees[gid]; g != nil && g.Status == GuaranteeStatusActive {
			result = append(result, g)
		}
	}
	return result
}

// ValidateGuarantee validates a guarantee
func (gm *GuaranteeManager) ValidateGuarantee(g *Guarantee) error {
	if g == nil {
		return fmt.Errorf("guarantee is nil")
	}
	if g.ID == "" {
		return fmt.Errorf("guarantee ID is empty")
	}
	if g.SponsorID == "" {
		return fmt.Errorf("sponsor ID is empty")
	}
	if g.NewNodeID == "" {
		return fmt.Errorf("new node ID is empty")
	}
	if g.SponsorID == g.NewNodeID {
		return fmt.Errorf("sponsor cannot guarantee self")
	}
	if g.SponsorReputation < MinSponsorReputation {
		return fmt.Errorf("sponsor reputation %.1f below minimum %.1f",
			g.SponsorReputation, MinSponsorReputation)
	}
	if g.LiabilityRatio < 0 || g.LiabilityRatio > 1 {
		return fmt.Errorf("liability ratio must be between 0 and 1")
	}
	if g.ValidUntil < time.Now().Unix() {
		return fmt.Errorf("guarantee has expired")
	}
	return nil
}

// ExpireGuarantees marks expired guarantees
func (gm *GuaranteeManager) ExpireGuarantees() int {
	gm.mu.Lock()
	defer gm.mu.Unlock()

	now := time.Now().Unix()
	count := 0

	for _, g := range gm.guarantees {
		if g.Status == GuaranteeStatusActive && g.ValidUntil < now {
			g.Status = GuaranteeStatusExpired
			g.UpdatedAt = now
			count++
		}
	}

	if count > 0 {
		gm.saveAsync()
	}

	return count
}

// GuaranteeCount returns the total number of guarantees
func (gm *GuaranteeManager) GuaranteeCount() int {
	gm.mu.RLock()
	defer gm.mu.RUnlock()
	return len(gm.guarantees)
}

// ActiveGuaranteeCount returns the number of active guarantees
func (gm *GuaranteeManager) ActiveGuaranteeCount() int {
	gm.mu.RLock()
	defer gm.mu.RUnlock()

	count := 0
	for _, g := range gm.guarantees {
		if g.Status == GuaranteeStatusActive {
			count++
		}
	}
	return count
}

// guaranteeData is used for serialization
type guaranteeData struct {
	Guarantees  []*Guarantee         `json:"guarantees"`
	Liabilities []*LiabilityRecord   `json:"liabilities"`
	Invitations []InvitationRecord   `json:"invitations"`
	SavedAt     int64                `json:"saved_at"`
}

// save persists data to disk
func (gm *GuaranteeManager) save() error {
	if gm.dataDir == "" {
		return nil
	}

	guarantees := make([]*Guarantee, 0, len(gm.guarantees))
	for _, g := range gm.guarantees {
		guarantees = append(guarantees, g)
	}

	liabilities := make([]*LiabilityRecord, 0, len(gm.liabilities))
	for _, l := range gm.liabilities {
		liabilities = append(liabilities, l)
	}

	data := guaranteeData{
		Guarantees:  guarantees,
		Liabilities: liabilities,
		Invitations: gm.invitations,
		SavedAt:     time.Now().Unix(),
	}

	bytes, err := json.MarshalIndent(data, "", "  ")
	if err != nil {
		return err
	}

	filePath := filepath.Join(gm.dataDir, "guarantees.json")
	return os.WriteFile(filePath, bytes, 0644)
}

// saveAsync saves data asynchronously
func (gm *GuaranteeManager) saveAsync() {
	go func() {
		if err := gm.save(); err != nil {
			fmt.Printf("Warning: failed to save guarantees: %v\n", err)
		}
	}()
}

// load loads data from disk
func (gm *GuaranteeManager) load() error {
	if gm.dataDir == "" {
		return nil
	}

	filePath := filepath.Join(gm.dataDir, "guarantees.json")
	bytes, err := os.ReadFile(filePath)
	if err != nil {
		return err
	}

	var data guaranteeData
	if err := json.Unmarshal(bytes, &data); err != nil {
		return err
	}

	// Rebuild maps
	for _, g := range data.Guarantees {
		gm.guarantees[g.ID] = g
		gm.byNode[g.NewNodeID] = append(gm.byNode[g.NewNodeID], g.ID)
		gm.bySponsor[g.SponsorID] = append(gm.bySponsor[g.SponsorID], g.ID)
	}

	for _, l := range data.Liabilities {
		gm.liabilities[l.ID] = l
	}

	gm.invitations = data.Invitations

	return nil
}

// Reset clears all data (for testing)
func (gm *GuaranteeManager) Reset() {
	gm.mu.Lock()
	defer gm.mu.Unlock()

	gm.guarantees = make(map[string]*Guarantee)
	gm.byNode = make(map[string][]string)
	gm.bySponsor = make(map[string][]string)
	gm.liabilities = make(map[string]*LiabilityRecord)
	gm.invitations = make([]InvitationRecord, 0)
}
