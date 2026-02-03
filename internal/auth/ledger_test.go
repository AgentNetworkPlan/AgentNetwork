package auth

import (
	"testing"
	"time"
)

func TestNewSignedLedger(t *testing.T) {
	ledger := NewSignedLedger(nil, nil)
	if ledger == nil {
		t.Fatal("签名账本不应为 nil")
	}
}

func TestSignedLedger_AddEntry(t *testing.T) {
	ledger := NewSignedLedger(nil, nil)

	data := NodeRegistrationData{
		NodeID:    "node-001",
		PublicKey: "test-public-key",
		Endpoint:  "127.0.0.1:8000",
	}

	entry, err := ledger.AddEntry(EntryNodeRegistration, "node-001", data)
	if err != nil {
		t.Fatalf("添加条目失败: %v", err)
	}

	if entry.ID == "" {
		t.Error("条目 ID 不应为空")
	}

	if entry.Type != EntryNodeRegistration {
		t.Error("条目类型不匹配")
	}

	if entry.Hash == "" {
		t.Error("条目哈希不应为空")
	}
}

func TestSignedLedger_ChainIntegrity(t *testing.T) {
	ledger := NewSignedLedger(nil, nil)

	// 添加多个条目
	for i := 0; i < 5; i++ {
		data := TaskSubmissionData{
			TaskID:      string(rune('a' + i)),
			TaskType:    "compute",
			Difficulty:  3,
			SubmitterID: "submitter-001",
		}
		ledger.AddEntry(EntryTaskSubmission, "submitter-001", data)
	}

	// 验证链完整性
	valid, failedAt := ledger.VerifyChain()
	if !valid {
		t.Errorf("链验证失败，位置: %d", failedAt)
	}
}

func TestSignedLedger_GetEntry(t *testing.T) {
	ledger := NewSignedLedger(nil, nil)

	data := NodeRegistrationData{NodeID: "node-001"}
	entry, _ := ledger.AddEntry(EntryNodeRegistration, "node-001", data)

	retrieved, err := ledger.GetEntry(entry.ID)
	if err != nil {
		t.Fatalf("获取条目失败: %v", err)
	}

	if retrieved.ID != entry.ID {
		t.Error("获取的条目 ID 不匹配")
	}
}

func TestSignedLedger_GetNodeEntries(t *testing.T) {
	ledger := NewSignedLedger(nil, nil)

	// 添加不同节点的条目
	ledger.AddEntry(EntryNodeRegistration, "node-001", NodeRegistrationData{NodeID: "node-001"})
	ledger.AddEntry(EntryNodeRegistration, "node-002", NodeRegistrationData{NodeID: "node-002"})
	ledger.AddEntry(EntryTaskSubmission, "node-001", TaskSubmissionData{TaskID: "task-1", SubmitterID: "node-001"})

	// 获取 node-001 的条目
	entries := ledger.GetNodeEntries("node-001")
	if len(entries) != 2 {
		t.Errorf("node-001 应有 2 条记录，实际有 %d", len(entries))
	}
}

func TestSignedLedger_GetTaskEntries(t *testing.T) {
	ledger := NewSignedLedger(nil, nil)

	// 添加任务相关条目
	ledger.AddEntry(EntryTaskSubmission, "node-001", TaskSubmissionData{TaskID: "task-001"})
	ledger.AddEntry(EntryProofSubmission, "node-002", ProofSubmissionData{TaskID: "task-001", WorkerID: "node-002"})
	ledger.AddEntry(EntryVerificationVote, "node-003", VerificationVoteData{TaskID: "task-001", VerifierID: "node-003"})

	// 获取任务相关条目
	entries := ledger.GetTaskEntries("task-001")
	if len(entries) != 3 {
		t.Errorf("task-001 应有 3 条记录，实际有 %d", len(entries))
	}
}

