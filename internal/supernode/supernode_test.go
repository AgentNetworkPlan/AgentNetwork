package supernode

import (
	"testing"
	"time"
)

// 测试辅助函数
func createTestConfig(t *testing.T) *SuperNodeConfig {
	return &SuperNodeConfig{
		NodeID:           "test-node-001",
		DataDir:          t.TempDir(),
		MaxSuperNodes:    3,
		TermDuration:     7 * 24 * time.Hour,
		ElectionDuration: 1 * time.Hour,
		MinReputation:    50,
		MinStake:         30,
		AuditThreshold:   0.6,
		AuditorsPerTask:  3,
		CleanupInterval:  1 * time.Hour,
	}
}

func createTestManager(t *testing.T) *SuperNodeManager {
	config := createTestConfig(t)
	sm, err := NewSuperNodeManager(config)
	if err != nil {
		t.Fatalf("Failed to create manager: %v", err)
	}
	return sm
}

func mockSignFunc(data []byte) ([]byte, error) {
	result := make([]byte, 32)
	copy(result, data)
	return result, nil
}

func mockVerifyFunc(pubKey string, data, signature []byte) (bool, error) {
	return true, nil
}

// === 测试用例 ===

func TestNewSuperNodeManager(t *testing.T) {
	tests := []struct {
		name    string
		config  *SuperNodeConfig
		wantErr bool
	}{
		{
			name:    "nil config",
			config:  nil,
			wantErr: true,
		},
		{
			name: "empty node ID",
			config: &SuperNodeConfig{
				NodeID:        "",
				MaxSuperNodes: 5,
			},
			wantErr: true,
		},
		{
			name: "zero max super nodes",
			config: &SuperNodeConfig{
				NodeID:        "test",
				MaxSuperNodes: 0,
			},
			wantErr: true,
		},
		{
			name: "valid config",
			config: &SuperNodeConfig{
				NodeID:        "test",
				DataDir:       t.TempDir(),
				MaxSuperNodes: 5,
			},
			wantErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			sm, err := NewSuperNodeManager(tt.config)
			if (err != nil) != tt.wantErr {
				t.Errorf("NewSuperNodeManager() error = %v, wantErr %v", err, tt.wantErr)
			}
			if !tt.wantErr && sm == nil {
				t.Error("NewSuperNodeManager() returned nil")
			}
		})
	}
}

func TestDefaultConfig(t *testing.T) {
	nodeID := "test-node"
	config := DefaultConfig(nodeID)

	if config.NodeID != nodeID {
		t.Errorf("NodeID = %v, want %v", config.NodeID, nodeID)
	}
	if config.MaxSuperNodes <= 0 {
		t.Error("MaxSuperNodes should be positive")
	}
	if config.TermDuration <= 0 {
		t.Error("TermDuration should be positive")
	}
}

func TestApplyCandidate(t *testing.T) {
	sm := createTestManager(t)

	// 申请成为候选
	err := sm.ApplyCandidate("node-001", 60, 40)
	if err != nil {
		t.Fatalf("ApplyCandidate() error = %v", err)
	}

	// 检查是否成功
	role := sm.GetNodeRole("node-001")
	if role != RoleCandidate {
		t.Errorf("Role = %v, want %v", role, RoleCandidate)
	}
}

func TestApplyCandidateLowReputation(t *testing.T) {
	sm := createTestManager(t)

	err := sm.ApplyCandidate("node-001", 30, 40) // 信誉低于阈值
	if err == nil {
		t.Error("ApplyCandidate() should fail with low reputation")
	}
}

func TestApplyCandidateLowStake(t *testing.T) {
	sm := createTestManager(t)

	err := sm.ApplyCandidate("node-001", 60, 20) // 抵押低于阈值
	if err == nil {
		t.Error("ApplyCandidate() should fail with low stake")
	}
}

func TestApplyCandidateDuplicate(t *testing.T) {
	sm := createTestManager(t)

	sm.ApplyCandidate("node-001", 60, 40)
	err := sm.ApplyCandidate("node-001", 70, 50)
	if err == nil {
		t.Error("Duplicate ApplyCandidate() should fail")
	}
}

func TestWithdrawCandidate(t *testing.T) {
	sm := createTestManager(t)

	sm.ApplyCandidate("node-001", 60, 40)
	
	err := sm.WithdrawCandidate("node-001")
	if err != nil {
		t.Fatalf("WithdrawCandidate() error = %v", err)
	}

	role := sm.GetNodeRole("node-001")
	if role != RoleNormal {
		t.Errorf("Role = %v, want %v", role, RoleNormal)
	}
}

