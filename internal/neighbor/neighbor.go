package neighbor

import (
	"context"
	"encoding/hex"
	"errors"
	"fmt"
	"sort"
	"sync"
	"time"
)

// 错误定义
var (
	ErrNeighborNotFound      = errors.New("邻居不存在")
	ErrNeighborAlreadyExists = errors.New("邻居已存在")
	ErrMaxNeighborsReached   = errors.New("已达最大邻居数")
	ErrReputationTooLow      = errors.New("声誉过低")
	ErrInvalidSignature      = errors.New("签名无效")
	ErrNeighborOffline       = errors.New("邻居离线")
)

// NeighborType 邻居类型
type NeighborType string

const (
	TypeNormal NeighborType = "normal" // 普通邻居
	TypeSuper  NeighborType = "super"  // 超级节点邻居
	TypeRelay  NeighborType = "relay"  // 中继邻居
)

// PingStatus 在线状态
type PingStatus string

const (
	StatusOnline  PingStatus = "online"
	StatusOffline PingStatus = "offline"
	StatusUnknown PingStatus = "unknown"
)

// Neighbor 邻居信息
type Neighbor struct {
	NodeID      string       `json:"node_id"`
	PublicKey   string       `json:"public_key"`
	Type        NeighborType `json:"type"`
	Reputation  int64        `json:"reputation"`
	Contribution int64       `json:"contribution"`
	LastSeen    time.Time    `json:"last_seen"`
	PingStatus  PingStatus   `json:"ping_status"`
	TrustScore  float64      `json:"trust_score"`
	Addresses   []string     `json:"addresses"`
	
	// 统计信息
	SuccessfulPings int `json:"successful_pings"`
	FailedPings     int `json:"failed_pings"`
	AddedAt         time.Time `json:"added_at"`
}

// NeighborConfig 邻居管理配置
type NeighborConfig struct {
	MinNeighbors       int           `json:"min_neighbors"`        // 最小邻居数
	MaxNeighbors       int           `json:"max_neighbors"`        // 最大邻居数
	MinReputation      int64         `json:"min_reputation"`       // 最低声誉要求
	PingInterval       time.Duration `json:"ping_interval"`        // 心跳间隔
	PingTimeout        time.Duration `json:"ping_timeout"`         // 心跳超时
	MaxPingFailures    int           `json:"max_ping_failures"`    // 最大心跳失败次数
	RefreshInterval    time.Duration `json:"refresh_interval"`     // 刷新间隔
	OfflineThreshold   time.Duration `json:"offline_threshold"`    // 离线阈值
}

// DefaultConfig 默认配置
func DefaultConfig() *NeighborConfig {
	return &NeighborConfig{
		MinNeighbors:       3,
		MaxNeighbors:       15,
		MinReputation:      5,
		PingInterval:       30 * time.Second,
		PingTimeout:        5 * time.Second,
		MaxPingFailures:    3,
		RefreshInterval:    5 * time.Minute,
		OfflineThreshold:   2 * time.Minute,
	}
}

// PingFunc 心跳函数类型
type PingFunc func(nodeID string) error

// ReputationFunc 获取节点声誉函数类型
type ReputationFunc func(nodeID string) (int64, error)

// CandidateProvider 候选邻居提供者
type CandidateProvider interface {
	GetCandidates(excludeIDs []string, count int) ([]*Neighbor, error)
}

// NeighborManager 邻居管理器
type NeighborManager struct {
	config      *NeighborConfig
	neighbors   map[string]*Neighbor
	candidates  map[string]*Neighbor
	mu          sync.RWMutex
	
	// 回调函数
	pingFunc       PingFunc
	reputationFunc ReputationFunc
	candidateProvider CandidateProvider
	
	// 事件通知
	onNeighborAdded   func(*Neighbor)
	onNeighborRemoved func(*Neighbor)
	onNeighborOffline func(*Neighbor)
	
	ctx    context.Context
	cancel context.CancelFunc
	wg     sync.WaitGroup
}

// NewNeighborManager 创建邻居管理器
func NewNeighborManager(config *NeighborConfig) *NeighborManager {
	if config == nil {
		config = DefaultConfig()
	}
	
	ctx, cancel := context.WithCancel(context.Background())
	
	return &NeighborManager{
		config:     config,
		neighbors:  make(map[string]*Neighbor),
		candidates: make(map[string]*Neighbor),
		ctx:        ctx,
		cancel:     cancel,
	}
}

// SetPingFunc 设置心跳函数
func (nm *NeighborManager) SetPingFunc(fn PingFunc) {
	nm.pingFunc = fn
}

