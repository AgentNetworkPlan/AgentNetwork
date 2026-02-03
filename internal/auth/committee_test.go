package auth

import (
	"testing"
	"time"
)

func TestNewVerificationCommittee(t *testing.T) {
	committee := NewVerificationCommittee(3, 0.6)

	if committee.minMembers != 3 {
		t.Errorf("最小成员数应为 3，实际为 %d", committee.minMembers)
	}

	if committee.quorum != 0.6 {
		t.Errorf("法定人数比例应为 0.6，实际为 %f", committee.quorum)
	}
}

func TestCommittee_AddRemoveMember(t *testing.T) {
	committee := NewVerificationCommittee(3, 0.6)

	// 添加成员
	err := committee.AddMember("node-001", "pub-key-001", 0.8)
	if err != nil {
		t.Fatalf("添加成员失败: %v", err)
	}

	// 获取成员
	member, err := committee.GetMember("node-001")
	if err != nil {
		t.Fatalf("获取成员失败: %v", err)
	}

	if member.NodeID != "node-001" {
		t.Error("成员 ID 不匹配")
	}

	if member.ReputationScore != 0.8 {
		t.Errorf("信誉分应为 0.8，实际为 %f", member.ReputationScore)
	}

	// 移除成员
	err = committee.RemoveMember("node-001")
	if err != nil {
		t.Fatalf("移除成员失败: %v", err)
	}

	_, err = committee.GetMember("node-001")
	if err == nil {
		t.Error("移除后不应能获取成员")
	}
}

func TestCommittee_UpdateMemberReputation(t *testing.T) {
	committee := NewVerificationCommittee(3, 0.6)

	committee.AddMember("node-001", "pub-key-001", 0.5)

	// 更新信誉
	err := committee.UpdateMemberReputation("node-001", 0.9)
	if err != nil {
		t.Fatalf("更新信誉失败: %v", err)
	}

	member, _ := committee.GetMember("node-001")
	if member.ReputationScore != 0.9 {
		t.Errorf("信誉分应为 0.9，实际为 %f", member.ReputationScore)
	}
}

func TestCommittee_SetMemberActive(t *testing.T) {
	committee := NewVerificationCommittee(3, 0.6)

	committee.AddMember("node-001", "pub-key-001", 0.8)

	// 设置为非活跃
	err := committee.SetMemberActive("node-001", false)
	if err != nil {
		t.Fatalf("设置活跃状态失败: %v", err)
	}

	member, _ := committee.GetMember("node-001")
	if member.IsActive {
		t.Error("成员应该是非活跃的")
	}
}

func TestCommittee_SelectVerifiers(t *testing.T) {
	committee := NewVerificationCommittee(3, 0.6)

	// 添加 5 个成员
	for i := 0; i < 5; i++ {
		nodeID := string(rune('a' + i))
		committee.AddMember(nodeID, "pub-key-"+nodeID, 0.7+float64(i)*0.05)
	}

	// 选择 3 个验证者
	verifiers, err := committee.SelectVerifiers("task-001", 3)
	if err != nil {
		t.Fatalf("选择验证者失败: %v", err)
	}

	if len(verifiers) != 3 {
		t.Errorf("应选择 3 个验证者，实际选择了 %d", len(verifiers))
	}

	// 验证没有重复
	nodeSet := make(map[string]bool)
	for _, v := range verifiers {
		if nodeSet[v.NodeID] {
			t.Error("选择了重复的节点")
		}
		nodeSet[v.NodeID] = true
	}
}

func TestCommittee_GetActiveMembers(t *testing.T) {
	committee := NewVerificationCommittee(3, 0.6)

	// 添加成员，部分设为非活跃
	committee.AddMember("node-a", "pub-key-a", 0.8)
	committee.AddMember("node-b", "pub-key-b", 0.8)
	committee.AddMember("node-c", "pub-key-c", 0.8)
	
	committee.SetMemberActive("node-b", false)

	activeMembers := committee.GetActiveMembers()
	if len(activeMembers) != 2 {
		t.Errorf("活跃成员数应为 2，实际为 %d", len(activeMembers))
	}
}

