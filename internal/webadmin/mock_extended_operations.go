package webadmin

import (
	"fmt"
	"time"
)

// MockExtendedOperationsProvider Mock扩展操作提供者（用于测试）
type MockExtendedOperationsProvider struct {
	*MockOperationsProvider
}

// NewMockExtendedOperationsProvider 创建Mock扩展操作提供者
func NewMockExtendedOperationsProvider() *MockExtendedOperationsProvider {
	return &MockExtendedOperationsProvider{
		MockOperationsProvider: NewMockOperationsProvider(),
	}
}

// ========== 声誉扩展 ==========

func (m *MockExtendedOperationsProvider) UpdateReputation(nodeID string, delta int, reason string) (*ReputationUpdateResult, error) {
	return &ReputationUpdateResult{
		NewReputation: 85.0 + float64(delta),
	}, nil
}

func (m *MockExtendedOperationsProvider) GetReputationHistory(nodeID string, limit int) ([]*ReputationHistoryEntry, error) {
	return []*ReputationHistoryEntry{
		{Timestamp: time.Now().Add(-1 * time.Hour).Format(time.RFC3339), Delta: 5, Reason: "task_complete", NewValue: 85.0},
		{Timestamp: time.Now().Add(-2 * time.Hour).Format(time.RFC3339), Delta: -2, Reason: "timeout", NewValue: 80.0},
	}, nil
}

// ========== 任务管理 ==========

func (m *MockExtendedOperationsProvider) CreateTask(taskType, description string, deadline int64) (*TaskCreateResult, error) {
	return &TaskCreateResult{
		TaskID: fmt.Sprintf("task-%d", time.Now().UnixNano()),
	}, nil
}

func (m *MockExtendedOperationsProvider) GetTaskStatus(taskID string) (*TaskStatus, error) {
	return &TaskStatus{
		TaskID:   taskID,
		Status:   "pending",
		Progress: 0,
	}, nil
}

func (m *MockExtendedOperationsProvider) AcceptTask(taskID string) error {
	return nil
}

func (m *MockExtendedOperationsProvider) SubmitTaskResult(taskID, result string) error {
	return nil
}

func (m *MockExtendedOperationsProvider) ListTasks(status string, limit int) ([]*TaskInfo, error) {
	return []*TaskInfo{
		{
			TaskID:      "task-001",
			Type:        "compute",
			Description: "Sample task",
			Status:      "pending",
			Creator:     "node-A",
			CreatedAt:   time.Now().Format(time.RFC3339),
		},
	}, nil
}

// ========== 指责系统 ==========

func (m *MockExtendedOperationsProvider) CreateAccusation(accused, accusationType, reason string) (*AccusationCreateResult, error) {
	return &AccusationCreateResult{
		AccusationID: fmt.Sprintf("acc-%d", time.Now().UnixNano()),
	}, nil
}

func (m *MockExtendedOperationsProvider) ListAccusations(accused string, limit int) ([]*AccusationInfo, error) {
	return []*AccusationInfo{
		{
			ID:        "acc-001",
			Accuser:   "node-A",
			Accused:   accused,
			Type:      "spam",
			Reason:    "Excessive messages",
			Status:    "pending",
			Timestamp: time.Now().Format(time.RFC3339),
		},
	}, nil
}

func (m *MockExtendedOperationsProvider) GetAccusationDetail(id string) (*AccusationDetail, error) {
	return &AccusationDetail{
		AccusationInfo: AccusationInfo{
			ID:        id,
			Accuser:   "node-A",
			Accused:   "node-B",
			Type:      "spam",
			Reason:    "Excessive messages",
			Status:    "pending",
			Timestamp: time.Now().Format(time.RFC3339),
		},
		Evidence: []string{"evidence-1", "evidence-2"},
	}, nil
}

func (m *MockExtendedOperationsProvider) AnalyzeAccusations(nodeID string) (*AccusationAnalysis, error) {
	return &AccusationAnalysis{
		NodeID:        nodeID,
		TotalReceived: 3,
		Credibility:   0.7,
		TrendScore:    0.5,
	}, nil
}

// ========== 激励系统 ==========

func (m *MockExtendedOperationsProvider) AwardIncentive(nodeID, taskType string) (*IncentiveAward, error) {
	return &IncentiveAward{
		Reward: 10.0,
	}, nil
}

func (m *MockExtendedOperationsProvider) PropagateReputation(target string, delta int) (*PropagateResult, error) {
	return &PropagateResult{
		PropagatedTo: 3,
	}, nil
}

