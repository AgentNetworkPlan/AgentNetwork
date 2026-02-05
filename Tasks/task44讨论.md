
# Task44：审计冲突裁决与资产处置闭环（正式讨论稿）

日期：2026-02-05

## 0. 背景与目标

AgentNetwork 当前已经具备：

- **争议/仲裁（Dispute）**：支持“System（自动规则）”与“Committee（投票仲裁）”两条路径。
- **托管资金（Escrow）**：支持锁定、争议、释放、退款，并已具备 **forfeited（罚没）** 状态机。
- **抵押物（Collateral）**：支持质押与 **SlashCollateral（抵押罚没）**。
- **审计（Supernode MultiAudit）**：支持多审计者分配、阈值/比例汇总、最终结果回调，但目前仅更新 PassRate，未形成惩罚闭环。
- **共识存证（Ledger + Snapshot）**：支持违规事件（EventViolation）驱动的声誉扣减快照回放；当前不是支付清算链。

本 Task 的目标：

1) 在不引入“第二账本（资金划转/清算链）”的前提下，闭环实现：**审计冲突裁决 → 经济惩罚（押金/抵押） → 全网共识存证（声誉扣减）**。
2) 明确“AI 预审/审计助手”的职责边界：**可做报告/建议/投票输入，但不持有释放资金的核按钮**。
3) 以“复用既有状态机、最小侵入改动”为工程原则，给出可实现的落点与接口建议。

## 1. 核心结论：双层处置架构（资产执行层 + 共识存证层）

针对“审计者偏离共识 / 恶意审计 / 争议裁决失败”这类场景：

### 1.1 资产执行层：Escrow / Collateral 负责“真罚”

- **任务资金惩罚**：复用 Escrow 的 `Forfeit()`，把任务押金/保证金留在 escrow 状态机中完成罚没（资产状态真实、可审计）。
- **审计者抵押惩罚**：复用 Collateral 的 `SlashCollateral()`，把超级节点（审计者）的质押罚没留在 collateral 状态机中完成。

### 1.2 共识存证层：Ledger / Snapshot 负责“可共识的声誉扣减”

- 任何“罚没/扣罚”的事实，都必须被映射为一条可广播、可回放的 `EventViolation` 事件。
- Snapshot 回放负责对 `node.Reputation` 做确定性扣减，形成全网一致的“声誉账”。

**为什么这是最优解**：

- 复用成熟状态机（escrow/collateral 已经把生命周期建好），不在 ledger 里引入复杂资金逻辑。
- 职责分离：资产状态变化（执行）与可共识记录（存证）分层，避免对账不一致。
- 未来对接链上也自然：资产执行可映射为合约调用，ledger 可映射为事件日志/链下共识日志。

## 2. 现状问题复盘（来自代码扫描结论）

### 2.1 Dispute 的 System 自动裁决存在“演示级风险”

`TryAutoResolve()` 依赖 `defaultAutoRules()` 主要通过 evidence 的 `type` 字符串存在与否推断结果，且未强制使用 `Evidence.Verified`。

风险：

- “只要敢填，我就敢判”：攻击者可以伪造 evidence type 触发自动判定。
- 缺少证据完整性与可验证性约束：签名、哈希、链路证明、时间戳等都未成为硬条件。

结论：

- System 自动裁决应降级为“预审建议/分流”，不能直接触发资产释放。

### 2.2 Escrow 的争议释放路径存在“核按钮集中化”倾向

Escrow 在争议解决里存在 `arbitratorSig` 这类单点输入特征，若未经多方授权/阈值签名约束，容易形成中心化风险。

结论：

- AI/单一 arbitrator 不应直接成为释放资金的唯一授权来源。
- 建议把“资金释放授权”收敛到多方可验证的决议结果（委员会阈值、多审计阈值、或阈值签名）。

### 2.3 Supernode MultiAudit 已具备“去中心化审计骨架”，但缺少惩罚闭环

现状：

- 多个审计者对同一任务提交结果，按阈值/比例形成 `FinalResult`。
- 当前仅更新审计者 PassRate，并通过回调 `onAuditCompleted` 通知完成。

缺口：

- 没有把“偏离共识”的审计者惩罚（声誉/抵押）落到 ledger / collateral。

## 3. 方案设计：审计冲突裁决与处置闭环

### 3.1 事件链路（逻辑闭环）

1) **任务执行完成** → 进入审计（MultiAudit）。
2) **多个审计者提交结果** → Supernode 汇总得到 `FinalResult`。
3) 若 `FinalResult` 触发处罚条件（如任务失败、恶意行为、审计者偏离）：
	 - 资产执行层：调用 `escrow.Forfeit()` 或 `collateral.SlashCollateral()`。
	 - 共识存证层：同时 `ledger.Emit(EventViolation{...})`，Snapshot 回放扣减声誉。
