// Package consensus provides a simplified PBFT-like consensus mechanism
// for committee-based decision making in the AgentNetwork.
package consensus

import (
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"sync"
	"time"
)

// Constants for consensus
const (
	// Committee parameters
	MinCommitteeSize    = 4     // Minimum committee members
	DefaultCommitteeSize = 5    // Default committee size
	MaxCommitteeSize    = 11    // Maximum committee size

	// Consensus parameters
	ConsensusQuorum     = 0.67  // 2/3 majority required
	ConsensusTimeout    = 30 * time.Second  // Timeout for consensus
	MaxPendingRequests  = 100   // Maximum pending requests

	// Rotation parameters
	RotationInterval    = 100   // Rotate committee every N events
)

// ProposalType defines types of consensus proposals
type ProposalType string

const (
	ProposalJoin      ProposalType = "JOIN"       // Node join request
	ProposalKick      ProposalType = "KICK"       // Kick node request
	ProposalSuspend   ProposalType = "SUSPEND"    // Suspend node request
	ProposalParameter ProposalType = "PARAMETER"  // Parameter change
	ProposalEmergency ProposalType = "EMERGENCY"  // Emergency action
)

// ConsensusPhase represents the phase of PBFT consensus
type ConsensusPhase string

const (
	PhasePending    ConsensusPhase = "PENDING"     // Initial state
	PhasePrePrepare ConsensusPhase = "PRE_PREPARE" // Leader broadcast
	PhasePrepare    ConsensusPhase = "PREPARE"     // Collecting prepare votes
	PhaseCommit     ConsensusPhase = "COMMIT"      // Collecting commit votes
	PhaseFinalized  ConsensusPhase = "FINALIZED"   // Consensus reached
	PhaseTimeout    ConsensusPhase = "TIMEOUT"     // Timed out
	PhaseRejected   ConsensusPhase = "REJECTED"    // Rejected
)

// Vote represents a consensus vote
type Vote struct {
	VoterID   string `json:"voter_id"`   // Voter node ID
	Vote      bool   `json:"vote"`       // true = agree, false = disagree
	Reason    string `json:"reason"`     // Optional reason
	Timestamp int64  `json:"timestamp"`  // Vote timestamp
	Signature string `json:"signature"`  // Voter's signature
}

// Proposal represents a consensus proposal
type Proposal struct {
	ID          string        `json:"id"`           // Proposal ID
	Type        ProposalType  `json:"type"`         // Proposal type
	Data        interface{}   `json:"data"`         // Proposal data
	ProposerID  string        `json:"proposer_id"`  // Who proposed
	CreatedAt   int64         `json:"created_at"`   // Creation timestamp
	Deadline    int64         `json:"deadline"`     // Voting deadline

	// State
	Phase       ConsensusPhase `json:"phase"`
	Votes       map[string]*Vote `json:"votes"`  // voterID -> vote

	// Result
	Result      *ConsensusResult `json:"result,omitempty"`
}

// ConsensusResult represents the result of a consensus
type ConsensusResult struct {
	ProposalID    string   `json:"proposal_id"`
	Passed        bool     `json:"passed"`
	AgreeCount    int      `json:"agree_count"`
	DisagreeCount int      `json:"disagree_count"`
	TotalVoters   int      `json:"total_voters"`
	Quorum        float64  `json:"quorum"`
	FinalizedAt   int64    `json:"finalized_at"`
	Reason        string   `json:"reason,omitempty"`
}

// JoinProposalData contains data for a join proposal
type JoinProposalData struct {
	NewNodeID    string  `json:"new_node_id"`
	NewNodePubKey string `json:"new_node_pubkey"`
	SponsorID    string  `json:"sponsor_id"`
	GuaranteeID  string  `json:"guarantee_id"`
	InitialRep   float64 `json:"initial_rep"`
}

// KickProposalData contains data for a kick proposal
type KickProposalData struct {
	NodeID     string `json:"node_id"`
	Reason     string `json:"reason"`
	Evidence   string `json:"evidence"`
	ReporterID string `json:"reporter_id"`
}