func TestWithdrawCandidateNotFound(t *testing.T) {
	sm := createTestManager(t)

	err := sm.WithdrawCandidate("non-existent")
	if err == nil {
		t.Error("WithdrawCandidate() should fail for non-existent")
	}
}

func TestVoteForCandidate(t *testing.T) {
	sm := createTestManager(t)

	sm.ApplyCandidate("node-001", 60, 40)

	err := sm.VoteForCandidate("voter-001", "node-001", 50)
	if err != nil {
		t.Fatalf("VoteForCandidate() error = %v", err)
	}

	// 检查票数
	candidates := sm.GetCandidates()
	if len(candidates) != 1 {
		t.Fatalf("Candidates count = %d, want 1", len(candidates))
	}
	if candidates[0].Votes != 50 {
		t.Errorf("Votes = %v, want 50", candidates[0].Votes)
	}
}

func TestVoteForCandidateDuplicate(t *testing.T) {
	sm := createTestManager(t)

	sm.ApplyCandidate("node-001", 60, 40)
	sm.VoteForCandidate("voter-001", "node-001", 50)

	err := sm.VoteForCandidate("voter-001", "node-001", 30)
	if err == nil {
		t.Error("Duplicate vote should fail")
	}
}

func TestVoteForCandidateNotFound(t *testing.T) {
	sm := createTestManager(t)

	err := sm.VoteForCandidate("voter-001", "non-existent", 50)
	if err == nil {
		t.Error("VoteForCandidate() should fail for non-existent candidate")
	}
}

func TestStartElection(t *testing.T) {
	sm := createTestManager(t)

	// 添加候选人
	sm.ApplyCandidate("node-001", 60, 40)
	sm.ApplyCandidate("node-002", 70, 50)

	election, err := sm.StartElection()
	if err != nil {
		t.Fatalf("StartElection() error = %v", err)
	}

	if election.ID == "" {
		t.Error("Election ID should not be empty")
	}
	if election.Status != ElectionOpen {
		t.Errorf("Status = %v, want %v", election.Status, ElectionOpen)
	}
	if len(election.Candidates) != 2 {
		t.Errorf("Candidates count = %d, want 2", len(election.Candidates))
	}
}

func TestStartElectionDuplicate(t *testing.T) {
	sm := createTestManager(t)

	sm.ApplyCandidate("node-001", 60, 40)
	sm.StartElection()

	_, err := sm.StartElection()
	if err == nil {
		t.Error("Duplicate StartElection() should fail")
	}
}

func TestFinalizeElection(t *testing.T) {
	sm := createTestManager(t)

	// 添加候选人
	sm.ApplyCandidate("node-001", 60, 40)
	sm.ApplyCandidate("node-002", 70, 50)
	sm.ApplyCandidate("node-003", 80, 60)
	sm.ApplyCandidate("node-004", 90, 70)

	// 投票
	sm.VoteForCandidate("voter-001", "node-001", 100)
	sm.VoteForCandidate("voter-002", "node-002", 80)
	sm.VoteForCandidate("voter-003", "node-003", 60)
	sm.VoteForCandidate("voter-004", "node-004", 40)

	// 开始选举
	sm.StartElection()

	// 结束选举
	election, err := sm.FinalizeElection()
	if err != nil {
		t.Fatalf("FinalizeElection() error = %v", err)
	}

	if election.Status != ElectionFinalized {
		t.Errorf("Status = %v, want %v", election.Status, ElectionFinalized)
	}

	// 应该选出前3名
	if len(election.Winners) != 3 {
		t.Errorf("Winners count = %d, want 3", len(election.Winners))
	}

	// 检查当选者
	activeSuperNodes := sm.GetActiveSuperNodes()
	if len(activeSuperNodes) != 3 {
		t.Errorf("Active super nodes = %d, want 3", len(activeSuperNodes))
	}
}

func TestFinalizeElectionNoVotes(t *testing.T) {
	sm := createTestManager(t)

	sm.ApplyCandidate("node-001", 60, 40)
	sm.StartElection()

	election, err := sm.FinalizeElection()
	if err != nil {
		t.Fatalf("FinalizeElection() error = %v", err)
	}

	// 没有投票，不应该有赢家
	if len(election.Winners) != 0 {
		t.Errorf("Winners count = %d, want 0 (no votes)", len(election.Winners))
	}
}