// SetReputationFunc 设置声誉查询函数
func (nm *NeighborManager) SetReputationFunc(fn ReputationFunc) {
	nm.reputationFunc = fn
}

// SetCandidateProvider 设置候选邻居提供者
func (nm *NeighborManager) SetCandidateProvider(cp CandidateProvider) {
	nm.candidateProvider = cp
}

// SetOnNeighborAdded 设置邻居添加回调
func (nm *NeighborManager) SetOnNeighborAdded(fn func(*Neighbor)) {
	nm.onNeighborAdded = fn
}

// SetOnNeighborRemoved 设置邻居移除回调
func (nm *NeighborManager) SetOnNeighborRemoved(fn func(*Neighbor)) {
	nm.onNeighborRemoved = fn
}

// SetOnNeighborOffline 设置邻居离线回调
func (nm *NeighborManager) SetOnNeighborOffline(fn func(*Neighbor)) {
	nm.onNeighborOffline = fn
}

// Start 启动邻居管理
func (nm *NeighborManager) Start() {
	// 启动心跳检测
	nm.wg.Add(1)
	go nm.pingLoop()
	
	// 启动定期刷新
	nm.wg.Add(1)
	go nm.refreshLoop()
}

// Stop 停止邻居管理
func (nm *NeighborManager) Stop() {
	nm.cancel()
	nm.wg.Wait()
}

// AddNeighbor 添加邻居
func (nm *NeighborManager) AddNeighbor(neighbor *Neighbor) error {
	nm.mu.Lock()
	defer nm.mu.Unlock()
	
	// 检查是否已存在
	if _, ok := nm.neighbors[neighbor.NodeID]; ok {
		return ErrNeighborAlreadyExists
	}
	
	// 检查邻居数量
	if len(nm.neighbors) >= nm.config.MaxNeighbors {
		// 尝试移除低质量邻居
		if !nm.removeLowestQualityNeighborLocked() {
			return ErrMaxNeighborsReached
		}
	}
	
	// 检查声誉
	if neighbor.Reputation < nm.config.MinReputation {
		return ErrReputationTooLow
	}
	
	// 初始化
	neighbor.AddedAt = time.Now()
	neighbor.LastSeen = time.Now()
	neighbor.PingStatus = StatusUnknown
	if neighbor.TrustScore == 0 {
		neighbor.TrustScore = 0.5 // 默认信任分
	}
	
	nm.neighbors[neighbor.NodeID] = neighbor
	
	// 触发回调
	if nm.onNeighborAdded != nil {
		go nm.onNeighborAdded(neighbor)
	}
	
	return nil
}

// RemoveNeighbor 移除邻居
func (nm *NeighborManager) RemoveNeighbor(nodeID string, reason string) error {
	nm.mu.Lock()
	defer nm.mu.Unlock()
	
	neighbor, ok := nm.neighbors[nodeID]
	if !ok {
		return ErrNeighborNotFound
	}
	
	delete(nm.neighbors, nodeID)
	
	// 触发回调
	if nm.onNeighborRemoved != nil {
		go nm.onNeighborRemoved(neighbor)
	}
	
	return nil
}

// GetNeighbor 获取邻居
func (nm *NeighborManager) GetNeighbor(nodeID string) (*Neighbor, error) {
	nm.mu.RLock()
	defer nm.mu.RUnlock()
	
	neighbor, ok := nm.neighbors[nodeID]
	if !ok {
		return nil, ErrNeighborNotFound
	}
	
	return neighbor, nil
}

// GetAllNeighbors 获取所有邻居
func (nm *NeighborManager) GetAllNeighbors() []*Neighbor {
	nm.mu.RLock()
	defer nm.mu.RUnlock()
	
	neighbors := make([]*Neighbor, 0, len(nm.neighbors))
	for _, n := range nm.neighbors {
		neighbors = append(neighbors, n)
	}
	
	return neighbors
}

// GetOnlineNeighbors 获取在线邻居
func (nm *NeighborManager) GetOnlineNeighbors() []*Neighbor {
	nm.mu.RLock()
	defer nm.mu.RUnlock()
	
	neighbors := make([]*Neighbor, 0)
	for _, n := range nm.neighbors {
		if n.PingStatus == StatusOnline {
			neighbors = append(neighbors, n)
		}
	}
	
	return neighbors
}

