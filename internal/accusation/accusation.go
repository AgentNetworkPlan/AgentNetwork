// Package accusation 实现去中心化声誉指责与惩罚机制
// 包括指责发起、传播、验证、自然衰减等功能
package accusation

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"sync"
	"time"
)

// 错误定义
var (
	ErrNilConfig           = errors.New("config cannot be nil")
	ErrEmptyNodeID         = errors.New("node ID cannot be empty")
	ErrEmptyAccuser        = errors.New("accuser cannot be empty")
	ErrEmptyAccused        = errors.New("accused cannot be empty")
	ErrSelfAccusation      = errors.New("cannot accuse self")
	ErrAccusationNotFound  = errors.New("accusation not found")
	ErrDuplicateAccusation = errors.New("duplicate accusation")
	ErrInvalidSignature    = errors.New("invalid signature")
	ErrToleranceExceeded   = errors.New("tolerance exceeded for this accuser")
	ErrLowReputation       = errors.New("accuser reputation too low")
	ErrAccusationExpired   = errors.New("accusation has expired")
)

// AccusationStatus 指责状态
type AccusationStatus string

const (
	StatusPending   AccusationStatus = "pending"   // 待处理
	StatusDelivered AccusationStatus = "delivered" // 已传递
	StatusVerified  AccusationStatus = "verified"  // 已验证
	StatusRejected  AccusationStatus = "rejected"  // 被拒绝
	StatusArchived  AccusationStatus = "archived"  // 已归档
)

// AccusationType 指责类型
type AccusationType string

const (
	TypeTaskCheating     AccusationType = "task_cheating"     // 任务作弊
	TypeMessageSpam      AccusationType = "message_spam"      // 消息垃圾
	TypeServiceDenial    AccusationType = "service_denial"    // 服务拒绝
	TypeDataCorruption   AccusationType = "data_corruption"   // 数据损坏
	TypeProtocolViolation AccusationType = "protocol_violation" // 协议违规
	TypeOther            AccusationType = "other"             // 其他
)

// Accusation 指责记录
type Accusation struct {
	AccusationID    string           `json:"accusation_id"`    // 指责唯一ID
	Accuser         string           `json:"accuser"`          // 指责者
	Accused         string           `json:"accused"`          // 被指责者
	Type            AccusationType   `json:"type"`             // 指责类型
	Reason          string           `json:"reason"`           // 原因说明
	Evidence        string           `json:"evidence"`         // 证据（可选）
	Timestamp       time.Time        `json:"timestamp"`        // 时间戳
	ExpiresAt       time.Time        `json:"expires_at"`       // 过期时间
	Signature       string           `json:"signature"`        // SM2签名
	Status          AccusationStatus `json:"status"`           // 状态
	AccuserReputation float64        `json:"accuser_reputation"` // 指责者声誉
	BasePenalty     float64          `json:"base_penalty"`     // 基础惩罚
	AccuserCost     float64          `json:"accuser_cost"`     // 指责者代价
	PropagationDepth int             `json:"propagation_depth"` // 当前传播深度
	PropagatedTo    []string         `json:"propagated_to"`    // 已传播到的节点
}

// AccusationAnalysis 指责分析结果
type AccusationAnalysis struct {
	AccusationID      string    `json:"accusation_id"`
	AnalyzerNodeID    string    `json:"analyzer_node_id"`
	Timestamp         time.Time `json:"timestamp"`
	PenaltyToAccused  float64   `json:"penalty_to_accused"`  // 对被指责者的惩罚
	CostToAccuser     float64   `json:"cost_to_accuser"`     // 指责者的代价
	Accepted          bool      `json:"accepted"`            // 是否接受
	Reason            string    `json:"reason"`              // 分析原因
	Signature         string    `json:"signature"`           // 分析签名
}

