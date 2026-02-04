package consensus

import (
	"encoding/json"
	"time"
)

// MessageType defines types of consensus messages
type MessageType string

const (
	MsgPrePrepare MessageType = "PRE_PREPARE"  // Leader broadcasts proposal
	MsgPrepare    MessageType = "PREPARE"      // Prepare vote
	MsgCommit     MessageType = "COMMIT"       // Commit vote
	MsgReply      MessageType = "REPLY"        // Reply to proposer
	MsgViewChange MessageType = "VIEW_CHANGE"  // Leader change
)

// ConsensusMessage represents a message in the consensus protocol
type ConsensusMessage struct {
	Type       MessageType `json:"type"`
	ProposalID string      `json:"proposal_id"`
	Sequence   uint64      `json:"sequence"`
	View       uint64      `json:"view"`       // Current view (leader round)
	SenderID   string      `json:"sender_id"`
	Timestamp  int64       `json:"timestamp"`
	Signature  string      `json:"signature"`
	Payload    []byte      `json:"payload"`    // Proposal or vote data
}

// NewPrePrepareMessage creates a pre-prepare message
func NewPrePrepareMessage(proposal *Proposal, seq, view uint64, senderID string) (*ConsensusMessage, error) {
	payload, err := json.Marshal(proposal)
	if err != nil {
		return nil, err
	}

	return &ConsensusMessage{
		Type:       MsgPrePrepare,
		ProposalID: proposal.ID,
		Sequence:   seq,
		View:       view,
		SenderID:   senderID,
		Timestamp:  time.Now().Unix(),
		Payload:    payload,
	}, nil
}

// NewPrepareMessage creates a prepare message
func NewPrepareMessage(proposalID string, seq, view uint64, senderID string, vote *Vote) (*ConsensusMessage, error) {
	payload, err := json.Marshal(vote)
	if err != nil {
		return nil, err
	}

	return &ConsensusMessage{
		Type:       MsgPrepare,
		ProposalID: proposalID,
		Sequence:   seq,
		View:       view,
		SenderID:   senderID,
		Timestamp:  time.Now().Unix(),
		Payload:    payload,
	}, nil
}

// NewCommitMessage creates a commit message
func NewCommitMessage(proposalID string, seq, view uint64, senderID string) *ConsensusMessage {
	return &ConsensusMessage{
		Type:       MsgCommit,
		ProposalID: proposalID,
		Sequence:   seq,
		View:       view,
		SenderID:   senderID,
		Timestamp:  time.Now().Unix(),
	}
}

// GetProposal extracts the proposal from a pre-prepare message
func (m *ConsensusMessage) GetProposal() (*Proposal, error) {
	if m.Type != MsgPrePrepare {
		return nil, nil
	}

	var proposal Proposal
	if err := json.Unmarshal(m.Payload, &proposal); err != nil {
		return nil, err
	}
	return &proposal, nil
}

// GetVote extracts the vote from a prepare message
func (m *ConsensusMessage) GetVote() (*Vote, error) {
	if m.Type != MsgPrepare {
		return nil, nil
	}

	var vote Vote
	if err := json.Unmarshal(m.Payload, &vote); err != nil {
		return nil, err
	}
	return &vote, nil
}

// PBFTState tracks the state of PBFT consensus for a proposal
type PBFTState struct {
	ProposalID      string         `json:"proposal_id"`
	Phase           ConsensusPhase `json:"phase"`
	View            uint64         `json:"view"`
	Sequence        uint64         `json:"sequence"`
	
	// Message tracking
	PrePrepareRecv  bool           `json:"pre_prepare_recv"`
	PrepareCount    int            `json:"prepare_count"`
	CommitCount     int            `json:"commit_count"`
	
	// Quorum tracking
	PrepareQuorum   bool           `json:"prepare_quorum"`
	CommitQuorum    bool           `json:"commit_quorum"`
	
	// Tracking who sent what
	PrepareVotes    map[string]*Vote `json:"prepare_votes"`
	CommitVotes     map[string]bool  `json:"commit_votes"`
	
	// Timestamps
	CreatedAt       int64          `json:"created_at"`
	PrepareAt       int64          `json:"prepare_at"`
	CommitAt        int64          `json:"commit_at"`
	FinalizedAt     int64          `json:"finalized_at"`
}

// NewPBFTState creates a new PBFT state tracker
func NewPBFTState(proposalID string, seq, view uint64) *PBFTState {
	return &PBFTState{
		ProposalID:    proposalID,
		Phase:         PhasePending,
		View:          view,
		Sequence:      seq,
		PrepareVotes:  make(map[string]*Vote),
		CommitVotes:   make(map[string]bool),
		CreatedAt:     time.Now().Unix(),
	}
}

// AddPrepareVote adds a prepare vote
func (s *PBFTState) AddPrepareVote(voterID string, vote *Vote, quorumSize int) bool {
	if _, exists := s.PrepareVotes[voterID]; exists {
		return false // Already voted
	}

	s.PrepareVotes[voterID] = vote
	s.PrepareCount = len(s.PrepareVotes)

	if s.PrepareCount >= quorumSize && !s.PrepareQuorum {
		s.PrepareQuorum = true
		s.PrepareAt = time.Now().Unix()
		s.Phase = PhasePrepare
		return true // Quorum reached
	}
	return false
}

// AddCommitVote adds a commit vote
func (s *PBFTState) AddCommitVote(voterID string, quorumSize int) bool {
	if s.CommitVotes[voterID] {
		return false // Already committed
	}

	s.CommitVotes[voterID] = true
	s.CommitCount = len(s.CommitVotes)

	if s.CommitCount >= quorumSize && !s.CommitQuorum {
		s.CommitQuorum = true
		s.CommitAt = time.Now().Unix()
		s.Phase = PhaseCommit
		return true // Quorum reached
	}
	return false
}

// IsFinalized checks if consensus is finalized
func (s *PBFTState) IsFinalized() bool {
	return s.Phase == PhaseFinalized
}

// Finalize marks the state as finalized
func (s *PBFTState) Finalize() {
	s.Phase = PhaseFinalized
	s.FinalizedAt = time.Now().Unix()
}