func TestSignedLedger_GetEntriesByType(t *testing.T) {
	ledger := NewSignedLedger(nil, nil)

	// 添加不同类型的条目
	ledger.AddEntry(EntryNodeRegistration, "node-001", NodeRegistrationData{})
	ledger.AddEntry(EntryNodeRegistration, "node-002", NodeRegistrationData{})
	ledger.AddEntry(EntryTaskSubmission, "node-001", TaskSubmissionData{})

	// 获取注册类型条目
	entries := ledger.GetEntriesByType(EntryNodeRegistration)
	if len(entries) != 2 {
		t.Errorf("应有 2 条注册记录，实际有 %d", len(entries))
	}
}

func TestSignedLedger_GetEntriesInRange(t *testing.T) {
	ledger := NewSignedLedger(nil, nil)

	start := time.Now()
	time.Sleep(10 * time.Millisecond)

	// 添加条目
	ledger.AddEntry(EntryNodeRegistration, "node-001", NodeRegistrationData{})

	time.Sleep(10 * time.Millisecond)
	end := time.Now()

	// 获取时间范围内的条目
	entries := ledger.GetEntriesInRange(start, end)
	if len(entries) != 1 {
		t.Errorf("时间范围内应有 1 条记录，实际有 %d", len(entries))
	}
}

func TestSignedLedger_AddWitness(t *testing.T) {
	ledger := NewSignedLedger(nil, nil)

	entry, _ := ledger.AddEntry(EntryTaskSubmission, "node-001", TaskSubmissionData{})

	// 添加见证者
	err := ledger.AddWitness(entry.ID, "witness-001")
	if err != nil {
		t.Fatalf("添加见证者失败: %v", err)
	}

	retrieved, _ := ledger.GetEntry(entry.ID)
	if len(retrieved.Witnesses) != 1 {
		t.Error("见证者数量应为 1")
	}

	// 重复添加不应增加
	ledger.AddWitness(entry.ID, "witness-001")
	retrieved, _ = ledger.GetEntry(entry.ID)
	if len(retrieved.Witnesses) != 1 {
		t.Error("重复添加见证者不应增加数量")
	}
}

func TestSignedLedger_MarkVerified(t *testing.T) {
	ledger := NewSignedLedger(nil, nil)

	entry, _ := ledger.AddEntry(EntryTaskSubmission, "node-001", TaskSubmissionData{})

	if entry.Verified {
		t.Error("新条目不应被标记为已验证")
	}

	err := ledger.MarkVerified(entry.ID)
	if err != nil {
		t.Fatalf("标记验证失败: %v", err)
	}

	retrieved, _ := ledger.GetEntry(entry.ID)
	if !retrieved.Verified {
		t.Error("条目应被标记为已验证")
	}
}

func TestSignedLedger_Count(t *testing.T) {
	ledger := NewSignedLedger(nil, nil)

	for i := 0; i < 5; i++ {
		ledger.AddEntry(EntryNodeRegistration, "node", NodeRegistrationData{})
	}

	if ledger.Count() != 5 {
		t.Errorf("条目数量应为 5，实际为 %d", ledger.Count())
	}
}

func TestSignedLedger_GetLatestEntry(t *testing.T) {
	ledger := NewSignedLedger(nil, nil)

	// 空账本
	if ledger.GetLatestEntry() != nil {
		t.Error("空账本的最新条目应为 nil")
	}

	// 添加条目
	ledger.AddEntry(EntryNodeRegistration, "node-001", NodeRegistrationData{})
	entry2, _ := ledger.AddEntry(EntryTaskSubmission, "node-001", TaskSubmissionData{})

	latest := ledger.GetLatestEntry()
	if latest.ID != entry2.ID {
		t.Error("最新条目不匹配")
	}
}

func TestSignedLedger_GetStats(t *testing.T) {
	ledger := NewSignedLedger(nil, nil)

	ledger.AddEntry(EntryNodeRegistration, "node-001", NodeRegistrationData{})
	ledger.AddEntry(EntryNodeRegistration, "node-002", NodeRegistrationData{})
	ledger.AddEntry(EntryTaskSubmission, "node-001", TaskSubmissionData{TaskID: "task-001"})

	stats := ledger.GetStats()

	if stats["total_entries"].(int) != 3 {
		t.Error("总条目数应为 3")
	}
	if stats["unique_nodes"].(int) != 2 {
		t.Error("唯一节点数应为 2")
	}
}