// GetNeighborsByType 按类型获取邻居
func (nm *NeighborManager) GetNeighborsByType(t NeighborType) []*Neighbor {
	nm.mu.RLock()
	defer nm.mu.RUnlock()
	
	neighbors := make([]*Neighbor, 0)
	for _, n := range nm.neighbors {
		if n.Type == t {
			neighbors = append(neighbors, n)
		}
	}
	
	return neighbors
}

// NeighborCount 邻居数量
func (nm *NeighborManager) NeighborCount() int {
	nm.mu.RLock()
	defer nm.mu.RUnlock()
	return len(nm.neighbors)
}

// OnlineCount 在线邻居数量
func (nm *NeighborManager) OnlineCount() int {
	nm.mu.RLock()
	defer nm.mu.RUnlock()
	
	count := 0
	for _, n := range nm.neighbors {
		if n.PingStatus == StatusOnline {
			count++
		}
	}
	return count
}

// IsNeighbor 是否为邻居
func (nm *NeighborManager) IsNeighbor(nodeID string) bool {
	nm.mu.RLock()
	defer nm.mu.RUnlock()
	_, ok := nm.neighbors[nodeID]
	return ok
}

// UpdateNeighborReputation 更新邻居声誉
func (nm *NeighborManager) UpdateNeighborReputation(nodeID string, reputation int64) error {
	nm.mu.Lock()
	defer nm.mu.Unlock()
	
	neighbor, ok := nm.neighbors[nodeID]
	if !ok {
		return ErrNeighborNotFound
	}
	
	neighbor.Reputation = reputation
	
	// 如果声誉过低，移除邻居
	if reputation < nm.config.MinReputation {
		delete(nm.neighbors, nodeID)
		if nm.onNeighborRemoved != nil {
			go nm.onNeighborRemoved(neighbor)
		}
	}
	
	return nil
}

// UpdateNeighborContribution 更新邻居贡献
func (nm *NeighborManager) UpdateNeighborContribution(nodeID string, delta int64) error {
	nm.mu.Lock()
	defer nm.mu.Unlock()
	
	neighbor, ok := nm.neighbors[nodeID]
	if !ok {
		return ErrNeighborNotFound
	}
	
	neighbor.Contribution += delta
	nm.updateTrustScoreLocked(neighbor)
	
	return nil
}

// Ping 对单个邻居进行心跳检测
func (nm *NeighborManager) Ping(nodeID string) error {
	nm.mu.RLock()
	neighbor, ok := nm.neighbors[nodeID]
	nm.mu.RUnlock()
	
	if !ok {
		return ErrNeighborNotFound
	}
	
	if nm.pingFunc == nil {
		return errors.New("未设置心跳函数")
	}
	
	err := nm.pingFunc(nodeID)
	
	nm.mu.Lock()
	defer nm.mu.Unlock()
	
	// 重新获取邻居（可能已被移除）
	neighbor, ok = nm.neighbors[nodeID]
	if !ok {
		return ErrNeighborNotFound
	}
	
	if err == nil {
		neighbor.LastSeen = time.Now()
		neighbor.PingStatus = StatusOnline
		neighbor.SuccessfulPings++
		neighbor.FailedPings = 0
		nm.updateTrustScoreLocked(neighbor)
	} else {
		neighbor.FailedPings++
		if neighbor.FailedPings >= nm.config.MaxPingFailures {
			neighbor.PingStatus = StatusOffline
			if nm.onNeighborOffline != nil {
				go nm.onNeighborOffline(neighbor)
			}
		}
	}
	
	return err
}

// PingAll 对所有邻居进行心跳检测
func (nm *NeighborManager) PingAll() map[string]error {
	neighbors := nm.GetAllNeighbors()
	results := make(map[string]error)
	
	var wg sync.WaitGroup
	var mu sync.Mutex
	
	for _, n := range neighbors {
		wg.Add(1)
		go func(nodeID string) {
			defer wg.Done()
			err := nm.Ping(nodeID)
			mu.Lock()
			results[nodeID] = err
			mu.Unlock()
		}(n.NodeID)
	}
	
	wg.Wait()
	return results
}

// GetBestNeighbors 获取最优邻居（用于消息转发）
func (nm *NeighborManager) GetBestNeighbors(count int) []*Neighbor {
	nm.mu.RLock()
	defer nm.mu.RUnlock()
	
	// 收集在线邻居
	online := make([]*Neighbor, 0)
	for _, n := range nm.neighbors {
		if n.PingStatus == StatusOnline {
			online = append(online, n)
		}
	}
	
	// 按信任分排序
	sort.Slice(online, func(i, j int) bool {
		return online[i].TrustScore > online[j].TrustScore
	})
	
	if count > len(online) {
		count = len(online)
	}
	
	return online[:count]
}

