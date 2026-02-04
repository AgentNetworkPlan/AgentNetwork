package consensus

import (
	"testing"
	"time"
)

func TestNewConsensusManager(t *testing.T) {
	cm := NewConsensusManager("node1")
	if cm == nil {
		t.Fatal("Failed to create consensus manager")
	}
	if cm.nodeID != "node1" {
		t.Errorf("Expected nodeID node1, got %s", cm.nodeID)
	}
}

func TestSetCommittee(t *testing.T) {
	cm := NewConsensusManager("node1")

	members := []*CommitteeMember{
		{NodeID: "node1", Reputation: 50.0},
		{NodeID: "node2", Reputation: 60.0},
		{NodeID: "node3", Reputation: 70.0},
	}

	cm.SetCommittee(members)

	committee := cm.GetCommittee()
	if committee.Size() != 3 {
		t.Errorf("Expected committee size 3, got %d", committee.Size())
	}

	leader := committee.GetLeader()
	if leader == nil {
		t.Fatal("Leader should not be nil")
	}
	if leader.NodeID != "node1" {
		t.Errorf("Expected leader node1, got %s", leader.NodeID)
	}
}

func TestIsCommitteeMember(t *testing.T) {
	cm := NewConsensusManager("node1")

	members := []*CommitteeMember{
		{NodeID: "node1", Reputation: 50.0},
		{NodeID: "node2", Reputation: 60.0},
	}
	cm.SetCommittee(members)

	if !cm.IsCommitteeMember() {
		t.Error("node1 should be a committee member")
	}

	cm2 := NewConsensusManager("node3")
	cm2.SetCommittee(members)
	if cm2.IsCommitteeMember() {
		t.Error("node3 should not be a committee member")
	}
}

func TestIsLeader(t *testing.T) {
	cm := NewConsensusManager("node1")

	members := []*CommitteeMember{
		{NodeID: "node1", Reputation: 50.0},
		{NodeID: "node2", Reputation: 60.0},
	}
	cm.SetCommittee(members)

	if !cm.IsLeader() {
		t.Error("node1 should be the leader")
	}

	cm2 := NewConsensusManager("node2")
	cm2.SetCommittee(members)
	if cm2.IsLeader() {
		t.Error("node2 should not be the leader initially")
	}
}

func TestRotateLeader(t *testing.T) {
	cm := NewConsensusManager("node1")

	members := []*CommitteeMember{
		{NodeID: "node1", Reputation: 50.0},
		{NodeID: "node2", Reputation: 60.0},
		{NodeID: "node3", Reputation: 70.0},
	}
	cm.SetCommittee(members)

	// Initial leader is node1
	leader := cm.GetCommittee().GetLeader()
	if leader.NodeID != "node1" {
		t.Errorf("Initial leader should be node1")
	}

	// Rotate
	cm.RotateLeader()
	leader = cm.GetCommittee().GetLeader()
	if leader.NodeID != "node2" {
		t.Errorf("After rotation, leader should be node2")
	}

	// Rotate again
	cm.RotateLeader()
	leader = cm.GetCommittee().GetLeader()
	if leader.NodeID != "node3" {
		t.Errorf("After second rotation, leader should be node3")
	}

	// Rotate wraps around
	cm.RotateLeader()
	leader = cm.GetCommittee().GetLeader()
	if leader.NodeID != "node1" {
		t.Errorf("Rotation should wrap around to node1")
	}
}

func TestProposeJoin(t *testing.T) {
	cm := NewConsensusManager("node1")

	data := &JoinProposalData{
		NewNodeID:    "newnode",
		NewNodePubKey: "pubkey",
		SponsorID:    "sponsor",
		GuaranteeID:  "g1",
		InitialRep:   1.0,
	}

	proposal, err := cm.ProposeJoin(data)
	if err != nil {
		t.Fatalf("Failed to create proposal: %v", err)
	}

	if proposal.ID == "" {
		t.Error("Proposal ID should not be empty")
	}
	if proposal.Type != ProposalJoin {
		t.Errorf("Expected type JOIN, got %s", proposal.Type)
	}
	if proposal.Phase != PhasePending {
		t.Errorf("Initial phase should be pending")
	}
}

func TestProposeKick(t *testing.T) {
	cm := NewConsensusManager("node1")

	data := &KickProposalData{
		NodeID:     "badnode",
		Reason:     "violation",
		Evidence:   "evidence",
		ReporterID: "reporter",
	}

	proposal, err := cm.ProposeKick(data)
	if err != nil {
		t.Fatalf("Failed to create proposal: %v", err)
	}

	if proposal.Type != ProposalKick {
		t.Errorf("Expected type KICK, got %s", proposal.Type)
	}
}

