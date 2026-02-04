// Package collateral 实现抵押物验证机制
// 抵押物是声誉的经济绑定 - 提供可验证的承诺和惩罚机制
package collateral

import (
	"errors"
	"fmt"
	"sync"
	"time"
)

// 抵押物相关常量
const (
	// 抵押物类型
	CollateralTypeToken     = "token"     // 代币抵押
	CollateralTypeStake     = "stake"     // 质押权益
	CollateralTypeReputation = "reputation" // 声誉质押
	CollateralTypeService   = "service"   // 服务承诺

	// 抵押物状态
	CollateralStatusPending  = "pending"   // 待确认
	CollateralStatusActive   = "active"    // 生效中
	CollateralStatusLocked   = "locked"    // 锁定中（争议处理）
	CollateralStatusSlashed  = "slashed"   // 已惩罚
	CollateralStatusReturned = "returned"  // 已归还
	CollateralStatusExpired  = "expired"   // 已过期

	// 默认参数
	MinCollateralAmount    = 1.0            // 最小抵押金额
	MaxCollateralRatio     = 10.0           // 最大抵押倍数（相对于声誉）
	DefaultLockPeriod      = 24 * time.Hour // 默认锁定期
	SlashRatio             = 0.5            // 惩罚比例（没收50%）
	MinGuarantorCollateral = 100.0          // 担保人最低抵押要求
)

// 错误定义
var (
	ErrCollateralNotFound    = errors.New("collateral not found")
	ErrInsufficientAmount    = errors.New("insufficient collateral amount")
	ErrCollateralLocked      = errors.New("collateral is locked")
	ErrCollateralExpired     = errors.New("collateral has expired")
	ErrInvalidCollateralType = errors.New("invalid collateral type")
	ErrAlreadySlashed        = errors.New("collateral already slashed")
	ErrUnauthorized          = errors.New("unauthorized operation")
	ErrSelfCollateral        = errors.New("cannot use self as guarantor")
)

// Collateral 抵押物
type Collateral struct {
	ID           string            `json:"id"`             // 抵押物ID
	Owner        string            `json:"owner"`          // 所有者
	Type         string            `json:"type"`           // 抵押物类型
	Amount       float64           `json:"amount"`         // 抵押金额
	Status       string            `json:"status"`         // 当前状态
	Purpose      string            `json:"purpose"`        // 抵押目的
	Beneficiary  string            `json:"beneficiary"`    // 受益人（争议时）
	CreatedAt    time.Time         `json:"created_at"`     // 创建时间
	ExpiresAt    time.Time         `json:"expires_at"`     // 过期时间
	LockedAt     *time.Time        `json:"locked_at"`      // 锁定时间
	SlashedAt    *time.Time        `json:"slashed_at"`     // 惩罚时间
	ReturnedAt   *time.Time        `json:"returned_at"`    // 归还时间
	SlashAmount  float64           `json:"slash_amount"`   // 惩罚金额
	Metadata     map[string]string `json:"metadata"`       // 元数据
}

// CollateralRequirement 抵押要求
type CollateralRequirement struct {
	MinAmount    float64 `json:"min_amount"`     // 最低金额
	MaxAmount    float64 `json:"max_amount"`     // 最高金额
	AcceptedTypes []string `json:"accepted_types"` // 接受的类型
	LockPeriod   time.Duration `json:"lock_period"` // 锁定期
	Purpose      string  `json:"purpose"`        // 用途
}

// CollateralProof 抵押证明
type CollateralProof struct {
	CollateralID string    `json:"collateral_id"` // 抵押物ID
	Owner        string    `json:"owner"`         // 所有者
	Amount       float64   `json:"amount"`        // 金额
	Type         string    `json:"type"`          // 类型
	IsValid      bool      `json:"is_valid"`      // 是否有效
	VerifiedAt   time.Time `json:"verified_at"`   // 验证时间
	Signature    string    `json:"signature"`     // 验证签名
}

// SlashEvent 惩罚事件
type SlashEvent struct {
	CollateralID string    `json:"collateral_id"` // 抵押物ID
	Owner        string    `json:"owner"`         // 被惩罚者
	Amount       float64   `json:"amount"`        // 惩罚金额
	Reason       string    `json:"reason"`        // 惩罚原因
	Evidence     []string  `json:"evidence"`      // 证据
	Beneficiary  string    `json:"beneficiary"`   // 受益人
	Timestamp    time.Time `json:"timestamp"`     // 时间
}

