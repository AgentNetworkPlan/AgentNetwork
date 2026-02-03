package auth

import (
	"encoding/json"
	"errors"
	"fmt"
	"math"
	"sync"
	"time"
)

// TokenConstants 贡献代币常量
const (
	BaseReward        = 100.0  // 基础奖励
	DifficultyWeight  = 2.0    // 难度权重
	TimeBonus         = 0.2    // 时间奖励比例
	QualityMultiplier = 1.5    // 质量乘数
	RedundancyPenalty = 0.1    // 冗余惩罚
)

// ContributionType 贡献类型
type ContributionType string

const (
	ContributionTaskExecution  ContributionType = "task_execution"   // 任务执行
	ContributionTaskVerification ContributionType = "task_verification" // 任务验证
	ContributionCommitteeWork  ContributionType = "committee_work"   // 委员会工作
	ContributionNetworkRelay   ContributionType = "network_relay"    // 网络中继
	ContributionDataStorage    ContributionType = "data_storage"     // 数据存储
)

// ContributionRecord 贡献记录
type ContributionRecord struct {
	ID              string           `json:"id"`
	NodeID          string           `json:"node_id"`
	Type            ContributionType `json:"type"`
	TaskID          string           `json:"task_id,omitempty"`
	Difficulty      int              `json:"difficulty"`
	TimeTaken       time.Duration    `json:"time_taken"`
	ExpectedTime    time.Duration    `json:"expected_time"`
	QualityScore    float64          `json:"quality_score"` // 0-1
	RedundancyCount int              `json:"redundancy_count"`
	TokensAwarded   float64          `json:"tokens_awarded"`
	Timestamp       time.Time        `json:"timestamp"`
	Signature       []byte           `json:"signature,omitempty"`
}

// NodeTokenAccount 节点代币账户
type NodeTokenAccount struct {
	NodeID           string               `json:"node_id"`
	TotalTokens      float64              `json:"total_tokens"`
	AvailableTokens  float64              `json:"available_tokens"`
	LockedTokens     float64              `json:"locked_tokens"` // 锁定的代币 (质押等)
	TotalContributions int               `json:"total_contributions"`
	Contributions    []ContributionRecord `json:"contributions"`
	LastUpdated      time.Time            `json:"last_updated"`
}

// TokenCalculator 代币计算器
type TokenCalculator struct {
	// 可配置参数
	BaseReward        float64
	DifficultyWeight  float64
	TimeBonus         float64
	QualityMultiplier float64
	RedundancyPenalty float64
}

// NewTokenCalculator 创建默认计算器
func NewTokenCalculator() *TokenCalculator {
	return &TokenCalculator{
		BaseReward:        BaseReward,
		DifficultyWeight:  DifficultyWeight,
		TimeBonus:         TimeBonus,
		QualityMultiplier: QualityMultiplier,
		RedundancyPenalty: RedundancyPenalty,
	}
}

// CalculateTaskReward 计算任务奖励
// token = base_reward * difficulty_factor * time_factor * quality_factor * redundancy_factor
func (tc *TokenCalculator) CalculateTaskReward(difficulty int, timeTaken, expectedTime time.Duration, quality float64, redundancy int) float64 {
	// 难度因子: 指数增长
	difficultyFactor := math.Pow(float64(difficulty), tc.DifficultyWeight) / math.Pow(5, tc.DifficultyWeight)

	// 时间因子: 提前完成有奖励，超时有惩罚
	timeFactor := 1.0
	if expectedTime > 0 {
		ratio := float64(timeTaken) / float64(expectedTime)
		if ratio < 1 {
			// 提前完成，奖励
			timeFactor = 1 + (1-ratio)*tc.TimeBonus
		} else {
			// 超时，惩罚 (但不低于 0.5)
			timeFactor = math.Max(0.5, 1-(ratio-1)*tc.TimeBonus)
		}
	}

	// 质量因子: 根据质量分数
	qualityFactor := quality * tc.QualityMultiplier
	if quality < 0.5 {
		qualityFactor = quality // 低质量不使用乘数
	}

	// 冗余因子: 多人完成同一任务时分摊奖励
	redundancyFactor := 1.0
	if redundancy > 1 {
		redundancyFactor = 1 / (1 + float64(redundancy-1)*tc.RedundancyPenalty)
	}

	return tc.BaseReward * difficultyFactor * timeFactor * qualityFactor * redundancyFactor
}

// CalculateVerificationReward 计算验证奖励
func (tc *TokenCalculator) CalculateVerificationReward(taskDifficulty int, wasCorrect bool) float64 {
	baseVerificationReward := tc.BaseReward * 0.1 // 验证奖励是基础奖励的 10%
	difficultyFactor := float64(taskDifficulty) / 5.0

	if !wasCorrect {
		return 0 // 错误验证不得奖励
	}

	return baseVerificationReward * difficultyFactor
}

// CalculateCommitteeReward 计算委员会工作奖励
func (tc *TokenCalculator) CalculateCommitteeReward(participatedInVotes int, correctVotes int) float64 {
	if participatedInVotes == 0 {
		return 0
	}

	accuracy := float64(correctVotes) / float64(participatedInVotes)
	baseCommitteeReward := tc.BaseReward * 0.05 // 委员会基础奖励

	return baseCommitteeReward * float64(participatedInVotes) * accuracy
}

