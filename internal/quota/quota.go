// Package quota 实现消息配额与邮资系统
// 基于 Task 35 设计：防止滥发消息
package quota

import (
	"errors"
	"sync"
	"time"

	"github.com/AgentNetworkPlan/AgentNetwork/internal/reputation"
)

// 配额常量
const (
	// 基于声誉等级的每日配额
	QuotaBlacklist = 0     // 黑名单：不能发送
	QuotaProbation = 50    // 观察期
	QuotaNormal    = 200   // 正常节点
	QuotaActive    = 500   // 活跃节点
	QuotaTrusted   = 1000  // 信任节点
	QuotaElder     = 2000  // 元老节点

	// 速率限制
	MaxPerSecond = 10  // 每秒最多消息数
	MaxPerMinute = 100 // 每分钟最多消息数

	// 邮资（声誉消耗）
	PostageNormal     = 0.001 // 普通消息
	PostageBroadcast  = 0.01  // 广播消息
	PostageTask       = 0.1   // 任务相关
	PostageAccusation = 1.0   // 指责消息（高成本）

	// 配额恢复
	QuotaResetHour = 0 // 每天0点重置
)

var (
	ErrQuotaExceeded   = errors.New("daily quota exceeded")
	ErrRateLimitExceed = errors.New("rate limit exceeded")
	ErrInsufficientRep = errors.New("insufficient reputation for postage")
	ErrBlacklisted     = errors.New("node is blacklisted")
)

// MessageType 消息类型
type MessageType string

const (
	MessageTypeNormal     MessageType = "normal"
	MessageTypeBroadcast  MessageType = "broadcast"
	MessageTypeTask       MessageType = "task"
	MessageTypeAccusation MessageType = "accusation"
)

// GetPostage 获取消息类型对应的邮资
func GetPostage(msgType MessageType) float64 {
	switch msgType {
	case MessageTypeBroadcast:
		return PostageBroadcast
	case MessageTypeTask:
		return PostageTask
	case MessageTypeAccusation:
		return PostageAccusation
	default:
		return PostageNormal
	}
}

// NodeQuota 节点配额状态
type NodeQuota struct {
	NodeID          string    `json:"node_id"`
	Reputation      float64   `json:"reputation"`
	DailyLimit      int       `json:"daily_limit"`      // 每日配额
	UsedToday       int       `json:"used_today"`       // 今日已用
	RemainingToday  int       `json:"remaining_today"`  // 今日剩余
	LastReset       time.Time `json:"last_reset"`       // 上次重置时间

	// 速率限制
	RecentSecond    int       `json:"recent_second"`    // 最近1秒消息数
	RecentMinute    int       `json:"recent_minute"`    // 最近1分钟消息数
	LastSecondReset time.Time `json:"last_second_reset"`
	LastMinuteReset time.Time `json:"last_minute_reset"`

	// 累计邮资消耗
	TotalPostage    float64   `json:"total_postage"`    // 累计邮资消耗
}

// QuotaManager 配额管理器
type QuotaManager struct {
	mu     sync.RWMutex
	quotas map[string]*NodeQuota // nodeID -> quota

	// 回调函数：扣除声誉（邮资）
	deductReputation func(nodeID string, amount float64) error
}

// NewQuotaManager 创建配额管理器
func NewQuotaManager(deductRep func(nodeID string, amount float64) error) *QuotaManager {
	return &QuotaManager{
		quotas:           make(map[string]*NodeQuota),
		deductReputation: deductRep,
	}
}

// GetDailyQuota 根据声誉获取每日配额
func GetDailyQuota(rep float64) int {
	tier := reputation.GetTier(rep)
	switch tier {
	case reputation.TierLevelBlacklist:
		return QuotaBlacklist
	case reputation.TierLevelProbation:
		return QuotaProbation
	case reputation.TierLevelNormal:
		return QuotaNormal
	case reputation.TierLevelActive:
		return QuotaActive
	case reputation.TierLevelTrusted:
		return QuotaTrusted
	case reputation.TierLevelElder:
		return QuotaElder
	default:
		return QuotaNormal
	}
}

