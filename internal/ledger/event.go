// Package ledger provides a lightweight event ledger for tracking network events.
// It implements Event Sourcing pattern - storing all events instead of current state.
package ledger

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"time"
)

// EventType defines the type of events that can be recorded
type EventType string

const (
	// Node lifecycle events
	EventNodeJoin  EventType = "NODE_JOIN"  // Node joining the network
	EventNodeLeave EventType = "NODE_LEAVE" // Node leaving the network

	// Reputation events
	EventReputationChange EventType = "REPUTATION_CHANGE" // Reputation modification

	// Consensus events
	EventConsensusDecision EventType = "CONSENSUS_DECISION" // Consensus decision result

	// Violation events
	EventViolation       EventType = "VIOLATION"        // Violation record
	EventLiabilitySettle EventType = "LIABILITY_SETTLE" // Liability settlement

	// Guarantee events
	EventGuaranteeCreate EventType = "GUARANTEE_CREATE" // Guarantee creation
	EventGuaranteeExpire EventType = "GUARANTEE_EXPIRE" // Guarantee expiration
	EventGuaranteeRevoke EventType = "GUARANTEE_REVOKE" // Guarantee revocation

	// Committee events
	EventCommitteeChange EventType = "COMMITTEE_CHANGE" // Committee member change
	EventCommitteeVote   EventType = "COMMITTEE_VOTE"   // Committee voting event
)

// Event represents a single event in the ledger
type Event struct {
	Sequence  uint64          `json:"seq"`        // Global sequence number
	Type      EventType       `json:"type"`       // Event type
	NodeID    string          `json:"node_id"`    // Related node ID
	Data      json.RawMessage `json:"data"`       // Event data (JSON)
	Timestamp int64           `json:"timestamp"`  // Unix timestamp
	SignerID  string          `json:"signer_id"`  // ID of the signer
	Signature string          `json:"signature"`  // SM2/Ed25519 signature
	PrevHash  string          `json:"prev_hash"`  // Hash of previous event (chain verification)
	Hash      string          `json:"hash"`       // Hash of this event
}

// NodeJoinData represents data for NODE_JOIN event
type NodeJoinData struct {
	NodeID            string  `json:"node_id"`
	NodePubKey        string  `json:"node_pubkey"`
	SponsorID         string  `json:"sponsor_id"`
	GuaranteeID       string  `json:"guarantee_id"`
	InitialReputation float64 `json:"initial_reputation"`
}

// NodeLeaveData represents data for NODE_LEAVE event
type NodeLeaveData struct {
	NodeID     string `json:"node_id"`
	Reason     string `json:"reason"`      // voluntary, kicked, timeout
	InitiatorID string `json:"initiator_id"` // Who initiated the leave (self or committee)
}

// ReputationChangeData represents data for REPUTATION_CHANGE event
type ReputationChangeData struct {
	NodeID    string  `json:"node_id"`
	Delta     float64 `json:"delta"`      // Change amount (+/-)
	NewValue  float64 `json:"new_value"`  // New reputation value
	Source    string  `json:"source"`     // task_complete, violation, decay, etc.
	Reference string  `json:"reference"`  // Reference to related event/task
}

// ConsensusDecisionData represents data for CONSENSUS_DECISION event
type ConsensusDecisionData struct {
	ProposalID   string   `json:"proposal_id"`
	ProposalType string   `json:"proposal_type"` // join, kick, etc.
	Result       string   `json:"result"`        // passed, rejected
	AgreeCount   int      `json:"agree_count"`
	TotalVoters  int      `json:"total_voters"`
	VoterIDs     []string `json:"voter_ids"`
}

// ViolationData represents data for VIOLATION event
type ViolationData struct {
	NodeID        string  `json:"node_id"`
	ViolationType string  `json:"violation_type"` // spam, fraud, malicious, etc.
	Severity      string  `json:"severity"`       // minor, moderate, severe, critical
	Penalty       float64 `json:"penalty"`        // Reputation penalty
	Evidence      string  `json:"evidence"`       // Evidence reference
	ReporterID    string  `json:"reporter_id"`    // Who reported
}