func TestVote(t *testing.T) {
	cm := NewConsensusManager("node1")

	members := []*CommitteeMember{
		{NodeID: "node1", Reputation: 50.0},
		{NodeID: "node2", Reputation: 60.0},
		{NodeID: "node3", Reputation: 70.0},
	}
	cm.SetCommittee(members)

	proposal, _ := cm.ProposeJoin(&JoinProposalData{NewNodeID: "new"})

	// Vote
	err := cm.Vote(proposal.ID, true, "approved")
	if err != nil {
		t.Fatalf("Failed to vote: %v", err)
	}

	p := cm.GetProposal(proposal.ID)
	if len(p.Votes) != 1 {
		t.Errorf("Expected 1 vote, got %d", len(p.Votes))
	}
}

func TestVoteNotCommitteeMember(t *testing.T) {
	cm := NewConsensusManager("outsider")

	members := []*CommitteeMember{
		{NodeID: "node1", Reputation: 50.0},
		{NodeID: "node2", Reputation: 60.0},
	}
	cm.SetCommittee(members)

	proposal, _ := cm.ProposeJoin(&JoinProposalData{NewNodeID: "new"})

	err := cm.Vote(proposal.ID, true, "approved")
	if err == nil {
		t.Error("Non-committee member should not be able to vote")
	}
}

func TestVoteAlreadyVoted(t *testing.T) {
	cm := NewConsensusManager("node1")

	members := []*CommitteeMember{
		{NodeID: "node1", Reputation: 50.0},
	}
	cm.SetCommittee(members)

	proposal, _ := cm.ProposeJoin(&JoinProposalData{NewNodeID: "new"})
	cm.Vote(proposal.ID, true, "first vote")

	err := cm.Vote(proposal.ID, true, "second vote")
	if err == nil {
		t.Error("Should not allow double voting")
	}
}

func TestConsensusReached(t *testing.T) {
	cm := NewConsensusManager("node1")

	members := []*CommitteeMember{
		{NodeID: "node1", Reputation: 50.0},
		{NodeID: "node2", Reputation: 60.0},
		{NodeID: "node3", Reputation: 70.0},
	}
	cm.SetCommittee(members)

	proposal, _ := cm.ProposeJoin(&JoinProposalData{NewNodeID: "new"})

	// Need 2/3 = 2 votes to pass
	cm.Vote(proposal.ID, true, "yes")

	// Create another manager for second vote
	cm2 := NewConsensusManager("node2")
	cm2.SetCommittee(members)

	// Manually add vote to the same proposal
	cm.proposals[proposal.ID].Votes["node2"] = &Vote{
		VoterID:   "node2",
		Vote:      true,
		Timestamp: time.Now().Unix(),
	}

	// Trigger consensus check
	cm.checkConsensus(cm.proposals[proposal.ID])

	result := cm.GetProposalResult(proposal.ID)
	if result == nil {
		t.Fatal("Result should not be nil")
	}
	if !result.Passed {
		t.Error("Proposal should pass with 2/3 majority")
	}
	if result.AgreeCount != 2 {
		t.Errorf("Expected 2 agree votes, got %d", result.AgreeCount)
	}
}

func TestConsensusRejected(t *testing.T) {
	cm := NewConsensusManager("node1")

	members := []*CommitteeMember{
		{NodeID: "node1", Reputation: 50.0},
		{NodeID: "node2", Reputation: 60.0},
		{NodeID: "node3", Reputation: 70.0},
	}
	cm.SetCommittee(members)

	proposal, _ := cm.ProposeJoin(&JoinProposalData{NewNodeID: "new"})

	// Add 2 disagree votes (can't reach quorum)
	cm.proposals[proposal.ID].Votes["node1"] = &Vote{VoterID: "node1", Vote: false}
	cm.proposals[proposal.ID].Votes["node2"] = &Vote{VoterID: "node2", Vote: false}

	cm.checkConsensus(cm.proposals[proposal.ID])

	result := cm.GetProposalResult(proposal.ID)
	if result == nil {
		t.Fatal("Result should not be nil")
	}
	if result.Passed {
		t.Error("Proposal should be rejected")
	}
}

