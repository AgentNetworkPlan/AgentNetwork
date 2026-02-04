package reputation

import (
	"math"
)

// 声誉边界常量 - 基于 Task 36 设计
const (
	// 声誉范围 [0, 1000]
	ReputationMin     = 0.0
	ReputationMax     = 1000.0
	ReputationInitial = 10.0 // 新节点初始声誉

	// 声誉等级阈值
	TierBlacklist  = 10.0   // 0-10: 黑名单
	TierProbation  = 50.0   // 10-50: 观察期
	TierNormal     = 200.0  // 50-200: 正常节点
	TierActive     = 500.0  // 200-500: 活跃节点
	TierTrusted    = 800.0  // 500-800: 信任节点
	TierElder      = 1000.0 // 800-1000: 元老节点

	// 自然衰减参数
	DecayGraceDays   = 7    // 宽限期（天）
	DecayRatePerWeek = 0.01 // 每周衰减 1%
	DecayFloor       = 50.0 // 衰减底线（保护老用户）
	MaxDecayRate     = 0.10 // 单次最大衰减 10%
)

// ReputationTier 声誉等级
type ReputationTier int

const (
	TierLevelBlacklist ReputationTier = iota // 黑名单
	TierLevelProbation                       // 观察期
	TierLevelNormal                          // 正常
	TierLevelActive                          // 活跃
	TierLevelTrusted                         // 信任
	TierLevelElder                           // 元老
)

// TierName 返回等级名称
func (t ReputationTier) String() string {
	names := []string{"黑名单", "观察期", "正常节点", "活跃节点", "信任节点", "元老节点"}
	if int(t) < len(names) {
		return names[t]
	}
	return "未知"
}

// GetTier 根据声誉值获取等级
func GetTier(reputation float64) ReputationTier {
	switch {
	case reputation < TierBlacklist:
		return TierLevelBlacklist
	case reputation < TierProbation:
		return TierLevelProbation
	case reputation < TierNormal:
		return TierLevelNormal
	case reputation < TierActive:
		return TierLevelActive
	case reputation < TierTrusted:
		return TierLevelTrusted
	default:
		return TierLevelElder
	}
}

// BoundedChange 带边际效应的声誉变化
type BoundedChange struct {
	BaseAmount float64 // 基础变化量
	CurrentRep float64 // 当前声誉
}

// CalculateGain 计算声誉获取（边际递减）
// 公式：实际获取 = 基础量 × (1 - 当前声誉/上限)^0.5
// 越接近上限，获取越难
func (bc *BoundedChange) CalculateGain() float64 {
	if bc.BaseAmount <= 0 {
		return 0
	}

	// 边际递减因子
	diminishingFactor := math.Pow(1-bc.CurrentRep/ReputationMax, 0.5)
	gain := bc.BaseAmount * diminishingFactor

	// 确保不超过上限
	newRep := bc.CurrentRep + gain
	if newRep > ReputationMax {
		return ReputationMax - bc.CurrentRep
	}

	return gain
}

// CalculateLoss 计算声誉损失（高位高责）
// 公式：实际损失 = 基础量 × (当前声誉/上限)^0.3
// 高声誉者损失更大
func (bc *BoundedChange) CalculateLoss() float64 {
	if bc.BaseAmount <= 0 {
		return 0
	}

	// 高位高责因子
	scaleFactor := math.Pow(bc.CurrentRep/ReputationMax, 0.3)
	loss := bc.BaseAmount * scaleFactor

	// 确保不低于下限
	newRep := bc.CurrentRep - loss
	if newRep < ReputationMin {
		return bc.CurrentRep - ReputationMin
	}

	return loss
}

// ApplyGain 应用声誉获取
func ApplyGain(currentRep, baseAmount float64) float64 {
	bc := &BoundedChange{BaseAmount: baseAmount, CurrentRep: currentRep}
	gain := bc.CalculateGain()
	return ClipReputation(currentRep + gain)
}

// ApplyLoss 应用声誉损失
func ApplyLoss(currentRep, baseAmount float64) float64 {
	bc := &BoundedChange{BaseAmount: baseAmount, CurrentRep: currentRep}
	loss := bc.CalculateLoss()
	return ClipReputation(currentRep - loss)
}

