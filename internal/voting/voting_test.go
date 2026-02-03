package voting

import (
	"testing"
	"time"
)

// 测试辅助函数
func createTestConfig(t *testing.T) *VotingConfig {
	return &VotingConfig{
		NodeID:           "test-node-001",
		DataDir:          t.TempDir(),
		PassThreshold:    0.6,
		QuorumThreshold:  0.3,
		ProposalDuration: 30 * time.Minute,
		BufferPeriod:     0, // 测试时禁用缓冲期
		ReputationWeight: 0.7,
		StakeWeight:      0.3,
		MinRepToVote:     10,
		MinRepToPropose:  30,
		CleanupInterval:  1 * time.Hour,
	}
}

func createTestVotingManager(t *testing.T) *VotingManager {
	config := createTestConfig(t)
	vm, err := NewVotingManager(config)
	if err != nil {
		t.Fatalf("Failed to create voting manager: %v", err)
	}
	// 注册当前节点
	vm.RegisterNode(config.NodeID, 50, 30)
	return vm
}

// Mock签名函数
func mockSignFunc(data []byte) ([]byte, error) {
	result := make([]byte, 32)
	copy(result, data)
	return result, nil
}

// Mock验签函数
func mockVerifyFunc(pubKey string, data, signature []byte) (bool, error) {
	return true, nil
}

// === 测试用例 ===