// InitQuota 初始化节点配额
func (qm *QuotaManager) InitQuota(nodeID string, rep float64) {
	qm.mu.Lock()
	defer qm.mu.Unlock()

	dailyLimit := GetDailyQuota(rep)
	now := time.Now()

	qm.quotas[nodeID] = &NodeQuota{
		NodeID:          nodeID,
		Reputation:      rep,
		DailyLimit:      dailyLimit,
		UsedToday:       0,
		RemainingToday:  dailyLimit,
		LastReset:       now,
		RecentSecond:    0,
		RecentMinute:    0,
		LastSecondReset: now,
		LastMinuteReset: now,
		TotalPostage:    0,
	}
}

// UpdateReputation 更新节点声誉（影响配额）
func (qm *QuotaManager) UpdateReputation(nodeID string, rep float64) {
	qm.mu.Lock()
	defer qm.mu.Unlock()

	quota, exists := qm.quotas[nodeID]
	if !exists {
		qm.quotas[nodeID] = &NodeQuota{
			NodeID:         nodeID,
			Reputation:     rep,
			DailyLimit:     GetDailyQuota(rep),
			RemainingToday: GetDailyQuota(rep),
			LastReset:      time.Now(),
		}
		return
	}

	quota.Reputation = rep
	quota.DailyLimit = GetDailyQuota(rep)
	// 不重置已用配额，只更新上限
}

// CanSend 检查是否可以发送消息
func (qm *QuotaManager) CanSend(nodeID string, msgType MessageType) error {
	qm.mu.RLock()
	defer qm.mu.RUnlock()

	quota, exists := qm.quotas[nodeID]
	if !exists {
		return errors.New("node not registered")
	}

	// 检查是否黑名单
	if quota.DailyLimit == 0 {
		return ErrBlacklisted
	}

	// 检查每日配额
	if quota.UsedToday >= quota.DailyLimit {
		return ErrQuotaExceeded
	}

	// 检查速率限制
	now := time.Now()
	
	// 每秒限制
	if now.Sub(quota.LastSecondReset) < time.Second {
		if quota.RecentSecond >= MaxPerSecond {
			return ErrRateLimitExceed
		}
	}

	// 每分钟限制
	if now.Sub(quota.LastMinuteReset) < time.Minute {
		if quota.RecentMinute >= MaxPerMinute {
			return ErrRateLimitExceed
		}
	}

	// 检查声誉是否足够支付邮资
	postage := GetPostage(msgType)
	if quota.Reputation < postage {
		return ErrInsufficientRep
	}

	return nil
}

// ConsumeQuota 消费配额（发送消息后调用）
func (qm *QuotaManager) ConsumeQuota(nodeID string, msgType MessageType) error {
	qm.mu.Lock()
	defer qm.mu.Unlock()

	quota, exists := qm.quotas[nodeID]
	if !exists {
		return errors.New("node not registered")
	}

	now := time.Now()

	// 检查是否需要重置
	qm.checkAndResetQuota(quota, now)

	// 消费每日配额
	quota.UsedToday++
	quota.RemainingToday = quota.DailyLimit - quota.UsedToday

	// 更新速率计数
	if now.Sub(quota.LastSecondReset) >= time.Second {
		quota.RecentSecond = 1
		quota.LastSecondReset = now
	} else {
		quota.RecentSecond++
	}

	if now.Sub(quota.LastMinuteReset) >= time.Minute {
		quota.RecentMinute = 1
		quota.LastMinuteReset = now
	} else {
		quota.RecentMinute++
	}

	// 扣除邮资（声誉）
	postage := GetPostage(msgType)
	quota.TotalPostage += postage
	
	if qm.deductReputation != nil {
		if err := qm.deductReputation(nodeID, postage); err != nil {
			// 邮资扣除失败，但消息已发送，记录日志即可
			// 实际实现中可能需要回滚
		}
	}

	return nil
}

