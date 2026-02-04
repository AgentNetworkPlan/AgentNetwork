// Package address 实现门牌号系统
// 门牌号 = 公钥 + 注册证明 + 邻居见证
// 基于 Task 35 设计
package address

import (
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"sync"
	"time"
)

// 门牌号系统常量
const (
	// 注册要求
	MinWitnessCount       = 2               // 最少见证人数
	RegistrationValidDays = 365             // 注册有效期（天）
	WitnessValidDays      = 30              // 见证有效期（天）

	// 担保要求
	MinGuarantorReputation = 200.0 // 担保人最低声誉
	MaxGuaranteesPerNode   = 3     // 每节点最多担保人数

	// 门牌号状态
	AddressStatusPending   = "pending"   // 待确认
	AddressStatusActive    = "active"    // 活跃
	AddressStatusSuspended = "suspended" // 暂停
	AddressStatusRevoked   = "revoked"   // 撤销
)

var (
	ErrInvalidPublicKey      = errors.New("invalid public key")
	ErrNoGuarantor           = errors.New("no guarantor provided")
	ErrGuarantorLowRep       = errors.New("guarantor reputation too low")
	ErrGuarantorLimitReached = errors.New("guarantor has reached guarantee limit")
	ErrInsufficientWitness   = errors.New("insufficient witnesses")
	ErrAddressExists         = errors.New("address already exists")
	ErrAddressNotFound       = errors.New("address not found")
	ErrAddressRevoked        = errors.New("address has been revoked")
	ErrInvalidSignature      = errors.New("invalid signature")
	ErrExpiredWitness        = errors.New("witness has expired")
)

// Address 门牌号
type Address struct {
	// 核心身份
	ID          string `json:"id"`          // 门牌号ID（公钥哈希）
	PublicKey   string `json:"public_key"`  // 公钥（SM2）
	NodeID      string `json:"node_id"`     // 节点ID

	// 注册信息
	GuarantorID string    `json:"guarantor_id"` // 担保人ID
	RegisteredAt time.Time `json:"registered_at"` // 注册时间
	ExpiresAt   time.Time `json:"expires_at"`   // 过期时间

	// 见证记录
	Witnesses   []Witness `json:"witnesses"` // 邻居见证

	// 状态
	Status      string    `json:"status"`       // 状态
	Reputation  float64   `json:"reputation"`   // 当前声誉
	LastSeen    time.Time `json:"last_seen"`    // 最后活跃时间

	// 抵押物绑定（可选，来自 Task 36）
	CollateralIDs []string `json:"collateral_ids,omitempty"` // 绑定的抵押物ID
}

// Witness 见证记录
type Witness struct {
	WitnessID   string    `json:"witness_id"`   // 见证人节点ID
	Timestamp   time.Time `json:"timestamp"`    // 见证时间
	Signature   string    `json:"signature"`    // 见证签名
	ExpiresAt   time.Time `json:"expires_at"`   // 有效期
}

// RegistrationRequest 注册请求
type RegistrationRequest struct {
	PublicKey    string `json:"public_key"`    // 申请者公钥
	NodeID       string `json:"node_id"`       // 申请者节点ID
	GuarantorID  string `json:"guarantor_id"`  // 担保人ID
	GuarantorSig string `json:"guarantor_sig"` // 担保人签名
	Message      string `json:"message"`       // 可选：申请说明
}

// RegistrationResult 注册结果
type RegistrationResult struct {
	Success   bool    `json:"success"`
	AddressID string  `json:"address_id,omitempty"`
	Error     string  `json:"error,omitempty"`
}

// Registry 门牌号注册表
type Registry struct {
	mu        sync.RWMutex
	addresses map[string]*Address          // ID -> Address
	byPubKey  map[string]string            // 公钥 -> ID
	byNodeID  map[string]string            // 节点ID -> ID
	
	// 担保统计
	guaranteeCount map[string]int          // 担保人ID -> 担保数量

	// 依赖（通过接口解耦）
	reputationGetter ReputationGetter
	signatureVerifier SignatureVerifier
}

// ReputationGetter 声誉查询接口
type ReputationGetter interface {
	GetReputation(nodeID string) float64
}

// SignatureVerifier 签名验证接口
type SignatureVerifier interface {
	Verify(publicKey, message, signature string) bool
}

