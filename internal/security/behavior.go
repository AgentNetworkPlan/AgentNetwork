// Package security - 行为分析器
// 检测异常行为模式，如女巫攻击、协调攻击等
package security

import (
	"math"
	"sync"
	"time"
)

// BehaviorType 行为类型
type BehaviorType string

const (
	BehaviorPublish    BehaviorType = "publish"     // 发布消息
	BehaviorSendMail   BehaviorType = "send_mail"   // 发送邮件
	BehaviorVote       BehaviorType = "vote"        // 投票
	BehaviorAccuse     BehaviorType = "accuse"      // 指责
	BehaviorConnect    BehaviorType = "connect"     // 连接
	BehaviorDisconnect BehaviorType = "disconnect"  // 断开
)

// BehaviorEvent 行为事件
type BehaviorEvent struct {
	NodeID    string       `json:"node_id"`
	Type      BehaviorType `json:"type"`
	Target    string       `json:"target,omitempty"` // 目标节点/话题
	Timestamp time.Time    `json:"timestamp"`
	Metadata  map[string]interface{} `json:"metadata,omitempty"`
}

// NodeBehavior 节点行为记录
type NodeBehavior struct {
	NodeID           string    `json:"node_id"`
	FirstSeen        time.Time `json:"first_seen"`
	LastSeen         time.Time `json:"last_seen"`
	
	// 行为统计
	TotalActions     int64     `json:"total_actions"`
	RecentActions    []time.Time `json:"-"` // 最近行为时间（用于计算频率）
	
	// 按类型统计
	ActionCounts     map[BehaviorType]int64 `json:"action_counts"`
	
	// 目标分析
	TargetCounts     map[string]int64 `json:"target_counts"` // 经常交互的目标
	
	// 时间模式
	HourlyPattern    [24]int64 `json:"hourly_pattern"`  // 每小时活动分布
	DailyPattern     [7]int64  `json:"daily_pattern"`   // 每周活动分布
	
	// 异常指标
	SuspicionScore   float64   `json:"suspicion_score"`
	Flags            []string  `json:"flags,omitempty"`
}

// BehaviorAnalyzerConfig 行为分析器配置
type BehaviorAnalyzerConfig struct {
	// 女巫攻击检测
	SybilDetectionWindow    time.Duration // 检测时间窗口
	SybilMinCorrelation     float64       // 最小相关性阈值
	SybilMinGroupSize       int           // 最小群组大小
	
	// 垃圾行为检测
	SpamBurstThreshold      int           // 突发行为阈值
	SpamBurstWindow         time.Duration // 突发检测窗口
	
	// 协调攻击检测
	CoordinatedTimeWindow   time.Duration // 协调行为时间窗口
	CoordinatedMinNodes     int           // 最小协调节点数
	
	// 异常行为阈值
	AnomalyScoreThreshold   float64       // 异常分数阈值
	
	// 历史保留
	MaxHistoryDuration      time.Duration // 最大历史保留时间
	MaxEventsPerNode        int           // 每节点最大事件数
}

// DefaultBehaviorAnalyzerConfig 默认配置
func DefaultBehaviorAnalyzerConfig() *BehaviorAnalyzerConfig {
	return &BehaviorAnalyzerConfig{
		SybilDetectionWindow:    10 * time.Minute,
		SybilMinCorrelation:     0.8,
		SybilMinGroupSize:       3,
		SpamBurstThreshold:      20,
		SpamBurstWindow:         1 * time.Minute,
		CoordinatedTimeWindow:   30 * time.Second,
		CoordinatedMinNodes:     3,
		AnomalyScoreThreshold:   0.7,
		MaxHistoryDuration:      24 * time.Hour,
		MaxEventsPerNode:        1000,
	}
}

// BehaviorAnalyzer 行为分析器
type BehaviorAnalyzer struct {
	mu       sync.RWMutex
	config   *BehaviorAnalyzerConfig
	nodes    map[string]*NodeBehavior
	events   []BehaviorEvent  // 全局事件队列
	
	// 检测结果缓存
	sybilGroups     [][]string  // 检测到的女巫群组
	lastSybilCheck  time.Time
	
	// 回调
	OnSuspiciousBehavior func(nodeID string, reason string, score float64)
	OnSybilDetected      func(group []string)
}