// checkAndResetQuota 检查并重置每日配额
func (qm *QuotaManager) checkAndResetQuota(quota *NodeQuota, now time.Time) {
	// 如果跨天了，重置配额
	if !sameDay(quota.LastReset, now) {
		quota.UsedToday = 0
		quota.RemainingToday = quota.DailyLimit
		quota.LastReset = now
	}
}

// sameDay 判断两个时间是否同一天
func sameDay(t1, t2 time.Time) bool {
	y1, m1, d1 := t1.Date()
	y2, m2, d2 := t2.Date()
	return y1 == y2 && m1 == m2 && d1 == d2
}

// GetQuota 获取节点配额状态
func (qm *QuotaManager) GetQuota(nodeID string) (*NodeQuota, error) {
	qm.mu.RLock()
	defer qm.mu.RUnlock()

	quota, exists := qm.quotas[nodeID]
	if !exists {
		return nil, errors.New("node not registered")
	}

	// 检查是否需要重置
	now := time.Now()
	if !sameDay(quota.LastReset, now) {
		// 返回重置后的状态（不修改原数据，只影响返回值）
		return &NodeQuota{
			NodeID:         quota.NodeID,
			Reputation:     quota.Reputation,
			DailyLimit:     quota.DailyLimit,
			UsedToday:      0,
			RemainingToday: quota.DailyLimit,
			LastReset:      now,
			TotalPostage:   quota.TotalPostage,
		}, nil
	}

	return quota, nil
}

// GetAllQuotas 获取所有节点配额状态
func (qm *QuotaManager) GetAllQuotas() map[string]*NodeQuota {
	qm.mu.RLock()
	defer qm.mu.RUnlock()

	result := make(map[string]*NodeQuota)
	for k, v := range qm.quotas {
		result[k] = v
	}
	return result
}

// QuotaInfo 配额信息（Agent可读）
type QuotaInfo struct {
	NodeID         string  `json:"node_id"`
	DailyLimit     int     `json:"daily_limit"`
	UsedToday      int     `json:"used_today"`
	RemainingToday int     `json:"remaining_today"`
	Percentage     float64 `json:"percentage"`       // 已用百分比
	CanSend        bool    `json:"can_send"`
	NextResetIn    string  `json:"next_reset_in"`    // 距离下次重置
	TotalPostage   float64 `json:"total_postage"`    // 累计邮资
	Explanation    string  `json:"explanation"`
}

// GetQuotaInfo 获取配额详情（Agent可读）
func (qm *QuotaManager) GetQuotaInfo(nodeID string) (*QuotaInfo, error) {
	quota, err := qm.GetQuota(nodeID)
	if err != nil {
		return nil, err
	}

	// 计算下次重置时间
	now := time.Now()
	nextReset := time.Date(now.Year(), now.Month(), now.Day()+1, QuotaResetHour, 0, 0, 0, now.Location())
	nextResetIn := nextReset.Sub(now)

	canSend := qm.CanSend(nodeID, MessageTypeNormal) == nil

	percentage := 0.0
	if quota.DailyLimit > 0 {
		percentage = float64(quota.UsedToday) / float64(quota.DailyLimit) * 100
	}

	explanation := ""
	if !canSend {
		if quota.DailyLimit == 0 {
			explanation = "该节点处于黑名单状态，无法发送消息"
		} else if quota.UsedToday >= quota.DailyLimit {
			explanation = "今日配额已用完，请等待明日重置"
		} else {
			explanation = "当前发送速率过快，请稍后重试"
		}
	} else {
		explanation = "可正常发送消息"
	}

	return &QuotaInfo{
		NodeID:         nodeID,
		DailyLimit:     quota.DailyLimit,
		UsedToday:      quota.UsedToday,
		RemainingToday: quota.RemainingToday,
		Percentage:     percentage,
		CanSend:        canSend,
		NextResetIn:    formatDuration(nextResetIn),
		TotalPostage:   quota.TotalPostage,
		Explanation:    explanation,
	}, nil
}

// formatDuration 格式化时间间隔
func formatDuration(d time.Duration) string {
	hours := int(d.Hours())
	minutes := int(d.Minutes()) % 60
	return (time.Duration(hours)*time.Hour + time.Duration(minutes)*time.Minute).String()
}