4) 对应证据（AuditID / TaskID / EscrowID / CollateralID / 结果签名集合）进入 `EventViolation.Evidence`，保证可追溯。

### 3.2 处置对象与动作矩阵

| 场景 | 处罚对象 | 资产执行层动作 | 共识存证层动作 |
|---|---|---|---|
| 任务失败且需罚没押金 | 任务相关方（如执行者/委托方，按 dispute 决议） | `escrow.Forfeit(escrowID, reason)` | `EventViolation(Penalty=..., Evidence={EscrowID,...})` |
| 审计者严重偏离共识 | 审计者节点 | `collateral.SlashCollateral(collateralID, amount, reason, evidence)` | `EventViolation(Penalty=..., Evidence={AuditID, CollateralID,...})` |
| 轻微偏离/误差 | 审计者节点 | （可选）不 slash，仅警告/降低权重 | `EventViolation` 或仅记录审计统计 |

注：处罚阈值与惩罚强度建议可配置（避免硬编码）。

## 4. 详细执行方案（按模块落点）

### 4.1 任务押金没收（Task Escrow Forfeit）

触发条件（示例）：

- Dispute 最终裁决为“任务失败且执行方违约”；或
- 审计 FinalResult 认定任务未完成/造假，且进入“资产处置”分支。

动作：

- 调用 Escrow 的 `Forfeit(escrowID, reason)`。
- 状态变化预期：
	- `Status = EscrowForfeited`
	- `ReleaseCondition = "forfeited: <reason>"`

联动存证：

- 同步产生 `EventViolation`，Evidence 至少包含：`EscrowID`、`TaskID`、裁决来源（system/committee/audit）、相关签名/投票摘要。

### 4.2 审计者抵押罚没（Auditor Collateral Slashing）

触发条件（建议分级）：

- **严重偏离**：审计者结论与 `FinalResult` 相反，且偏离次数/比例超过阈值；
- **恶意证据**：提交无效签名/伪造证明（可验证）；
- **拒绝服务**：被分配审计但长期不提交结果（可作为单独的 violation）。

动作：

- 调用 Collateral 的 `SlashCollateral(collateralID, amount, reason, evidence)`（具体签名参数以现有实现为准）。
- 该操作会产生 SlashEvent 记录并记录证据。

联动存证：

- 同步产生 `EventViolation`：
	- `ReporterID`：建议为 supernode/audit 模块的“系统角色 ID”或委员会 ID
	- `Penalty`：声誉扣减值（与 slash 金额解耦，但应存在映射/比例关系）
	- `Evidence`：包含 `AuditID`、审计者提交结果摘要、FinalResult、审计记录签名集合、`CollateralID`

### 4.3 声誉共识扣减（Ledger / Snapshot）

要求：

- 所有可罚事件都必须落一条 `EventViolation`，使各节点 Snapshot 回放得到一致的声誉扣减。
- 避免在 ledger 中写“资金划转/公共奖励池分配”逻辑；ledger 只做存证与声誉共识。

## 5. AI 预审/审计助手的定位（替代/增强 TryAutoResolve 的正确方式）

AI 不应被设计成“仲裁者签名持有者”，更适合承担：

1) **预审报告**：对证据完整性、可验证性、时序一致性生成结构化报告；
2) **建议结论**：给出“建议裁决方向 + 置信度 + 关键缺失证据”；
3) **生成可复核的检查清单**：把自动规则从“字符串存在”升级为“可验证断言”。

建议 Prompt 三大维度（可直接落地为审计表单）：

- **协议事实维度**：任务约定、截止时间、交付定义、付款条件是否满足；
- **证据完整性维度**：证据是否具备哈希/签名/来源、是否可复验、是否存在矛盾；
- **可执行性维度**：若进入资产处置，指明应走 escrow forfeiture 还是 collateral slashing，并输出 evidence 映射字段。

## 6. 安全与一致性要求

- **证据可验证**：关键 evidence 必须可复验（签名、哈希、来源、时间戳、关联 ID）。
- **权限边界**：
	- 资产释放/罚没必须来自“可验证的集体决议”（委员会投票/多审计阈值/阈值签名），不能由单点 AI/单点 arbitrator 决定。
- **可追溯**：EventViolation 的 Evidence 必须能回链到 Audit/Dispute/Escrow/Collateral 实体。
- **确定性**：Snapshot 回放对声誉的影响必须确定性（同输入同输出）。

## 7. 风险提示与非目标（避免范围失控）

### 7.1 风险提示

- **资金归属**：当前 forfeited 的资金在代码层面处于“被锁定/被没收”状态，但未定义自动流向公共奖励池/举报人/保险库。
- **Sybil 防御**：需明确 collateral 的最小抵押门槛与审计分配策略，避免低成本马甲稀释审计权重。
- **惩罚参数治理**：Penalty/Slash 金额、偏离阈值、超时阈值应可配置并可治理（避免硬编码）。