// ClipReputation 将声誉值限制在 [0, 1000] 范围内
func ClipReputation(value float64) float64 {
	return math.Max(ReputationMin, math.Min(ReputationMax, value))
}

// CalculateNaturalDecay 计算自然衰减
// 超过宽限期后，每周衰减 1%，但不会低于 DecayFloor
func CalculateNaturalDecay(currentRep float64, daysSinceLastActivity int) float64 {
	if daysSinceLastActivity <= DecayGraceDays {
		return currentRep // 宽限期内不衰减
	}

	// 计算衰减率
	weeksInactive := float64(daysSinceLastActivity-DecayGraceDays) / 7.0
	decayRate := DecayRatePerWeek * weeksInactive

	// 限制单次最大衰减
	if decayRate > MaxDecayRate {
		decayRate = MaxDecayRate
	}

	newRep := currentRep * (1 - decayRate)

	// 衰减不会低于底线（如果当前声誉高于底线）
	if newRep < DecayFloor && currentRep >= DecayFloor {
		return DecayFloor
	}

	return ClipReputation(newRep)
}

// ReputationInfo 声誉详情（Agent可读）
type ReputationInfo struct {
	Current      float64        `json:"current"`       // 当前声誉
	Tier         ReputationTier `json:"tier"`          // 等级
	TierName     string         `json:"tier_name"`     // 等级名称
	Percentile   float64        `json:"percentile"`    // 全网排名百分位
	Trend        string         `json:"trend"`         // 趋势：上升/下降/稳定
	HighestEver  float64        `json:"highest_ever"`  // 历史最高
	LowestEver   float64        `json:"lowest_ever"`   // 历史最低
	Violations   int            `json:"violations"`    // 归零次数
	CanEndorse   bool           `json:"can_endorse"`   // 是否可担保他人
	MaxEndorsees int            `json:"max_endorsees"` // 最多可担保人数
}

// GetReputationInfo 获取声誉详情
func GetReputationInfo(currentRep, highestRep, lowestRep float64, violations int, trend string) *ReputationInfo {
	tier := GetTier(currentRep)

	// 计算担保能力 - 基于等级而非具体阈值
	canEndorse := tier >= TierLevelNormal
	maxEndorsees := 0
	switch tier {
	case TierLevelElder:
		maxEndorsees = 10
	case TierLevelTrusted:
		maxEndorsees = 5
	case TierLevelActive:
		maxEndorsees = 3
	case TierLevelNormal:
		maxEndorsees = 1
	}

	return &ReputationInfo{
		Current:      currentRep,
		Tier:         tier,
		TierName:     tier.String(),
		Trend:        trend,
		HighestEver:  highestRep,
		LowestEver:   lowestRep,
		Violations:   violations,
		CanEndorse:   canEndorse,
		MaxEndorsees: maxEndorsees,
	}
}

// TierPermissions 返回该等级的权限说明
func TierPermissions(tier ReputationTier) map[string]interface{} {
	permissions := map[string]interface{}{
		"can_send_messages": true,
		"can_receive":       true,
		"daily_quota":       100,
		"can_endorse":       false,
		"max_endorsees":     0,
		"can_join_committee": false,
		"can_nominate_super": false,
	}

	switch tier {
	case TierLevelBlacklist:
		permissions["can_send_messages"] = false
		permissions["daily_quota"] = 0
	case TierLevelProbation:
		permissions["daily_quota"] = 50
	case TierLevelNormal:
		permissions["daily_quota"] = 200
	case TierLevelActive:
		permissions["daily_quota"] = 500
		permissions["can_endorse"] = true
		permissions["max_endorsees"] = 1
	case TierLevelTrusted:
		permissions["daily_quota"] = 1000
		permissions["can_endorse"] = true
		permissions["max_endorsees"] = 3
		permissions["can_join_committee"] = true
	case TierLevelElder:
		permissions["daily_quota"] = 2000
		permissions["can_endorse"] = true
		permissions["max_endorsees"] = 5
		permissions["can_join_committee"] = true
		permissions["can_nominate_super"] = true
	}

	return permissions
}
