// Package security - 安全管理器
// 整合速率限制、行为分析等安全功能
package security

import (
	"sync"
	"time"
)

// SecurityManager 安全管理器
type SecurityManager struct {
	mu sync.RWMutex

	// 各功能的限流器
	bulletinLimiter *RateLimiter
	mailboxLimiter  *RateLimiter
	messageLimiter  *RateLimiter

	// 行为分析器
	behaviorAnalyzer *BehaviorAnalyzer

	// 声誉查询函数
	getReputation func(nodeID string) float64

	// 黑名单
	blacklist map[string]time.Time // nodeID -> 解禁时间

	// 回调函数
	OnSecurityEvent func(event SecurityEvent)
}

// SecurityEvent 安全事件
type SecurityEvent struct {
	Timestamp   time.Time `json:"timestamp"`
	Type        string    `json:"type"`        // rate_limit, suspicious, sybil, blacklist
	NodeID      string    `json:"node_id"`
	Description string    `json:"description"`
	Severity    string    `json:"severity"`    // low, medium, high, critical
	Action      string    `json:"action"`      // blocked, warned, monitored
}

// NewSecurityManager 创建安全管理器
func NewSecurityManager() *SecurityManager {
	sm := &SecurityManager{
		bulletinLimiter:  NewRateLimiter("bulletin", BulletinRateLimitConfig()),
		mailboxLimiter:   NewRateLimiter("mailbox", MailboxRateLimitConfig()),
		messageLimiter:   NewRateLimiter("message", DefaultRateLimitConfig()),
		behaviorAnalyzer: NewBehaviorAnalyzer(DefaultBehaviorAnalyzerConfig()),
		blacklist:        make(map[string]time.Time),
	}

	// 设置行为分析器回调
	sm.behaviorAnalyzer.OnSuspiciousBehavior = func(nodeID string, reason string, score float64) {
		sm.handleSuspiciousBehavior(nodeID, reason, score)
	}
	sm.behaviorAnalyzer.OnSybilDetected = func(group []string) {
		sm.handleSybilDetected(group)
	}

	return sm
}

// SetReputationFunc 设置声誉查询函数
func (sm *SecurityManager) SetReputationFunc(fn func(nodeID string) float64) {
	sm.mu.Lock()
	defer sm.mu.Unlock()

	sm.getReputation = fn
	sm.bulletinLimiter.SetReputationFunc(fn)
	sm.mailboxLimiter.SetReputationFunc(fn)
	sm.messageLimiter.SetReputationFunc(fn)
}

// CheckBulletinPublish 检查是否允许发布留言
func (sm *SecurityManager) CheckBulletinPublish(nodeID string) error {
	sm.mu.RLock()
	defer sm.mu.RUnlock()

	// 检查黑名单
	if err := sm.checkBlacklist(nodeID); err != nil {
		return err
	}

	// 检查限流
	if err := sm.bulletinLimiter.Allow(nodeID); err != nil {
		sm.emitEvent(SecurityEvent{
			Timestamp:   time.Now(),
			Type:        "rate_limit",
			NodeID:      nodeID,
			Description: "Bulletin publish rate limited: " + err.Error(),
			Severity:    "medium",
			Action:      "blocked",
		})
		return err
	}

	return nil
}

// ConsumeBulletinQuota 消费留言板配额
func (sm *SecurityManager) ConsumeBulletinQuota(nodeID string, topic string) {
	sm.bulletinLimiter.Consume(nodeID)

	// 记录行为
	sm.behaviorAnalyzer.RecordEvent(BehaviorEvent{
		NodeID: nodeID,
		Type:   BehaviorPublish,
		Target: topic,
	})
}

// CheckMailboxSend 检查是否允许发送邮件
func (sm *SecurityManager) CheckMailboxSend(nodeID string) error {
	sm.mu.RLock()
	defer sm.mu.RUnlock()

	// 检查黑名单
	if err := sm.checkBlacklist(nodeID); err != nil {
		return err
	}

	// 检查限流
	if err := sm.mailboxLimiter.Allow(nodeID); err != nil {
		sm.emitEvent(SecurityEvent{
			Timestamp:   time.Now(),
			Type:        "rate_limit",
			NodeID:      nodeID,
			Description: "Mailbox send rate limited: " + err.Error(),
			Severity:    "medium",
			Action:      "blocked",
		})
		return err
	}

	return nil
}

// ConsumeMailboxQuota 消费邮箱配额
func (sm *SecurityManager) ConsumeMailboxQuota(nodeID string, to string) {
	sm.mailboxLimiter.Consume(nodeID)

	// 记录行为
	sm.behaviorAnalyzer.RecordEvent(BehaviorEvent{
		NodeID: nodeID,
		Type:   BehaviorSendMail,
		Target: to,
	})
}

// CheckMessage 检查是否允许发送消息
func (sm *SecurityManager) CheckMessage(nodeID string) error {
	sm.mu.RLock()
	defer sm.mu.RUnlock()

	if err := sm.checkBlacklist(nodeID); err != nil {
		return err
	}

	return sm.messageLimiter.Allow(nodeID)
}

// ConsumeMessageQuota 消费消息配额
func (sm *SecurityManager) ConsumeMessageQuota(nodeID string) {
	sm.messageLimiter.Consume(nodeID)
}