// NewBehaviorAnalyzer 创建行为分析器
func NewBehaviorAnalyzer(config *BehaviorAnalyzerConfig) *BehaviorAnalyzer {
	if config == nil {
		config = DefaultBehaviorAnalyzerConfig()
	}
	return &BehaviorAnalyzer{
		config:  config,
		nodes:   make(map[string]*NodeBehavior),
		events:  make([]BehaviorEvent, 0),
	}
}

// RecordEvent 记录行为事件
func (ba *BehaviorAnalyzer) RecordEvent(event BehaviorEvent) {
	ba.mu.Lock()
	defer ba.mu.Unlock()
	
	if event.Timestamp.IsZero() {
		event.Timestamp = time.Now()
	}
	
	// 获取或创建节点行为记录
	behavior, exists := ba.nodes[event.NodeID]
	if !exists {
		behavior = &NodeBehavior{
			NodeID:       event.NodeID,
			FirstSeen:    event.Timestamp,
			ActionCounts: make(map[BehaviorType]int64),
			TargetCounts: make(map[string]int64),
		}
		ba.nodes[event.NodeID] = behavior
	}
	
	// 更新统计
	behavior.LastSeen = event.Timestamp
	behavior.TotalActions++
	behavior.ActionCounts[event.Type]++
	
	if event.Target != "" {
		behavior.TargetCounts[event.Target]++
	}
	
	// 更新时间模式
	hour := event.Timestamp.Hour()
	weekday := int(event.Timestamp.Weekday())
	behavior.HourlyPattern[hour]++
	behavior.DailyPattern[weekday]++
	
	// 记录最近行为
	behavior.RecentActions = append(behavior.RecentActions, event.Timestamp)
	// 限制记录数量
	if len(behavior.RecentActions) > ba.config.MaxEventsPerNode {
		behavior.RecentActions = behavior.RecentActions[len(behavior.RecentActions)-ba.config.MaxEventsPerNode:]
	}
	
	// 添加到全局事件队列
	ba.events = append(ba.events, event)
	
	// 清理过期事件
	ba.cleanupOldEvents()
	
	// 实时检测
	ba.detectAnomaliesForNode(event.NodeID)
}

// cleanupOldEvents 清理过期事件
func (ba *BehaviorAnalyzer) cleanupOldEvents() {
	cutoff := time.Now().Add(-ba.config.MaxHistoryDuration)
	
	// 清理全局事件
	newEvents := make([]BehaviorEvent, 0)
	for _, e := range ba.events {
		if e.Timestamp.After(cutoff) {
			newEvents = append(newEvents, e)
		}
	}
	ba.events = newEvents
	
	// 清理节点的最近行为
	for _, behavior := range ba.nodes {
		newRecent := make([]time.Time, 0)
		for _, t := range behavior.RecentActions {
			if t.After(cutoff) {
				newRecent = append(newRecent, t)
			}
		}
		behavior.RecentActions = newRecent
	}
}

// detectAnomaliesForNode 检测单个节点的异常行为
func (ba *BehaviorAnalyzer) detectAnomaliesForNode(nodeID string) {
	behavior, exists := ba.nodes[nodeID]
	if !exists {
		return
	}
	
	var flags []string
	var totalScore float64
	
	// 1. 检测突发行为（垃圾攻击）
	burstScore := ba.detectBurstBehavior(behavior)
	if burstScore > 0.5 {
		flags = append(flags, "burst_behavior")
		totalScore += burstScore * 0.4
	}
	
	// 2. 检测时间模式异常
	patternScore := ba.detectPatternAnomaly(behavior)
	if patternScore > 0.5 {
		flags = append(flags, "unusual_pattern")
		totalScore += patternScore * 0.3
	}
	
	// 3. 检测目标集中度
	targetScore := ba.detectTargetConcentration(behavior)
	if targetScore > 0.5 {
		flags = append(flags, "target_concentration")
		totalScore += targetScore * 0.3
	}
	
	behavior.SuspicionScore = totalScore
	behavior.Flags = flags
	
	// 触发回调
	if totalScore >= ba.config.AnomalyScoreThreshold && ba.OnSuspiciousBehavior != nil {
		reason := "suspicious behavior detected"
		if len(flags) > 0 {
			reason = flags[0]
		}
		ba.OnSuspiciousBehavior(nodeID, reason, totalScore)
	}
}

