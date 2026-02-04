// Package task 提供委托任务管理功能
// Task 27: 委托任务与文件传输
package task

import (
	"time"
)

// TaskType 任务类型
type TaskType string

const (
	TaskTypeSearch   TaskType = "search"   // 搜索任务
	TaskTypeTransfer TaskType = "transfer" // 文件传输
	TaskTypeStorage  TaskType = "storage"  // 文件存储
	TaskTypeCompute  TaskType = "compute"  // 计算任务
	TaskTypeCustom   TaskType = "custom"   // 自定义任务
)

// TaskStatus 任务状态
type TaskStatus string

const (
	StatusDraft      TaskStatus = "draft"       // 草稿
	StatusPublished  TaskStatus = "published"   // 已发布
	StatusAccepted   TaskStatus = "accepted"    // 已接受
	StatusInProgress TaskStatus = "in_progress" // 执行中
	StatusDelivered  TaskStatus = "delivered"   // 已交付
	StatusVerified   TaskStatus = "verified"    // 已验收
	StatusSettled    TaskStatus = "settled"     // 已结算
	StatusCompleted  TaskStatus = "completed"   // 已完成
	StatusDisputed   TaskStatus = "disputed"    // 争议中
	StatusCancelled  TaskStatus = "cancelled"   // 已取消
	StatusExpired    TaskStatus = "expired"     // 已过期
)

// PublishMode 发布模式
type PublishMode string

const (
	ModeBroadcast       PublishMode = "broadcast"        // 广播到市场
	ModeDirect          PublishMode = "direct"           // 定向委托
	ModeCapabilityMatch PublishMode = "capability_match" // 能力匹配
)

// TaskRiskLevel 任务风险级别
type TaskRiskLevel string

const (
	RiskLow     TaskRiskLevel = "low"     // 低风险：搜索、查询
	RiskMedium  TaskRiskLevel = "medium"  // 中等风险：文件传输
	RiskHigh    TaskRiskLevel = "high"    // 高风险：计算任务
	RiskBlocked TaskRiskLevel = "blocked" // 禁止执行
)

// Task 委托任务
type Task struct {
	// 基本信息
	ID          string   `json:"id"`
	Type        TaskType `json:"type"`
	Title       string   `json:"title"`
	Description string   `json:"description"` // 可加密

	// 参与方
	RequesterID string `json:"requester_id"` // 委托方
	ExecutorID  string `json:"executor_id"`  // 执行方（接受后填入）

	// 奖励与押金
	Reward           float64 `json:"reward"`            // 声誉奖励
	RequesterDeposit float64 `json:"requester_deposit"` // 委托方押金
	ExecutorDeposit  float64 `json:"executor_deposit"`  // 执行方押金

	// 时间约束
	CreatedAt int64 `json:"created_at"`
	Deadline  int64 `json:"deadline"`   // 截止时间
	ExpiresAt int64 `json:"expires_at"` // 任务过期时间

	// 验收条件
	AcceptanceCriteria string `json:"acceptance_criteria"` // 验收标准
	DeliverableHash    string `json:"deliverable_hash"`    // 交付物哈希（可选）

	// 发布选项
	PublishMode       PublishMode `json:"publish_mode"`
	BiddingPeriod     int64       `json:"bidding_period"`      // 竞标期（秒）
	MaxExecutors      int         `json:"max_executors"`       // 最大执行者数
	RequiredCaps      []string    `json:"required_caps"`       // 要求的能力
	MinReputation     float64     `json:"min_reputation"`      // 最低声誉要求
	TargetExecutorID  string      `json:"target_executor_id"`  // 定向委托目标
	BiddingEndsAt     int64       `json:"bidding_ends_at"`     // 竞标截止时间

	// 状态
	Status TaskStatus `json:"status"`

	// 隐私保护
	IsEncrypted   bool   `json:"is_encrypted"`    // 描述是否加密
	PublicKeyHash string `json:"public_key_hash"` // 执行方公钥哈希（解密用）

	// 签名
	RequesterSig string `json:"requester_sig"`
	ExecutorSig  string `json:"executor_sig"`
	Nonce        string `json:"nonce"`

	// 竞标信息
	Bids []TaskBid `json:"bids,omitempty"`
}

// TaskBid 任务竞标
type TaskBid struct {
	TaskID        string   `json:"task_id"`
	BidderID      string   `json:"bidder_id"`
	BidAmount     float64  `json:"bid_amount"`     // 愿意接受的报酬
	EstimatedTime int64    `json:"estimated_time"` // 预估完成时间（秒）
	Capabilities  []string `json:"capabilities"`   // 自我声明的能力
	Reputation    float64  `json:"reputation"`     // 当前声誉
	Message       string   `json:"message"`        // 竞标理由
	Signature     string   `json:"signature"`
	BidTime       int64    `json:"bid_time"`
}

