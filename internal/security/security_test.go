package security

import (
	"testing"
	"time"
)

func TestRateLimiter_BasicUsage(t *testing.T) {
	limiter := NewRateLimiter("test", &RateLimitConfig{
		MaxPerSecond: 2,
		MaxPerMinute: 10,
		MaxPerHour:   100,
		MaxPerDay:    1000,
		MinReputation: 0,
		BanDuration:   time.Hour,
	})

	nodeID := "test-node"

	// 前两次应该允许
	for i := 0; i < 2; i++ {
		if err := limiter.AllowAndConsume(nodeID); err != nil {
			t.Errorf("Request %d should be allowed: %v", i+1, err)
		}
	}

	// 第三次应该被拒绝（每秒限制2次）
	if err := limiter.AllowAndConsume(nodeID); err == nil {
		t.Error("Request 3 should be rate limited")
	}
}

func TestRateLimiter_ReputationThreshold(t *testing.T) {
	limiter := NewRateLimiter("test", &RateLimitConfig{
		MaxPerSecond:  5,
		MaxPerMinute:  30,
		MaxPerHour:    100,
		MaxPerDay:     1000,
		MinReputation: 10.0,
		BanDuration:   time.Hour,
	})

	// 设置声誉函数返回低声誉
	limiter.SetReputationFunc(func(nodeID string) float64 {
		return 5.0 // 低于阈值
	})

	nodeID := "low-rep-node"

	// 应该因声誉过低被拒绝
	err := limiter.Allow(nodeID)
	if err != ErrReputationTooLow {
		t.Errorf("Expected ErrReputationTooLow, got: %v", err)
	}
}

func TestRateLimiter_HighReputationBonus(t *testing.T) {
	limiter := NewRateLimiter("test", &RateLimitConfig{
		MaxPerSecond:         2,
		MaxPerMinute:         10,
		MaxPerHour:           100,
		MaxPerDay:            1000,
		ReputationMultiplier: 2.0,
		MinReputation:        0,
		BanDuration:          time.Hour,
	})

	// 高声誉节点
	limiter.SetReputationFunc(func(nodeID string) float64 {
		return 100.0
	})

	nodeID := "high-rep-node"
	
	// 先进行一次操作以初始化状态
	limiter.AllowAndConsume(nodeID)
	
	status := limiter.GetStatus(nodeID)
	
	// 高声誉应该获得更高配额（基础10，100声誉约3倍=30）
	// 由于已消费1次，剩余应该接近29
	if status.Remaining < 15 {
		t.Errorf("High reputation node should have higher quota, got %d remaining", status.Remaining)
	}
}

func TestRateLimiter_ViolationBan(t *testing.T) {
	limiter := NewRateLimiter("test", &RateLimitConfig{
		MaxPerSecond:  1,
		MaxPerMinute:  5,
		MaxPerHour:    100,
		MaxPerDay:     1000,
		MinReputation: 0,
		BanDuration:   time.Hour,
	})

	nodeID := "bad-node"

	// 触发多次违规
	for i := 0; i < 10; i++ {
		limiter.AllowAndConsume(nodeID)
	}

	status := limiter.GetStatus(nodeID)
	if !status.Banned {
		t.Error("Node should be banned after multiple violations")
	}
}

func TestBehaviorAnalyzer_RecordEvent(t *testing.T) {
	analyzer := NewBehaviorAnalyzer(nil)

	nodeID := "test-node"

	// 记录一些事件
	for i := 0; i < 10; i++ {
		analyzer.RecordEvent(BehaviorEvent{
			NodeID: nodeID,
			Type:   BehaviorPublish,
			Target: "general",
		})
	}

	behavior := analyzer.GetNodeBehavior(nodeID)
	if behavior == nil {
		t.Fatal("Node behavior should exist")
	}

	if behavior.TotalActions != 10 {
		t.Errorf("Expected 10 actions, got %d", behavior.TotalActions)
	}
}

func TestBehaviorAnalyzer_BurstDetection(t *testing.T) {
	config := &BehaviorAnalyzerConfig{
		SpamBurstThreshold:    10,
		SpamBurstWindow:       time.Minute,
		AnomalyScoreThreshold: 0.5,
		MaxHistoryDuration:    time.Hour,
		MaxEventsPerNode:      1000,
	}
	analyzer := NewBehaviorAnalyzer(config)

	nodeID := "spam-node"
	detected := false

	analyzer.OnSuspiciousBehavior = func(id string, reason string, score float64) {
		if id == nodeID {
			detected = true
		}
	}

	// 快速发送大量消息
	for i := 0; i < 50; i++ {
		analyzer.RecordEvent(BehaviorEvent{
			NodeID:    nodeID,
			Type:      BehaviorPublish,
			Target:    "spam-topic",
			Timestamp: time.Now(),
		})
	}

	if !detected {
		t.Error("Burst behavior should be detected")
	}
}

func TestSecurityManager_Integration(t *testing.T) {
	sm := NewSecurityManager()

	// 设置声誉函数
	sm.SetReputationFunc(func(nodeID string) float64 {
		return 50.0 // 正常声誉
	})

	nodeID := "test-node"

	// 正常操作应该允许
	if err := sm.CheckBulletinPublish(nodeID); err != nil {
		t.Errorf("Normal operation should be allowed: %v", err)
	}
	sm.ConsumeBulletinQuota(nodeID, "general")

	// 获取状态
	status := sm.GetBulletinStatus(nodeID)
	if status == nil {
		t.Error("Status should not be nil")
	}

	// 检查黑名单功能
	sm.Blacklist(nodeID, time.Hour)
	if !sm.IsBlacklisted(nodeID) {
		t.Error("Node should be blacklisted")
	}

	// 黑名单节点不能操作
	if err := sm.CheckBulletinPublish(nodeID); err == nil {
		t.Error("Blacklisted node should not be allowed")
	}

	// 解除黑名单
	sm.Unblacklist(nodeID)
	if sm.IsBlacklisted(nodeID) {
		t.Error("Node should not be blacklisted")
	}
}

func TestSecurityManager_Report(t *testing.T) {
	sm := NewSecurityManager()

	// 生成报告
	report := sm.GenerateSecurityReport()
	if report == nil {
		t.Error("Report should not be nil")
	}

	if report.Timestamp.IsZero() {
		t.Error("Report timestamp should be set")
	}
}
