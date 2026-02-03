package auth

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"sync"
	"time"
)

// LedgerEntryType 账本条目类型
type LedgerEntryType string

const (
	EntryNodeRegistration   LedgerEntryType = "node_registration"
	EntryTaskSubmission     LedgerEntryType = "task_submission"
	EntryProofSubmission    LedgerEntryType = "proof_submission"
	EntryVerificationVote   LedgerEntryType = "verification_vote"
	EntryReputationChange   LedgerEntryType = "reputation_change"
	EntryTokenTransfer      LedgerEntryType = "token_transfer"
	EntryTokenReward        LedgerEntryType = "token_reward"
	EntryCommitteeDecision  LedgerEntryType = "committee_decision"
	EntrySybilReport        LedgerEntryType = "sybil_report"
)

// LedgerEntry 账本条目
type LedgerEntry struct {
	ID           string          `json:"id"`
	Type         LedgerEntryType `json:"type"`
	Timestamp    time.Time       `json:"timestamp"`
	NodeID       string          `json:"node_id"`       // 操作发起者
	TargetNodeID string          `json:"target_node_id,omitempty"` // 目标节点
	TaskID       string          `json:"task_id,omitempty"`
	Data         json.RawMessage `json:"data"`          // 具体数据
	PrevHash     string          `json:"prev_hash"`     // 前一条目的哈希
	Hash         string          `json:"hash"`          // 本条目哈希
	Signature    []byte          `json:"signature"`     // 发起者签名
	Verified     bool            `json:"verified"`      // 是否已验证
	Witnesses    []string        `json:"witnesses,omitempty"` // 见证者列表
}

// NodeRegistrationData 节点注册数据
type NodeRegistrationData struct {
	NodeID    string `json:"node_id"`
	PublicKey string `json:"public_key"`
	Endpoint  string `json:"endpoint,omitempty"`
}

// TaskSubmissionData 任务提交数据
type TaskSubmissionData struct {
	TaskID      string `json:"task_id"`
	TaskType    string `json:"task_type"`
	Difficulty  int    `json:"difficulty"`
	SubmitterID string `json:"submitter_id"`
	WorkerID    string `json:"worker_id,omitempty"`
}

// ProofSubmissionData 证明提交数据
type ProofSubmissionData struct {
	TaskID      string  `json:"task_id"`
	WorkerID    string  `json:"worker_id"`
	ProofHash   string  `json:"proof_hash"`
	ResultHash  string  `json:"result_hash"`
}

// VerificationVoteData 验证投票数据
type VerificationVoteData struct {
	TaskID     string  `json:"task_id"`
	VerifierID string  `json:"verifier_id"`
	IsApproved bool    `json:"is_approved"`
	Weight     float64 `json:"weight"`
	Comment    string  `json:"comment,omitempty"`
}

// ReputationChangeData 信誉变更数据
type ReputationChangeData struct {
	NodeID    string  `json:"node_id"`
	OldScore  float64 `json:"old_score"`
	NewScore  float64 `json:"new_score"`
	Reason    string  `json:"reason"`
}

// TokenTransferData 代币转账数据
type TokenTransferData struct {
	FromNodeID string  `json:"from_node_id"`
	ToNodeID   string  `json:"to_node_id"`
	Amount     float64 `json:"amount"`
	Reason     string  `json:"reason,omitempty"`
}

// TokenRewardData 代币奖励数据
type TokenRewardData struct {
	NodeID string  `json:"node_id"`
	Amount float64 `json:"amount"`
	TaskID string  `json:"task_id,omitempty"`
	Type   string  `json:"type"`
}

// CommitteeDecisionData 委员会决策数据
type CommitteeDecisionData struct {
	TaskID         string             `json:"task_id"`
	Decision       string             `json:"decision"`
	ApprovalRate   float64            `json:"approval_rate"`
	TotalWeight    float64            `json:"total_weight"`
	Votes          []VerificationVoteData `json:"votes"`
}

// SignedLedger 签名账本
type SignedLedger struct {
	entries    []*LedgerEntry
	entryIndex map[string]*LedgerEntry // 按 ID 索引
	nodeIndex  map[string][]*LedgerEntry // 按节点索引
	taskIndex  map[string][]*LedgerEntry // 按任务索引
	mu         sync.RWMutex
	signer     func(data []byte) ([]byte, error) // 签名函数
	verifier   func(nodeID string, data, sig []byte) bool // 验证函数
}