// detectBurstBehavior 检测突发行为
func (ba *BehaviorAnalyzer) detectBurstBehavior(behavior *NodeBehavior) float64 {
	now := time.Now()
	windowStart := now.Add(-ba.config.SpamBurstWindow)
	
	// 统计时间窗口内的行为数
	count := 0
	for _, t := range behavior.RecentActions {
		if t.After(windowStart) {
			count++
		}
	}
	
	// 计算得分
	if count >= ba.config.SpamBurstThreshold {
		return 1.0
	}
	if count >= ba.config.SpamBurstThreshold/2 {
		return float64(count) / float64(ba.config.SpamBurstThreshold)
	}
	return 0
}

// detectPatternAnomaly 检测时间模式异常
func (ba *BehaviorAnalyzer) detectPatternAnomaly(behavior *NodeBehavior) float64 {
	// 计算小时分布的熵
	entropy := ba.calculateEntropy(behavior.HourlyPattern[:])
	
	// 正常用户的熵应该在某个范围内
	// 非常规律（熵低）或完全随机（熵高）都可能是异常
	maxEntropy := math.Log2(24) // 完全随机的熵
	
	// 如果熵太低，说明行为过于集中在某些时间
	if entropy < maxEntropy*0.3 {
		return 0.7
	}
	
	return 0
}

// detectTargetConcentration 检测目标集中度
func (ba *BehaviorAnalyzer) detectTargetConcentration(behavior *NodeBehavior) float64 {
	if len(behavior.TargetCounts) == 0 {
		return 0
	}
	
	// 计算最高频目标的占比
	var maxCount, totalCount int64
	for _, count := range behavior.TargetCounts {
		if count > maxCount {
			maxCount = count
		}
		totalCount += count
	}
	
	if totalCount == 0 {
		return 0
	}
	
	concentration := float64(maxCount) / float64(totalCount)
	
	// 如果超过80%的行为都针对同一目标，可能是攻击
	if concentration > 0.8 {
		return concentration
	}
	
	return 0
}

// calculateEntropy 计算熵
func (ba *BehaviorAnalyzer) calculateEntropy(counts []int64) float64 {
	var total int64
	for _, c := range counts {
		total += c
	}
	
	if total == 0 {
		return 0
	}
	
	var entropy float64
	for _, c := range counts {
		if c > 0 {
			p := float64(c) / float64(total)
			entropy -= p * math.Log2(p)
		}
	}
	
	return entropy
}

// DetectSybilAttack 检测女巫攻击
// 通过分析多个节点的行为相关性来检测
func (ba *BehaviorAnalyzer) DetectSybilAttack() [][]string {
	ba.mu.Lock()
	defer ba.mu.Unlock()
	
	now := time.Now()
	windowStart := now.Add(-ba.config.SybilDetectionWindow)
	
	// 收集时间窗口内的活跃节点
	activeNodes := make([]string, 0)
	for nodeID, behavior := range ba.nodes {
		if behavior.LastSeen.After(windowStart) {
			activeNodes = append(activeNodes, nodeID)
		}
	}
	
	if len(activeNodes) < ba.config.SybilMinGroupSize {
		return nil
	}
	
	// 计算节点间的行为相关性
	groups := make([][]string, 0)
	visited := make(map[string]bool)
	
	for _, node1 := range activeNodes {
		if visited[node1] {
			continue
		}
		
		group := []string{node1}
		visited[node1] = true
		
		for _, node2 := range activeNodes {
			if visited[node2] || node1 == node2 {
				continue
			}
			
			correlation := ba.calculateBehaviorCorrelation(node1, node2, windowStart)
			if correlation >= ba.config.SybilMinCorrelation {
				group = append(group, node2)
				visited[node2] = true
			}
		}
		
		if len(group) >= ba.config.SybilMinGroupSize {
			groups = append(groups, group)
			
			// 触发回调
			if ba.OnSybilDetected != nil {
				ba.OnSybilDetected(group)
			}
		}
	}
	
	ba.sybilGroups = groups
	ba.lastSybilCheck = now
	
	return groups
}

