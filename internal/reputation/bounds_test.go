package reputation

import (
	"math"
	"testing"
)

func TestGetTier(t *testing.T) {
	tests := []struct {
		reputation float64
		expected   ReputationTier
	}{
		{0, TierLevelBlacklist},
		{5, TierLevelBlacklist},
		{10, TierLevelProbation},
		{49, TierLevelProbation},
		{50, TierLevelNormal},
		{199, TierLevelNormal},
		{200, TierLevelActive},
		{499, TierLevelActive},
		{500, TierLevelTrusted},
		{799, TierLevelTrusted},
		{800, TierLevelElder},
		{1000, TierLevelElder},
	}

	for _, tt := range tests {
		got := GetTier(tt.reputation)
		if got != tt.expected {
			t.Errorf("GetTier(%v) = %v, want %v", tt.reputation, got, tt.expected)
		}
	}
}

func TestBoundedChangeGain(t *testing.T) {
	tests := []struct {
		name       string
		baseAmount float64
		currentRep float64
		wantMin    float64
		wantMax    float64
	}{
		{
			name:       "新人获取声誉（接近完整）",
			baseAmount: 10,
			currentRep: 10,
			wantMin:    9.5,
			wantMax:    10.0,
		},
		{
			name:       "元老获取声誉（大幅递减）",
			baseAmount: 10,
			currentRep: 900,
			wantMin:    2.5,
			wantMax:    4.0,
		},
		{
			name:       "中等声誉获取",
			baseAmount: 10,
			currentRep: 500,
			wantMin:    6.0,
			wantMax:    8.0,
		},
		{
			name:       "接近上限时获取",
			baseAmount: 100,
			currentRep: 990,
			wantMin:    0,
			wantMax:    11, // 只能获取到上限，允许浮点误差
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			bc := &BoundedChange{BaseAmount: tt.baseAmount, CurrentRep: tt.currentRep}
			got := bc.CalculateGain()
			if got < tt.wantMin || got > tt.wantMax {
				t.Errorf("CalculateGain() = %v, want between %v and %v", got, tt.wantMin, tt.wantMax)
			}
		})
	}
}

func TestBoundedChangeLoss(t *testing.T) {
	tests := []struct {
		name       string
		baseAmount float64
		currentRep float64
		wantMin    float64
		wantMax    float64
	}{
		{
			name:       "新人损失声誉（保护新人）",
			baseAmount: 50,
			currentRep: 50,
			wantMin:    10,
			wantMax:    25,
		},
		{
			name:       "元老损失声誉（高位高责）",
			baseAmount: 50,
			currentRep: 900,
			wantMin:    45,
			wantMax:    50,
		},
		{
			name:       "中等声誉损失",
			baseAmount: 50,
			currentRep: 500,
			wantMin:    35,
			wantMax:    45,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			bc := &BoundedChange{BaseAmount: tt.baseAmount, CurrentRep: tt.currentRep}
			got := bc.CalculateLoss()
			if got < tt.wantMin || got > tt.wantMax {
				t.Errorf("CalculateLoss() = %v, want between %v and %v", got, tt.wantMin, tt.wantMax)
			}
		})
	}
}

func TestApplyGainAndLoss(t *testing.T) {
	// 测试获取
	newRep := ApplyGain(10, 10)
	if newRep <= 10 || newRep > 20 {
		t.Errorf("ApplyGain(10, 10) = %v, expected between 10 and 20", newRep)
	}

	// 测试损失
	newRep = ApplyLoss(100, 20)
	if newRep >= 100 || newRep < 80 {
		t.Errorf("ApplyLoss(100, 20) = %v, expected between 80 and 100", newRep)
	}

	// 测试边界：不能超过上限
	newRep = ApplyGain(990, 100)
	if newRep > ReputationMax {
		t.Errorf("ApplyGain should not exceed max, got %v", newRep)
	}

	// 测试边界：不能低于下限
	newRep = ApplyLoss(5, 100)
	if newRep < ReputationMin {
		t.Errorf("ApplyLoss should not go below min, got %v", newRep)
	}
}