// NewSignedLedger 创建签名账本
func NewSignedLedger(
	signer func(data []byte) ([]byte, error),
	verifier func(nodeID string, data, sig []byte) bool,
) *SignedLedger {
	return &SignedLedger{
		entries:    make([]*LedgerEntry, 0),
		entryIndex: make(map[string]*LedgerEntry),
		nodeIndex:  make(map[string][]*LedgerEntry),
		taskIndex:  make(map[string][]*LedgerEntry),
		signer:     signer,
		verifier:   verifier,
	}
}

// AddEntry 添加条目
func (sl *SignedLedger) AddEntry(entryType LedgerEntryType, nodeID string, data interface{}) (*LedgerEntry, error) {
	sl.mu.Lock()
	defer sl.mu.Unlock()

	// 序列化数据
	dataJSON, err := json.Marshal(data)
	if err != nil {
		return nil, err
	}

	// 获取前一个哈希
	prevHash := "genesis"
	if len(sl.entries) > 0 {
		prevHash = sl.entries[len(sl.entries)-1].Hash
	}

	entry := &LedgerEntry{
		ID:        generateEntryID(),
		Type:      entryType,
		Timestamp: time.Now(),
		NodeID:    nodeID,
		Data:      dataJSON,
		PrevHash:  prevHash,
	}

	// 提取相关 ID
	sl.extractRelatedIDs(entry, data)

	// 计算哈希
	entry.Hash = sl.calculateEntryHash(entry)

	// 签名
	if sl.signer != nil {
		sig, err := sl.signer([]byte(entry.Hash))
		if err != nil {
			return nil, err
		}
		entry.Signature = sig
	}

	// 添加到账本
	sl.entries = append(sl.entries, entry)
	sl.entryIndex[entry.ID] = entry

	// 更新索引
	sl.nodeIndex[nodeID] = append(sl.nodeIndex[nodeID], entry)
	if entry.TaskID != "" {
		sl.taskIndex[entry.TaskID] = append(sl.taskIndex[entry.TaskID], entry)
	}

	return entry, nil
}

// extractRelatedIDs 提取相关 ID
func (sl *SignedLedger) extractRelatedIDs(entry *LedgerEntry, data interface{}) {
	switch d := data.(type) {
	case TaskSubmissionData:
		entry.TaskID = d.TaskID
		entry.TargetNodeID = d.WorkerID
	case ProofSubmissionData:
		entry.TaskID = d.TaskID
		entry.NodeID = d.WorkerID
	case VerificationVoteData:
		entry.TaskID = d.TaskID
	case ReputationChangeData:
		entry.TargetNodeID = d.NodeID
	case TokenTransferData:
		entry.TargetNodeID = d.ToNodeID
	case TokenRewardData:
		entry.TaskID = d.TaskID
		entry.TargetNodeID = d.NodeID
	case CommitteeDecisionData:
		entry.TaskID = d.TaskID
	}
}

// calculateEntryHash 计算条目哈希
func (sl *SignedLedger) calculateEntryHash(entry *LedgerEntry) string {
	hashData := struct {
		ID        string
		Type      LedgerEntryType
		Timestamp int64
		NodeID    string
		Data      string
		PrevHash  string
	}{
		ID:        entry.ID,
		Type:      entry.Type,
		Timestamp: entry.Timestamp.UnixNano(),
		NodeID:    entry.NodeID,
		Data:      string(entry.Data),
		PrevHash:  entry.PrevHash,
	}

	data, _ := json.Marshal(hashData)
	hash := sha256.Sum256(data)
	return hex.EncodeToString(hash[:])
}

// GetEntry 获取条目
func (sl *SignedLedger) GetEntry(id string) (*LedgerEntry, error) {
	sl.mu.RLock()
	defer sl.mu.RUnlock()

	entry, exists := sl.entryIndex[id]
	if !exists {
		return nil, errors.New("条目不存在")
	}
	return entry, nil
}