func (m *MockExtendedOperationsProvider) GetIncentiveHistory(nodeID string, limit int) ([]*IncentiveRecord, error) {
	return []*IncentiveRecord{
		{Timestamp: time.Now().Format(time.RFC3339), Type: "task", Amount: 10.0, Reason: "compute_complete"},
	}, nil
}

func (m *MockExtendedOperationsProvider) GetTolerance(nodeID string) (*ToleranceInfo, error) {
	return &ToleranceInfo{
		Tolerance: 5,
		Max:       10,
	}, nil
}

// ========== 投票系统 ==========

func (m *MockExtendedOperationsProvider) CreateProposal(title, proposalType, target string) (*ProposalCreateResult, error) {
	return &ProposalCreateResult{
		ProposalID: fmt.Sprintf("prop-%d", time.Now().UnixNano()),
	}, nil
}

func (m *MockExtendedOperationsProvider) ListProposals(status string) ([]*ProposalInfo, error) {
	return []*ProposalInfo{
		{
			ID:        "prop-001",
			Title:     "Kick bad node",
			Type:      "kick",
			Target:    "node-bad",
			Status:    "pending",
			Creator:   "node-A",
			CreatedAt: time.Now().Format(time.RFC3339),
			VotesYes:  3,
			VotesNo:   1,
		},
	}, nil
}

func (m *MockExtendedOperationsProvider) GetProposal(id string) (*ProposalDetail, error) {
	return &ProposalDetail{
		ProposalInfo: ProposalInfo{
			ID:        id,
			Title:     "Kick bad node",
			Type:      "kick",
			Target:    "node-bad",
			Status:    "pending",
			Creator:   "node-A",
			CreatedAt: time.Now().Format(time.RFC3339),
			VotesYes:  3,
			VotesNo:   1,
		},
		Description: "This node has been spamming the network",
		Votes:       map[string]string{"node-A": "yes", "node-B": "yes", "node-C": "yes", "node-D": "no"},
		ExpiresAt:   time.Now().Add(24 * time.Hour).Format(time.RFC3339),
	}, nil
}

func (m *MockExtendedOperationsProvider) Vote(proposalID, vote string) error {
	return nil
}

func (m *MockExtendedOperationsProvider) FinalizeProposal(proposalID string) (*ProposalResult, error) {
	return &ProposalResult{
		Result: "passed",
	}, nil
}

// ========== 超级节点 ==========

func (m *MockExtendedOperationsProvider) ListSupernodes() ([]*SupernodeInfo, error) {
	return []*SupernodeInfo{
		{NodeID: "super-A", Term: 1, ElectedAt: time.Now().Format(time.RFC3339), Status: "active"},
		{NodeID: "super-B", Term: 1, ElectedAt: time.Now().Format(time.RFC3339), Status: "active"},
	}, nil
}

func (m *MockExtendedOperationsProvider) ListCandidates() ([]*CandidateInfo, error) {
	return []*CandidateInfo{
		{NodeID: "cand-A", Stake: 1000, Votes: 10, AppliedAt: time.Now().Format(time.RFC3339), Score: 0.85},
	}, nil
}

func (m *MockExtendedOperationsProvider) ApplyCandidate(stake int64) error {
	return nil
}

func (m *MockExtendedOperationsProvider) WithdrawCandidate() error {
	return nil
}

func (m *MockExtendedOperationsProvider) VoteCandidate(candidate string) error {
	return nil
}

func (m *MockExtendedOperationsProvider) StartElection() (*ElectionInfo, error) {
	return &ElectionInfo{
		ElectionID: fmt.Sprintf("election-%d", time.Now().UnixNano()),
		Status:     "started",
		StartedAt:  time.Now().Format(time.RFC3339),
	}, nil
}

func (m *MockExtendedOperationsProvider) FinalizeElection(electionID string) (*ElectionResult, error) {
	return &ElectionResult{
		Elected: []string{"super-A", "super-B", "super-C"},
	}, nil
}

func (m *MockExtendedOperationsProvider) SubmitAudit(target string, passed bool) (*AuditSubmitResult, error) {
	return &AuditSubmitResult{
		AuditID: fmt.Sprintf("audit-%d", time.Now().UnixNano()),
	}, nil
}

func (m *MockExtendedOperationsProvider) GetAuditResult(target string) (*AuditResultInfo, error) {
	return &AuditResultInfo{
		Target:   target,
		PassRate: 0.8,
		Total:    10,
		Passed:   8,
	}, nil
}

// ========== 创世节点 ==========