func TestCommitteeManager(t *testing.T) {
	manager := NewCommitteeManager(3, 0.6)

	committee := manager.GetCommittee()
	if committee == nil {
		t.Error("委员会不应为 nil")
	}

	// 添加成员
	committee.AddMember("node-001", "pub-key-001", 0.8)
	committee.AddMember("node-002", "pub-key-002", 0.8)
	committee.AddMember("node-003", "pub-key-003", 0.8)

	// 验证成员数
	activeMembers := committee.GetActiveMembers()
	if len(activeMembers) != 3 {
		t.Errorf("应有 3 个成员，实际有 %d", len(activeMembers))
	}
}

func TestVerificationSession(t *testing.T) {
	manager := NewCommitteeManager(3, 0.6)
	committee := manager.GetCommittee()

	// 添加成员
	for i := 0; i < 5; i++ {
		nodeID := string(rune('a' + i))
		committee.AddMember(nodeID, "pub-key-"+nodeID, 0.8)
	}

	// 创建 Proof
	identity, _ := NewNodeIdentity()
	proof := NewProofOfTask("task-001", identity.NodeID(), []byte("result"), 1000)
	proof.Sign(identity)

	// 发起验证
	request, err := manager.InitiateVerification(proof, 3, 5*time.Minute)
	if err != nil {
		t.Fatalf("发起验证失败: %v", err)
	}

	if len(request.Verifiers) != 3 {
		t.Errorf("应有 3 个验证者，实际有 %d", len(request.Verifiers))
	}

	// 提交验证结果（可能在达到共识后停止）
	successCount := 0
	for _, verifier := range request.Verifiers {
		verification := &TaskVerification{
			TaskID:     proof.TaskID,
			VerifierID: verifier.NodeID,
			IsValid:    true,
			Timestamp:  time.Now(),
		}
		err := manager.SubmitVerification(verification)
		if err != nil {
			// 验证完成后会返回错误，这是正常的
			break
		}
		successCount++
	}

	// 至少应该成功提交了一些验证
	if successCount == 0 {
		t.Error("至少应该有一次验证成功")
	}

	// 获取结果
	result, err := manager.GetVerificationResult("task-001")
	if err != nil {
		t.Fatalf("获取结果失败: %v", err)
	}

	if result == nil {
		t.Error("应该有验证结果")
	} else if !*result {
		t.Error("验证结果应该是通过")
	}
}

func TestWeightedRandomSelection(t *testing.T) {
	committee := NewVerificationCommittee(3, 0.6)

	// 添加不同权重的成员
	members := []struct {
		nodeID     string
		reputation float64
	}{
		{"node-a", 0.9},
		{"node-b", 0.8},
		{"node-c", 0.7},
		{"node-d", 0.6},
		{"node-e", 0.5},
	}

	for _, m := range members {
		committee.AddMember(m.nodeID, "pub-key-"+m.nodeID, m.reputation)
	}

	// 多次选择，验证结果合理
	for i := 0; i < 10; i++ {
		selected, err := committee.SelectVerifiers("task-"+string(rune('0'+i)), 3)
		if err != nil {
			t.Fatalf("选择验证者失败: %v", err)
		}
		if len(selected) != 3 {
			t.Errorf("应选择 3 个验证者，实际选择了 %d", len(selected))
		}
	}
}

func TestCalculateVotingPower(t *testing.T) {
	// 测试投票权重计算
	testCases := []struct {
		reputation float64
		minPower   float64
		maxPower   float64
	}{
		{0.0, 0.5, 0.5},
		{0.5, 1.0, 1.3},
		{1.0, 1.9, 2.1},
		{-0.5, 0.5, 0.5}, // 负信誉应该得到最低权重
	}

	for _, tc := range testCases {
		power := calculateVotingPower(tc.reputation)
		if power < tc.minPower || power > tc.maxPower {
			t.Errorf("信誉 %f 的投票权重 %f 不在预期范围 [%f, %f] 内", 
				tc.reputation, power, tc.minPower, tc.maxPower)
		}
	}
}