// checkBlacklist 检查黑名单
func (sm *SecurityManager) checkBlacklist(nodeID string) error {
	if expiry, exists := sm.blacklist[nodeID]; exists {
		if time.Now().Before(expiry) {
			return ErrBlacklisted
		}
		// 已过期，移除
		delete(sm.blacklist, nodeID)
	}
	return nil
}

// Blacklist 将节点加入黑名单
func (sm *SecurityManager) Blacklist(nodeID string, duration time.Duration) {
	sm.mu.Lock()
	defer sm.mu.Unlock()

	sm.blacklist[nodeID] = time.Now().Add(duration)

	sm.emitEvent(SecurityEvent{
		Timestamp:   time.Now(),
		Type:        "blacklist",
		NodeID:      nodeID,
		Description: "Node blacklisted for " + duration.String(),
		Severity:    "high",
		Action:      "blocked",
	})
}

// Unblacklist 从黑名单移除
func (sm *SecurityManager) Unblacklist(nodeID string) {
	sm.mu.Lock()
	defer sm.mu.Unlock()

	delete(sm.blacklist, nodeID)
}

// IsBlacklisted 检查是否在黑名单中
func (sm *SecurityManager) IsBlacklisted(nodeID string) bool {
	sm.mu.RLock()
	defer sm.mu.RUnlock()

	if expiry, exists := sm.blacklist[nodeID]; exists {
		return time.Now().Before(expiry)
	}
	return false
}

// handleSuspiciousBehavior 处理可疑行为
func (sm *SecurityManager) handleSuspiciousBehavior(nodeID string, reason string, score float64) {
	severity := "medium"
	action := "monitored"

	if score >= 0.9 {
		severity = "critical"
		action = "blocked"
		// 自动加入黑名单
		sm.mu.Lock()
		sm.blacklist[nodeID] = time.Now().Add(2 * time.Hour)
		sm.mu.Unlock()
	} else if score >= 0.8 {
		severity = "high"
		action = "warned"
	}

	sm.emitEvent(SecurityEvent{
		Timestamp:   time.Now(),
		Type:        "suspicious",
		NodeID:      nodeID,
		Description: reason,
		Severity:    severity,
		Action:      action,
	})
}

// handleSybilDetected 处理女巫攻击检测
func (sm *SecurityManager) handleSybilDetected(group []string) {
	for _, nodeID := range group {
		sm.emitEvent(SecurityEvent{
			Timestamp:   time.Now(),
			Type:        "sybil",
			NodeID:      nodeID,
			Description: "Possible Sybil attack member",
			Severity:    "critical",
			Action:      "monitored",
		})
	}
}

// emitEvent 发送安全事件
func (sm *SecurityManager) emitEvent(event SecurityEvent) {
	if sm.OnSecurityEvent != nil {
		sm.OnSecurityEvent(event)
	}
}

// GetBulletinStatus 获取留言板限流状态
func (sm *SecurityManager) GetBulletinStatus(nodeID string) *RateLimitStatus {
	return sm.bulletinLimiter.GetStatus(nodeID)
}

// GetMailboxStatus 获取邮箱限流状态
func (sm *SecurityManager) GetMailboxStatus(nodeID string) *RateLimitStatus {
	return sm.mailboxLimiter.GetStatus(nodeID)
}

// GetNodeBehavior 获取节点行为分析
func (sm *SecurityManager) GetNodeBehavior(nodeID string) *NodeBehavior {
	return sm.behaviorAnalyzer.GetNodeBehavior(nodeID)
}

// DetectSybilAttack 检测女巫攻击
func (sm *SecurityManager) DetectSybilAttack() [][]string {
	return sm.behaviorAnalyzer.DetectSybilAttack()
}

// GenerateSecurityReport 生成安全报告
func (sm *SecurityManager) GenerateSecurityReport() *SecurityReport {
	sm.mu.RLock()
	defer sm.mu.RUnlock()

	behaviorReport := sm.behaviorAnalyzer.GenerateReport()

	blacklistedNodes := make([]string, 0)
	for nodeID, expiry := range sm.blacklist {
		if time.Now().Before(expiry) {
			blacklistedNodes = append(blacklistedNodes, nodeID)
		}
	}

	return &SecurityReport{
		Timestamp:       time.Now(),
		BlacklistedCount: len(blacklistedNodes),
		BlacklistedNodes: blacklistedNodes,
		BehaviorReport:   behaviorReport,
	}
}

// SecurityReport 安全报告
type SecurityReport struct {
	Timestamp        time.Time       `json:"timestamp"`
	BlacklistedCount int             `json:"blacklisted_count"`
	BlacklistedNodes []string        `json:"blacklisted_nodes,omitempty"`
	BehaviorReport   *AnalysisReport `json:"behavior_report"`
}

// RecordVote 记录投票行为
func (sm *SecurityManager) RecordVote(nodeID string, proposalID string) {
	sm.behaviorAnalyzer.RecordEvent(BehaviorEvent{
		NodeID: nodeID,
		Type:   BehaviorVote,
		Target: proposalID,
	})
}

// RecordAccusation 记录指责行为
func (sm *SecurityManager) RecordAccusation(nodeID string, targetID string) {
	sm.behaviorAnalyzer.RecordEvent(BehaviorEvent{
		NodeID: nodeID,
		Type:   BehaviorAccuse,
		Target: targetID,
	})
}