// calculateBehaviorCorrelation 计算两个节点的行为相关性
func (ba *BehaviorAnalyzer) calculateBehaviorCorrelation(node1, node2 string, windowStart time.Time) float64 {
	behavior1 := ba.nodes[node1]
	behavior2 := ba.nodes[node2]
	
	if behavior1 == nil || behavior2 == nil {
		return 0
	}
	
	// 获取时间窗口内的行为时间
	times1 := filterTimes(behavior1.RecentActions, windowStart)
	times2 := filterTimes(behavior2.RecentActions, windowStart)
	
	if len(times1) == 0 || len(times2) == 0 {
		return 0
	}
	
	// 计算时间接近度
	// 如果两个节点的行为在时间上高度同步，说明可能是同一控制者
	matchCount := 0
	for _, t1 := range times1 {
		for _, t2 := range times2 {
			diff := t1.Sub(t2)
			if diff < 0 {
				diff = -diff
			}
			if diff < ba.config.CoordinatedTimeWindow {
				matchCount++
			}
		}
	}
	
	// 归一化
	maxMatches := len(times1)
	if len(times2) < maxMatches {
		maxMatches = len(times2)
	}
	
	if maxMatches == 0 {
		return 0
	}
	
	return float64(matchCount) / float64(maxMatches)
}

// filterTimes 过滤时间窗口内的时间点
func filterTimes(times []time.Time, windowStart time.Time) []time.Time {
	result := make([]time.Time, 0)
	for _, t := range times {
		if t.After(windowStart) {
			result = append(result, t)
		}
	}
	return result
}

// GetNodeBehavior 获取节点行为分析
func (ba *BehaviorAnalyzer) GetNodeBehavior(nodeID string) *NodeBehavior {
	ba.mu.RLock()
	defer ba.mu.RUnlock()
	return ba.nodes[nodeID]
}

// GetSuspiciousNodes 获取可疑节点列表
func (ba *BehaviorAnalyzer) GetSuspiciousNodes(threshold float64) []*NodeBehavior {
	ba.mu.RLock()
	defer ba.mu.RUnlock()
	
	result := make([]*NodeBehavior, 0)
	for _, behavior := range ba.nodes {
		if behavior.SuspicionScore >= threshold {
			result = append(result, behavior)
		}
	}
	return result
}

// AnalysisReport 分析报告
type AnalysisReport struct {
	Timestamp       time.Time        `json:"timestamp"`
	TotalNodes      int              `json:"total_nodes"`
	SuspiciousNodes int              `json:"suspicious_nodes"`
	SybilGroups     [][]string       `json:"sybil_groups,omitempty"`
	TopSuspicious   []*NodeBehavior  `json:"top_suspicious,omitempty"`
}

// GenerateReport 生成分析报告
func (ba *BehaviorAnalyzer) GenerateReport() *AnalysisReport {
	ba.mu.RLock()
	defer ba.mu.RUnlock()
	
	suspicious := ba.GetSuspiciousNodes(ba.config.AnomalyScoreThreshold)
	
	// 获取Top 10 可疑节点
	topSuspicious := suspicious
	if len(topSuspicious) > 10 {
		topSuspicious = topSuspicious[:10]
	}
	
	return &AnalysisReport{
		Timestamp:       time.Now(),
		TotalNodes:      len(ba.nodes),
		SuspiciousNodes: len(suspicious),
		SybilGroups:     ba.sybilGroups,
		TopSuspicious:   topSuspicious,
	}
}