// GetNodeEntries 获取节点的所有条目
func (sl *SignedLedger) GetNodeEntries(nodeID string) []*LedgerEntry {
	sl.mu.RLock()
	defer sl.mu.RUnlock()

	entries := sl.nodeIndex[nodeID]
	result := make([]*LedgerEntry, len(entries))
	copy(result, entries)
	return result
}

// GetTaskEntries 获取任务的所有条目
func (sl *SignedLedger) GetTaskEntries(taskID string) []*LedgerEntry {
	sl.mu.RLock()
	defer sl.mu.RUnlock()

	entries := sl.taskIndex[taskID]
	result := make([]*LedgerEntry, len(entries))
	copy(result, entries)
	return result
}

// GetEntriesByType 获取指定类型的条目
func (sl *SignedLedger) GetEntriesByType(entryType LedgerEntryType) []*LedgerEntry {
	sl.mu.RLock()
	defer sl.mu.RUnlock()

	var result []*LedgerEntry
	for _, entry := range sl.entries {
		if entry.Type == entryType {
			result = append(result, entry)
		}
	}
	return result
}

// GetEntriesInRange 获取时间范围内的条目
func (sl *SignedLedger) GetEntriesInRange(start, end time.Time) []*LedgerEntry {
	sl.mu.RLock()
	defer sl.mu.RUnlock()

	var result []*LedgerEntry
	for _, entry := range sl.entries {
		if entry.Timestamp.After(start) && entry.Timestamp.Before(end) {
			result = append(result, entry)
		}
	}
	return result
}

// VerifyEntry 验证条目
func (sl *SignedLedger) VerifyEntry(entry *LedgerEntry) bool {
	// 验证哈希
	expectedHash := sl.calculateEntryHash(entry)
	if entry.Hash != expectedHash {
		return false
	}

	// 验证签名
	if sl.verifier != nil && entry.Signature != nil {
		if !sl.verifier(entry.NodeID, []byte(entry.Hash), entry.Signature) {
			return false
		}
	}

	return true
}

// VerifyChain 验证整个链
func (sl *SignedLedger) VerifyChain() (bool, int) {
	sl.mu.RLock()
	defer sl.mu.RUnlock()

	prevHash := "genesis"
	for i, entry := range sl.entries {
		// 验证前一个哈希
		if entry.PrevHash != prevHash {
			return false, i
		}

		// 验证条目
		if !sl.VerifyEntry(entry) {
			return false, i
		}

		prevHash = entry.Hash
	}

	return true, -1
}

// AddWitness 添加见证者
func (sl *SignedLedger) AddWitness(entryID, witnessID string) error {
	sl.mu.Lock()
	defer sl.mu.Unlock()

	entry, exists := sl.entryIndex[entryID]
	if !exists {
		return errors.New("条目不存在")
	}

	// 检查是否已经是见证者
	for _, w := range entry.Witnesses {
		if w == witnessID {
			return nil
		}
	}

	entry.Witnesses = append(entry.Witnesses, witnessID)
	return nil
}

// MarkVerified 标记为已验证
func (sl *SignedLedger) MarkVerified(entryID string) error {
	sl.mu.Lock()
	defer sl.mu.Unlock()

	entry, exists := sl.entryIndex[entryID]
	if !exists {
		return errors.New("条目不存在")
	}

	entry.Verified = true
	return nil
}

// GetAllEntries 获取所有条目
func (sl *SignedLedger) GetAllEntries() []*LedgerEntry {
	sl.mu.RLock()
	defer sl.mu.RUnlock()

	result := make([]*LedgerEntry, len(sl.entries))
	copy(result, sl.entries)
	return result
}

// GetLatestEntry 获取最新条目
func (sl *SignedLedger) GetLatestEntry() *LedgerEntry {
	sl.mu.RLock()
	defer sl.mu.RUnlock()

	if len(sl.entries) == 0 {
		return nil
	}
	return sl.entries[len(sl.entries)-1]
}

// Count 获取条目数量
func (sl *SignedLedger) Count() int {
	sl.mu.RLock()
	defer sl.mu.RUnlock()
	return len(sl.entries)
}

// ExportState 导出状态
func (sl *SignedLedger) ExportState() ([]byte, error) {
	sl.mu.RLock()
	defer sl.mu.RUnlock()

	return json.Marshal(sl.entries)
}