func TestGetCurrentElection(t *testing.T) {
	sm := createTestManager(t)

	// 无选举
	if sm.GetCurrentElection() != nil {
		t.Error("Should be nil when no election")
	}

	sm.ApplyCandidate("node-001", 60, 40)
	sm.StartElection()

	if sm.GetCurrentElection() == nil {
		t.Error("Should not be nil during election")
	}
}

func TestIsSuperNode(t *testing.T) {
	sm := createTestManager(t)

	// 普通节点
	if sm.IsSuperNode("node-001") {
		t.Error("Should not be super node")
	}

	// 添加超级节点
	sm.mu.Lock()
	sm.superNodes["node-001"] = &SuperNode{
		NodeID:   "node-001",
		IsActive: true,
	}
	sm.mu.Unlock()

	if !sm.IsSuperNode("node-001") {
		t.Error("Should be super node")
	}
}

func TestGetSuperNode(t *testing.T) {
	sm := createTestManager(t)

	// 添加超级节点
	sm.mu.Lock()
	sm.superNodes["node-001"] = &SuperNode{
		NodeID:     "node-001",
		Reputation: 70,
		Stake:      50,
		IsActive:   true,
	}
	sm.mu.Unlock()

	sn, err := sm.GetSuperNode("node-001")
	if err != nil {
		t.Fatalf("GetSuperNode() error = %v", err)
	}
	if sn.Reputation != 70 {
		t.Errorf("Reputation = %v, want 70", sn.Reputation)
	}
}

func TestGetSuperNodeNotFound(t *testing.T) {
	sm := createTestManager(t)

	_, err := sm.GetSuperNode("non-existent")
	if err == nil {
		t.Error("GetSuperNode() should fail for non-existent")
	}
}

func TestRemoveSuperNode(t *testing.T) {
	sm := createTestManager(t)

	sm.mu.Lock()
	sm.superNodes["node-001"] = &SuperNode{
		NodeID:   "node-001",
		IsActive: true,
	}
	sm.mu.Unlock()

	err := sm.RemoveSuperNode("node-001", "test reason")
	if err != nil {
		t.Fatalf("RemoveSuperNode() error = %v", err)
	}

	if sm.IsSuperNode("node-001") {
		t.Error("Should not be active after removal")
	}
}

func TestRemoveSuperNodeNotFound(t *testing.T) {
	sm := createTestManager(t)

	err := sm.RemoveSuperNode("non-existent", "test")
	if err == nil {
		t.Error("RemoveSuperNode() should fail for non-existent")
	}
}

func TestGetNodeRole(t *testing.T) {
	sm := createTestManager(t)

	// 普通节点
	role := sm.GetNodeRole("normal-node")
	if role != RoleNormal {
		t.Errorf("Role = %v, want %v", role, RoleNormal)
	}

	// 候选节点
	sm.ApplyCandidate("candidate-node", 60, 40)
	role = sm.GetNodeRole("candidate-node")
	if role != RoleCandidate {
		t.Errorf("Role = %v, want %v", role, RoleCandidate)
	}

	// 超级节点
	sm.mu.Lock()
	sm.superNodes["super-node"] = &SuperNode{NodeID: "super-node", IsActive: true}
	sm.mu.Unlock()
	role = sm.GetNodeRole("super-node")
	if role != RoleSuper {
		t.Errorf("Role = %v, want %v", role, RoleSuper)
	}
}

func TestCreateAudit(t *testing.T) {
	sm := createTestManager(t)

	// 添加超级节点
	sm.mu.Lock()
	sm.superNodes["super-001"] = &SuperNode{NodeID: "super-001", IsActive: true}
	sm.superNodes["super-002"] = &SuperNode{NodeID: "super-002", IsActive: true}
	sm.superNodes["super-003"] = &SuperNode{NodeID: "super-003", IsActive: true}
	sm.mu.Unlock()

	audit, err := sm.CreateAudit(AuditTask, "target-001")
	if err != nil {
		t.Fatalf("CreateAudit() error = %v", err)
	}

	if audit.ID == "" {
		t.Error("Audit ID should not be empty")
	}
	if audit.Type != AuditTask {
		t.Errorf("Type = %v, want %v", audit.Type, AuditTask)
	}
	if len(audit.Auditors) != 3 {
		t.Errorf("Auditors count = %d, want 3", len(audit.Auditors))
	}
}

func TestCreateAuditNoSuperNodes(t *testing.T) {
	sm := createTestManager(t)

	_, err := sm.CreateAudit(AuditTask, "target-001")
	if err == nil {
		t.Error("CreateAudit() should fail with no super nodes")
	}
}