### 7.2 非目标

- 本 Task 不引入 ledger 的资金划转/支付清算功能。
- 本 Task 不实现完整链上合约，只保证未来可映射。

## 8. 下一步落地建议（最小改动实施顺序）

> **✅ 已实现 (2026-02-05)**

1) ✅ **审计偏离→Violation桥接**：在 `supernode.go` 中新增 `AuditDeviation` 结构和 `onAuditorDeviation` 回调，`tryFinalizeAudit` 现在会检测偏离共识的审计者并触发回调（可用于 Emit EventViolation / SlashCollateral）。
   - 代码位置：[internal/supernode/supernode.go](../internal/supernode/supernode.go)
   - 新增类型：`AuditDeviation`（含 AuditID、AuditorID、ExpectedResult、ActualResult、Severity）
   - 新增回调：`SetOnAuditorDeviation(func(*AuditDeviation))`

2) ✅ **Collateral绑定NodeID映射**：在 `collateral.go` 中新增 `byNodePurpose` 索引（NodeID+Purpose → CollateralID），以及 `GetCollateralByNodePurpose` 和 `SlashByNodePurpose` 便捷方法。
   - 代码位置：[internal/collateral/collateral.go](../internal/collateral/collateral.go)
   - 新增方法：`GetCollateralByNodePurpose(nodeID, purpose)`、`SlashByNodePurpose(...)`

3) ✅ **Dispute自动裁决降级+Verified**：`TryAutoResolve` 现在返回 `AutoResolveSuggestion`（预审建议）而非直接裁决，包含置信度、缺失证据、警告列表。只有当证据已验证（`Evidence.Verified`）时 `CanAutoExecute` 才为 true。新增 `ApplyAutoResolution` 方法用于批准执行，新增 `VerifyEvidence` 方法用于标记证据已验证。
   - 代码位置：[internal/dispute/dispute.go](../internal/dispute/dispute.go)
   - 新增类型：`AutoResolveSuggestion`
   - 新增方法：`ApplyAutoResolution`、`VerifyEvidence`

4) ✅ **Escrow争议释放多方约束**：`ResolveDispute` 现在接受 `map[string]string`（多个仲裁者签名）而非单一 `arbitratorSig`，配置中新增 `MinArbitratorSigs`（默认 2）和 `ArbitratorSigThreshold`。新增 `SubmitArbitratorSignature` 用于逐步收集签名，`GetArbitratorSignatureCount` 用于查询签名进度。
   - 代码位置：[internal/escrow/escrow.go](../internal/escrow/escrow.go)
   - 新增配置：`MinArbitratorSigs`、`ArbitratorSigThreshold`
   - 新增方法：`SubmitArbitratorSignature`、`GetArbitratorSignatureCount`

## 9. 后续可选迭代

> **✅ 已完成 (2026-02-05)**

1) ✅ **审计偏离→Violation事件集成**：创建 `AuditIntegration` 模块，自动将审计偏离转换为 Ledger Violation 事件。
   - 代码位置：[internal/supernode/audit_integration.go](../internal/supernode/audit_integration.go)
   - 配置类型：`AuditPenaltyConfig`（含声誉惩罚、抵押罚没比例等）
   - 核心方法：`Start()`（注册回调）、`ManualPenalty()`（手动触发惩罚）

2) ✅ **SlashByNodePurpose联动**：`AuditIntegration.handleAuditorDeviation` 自动调用 `SlashByNodePurpose` 实现抵押罚没。
   - 严重偏离（severe）：30% 抵押罚没 + 20 点声誉扣减
   - 轻微偏离（minor）：10% 抵押罚没 + 5 点声誉扣减
   - 支持回调 `onPenaltyApplied` 用于外部监听

**待实现：**

3) 为 `VerifyEvidence` 实现可验证签名校验（如调用 crypto 模块验证 hash/signature）。
4) 设计"资金归属"机制：forfeited 押金自动流向公共奖励池/举报人激励。

## 10. 使用示例

```go
// 创建审计惩罚闭环集成
config := supernode.DefaultAuditPenaltyConfig()
ai := supernode.NewAuditIntegration(config, ledger, collateralMgr, supernodeMgr, "system")

// 设置惩罚应用回调（可选）
ai.SetOnPenaltyApplied(func(d *AuditDeviation, e *ledger.Event, s *collateral.SlashEvent) {
    log.Printf("Auditor %s penalized: violation=%v, slash=%v", d.AuditorID, e != nil, s != nil)
})

// 启动集成（自动注册 onAuditorDeviation 回调）
ai.Start()

// 之后，当审计完成时，偏离的审计者将自动被惩罚
```