func (m *MockExtendedOperationsProvider) GetGenesisInfo() (*GenesisInfo, error) {
	return &GenesisInfo{
		GenesisID:   "genesis-001",
		CreatedAt:   "2026-01-01T00:00:00Z",
		NetworkName: "AgentNetwork",
	}, nil
}

func (m *MockExtendedOperationsProvider) CreateInvite(forPubkey string) (*InviteCreateResult, error) {
	return &InviteCreateResult{
		InvitationID: fmt.Sprintf("invite-%d", time.Now().UnixNano()),
	}, nil
}

func (m *MockExtendedOperationsProvider) VerifyInvite(invitation string) (*InviteVerifyResult, error) {
	return &InviteVerifyResult{
		Valid:   true,
		Inviter: "genesis-node",
	}, nil
}

func (m *MockExtendedOperationsProvider) JoinNetwork(invitation, pubkey string) (*JoinResult, error) {
	return &JoinResult{
		NodeID:    fmt.Sprintf("node-%s", pubkey[:8]),
		Neighbors: []string{"node-A", "node-B"},
	}, nil
}

// ========== 日志系统 ==========

func (m *MockExtendedOperationsProvider) SubmitLog(eventType string, data map[string]interface{}) (*LogSubmitResult, error) {
	return &LogSubmitResult{
		LogID: fmt.Sprintf("log-%d", time.Now().UnixNano()),
	}, nil
}

func (m *MockExtendedOperationsProvider) QueryLogs(nodeID, eventType string, limit int) ([]*LogEntryInfo, error) {
	return []*LogEntryInfo{
		{
			ID:        "log-001",
			NodeID:    nodeID,
			EventType: "task_complete",
			Data:      map[string]interface{}{"task_id": "task-001"},
			Timestamp: time.Now().Format(time.RFC3339),
		},
	}, nil
}

func (m *MockExtendedOperationsProvider) ExportLogs(format string, start, end int64) (*LogExportResult, error) {
	return &LogExportResult{
		File: fmt.Sprintf("logs_%d_%d.%s", start, end, format),
	}, nil
}

// ========== 审计集成 (Task44) ==========

func (m *MockExtendedOperationsProvider) GetAuditDeviations(limit int) ([]*AuditDeviationInfo, error) {
	return []*AuditDeviationInfo{
		{
			AuditID:        "audit-001",
			AuditorID:      "super-A",
			ExpectedResult: true,
			ActualResult:   false,
			Severity:       "minor",
			Timestamp:      time.Now().Format(time.RFC3339),
		},
	}, nil
}

func (m *MockExtendedOperationsProvider) GetAuditPenaltyConfig() (*AuditPenaltyConfigInfo, error) {
	return &AuditPenaltyConfigInfo{
		Minor:  PenaltyConfig{RepPenalty: 5, SlashRatio: 0.1},
		Severe: PenaltyConfig{RepPenalty: 20, SlashRatio: 0.3},
	}, nil
}

func (m *MockExtendedOperationsProvider) SetAuditPenaltyConfig(severity string, repPenalty int, slashRatio float64) error {
	return nil
}

func (m *MockExtendedOperationsProvider) ManualPenalty(nodeID, severity, reason string) (*ManualPenaltyResult, error) {
	return &ManualPenaltyResult{
		PenaltyApplied: true,
		RepDelta:       -5,
		Slashed:        100.0,
	}, nil
}

// ========== 抵押物管理 (Task44) ==========

func (m *MockExtendedOperationsProvider) ListCollaterals(status string) ([]*CollateralInfo, error) {
	return []*CollateralInfo{
		{
			CollateralID: "coll-001",
			NodeID:       "node-A",
			Purpose:      "supernode_stake",
			Amount:       1000.0,
			Slashed:      100.0,
			Status:       "active",
			CreatedAt:    time.Now().Format(time.RFC3339),
		},
	}, nil
}

func (m *MockExtendedOperationsProvider) GetCollateralByNode(nodeID, purpose string) (*CollateralInfo, error) {
	return &CollateralInfo{
		CollateralID: "coll-001",
		NodeID:       nodeID,
		Purpose:      purpose,
		Amount:       1000.0,
		Slashed:      100.0,
		Status:       "active",
		CreatedAt:    time.Now().Format(time.RFC3339),
	}, nil
}

func (m *MockExtendedOperationsProvider) SlashByNode(nodeID, purpose, reason, evidence string, ratio float64) (*SlashResult, error) {
	amount := 1000.0
	slashed := amount * ratio
	return &SlashResult{
		SlashedAmount: slashed,
		Remaining:     amount - slashed,
	}, nil
}

