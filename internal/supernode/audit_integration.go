// Package supernode - audit_integration.go
// Task44: 审计偏离惩罚闭环集成
// 将 onAuditorDeviation 回调与 Ledger Violation 事件和 Collateral Slashing 联动

package supernode

import (
	"encoding/json"
	"fmt"

	"github.com/AgentNetworkPlan/AgentNetwork/internal/collateral"
	"github.com/AgentNetworkPlan/AgentNetwork/internal/ledger"
)

// AuditPenaltyConfig 审计惩罚配置
type AuditPenaltyConfig struct {
	// 声誉惩罚
	MinorDeviationPenalty  float64 // 轻微偏离的声誉惩罚
	SevereDeviationPenalty float64 // 严重偏离的声誉惩罚

	// 抵押物惩罚
	MinorSlashRatio  float64 // 轻微偏离的抵押罚没比例
	SevereSlashRatio float64 // 严重偏离的抵押罚没比例

	// 抵押物用途标识（用于 GetCollateralByNodePurpose）
	AuditorCollateralPurpose string

	// 是否启用自动惩罚
	EnableAutoSlash bool
}

// DefaultAuditPenaltyConfig 返回默认配置
func DefaultAuditPenaltyConfig() *AuditPenaltyConfig {
	return &AuditPenaltyConfig{
		MinorDeviationPenalty:    5.0,  // 轻微偏离扣5点声誉
		SevereDeviationPenalty:   20.0, // 严重偏离扣20点声誉
		MinorSlashRatio:          0.1,  // 轻微偏离罚没10%抵押
		SevereSlashRatio:         0.3,  // 严重偏离罚没30%抵押
		AuditorCollateralPurpose: "supernode_auditor",
		EnableAutoSlash:          true,
	}
}

// AuditIntegration 审计惩罚闭环集成器
type AuditIntegration struct {
	config           *AuditPenaltyConfig
	ledger           *ledger.Ledger
	collateralMgr    *collateral.CollateralManager
	supernodeMgr     *SuperNodeManager
	systemSignerID   string // 系统签名者ID（用于 Ledger 事件）

	// 回调
	onPenaltyApplied func(deviation *AuditDeviation, event *ledger.Event, slashEvent *collateral.SlashEvent)
}

// NewAuditIntegration 创建审计惩罚集成器
func NewAuditIntegration(
	config *AuditPenaltyConfig,
	ledgerInstance *ledger.Ledger,
	collateralMgr *collateral.CollateralManager,
	supernodeMgr *SuperNodeManager,
	systemSignerID string,
) *AuditIntegration {
	if config == nil {
		config = DefaultAuditPenaltyConfig()
	}

	ai := &AuditIntegration{
		config:         config,
		ledger:         ledgerInstance,
		collateralMgr:  collateralMgr,
		supernodeMgr:   supernodeMgr,
		systemSignerID: systemSignerID,
	}

	return ai
}

// SetOnPenaltyApplied 设置惩罚应用后的回调
func (ai *AuditIntegration) SetOnPenaltyApplied(fn func(*AuditDeviation, *ledger.Event, *collateral.SlashEvent)) {
	ai.onPenaltyApplied = fn
}

// Start 启动集成（注册回调）
func (ai *AuditIntegration) Start() {
	if ai.supernodeMgr == nil {
		return
	}

	// 注册审计偏离回调
	ai.supernodeMgr.SetOnAuditorDeviation(ai.handleAuditorDeviation)
}

