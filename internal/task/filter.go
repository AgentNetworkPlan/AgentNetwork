// Package task 提供任务过滤和限速功能
package task

import (
	"strings"
	"sync"
	"time"
)

// TaskFilter 任务过滤器
type TaskFilter struct {
	// 黑名单关键词
	BlacklistKeywords []string

	// 任务类型白名单
	AllowedTaskTypes []TaskType

	// 资源限制
	MaxComputeTime  time.Duration // 最大计算时间
	MaxMemoryUsage  int64         // 最大内存使用（字节）
	MaxNetworkCalls int           // 最大网络调用数

	// 沙箱执行
	SandboxEnabled bool
}

// NewTaskFilter 创建默认过滤器
func NewTaskFilter() *TaskFilter {
	return &TaskFilter{
		BlacklistKeywords: []string{
			"attack", "ddos", "hack", "malware", "virus",
			"phishing", "spam", "illegal", "攻击", "黑客",
		},
		AllowedTaskTypes: []TaskType{
			TaskTypeSearch,
			TaskTypeTransfer,
			TaskTypeStorage,
			TaskTypeCompute,
			TaskTypeCustom,
		},
		MaxComputeTime:  30 * time.Minute,
		MaxMemoryUsage:  1024 * 1024 * 1024, // 1GB
		MaxNetworkCalls: 100,
		SandboxEnabled:  true,
	}
}

// AssessRisk 评估任务风险
func (f *TaskFilter) AssessRisk(task *Task) TaskRiskLevel {
	// 1. 关键词检查
	if f.containsBlacklist(task.Title) || f.containsBlacklist(task.Description) {
		return RiskBlocked
	}

	// 2. 类型检查
	if !f.isTypeAllowed(task.Type) {
		return RiskBlocked
	}

	// 3. 基于类型的风险评估
	switch task.Type {
	case TaskTypeSearch:
		return RiskLow
	case TaskTypeTransfer, TaskTypeStorage:
		return RiskMedium
	case TaskTypeCompute:
		return RiskHigh
	default:
		return RiskMedium
	}
}

// IsAllowed 检查任务是否允许
func (f *TaskFilter) IsAllowed(task *Task) (bool, string) {
	risk := f.AssessRisk(task)
	if risk == RiskBlocked {
		return false, "task contains blocked content or type"
	}
	return true, ""
}

func (f *TaskFilter) containsBlacklist(text string) bool {
	lower := strings.ToLower(text)
	for _, keyword := range f.BlacklistKeywords {
		if strings.Contains(lower, strings.ToLower(keyword)) {
			return true
		}
	}
	return false
}

func (f *TaskFilter) isTypeAllowed(t TaskType) bool {
	for _, allowed := range f.AllowedTaskTypes {
		if t == allowed {
			return true
		}
	}
	return false
}

// AddBlacklistKeyword 添加黑名单关键词
func (f *TaskFilter) AddBlacklistKeyword(keyword string) {
	f.BlacklistKeywords = append(f.BlacklistKeywords, keyword)
}

// RemoveBlacklistKeyword 移除黑名单关键词
func (f *TaskFilter) RemoveBlacklistKeyword(keyword string) {
	for i, k := range f.BlacklistKeywords {
		if k == keyword {
			f.BlacklistKeywords = append(f.BlacklistKeywords[:i], f.BlacklistKeywords[i+1:]...)
			return
		}
	}
}

// TaskRateLimiter 任务速率限制器
type TaskRateLimiter struct {
	mu sync.RWMutex

	// 基于声誉的配额
	BaseQuota            int     // 基础配额（如：每小时2个任务）
	ReputationMultiplier float64 // 声誉乘数（如：声誉/20）

	// 冷却机制
	CooldownAfterReject time.Duration // 被拒绝后冷却期

	// 当前状态
	quotaUsed   map[string]int       // nodeID -> 已用配额
	lastReset   map[string]time.Time // nodeID -> 上次重置时间
	cooldownEnd map[string]time.Time // nodeID -> 冷却结束时间
}