func TestCreateAuditEmptyTarget(t *testing.T) {
	sm := createTestManager(t)

	sm.mu.Lock()
	sm.superNodes["super-001"] = &SuperNode{NodeID: "super-001", IsActive: true}
	sm.mu.Unlock()

	_, err := sm.CreateAudit(AuditTask, "")
	if err == nil {
		t.Error("CreateAudit() should fail with empty target")
	}
}

func TestSubmitAuditResult(t *testing.T) {
	sm := createTestManager(t)
	sm.SetSignFunc(mockSignFunc)

	// 添加超级节点
	sm.mu.Lock()
	sm.superNodes["super-001"] = &SuperNode{NodeID: "super-001", IsActive: true}
	sm.superNodes["super-002"] = &SuperNode{NodeID: "super-002", IsActive: true}
	sm.superNodes["super-003"] = &SuperNode{NodeID: "super-003", IsActive: true}
	sm.mu.Unlock()

	audit, _ := sm.CreateAudit(AuditTask, "target-001")

	// 提交结果
	for _, auditor := range audit.Auditors {
		err := sm.SubmitAuditResult(audit.ID, auditor, ResultPass, "looks good")
		if err != nil {
			t.Fatalf("SubmitAuditResult() error = %v", err)
		}
	}

	// 检查是否已完成
	completed, _ := sm.GetAudit(audit.ID)
	if !completed.Finalized {
		t.Error("Audit should be finalized")
	}
	if completed.FinalResult != ResultPass {
		t.Errorf("FinalResult = %v, want %v", completed.FinalResult, ResultPass)
	}
}

func TestSubmitAuditResultNotAuditor(t *testing.T) {
	sm := createTestManager(t)

	sm.mu.Lock()
	sm.superNodes["super-001"] = &SuperNode{NodeID: "super-001", IsActive: true}
	sm.mu.Unlock()

	audit, _ := sm.CreateAudit(AuditTask, "target-001")

	err := sm.SubmitAuditResult(audit.ID, "not-auditor", ResultPass, "")
	if err == nil {
		t.Error("SubmitAuditResult() should fail for non-auditor")
	}
}

func TestSubmitAuditResultDuplicate(t *testing.T) {
	sm := createTestManager(t)

	sm.mu.Lock()
	sm.superNodes["super-001"] = &SuperNode{NodeID: "super-001", IsActive: true}
	sm.mu.Unlock()

	audit, _ := sm.CreateAudit(AuditTask, "target-001")
	auditor := audit.Auditors[0]

	sm.SubmitAuditResult(audit.ID, auditor, ResultPass, "")
	err := sm.SubmitAuditResult(audit.ID, auditor, ResultFail, "")
	if err == nil {
		t.Error("Duplicate SubmitAuditResult() should fail")
	}
}

func TestAuditMajorityFail(t *testing.T) {
	sm := createTestManager(t)

	sm.mu.Lock()
	sm.superNodes["super-001"] = &SuperNode{NodeID: "super-001", IsActive: true}
	sm.superNodes["super-002"] = &SuperNode{NodeID: "super-002", IsActive: true}
	sm.superNodes["super-003"] = &SuperNode{NodeID: "super-003", IsActive: true}
	sm.mu.Unlock()

	audit, _ := sm.CreateAudit(AuditTask, "target-001")

	// 2个失败，1个通过
	results := []AuditResult{ResultFail, ResultFail, ResultPass}
	for i, auditor := range audit.Auditors {
		sm.SubmitAuditResult(audit.ID, auditor, results[i], "")
	}

	completed, _ := sm.GetAudit(audit.ID)
	if completed.FinalResult != ResultFail {
		t.Errorf("FinalResult = %v, want %v", completed.FinalResult, ResultFail)
	}
}

func TestGetPendingAudits(t *testing.T) {
	sm := createTestManager(t)

	sm.mu.Lock()
	sm.superNodes["super-001"] = &SuperNode{NodeID: "super-001", IsActive: true}
	sm.superNodes["super-002"] = &SuperNode{NodeID: "super-002", IsActive: true}
	sm.mu.Unlock()

	// 创建审计
	audit, _ := sm.CreateAudit(AuditTask, "target-001")

	// 获取待处理
	pending := sm.GetPendingAudits("super-001")
	found := false
	for _, p := range pending {
		if p.ID == audit.ID {
			found = true
			break
		}
	}
	if !found {
		t.Error("Audit should be in pending list")
	}

	// 提交结果后不再出现在待处理列表
	sm.SubmitAuditResult(audit.ID, "super-001", ResultPass, "")
	pending = sm.GetPendingAudits("super-001")
	for _, p := range pending {
		if p.ID == audit.ID {
			t.Error("Audit should not be in pending list after submission")
		}
	}
}

