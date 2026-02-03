package auth

import (
	"testing"
	"time"
)

func TestNewTokenCalculator(t *testing.T) {
	calc := NewTokenCalculator()
	if calc == nil {
		t.Fatal("代币计算器不应为 nil")
	}

	if calc.BaseReward != BaseReward {
		t.Errorf("基础奖励应为 %f，实际为 %f", BaseReward, calc.BaseReward)
	}
}

func TestTokenCalculator_CalculateTaskReward(t *testing.T) {
	calc := NewTokenCalculator()

	// 基础测试
	reward := calc.CalculateTaskReward(5, time.Hour, time.Hour, 1.0, 1)
	if reward <= 0 {
		t.Error("奖励应该大于 0")
	}

	t.Logf("基础奖励: %.2f", reward)
}

func TestTokenCalculator_DifficultyFactor(t *testing.T) {
	calc := NewTokenCalculator()

	// 低难度任务
	lowReward := calc.CalculateTaskReward(1, time.Hour, time.Hour, 1.0, 1)
	// 高难度任务
	highReward := calc.CalculateTaskReward(5, time.Hour, time.Hour, 1.0, 1)

	if highReward <= lowReward {
		t.Error("高难度任务奖励应高于低难度任务")
	}

	t.Logf("低难度奖励: %.2f, 高难度奖励: %.2f", lowReward, highReward)
}

func TestTokenCalculator_TimeFactor(t *testing.T) {
	calc := NewTokenCalculator()

	// 提前完成
	earlyReward := calc.CalculateTaskReward(3, 30*time.Minute, time.Hour, 1.0, 1)
	// 准时完成
	onTimeReward := calc.CalculateTaskReward(3, time.Hour, time.Hour, 1.0, 1)
	// 延迟完成
	lateReward := calc.CalculateTaskReward(3, 2*time.Hour, time.Hour, 1.0, 1)

	if earlyReward <= onTimeReward {
		t.Error("提前完成的奖励应高于准时完成")
	}

	if lateReward >= onTimeReward {
		t.Error("延迟完成的奖励应低于准时完成")
	}

	t.Logf("提前: %.2f, 准时: %.2f, 延迟: %.2f", earlyReward, onTimeReward, lateReward)
}

func TestTokenCalculator_QualityFactor(t *testing.T) {
	calc := NewTokenCalculator()

	// 高质量
	highQualityReward := calc.CalculateTaskReward(3, time.Hour, time.Hour, 1.0, 1)
	// 低质量
	lowQualityReward := calc.CalculateTaskReward(3, time.Hour, time.Hour, 0.5, 1)

	if lowQualityReward >= highQualityReward {
		t.Error("高质量任务奖励应高于低质量任务")
	}

	t.Logf("高质量: %.2f, 低质量: %.2f", highQualityReward, lowQualityReward)
}

func TestTokenCalculator_RedundancyFactor(t *testing.T) {
	calc := NewTokenCalculator()

	// 单人完成
	singleReward := calc.CalculateTaskReward(3, time.Hour, time.Hour, 1.0, 1)
	// 多人完成
	multiReward := calc.CalculateTaskReward(3, time.Hour, time.Hour, 1.0, 3)

	if multiReward >= singleReward {
		t.Error("多人完成时奖励应分摊（减少）")
	}

	t.Logf("单人: %.2f, 多人(3): %.2f", singleReward, multiReward)
}

func TestTokenCalculator_VerificationReward(t *testing.T) {
	calc := NewTokenCalculator()

	// 正确验证
	correctReward := calc.CalculateVerificationReward(5, true)
	// 错误验证
	wrongReward := calc.CalculateVerificationReward(5, false)

	if correctReward <= 0 {
		t.Error("正确验证应该有奖励")
	}

	if wrongReward != 0 {
		t.Error("错误验证不应有奖励")
	}
}

func TestTokenCalculator_CommitteeReward(t *testing.T) {
	calc := NewTokenCalculator()

	// 高准确率
	highAccuracyReward := calc.CalculateCommitteeReward(10, 10)
	// 低准确率
	lowAccuracyReward := calc.CalculateCommitteeReward(10, 5)

	if lowAccuracyReward >= highAccuracyReward {
		t.Error("高准确率应该有更高奖励")
	}
}

func TestNewTokenLedger(t *testing.T) {
	ledger := NewTokenLedger()
	if ledger == nil {
		t.Fatal("代币账本不应为 nil")
	}
}

func TestTokenLedger_GetOrCreateAccount(t *testing.T) {
	ledger := NewTokenLedger()

	account := ledger.GetOrCreateAccount("node-001")
	if account == nil {
		t.Fatal("账户不应为 nil")
	}

	if account.NodeID != "node-001" {
		t.Error("节点 ID 不匹配")
	}

	// 再次获取应返回同一账户
	account2 := ledger.GetOrCreateAccount("node-001")
	if account != account2 {
		t.Error("应返回同一账户")
	}
}

func TestTokenLedger_RecordTaskContribution(t *testing.T) {
	ledger := NewTokenLedger()
	ledger.GetOrCreateAccount("node-001")

	record, err := ledger.RecordTaskContribution(
		"node-001", "task-001",
		5, time.Hour, time.Hour,
		1.0, 1,
	)

	if err != nil {
		t.Fatalf("记录贡献失败: %v", err)
	}

	if record.TokensAwarded <= 0 {
		t.Error("奖励代币应大于 0")
	}

	// 检查账户余额
	balance, _ := ledger.GetBalance("node-001")
	if balance != record.TokensAwarded {
		t.Error("账户余额与奖励不匹配")
	}
}