func TestSignedLedger_ExportImportState(t *testing.T) {
	ledger := NewSignedLedger(nil, nil)

	ledger.AddEntry(EntryNodeRegistration, "node-001", NodeRegistrationData{NodeID: "node-001"})
	ledger.AddEntry(EntryTaskSubmission, "node-001", TaskSubmissionData{TaskID: "task-001"})

	// 导出
	data, err := ledger.ExportState()
	if err != nil {
		t.Fatalf("导出状态失败: %v", err)
	}

	// 导入到新账本
	ledger2 := NewSignedLedger(nil, nil)
	err = ledger2.ImportState(data)
	if err != nil {
		t.Fatalf("导入状态失败: %v", err)
	}

	if ledger2.Count() != 2 {
		t.Errorf("导入后条目数应为 2，实际为 %d", ledger2.Count())
	}

	// 验证索引重建
	nodeEntries := ledger2.GetNodeEntries("node-001")
	if len(nodeEntries) != 2 {
		t.Error("节点索引重建失败")
	}
}

func TestSignedLedger_WithSigner(t *testing.T) {
	identity, _ := NewNodeIdentity()

	signer := func(data []byte) ([]byte, error) {
		return identity.Sign(data)
	}

	verifier := func(nodeID string, data, sig []byte) bool {
		return identity.Verify(data, sig)
	}

	ledger := NewSignedLedger(signer, verifier)

	entry, err := ledger.AddEntry(EntryNodeRegistration, identity.NodeID(), NodeRegistrationData{
		NodeID: identity.NodeID(),
	})
	if err != nil {
		t.Fatalf("添加条目失败: %v", err)
	}

	if len(entry.Signature) == 0 {
		t.Error("条目应有签名")
	}

	// 验证条目
	if !ledger.VerifyEntry(entry) {
		t.Error("条目验证应通过")
	}
}

func TestAuditLog(t *testing.T) {
	ledger := NewSignedLedger(nil, nil)
	auditLog := NewAuditLog(ledger)

	// 添加一些条目
	ledger.AddEntry(EntryNodeRegistration, "node-001", NodeRegistrationData{NodeID: "node-001"})
	ledger.AddEntry(EntryTaskSubmission, "node-001", TaskSubmissionData{TaskID: "task-001"})

	// 获取节点历史
	history := auditLog.GetNodeHistory("node-001")
	if len(history) != 2 {
		t.Errorf("节点历史应有 2 条，实际有 %d", len(history))
	}
}

func TestAuditLog_GetTaskHistory(t *testing.T) {
	ledger := NewSignedLedger(nil, nil)
	auditLog := NewAuditLog(ledger)

	// 添加任务相关条目
	ledger.AddEntry(EntryTaskSubmission, "node-001", TaskSubmissionData{TaskID: "task-001"})
	ledger.AddEntry(EntryProofSubmission, "node-002", ProofSubmissionData{TaskID: "task-001", WorkerID: "node-002"})

	// 获取任务历史
	history := auditLog.GetTaskHistory("task-001")
	if len(history) != 2 {
		t.Errorf("任务历史应有 2 条，实际有 %d", len(history))
	}
}

func TestAuditLog_SearchEntries(t *testing.T) {
	ledger := NewSignedLedger(nil, nil)
	auditLog := NewAuditLog(ledger)

	// 添加条目
	ledger.AddEntry(EntryNodeRegistration, "node-001", NodeRegistrationData{})
	ledger.AddEntry(EntryNodeRegistration, "node-002", NodeRegistrationData{})
	ledger.AddEntry(EntryTaskSubmission, "node-001", TaskSubmissionData{TaskID: "task-001"})

	// 搜索特定节点的条目
	results := auditLog.SearchEntries(map[string]interface{}{
		"node_id": "node-001",
	})
	if len(results) != 2 {
		t.Errorf("搜索结果应有 2 条，实际有 %d", len(results))
	}

	// 搜索特定类型的条目
	results = auditLog.SearchEntries(map[string]interface{}{
		"type": EntryNodeRegistration,
	})
	if len(results) != 2 {
		t.Errorf("搜索结果应有 2 条，实际有 %d", len(results))
	}
}
