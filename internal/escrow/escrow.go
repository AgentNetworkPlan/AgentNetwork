// Package escrow 提供押金托管功能
// Task 27: 委托任务与文件传输
package escrow

import (
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"time"
)

var (
	ErrEscrowNotFound     = errors.New("escrow not found")
	ErrInsufficientFunds  = errors.New("insufficient funds")
	ErrEscrowLocked       = errors.New("escrow is locked")
	ErrEscrowNotLocked    = errors.New("escrow is not locked")
	ErrInvalidSignature   = errors.New("invalid signature")
	ErrEscrowDisputed     = errors.New("escrow is disputed")
	ErrUnauthorized       = errors.New("unauthorized operation")
	ErrAlreadyDeposited   = errors.New("already deposited")
)

// EscrowStatus 押金状态
type EscrowStatus string

const (
	EscrowPending   EscrowStatus = "pending"   // 等待存入
	EscrowLocked    EscrowStatus = "locked"    // 已锁定
	EscrowReleased  EscrowStatus = "released"  // 已释放
	EscrowDisputed  EscrowStatus = "disputed"  // 争议中
	EscrowRefunded  EscrowStatus = "refunded"  // 已退款
	EscrowForfeited EscrowStatus = "forfeited" // 已没收
)

// Escrow 押金托管
type Escrow struct {
	ID     string `json:"id"`
	TaskID string `json:"task_id"`

	// 押金明细
	Deposits    map[string]float64 `json:"deposits"`     // nodeID -> amount
	TotalAmount float64            `json:"total_amount"`

	// 要求的押金金额
	RequiredDeposits map[string]float64 `json:"required_deposits"` // nodeID -> required amount

	// 状态
	Status EscrowStatus `json:"status"`

	// 释放条件
	ReleaseCondition string  `json:"release_condition"`
	ReleasedTo       string  `json:"released_to"`
	ReleasedAmount   float64 `json:"released_amount"`

	// 多签名锁定
	LockSignatures   map[string]string `json:"lock_signatures"`   // nodeID -> signature
	UnlockSignatures map[string]string `json:"unlock_signatures"` // nodeID -> signature

	// 参与方
	Participants []string `json:"participants"` // 参与押金的节点

	// 时间
	CreatedAt   int64 `json:"created_at"`
	LockedAt    int64 `json:"locked_at"`
	LockedUntil int64 `json:"locked_until"` // 锁定截止时间
	ReleasedAt  int64 `json:"released_at"`

	// 争议信息
	DisputeReason string `json:"dispute_reason,omitempty"`
	DisputedBy    string `json:"disputed_by,omitempty"`
	DisputedAt    int64  `json:"disputed_at,omitempty"`
}

// EscrowConfig 托管配置
type EscrowConfig struct {
	DataDir               string        // 数据目录
	DefaultLockTime       time.Duration // 默认锁定时间
	MinDeposit            float64       // 最小押金
	MaxDeposit            float64       // 最大押金
	DisputeTimeout        time.Duration // 争议超时
	AutoReleaseDelay      time.Duration // 自动释放延迟
	MinArbitratorSigs     int           // Task44: 争议释放所需最少仲裁签名数
	ArbitratorSigThreshold float64      // Task44: 仲裁签名阈值比例 (0-1)
}

// DefaultEscrowConfig 返回默认配置
func DefaultEscrowConfig() *EscrowConfig {
	return &EscrowConfig{
		DataDir:               "data/escrow",
		DefaultLockTime:       7 * 24 * time.Hour, // 7天
		MinDeposit:            0.1,
		MaxDeposit:            1000.0,
		DisputeTimeout:        72 * time.Hour, // 3天
		AutoReleaseDelay:      24 * time.Hour, // 1天
		MinArbitratorSigs:     2,              // Task44: 默认需要至少2个仲裁签名
		ArbitratorSigThreshold: 0.5,           // Task44: 默认需要>50%仲裁签名
	}
}

// EscrowManager 押金托管管理器
type EscrowManager struct {
	mu     sync.RWMutex
	config *EscrowConfig

	// 托管记录
	escrows map[string]*Escrow // escrowID -> escrow

	// 索引
	escrowsByTask   map[string]string   // taskID -> escrowID
	escrowsByNode   map[string][]string // nodeID -> []escrowID
	escrowsByStatus map[EscrowStatus][]string
}

