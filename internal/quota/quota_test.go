package quota

import (
	"testing"
	"time"
)

func newTestQuotaManager() *QuotaManager {
	// 模拟声誉扣除函数
	deductRep := func(nodeID string, amount float64) error {
		return nil // 测试时总是成功
	}
	return NewQuotaManager(deductRep)
}

func TestGetDailyQuota(t *testing.T) {
	tests := []struct {
		reputation float64
		expected   int
	}{
		{5, QuotaBlacklist},      // 黑名单
		{30, QuotaProbation},     // 观察期
		{100, QuotaNormal},       // 正常
		{300, QuotaActive},       // 活跃
		{600, QuotaTrusted},      // 信任
		{900, QuotaElder},        // 元老
	}

	for _, tt := range tests {
		got := GetDailyQuota(tt.reputation)
		if got != tt.expected {
			t.Errorf("GetDailyQuota(%v) = %v, want %v", tt.reputation, got, tt.expected)
		}
	}
}

func TestGetPostage(t *testing.T) {
	tests := []struct {
		msgType  MessageType
		expected float64
	}{
		{MessageTypeNormal, PostageNormal},
		{MessageTypeBroadcast, PostageBroadcast},
		{MessageTypeTask, PostageTask},
		{MessageTypeAccusation, PostageAccusation},
	}

	for _, tt := range tests {
		got := GetPostage(tt.msgType)
		if got != tt.expected {
			t.Errorf("GetPostage(%v) = %v, want %v", tt.msgType, got, tt.expected)
		}
	}
}

func TestQuotaManager_InitQuota(t *testing.T) {
	qm := newTestQuotaManager()

	qm.InitQuota("node1", 300.0) // 活跃节点

	quota, err := qm.GetQuota("node1")
	if err != nil {
		t.Fatalf("GetQuota failed: %v", err)
	}

	if quota.DailyLimit != QuotaActive {
		t.Errorf("Expected quota %d, got %d", QuotaActive, quota.DailyLimit)
	}
	if quota.UsedToday != 0 {
		t.Errorf("Expected used 0, got %d", quota.UsedToday)
	}
	if quota.RemainingToday != QuotaActive {
		t.Errorf("Expected remaining %d, got %d", QuotaActive, quota.RemainingToday)
	}
}

func TestQuotaManager_CanSend_Normal(t *testing.T) {
	qm := newTestQuotaManager()
	qm.InitQuota("node1", 300.0)

	err := qm.CanSend("node1", MessageTypeNormal)
	if err != nil {
		t.Errorf("Expected can send, got error: %v", err)
	}
}

func TestQuotaManager_CanSend_Blacklisted(t *testing.T) {
	qm := newTestQuotaManager()
	qm.InitQuota("node1", 5.0) // 黑名单声誉

	err := qm.CanSend("node1", MessageTypeNormal)
	if err != ErrBlacklisted {
		t.Errorf("Expected ErrBlacklisted, got %v", err)
	}
}

func TestQuotaManager_ConsumeQuota(t *testing.T) {
	qm := newTestQuotaManager()
	qm.InitQuota("node1", 300.0)

	// 发送一条消息
	err := qm.ConsumeQuota("node1", MessageTypeNormal)
	if err != nil {
		t.Fatalf("ConsumeQuota failed: %v", err)
	}

	quota, _ := qm.GetQuota("node1")
	if quota.UsedToday != 1 {
		t.Errorf("Expected used 1, got %d", quota.UsedToday)
	}
	if quota.RemainingToday != QuotaActive-1 {
		t.Errorf("Expected remaining %d, got %d", QuotaActive-1, quota.RemainingToday)
	}
	if quota.TotalPostage != PostageNormal {
		t.Errorf("Expected postage %v, got %v", PostageNormal, quota.TotalPostage)
	}
}

func TestQuotaManager_QuotaExceeded(t *testing.T) {
	qm := newTestQuotaManager()
	qm.InitQuota("node1", 30.0) // 观察期，配额50

	// 用完配额
	for i := 0; i < QuotaProbation; i++ {
		err := qm.ConsumeQuota("node1", MessageTypeNormal)
		if err != nil {
			t.Fatalf("ConsumeQuota %d failed: %v", i, err)
		}
	}

	// 再发送应该失败
	err := qm.CanSend("node1", MessageTypeNormal)
	if err != ErrQuotaExceeded {
		t.Errorf("Expected ErrQuotaExceeded, got %v", err)
	}
}