// NewRegistry 创建门牌号注册表
func NewRegistry(repGetter ReputationGetter, sigVerifier SignatureVerifier) *Registry {
	return &Registry{
		addresses:        make(map[string]*Address),
		byPubKey:         make(map[string]string),
		byNodeID:         make(map[string]string),
		guaranteeCount:   make(map[string]int),
		reputationGetter: repGetter,
		signatureVerifier: sigVerifier,
	}
}

// Register 注册门牌号
func (r *Registry) Register(req *RegistrationRequest) (*RegistrationResult, error) {
	r.mu.Lock()
	defer r.mu.Unlock()

	// 1. 验证公钥
	if req.PublicKey == "" {
		return nil, ErrInvalidPublicKey
	}

	// 2. 检查是否已注册
	if _, exists := r.byPubKey[req.PublicKey]; exists {
		return nil, ErrAddressExists
	}
	if _, exists := r.byNodeID[req.NodeID]; exists {
		return nil, ErrAddressExists
	}

	// 3. 验证担保人
	if req.GuarantorID == "" {
		return nil, ErrNoGuarantor
	}

	// 检查担保人声誉
	if r.reputationGetter != nil {
		guarantorRep := r.reputationGetter.GetReputation(req.GuarantorID)
		if guarantorRep < MinGuarantorReputation {
			return nil, ErrGuarantorLowRep
		}
	}

	// 检查担保人担保数量限制
	if r.guaranteeCount[req.GuarantorID] >= MaxGuaranteesPerNode {
		return nil, ErrGuarantorLimitReached
	}

	// 4. 验证担保人签名
	if r.signatureVerifier != nil {
		// 担保消息格式: "GUARANTEE:{nodeID}:{publicKey}:{timestamp}"
		message := "GUARANTEE:" + req.NodeID + ":" + req.PublicKey
		if !r.signatureVerifier.Verify(req.GuarantorID, message, req.GuarantorSig) {
			return nil, ErrInvalidSignature
		}
	}

	// 5. 生成门牌号ID
	addressID := generateAddressID(req.PublicKey)

	// 6. 创建门牌号
	now := time.Now()
	addr := &Address{
		ID:           addressID,
		PublicKey:    req.PublicKey,
		NodeID:       req.NodeID,
		GuarantorID:  req.GuarantorID,
		RegisteredAt: now,
		ExpiresAt:    now.AddDate(0, 0, RegistrationValidDays),
		Witnesses:    make([]Witness, 0),
		Status:       AddressStatusPending, // 待见证确认
		Reputation:   10.0,                 // 初始声誉
		LastSeen:     now,
	}

	// 7. 保存
	r.addresses[addressID] = addr
	r.byPubKey[req.PublicKey] = addressID
	r.byNodeID[req.NodeID] = addressID
	r.guaranteeCount[req.GuarantorID]++

	return &RegistrationResult{
		Success:   true,
		AddressID: addressID,
	}, nil
}

// AddWitness 添加见证
func (r *Registry) AddWitness(addressID, witnessID, signature string) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	addr, exists := r.addresses[addressID]
	if !exists {
		return ErrAddressNotFound
	}

	// 检查见证人是否已存在
	for _, w := range addr.Witnesses {
		if w.WitnessID == witnessID {
			return nil // 已见证，忽略
		}
	}

	// 验证签名
	if r.signatureVerifier != nil {
		message := "WITNESS:" + addressID + ":" + addr.NodeID
		if !r.signatureVerifier.Verify(witnessID, message, signature) {
			return ErrInvalidSignature
		}
	}

	// 添加见证
	now := time.Now()
	witness := Witness{
		WitnessID: witnessID,
		Timestamp: now,
		Signature: signature,
		ExpiresAt: now.AddDate(0, 0, WitnessValidDays),
	}
	addr.Witnesses = append(addr.Witnesses, witness)

	// 检查是否达到激活条件
	if len(addr.Witnesses) >= MinWitnessCount && addr.Status == AddressStatusPending {
		addr.Status = AddressStatusActive
	}

	return nil
}

// GetAddress 获取门牌号
func (r *Registry) GetAddress(addressID string) (*Address, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	addr, exists := r.addresses[addressID]
	if !exists {
		return nil, ErrAddressNotFound
	}

	return addr, nil
}

// GetAddressByNodeID 通过节点ID获取门牌号
func (r *Registry) GetAddressByNodeID(nodeID string) (*Address, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	addressID, exists := r.byNodeID[nodeID]
	if !exists {
		return nil, ErrAddressNotFound
	}

	return r.addresses[addressID], nil
}