// TokenLedger 代币账本
type TokenLedger struct {
	accounts   map[string]*NodeTokenAccount
	calculator *TokenCalculator
	records    []ContributionRecord
	mu         sync.RWMutex
}

// NewTokenLedger 创建代币账本
func NewTokenLedger() *TokenLedger {
	return &TokenLedger{
		accounts:   make(map[string]*NodeTokenAccount),
		calculator: NewTokenCalculator(),
		records:    make([]ContributionRecord, 0),
	}
}

// GetOrCreateAccount 获取或创建账户
func (tl *TokenLedger) GetOrCreateAccount(nodeID string) *NodeTokenAccount {
	tl.mu.Lock()
	defer tl.mu.Unlock()

	if account, exists := tl.accounts[nodeID]; exists {
		return account
	}

	account := &NodeTokenAccount{
		NodeID:        nodeID,
		Contributions: make([]ContributionRecord, 0),
		LastUpdated:   time.Now(),
	}
	tl.accounts[nodeID] = account
	return account
}

// GetAccount 获取账户
func (tl *TokenLedger) GetAccount(nodeID string) (*NodeTokenAccount, error) {
	tl.mu.RLock()
	defer tl.mu.RUnlock()

	account, exists := tl.accounts[nodeID]
	if !exists {
		return nil, errors.New("账户不存在")
	}
	return account, nil
}

// GetBalance 获取余额
func (tl *TokenLedger) GetBalance(nodeID string) (float64, error) {
	tl.mu.RLock()
	defer tl.mu.RUnlock()

	account, exists := tl.accounts[nodeID]
	if !exists {
		return 0, errors.New("账户不存在")
	}
	return account.AvailableTokens, nil
}

// RecordTaskContribution 记录任务贡献
func (tl *TokenLedger) RecordTaskContribution(
	nodeID, taskID string,
	difficulty int,
	timeTaken, expectedTime time.Duration,
	quality float64,
	redundancy int,
) (*ContributionRecord, error) {
	tl.mu.Lock()
	defer tl.mu.Unlock()

	account, exists := tl.accounts[nodeID]
	if !exists {
		account = &NodeTokenAccount{
			NodeID:        nodeID,
			Contributions: make([]ContributionRecord, 0),
		}
		tl.accounts[nodeID] = account
	}

	tokens := tl.calculator.CalculateTaskReward(difficulty, timeTaken, expectedTime, quality, redundancy)

	record := ContributionRecord{
		ID:              fmt.Sprintf("contrib-%s-%d", taskID, time.Now().UnixNano()),
		NodeID:          nodeID,
		Type:            ContributionTaskExecution,
		TaskID:          taskID,
		Difficulty:      difficulty,
		TimeTaken:       timeTaken,
		ExpectedTime:    expectedTime,
		QualityScore:    quality,
		RedundancyCount: redundancy,
		TokensAwarded:   tokens,
		Timestamp:       time.Now(),
	}

	account.TotalTokens += tokens
	account.AvailableTokens += tokens
	account.TotalContributions++
	account.Contributions = append(account.Contributions, record)
	account.LastUpdated = time.Now()

	tl.records = append(tl.records, record)
	return &record, nil
}

// RecordVerificationContribution 记录验证贡献
func (tl *TokenLedger) RecordVerificationContribution(
	nodeID, taskID string,
	taskDifficulty int,
	wasCorrect bool,
) (*ContributionRecord, error) {
	tl.mu.Lock()
	defer tl.mu.Unlock()

	account, exists := tl.accounts[nodeID]
	if !exists {
		account = &NodeTokenAccount{
			NodeID:        nodeID,
			Contributions: make([]ContributionRecord, 0),
		}
		tl.accounts[nodeID] = account
	}

	tokens := tl.calculator.CalculateVerificationReward(taskDifficulty, wasCorrect)

	var quality float64 = 0
	if wasCorrect {
		quality = 1.0
	}

	record := ContributionRecord{
		ID:            fmt.Sprintf("verify-%s-%d", taskID, time.Now().UnixNano()),
		NodeID:        nodeID,
		Type:          ContributionTaskVerification,
		TaskID:        taskID,
		Difficulty:    taskDifficulty,
		QualityScore:  quality,
		TokensAwarded: tokens,
		Timestamp:     time.Now(),
	}

	account.TotalTokens += tokens
	account.AvailableTokens += tokens
	account.TotalContributions++
	account.Contributions = append(account.Contributions, record)
	account.LastUpdated = time.Now()

	tl.records = append(tl.records, record)
	return &record, nil
}