// AddCandidate 添加候选邻居
func (nm *NeighborManager) AddCandidate(candidate *Neighbor) {
	nm.mu.Lock()
	defer nm.mu.Unlock()
	
	// 不添加已是邻居的节点
	if _, ok := nm.neighbors[candidate.NodeID]; ok {
		return
	}
	
	nm.candidates[candidate.NodeID] = candidate
}

// GetCandidates 获取候选邻居
func (nm *NeighborManager) GetCandidates() []*Neighbor {
	nm.mu.RLock()
	defer nm.mu.RUnlock()
	
	candidates := make([]*Neighbor, 0, len(nm.candidates))
	for _, c := range nm.candidates {
		candidates = append(candidates, c)
	}
	
	return candidates
}

// PromoteCandidate 将候选提升为邻居
func (nm *NeighborManager) PromoteCandidate(nodeID string) error {
	nm.mu.Lock()
	candidate, ok := nm.candidates[nodeID]
	if !ok {
		nm.mu.Unlock()
		return errors.New("候选邻居不存在")
	}
	delete(nm.candidates, nodeID)
	nm.mu.Unlock()
	
	return nm.AddNeighbor(candidate)
}

// NeedMoreNeighbors 是否需要更多邻居
func (nm *NeighborManager) NeedMoreNeighbors() bool {
	nm.mu.RLock()
	defer nm.mu.RUnlock()
	return len(nm.neighbors) < nm.config.MinNeighbors
}

// GetStats 获取统计信息
func (nm *NeighborManager) GetStats() map[string]interface{} {
	nm.mu.RLock()
	defer nm.mu.RUnlock()
	
	online := 0
	offline := 0
	totalReputation := int64(0)
	totalTrust := float64(0)
	
	for _, n := range nm.neighbors {
		if n.PingStatus == StatusOnline {
			online++
		} else if n.PingStatus == StatusOffline {
			offline++
		}
		totalReputation += n.Reputation
		totalTrust += n.TrustScore
	}
	
	avgReputation := int64(0)
	avgTrust := float64(0)
	if len(nm.neighbors) > 0 {
		avgReputation = totalReputation / int64(len(nm.neighbors))
		avgTrust = totalTrust / float64(len(nm.neighbors))
	}
	
	return map[string]interface{}{
		"total":           len(nm.neighbors),
		"online":          online,
		"offline":         offline,
		"candidates":      len(nm.candidates),
		"avg_reputation":  avgReputation,
		"avg_trust_score": avgTrust,
		"min_neighbors":   nm.config.MinNeighbors,
		"max_neighbors":   nm.config.MaxNeighbors,
	}
}

// 内部方法

func (nm *NeighborManager) pingLoop() {
	defer nm.wg.Done()
	
	ticker := time.NewTicker(nm.config.PingInterval)
	defer ticker.Stop()
	
	for {
		select {
		case <-nm.ctx.Done():
			return
		case <-ticker.C:
			nm.PingAll()
			nm.checkOfflineNeighbors()
		}
	}
}

func (nm *NeighborManager) refreshLoop() {
	defer nm.wg.Done()
	
	ticker := time.NewTicker(nm.config.RefreshInterval)
	defer ticker.Stop()
	
	for {
		select {
		case <-nm.ctx.Done():
			return
		case <-ticker.C:
			nm.refreshNeighbors()
		}
	}
}

func (nm *NeighborManager) checkOfflineNeighbors() {
	nm.mu.Lock()
	defer nm.mu.Unlock()
	
	now := time.Now()
	for nodeID, n := range nm.neighbors {
		if now.Sub(n.LastSeen) > nm.config.OfflineThreshold {
			if n.PingStatus != StatusOffline {
				n.PingStatus = StatusOffline
				if nm.onNeighborOffline != nil {
					go nm.onNeighborOffline(n)
				}
			}
			
			// 如果长时间离线，移除
			if now.Sub(n.LastSeen) > nm.config.OfflineThreshold*3 {
				delete(nm.neighbors, nodeID)
				if nm.onNeighborRemoved != nil {
					go nm.onNeighborRemoved(n)
				}
			}
		}
	}
}

func (nm *NeighborManager) refreshNeighbors() {
	// 更新声誉
	if nm.reputationFunc != nil {
		neighbors := nm.GetAllNeighbors()
		for _, n := range neighbors {
			if rep, err := nm.reputationFunc(n.NodeID); err == nil {
				nm.UpdateNeighborReputation(n.NodeID, rep)
			}
		}
	}
	
	// 补充邻居
	nm.fillNeighbors()
}