// GetAddressByPublicKey 通过公钥获取门牌号
func (r *Registry) GetAddressByPublicKey(publicKey string) (*Address, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	addressID, exists := r.byPubKey[publicKey]
	if !exists {
		return nil, ErrAddressNotFound
	}

	return r.addresses[addressID], nil
}

// ValidateAddress 验证门牌号是否有效
func (r *Registry) ValidateAddress(addressID string) error {
	r.mu.RLock()
	defer r.mu.RUnlock()

	addr, exists := r.addresses[addressID]
	if !exists {
		return ErrAddressNotFound
	}

	if addr.Status == AddressStatusRevoked {
		return ErrAddressRevoked
	}

	// 检查是否过期
	if time.Now().After(addr.ExpiresAt) {
		return errors.New("address expired")
	}

	// 检查见证是否充足
	validWitnesses := 0
	now := time.Now()
	for _, w := range addr.Witnesses {
		if now.Before(w.ExpiresAt) {
			validWitnesses++
		}
	}
	if validWitnesses < MinWitnessCount {
		return ErrInsufficientWitness
	}

	return nil
}

// RevokeAddress 撤销门牌号
func (r *Registry) RevokeAddress(addressID, reason string) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	addr, exists := r.addresses[addressID]
	if !exists {
		return ErrAddressNotFound
	}

	addr.Status = AddressStatusRevoked
	
	// 释放担保人的担保配额
	if addr.GuarantorID != "" {
		r.guaranteeCount[addr.GuarantorID]--
	}

	return nil
}

// UpdateLastSeen 更新最后活跃时间
func (r *Registry) UpdateLastSeen(addressID string) {
	r.mu.Lock()
	defer r.mu.Unlock()

	if addr, exists := r.addresses[addressID]; exists {
		addr.LastSeen = time.Now()
	}
}

// UpdateReputation 更新声誉
func (r *Registry) UpdateReputation(addressID string, reputation float64) {
	r.mu.Lock()
	defer r.mu.Unlock()

	if addr, exists := r.addresses[addressID]; exists {
		addr.Reputation = reputation
	}
}

// GetAllAddresses 获取所有门牌号（用于调试）
func (r *Registry) GetAllAddresses() []*Address {
	r.mu.RLock()
	defer r.mu.RUnlock()

	result := make([]*Address, 0, len(r.addresses))
	for _, addr := range r.addresses {
		result = append(result, addr)
	}
	return result
}

// GetActiveAddresses 获取所有活跃门牌号
func (r *Registry) GetActiveAddresses() []*Address {
	r.mu.RLock()
	defer r.mu.RUnlock()

	result := make([]*Address, 0)
	for _, addr := range r.addresses {
		if addr.Status == AddressStatusActive {
			result = append(result, addr)
		}
	}
	return result
}

// generateAddressID 生成门牌号ID（公钥哈希）
func generateAddressID(publicKey string) string {
	hash := sha256.Sum256([]byte(publicKey))
	return hex.EncodeToString(hash[:16]) // 取前16字节
}

// AddressInfo 门牌号信息（Agent可读）
type AddressInfo struct {
	ID            string    `json:"id"`
	Status        string    `json:"status"`
	StatusName    string    `json:"status_name"`
	Reputation    float64   `json:"reputation"`
	RegisteredAt  time.Time `json:"registered_at"`
	WitnessCount  int       `json:"witness_count"`
	IsValid       bool      `json:"is_valid"`
	GuarantorID   string    `json:"guarantor_id"`
	CollateralIDs []string  `json:"collateral_ids,omitempty"`
}

// GetAddressInfo 获取门牌号详情（Agent可读）
func (r *Registry) GetAddressInfo(addressID string) (*AddressInfo, error) {
	addr, err := r.GetAddress(addressID)
	if err != nil {
		return nil, err
	}

	statusNames := map[string]string{
		AddressStatusPending:   "待确认",
		AddressStatusActive:    "活跃",
		AddressStatusSuspended: "暂停",
		AddressStatusRevoked:   "已撤销",
	}

	isValid := r.ValidateAddress(addressID) == nil

	return &AddressInfo{
		ID:            addr.ID,
		Status:        addr.Status,
		StatusName:    statusNames[addr.Status],
		Reputation:    addr.Reputation,
		RegisteredAt:  addr.RegisteredAt,
		WitnessCount:  len(addr.Witnesses),
		IsValid:       isValid,
		GuarantorID:   addr.GuarantorID,
		CollateralIDs: addr.CollateralIDs,
	}, nil
}