func TestTokenLedger_RecordVerificationContribution(t *testing.T) {
	ledger := NewTokenLedger()
	ledger.GetOrCreateAccount("node-001")

	record, err := ledger.RecordVerificationContribution(
		"node-001", "task-001",
		5, true,
	)

	if err != nil {
		t.Fatalf("记录验证贡献失败: %v", err)
	}

	if record.TokensAwarded <= 0 {
		t.Error("正确验证应有奖励")
	}
}

func TestTokenLedger_LockUnlockTokens(t *testing.T) {
	ledger := NewTokenLedger()
	ledger.GetOrCreateAccount("node-001")

	// 记录贡献获得代币
	ledger.RecordTaskContribution("node-001", "task-001", 5, time.Hour, time.Hour, 1.0, 1)

	account, _ := ledger.GetAccount("node-001")
	initialAvailable := account.AvailableTokens

	// 锁定部分代币
	lockAmount := initialAvailable * 0.5
	err := ledger.LockTokens("node-001", lockAmount)
	if err != nil {
		t.Fatalf("锁定代币失败: %v", err)
	}

	account, _ = ledger.GetAccount("node-001")
	if account.AvailableTokens != initialAvailable-lockAmount {
		t.Error("可用代币应减少")
	}
	if account.LockedTokens != lockAmount {
		t.Error("锁定代币应增加")
	}

	// 解锁代币
	err = ledger.UnlockTokens("node-001", lockAmount)
	if err != nil {
		t.Fatalf("解锁代币失败: %v", err)
	}

	account, _ = ledger.GetAccount("node-001")
	if account.AvailableTokens != initialAvailable {
		t.Error("解锁后可用代币应恢复")
	}
}

func TestTokenLedger_Transfer(t *testing.T) {
	ledger := NewTokenLedger()
	ledger.GetOrCreateAccount("node-001")
	ledger.GetOrCreateAccount("node-002")

	// 给 node-001 一些代币
	ledger.RecordTaskContribution("node-001", "task-001", 5, time.Hour, time.Hour, 1.0, 1)
	
	balance1, _ := ledger.GetBalance("node-001")
	transferAmount := balance1 * 0.5

	// 转账
	err := ledger.Transfer("node-001", "node-002", transferAmount)
	if err != nil {
		t.Fatalf("转账失败: %v", err)
	}

	newBalance1, _ := ledger.GetBalance("node-001")
	balance2, _ := ledger.GetBalance("node-002")

	if newBalance1 != balance1-transferAmount {
		t.Error("发送方余额应减少")
	}
	if balance2 != transferAmount {
		t.Error("接收方余额应增加")
	}
}

func TestTokenLedger_TransferInsufficientFunds(t *testing.T) {
	ledger := NewTokenLedger()
	ledger.GetOrCreateAccount("node-001")
	ledger.GetOrCreateAccount("node-002")

	// 尝试转账（没有代币）
	err := ledger.Transfer("node-001", "node-002", 100)
	if err == nil {
		t.Error("余额不足时转账应失败")
	}
}

func TestTokenLedger_GetTopContributors(t *testing.T) {
	ledger := NewTokenLedger()

	// 创建多个账户并记录贡献
	nodes := []string{"node-a", "node-b", "node-c", "node-d", "node-e"}
	for i, nodeID := range nodes {
		ledger.GetOrCreateAccount(nodeID)
		// 每个节点贡献不同数量
		for j := 0; j < i+1; j++ {
			ledger.RecordTaskContribution(nodeID, "task", 5, time.Hour, time.Hour, 1.0, 1)
		}
	}

	// 获取前 3 名
	top := ledger.GetTopContributors(3)
	if len(top) != 3 {
		t.Errorf("应返回 3 个贡献者，实际返回 %d", len(top))
	}

	// 验证排序（node-e 贡献最多）
	if top[0].NodeID != "node-e" {
		t.Error("node-e 应该排第一")
	}
}

func TestTokenLedger_ExportImportState(t *testing.T) {
	ledger := NewTokenLedger()
	ledger.GetOrCreateAccount("node-001")
	ledger.RecordTaskContribution("node-001", "task-001", 5, time.Hour, time.Hour, 1.0, 1)

	// 导出状态
	data, err := ledger.ExportState()
	if err != nil {
		t.Fatalf("导出状态失败: %v", err)
	}

	// 创建新账本并导入
	ledger2 := NewTokenLedger()
	err = ledger2.ImportState(data)
	if err != nil {
		t.Fatalf("导入状态失败: %v", err)
	}

	// 验证状态
	balance, _ := ledger2.GetBalance("node-001")
	if balance == 0 {
		t.Error("导入后余额应该大于 0")
	}
}

func TestTokenLedger_GetTotalTokensInCirculation(t *testing.T) {
	ledger := NewTokenLedger()

	// 多个节点贡献
	nodes := []string{"node-a", "node-b", "node-c"}
	for _, nodeID := range nodes {
		ledger.GetOrCreateAccount(nodeID)
		ledger.RecordTaskContribution(nodeID, "task", 5, time.Hour, time.Hour, 1.0, 1)
	}

	total := ledger.GetTotalTokensInCirculation()
	if total <= 0 {
		t.Error("流通代币总量应大于 0")
	}

	t.Logf("流通代币总量: %.2f", total)
}