// CommitteeMember represents a committee member
type CommitteeMember struct {
	NodeID     string  `json:"node_id"`
	PubKey     string  `json:"pubkey"`
	Reputation float64 `json:"reputation"`
	JoinedAt   int64   `json:"joined_at"`  // When joined committee
	IsLeader   bool    `json:"is_leader"`  // Current leader
}

// Committee represents the current consensus committee
type Committee struct {
	Members     []*CommitteeMember `json:"members"`
	LeaderIndex int                `json:"leader_index"`
	RotationSeq uint64             `json:"rotation_seq"` // Last rotation sequence
	UpdatedAt   int64              `json:"updated_at"`
}

// GetLeader returns the current leader
func (c *Committee) GetLeader() *CommitteeMember {
	if len(c.Members) == 0 || c.LeaderIndex >= len(c.Members) {
		return nil
	}
	return c.Members[c.LeaderIndex]
}

// Size returns committee size
func (c *Committee) Size() int {
	return len(c.Members)
}

// IsMember checks if a node is a committee member
func (c *Committee) IsMember(nodeID string) bool {
	for _, m := range c.Members {
		if m.NodeID == nodeID {
			return true
		}
	}
	return false
}

// QuorumSize returns the required quorum size
func (c *Committee) QuorumSize() int {
	size := len(c.Members)
	quorum := int(float64(size) * ConsensusQuorum)
	if quorum < 1 {
		quorum = 1
	}
	return quorum
}

// ConsensusManager manages consensus operations
type ConsensusManager struct {
	nodeID     string
	committee  *Committee
	proposals  map[string]*Proposal  // proposalID -> Proposal
	
	// Callbacks
	signFunc   func(data []byte) (string, error)
	verifyFunc func(nodeID string, data []byte, signature string) bool
	broadcastFunc func(msg *ConsensusMessage) error
	getReputation func(nodeID string) float64

	mu sync.RWMutex
}

// NewConsensusManager creates a new consensus manager
func NewConsensusManager(nodeID string) *ConsensusManager {
	return &ConsensusManager{
		nodeID:    nodeID,
		committee: &Committee{
			Members:     make([]*CommitteeMember, 0),
			LeaderIndex: 0,
			UpdatedAt:   time.Now().Unix(),
		},
		proposals: make(map[string]*Proposal),
	}
}

// SetSignFunc sets the signing function
func (cm *ConsensusManager) SetSignFunc(fn func(data []byte) (string, error)) {
	cm.mu.Lock()
	defer cm.mu.Unlock()
	cm.signFunc = fn
}

// SetVerifyFunc sets the verification function
func (cm *ConsensusManager) SetVerifyFunc(fn func(nodeID string, data []byte, signature string) bool) {
	cm.mu.Lock()
	defer cm.mu.Unlock()
	cm.verifyFunc = fn
}

// SetBroadcastFunc sets the broadcast function
func (cm *ConsensusManager) SetBroadcastFunc(fn func(msg *ConsensusMessage) error) {
	cm.mu.Lock()
	defer cm.mu.Unlock()
	cm.broadcastFunc = fn
}

// SetReputationFunc sets the reputation getter function
func (cm *ConsensusManager) SetReputationFunc(fn func(nodeID string) float64) {
	cm.mu.Lock()
	defer cm.mu.Unlock()
	cm.getReputation = fn
}

// generateID generates a random ID
func generateID() string {
	bytes := make([]byte, 16)
	rand.Read(bytes)
	return hex.EncodeToString(bytes)
}

// SetCommittee sets the committee members
func (cm *ConsensusManager) SetCommittee(members []*CommitteeMember) {
	cm.mu.Lock()
	defer cm.mu.Unlock()

	cm.committee = &Committee{
		Members:     members,
		LeaderIndex: 0,
		UpdatedAt:   time.Now().Unix(),
	}

	if len(members) > 0 {
		members[0].IsLeader = true
	}
}

// GetCommittee returns the current committee
func (cm *ConsensusManager) GetCommittee() *Committee {
	cm.mu.RLock()
	defer cm.mu.RUnlock()
	return cm.committee
}

// IsCommitteeMember checks if this node is a committee member
func (cm *ConsensusManager) IsCommitteeMember() bool {
	cm.mu.RLock()
	defer cm.mu.RUnlock()
	return cm.committee.IsMember(cm.nodeID)
}