// ImportState 导入状态
func (sl *SignedLedger) ImportState(data []byte) error {
	sl.mu.Lock()
	defer sl.mu.Unlock()

	var entries []*LedgerEntry
	if err := json.Unmarshal(data, &entries); err != nil {
		return err
	}

	// 重建索引
	sl.entries = entries
	sl.entryIndex = make(map[string]*LedgerEntry)
	sl.nodeIndex = make(map[string][]*LedgerEntry)
	sl.taskIndex = make(map[string][]*LedgerEntry)

	for _, entry := range entries {
		sl.entryIndex[entry.ID] = entry
		sl.nodeIndex[entry.NodeID] = append(sl.nodeIndex[entry.NodeID], entry)
		if entry.TaskID != "" {
			sl.taskIndex[entry.TaskID] = append(sl.taskIndex[entry.TaskID], entry)
		}
	}

	return nil
}

// GetStats 获取统计信息
func (sl *SignedLedger) GetStats() map[string]interface{} {
	sl.mu.RLock()
	defer sl.mu.RUnlock()

	typeCounts := make(map[LedgerEntryType]int)
	for _, entry := range sl.entries {
		typeCounts[entry.Type]++
	}

	return map[string]interface{}{
		"total_entries":  len(sl.entries),
		"unique_nodes":   len(sl.nodeIndex),
		"unique_tasks":   len(sl.taskIndex),
		"entry_by_type":  typeCounts,
	}
}

// generateEntryID 生成条目 ID
func generateEntryID() string {
	data := make([]byte, 16)
	timestamp := time.Now().UnixNano()
	for i := 0; i < 8; i++ {
		data[i] = byte(timestamp >> (8 * i))
	}
	hash := sha256.Sum256(data)
	return hex.EncodeToString(hash[:8])
}

// AuditLog 审计日志
type AuditLog struct {
	ledger *SignedLedger
}

// NewAuditLog 创建审计日志
func NewAuditLog(ledger *SignedLedger) *AuditLog {
	return &AuditLog{ledger: ledger}
}

// GetNodeHistory 获取节点历史
func (al *AuditLog) GetNodeHistory(nodeID string) []map[string]interface{} {
	entries := al.ledger.GetNodeEntries(nodeID)
	history := make([]map[string]interface{}, len(entries))

	for i, entry := range entries {
		history[i] = map[string]interface{}{
			"id":        entry.ID,
			"type":      entry.Type,
			"timestamp": entry.Timestamp,
			"verified":  entry.Verified,
			"witnesses": len(entry.Witnesses),
		}
	}

	return history
}

// GetTaskHistory 获取任务历史
func (al *AuditLog) GetTaskHistory(taskID string) []map[string]interface{} {
	entries := al.ledger.GetTaskEntries(taskID)
	history := make([]map[string]interface{}, len(entries))

	for i, entry := range entries {
		history[i] = map[string]interface{}{
			"id":        entry.ID,
			"type":      entry.Type,
			"node":      entry.NodeID,
			"timestamp": entry.Timestamp,
			"verified":  entry.Verified,
		}
	}

	return history
}

// SearchEntries 搜索条目
func (al *AuditLog) SearchEntries(criteria map[string]interface{}) []*LedgerEntry {
	entries := al.ledger.GetAllEntries()
	var results []*LedgerEntry

	for _, entry := range entries {
		match := true

		if nodeID, ok := criteria["node_id"].(string); ok && nodeID != "" {
			if entry.NodeID != nodeID && entry.TargetNodeID != nodeID {
				match = false
			}
		}

		if taskID, ok := criteria["task_id"].(string); ok && taskID != "" {
			if entry.TaskID != taskID {
				match = false
			}
		}

		if entryType, ok := criteria["type"].(LedgerEntryType); ok {
			if entry.Type != entryType {
				match = false
			}
		}

		if startTime, ok := criteria["start_time"].(time.Time); ok {
			if entry.Timestamp.Before(startTime) {
				match = false
			}
		}

		if endTime, ok := criteria["end_time"].(time.Time); ok {
			if entry.Timestamp.After(endTime) {
				match = false
			}
		}

		if match {
			results = append(results, entry)
		}
	}

	return results
}