// CollateralManager 抵押物管理器
type CollateralManager struct {
	collaterals  map[string]*Collateral     // collateralID -> Collateral
	byOwner      map[string][]string        // owner -> []collateralID
	slashHistory map[string][]*SlashEvent   // owner -> []SlashEvent
	totalSlashed map[string]float64         // owner -> total slashed amount
	idCounter    int64                      // ID计数器
	mu           sync.RWMutex
}

// NewCollateralManager 创建新的抵押物管理器
func NewCollateralManager() *CollateralManager {
	return &CollateralManager{
		collaterals:  make(map[string]*Collateral),
		byOwner:      make(map[string][]string),
		slashHistory: make(map[string][]*SlashEvent),
		totalSlashed: make(map[string]float64),
		idCounter:    0,
	}
}

// CreateCollateral 创建抵押物
func (cm *CollateralManager) CreateCollateral(owner, collateralType, purpose string, amount float64, duration time.Duration) (*Collateral, error) {
	if amount < MinCollateralAmount {
		return nil, ErrInsufficientAmount
	}

	if !isValidCollateralType(collateralType) {
		return nil, ErrInvalidCollateralType
	}

	cm.mu.Lock()
	defer cm.mu.Unlock()

	now := time.Now()
	cm.idCounter++
	collateral := &Collateral{
		ID:          generateCollateralID(owner, now, cm.idCounter),
		Owner:       owner,
		Type:        collateralType,
		Amount:      amount,
		Status:      CollateralStatusPending,
		Purpose:     purpose,
		CreatedAt:   now,
		ExpiresAt:   now.Add(duration),
		Metadata:    make(map[string]string),
	}

	cm.collaterals[collateral.ID] = collateral
	cm.byOwner[owner] = append(cm.byOwner[owner], collateral.ID)

	return collateral, nil
}

// ActivateCollateral 激活抵押物（确认抵押到位）
func (cm *CollateralManager) ActivateCollateral(collateralID string) error {
	cm.mu.Lock()
	defer cm.mu.Unlock()

	collateral, ok := cm.collaterals[collateralID]
	if !ok {
		return ErrCollateralNotFound
	}

	if collateral.Status != CollateralStatusPending {
		return fmt.Errorf("cannot activate collateral in status %s", collateral.Status)
	}

	collateral.Status = CollateralStatusActive
	return nil
}

// LockCollateral 锁定抵押物（争议处理期间）
func (cm *CollateralManager) LockCollateral(collateralID, beneficiary, reason string) error {
	cm.mu.Lock()
	defer cm.mu.Unlock()

	collateral, ok := cm.collaterals[collateralID]
	if !ok {
		return ErrCollateralNotFound
	}

	if collateral.Status != CollateralStatusActive {
		return fmt.Errorf("cannot lock collateral in status %s", collateral.Status)
	}

	now := time.Now()
	collateral.Status = CollateralStatusLocked
	collateral.LockedAt = &now
	collateral.Beneficiary = beneficiary
	collateral.Metadata["lock_reason"] = reason

	return nil
}

// SlashCollateral 惩罚抵押物
func (cm *CollateralManager) SlashCollateral(collateralID, reason string, evidence []string, slashRatio float64) (*SlashEvent, error) {
	cm.mu.Lock()
	defer cm.mu.Unlock()

	collateral, ok := cm.collaterals[collateralID]
	if !ok {
		return nil, ErrCollateralNotFound
	}

	if collateral.Status == CollateralStatusSlashed {
		return nil, ErrAlreadySlashed
	}

	if collateral.Status == CollateralStatusReturned || collateral.Status == CollateralStatusExpired {
		return nil, ErrCollateralExpired
	}

	// 限制惩罚比例
	if slashRatio <= 0 {
		slashRatio = SlashRatio
	}
	if slashRatio > 1.0 {
		slashRatio = 1.0
	}

	slashAmount := collateral.Amount * slashRatio
	now := time.Now()

	collateral.Status = CollateralStatusSlashed
	collateral.SlashedAt = &now
	collateral.SlashAmount = slashAmount

	event := &SlashEvent{
		CollateralID: collateralID,
		Owner:        collateral.Owner,
		Amount:       slashAmount,
		Reason:       reason,
		Evidence:     evidence,
		Beneficiary:  collateral.Beneficiary,
		Timestamp:    now,
	}

	cm.slashHistory[collateral.Owner] = append(cm.slashHistory[collateral.Owner], event)
	cm.totalSlashed[collateral.Owner] += slashAmount

	return event, nil
}