// IsLeader checks if this node is the current leader
func (cm *ConsensusManager) IsLeader() bool {
	cm.mu.RLock()
	defer cm.mu.RUnlock()
	leader := cm.committee.GetLeader()
	return leader != nil && leader.NodeID == cm.nodeID
}

// RotateLeader rotates to the next leader
func (cm *ConsensusManager) RotateLeader() {
	cm.mu.Lock()
	defer cm.mu.Unlock()

	if len(cm.committee.Members) == 0 {
		return
	}

	// Unset current leader
	if cm.committee.LeaderIndex < len(cm.committee.Members) {
		cm.committee.Members[cm.committee.LeaderIndex].IsLeader = false
	}

	// Move to next leader
	cm.committee.LeaderIndex = (cm.committee.LeaderIndex + 1) % len(cm.committee.Members)
	cm.committee.Members[cm.committee.LeaderIndex].IsLeader = true
	cm.committee.UpdatedAt = time.Now().Unix()
}

// ProposeJoin creates a join proposal
func (cm *ConsensusManager) ProposeJoin(data *JoinProposalData) (*Proposal, error) {
	return cm.createProposal(ProposalJoin, data)
}

// ProposeKick creates a kick proposal
func (cm *ConsensusManager) ProposeKick(data *KickProposalData) (*Proposal, error) {
	return cm.createProposal(ProposalKick, data)
}

// createProposal creates a new proposal
func (cm *ConsensusManager) createProposal(propType ProposalType, data interface{}) (*Proposal, error) {
	cm.mu.Lock()
	defer cm.mu.Unlock()

	// Check pending count
	pendingCount := 0
	for _, p := range cm.proposals {
		if p.Phase != PhaseFinalized && p.Phase != PhaseRejected && p.Phase != PhaseTimeout {
			pendingCount++
		}
	}
	if pendingCount >= MaxPendingRequests {
		return nil, fmt.Errorf("too many pending proposals (%d)", pendingCount)
	}

	now := time.Now()
	proposal := &Proposal{
		ID:         generateID(),
		Type:       propType,
		Data:       data,
		ProposerID: cm.nodeID,
		CreatedAt:  now.Unix(),
		Deadline:   now.Add(ConsensusTimeout).Unix(),
		Phase:      PhasePending,
		Votes:      make(map[string]*Vote),
	}

	cm.proposals[proposal.ID] = proposal

	return proposal, nil
}

// Vote submits a vote for a proposal
func (cm *ConsensusManager) Vote(proposalID string, agree bool, reason string) error {
	cm.mu.Lock()
	defer cm.mu.Unlock()

	// Check if committee member
	if !cm.committee.IsMember(cm.nodeID) {
		return fmt.Errorf("not a committee member")
	}

	proposal, ok := cm.proposals[proposalID]
	if !ok {
		return fmt.Errorf("proposal not found: %s", proposalID)
	}

	// Check if already voted
	if _, exists := proposal.Votes[cm.nodeID]; exists {
		return fmt.Errorf("already voted")
	}

	// Check deadline
	if time.Now().Unix() > proposal.Deadline {
		proposal.Phase = PhaseTimeout
		return fmt.Errorf("voting deadline passed")
	}

	// Create vote
	vote := &Vote{
		VoterID:   cm.nodeID,
		Vote:      agree,
		Reason:    reason,
		Timestamp: time.Now().Unix(),
	}

	// Sign vote
	if cm.signFunc != nil {
		voteData := fmt.Sprintf("%s:%s:%v", proposalID, cm.nodeID, agree)
		sig, err := cm.signFunc([]byte(voteData))
		if err != nil {
			return fmt.Errorf("failed to sign vote: %w", err)
		}
		vote.Signature = sig
	}

	proposal.Votes[cm.nodeID] = vote

	// Check if consensus reached
	cm.checkConsensus(proposal)

	return nil
}