func TestGetPendingProposals(t *testing.T) {
	cm := NewConsensusManager("node1")

	cm.ProposeJoin(&JoinProposalData{NewNodeID: "new1"})
	p2, _ := cm.ProposeJoin(&JoinProposalData{NewNodeID: "new2"})

	// Finalize one
	cm.proposals[p2.ID].Phase = PhaseFinalized

	pending := cm.GetPendingProposals()
	if len(pending) != 1 {
		t.Errorf("Expected 1 pending, got %d", len(pending))
	}
}

func TestCheckTimeouts(t *testing.T) {
	cm := NewConsensusManager("node1")

	members := []*CommitteeMember{
		{NodeID: "node1", Reputation: 50.0},
	}
	cm.SetCommittee(members)

	proposal, _ := cm.ProposeJoin(&JoinProposalData{NewNodeID: "new"})

	// Set deadline to past
	cm.proposals[proposal.ID].Deadline = time.Now().Add(-1 * time.Hour).Unix()

	count := cm.CheckTimeouts()
	if count != 1 {
		t.Errorf("Expected 1 timeout, got %d", count)
	}

	p := cm.GetProposal(proposal.ID)
	if p.Phase != PhaseTimeout {
		t.Errorf("Phase should be timeout")
	}
}

func TestProposalCount(t *testing.T) {
	cm := NewConsensusManager("node1")

	if cm.ProposalCount() != 0 {
		t.Error("Initial count should be 0")
	}

	cm.ProposeJoin(&JoinProposalData{NewNodeID: "new1"})
	cm.ProposeJoin(&JoinProposalData{NewNodeID: "new2"})

	if cm.ProposalCount() != 2 {
		t.Errorf("Expected 2 proposals, got %d", cm.ProposalCount())
	}
}

func TestCleanupOldProposals(t *testing.T) {
	cm := NewConsensusManager("node1")

	p1, _ := cm.ProposeJoin(&JoinProposalData{NewNodeID: "new1"})
	p2, _ := cm.ProposeJoin(&JoinProposalData{NewNodeID: "new2"})

	// Finalize and make old
	cm.proposals[p1.ID].Phase = PhaseFinalized
	cm.proposals[p1.ID].CreatedAt = time.Now().Add(-2 * time.Hour).Unix()

	cm.proposals[p2.ID].Phase = PhaseFinalized
	cm.proposals[p2.ID].CreatedAt = time.Now().Unix() // Recent

	count := cm.CleanupOldProposals(1 * time.Hour)
	if count != 1 {
		t.Errorf("Expected 1 cleanup, got %d", count)
	}

	if cm.ProposalCount() != 1 {
		t.Errorf("Expected 1 remaining proposal, got %d", cm.ProposalCount())
	}
}

func TestCommitteeQuorumSize(t *testing.T) {
	tests := []struct {
		size     int
		expected int
	}{
		{3, 2},  // 3 * 0.67 = 2.01 -> 2
		{5, 3},  // 5 * 0.67 = 3.35 -> 3
		{7, 4},  // 7 * 0.67 = 4.69 -> 4
		{11, 7}, // 11 * 0.67 = 7.37 -> 7
	}

	for _, tt := range tests {
		members := make([]*CommitteeMember, tt.size)
		for i := 0; i < tt.size; i++ {
			members[i] = &CommitteeMember{NodeID: string(rune('A' + i))}
		}

		c := &Committee{Members: members}
		quorum := c.QuorumSize()
		if quorum != tt.expected {
			t.Errorf("For size %d: expected quorum %d, got %d", tt.size, tt.expected, quorum)
		}
	}
}

func TestReset(t *testing.T) {
	cm := NewConsensusManager("node1")

	cm.ProposeJoin(&JoinProposalData{NewNodeID: "new1"})
	cm.ProposeJoin(&JoinProposalData{NewNodeID: "new2"})

	cm.Reset()

	if cm.ProposalCount() != 0 {
		t.Errorf("Expected 0 after reset, got %d", cm.ProposalCount())
	}
}

func TestMaxPendingRequests(t *testing.T) {
	cm := NewConsensusManager("node1")

	// Create max pending requests
	for i := 0; i < MaxPendingRequests; i++ {
		_, err := cm.ProposeJoin(&JoinProposalData{NewNodeID: "new"})
		if err != nil {
			t.Fatalf("Failed at %d: %v", i, err)
		}
	}

	// Next one should fail
	_, err := cm.ProposeJoin(&JoinProposalData{NewNodeID: "new"})
	if err == nil {
		t.Error("Should fail when max pending reached")
	}
}