// AccusationConfig 指责系统配置
type AccusationConfig struct {
	NodeID              string        // 本节点ID
	DataDir             string        // 数据目录
	DefaultExpiry       time.Duration // 默认过期时间
	DecayFactor         float64       // 衰减因子
	DefaultTolerance    float64       // 默认耐受值
	ToleranceResetPeriod time.Duration // 耐受值重置周期
	BasePenalty         float64       // 基础惩罚值
	BaseAccuserCost     float64       // 基础指责代价
	MinAccuserReputation float64      // 最低指责者声誉
	MaxPropagationDepth int           // 最大传播深度
	NaturalDecayAmount  float64       // 自然衰减量（每日）
	NaturalDecayInterval time.Duration // 自然衰减间隔
	CleanupInterval     time.Duration // 清理间隔
	
	// 签名函数
	SignFunc   func(data []byte) (string, error)
	VerifyFunc func(publicKey string, data []byte, signature string) bool
	
	// 获取邻居函数
	GetNeighborsFunc func(nodeID string) []string
	
	// 获取/更新声誉函数
	GetReputationFunc    func(nodeID string) float64
	UpdateReputationFunc func(nodeID string, delta float64) error
}

// DefaultAccusationConfig 返回默认配置
func DefaultAccusationConfig(nodeID string) *AccusationConfig {
	return &AccusationConfig{
		NodeID:              nodeID,
		DataDir:             "./data/accusation",
		DefaultExpiry:       7 * 24 * time.Hour, // 7天
		DecayFactor:         0.7,
		DefaultTolerance:    50.0,
		ToleranceResetPeriod: 24 * time.Hour,
		BasePenalty:         10.0,
		BaseAccuserCost:     2.0,
		MinAccuserReputation: 20.0,
		MaxPropagationDepth: 5,
		NaturalDecayAmount:  1.0,
		NaturalDecayInterval: 24 * time.Hour,
		CleanupInterval:     time.Hour,
	}
}

// ToleranceRecord 耐受值记录
type ToleranceRecord struct {
	AccuserNodeID      string    `json:"accuser_node_id"`
	TotalPenaltyReceived float64  `json:"total_penalty_received"`
	MaxTolerance       float64   `json:"max_tolerance"`
	RemainingTolerance float64   `json:"remaining_tolerance"`
	LastResetTime      time.Time `json:"last_reset_time"`
	NextResetTime      time.Time `json:"next_reset_time"`
}

// AccusationManager 指责管理器
type AccusationManager struct {
	mu           sync.RWMutex
	config       *AccusationConfig
	accusations  map[string]*Accusation                  // AccusationID -> Accusation
	analyses     map[string][]*AccusationAnalysis        // AccusationID -> []Analysis
	tolerances   map[string]*ToleranceRecord             // AccuserNodeID -> Tolerance
	lastDecayTime time.Time                              // 上次自然衰减时间
	running      bool
	stopCh       chan struct{}
	
	// 回调
	OnAccusationCreated   func(*Accusation)
	OnAccusationReceived  func(*Accusation, string)
	OnAccusationVerified  func(*Accusation, *AccusationAnalysis)
	OnAccusationRejected  func(*Accusation, string)
	OnToleranceExceeded   func(accuserID string, penalty float64)
	OnNaturalDecay        func(nodeID string, amount float64)
}

// NewAccusationManager 创建指责管理器
func NewAccusationManager(config *AccusationConfig) (*AccusationManager, error) {
	if config == nil {
		return nil, ErrNilConfig
	}
	if config.NodeID == "" {
		return nil, ErrEmptyNodeID
	}
	
	// 创建数据目录
	if config.DataDir != "" {
		if err := os.MkdirAll(config.DataDir, 0755); err != nil {
			return nil, fmt.Errorf("failed to create data directory: %w", err)
		}
	}
	
	am := &AccusationManager{
		config:        config,
		accusations:   make(map[string]*Accusation),
		analyses:      make(map[string][]*AccusationAnalysis),
		tolerances:    make(map[string]*ToleranceRecord),
		lastDecayTime: time.Now(),
		stopCh:        make(chan struct{}),
	}
	
	// 加载持久化数据
	if err := am.load(); err != nil {
		// 忽略加载错误
	}
	
	return am, nil
}

// Start 启动指责管理器
func (am *AccusationManager) Start() {
	am.mu.Lock()
	if am.running {
		am.mu.Unlock()
		return
	}
	am.running = true
	am.stopCh = make(chan struct{})
	am.mu.Unlock()
	
	go am.mainLoop()
}

// Stop 停止指责管理器
func (am *AccusationManager) Stop() {
	am.mu.Lock()
	if !am.running {
		am.mu.Unlock()
		return
	}
	am.running = false
	close(am.stopCh)
	am.mu.Unlock()
	
	am.save()
}