// ReturnCollateral 归还抵押物
func (cm *CollateralManager) ReturnCollateral(collateralID string) error {
	cm.mu.Lock()
	defer cm.mu.Unlock()

	collateral, ok := cm.collaterals[collateralID]
	if !ok {
		return ErrCollateralNotFound
	}

	if collateral.Status == CollateralStatusLocked {
		return ErrCollateralLocked
	}

	if collateral.Status == CollateralStatusSlashed {
		return ErrAlreadySlashed
	}

	now := time.Now()
	collateral.Status = CollateralStatusReturned
	collateral.ReturnedAt = &now

	return nil
}

// GetCollateral 获取抵押物
func (cm *CollateralManager) GetCollateral(collateralID string) (*Collateral, error) {
	cm.mu.RLock()
	defer cm.mu.RUnlock()

	collateral, ok := cm.collaterals[collateralID]
	if !ok {
		return nil, ErrCollateralNotFound
	}

	// 返回副本
	copy := *collateral
	return &copy, nil
}

// GetOwnerCollaterals 获取所有者的所有抵押物
func (cm *CollateralManager) GetOwnerCollaterals(owner string) []*Collateral {
	cm.mu.RLock()
	defer cm.mu.RUnlock()

	var result []*Collateral
	for _, id := range cm.byOwner[owner] {
		if c, ok := cm.collaterals[id]; ok {
			copy := *c
			result = append(result, &copy)
		}
	}
	return result
}

// GetActiveCollateral 获取所有者的有效抵押金额
func (cm *CollateralManager) GetActiveCollateral(owner string) float64 {
	cm.mu.RLock()
	defer cm.mu.RUnlock()

	var total float64
	now := time.Now()
	for _, id := range cm.byOwner[owner] {
		if c, ok := cm.collaterals[id]; ok {
			if c.Status == CollateralStatusActive && c.ExpiresAt.After(now) {
				total += c.Amount
			}
		}
	}
	return total
}

// VerifyCollateral 验证抵押物
func (cm *CollateralManager) VerifyCollateral(collateralID string) (*CollateralProof, error) {
	cm.mu.RLock()
	defer cm.mu.RUnlock()

	collateral, ok := cm.collaterals[collateralID]
	if !ok {
		return nil, ErrCollateralNotFound
	}

	now := time.Now()
	isValid := collateral.Status == CollateralStatusActive && collateral.ExpiresAt.After(now)

	return &CollateralProof{
		CollateralID: collateralID,
		Owner:        collateral.Owner,
		Amount:       collateral.Amount,
		Type:         collateral.Type,
		IsValid:      isValid,
		VerifiedAt:   now,
		Signature:    fmt.Sprintf("verified:%s:%d", collateralID, now.Unix()),
	}, nil
}

// CheckRequirement 检查是否满足抵押要求
func (cm *CollateralManager) CheckRequirement(owner string, req *CollateralRequirement) (bool, string) {
	cm.mu.RLock()
	defer cm.mu.RUnlock()

	totalActive := float64(0)
	validTypes := make(map[string]bool)
	for _, t := range req.AcceptedTypes {
		validTypes[t] = true
	}

	now := time.Now()
	for _, id := range cm.byOwner[owner] {
		c, ok := cm.collaterals[id]
		if !ok {
			continue
		}
		if c.Status != CollateralStatusActive {
			continue
		}
		if c.ExpiresAt.Before(now) {
			continue
		}
		if len(validTypes) > 0 && !validTypes[c.Type] {
			continue
		}
		// 检查剩余锁定期
		if c.ExpiresAt.Sub(now) < req.LockPeriod {
			continue
		}
		totalActive += c.Amount
	}

	if totalActive < req.MinAmount {
		return false, fmt.Sprintf("insufficient collateral: have %.2f, need %.2f", totalActive, req.MinAmount)
	}

	return true, "requirement met"
}

// GetSlashHistory 获取惩罚历史
func (cm *CollateralManager) GetSlashHistory(owner string) []*SlashEvent {
	cm.mu.RLock()
	defer cm.mu.RUnlock()

	events := cm.slashHistory[owner]
	result := make([]*SlashEvent, len(events))
	for i, e := range events {
		copy := *e
		result[i] = &copy
	}
	return result
}

// GetTotalSlashed 获取总惩罚金额
func (cm *CollateralManager) GetTotalSlashed(owner string) float64 {
	cm.mu.RLock()
	defer cm.mu.RUnlock()
	return cm.totalSlashed[owner]
}

// ExpireCollaterals 处理过期抵押物
func (cm *CollateralManager) ExpireCollaterals() int {
	cm.mu.Lock()
	defer cm.mu.Unlock()

	count := 0
	now := time.Now()
	for _, c := range cm.collaterals {
		if c.Status == CollateralStatusActive && c.ExpiresAt.Before(now) {
			c.Status = CollateralStatusExpired
			count++
		}
	}
	return count
}