// LiabilitySettleData represents data for LIABILITY_SETTLE event
type LiabilitySettleData struct {
	SponsorID       string  `json:"sponsor_id"`
	ViolatorID      string  `json:"violator_id"`
	GuaranteeID     string  `json:"guarantee_id"`
	OriginalPenalty float64 `json:"original_penalty"`
	SponsorPenalty  float64 `json:"sponsor_penalty"`
	LiabilityRatio  float64 `json:"liability_ratio"`
}

// GuaranteeCreateData represents data for GUARANTEE_CREATE event
type GuaranteeCreateData struct {
	GuaranteeID     string  `json:"guarantee_id"`
	SponsorID       string  `json:"sponsor_id"`
	NewNodeID       string  `json:"new_node_id"`
	GuaranteeAmount float64 `json:"guarantee_amount"`
	LiabilityRatio  float64 `json:"liability_ratio"`
	ValidUntil      int64   `json:"valid_until"`
}

// GuaranteeExpireData represents data for GUARANTEE_EXPIRE event
type GuaranteeExpireData struct {
	GuaranteeID string `json:"guarantee_id"`
	SponsorID   string `json:"sponsor_id"`
	NewNodeID   string `json:"new_node_id"`
	Reason      string `json:"reason"` // expired, completed, revoked
}

// CommitteeChangeData represents data for COMMITTEE_CHANGE event
type CommitteeChangeData struct {
	ChangeType   string   `json:"change_type"`   // add, remove, rotate
	MemberID     string   `json:"member_id"`     // Affected member
	NewCommittee []string `json:"new_committee"` // New committee list
	Reason       string   `json:"reason"`
}

// NewEvent creates a new event with the given parameters
func NewEvent(seq uint64, eventType EventType, nodeID string, data interface{}, signerID, prevHash string) (*Event, error) {
	dataBytes, err := json.Marshal(data)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal event data: %w", err)
	}

	event := &Event{
		Sequence:  seq,
		Type:      eventType,
		NodeID:    nodeID,
		Data:      dataBytes,
		Timestamp: time.Now().Unix(),
		SignerID:  signerID,
		PrevHash:  prevHash,
	}

	// Calculate hash
	event.Hash = event.ComputeHash()

	return event, nil
}

// ComputeHash calculates the hash of the event (excluding Signature and Hash fields)
func (e *Event) ComputeHash() string {
	hashData := struct {
		Sequence  uint64          `json:"seq"`
		Type      EventType       `json:"type"`
		NodeID    string          `json:"node_id"`
		Data      json.RawMessage `json:"data"`
		Timestamp int64           `json:"timestamp"`
		SignerID  string          `json:"signer_id"`
		PrevHash  string          `json:"prev_hash"`
	}{
		Sequence:  e.Sequence,
		Type:      e.Type,
		NodeID:    e.NodeID,
		Data:      e.Data,
		Timestamp: e.Timestamp,
		SignerID:  e.SignerID,
		PrevHash:  e.PrevHash,
	}

	bytes, _ := json.Marshal(hashData)
	hash := sha256.Sum256(bytes)
	return hex.EncodeToString(hash[:])
}

// GetData unmarshals the event data into the provided struct
func (e *Event) GetData(v interface{}) error {
	return json.Unmarshal(e.Data, v)
}

// Validate performs basic validation on the event
func (e *Event) Validate() error {
	if e.Sequence == 0 {
		return fmt.Errorf("sequence number cannot be 0")
	}
	if e.Type == "" {
		return fmt.Errorf("event type cannot be empty")
	}
	if e.NodeID == "" {
		return fmt.Errorf("node ID cannot be empty")
	}
	if e.SignerID == "" {
		return fmt.Errorf("signer ID cannot be empty")
	}
	if e.Timestamp == 0 {
		return fmt.Errorf("timestamp cannot be 0")
	}
	if len(e.Data) == 0 {
		return fmt.Errorf("event data cannot be empty")
	}

	// Verify hash
	expectedHash := e.ComputeHash()
	if e.Hash != expectedHash {
		return fmt.Errorf("hash mismatch: expected %s, got %s", expectedHash, e.Hash)
	}

	return nil
}

// String returns a human-readable representation of the event
func (e *Event) String() string {
	return fmt.Sprintf("Event{seq=%d, type=%s, node=%s, time=%d}",
		e.Sequence, e.Type, e.NodeID, e.Timestamp)
}