// handleAuditorDeviation 处理审计偏离事件
func (ai *AuditIntegration) handleAuditorDeviation(deviation *AuditDeviation) {
	if deviation == nil {
		return
	}

	var violationEvent *ledger.Event
	var slashEvent *collateral.SlashEvent
	var err error

	// 1. 确定惩罚力度
	var reputationPenalty float64
	var slashRatio float64
	var severityStr string

	switch deviation.Severity {
	case "severe":
		reputationPenalty = ai.config.SevereDeviationPenalty
		slashRatio = ai.config.SevereSlashRatio
		severityStr = "severe"
	default: // "minor" or unknown
		reputationPenalty = ai.config.MinorDeviationPenalty
		slashRatio = ai.config.MinorSlashRatio
		severityStr = "minor"
	}

	// 2. 发送 Ledger Violation 事件
	if ai.ledger != nil {
		violationData := ledger.ViolationData{
			NodeID:        deviation.AuditorID,
			ViolationType: "audit_deviation",
			Severity:      severityStr,
			Penalty:       reputationPenalty,
			Evidence:      ai.buildEvidenceString(deviation),
			ReporterID:    ai.systemSignerID,
		}

		violationEvent, err = ai.ledger.AppendEvent(
			ledger.EventViolation,
			deviation.AuditorID,
			violationData,
			ai.systemSignerID,
		)
		if err != nil {
			// 记录错误但继续执行抵押惩罚
			fmt.Printf("Warning: failed to emit violation event for auditor %s: %v\n",
				deviation.AuditorID, err)
		}
	}

	// 3. 抵押物惩罚（如果启用）
	if ai.config.EnableAutoSlash && ai.collateralMgr != nil {
		evidence := []string{
			fmt.Sprintf("audit_id:%s", deviation.AuditID),
			fmt.Sprintf("expected:%s", deviation.ExpectedResult),
			fmt.Sprintf("actual:%s", deviation.ActualResult),
			fmt.Sprintf("severity:%s", deviation.Severity),
		}

		slashEvent, err = ai.collateralMgr.SlashByNodePurpose(
			deviation.AuditorID,
			ai.config.AuditorCollateralPurpose,
			fmt.Sprintf("audit_deviation:%s", deviation.Severity),
			evidence,
			slashRatio,
		)
		if err != nil {
			// 可能是审计者没有抵押物，记录警告但不中断
			fmt.Printf("Warning: failed to slash collateral for auditor %s: %v\n",
				deviation.AuditorID, err)
		}
	}

	// 4. 触发惩罚应用回调
	if ai.onPenaltyApplied != nil {
		ai.onPenaltyApplied(deviation, violationEvent, slashEvent)
	}
}

// buildEvidenceString 构建证据字符串
func (ai *AuditIntegration) buildEvidenceString(deviation *AuditDeviation) string {
	evidence := map[string]interface{}{
		"audit_id":        deviation.AuditID,
		"auditor_id":      deviation.AuditorID,
		"expected_result": deviation.ExpectedResult,
		"actual_result":   deviation.ActualResult,
		"severity":        deviation.Severity,
		"detected_at":     deviation.DetectedAt.Unix(),
	}
	bytes, _ := json.Marshal(evidence)
	return string(bytes)
}

// GetPenaltyForSeverity 获取指定严重程度的惩罚配置
func (ai *AuditIntegration) GetPenaltyForSeverity(severity string) (reputationPenalty, slashRatio float64) {
	switch severity {
	case "severe":
		return ai.config.SevereDeviationPenalty, ai.config.SevereSlashRatio
	default:
		return ai.config.MinorDeviationPenalty, ai.config.MinorSlashRatio
	}
}

// ManualPenalty 手动触发惩罚（用于外部调用）
func (ai *AuditIntegration) ManualPenalty(deviation *AuditDeviation) (*ledger.Event, *collateral.SlashEvent, error) {
	if deviation == nil {
		return nil, nil, fmt.Errorf("deviation cannot be nil")
	}

	var violationEvent *ledger.Event
	var slashEvent *collateral.SlashEvent
	var lastErr error

	// 确定惩罚力度
	reputationPenalty, slashRatio := ai.GetPenaltyForSeverity(deviation.Severity)

	// 发送 Violation 事件
	if ai.ledger != nil {
		violationData := ledger.ViolationData{
			NodeID:        deviation.AuditorID,
			ViolationType: "audit_deviation",
			Severity:      deviation.Severity,
			Penalty:       reputationPenalty,
			Evidence:      ai.buildEvidenceString(deviation),
			ReporterID:    ai.systemSignerID,
		}

		var err error
		violationEvent, err = ai.ledger.AppendEvent(
			ledger.EventViolation,
			deviation.AuditorID,
			violationData,
			ai.systemSignerID,
		)
		if err != nil {
			lastErr = fmt.Errorf("failed to emit violation event: %w", err)
		}
	}

	// 抵押物惩罚
	if ai.collateralMgr != nil {
		evidence := []string{
			fmt.Sprintf("audit_id:%s", deviation.AuditID),
			fmt.Sprintf("severity:%s", deviation.Severity),
		}

		var err error
		slashEvent, err = ai.collateralMgr.SlashByNodePurpose(
			deviation.AuditorID,
			ai.config.AuditorCollateralPurpose,
			fmt.Sprintf("audit_deviation:%s", deviation.Severity),
			evidence,
			slashRatio,
		)
		if err != nil {
			if lastErr != nil {
				lastErr = fmt.Errorf("%v; also failed to slash: %w", lastErr, err)
			} else {
				lastErr = fmt.Errorf("failed to slash collateral: %w", err)
			}
		}
	}

	return violationEvent, slashEvent, lastErr
}