// GetStats 获取统计信息
func (cm *CollateralManager) GetStats() map[string]interface{} {
	cm.mu.RLock()
	defer cm.mu.RUnlock()

	totalCollaterals := len(cm.collaterals)
	activeCount := 0
	lockedCount := 0
	slashedCount := 0
	totalAmount := 0.0
	totalSlashed := 0.0

	for _, c := range cm.collaterals {
		totalAmount += c.Amount
		switch c.Status {
		case CollateralStatusActive:
			activeCount++
		case CollateralStatusLocked:
			lockedCount++
		case CollateralStatusSlashed:
			slashedCount++
			totalSlashed += c.SlashAmount
		}
	}

	return map[string]interface{}{
		"total_collaterals": totalCollaterals,
		"active_count":      activeCount,
		"locked_count":      lockedCount,
		"slashed_count":     slashedCount,
		"total_amount":      totalAmount,
		"total_slashed":     totalSlashed,
		"unique_owners":     len(cm.byOwner),
	}
}

// 辅助函数

func isValidCollateralType(t string) bool {
	switch t {
	case CollateralTypeToken, CollateralTypeStake, CollateralTypeReputation, CollateralTypeService:
		return true
	default:
		return false
	}
}

func generateCollateralID(owner string, t time.Time, counter int64) string {
	return fmt.Sprintf("col_%s_%d_%d", owner[:min(8, len(owner))], t.UnixNano(), counter)
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}

// GuaranteePool 担保池
// 用于管理担保人与被担保人之间的关系
type GuaranteePool struct {
	// 担保关系：guarantor -> [guaranteed nodes]
	guarantees map[string][]string
	// 担保抵押：guarantor -> node -> collateralID
	collateralMap map[string]map[string]string
	cm            *CollateralManager
	mu            sync.RWMutex
}

// NewGuaranteePool 创建担保池
func NewGuaranteePool(cm *CollateralManager) *GuaranteePool {
	return &GuaranteePool{
		guarantees:    make(map[string][]string),
		collateralMap: make(map[string]map[string]string),
		cm:            cm,
	}
}

// AddGuarantee 添加担保关系
func (gp *GuaranteePool) AddGuarantee(guarantor, guaranteed, collateralID string) error {
	if guarantor == guaranteed {
		return ErrSelfCollateral
	}

	// 验证抵押物
	proof, err := gp.cm.VerifyCollateral(collateralID)
	if err != nil {
		return err
	}
	if !proof.IsValid {
		return ErrCollateralExpired
	}
	if proof.Owner != guarantor {
		return ErrUnauthorized
	}
	if proof.Amount < MinGuarantorCollateral {
		return ErrInsufficientAmount
	}

	gp.mu.Lock()
	defer gp.mu.Unlock()

	gp.guarantees[guarantor] = append(gp.guarantees[guarantor], guaranteed)
	if gp.collateralMap[guarantor] == nil {
		gp.collateralMap[guarantor] = make(map[string]string)
	}
	gp.collateralMap[guarantor][guaranteed] = collateralID

	return nil
}

// GetGuarantors 获取节点的担保人列表
func (gp *GuaranteePool) GetGuarantors(node string) []string {
	gp.mu.RLock()
	defer gp.mu.RUnlock()

	var guarantors []string
	for guarantor, guaranteed := range gp.guarantees {
		for _, g := range guaranteed {
			if g == node {
				guarantors = append(guarantors, guarantor)
				break
			}
		}
	}
	return guarantors
}

// GetGuaranteed 获取担保人担保的节点列表
func (gp *GuaranteePool) GetGuaranteed(guarantor string) []string {
	gp.mu.RLock()
	defer gp.mu.RUnlock()

	result := make([]string, len(gp.guarantees[guarantor]))
	copy(result, gp.guarantees[guarantor])
	return result
}

// SlashGuarantor 惩罚担保人（当被担保人违规时）
func (gp *GuaranteePool) SlashGuarantor(guarantor, guaranteed, reason string, evidence []string) (*SlashEvent, error) {
	gp.mu.RLock()
	collateralID, ok := gp.collateralMap[guarantor][guaranteed]
	gp.mu.RUnlock()

	if !ok {
		return nil, fmt.Errorf("no guarantee relationship found")
	}

	// 担保人也要承担部分损失
	return gp.cm.SlashCollateral(collateralID, reason, evidence, SlashRatio*0.5)
}