// checkConsensus checks if consensus has been reached
func (cm *ConsensusManager) checkConsensus(proposal *Proposal) {
	agreeCount := 0
	disagreeCount := 0

	for _, vote := range proposal.Votes {
		if vote.Vote {
			agreeCount++
		} else {
			disagreeCount++
		}
	}

	totalVoters := len(cm.committee.Members)
	quorum := cm.committee.QuorumSize()

	// Check if passed
	if agreeCount >= quorum {
		proposal.Phase = PhaseFinalized
		proposal.Result = &ConsensusResult{
			ProposalID:    proposal.ID,
			Passed:        true,
			AgreeCount:    agreeCount,
			DisagreeCount: disagreeCount,
			TotalVoters:   totalVoters,
			Quorum:        ConsensusQuorum,
			FinalizedAt:   time.Now().Unix(),
			Reason:        "quorum reached",
		}
		return
	}

	// Check if definitely rejected (not enough remaining votes)
	remaining := totalVoters - len(proposal.Votes)
	if agreeCount+remaining < quorum {
		proposal.Phase = PhaseRejected
		proposal.Result = &ConsensusResult{
			ProposalID:    proposal.ID,
			Passed:        false,
			AgreeCount:    agreeCount,
			DisagreeCount: disagreeCount,
			TotalVoters:   totalVoters,
			Quorum:        ConsensusQuorum,
			FinalizedAt:   time.Now().Unix(),
			Reason:        "cannot reach quorum",
		}
	}
}

// GetProposal returns a proposal by ID
func (cm *ConsensusManager) GetProposal(id string) *Proposal {
	cm.mu.RLock()
	defer cm.mu.RUnlock()
	return cm.proposals[id]
}

// GetProposalResult returns the result of a proposal
func (cm *ConsensusManager) GetProposalResult(id string) *ConsensusResult {
	cm.mu.RLock()
	defer cm.mu.RUnlock()

	proposal := cm.proposals[id]
	if proposal == nil {
		return nil
	}
	return proposal.Result
}

// GetPendingProposals returns all pending proposals
func (cm *ConsensusManager) GetPendingProposals() []*Proposal {
	cm.mu.RLock()
	defer cm.mu.RUnlock()

	var result []*Proposal
	for _, p := range cm.proposals {
		if p.Phase != PhaseFinalized && p.Phase != PhaseRejected && p.Phase != PhaseTimeout {
			result = append(result, p)
		}
	}
	return result
}

// CheckTimeouts checks for timed out proposals
func (cm *ConsensusManager) CheckTimeouts() int {
	cm.mu.Lock()
	defer cm.mu.Unlock()

	count := 0
	now := time.Now().Unix()

	for _, p := range cm.proposals {
		if p.Phase != PhaseFinalized && p.Phase != PhaseRejected && p.Phase != PhaseTimeout {
			if now > p.Deadline {
				p.Phase = PhaseTimeout
				p.Result = &ConsensusResult{
					ProposalID:    p.ID,
					Passed:        false,
					AgreeCount:    cm.countVotes(p, true),
					DisagreeCount: cm.countVotes(p, false),
					TotalVoters:   len(cm.committee.Members),
					Quorum:        ConsensusQuorum,
					FinalizedAt:   now,
					Reason:        "timeout",
				}
				count++
			}
		}
	}
	return count
}

func (cm *ConsensusManager) countVotes(p *Proposal, agree bool) int {
	count := 0
	for _, v := range p.Votes {
		if v.Vote == agree {
			count++
		}
	}
	return count
}

// ProposalCount returns the total number of proposals
func (cm *ConsensusManager) ProposalCount() int {
	cm.mu.RLock()
	defer cm.mu.RUnlock()
	return len(cm.proposals)
}

// CleanupOldProposals removes old finalized proposals
func (cm *ConsensusManager) CleanupOldProposals(maxAge time.Duration) int {
	cm.mu.Lock()
	defer cm.mu.Unlock()

	cutoff := time.Now().Add(-maxAge).Unix()
	count := 0

	for id, p := range cm.proposals {
		if (p.Phase == PhaseFinalized || p.Phase == PhaseRejected || p.Phase == PhaseTimeout) &&
			p.CreatedAt < cutoff {
			delete(cm.proposals, id)
			count++
		}
	}
	return count
}

// Reset clears all proposals (for testing)
func (cm *ConsensusManager) Reset() {
	cm.mu.Lock()
	defer cm.mu.Unlock()
	cm.proposals = make(map[string]*Proposal)
}