// mainLoop 主循环
func (am *AccusationManager) mainLoop() {
	decayTicker := time.NewTicker(am.config.NaturalDecayInterval)
	cleanupTicker := time.NewTicker(am.config.CleanupInterval)
	toleranceTicker := time.NewTicker(time.Hour)
	
	defer decayTicker.Stop()
	defer cleanupTicker.Stop()
	defer toleranceTicker.Stop()
	
	for {
		select {
		case <-decayTicker.C:
			am.applyNaturalDecay()
		case <-cleanupTicker.C:
			am.cleanup()
		case <-toleranceTicker.C:
			am.checkAndResetTolerances()
		case <-am.stopCh:
			return
		}
	}
}

// applyNaturalDecay 应用自然衰减
func (am *AccusationManager) applyNaturalDecay() {
	if am.config.UpdateReputationFunc == nil {
		return
	}
	
	// 对本节点应用自然衰减
	err := am.config.UpdateReputationFunc(am.config.NodeID, -am.config.NaturalDecayAmount)
	if err == nil && am.OnNaturalDecay != nil {
		am.OnNaturalDecay(am.config.NodeID, am.config.NaturalDecayAmount)
	}
	
	am.mu.Lock()
	am.lastDecayTime = time.Now()
	am.mu.Unlock()
}

// cleanup 清理过期指责
func (am *AccusationManager) cleanup() {
	am.mu.Lock()
	defer am.mu.Unlock()
	
	now := time.Now()
	for id, acc := range am.accusations {
		if now.After(acc.ExpiresAt) {
			acc.Status = StatusArchived
			delete(am.accusations, id)
			delete(am.analyses, id)
		}
	}
}

// checkAndResetTolerances 检查并重置耐受值
func (am *AccusationManager) checkAndResetTolerances() {
	am.mu.Lock()
	defer am.mu.Unlock()
	
	now := time.Now()
	for accuserID, record := range am.tolerances {
		if now.After(record.NextResetTime) {
			record.TotalPenaltyReceived = 0
			record.RemainingTolerance = record.MaxTolerance
			record.LastResetTime = now
			record.NextResetTime = now.Add(am.config.ToleranceResetPeriod)
			
			_ = accuserID // 避免未使用警告
		}
	}
}

// CreateAccusation 创建指责
func (am *AccusationManager) CreateAccusation(accused string, accusationType AccusationType, reason, evidence string) (*Accusation, error) {
	if accused == "" {
		return nil, ErrEmptyAccused
	}
	if accused == am.config.NodeID {
		return nil, ErrSelfAccusation
	}
	
	// 检查指责者声誉
	var accuserRep float64 = 50.0
	if am.config.GetReputationFunc != nil {
		accuserRep = am.config.GetReputationFunc(am.config.NodeID)
		if accuserRep < am.config.MinAccuserReputation {
			return nil, ErrLowReputation
		}
	}
	
	now := time.Now()
	
	// 生成指责ID
	idData := fmt.Sprintf("%s%s%d%s", am.config.NodeID, accused, now.UnixNano(), reason)
	hash := sha256.Sum256([]byte(idData))
	accusationID := hex.EncodeToString(hash[:16])
	
	// 计算惩罚值（高声誉指责者，惩罚更重）
	reputationFactor := am.calculateReputationFactor(accuserRep)
	basePenalty := am.config.BasePenalty * reputationFactor
	
	// 计算指责者代价（高声誉指责者，代价更低）
	costFactor := 1.0 / reputationFactor
	accuserCost := am.config.BaseAccuserCost * costFactor
	
	acc := &Accusation{
		AccusationID:     accusationID,
		Accuser:          am.config.NodeID,
		Accused:          accused,
		Type:             accusationType,
		Reason:           reason,
		Evidence:         evidence,
		Timestamp:        now,
		ExpiresAt:        now.Add(am.config.DefaultExpiry),
		Status:           StatusPending,
		AccuserReputation: accuserRep,
		BasePenalty:      basePenalty,
		AccuserCost:      accuserCost,
		PropagationDepth: 0,
		PropagatedTo:     make([]string, 0),
	}
	
	// 签名
	if am.config.SignFunc != nil {
		signData := am.getSignData(acc)
		sig, err := am.config.SignFunc(signData)
		if err != nil {
			return nil, fmt.Errorf("failed to sign accusation: %w", err)
		}
		acc.Signature = sig
	}
	
	am.mu.Lock()
	am.accusations[accusationID] = acc
	am.analyses[accusationID] = make([]*AccusationAnalysis, 0)
	am.mu.Unlock()
	
	// 扣除指责者声誉（代价）
	if am.config.UpdateReputationFunc != nil {
		am.config.UpdateReputationFunc(am.config.NodeID, -accuserCost)
	}
	
	// 保存
	am.save()
	
	// 触发回调
	if am.OnAccusationCreated != nil {
		am.OnAccusationCreated(acc)
	}
	
	return acc, nil
}

