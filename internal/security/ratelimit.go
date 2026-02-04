// Package security 提供安全相关的功能
// 包括速率限制、行为分析、异常检测等
package security

import (
	"errors"
	"sync"
	"time"
)

// 错误定义
var (
	ErrRateLimitExceeded    = errors.New("rate limit exceeded")
	ErrReputationTooLow     = errors.New("reputation too low for this operation")
	ErrBlacklisted          = errors.New("node is blacklisted")
	ErrSuspiciousBehavior   = errors.New("suspicious behavior detected")
	ErrNodeNotRegistered    = errors.New("node not registered")
)

// RateLimitConfig 速率限制配置
type RateLimitConfig struct {
	// 每秒最大请求数
	MaxPerSecond int
	// 每分钟最大请求数
	MaxPerMinute int
	// 每小时最大请求数
	MaxPerHour int
	// 每天最大请求数
	MaxPerDay int
	// 声誉倍率（高声誉节点可获得更高配额）
	ReputationMultiplier float64
	// 最低声誉阈值（低于此值不能操作）
	MinReputation float64
	// 封禁持续时间
	BanDuration time.Duration
}

// DefaultRateLimitConfig 返回默认配置
func DefaultRateLimitConfig() *RateLimitConfig {
	return &RateLimitConfig{
		MaxPerSecond:         5,
		MaxPerMinute:         30,
		MaxPerHour:           200,
		MaxPerDay:            1000,
		ReputationMultiplier: 1.5,  // 每50点声誉增加1.5倍配额
		MinReputation:        10.0, // 最低声誉阈值
		BanDuration:          1 * time.Hour,
	}
}

// BulletinRateLimitConfig 留言板限流配置（更严格）
func BulletinRateLimitConfig() *RateLimitConfig {
	return &RateLimitConfig{
		MaxPerSecond:         2,
		MaxPerMinute:         20,
		MaxPerHour:           100,
		MaxPerDay:            500,
		ReputationMultiplier: 1.5,
		MinReputation:        10.0,
		BanDuration:          2 * time.Hour,
	}
}

// MailboxRateLimitConfig 邮箱限流配置
func MailboxRateLimitConfig() *RateLimitConfig {
	return &RateLimitConfig{
		MaxPerSecond:         3,
		MaxPerMinute:         30,
		MaxPerHour:           150,
		MaxPerDay:            800,
		ReputationMultiplier: 1.5,
		MinReputation:        5.0,
		BanDuration:          1 * time.Hour,
	}
}

// nodeRateState 节点的速率状态
type nodeRateState struct {
	// 时间窗口计数
	secondCount  int
	minuteCount  int
	hourCount    int
	dayCount     int

	// 时间窗口重置时间
	secondReset  time.Time
	minuteReset  time.Time
	hourReset    time.Time
	dayReset     time.Time

	// 封禁状态
	banned       bool
	banExpiry    time.Time

	// 连续违规计数
	violations   int
	lastViolation time.Time
}

// RateLimiter 速率限制器
type RateLimiter struct {
	mu      sync.RWMutex
	config  *RateLimitConfig
	states  map[string]*nodeRateState
	name    string // 限流器名称（用于日志）

	// 声誉查询函数
	getReputation func(nodeID string) float64
}

// NewRateLimiter 创建速率限制器
func NewRateLimiter(name string, config *RateLimitConfig) *RateLimiter {
	if config == nil {
		config = DefaultRateLimitConfig()
	}
	return &RateLimiter{
		config: config,
		states: make(map[string]*nodeRateState),
		name:   name,
	}
}

// SetReputationFunc 设置声誉查询函数
func (rl *RateLimiter) SetReputationFunc(fn func(nodeID string) float64) {
	rl.mu.Lock()
	defer rl.mu.Unlock()
	rl.getReputation = fn
}

// getEffectiveLimit 根据声誉计算有效限额
func (rl *RateLimiter) getEffectiveLimit(baseLimit int, nodeID string) int {
	if rl.getReputation == nil {
		return baseLimit
	}
	
	rep := rl.getReputation(nodeID)
	if rep <= 0 {
		return baseLimit / 2 // 低声誉减半
	}
	
	// 每50点声誉增加倍率
	multiplier := 1.0 + (rep/50.0)*(rl.config.ReputationMultiplier-1.0)
	if multiplier < 0.5 {
		multiplier = 0.5
	}
	if multiplier > 5.0 {
		multiplier = 5.0 // 最高5倍
	}
	
	return int(float64(baseLimit) * multiplier)
}

// Allow 检查是否允许操作
func (rl *RateLimiter) Allow(nodeID string) error {
	rl.mu.Lock()
	defer rl.mu.Unlock()

	now := time.Now()

	// 检查声誉阈值
	if rl.getReputation != nil {
		rep := rl.getReputation(nodeID)
		if rep < rl.config.MinReputation {
			return ErrReputationTooLow
		}
	}

	// 获取或创建状态
	state, exists := rl.states[nodeID]
	if !exists {
		state = &nodeRateState{
			secondReset: now,
			minuteReset: now,
			hourReset:   now,
			dayReset:    now,
		}
		rl.states[nodeID] = state
	}

	// 检查封禁状态
	if state.banned {
		if now.Before(state.banExpiry) {
			return ErrBlacklisted
		}
		// 解除封禁
		state.banned = false
		state.violations = 0
	}

	// 重置过期的时间窗口
	rl.resetExpiredWindows(state, now)

	// 检查各时间窗口限制
	if err := rl.checkLimits(state, nodeID); err != nil {
		rl.recordViolation(state, now)
		return err
	}

	return nil
}