func TestQuotaManager_RateLimit(t *testing.T) {
	qm := newTestQuotaManager()
	qm.InitQuota("node1", 900.0) // 元老，高配额

	// 快速发送超过每秒限制
	for i := 0; i < MaxPerSecond; i++ {
		qm.ConsumeQuota("node1", MessageTypeNormal)
	}

	// 再发送应该触发速率限制
	err := qm.CanSend("node1", MessageTypeNormal)
	if err != ErrRateLimitExceed {
		t.Errorf("Expected ErrRateLimitExceed, got %v", err)
	}

	// 等待1秒后应该可以发送
	time.Sleep(1100 * time.Millisecond)
	err = qm.CanSend("node1", MessageTypeNormal)
	if err != nil {
		t.Errorf("Expected can send after 1 second, got %v", err)
	}
}

func TestQuotaManager_UpdateReputation(t *testing.T) {
	qm := newTestQuotaManager()
	qm.InitQuota("node1", 100.0) // 正常节点

	quota, _ := qm.GetQuota("node1")
	if quota.DailyLimit != QuotaNormal {
		t.Errorf("Initial quota should be %d, got %d", QuotaNormal, quota.DailyLimit)
	}

	// 升级声誉
	qm.UpdateReputation("node1", 600.0) // 信任节点

	quota, _ = qm.GetQuota("node1")
	if quota.DailyLimit != QuotaTrusted {
		t.Errorf("Updated quota should be %d, got %d", QuotaTrusted, quota.DailyLimit)
	}
}

func TestQuotaManager_PostageAccumulation(t *testing.T) {
	qm := newTestQuotaManager()
	qm.InitQuota("node1", 300.0)

	// 发送不同类型的消息
	qm.ConsumeQuota("node1", MessageTypeNormal)     // 0.001
	qm.ConsumeQuota("node1", MessageTypeBroadcast)  // 0.01
	qm.ConsumeQuota("node1", MessageTypeTask)       // 0.1

	quota, _ := qm.GetQuota("node1")
	expectedPostage := PostageNormal + PostageBroadcast + PostageTask
	if quota.TotalPostage != expectedPostage {
		t.Errorf("Expected total postage %v, got %v", expectedPostage, quota.TotalPostage)
	}
}

func TestQuotaManager_GetQuotaInfo(t *testing.T) {
	qm := newTestQuotaManager()
	qm.InitQuota("node1", 300.0)

	// 发送一些消息
	qm.ConsumeQuota("node1", MessageTypeNormal)
	qm.ConsumeQuota("node1", MessageTypeNormal)

	info, err := qm.GetQuotaInfo("node1")
	if err != nil {
		t.Fatalf("GetQuotaInfo failed: %v", err)
	}

	if info.UsedToday != 2 {
		t.Errorf("Expected used 2, got %d", info.UsedToday)
	}
	if !info.CanSend {
		t.Error("Expected can send")
	}
	if info.Percentage != float64(2)/float64(QuotaActive)*100 {
		t.Errorf("Unexpected percentage: %v", info.Percentage)
	}
}

func TestQuotaManager_UnregisteredNode(t *testing.T) {
	qm := newTestQuotaManager()

	err := qm.CanSend("unknown_node", MessageTypeNormal)
	if err == nil {
		t.Error("Expected error for unregistered node")
	}

	_, err = qm.GetQuota("unknown_node")
	if err == nil {
		t.Error("Expected error for unregistered node")
	}
}

func TestSameDay(t *testing.T) {
	t1 := time.Date(2026, 2, 4, 10, 0, 0, 0, time.UTC)
	t2 := time.Date(2026, 2, 4, 23, 59, 59, 0, time.UTC)
	t3 := time.Date(2026, 2, 5, 0, 0, 0, 0, time.UTC)

	if !sameDay(t1, t2) {
		t.Error("Same day should return true")
	}
	if sameDay(t1, t3) {
		t.Error("Different days should return false")
	}
}