// TaskClaim 任务抢单
type TaskClaim struct {
	TaskID    string `json:"task_id"`
	ClaimerID string `json:"claimer_id"`
	ClaimTime int64  `json:"claim_time"`
	Signature string `json:"signature"`
}

// TaskAssignment 任务分配
type TaskAssignment struct {
	TaskID     string `json:"task_id"`
	AssignedTo string `json:"assigned_to"`
	AssignedAt int64  `json:"assigned_at"`
	Reason     string `json:"reason"` // 选择理由
	Signature  string `json:"signature"`
}

// DeliveryProof 交付证明
type DeliveryProof struct {
	TaskID          string `json:"task_id"`
	DeliverableHash string `json:"deliverable_hash"` // 交付物哈希

	// 执行方证明
	ExecutorSig  string `json:"executor_sig"` // B签名：我交付了这个
	DeliveryTime int64  `json:"delivery_time"`

	// 委托方确认
	RequesterSig string `json:"requester_sig"` // A签名：我收到了这个
	ReceiveTime  int64  `json:"receive_time"`

	// 第三方见证（可选，用于争议）
	WitnessSigs []string `json:"witness_sigs"`
}

// CommitReveal 承诺-揭示协议
type CommitReveal struct {
	TaskID string `json:"task_id"`

	// 阶段1: 执行方提交交付物哈希（不揭示内容）
	DeliverableCommit string `json:"deliverable_commit"` // Hash(交付物 + 随机数)
	CommitTimestamp   int64  `json:"commit_timestamp"`

	// 阶段2: 委托方确认收到（锁定资金）
	ReceivedAck  bool  `json:"received_ack"`
	AckTimestamp int64 `json:"ack_timestamp"`

	// 阶段3: 执行方揭示交付物
	DeliverableData []byte `json:"deliverable_data"` // 实际交付物
	RevealNonce     string `json:"reveal_nonce"`     // 随机数
	RevealTimestamp int64  `json:"reveal_timestamp"`

	// 阶段4: 验证匹配
	Verified bool `json:"verified"`
}

// AgentCapability Agent能力注册
type AgentCapability struct {
	AgentID       string   `json:"agent_id"`
	Capabilities  []string `json:"capabilities"` // ["coding", "search", "translation", "image_analysis"]
	Languages     []string `json:"languages"`    // ["zh", "en", "ja"]
	MaxConcurrent int      `json:"max_concurrent"`
	AvailableFrom int      `json:"available_from"` // 可用开始时间（小时，0-23）
	AvailableTo   int      `json:"available_to"`   // 可用结束时间（小时，0-23）
	UpdatedAt     int64    `json:"updated_at"`
}

// IsValid 检查任务是否有效（用于发布前验证）
func (t *Task) IsValid() bool {
	// ID 可以在发布时生成，所以不检查
	if t.RequesterID == "" {
		return false
	}
	if t.Title == "" || t.Type == "" {
		return false
	}
	if t.Reward < 0 {
		return false
	}
	if t.Deadline > 0 && t.Deadline < time.Now().Unix() {
		return false
	}
	return true
}

// IsComplete 检查任务数据是否完整（用于存储后验证）
func (t *Task) IsComplete() bool {
	return t.ID != "" && t.IsValid()
}

// IsExpired 检查任务是否已过期
func (t *Task) IsExpired() bool {
	if t.ExpiresAt > 0 {
		return time.Now().Unix() > t.ExpiresAt
	}
	if t.Deadline > 0 {
		return time.Now().Unix() > t.Deadline
	}
	return false
}

// IsBiddingOpen 检查竞标是否开放
func (t *Task) IsBiddingOpen() bool {
	if t.Status != StatusPublished {
		return false
	}
	if t.BiddingPeriod == 0 {
		return false // 不是竞标模式
	}
	if t.BiddingEndsAt > 0 {
		return time.Now().Unix() < t.BiddingEndsAt
	}
	return false
}

// CanTransition 检查是否可以转换到指定状态
func (t *Task) CanTransition(newStatus TaskStatus) bool {
	transitions := map[TaskStatus][]TaskStatus{
		StatusDraft:      {StatusPublished, StatusCancelled},
		StatusPublished:  {StatusAccepted, StatusExpired, StatusCancelled},
		StatusAccepted:   {StatusInProgress, StatusCancelled, StatusDisputed},
		StatusInProgress: {StatusDelivered, StatusCancelled, StatusDisputed},
		StatusDelivered:  {StatusVerified, StatusDisputed},
		StatusVerified:   {StatusSettled},
		StatusSettled:    {StatusCompleted},
		StatusDisputed:   {StatusVerified, StatusCancelled, StatusSettled},
	}

	allowed, exists := transitions[t.Status]
	if !exists {
		return false
	}

	for _, status := range allowed {
		if status == newStatus {
			return true
		}
	}
	return false
}

// GetRiskLevel 获取任务风险级别
func (t *Task) GetRiskLevel() TaskRiskLevel {
	switch t.Type {
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