// NewTaskRateLimiter 创建速率限制器
func NewTaskRateLimiter() *TaskRateLimiter {
	return &TaskRateLimiter{
		BaseQuota:            2,
		ReputationMultiplier: 0.05, // 每点声誉增加5%配额
		CooldownAfterReject:  30 * time.Minute,
		quotaUsed:            make(map[string]int),
		lastReset:            make(map[string]time.Time),
		cooldownEnd:          make(map[string]time.Time),
	}
}

// GetQuota 计算节点配额
func (r *TaskRateLimiter) GetQuota(nodeID string, reputation float64) int {
	base := r.BaseQuota
	bonus := int(reputation * r.ReputationMultiplier)
	return base + bonus
	// 例：声誉=50 → 2 + 50*0.05 = 2 + 2.5 → 4个任务/小时
}

// CanPublish 检查是否可以发布任务
func (r *TaskRateLimiter) CanPublish(nodeID string, reputation float64) (bool, string) {
	r.mu.Lock()
	defer r.mu.Unlock()

	// 检查冷却期
	if cooldown, exists := r.cooldownEnd[nodeID]; exists {
		if time.Now().Before(cooldown) {
			remaining := time.Until(cooldown)
			return false, "in cooldown period, " + remaining.Round(time.Second).String() + " remaining"
		}
	}

	// 检查是否需要重置配额
	if lastReset, exists := r.lastReset[nodeID]; exists {
		if time.Since(lastReset) > time.Hour {
			r.quotaUsed[nodeID] = 0
			r.lastReset[nodeID] = time.Now()
		}
	} else {
		r.lastReset[nodeID] = time.Now()
	}

	// 检查配额
	used := r.quotaUsed[nodeID]
	quota := r.GetQuota(nodeID, reputation)

	if used >= quota {
		return false, "quota exceeded"
	}

	return true, ""
}

// ConsumeQuota 消耗配额
func (r *TaskRateLimiter) ConsumeQuota(nodeID string) {
	r.mu.Lock()
	defer r.mu.Unlock()

	r.quotaUsed[nodeID]++
}

// ApplyCooldown 应用冷却期（当任务被拒绝时）
func (r *TaskRateLimiter) ApplyCooldown(nodeID string) {
	r.mu.Lock()
	defer r.mu.Unlock()

	r.cooldownEnd[nodeID] = time.Now().Add(r.CooldownAfterReject)
}

// GetRemainingQuota 获取剩余配额
func (r *TaskRateLimiter) GetRemainingQuota(nodeID string, reputation float64) int {
	r.mu.RLock()
	defer r.mu.RUnlock()

	quota := r.GetQuota(nodeID, reputation)
	used := r.quotaUsed[nodeID]

	remaining := quota - used
	if remaining < 0 {
		return 0
	}
	return remaining
}

// Reset 重置节点配额
func (r *TaskRateLimiter) Reset(nodeID string) {
	r.mu.Lock()
	defer r.mu.Unlock()

	delete(r.quotaUsed, nodeID)
	delete(r.lastReset, nodeID)
	delete(r.cooldownEnd, nodeID)
}

// 消息大小限制常量
const (
	MaxTaskTitleSize       = 200             // 标题最大200字符
	MaxTaskDescriptionSize = 10 * 1024       // 描述最大10KB
	MaxAttachmentSize      = 1 * 1024 * 1024 // 附件最大1MB
	MaxBidsPerTask         = 100             // 每个任务最多100个竞标
)

// ValidateTaskSize 验证任务大小限制
func ValidateTaskSize(task *Task) (bool, string) {
	if len(task.Title) > MaxTaskTitleSize {
		return false, "title too long"
	}
	if len(task.Description) > MaxTaskDescriptionSize {
		return false, "description too long"
	}
	if len(task.Bids) > MaxBidsPerTask {
		return false, "too many bids"
	}
	return true, ""
}