// getSignData 获取签名数据
func (am *AccusationManager) getSignData(acc *Accusation) []byte {
	data := fmt.Sprintf("%s|%s|%s|%s|%s|%d",
		acc.AccusationID,
		acc.Accuser,
		acc.Accused,
		acc.Type,
		acc.Reason,
		acc.Timestamp.UnixNano())
	return []byte(data)
}

// calculateReputationFactor 计算声誉因子
func (am *AccusationManager) calculateReputationFactor(reputation float64) float64 {
	// 声誉在 0-100 范围，归一化到 0.5-2.0
	if reputation < 0 {
		reputation = 0
	}
	if reputation > 100 {
		reputation = 100
	}
	return 0.5 + (reputation / 100.0) * 1.5
}

// PropagateAccusation 传播指责到邻居
func (am *AccusationManager) PropagateAccusation(accusationID string) ([]string, error) {
	am.mu.Lock()
	
	acc, ok := am.accusations[accusationID]
	if !ok {
		am.mu.Unlock()
		return nil, ErrAccusationNotFound
	}
	
	if acc.Status == StatusArchived {
		am.mu.Unlock()
		return nil, ErrAccusationExpired
	}
	
	am.mu.Unlock()
	
	// 获取邻居
	var neighbors []string
	if am.config.GetNeighborsFunc != nil {
		neighbors = am.config.GetNeighborsFunc(am.config.NodeID)
	}
	
	if len(neighbors) == 0 {
		return []string{}, nil
	}
	
	propagatedTo := make([]string, 0)
	
	for _, neighborID := range neighbors {
		if neighborID == acc.Accuser || neighborID == am.config.NodeID {
			continue
		}
		
		// 记录传播
		propagatedTo = append(propagatedTo, neighborID)
	}
	
	am.mu.Lock()
	acc.PropagatedTo = append(acc.PropagatedTo, propagatedTo...)
	acc.Status = StatusDelivered
	am.mu.Unlock()
	
	am.save()
	
	return propagatedTo, nil
}

// ReceiveAccusation 接收外部指责
func (am *AccusationManager) ReceiveAccusation(acc *Accusation, fromNode string) error {
	if acc == nil {
		return errors.New("accusation is nil")
	}
	if acc.AccusationID == "" {
		return errors.New("invalid accusation ID")
	}
	if acc.Accuser == "" {
		return ErrEmptyAccuser
	}
	if acc.Accused == "" {
		return ErrEmptyAccused
	}
	
	// 检查是否过期
	if time.Now().After(acc.ExpiresAt) {
		return ErrAccusationExpired
	}
	
	// 验证签名
	if am.config.VerifyFunc != nil && acc.Signature != "" {
		signData := am.getSignData(acc)
		if !am.config.VerifyFunc(acc.Accuser, signData, acc.Signature) {
			return ErrInvalidSignature
		}
	}
	
	am.mu.Lock()
	
	// 检查重复
	if _, exists := am.accusations[acc.AccusationID]; exists {
		am.mu.Unlock()
		return ErrDuplicateAccusation
	}
	
	// 检查耐受值
	if record, ok := am.tolerances[acc.Accuser]; ok {
		if record.RemainingTolerance < acc.BasePenalty {
			am.mu.Unlock()
			if am.OnToleranceExceeded != nil {
				am.OnToleranceExceeded(acc.Accuser, acc.BasePenalty)
			}
			return ErrToleranceExceeded
		}
		record.TotalPenaltyReceived += acc.BasePenalty
		record.RemainingTolerance -= acc.BasePenalty
	} else {
		now := time.Now()
		am.tolerances[acc.Accuser] = &ToleranceRecord{
			AccuserNodeID:        acc.Accuser,
			TotalPenaltyReceived: acc.BasePenalty,
			MaxTolerance:         am.config.DefaultTolerance,
			RemainingTolerance:   am.config.DefaultTolerance - acc.BasePenalty,
			LastResetTime:        now,
			NextResetTime:        now.Add(am.config.ToleranceResetPeriod),
		}
	}
	
	// 增加传播深度
	acc.PropagationDepth++
	
	// 存储
	am.accusations[acc.AccusationID] = acc
	am.analyses[acc.AccusationID] = make([]*AccusationAnalysis, 0)
	
	am.mu.Unlock()
	
	// 触发回调
	if am.OnAccusationReceived != nil {
		am.OnAccusationReceived(acc, fromNode)
	}
	
	return nil
}