func TestStartStop(t *testing.T) {
	sm := createTestManager(t)

	err := sm.Start()
	if err != nil {
		t.Fatalf("Start() error = %v", err)
	}

	sm.ApplyCandidate("node-001", 60, 40)

	err = sm.Stop()
	if err != nil {
		t.Fatalf("Stop() error = %v", err)
	}
}

func TestPersistence(t *testing.T) {
	tempDir := t.TempDir()

	config := &SuperNodeConfig{
		NodeID:          "test-node",
		DataDir:         tempDir,
		MaxSuperNodes:   5,
		MinReputation:   50,
		MinStake:        30,
		AuditThreshold:  0.6,
		AuditorsPerTask: 3,
		CleanupInterval: 1 * time.Hour,
	}

	// 创建第一个实例
	sm1, _ := NewSuperNodeManager(config)
	sm1.ApplyCandidate("node-001", 60, 40)
	sm1.mu.Lock()
	sm1.superNodes["super-001"] = &SuperNode{NodeID: "super-001", Reputation: 70, IsActive: true}
	sm1.mu.Unlock()

	err := sm1.saveToDisk()
	if err != nil {
		t.Fatalf("saveToDisk() error = %v", err)
	}

	// 创建第二个实例并加载
	sm2, _ := NewSuperNodeManager(config)
	err = sm2.loadFromDisk()
	if err != nil {
		t.Fatalf("loadFromDisk() error = %v", err)
	}

	// 验证数据
	candidates := sm2.GetCandidates()
	if len(candidates) != 1 {
		t.Errorf("Candidates count = %d, want 1", len(candidates))
	}

	if !sm2.IsSuperNode("super-001") {
		t.Error("Super node should be loaded")
	}
}

func TestGetStats(t *testing.T) {
	sm := createTestManager(t)

	sm.ApplyCandidate("node-001", 60, 40)
	sm.ApplyCandidate("node-002", 70, 50)

	sm.mu.Lock()
	sm.superNodes["super-001"] = &SuperNode{NodeID: "super-001", IsActive: true, PassRate: 0.8}
	sm.superNodes["super-002"] = &SuperNode{NodeID: "super-002", IsActive: true, PassRate: 0.9}
	sm.superNodes["super-003"] = &SuperNode{NodeID: "super-003", IsActive: false, PassRate: 0.7}
	sm.audits["audit-001"] = &MultiAudit{ID: "audit-001", Finalized: true}
	sm.audits["audit-002"] = &MultiAudit{ID: "audit-002", Finalized: false}
	sm.mu.Unlock()

	stats := sm.GetStats()

	if stats.TotalSuperNodes != 3 {
		t.Errorf("TotalSuperNodes = %d, want 3", stats.TotalSuperNodes)
	}
	if stats.ActiveSuperNodes != 2 {
		t.Errorf("ActiveSuperNodes = %d, want 2", stats.ActiveSuperNodes)
	}
	if stats.TotalCandidates != 2 {
		t.Errorf("TotalCandidates = %d, want 2", stats.TotalCandidates)
	}
	if stats.TotalAudits != 2 {
		t.Errorf("TotalAudits = %d, want 2", stats.TotalAudits)
	}
	if stats.CompletedAudits != 1 {
		t.Errorf("CompletedAudits = %d, want 1", stats.CompletedAudits)
	}
	// AveragePassRate = (0.8 + 0.9) / 2 = 0.85
	expectedPassRate := 0.85
	epsilon := 0.0001
	if stats.AveragePassRate < expectedPassRate-epsilon || stats.AveragePassRate > expectedPassRate+epsilon {
		t.Errorf("AveragePassRate = %v, want %v (±%v)", stats.AveragePassRate, expectedPassRate, epsilon)
	}
}