func TestClipReputation(t *testing.T) {
	tests := []struct {
		value    float64
		expected float64
	}{
		{-100, 0},
		{0, 0},
		{500, 500},
		{1000, 1000},
		{1500, 1000},
	}

	for _, tt := range tests {
		got := ClipReputation(tt.value)
		if got != tt.expected {
			t.Errorf("ClipReputation(%v) = %v, want %v", tt.value, got, tt.expected)
		}
	}
}

func TestCalculateNaturalDecay(t *testing.T) {
	tests := []struct {
		name       string
		currentRep float64
		daysAway   int
		wantMin    float64
		wantMax    float64
	}{
		{
			name:       "宽限期内不衰减",
			currentRep: 500,
			daysAway:   5,
			wantMin:    500,
			wantMax:    500,
		},
		{
			name:       "超过宽限期轻微衰减",
			currentRep: 500,
			daysAway:   14, // 超过宽限期1周
			wantMin:    490,
			wantMax:    500,
		},
		{
			name:       "长期不活跃衰减到底线",
			currentRep: 500,
			daysAway:   100,
			wantMin:    50,  // 不低于底线
			wantMax:    500,
		},
		{
			name:       "已低于底线继续衰减",
			currentRep: 30,
			daysAway:   30,
			wantMin:    25,
			wantMax:    30,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := CalculateNaturalDecay(tt.currentRep, tt.daysAway)
			if got < tt.wantMin || got > tt.wantMax {
				t.Errorf("CalculateNaturalDecay(%v, %v) = %v, want between %v and %v",
					tt.currentRep, tt.daysAway, got, tt.wantMin, tt.wantMax)
			}
		})
	}
}

func TestTierPermissions(t *testing.T) {
	// 黑名单不能发消息
	perms := TierPermissions(TierLevelBlacklist)
	if perms["can_send_messages"].(bool) {
		t.Error("Blacklist should not be able to send messages")
	}

	// 活跃节点可以担保
	perms = TierPermissions(TierLevelActive)
	if !perms["can_endorse"].(bool) {
		t.Error("Active nodes should be able to endorse")
	}
	if perms["max_endorsees"].(int) != 1 {
		t.Error("Active nodes should be able to endorse 1 person")
	}

	// 元老节点可以提名超级节点
	perms = TierPermissions(TierLevelElder)
	if !perms["can_nominate_super"].(bool) {
		t.Error("Elder nodes should be able to nominate super nodes")
	}
}

func TestReputationInfo(t *testing.T) {
	info := GetReputationInfo(350, 400, 50, 0, "上升")

	if info.Tier != TierLevelActive {
		t.Errorf("Expected tier Active, got %v", info.TierName)
	}
	if !info.CanEndorse {
		t.Error("Reputation 350 should be able to endorse")
	}
	if info.MaxEndorsees != 3 {
		t.Errorf("Expected max endorsees 3, got %d", info.MaxEndorsees)
	}
}

// 测试边际递减的数学性质
func TestDiminishingReturns(t *testing.T) {
	baseAmount := 10.0

	// 验证：声誉越高，获取越难
	var lastGain float64 = math.MaxFloat64
	for rep := 0.0; rep <= 900; rep += 100 {
		bc := &BoundedChange{BaseAmount: baseAmount, CurrentRep: rep}
		gain := bc.CalculateGain()
		if gain > lastGain {
			t.Errorf("Gain should decrease as reputation increases. At rep %v, gain %v > previous %v",
				rep, gain, lastGain)
		}
		lastGain = gain
	}

	// 验证：声誉越高，损失越大
	var lastLoss float64 = 0
	for rep := 100.0; rep <= 900; rep += 100 {
		bc := &BoundedChange{BaseAmount: baseAmount, CurrentRep: rep}
		loss := bc.CalculateLoss()
		if loss < lastLoss {
			t.Errorf("Loss should increase as reputation increases. At rep %v, loss %v < previous %v",
				rep, loss, lastLoss)
		}
		lastLoss = loss
	}
}