// Consume 消费配额（操作成功后调用）
func (rl *RateLimiter) Consume(nodeID string) {
	rl.mu.Lock()
	defer rl.mu.Unlock()

	state, exists := rl.states[nodeID]
	if !exists {
		return
	}

	state.secondCount++
	state.minuteCount++
	state.hourCount++
	state.dayCount++
}

// AllowAndConsume 检查并消费配额（原子操作）
func (rl *RateLimiter) AllowAndConsume(nodeID string) error {
	if err := rl.Allow(nodeID); err != nil {
		return err
	}
	rl.Consume(nodeID)
	return nil
}

// resetExpiredWindows 重置过期的时间窗口
func (rl *RateLimiter) resetExpiredWindows(state *nodeRateState, now time.Time) {
	if now.Sub(state.secondReset) >= time.Second {
		state.secondCount = 0
		state.secondReset = now
	}
	if now.Sub(state.minuteReset) >= time.Minute {
		state.minuteCount = 0
		state.minuteReset = now
	}
	if now.Sub(state.hourReset) >= time.Hour {
		state.hourCount = 0
		state.hourReset = now
	}
	if now.Sub(state.dayReset) >= 24*time.Hour {
		state.dayCount = 0
		state.dayReset = now
	}
}

// checkLimits 检查限制
func (rl *RateLimiter) checkLimits(state *nodeRateState, nodeID string) error {
	maxSecond := rl.getEffectiveLimit(rl.config.MaxPerSecond, nodeID)
	maxMinute := rl.getEffectiveLimit(rl.config.MaxPerMinute, nodeID)
	maxHour := rl.getEffectiveLimit(rl.config.MaxPerHour, nodeID)
	maxDay := rl.getEffectiveLimit(rl.config.MaxPerDay, nodeID)

	if state.secondCount >= maxSecond {
		return ErrRateLimitExceeded
	}
	if state.minuteCount >= maxMinute {
		return ErrRateLimitExceeded
	}
	if state.hourCount >= maxHour {
		return ErrRateLimitExceeded
	}
	if state.dayCount >= maxDay {
		return ErrRateLimitExceeded
	}

	return nil
}

// recordViolation 记录违规
func (rl *RateLimiter) recordViolation(state *nodeRateState, now time.Time) {
	// 如果上次违规超过1小时，重置计数
	if now.Sub(state.lastViolation) > time.Hour {
		state.violations = 0
	}

	state.violations++
	state.lastViolation = now

	// 多次违规后封禁
	if state.violations >= 5 {
		state.banned = true
		state.banExpiry = now.Add(rl.config.BanDuration)
	}
}

// GetStatus 获取节点限流状态
func (rl *RateLimiter) GetStatus(nodeID string) *RateLimitStatus {
	rl.mu.RLock()
	defer rl.mu.RUnlock()

	state, exists := rl.states[nodeID]
	if !exists {
		return &RateLimitStatus{
			NodeID:      nodeID,
			CanOperate:  true,
			Remaining:   rl.config.MaxPerMinute,
			ResetIn:     "N/A",
		}
	}

	now := time.Now()
	maxMinute := rl.getEffectiveLimit(rl.config.MaxPerMinute, nodeID)
	remaining := maxMinute - state.minuteCount
	if remaining < 0 {
		remaining = 0
	}

	resetIn := time.Minute - now.Sub(state.minuteReset)
	if resetIn < 0 {
		resetIn = 0
	}

	canOperate := !state.banned && remaining > 0
	if rl.getReputation != nil {
		rep := rl.getReputation(nodeID)
		if rep < rl.config.MinReputation {
			canOperate = false
		}
	}

	return &RateLimitStatus{
		NodeID:       nodeID,
		CanOperate:   canOperate,
		Remaining:    remaining,
		ResetIn:      formatDuration(resetIn),
		Banned:       state.banned,
		BanExpiry:    state.banExpiry,
		Violations:   state.violations,
		DayRemaining: rl.getEffectiveLimit(rl.config.MaxPerDay, nodeID) - state.dayCount,
	}
}

// RateLimitStatus 限流状态
type RateLimitStatus struct {
	NodeID       string    `json:"node_id"`
	CanOperate   bool      `json:"can_operate"`
	Remaining    int       `json:"remaining"`     // 当前分钟剩余配额
	ResetIn      string    `json:"reset_in"`      // 重置剩余时间
	Banned       bool      `json:"banned"`
	BanExpiry    time.Time `json:"ban_expiry,omitempty"`
	Violations   int       `json:"violations"`
	DayRemaining int       `json:"day_remaining"` // 今日剩余配额
}

// ClearState 清除节点状态（用于测试或管理）
func (rl *RateLimiter) ClearState(nodeID string) {
	rl.mu.Lock()
	defer rl.mu.Unlock()
	delete(rl.states, nodeID)
}

// Unban 解除封禁
func (rl *RateLimiter) Unban(nodeID string) {
	rl.mu.Lock()
	defer rl.mu.Unlock()

	if state, exists := rl.states[nodeID]; exists {
		state.banned = false
		state.violations = 0
	}
}

// formatDuration 格式化时间间隔
func formatDuration(d time.Duration) string {
	if d < 0 {
		return "0s"
	}
	if d < time.Minute {
		return d.Round(time.Second).String()
	}
	return d.Round(time.Minute).String()
}