// NewEscrowManager 创建押金托管管理器
func NewEscrowManager(config *EscrowConfig) *EscrowManager {
	if config == nil {
		config = DefaultEscrowConfig()
	}

	em := &EscrowManager{
		config:          config,
		escrows:         make(map[string]*Escrow),
		escrowsByTask:   make(map[string]string),
		escrowsByNode:   make(map[string][]string),
		escrowsByStatus: make(map[EscrowStatus][]string),
	}

	em.load()
	return em
}

// CreateEscrow 创建押金托管
func (em *EscrowManager) CreateEscrow(taskID string, requiredDeposits map[string]float64) (*Escrow, error) {
	em.mu.Lock()
	defer em.mu.Unlock()

	// 检查是否已存在
	if _, exists := em.escrowsByTask[taskID]; exists {
		return nil, errors.New("escrow already exists for this task")
	}

	// 验证押金金额
	var total float64
	participants := make([]string, 0, len(requiredDeposits))
	for nodeID, amount := range requiredDeposits {
		if amount < em.config.MinDeposit {
			return nil, fmt.Errorf("deposit for %s is below minimum: %.2f < %.2f", nodeID, amount, em.config.MinDeposit)
		}
		if amount > em.config.MaxDeposit {
			return nil, fmt.Errorf("deposit for %s exceeds maximum: %.2f > %.2f", nodeID, amount, em.config.MaxDeposit)
		}
		total += amount
		participants = append(participants, nodeID)
	}

	escrow := &Escrow{
		ID:               em.generateID(),
		TaskID:           taskID,
		Deposits:         make(map[string]float64),
		TotalAmount:      0,
		RequiredDeposits: requiredDeposits,
		Status:           EscrowPending,
		LockSignatures:   make(map[string]string),
		UnlockSignatures: make(map[string]string),
		Participants:     participants,
		CreatedAt:        time.Now().Unix(),
		LockedUntil:      time.Now().Add(em.config.DefaultLockTime).Unix(),
	}

	em.escrows[escrow.ID] = escrow
	em.escrowsByTask[taskID] = escrow.ID
	em.escrowsByStatus[EscrowPending] = append(em.escrowsByStatus[EscrowPending], escrow.ID)

	for _, nodeID := range participants {
		em.escrowsByNode[nodeID] = append(em.escrowsByNode[nodeID], escrow.ID)
	}

	em.save()
	return escrow, nil
}

// Deposit 存入押金
func (em *EscrowManager) Deposit(escrowID, nodeID string, amount float64, signature string) error {
	em.mu.Lock()
	defer em.mu.Unlock()

	escrow, exists := em.escrows[escrowID]
	if !exists {
		return ErrEscrowNotFound
	}

	if escrow.Status != EscrowPending {
		return fmt.Errorf("cannot deposit: escrow status is %s", escrow.Status)
	}

	// 检查是否是参与方
	isParticipant := false
	for _, p := range escrow.Participants {
		if p == nodeID {
			isParticipant = true
			break
		}
	}
	if !isParticipant {
		return ErrUnauthorized
	}

	// 检查是否已存入
	if _, deposited := escrow.Deposits[nodeID]; deposited {
		return ErrAlreadyDeposited
	}

	// 检查金额
	required := escrow.RequiredDeposits[nodeID]
	if amount < required {
		return fmt.Errorf("%w: required %.2f, got %.2f", ErrInsufficientFunds, required, amount)
	}

	// 存入
	escrow.Deposits[nodeID] = amount
	escrow.TotalAmount += amount
	escrow.LockSignatures[nodeID] = signature

	// 检查是否所有人都已存入
	allDeposited := true
	for _, p := range escrow.Participants {
		if _, deposited := escrow.Deposits[p]; !deposited {
			allDeposited = false
			break
		}
	}

	if allDeposited {
		escrow.Status = EscrowLocked
		escrow.LockedAt = time.Now().Unix()
		em.updateStatusIndex(escrow.ID, EscrowPending, EscrowLocked)
	}

	em.save()
	return nil
}