// RecordCommitteeContribution 记录委员会贡献
func (tl *TokenLedger) RecordCommitteeContribution(
	nodeID string,
	participatedVotes int,
	correctVotes int,
) (*ContributionRecord, error) {
	tl.mu.Lock()
	defer tl.mu.Unlock()

	account, exists := tl.accounts[nodeID]
	if !exists {
		account = &NodeTokenAccount{
			NodeID:        nodeID,
			Contributions: make([]ContributionRecord, 0),
		}
		tl.accounts[nodeID] = account
	}

	tokens := tl.calculator.CalculateCommitteeReward(participatedVotes, correctVotes)

	quality := 0.0
	if participatedVotes > 0 {
		quality = float64(correctVotes) / float64(participatedVotes)
	}

	record := ContributionRecord{
		ID:            fmt.Sprintf("committee-%s-%d", nodeID, time.Now().UnixNano()),
		NodeID:        nodeID,
		Type:          ContributionCommitteeWork,
		QualityScore:  quality,
		TokensAwarded: tokens,
		Timestamp:     time.Now(),
	}

	account.TotalTokens += tokens
	account.AvailableTokens += tokens
	account.TotalContributions++
	account.Contributions = append(account.Contributions, record)
	account.LastUpdated = time.Now()

	tl.records = append(tl.records, record)
	return &record, nil
}

// LockTokens 锁定代币 (质押)
func (tl *TokenLedger) LockTokens(nodeID string, amount float64) error {
	tl.mu.Lock()
	defer tl.mu.Unlock()

	account, exists := tl.accounts[nodeID]
	if !exists {
		return errors.New("账户不存在")
	}

	if account.AvailableTokens < amount {
		return errors.New("可用代币不足")
	}

	account.AvailableTokens -= amount
	account.LockedTokens += amount
	account.LastUpdated = time.Now()
	return nil
}

// UnlockTokens 解锁代币
func (tl *TokenLedger) UnlockTokens(nodeID string, amount float64) error {
	tl.mu.Lock()
	defer tl.mu.Unlock()

	account, exists := tl.accounts[nodeID]
	if !exists {
		return errors.New("账户不存在")
	}

	if account.LockedTokens < amount {
		return errors.New("锁定代币不足")
	}

	account.LockedTokens -= amount
	account.AvailableTokens += amount
	account.LastUpdated = time.Now()
	return nil
}

// Transfer 转账
func (tl *TokenLedger) Transfer(fromNodeID, toNodeID string, amount float64) error {
	tl.mu.Lock()
	defer tl.mu.Unlock()

	fromAccount, exists := tl.accounts[fromNodeID]
	if !exists {
		return errors.New("发送方账户不存在")
	}

	if fromAccount.AvailableTokens < amount {
		return errors.New("可用代币不足")
	}

	toAccount, exists := tl.accounts[toNodeID]
	if !exists {
		toAccount = &NodeTokenAccount{
			NodeID:        toNodeID,
			Contributions: make([]ContributionRecord, 0),
		}
		tl.accounts[toNodeID] = toAccount
	}

	fromAccount.AvailableTokens -= amount
	toAccount.AvailableTokens += amount
	toAccount.TotalTokens += amount
	fromAccount.LastUpdated = time.Now()
	toAccount.LastUpdated = time.Now()

	return nil
}

// GetTopContributors 获取贡献最多的节点
func (tl *TokenLedger) GetTopContributors(count int) []*NodeTokenAccount {
	tl.mu.RLock()
	defer tl.mu.RUnlock()

	accounts := make([]*NodeTokenAccount, 0, len(tl.accounts))
	for _, acc := range tl.accounts {
		accounts = append(accounts, acc)
	}

	// 按总代币排序
	for i := 0; i < len(accounts)-1; i++ {
		for j := i + 1; j < len(accounts); j++ {
			if accounts[j].TotalTokens > accounts[i].TotalTokens {
				accounts[i], accounts[j] = accounts[j], accounts[i]
			}
		}
	}

	if count > len(accounts) {
		count = len(accounts)
	}

	return accounts[:count]
}

// GetAllRecords 获取所有记录
func (tl *TokenLedger) GetAllRecords() []ContributionRecord {
	tl.mu.RLock()
	defer tl.mu.RUnlock()
	return append([]ContributionRecord{}, tl.records...)
}

// ExportState 导出状态
func (tl *TokenLedger) ExportState() ([]byte, error) {
	tl.mu.RLock()
	defer tl.mu.RUnlock()

	state := struct {
		Accounts map[string]*NodeTokenAccount `json:"accounts"`
		Records  []ContributionRecord         `json:"records"`
	}{
		Accounts: tl.accounts,
		Records:  tl.records,
	}

	return json.Marshal(state)
}

// ImportState 导入状态
func (tl *TokenLedger) ImportState(data []byte) error {
	tl.mu.Lock()
	defer tl.mu.Unlock()

	var state struct {
		Accounts map[string]*NodeTokenAccount `json:"accounts"`
		Records  []ContributionRecord         `json:"records"`
	}

	if err := json.Unmarshal(data, &state); err != nil {
		return err
	}

	tl.accounts = state.Accounts
	tl.records = state.Records
	return nil
}

// GetTotalTokensInCirculation 获取流通中的总代币
func (tl *TokenLedger) GetTotalTokensInCirculation() float64 {
	tl.mu.RLock()
	defer tl.mu.RUnlock()

	var total float64
	for _, acc := range tl.accounts {
		total += acc.TotalTokens
	}
	return total
}