func TestNewVotingManager(t *testing.T) {
	tests := []struct {
		name    string
		config  *VotingConfig
		wantErr bool
	}{
		{
			name:    "nil config",
			config:  nil,
			wantErr: true,
		},
		{
			name: "empty node ID",
			config: &VotingConfig{
				NodeID:        "",
				PassThreshold: 0.6,
			},
			wantErr: true,
		},
		{
			name: "invalid threshold",
			config: &VotingConfig{
				NodeID:        "test",
				PassThreshold: 1.5,
			},
			wantErr: true,
		},
		{
			name: "valid config",
			config: &VotingConfig{
				NodeID:        "test-node",
				DataDir:       t.TempDir(),
				PassThreshold: 0.6,
			},
			wantErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			vm, err := NewVotingManager(tt.config)
			if (err != nil) != tt.wantErr {
				t.Errorf("NewVotingManager() error = %v, wantErr %v", err, tt.wantErr)
			}
			if !tt.wantErr && vm == nil {
				t.Error("NewVotingManager() returned nil")
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
	if config.PassThreshold <= 0 || config.PassThreshold > 1 {
		t.Error("PassThreshold should be between 0 and 1")
	}
	if config.QuorumThreshold <= 0 || config.QuorumThreshold > 1 {
		t.Error("QuorumThreshold should be between 0 and 1")
	}
}

func TestRegisterNode(t *testing.T) {
	vm := createTestVotingManager(t)

	// 注册新节点
	err := vm.RegisterNode("node-002", 60, 40)
	if err != nil {
		t.Fatalf("RegisterNode() error = %v", err)
	}

	// 获取节点信息
	trust, err := vm.GetNodeTrust("node-002")
	if err != nil {
		t.Fatalf("GetNodeTrust() error = %v", err)
	}

	if trust.Reputation != 60 {
		t.Errorf("Reputation = %v, want 60", trust.Reputation)
	}
	if trust.Stake != 40 {
		t.Errorf("Stake = %v, want 40", trust.Stake)
	}
	if trust.Status != StatusActive {
		t.Errorf("Status = %v, want %v", trust.Status, StatusActive)
	}
}

func TestRegisterNodeDuplicate(t *testing.T) {
	vm := createTestVotingManager(t)

	// 注册相同节点两次
	err := vm.RegisterNode("node-002", 60, 40)
	if err != nil {
		t.Fatalf("First RegisterNode() error = %v", err)
	}

	err = vm.RegisterNode("node-002", 70, 50)
	if err == nil {
		t.Error("Second RegisterNode() should fail")
	}
}

func TestUpdateNodeTrust(t *testing.T) {
	vm := createTestVotingManager(t)
	vm.RegisterNode("node-002", 50, 30)

	// 更新信任信息
	err := vm.UpdateNodeTrust("node-002", 70, 50)
	if err != nil {
		t.Fatalf("UpdateNodeTrust() error = %v", err)
	}

	trust, _ := vm.GetNodeTrust("node-002")
	if trust.Reputation != 70 {
		t.Errorf("Reputation = %v, want 70", trust.Reputation)
	}
	if trust.Stake != 50 {
		t.Errorf("Stake = %v, want 50", trust.Stake)
	}
}

func TestUpdateNodeTrustNotFound(t *testing.T) {
	vm := createTestVotingManager(t)

	err := vm.UpdateNodeTrust("non-existent", 70, 50)
	if err == nil {
		t.Error("UpdateNodeTrust() should fail for non-existent node")
	}
}

func TestGetNodeStatus(t *testing.T) {
	vm := createTestVotingManager(t)
	vm.RegisterNode("node-002", 50, 30)

	status := vm.GetNodeStatus("node-002")
	if status != StatusActive {
		t.Errorf("Status = %v, want %v", status, StatusActive)
	}

	// 未注册节点
	status = vm.GetNodeStatus("non-existent")
	if status != StatusPending {
		t.Errorf("Status = %v, want %v", status, StatusPending)
	}
}

func TestCreateProposal(t *testing.T) {
	vm := createTestVotingManager(t)

	proposal, err := vm.CreateProposal(VoteKick, "target-node", "Test reason")
	if err != nil {
		t.Fatalf("CreateProposal() error = %v", err)
	}

	if proposal.ID == "" {
		t.Error("Proposal ID should not be empty")
	}
	if proposal.Type != VoteKick {
		t.Errorf("Type = %v, want %v", proposal.Type, VoteKick)
	}
	if proposal.TargetNodeID != "target-node" {
		t.Errorf("TargetNodeID = %v, want target-node", proposal.TargetNodeID)
	}
	if proposal.ProposerID != vm.config.NodeID {
		t.Errorf("ProposerID = %v, want %v", proposal.ProposerID, vm.config.NodeID)
	}
	if proposal.Status != ProposalPending {
		t.Errorf("Status = %v, want %v", proposal.Status, ProposalPending)
	}
}

func TestCreateProposalLowReputation(t *testing.T) {
	config := createTestConfig(t)
	config.MinRepToPropose = 100 // 设置很高的要求
	vm, _ := NewVotingManager(config)
	vm.RegisterNode(config.NodeID, 50, 30) // 信誉只有50

	_, err := vm.CreateProposal(VoteKick, "target-node", "Test reason")
	if err == nil {
		t.Error("CreateProposal() should fail with low reputation")
	}
}

func TestCreateProposalDuplicate(t *testing.T) {
	vm := createTestVotingManager(t)

	// 第一个提案
	_, err := vm.CreateProposal(VoteKick, "target-node", "Test reason")
	if err != nil {
		t.Fatalf("First CreateProposal() error = %v", err)
	}

	// 相同的提案应该失败
	_, err = vm.CreateProposal(VoteKick, "target-node", "Another reason")
	if err == nil {
		t.Error("Duplicate proposal should fail")
	}
}

func TestCastVote(t *testing.T) {
	vm := createTestVotingManager(t)
	vm.SetSignFunc(mockSignFunc)

	// 创建提案
	proposal, _ := vm.CreateProposal(VoteKick, "target-node", "Test")

	// 投票
	vote, err := vm.CastVote(proposal.ID, ChoiceYes, "I agree")
	if err != nil {
		t.Fatalf("CastVote() error = %v", err)
	}

	if vote.ID == "" {
		t.Error("Vote ID should not be empty")
	}
	if vote.Choice != ChoiceYes {
		t.Errorf("Choice = %v, want %v", vote.Choice, ChoiceYes)
	}
	if vote.Weight <= 0 {
		t.Error("Weight should be positive")
	}
}

func TestCastVoteLowReputation(t *testing.T) {
	config := createTestConfig(t)
	config.MinRepToVote = 100 // 高要求
	config.MinRepToPropose = 30
	vm, _ := NewVotingManager(config)
	vm.RegisterNode(config.NodeID, 50, 30)

	proposal, _ := vm.CreateProposal(VoteKick, "target-node", "Test")

	_, err := vm.CastVote(proposal.ID, ChoiceYes, "I agree")
	if err == nil {
		t.Error("CastVote() should fail with low reputation")
	}
}

func TestCastVoteDuplicate(t *testing.T) {
	vm := createTestVotingManager(t)
	proposal, _ := vm.CreateProposal(VoteKick, "target-node", "Test")

	// 第一次投票
	_, err := vm.CastVote(proposal.ID, ChoiceYes, "")
	if err != nil {
		t.Fatalf("First CastVote() error = %v", err)
	}

	// 重复投票
	_, err = vm.CastVote(proposal.ID, ChoiceNo, "")
	if err == nil {
		t.Error("Duplicate vote should fail")
	}
}

func TestCastVoteNotFound(t *testing.T) {
	vm := createTestVotingManager(t)

	_, err := vm.CastVote("non-existent", ChoiceYes, "")
	if err == nil {
		t.Error("CastVote() should fail for non-existent proposal")
	}
}

func TestCastVoteBufferPeriod(t *testing.T) {
	config := createTestConfig(t)
	config.BufferPeriod = 1 * time.Hour // 设置1小时缓冲期
	vm, _ := NewVotingManager(config)
	vm.RegisterNode(config.NodeID, 50, 30)

	proposal, _ := vm.CreateProposal(VoteKick, "target-node", "Test")

	// 在缓冲期内投票应该失败
	_, err := vm.CastVote(proposal.ID, ChoiceYes, "")
	if err == nil {
		t.Error("CastVote() should fail during buffer period")
	}
}

func TestReceiveVote(t *testing.T) {
	vm := createTestVotingManager(t)
	vm.SetVerifyFunc(mockVerifyFunc)
	vm.RegisterNode("voter-001", 50, 30)

	proposal, _ := vm.CreateProposal(VoteKick, "target-node", "Test")

	// 接收外部投票
	vote := &Vote{
		ID:         "vote-001",
		ProposalID: proposal.ID,
		VoterID:    "voter-001",
		Choice:     ChoiceYes,
		Weight:     35, // 0.7*50 + 0.3*30
		Timestamp:  time.Now(),
		Signature:  []byte("sig"),
	}

	err := vm.ReceiveVote(vote)
	if err != nil {
		t.Fatalf("ReceiveVote() error = %v", err)
	}

	// 检查投票已记录
	p, _ := vm.GetProposal(proposal.ID)
	if len(p.Votes) != 1 {
		t.Errorf("Votes count = %d, want 1", len(p.Votes))
	}
}

func TestReceiveVoteErrors(t *testing.T) {
	vm := createTestVotingManager(t)
	vm.RegisterNode("voter-001", 50, 30)

	proposal, _ := vm.CreateProposal(VoteKick, "target-node", "Test")

	tests := []struct {
		name    string
		vote    *Vote
		wantErr bool
	}{
		{
			name:    "nil vote",
			vote:    nil,
			wantErr: true,
		},
		{
			name: "non-existent proposal",
			vote: &Vote{
				ProposalID: "non-existent",
				VoterID:    "voter-001",
			},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := vm.ReceiveVote(tt.vote)
			if (err != nil) != tt.wantErr {
				t.Errorf("ReceiveVote() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}

	// 测试重复投票
	vote := &Vote{
		ID:         "vote-001",
		ProposalID: proposal.ID,
		VoterID:    "voter-001",
		Choice:     ChoiceYes,
		Weight:     35,
		Timestamp:  time.Now(),
	}
	vm.ReceiveVote(vote)

	err := vm.ReceiveVote(vote)
	if err == nil {
		t.Error("Duplicate ReceiveVote() should fail")
	}
}

func TestGetProposal(t *testing.T) {
	vm := createTestVotingManager(t)

	proposal, _ := vm.CreateProposal(VoteKick, "target-node", "Test")

	retrieved, err := vm.GetProposal(proposal.ID)
	if err != nil {
		t.Fatalf("GetProposal() error = %v", err)
	}

	if retrieved.ID != proposal.ID {
		t.Errorf("ID = %v, want %v", retrieved.ID, proposal.ID)
	}
}

func TestGetProposalNotFound(t *testing.T) {
	vm := createTestVotingManager(t)

	_, err := vm.GetProposal("non-existent")
	if err == nil {
		t.Error("GetProposal() should fail for non-existent")
	}
}

func TestListProposals(t *testing.T) {
	vm := createTestVotingManager(t)

	// 创建多个提案
	vm.CreateProposal(VoteKick, "target-1", "Test 1")
	vm.CreateProposal(VoteKick, "target-2", "Test 2")
	vm.CreateProposal(VoteKick, "target-3", "Test 3")

	// 获取所有提案
	proposals := vm.ListProposals("", 10, 0)
	if len(proposals) != 3 {
		t.Errorf("ListProposals() returned %d, want 3", len(proposals))
	}

	// 获取待处理提案
	pending := vm.ListProposals(ProposalPending, 10, 0)
	if len(pending) != 3 {
		t.Errorf("Pending proposals = %d, want 3", len(pending))
	}

	// 测试分页
	page1 := vm.ListProposals("", 2, 0)
	if len(page1) != 2 {
		t.Errorf("Page 1 should have 2, got %d", len(page1))
	}

	page2 := vm.ListProposals("", 2, 2)
	if len(page2) != 1 {
		t.Errorf("Page 2 should have 1, got %d", len(page2))
	}
}

func TestGetActiveProposals(t *testing.T) {
	vm := createTestVotingManager(t)

	vm.CreateProposal(VoteKick, "target-1", "Test 1")
	vm.CreateProposal(VoteKick, "target-2", "Test 2")

	active := vm.GetActiveProposals()
	if len(active) != 2 {
		t.Errorf("Active proposals = %d, want 2", len(active))
	}
}

func TestCalculateVoteWeight(t *testing.T) {
	vm := createTestVotingManager(t)
	vm.RegisterNode("node-002", 80, 40)

	// 权重 = 0.7 * 80 + 0.3 * 40 = 56 + 12 = 68
	weight := vm.calculateVoteWeight("node-002")
	expected := 0.7*80 + 0.3*40
	if weight != expected {
		t.Errorf("Weight = %v, want %v", weight, expected)
	}
}

func TestProposalFinalization(t *testing.T) {
	config := createTestConfig(t)
	config.QuorumThreshold = 0.3
	config.PassThreshold = 0.6
	vm, _ := NewVotingManager(config)

	// 注册多个节点
	vm.RegisterNode("node-001", 50, 30)  // 权重 = 44
	vm.RegisterNode("node-002", 60, 40)  // 权重 = 54
	vm.RegisterNode("node-003", 70, 50)  // 权重 = 64
	vm.RegisterNode("node-004", 80, 60)  // 权重 = 74
	
	// node-001 创建提案
	vm.config.NodeID = "node-001"
	proposal, _ := vm.CreateProposal(VoteKick, "target-node", "Test")

	// 投票（需要超过60%同意）
	vm.config.NodeID = "node-001"
	vm.CastVote(proposal.ID, ChoiceYes, "")  // +44

	vm.config.NodeID = "node-002"
	vm.CastVote(proposal.ID, ChoiceYes, "")  // +54

	vm.config.NodeID = "node-003"
	vm.CastVote(proposal.ID, ChoiceNo, "")   // -64 (反对)

	vm.config.NodeID = "node-004"
	vm.CastVote(proposal.ID, ChoiceYes, "")  // +74

	// 总权重 = 44+54+64+74 = 236
	// 同意权重 = 44+54+74 = 172
	// 同意率 = 172/236 ≈ 72.88% > 60%，应该通过

	p, _ := vm.GetProposal(proposal.ID)
	if p.Status != ProposalPassed {
		t.Errorf("Status = %v, want %v", p.Status, ProposalPassed)
	}
	if p.Result == nil {
		t.Fatal("Result should not be nil")
	}
	if !p.Result.Passed {
		t.Error("Proposal should pass")
	}
}

func TestProposalRejection(t *testing.T) {
	config := createTestConfig(t)
	config.QuorumThreshold = 0.3
	config.PassThreshold = 0.6
	vm, _ := NewVotingManager(config)

	// 注册多个节点
	vm.RegisterNode("node-001", 50, 30)  // 权重 = 44
	vm.RegisterNode("node-002", 60, 40)  // 权重 = 54
	vm.RegisterNode("node-003", 70, 50)  // 权重 = 64

	vm.config.NodeID = "node-001"
	proposal, _ := vm.CreateProposal(VoteKick, "target-node", "Test")

	// 投票（少数同意）
	vm.config.NodeID = "node-001"
	vm.CastVote(proposal.ID, ChoiceYes, "")  // +44

	vm.config.NodeID = "node-002"
	vm.CastVote(proposal.ID, ChoiceNo, "")   // -54

	vm.config.NodeID = "node-003"
	vm.CastVote(proposal.ID, ChoiceNo, "")   // -64

	// 同意率 = 44/(44+54+64) = 44/162 ≈ 27.16% < 60%

	p, _ := vm.GetProposal(proposal.ID)
	if p.Status != ProposalRejected {
		t.Errorf("Status = %v, want %v", p.Status, ProposalRejected)
	}
}

func TestNodeKickCallback(t *testing.T) {
	config := createTestConfig(t)
	config.QuorumThreshold = 0.2
	config.PassThreshold = 0.5
	vm, _ := NewVotingManager(config)

	var kickedNode string
	vm.SetOnNodeKicked(func(nodeID string) {
		kickedNode = nodeID
	})

	vm.RegisterNode("node-001", 50, 30)
	vm.RegisterNode("node-002", 60, 40)
	vm.RegisterNode("target-node", 30, 20) // 目标节点

	vm.config.NodeID = "node-001"
	proposal, _ := vm.CreateProposal(VoteKick, "target-node", "Test")

	vm.config.NodeID = "node-001"
	vm.CastVote(proposal.ID, ChoiceYes, "")

	vm.config.NodeID = "node-002"
	vm.CastVote(proposal.ID, ChoiceYes, "")

	// 等待回调
	time.Sleep(50 * time.Millisecond)

	if kickedNode != "target-node" {
		t.Errorf("KickedNode = %v, want target-node", kickedNode)
	}

	// 检查节点状态
	trust, _ := vm.GetNodeTrust("target-node")
	if trust.Status != StatusRemoved {
		t.Errorf("Status = %v, want %v", trust.Status, StatusRemoved)
	}
}

func TestNodeRestoreProposal(t *testing.T) {
	config := createTestConfig(t)
	config.QuorumThreshold = 0.2
	config.PassThreshold = 0.5
	vm, _ := NewVotingManager(config)

	var restoredNode string
	vm.SetOnNodeRestored(func(nodeID string) {
		restoredNode = nodeID
	})

	vm.RegisterNode("node-001", 50, 30)
	vm.RegisterNode("node-002", 60, 40)
	
	// 先添加一个被剔除的节点
	vm.RegisterNode("target-node", 30, 20)
	vm.mu.Lock()
	vm.nodes["target-node"].Status = StatusRemoved
	vm.mu.Unlock()

	// 创建恢复提案
	vm.config.NodeID = "node-001"
	proposal, _ := vm.CreateProposal(VoteRestore, "target-node", "Restore")

	vm.config.NodeID = "node-001"
	vm.CastVote(proposal.ID, ChoiceYes, "")

	vm.config.NodeID = "node-002"
	vm.CastVote(proposal.ID, ChoiceYes, "")

	time.Sleep(50 * time.Millisecond)

	if restoredNode != "target-node" {
		t.Errorf("RestoredNode = %v, want target-node", restoredNode)
	}

	trust, _ := vm.GetNodeTrust("target-node")
	if trust.Status != StatusActive {
		t.Errorf("Status = %v, want %v", trust.Status, StatusActive)
	}
}

func TestStartStop(t *testing.T) {
	vm := createTestVotingManager(t)

	err := vm.Start()
	if err != nil {
		t.Fatalf("Start() error = %v", err)
	}

	// 创建一些数据
	vm.CreateProposal(VoteKick, "target", "Test")

	err = vm.Stop()
	if err != nil {
		t.Fatalf("Stop() error = %v", err)
	}
}

func TestPersistence(t *testing.T) {
	tempDir := t.TempDir()

	config := &VotingConfig{
		NodeID:           "test-node",
		DataDir:          tempDir,
		PassThreshold:    0.6,
		QuorumThreshold:  0.3,
		ProposalDuration: 1 * time.Hour,
		CleanupInterval:  1 * time.Hour,
		ReputationWeight: 0.7,
		StakeWeight:      0.3,
		MinRepToVote:     10,
		MinRepToPropose:  30,
	}

	// 创建第一个实例
	vm1, _ := NewVotingManager(config)
	vm1.RegisterNode("test-node", 50, 30)
	vm1.RegisterNode("node-002", 60, 40)
	vm1.CreateProposal(VoteKick, "target", "Test")

	// 保存
	err := vm1.saveToDisk()
	if err != nil {
		t.Fatalf("saveToDisk() error = %v", err)
	}

	// 创建第二个实例并加载
	vm2, _ := NewVotingManager(config)
	err = vm2.loadFromDisk()
	if err != nil {
		t.Fatalf("loadFromDisk() error = %v", err)
	}

	// 验证数据
	proposals := vm2.ListProposals("", 10, 0)
	if len(proposals) != 1 {
		t.Errorf("Proposals count = %d, want 1", len(proposals))
	}

	_, err = vm2.GetNodeTrust("node-002")
	if err != nil {
		t.Error("Node should be loaded")
	}
}

func TestGetStats(t *testing.T) {
	vm := createTestVotingManager(t)
	vm.RegisterNode("node-002", 60, 40)
	vm.RegisterNode("node-003", 70, 50)

	// 创建提案
	vm.CreateProposal(VoteKick, "target-1", "Test 1")
	vm.CreateProposal(VoteKick, "target-2", "Test 2")

	// 手动设置一个为已通过
	vm.mu.Lock()
	for id := range vm.proposals {
		vm.proposals[id].Status = ProposalPassed
		break
	}
	vm.mu.Unlock()

	stats := vm.GetStats()

	if stats.TotalProposals != 2 {
		t.Errorf("TotalProposals = %d, want 2", stats.TotalProposals)
	}
	if stats.PendingProposals != 1 {
		t.Errorf("PendingProposals = %d, want 1", stats.PendingProposals)
	}
	if stats.PassedProposals != 1 {
		t.Errorf("PassedProposals = %d, want 1", stats.PassedProposals)
	}
	// 3个注册节点 + test-node-001
	if stats.TotalNodes != 3 {
		t.Errorf("TotalNodes = %d, want 3", stats.TotalNodes)
	}
}

func TestCheckExpiredProposals(t *testing.T) {
	vm := createTestVotingManager(t)

	// 创建一个已过期的提案
	proposal, _ := vm.CreateProposal(VoteKick, "target", "Test")

	// 手动设置过期
	vm.mu.Lock()
	vm.proposals[proposal.ID].ExpiresAt = time.Now().Add(-1 * time.Hour)
	vm.mu.Unlock()

	// 检查过期
	vm.checkExpiredProposals()

	p, _ := vm.GetProposal(proposal.ID)
	if p.Status != ProposalExpired {
		t.Errorf("Status = %v, want %v", p.Status, ProposalExpired)
	}
}

func TestCallbacks(t *testing.T) {
	vm := createTestVotingManager(t)

	var createdProposal *Proposal
	var castVote *Vote
	var finalizedProposal *Proposal

	vm.SetOnProposalCreated(func(p *Proposal) {
		createdProposal = p
	})
	vm.SetOnVoteCast(func(v *Vote) {
		castVote = v
	})
	vm.SetOnProposalFinalized(func(p *Proposal) {
		finalizedProposal = p
	})

	// 注册更多节点以达到法定人数
	vm.RegisterNode("node-002", 60, 40)

	// 创建提案
	proposal, _ := vm.CreateProposal(VoteKick, "target", "Test")
	time.Sleep(50 * time.Millisecond)
	if createdProposal == nil || createdProposal.ID != proposal.ID {
		t.Error("OnProposalCreated callback not triggered correctly")
	}

	// 投票
	vm.CastVote(proposal.ID, ChoiceYes, "")
	time.Sleep(50 * time.Millisecond)
	if castVote == nil {
		t.Error("OnVoteCast callback not triggered")
	}

	// 另一个节点投票以结束
	vm.config.NodeID = "node-002"
	vm.CastVote(proposal.ID, ChoiceYes, "")
	time.Sleep(50 * time.Millisecond)
	if finalizedProposal == nil {
		t.Error("OnProposalFinalized callback not triggered")
	}
}

func TestAbstainVote(t *testing.T) {
	config := createTestConfig(t)
	config.QuorumThreshold = 0.2
	config.PassThreshold = 0.5
	vm, _ := NewVotingManager(config)

	vm.RegisterNode("node-001", 50, 30)
	vm.RegisterNode("node-002", 60, 40)
	vm.RegisterNode("node-003", 70, 50)
	vm.RegisterNode("node-004", 80, 60) // 添加第四个节点以防止提前结束

	vm.config.NodeID = "node-001"
	proposal, _ := vm.CreateProposal(VoteKick, "target", "Test")

	// 一个同意，一个弃权，一个反对
	vm.config.NodeID = "node-001"
	vm.CastVote(proposal.ID, ChoiceYes, "")

	vm.config.NodeID = "node-002"
	vm.CastVote(proposal.ID, ChoiceAbstain, "")

	vm.config.NodeID = "node-003"
	vm.CastVote(proposal.ID, ChoiceNo, "")

	p, _ := vm.GetProposal(proposal.ID)
	
	// 计算当前投票结果（可能未finalize）
	var abstainWeight float64
	for _, vote := range p.Votes {
		if vote.Choice == ChoiceAbstain {
			abstainWeight = vote.Weight
		}
	}
	
	if abstainWeight == 0 {
		t.Error("AbstainWeight should be recorded in votes")
	}
}

func TestSetFunctions(t *testing.T) {
	vm := createTestVotingManager(t)

	// 测试设置函数
	vm.SetSignFunc(mockSignFunc)
	vm.SetVerifyFunc(mockVerifyFunc)
	vm.SetGetReputationFunc(func(nodeID string) float64 {
		return 50.0
	})

	// 使用设置的函数
	proposal, _ := vm.CreateProposal(VoteKick, "target", "Test")
	vote, err := vm.CastVote(proposal.ID, ChoiceYes, "")
	if err != nil {
		t.Fatalf("CastVote() error = %v", err)
	}
	if len(vote.Signature) == 0 {
		t.Error("Vote should be signed")
	}
}