// Release 释放押金给指定方
func (em *EscrowManager) Release(escrowID, releaseToNodeID string, amount float64, signatures map[string]string) error {
	em.mu.Lock()
	defer em.mu.Unlock()

	escrow, exists := em.escrows[escrowID]
	if !exists {
		return ErrEscrowNotFound
	}

	if escrow.Status != EscrowLocked {
		return ErrEscrowNotLocked
	}

	// 验证签名（需要多数参与方签名）
	signedCount := len(signatures)
	requiredSigns := (len(escrow.Participants) + 1) / 2 // 超过半数
	if signedCount < requiredSigns {
		return fmt.Errorf("insufficient signatures: need %d, got %d", requiredSigns, signedCount)
	}

	// 检查金额
	if amount > escrow.TotalAmount {
		return ErrInsufficientFunds
	}

	// 执行释放
	escrow.UnlockSignatures = signatures
	escrow.ReleasedTo = releaseToNodeID
	escrow.ReleasedAmount = amount
	escrow.ReleasedAt = time.Now().Unix()
	escrow.Status = EscrowReleased
	escrow.ReleaseCondition = "normal_completion"

	em.updateStatusIndex(escrow.ID, EscrowLocked, EscrowReleased)
	em.save()

	return nil
}

// Refund 退款给存入方
func (em *EscrowManager) Refund(escrowID string, signatures map[string]string) error {
	em.mu.Lock()
	defer em.mu.Unlock()

	escrow, exists := em.escrows[escrowID]
	if !exists {
		return ErrEscrowNotFound
	}

	if escrow.Status != EscrowLocked && escrow.Status != EscrowDisputed {
		return fmt.Errorf("cannot refund: escrow status is %s", escrow.Status)
	}

	// 验证签名
	signedCount := len(signatures)
	requiredSigns := (len(escrow.Participants) + 1) / 2
	if signedCount < requiredSigns {
		return fmt.Errorf("insufficient signatures: need %d, got %d", requiredSigns, signedCount)
	}

	escrow.UnlockSignatures = signatures
	escrow.Status = EscrowRefunded
	escrow.ReleasedAt = time.Now().Unix()
	escrow.ReleaseCondition = "refund"

	em.updateStatusIndex(escrow.ID, EscrowLocked, EscrowRefunded)
	em.save()

	return nil
}

// Dispute 发起争议
func (em *EscrowManager) Dispute(escrowID, disputerID, reason string) error {
	em.mu.Lock()
	defer em.mu.Unlock()

	escrow, exists := em.escrows[escrowID]
	if !exists {
		return ErrEscrowNotFound
	}

	if escrow.Status != EscrowLocked {
		return ErrEscrowNotLocked
	}

	// 检查是否是参与方
	isParticipant := false
	for _, p := range escrow.Participants {
		if p == disputerID {
			isParticipant = true
			break
		}
	}
	if !isParticipant {
		return ErrUnauthorized
	}

	oldStatus := escrow.Status
	escrow.Status = EscrowDisputed
	escrow.DisputeReason = reason
	escrow.DisputedBy = disputerID
	escrow.DisputedAt = time.Now().Unix()

	em.updateStatusIndex(escrow.ID, oldStatus, EscrowDisputed)
	em.save()

	return nil
}

// ResolveDispute Task44: 解决争议（需要多方仲裁签名）
// arbitratorSigs: map[arbitratorID]signature
func (em *EscrowManager) ResolveDispute(escrowID string, releaseToNodeID string, amount float64, arbitratorSigs map[string]string) error {
	em.mu.Lock()
	defer em.mu.Unlock()

	escrow, exists := em.escrows[escrowID]
	if !exists {
		return ErrEscrowNotFound
	}

	if escrow.Status != EscrowDisputed {
		return errors.New("escrow is not in disputed state")
	}

	// Task44: 验证仲裁签名数量是否满足阈值
	if len(arbitratorSigs) < em.config.MinArbitratorSigs {
		return fmt.Errorf("insufficient arbitrator signatures: need at least %d, got %d", 
			em.config.MinArbitratorSigs, len(arbitratorSigs))
	}

	escrow.ReleasedTo = releaseToNodeID
	escrow.ReleasedAmount = amount
	escrow.ReleasedAt = time.Now().Unix()
	escrow.Status = EscrowReleased
	escrow.ReleaseCondition = "dispute_resolution_multisig"

	// Task44: 存储所有仲裁签名（而不是单一签名）
	for arbitratorID, sig := range arbitratorSigs {
		escrow.UnlockSignatures["arbitrator_"+arbitratorID] = sig
	}

	em.updateStatusIndex(escrow.ID, EscrowDisputed, EscrowReleased)
	em.save()

	return nil
}