// AnalyzeAccusation 分析指责
func (am *AccusationManager) AnalyzeAccusation(accusationID string, accepted bool, reason string) (*AccusationAnalysis, error) {
	am.mu.Lock()
	
	acc, ok := am.accusations[accusationID]
	if !ok {
		am.mu.Unlock()
		return nil, ErrAccusationNotFound
	}
	
	// 计算衰减后的惩罚
	decayedPenalty := acc.BasePenalty * pow(am.config.DecayFactor, acc.PropagationDepth)
	decayedCost := acc.AccuserCost * pow(am.config.DecayFactor, acc.PropagationDepth)
	
	now := time.Now()
	
	analysis := &AccusationAnalysis{
		AccusationID:     accusationID,
		AnalyzerNodeID:   am.config.NodeID,
		Timestamp:        now,
		PenaltyToAccused: decayedPenalty,
		CostToAccuser:    decayedCost,
		Accepted:         accepted,
		Reason:           reason,
	}
	
	// 签名分析
	if am.config.SignFunc != nil {
		analysisData := fmt.Sprintf("%s|%s|%t|%d", accusationID, am.config.NodeID, accepted, now.UnixNano())
		sig, _ := am.config.SignFunc([]byte(analysisData))
		analysis.Signature = sig
	}
	
	am.analyses[accusationID] = append(am.analyses[accusationID], analysis)
	
	if accepted {
		acc.Status = StatusVerified
	} else {
		acc.Status = StatusRejected
	}
	
	am.mu.Unlock()
	
	// 如果接受，应用惩罚
	if accepted && am.config.UpdateReputationFunc != nil {
		am.config.UpdateReputationFunc(acc.Accused, -decayedPenalty)
	}
	
	// 保存
	am.save()
	
	// 触发回调
	if accepted && am.OnAccusationVerified != nil {
		am.OnAccusationVerified(acc, analysis)
	}
	if !accepted && am.OnAccusationRejected != nil {
		am.OnAccusationRejected(acc, reason)
	}
	
	return analysis, nil
}

// pow 计算幂
func pow(base float64, exp int) float64 {
	result := 1.0
	for i := 0; i < exp; i++ {
		result *= base
	}
	return result
}

// GetAccusation 获取指责
func (am *AccusationManager) GetAccusation(accusationID string) (*Accusation, error) {
	am.mu.RLock()
	defer am.mu.RUnlock()
	
	acc, ok := am.accusations[accusationID]
	if !ok {
		return nil, ErrAccusationNotFound
	}
	return acc, nil
}

// GetAccusationsByAccuser 获取指责者发起的指责
func (am *AccusationManager) GetAccusationsByAccuser(accuserID string) []*Accusation {
	am.mu.RLock()
	defer am.mu.RUnlock()
	
	result := make([]*Accusation, 0)
	for _, acc := range am.accusations {
		if acc.Accuser == accuserID {
			result = append(result, acc)
		}
	}
	return result
}

// GetAccusationsByAccused 获取针对某节点的指责
func (am *AccusationManager) GetAccusationsByAccused(accusedID string) []*Accusation {
	am.mu.RLock()
	defer am.mu.RUnlock()
	
	result := make([]*Accusation, 0)
	for _, acc := range am.accusations {
		if acc.Accused == accusedID {
			result = append(result, acc)
		}
	}
	return result
}