func (nm *NeighborManager) fillNeighbors() {
	nm.mu.RLock()
	needMore := len(nm.neighbors) < nm.config.MinNeighbors
	currentCount := len(nm.neighbors)
	nm.mu.RUnlock()
	
	if !needMore {
		return
	}
	
	// 从候选列表提升
	nm.mu.Lock()
	for nodeID, candidate := range nm.candidates {
		if currentCount >= nm.config.MinNeighbors {
			break
		}
		if candidate.Reputation >= nm.config.MinReputation {
			delete(nm.candidates, nodeID)
			nm.neighbors[nodeID] = candidate
			candidate.AddedAt = time.Now()
			candidate.LastSeen = time.Now()
			candidate.PingStatus = StatusUnknown
			currentCount++
			
			if nm.onNeighborAdded != nil {
				go nm.onNeighborAdded(candidate)
			}
		}
	}
	nm.mu.Unlock()
	
	// 从提供者获取更多候选
	if nm.candidateProvider != nil && currentCount < nm.config.MinNeighbors {
		excludeIDs := make([]string, 0)
		nm.mu.RLock()
		for id := range nm.neighbors {
			excludeIDs = append(excludeIDs, id)
		}
		nm.mu.RUnlock()
		
		needed := nm.config.MinNeighbors - currentCount
		if candidates, err := nm.candidateProvider.GetCandidates(excludeIDs, needed*2); err == nil {
			for _, c := range candidates {
				nm.AddCandidate(c)
			}
		}
	}
}

func (nm *NeighborManager) removeLowestQualityNeighborLocked() bool {
	if len(nm.neighbors) == 0 {
		return false
	}
	
	// 找到信任分最低的离线邻居
	var lowestNeighbor *Neighbor
	var lowestID string
	
	for id, n := range nm.neighbors {
		if n.PingStatus == StatusOffline {
			if lowestNeighbor == nil || n.TrustScore < lowestNeighbor.TrustScore {
				lowestNeighbor = n
				lowestID = id
			}
		}
	}
	
	// 如果没有离线邻居，找信任分最低的
	if lowestNeighbor == nil {
		for id, n := range nm.neighbors {
			if lowestNeighbor == nil || n.TrustScore < lowestNeighbor.TrustScore {
				lowestNeighbor = n
				lowestID = id
			}
		}
	}
	
	if lowestNeighbor != nil {
		delete(nm.neighbors, lowestID)
		// 添加到候选列表
		nm.candidates[lowestID] = lowestNeighbor
		if nm.onNeighborRemoved != nil {
			go nm.onNeighborRemoved(lowestNeighbor)
		}
		return true
	}
	
	return false
}

func (nm *NeighborManager) updateTrustScoreLocked(n *Neighbor) {
	// 信任分计算：
	// 基于成功心跳率、声誉、贡献
	totalPings := n.SuccessfulPings + n.FailedPings
	pingRate := float64(0.5)
	if totalPings > 0 {
		pingRate = float64(n.SuccessfulPings) / float64(totalPings)
	}
	
	reputationScore := float64(n.Reputation) / 100.0
	if reputationScore > 1.0 {
		reputationScore = 1.0
	}
	
	contributionScore := float64(n.Contribution) / 100.0
	if contributionScore > 1.0 {
		contributionScore = 1.0
	}
	
	// 加权计算
	n.TrustScore = pingRate*0.4 + reputationScore*0.4 + contributionScore*0.2
}

// ExportNeighbors 导出邻居列表（用于持久化）
func (nm *NeighborManager) ExportNeighbors() []*Neighbor {
	return nm.GetAllNeighbors()
}

// ImportNeighbors 导入邻居列表
func (nm *NeighborManager) ImportNeighbors(neighbors []*Neighbor) {
	for _, n := range neighbors {
		nm.AddNeighbor(n)
	}
}

// String 格式化输出
func (n *Neighbor) String() string {
	return fmt.Sprintf("Neighbor{ID: %s, Type: %s, Rep: %d, Status: %s, Trust: %.2f}",
		truncateID(n.NodeID), n.Type, n.Reputation, n.PingStatus, n.TrustScore)
}

func truncateID(id string) string {
	if len(id) > 8 {
		return id[:8] + "..."
	}
	return id
}

// FormatPublicKey 格式化公钥
func FormatPublicKey(pubKey []byte) string {
	return hex.EncodeToString(pubKey)
}