// SubmitArbitratorSignature Task44: 提交单个仲裁签名（用于逐步收集签名）
func (em *EscrowManager) SubmitArbitratorSignature(escrowID, arbitratorID, signature string) (bool, error) {
	em.mu.Lock()
	defer em.mu.Unlock()

	escrow, exists := em.escrows[escrowID]
	if !exists {
		return false, ErrEscrowNotFound
	}

	if escrow.Status != EscrowDisputed {
		return false, errors.New("escrow is not in disputed state")
	}

	if escrow.UnlockSignatures == nil {
		escrow.UnlockSignatures = make(map[string]string)
	}

	key := "arbitrator_" + arbitratorID
	escrow.UnlockSignatures[key] = signature

	// 计算当前仲裁签名数量
	arbCount := 0
	for k := range escrow.UnlockSignatures {
		if len(k) > 11 && k[:11] == "arbitrator_" {
			arbCount++
		}
	}

	em.save()

	// 返回是否已达到阈值
	return arbCount >= em.config.MinArbitratorSigs, nil
}

// GetArbitratorSignatureCount Task44: 获取当前仲裁签名数量
func (em *EscrowManager) GetArbitratorSignatureCount(escrowID string) (int, int, error) {
	em.mu.RLock()
	defer em.mu.RUnlock()

	escrow, exists := em.escrows[escrowID]
	if !exists {
		return 0, 0, ErrEscrowNotFound
	}

	arbCount := 0
	for k := range escrow.UnlockSignatures {
		if len(k) > 11 && k[:11] == "arbitrator_" {
			arbCount++
		}
	}

	return arbCount, em.config.MinArbitratorSigs, nil
}

// Forfeit 没收押金
func (em *EscrowManager) Forfeit(escrowID, violatorID string, reason string) error {
	em.mu.Lock()
	defer em.mu.Unlock()

	escrow, exists := em.escrows[escrowID]
	if !exists {
		return ErrEscrowNotFound
	}

	if escrow.Status != EscrowLocked && escrow.Status != EscrowDisputed {
		return fmt.Errorf("cannot forfeit: escrow status is %s", escrow.Status)
	}

	oldStatus := escrow.Status
	escrow.Status = EscrowForfeited
	escrow.ReleaseCondition = "forfeited: " + reason
	escrow.ReleasedAt = time.Now().Unix()

	// 没收违规方押金
	if deposit, ok := escrow.Deposits[violatorID]; ok {
		escrow.ReleasedAmount = deposit
	}

	em.updateStatusIndex(escrow.ID, oldStatus, EscrowForfeited)
	em.save()

	return nil
}

// GetEscrow 获取押金托管信息
func (em *EscrowManager) GetEscrow(escrowID string) (*Escrow, error) {
	em.mu.RLock()
	defer em.mu.RUnlock()

	escrow, exists := em.escrows[escrowID]
	if !exists {
		return nil, ErrEscrowNotFound
	}
	return escrow, nil
}

// GetEscrowByTask 根据任务ID获取押金托管
func (em *EscrowManager) GetEscrowByTask(taskID string) (*Escrow, error) {
	em.mu.RLock()
	defer em.mu.RUnlock()

	escrowID, exists := em.escrowsByTask[taskID]
	if !exists {
		return nil, ErrEscrowNotFound
	}

	escrow, exists := em.escrows[escrowID]
	if !exists {
		return nil, ErrEscrowNotFound
	}

	return escrow, nil
}

// GetEscrowsByNode 获取节点的所有押金托管
func (em *EscrowManager) GetEscrowsByNode(nodeID string) []*Escrow {
	em.mu.RLock()
	defer em.mu.RUnlock()

	ids := em.escrowsByNode[nodeID]
	escrows := make([]*Escrow, 0, len(ids))
	for _, id := range ids {
		if escrow, exists := em.escrows[id]; exists {
			escrows = append(escrows, escrow)
		}
	}
	return escrows
}