// GetPendingAccusations 获取待处理的指责
func (am *AccusationManager) GetPendingAccusations() []*Accusation {
	am.mu.RLock()
	defer am.mu.RUnlock()
	
	result := make([]*Accusation, 0)
	for _, acc := range am.accusations {
		if acc.Status == StatusPending || acc.Status == StatusDelivered {
			result = append(result, acc)
		}
	}
	
	// 按时间排序
	sort.Slice(result, func(i, j int) bool {
		return result[i].Timestamp.After(result[j].Timestamp)
	})
	
	return result
}

// GetAnalyses 获取指责的分析结果
func (am *AccusationManager) GetAnalyses(accusationID string) []*AccusationAnalysis {
	am.mu.RLock()
	defer am.mu.RUnlock()
	
	if analyses, ok := am.analyses[accusationID]; ok {
		return analyses
	}
	return []*AccusationAnalysis{}
}

// GetToleranceRecord 获取耐受值记录
func (am *AccusationManager) GetToleranceRecord(accuserID string) *ToleranceRecord {
	am.mu.RLock()
	defer am.mu.RUnlock()
	
	if record, ok := am.tolerances[accuserID]; ok {
		return record
	}
	return nil
}

// GetAllTolerances 获取所有耐受值记录
func (am *AccusationManager) GetAllTolerances() []*ToleranceRecord {
	am.mu.RLock()
	defer am.mu.RUnlock()
	
	records := make([]*ToleranceRecord, 0, len(am.tolerances))
	for _, record := range am.tolerances {
		records = append(records, record)
	}
	return records
}

// SetTolerance 设置耐受值
func (am *AccusationManager) SetTolerance(accuserID string, tolerance float64) {
	am.mu.Lock()
	defer am.mu.Unlock()
	
	now := time.Now()
	if record, ok := am.tolerances[accuserID]; ok {
		record.MaxTolerance = tolerance
		record.RemainingTolerance = tolerance - record.TotalPenaltyReceived
		if record.RemainingTolerance < 0 {
			record.RemainingTolerance = 0
		}
	} else {
		am.tolerances[accuserID] = &ToleranceRecord{
			AccuserNodeID:        accuserID,
			TotalPenaltyReceived: 0,
			MaxTolerance:         tolerance,
			RemainingTolerance:   tolerance,
			LastResetTime:        now,
			NextResetTime:        now.Add(am.config.ToleranceResetPeriod),
		}
	}
}

// ResetTolerance 重置耐受值
func (am *AccusationManager) ResetTolerance(accuserID string) error {
	am.mu.Lock()
	defer am.mu.Unlock()
	
	record, ok := am.tolerances[accuserID]
	if !ok {
		return errors.New("tolerance record not found")
	}
	
	now := time.Now()
	record.TotalPenaltyReceived = 0
	record.RemainingTolerance = record.MaxTolerance
	record.LastResetTime = now
	record.NextResetTime = now.Add(am.config.ToleranceResetPeriod)
	
	return nil
}

// ContinuePropagation 继续传播指责到邻居
func (am *AccusationManager) ContinuePropagation(accusationID string) ([]string, error) {
	am.mu.Lock()
	
	acc, ok := am.accusations[accusationID]
	if !ok {
		am.mu.Unlock()
		return nil, ErrAccusationNotFound
	}
	
	// 检查传播深度
	if acc.PropagationDepth >= am.config.MaxPropagationDepth {
		am.mu.Unlock()
		return []string{}, nil
	}
	
	am.mu.Unlock()
	
	// 获取邻居
	var neighbors []string
	if am.config.GetNeighborsFunc != nil {
		neighbors = am.config.GetNeighborsFunc(am.config.NodeID)
	}
	
	propagatedTo := make([]string, 0)
	for _, neighborID := range neighbors {
		if neighborID == acc.Accuser || neighborID == am.config.NodeID {
			continue
		}
		propagatedTo = append(propagatedTo, neighborID)
	}
	
	am.mu.Lock()
	acc.PropagatedTo = append(acc.PropagatedTo, propagatedTo...)
	am.mu.Unlock()
	
	am.save()
	
	return propagatedTo, nil
}

// CalculatePenalty 计算指责对某节点的惩罚值
func (am *AccusationManager) CalculatePenalty(accuserReputation float64, depth int) float64 {
	repFactor := am.calculateReputationFactor(accuserReputation)
	penalty := am.config.BasePenalty * repFactor
	return penalty * pow(am.config.DecayFactor, depth)
}