func (m *MockExtendedOperationsProvider) GetSlashHistory(nodeID string, limit int) ([]*SlashRecord, error) {
	return []*SlashRecord{
		{
			ID:        "slash-001",
			NodeID:    nodeID,
			Purpose:   "audit_bond",
			Amount:    100.0,
			Reason:    "审计偏离",
			Timestamp: time.Now().Format(time.RFC3339),
		},
	}, nil
}

// ========== 争议预审 (Task44) ==========

func (m *MockExtendedOperationsProvider) ListDisputes(status string) ([]*DisputeInfo, error) {
	return []*DisputeInfo{
		{
			ID:        "dispute-001",
			Plaintiff: "node-A",
			Defendant: "node-B",
			Status:    "pending",
			CreatedAt: time.Now().Format(time.RFC3339),
			EscrowID:  "escrow-001",
		},
	}, nil
}

func (m *MockExtendedOperationsProvider) GetDisputeSuggestion(id string) (*DisputeSuggestionInfo, error) {
	return &DisputeSuggestionInfo{
		DisputeID:           id,
		SuggestedResolution: "favor_plaintiff",
		Confidence:          0.85,
		CanAutoExecute:      false,
		MissingEvidence:     []string{"delivery_proof"},
		Warnings:            []string{"证据未全部验证"},
	}, nil
}

func (m *MockExtendedOperationsProvider) VerifyEvidence(disputeID, evidenceID, verifierID string) error {
	return nil
}

func (m *MockExtendedOperationsProvider) ApplyDisputeSuggestion(disputeID, approverID string) (*ApplySuggestionResult, error) {
	return &ApplySuggestionResult{
		Applied:    true,
		Resolution: "favor_plaintiff",
	}, nil
}

func (m *MockExtendedOperationsProvider) GetDisputeDetail(id string) (*DisputeDetail, error) {
	return &DisputeDetail{
		DisputeInfo: DisputeInfo{
			ID:        id,
			Plaintiff: "node-A",
			Defendant: "node-B",
			Status:    "pending",
			CreatedAt: time.Now().Format(time.RFC3339),
			EscrowID:  "escrow-001",
		},
		Description: "Payment dispute for task completion",
		Evidence: []EvidenceInfo{
			{
				ID:          "ev-001",
				Type:        "message",
				Content:     "Task completion proof",
				SubmittedBy: "node-A",
				Verified:    true,
				VerifiedBy:  "super-A",
				Timestamp:   time.Now().Format(time.RFC3339),
			},
		},
	}, nil
}

// ========== 托管多签 (Task44) ==========

func (m *MockExtendedOperationsProvider) ListEscrows(status string) ([]*EscrowInfo, error) {
	return []*EscrowInfo{
		{
			ID:          "escrow-001",
			Amount:      1000.0,
			Depositor:   "node-A",
			Beneficiary: "node-B",
			Status:      "active",
			CreatedAt:   time.Now().Format(time.RFC3339),
		},
	}, nil
}

func (m *MockExtendedOperationsProvider) GetEscrowDetail(id string) (*EscrowDetail, error) {
	return &EscrowDetail{
		EscrowInfo: EscrowInfo{
			ID:          id,
			Amount:      1000.0,
			Depositor:   "node-A",
			Beneficiary: "node-B",
			Status:      "active",
			CreatedAt:   time.Now().Format(time.RFC3339),
		},
		Description:    "Task payment escrow",
		Arbitrators:    []string{"arb-1", "arb-2", "arb-3"},
		MinSignatures:  2,
		CurrentSigners: []string{"arb-1"},
		ExpiresAt:      time.Now().Add(7 * 24 * time.Hour).Format(time.RFC3339),
	}, nil
}

func (m *MockExtendedOperationsProvider) SubmitArbitratorSignature(escrowID, arbitratorID, signature string) (*SignatureSubmitResult, error) {
	return &SignatureSubmitResult{
		Submitted:    true,
		CurrentCount: 1,
		Required:     2,
	}, nil
}

func (m *MockExtendedOperationsProvider) GetSignatureCount(escrowID string) (*SignatureCountInfo, error) {
	return &SignatureCountInfo{
		EscrowID:     escrowID,
		CurrentCount: 1,
		Required:     2,
		Signers:      []string{"arb-1"},
	}, nil
}

func (m *MockExtendedOperationsProvider) ResolveEscrow(escrowID, winner string, signatures map[string]string) (*EscrowResolveResult, error) {
	return &EscrowResolveResult{
		Resolved: true,
		Winner:   winner,
		Amount:   1000.0,
	}, nil
}