// GetEscrowsByStatus 获取指定状态的押金托管
func (em *EscrowManager) GetEscrowsByStatus(status EscrowStatus) []*Escrow {
	em.mu.RLock()
	defer em.mu.RUnlock()

	ids := em.escrowsByStatus[status]
	escrows := make([]*Escrow, 0, len(ids))
	for _, id := range ids {
		if escrow, exists := em.escrows[id]; exists {
			escrows = append(escrows, escrow)
		}
	}
	return escrows
}

// GetLockedAmount 获取节点锁定的总金额
func (em *EscrowManager) GetLockedAmount(nodeID string) float64 {
	em.mu.RLock()
	defer em.mu.RUnlock()

	var total float64
	for _, id := range em.escrowsByNode[nodeID] {
		if escrow, exists := em.escrows[id]; exists {
			if escrow.Status == EscrowLocked || escrow.Status == EscrowDisputed {
				total += escrow.Deposits[nodeID]
			}
		}
	}
	return total
}

// CheckExpiredLocks 检查过期锁定
func (em *EscrowManager) CheckExpiredLocks() []string {
	em.mu.Lock()
	defer em.mu.Unlock()

	var expired []string
	now := time.Now().Unix()

	for id, escrow := range em.escrows {
		if escrow.Status == EscrowLocked && escrow.LockedUntil > 0 && now > escrow.LockedUntil {
			expired = append(expired, id)
		}
	}

	return expired
}

// GetStatistics 获取统计信息
func (em *EscrowManager) GetStatistics() *EscrowStatistics {
	em.mu.RLock()
	defer em.mu.RUnlock()

	stats := &EscrowStatistics{
		TotalEscrows:  len(em.escrows),
		ByStatus:      make(map[EscrowStatus]int),
		TotalLocked:   0,
		TotalReleased: 0,
	}

	for _, escrow := range em.escrows {
		stats.ByStatus[escrow.Status]++
		if escrow.Status == EscrowLocked || escrow.Status == EscrowDisputed {
			stats.TotalLocked += escrow.TotalAmount
		}
		if escrow.Status == EscrowReleased || escrow.Status == EscrowRefunded {
			stats.TotalReleased += escrow.ReleasedAmount
		}
	}

	return stats
}

// EscrowStatistics 押金统计
type EscrowStatistics struct {
	TotalEscrows  int
	ByStatus      map[EscrowStatus]int
	TotalLocked   float64
	TotalReleased float64
}

// 内部方法

func (em *EscrowManager) generateID() string {
	bytes := make([]byte, 16)
	rand.Read(bytes)
	return "escrow_" + hex.EncodeToString(bytes)
}

func (em *EscrowManager) updateStatusIndex(escrowID string, oldStatus, newStatus EscrowStatus) {
	// 从旧状态列表中移除
	oldList := em.escrowsByStatus[oldStatus]
	for i, id := range oldList {
		if id == escrowID {
			em.escrowsByStatus[oldStatus] = append(oldList[:i], oldList[i+1:]...)
			break
		}
	}

	// 添加到新状态列表
	em.escrowsByStatus[newStatus] = append(em.escrowsByStatus[newStatus], escrowID)
}

func (em *EscrowManager) load() {
	filePath := filepath.Join(em.config.DataDir, "escrows.json")
	data, err := os.ReadFile(filePath)
	if err != nil {
		return
	}

	var stored struct {
		Escrows map[string]*Escrow `json:"escrows"`
	}

	if err := json.Unmarshal(data, &stored); err != nil {
		return
	}

	if stored.Escrows != nil {
		em.escrows = stored.Escrows
		// 重建索引
		for id, escrow := range em.escrows {
			em.escrowsByTask[escrow.TaskID] = id
			em.escrowsByStatus[escrow.Status] = append(em.escrowsByStatus[escrow.Status], id)
			for _, nodeID := range escrow.Participants {
				em.escrowsByNode[nodeID] = append(em.escrowsByNode[nodeID], id)
			}
		}
	}
}

func (em *EscrowManager) save() {
	if err := os.MkdirAll(em.config.DataDir, 0755); err != nil {
		return
	}

	stored := struct {
		Escrows map[string]*Escrow `json:"escrows"`
	}{
		Escrows: em.escrows,
	}

	data, err := json.MarshalIndent(stored, "", "  ")
	if err != nil {
		return
	}

	filePath := filepath.Join(em.config.DataDir, "escrows.json")
	os.WriteFile(filePath, data, 0644)
}