// AccusationStats 统计信息
type AccusationStats struct {
	TotalAccusations     int64   `json:"total_accusations"`
	PendingAccusations   int64   `json:"pending_accusations"`
	VerifiedAccusations  int64   `json:"verified_accusations"`
	RejectedAccusations  int64   `json:"rejected_accusations"`
	TotalPenaltyApplied  float64 `json:"total_penalty_applied"`
	TotalAccuserCost     float64 `json:"total_accuser_cost"`
	ActiveTolerances     int     `json:"active_tolerances"`
	LastNaturalDecay     time.Time `json:"last_natural_decay"`
}

// GetStats 获取统计信息
func (am *AccusationManager) GetStats() *AccusationStats {
	am.mu.RLock()
	defer am.mu.RUnlock()
	
	stats := &AccusationStats{
		TotalAccusations:  int64(len(am.accusations)),
		ActiveTolerances:  len(am.tolerances),
		LastNaturalDecay:  am.lastDecayTime,
	}
	
	for _, acc := range am.accusations {
		switch acc.Status {
		case StatusPending, StatusDelivered:
			stats.PendingAccusations++
		case StatusVerified:
			stats.VerifiedAccusations++
			stats.TotalPenaltyApplied += acc.BasePenalty
		case StatusRejected:
			stats.RejectedAccusations++
		}
		stats.TotalAccuserCost += acc.AccuserCost
	}
	
	return stats
}

// SetDecayFactor 设置衰减因子
func (am *AccusationManager) SetDecayFactor(factor float64) {
	am.mu.Lock()
	am.config.DecayFactor = factor
	am.mu.Unlock()
}

// SetBasePenalty 设置基础惩罚
func (am *AccusationManager) SetBasePenalty(penalty float64) {
	am.mu.Lock()
	am.config.BasePenalty = penalty
	am.mu.Unlock()
}

// SetNaturalDecayAmount 设置自然衰减量
func (am *AccusationManager) SetNaturalDecayAmount(amount float64) {
	am.mu.Lock()
	am.config.NaturalDecayAmount = amount
	am.mu.Unlock()
}

// persistState 持久化状态
type persistState struct {
	Accusations   map[string]*Accusation             `json:"accusations"`
	Analyses      map[string][]*AccusationAnalysis   `json:"analyses"`
	Tolerances    map[string]*ToleranceRecord        `json:"tolerances"`
	LastDecayTime time.Time                          `json:"last_decay_time"`
}

// save 保存数据
func (am *AccusationManager) save() error {
	if am.config.DataDir == "" {
		return nil
	}
	
	am.mu.RLock()
	state := &persistState{
		Accusations:   am.accusations,
		Analyses:      am.analyses,
		Tolerances:    am.tolerances,
		LastDecayTime: am.lastDecayTime,
	}
	am.mu.RUnlock()
	
	data, err := json.MarshalIndent(state, "", "  ")
	if err != nil {
		return err
	}
	
	filePath := filepath.Join(am.config.DataDir, "accusation.json")
	return os.WriteFile(filePath, data, 0644)
}

// load 加载数据
func (am *AccusationManager) load() error {
	if am.config.DataDir == "" {
		return nil
	}
	
	filePath := filepath.Join(am.config.DataDir, "accusation.json")
	data, err := os.ReadFile(filePath)
	if err != nil {
		if os.IsNotExist(err) {
			return nil
		}
		return err
	}
	
	var state persistState
	if err := json.Unmarshal(data, &state); err != nil {
		return err
	}
	
	am.mu.Lock()
	defer am.mu.Unlock()
	
	if state.Accusations != nil {
		am.accusations = state.Accusations
	}
	if state.Analyses != nil {
		am.analyses = state.Analyses
	}
	if state.Tolerances != nil {
		am.tolerances = state.Tolerances
	}
	if !state.LastDecayTime.IsZero() {
		am.lastDecayTime = state.LastDecayTime
	}
	
	return nil
}

// Clear 清空所有数据
func (am *AccusationManager) Clear() {
	am.mu.Lock()
	defer am.mu.Unlock()
	
	am.accusations = make(map[string]*Accusation)
	am.analyses = make(map[string][]*AccusationAnalysis)
	am.tolerances = make(map[string]*ToleranceRecord)
}