func TestCallbacks(t *testing.T) {
	sm := createTestManager(t)

	var electedNode *SuperNode
	var removedNodeID string
	var completedAudit *MultiAudit
	var startedElection *Election
	var finalizedElection *Election

	sm.SetOnSuperNodeElected(func(sn *SuperNode) {
		electedNode = sn
	})
	sm.SetOnSuperNodeRemoved(func(nodeID string) {
		removedNodeID = nodeID
	})
	sm.SetOnAuditCompleted(func(ma *MultiAudit) {
		completedAudit = ma
	})
	sm.SetOnElectionStarted(func(e *Election) {
		startedElection = e
	})
	sm.SetOnElectionFinalized(func(e *Election) {
		finalizedElection = e
	})

	// 选举流程
	sm.ApplyCandidate("node-001", 60, 40)
	sm.VoteForCandidate("voter-001", "node-001", 100)
	sm.StartElection()
	time.Sleep(50 * time.Millisecond)
	if startedElection == nil {
		t.Error("OnElectionStarted not triggered")
	}

	sm.FinalizeElection()
	time.Sleep(50 * time.Millisecond)
	if finalizedElection == nil {
		t.Error("OnElectionFinalized not triggered")
	}
	if electedNode == nil {
		t.Error("OnSuperNodeElected not triggered")
	}

	// 移除超级节点
	sm.RemoveSuperNode("node-001", "test")
	time.Sleep(50 * time.Millisecond)
	if removedNodeID != "node-001" {
		t.Error("OnSuperNodeRemoved not triggered")
	}

	// 审计流程
	sm.mu.Lock()
	sm.superNodes["super-001"] = &SuperNode{NodeID: "super-001", IsActive: true}
	sm.mu.Unlock()

	audit, _ := sm.CreateAudit(AuditTask, "target-001")
	for _, auditor := range audit.Auditors {
		sm.SubmitAuditResult(audit.ID, auditor, ResultPass, "")
	}
	time.Sleep(50 * time.Millisecond)
	if completedAudit == nil {
		t.Error("OnAuditCompleted not triggered")
	}
}

func TestSetFunctions(t *testing.T) {
	sm := createTestManager(t)

	sm.SetSignFunc(mockSignFunc)
	sm.SetVerifyFunc(mockVerifyFunc)

	// 验证设置成功
	sm.mu.Lock()
	sm.superNodes["super-001"] = &SuperNode{NodeID: "super-001", IsActive: true}
	sm.mu.Unlock()

	audit, _ := sm.CreateAudit(AuditTask, "target-001")
	err := sm.SubmitAuditResult(audit.ID, audit.Auditors[0], ResultPass, "test")
	if err != nil {
		t.Errorf("SubmitAuditResult() after SetSignFunc error = %v", err)
	}

	a, _ := sm.GetAudit(audit.ID)
	if len(a.Results[audit.Auditors[0]].Signature) == 0 {
		t.Error("Audit result should be signed")
	}
}

func TestCheckTermExpiry(t *testing.T) {
	sm := createTestManager(t)

	// 添加即将过期的超级节点
	sm.mu.Lock()
	sm.superNodes["expired-node"] = &SuperNode{
		NodeID:     "expired-node",
		IsActive:   true,
		TermEndsAt: time.Now().Add(-1 * time.Hour), // 已过期
	}
	sm.superNodes["active-node"] = &SuperNode{
		NodeID:     "active-node",
		IsActive:   true,
		TermEndsAt: time.Now().Add(24 * time.Hour), // 未过期
	}
	sm.mu.Unlock()

	sm.checkTermExpiry()

	if sm.IsSuperNode("expired-node") {
		t.Error("Expired node should not be active")
	}
	if !sm.IsSuperNode("active-node") {
		t.Error("Active node should still be active")
	}
}

func TestGetCandidates(t *testing.T) {
	sm := createTestManager(t)

	sm.ApplyCandidate("node-001", 60, 40)
	sm.ApplyCandidate("node-002", 70, 50)
	sm.ApplyCandidate("node-003", 80, 60)

	sm.VoteForCandidate("voter-001", "node-001", 100)
	sm.VoteForCandidate("voter-002", "node-002", 50)
	sm.VoteForCandidate("voter-003", "node-003", 75)

	candidates := sm.GetCandidates()

	if len(candidates) != 3 {
		t.Fatalf("Candidates count = %d, want 3", len(candidates))
	}

	// 应该按票数排序（100 > 75 > 50）
	if candidates[0].NodeID != "node-001" {
		t.Errorf("First candidate = %v, want node-001", candidates[0].NodeID)
	}
	if candidates[1].NodeID != "node-003" {
		t.Errorf("Second candidate = %v, want node-003", candidates[1].NodeID)
	}
	if candidates[2].NodeID != "node-002" {
		t.Errorf("Third candidate = %v, want node-002", candidates[2].NodeID)
	}
}
