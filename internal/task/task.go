// Package task 提供任务管理功能
package task

import (
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"time"
)

var (
	ErrTaskNotFound       = errors.New("task not found")
	ErrTaskExpired        = errors.New("task expired")
	ErrTaskAlreadyAssigned = errors.New("task already assigned")
	ErrInvalidTransition  = errors.New("invalid status transition")
	ErrInsufficientRep    = errors.New("insufficient reputation")
	ErrBiddingClosed      = errors.New("bidding is closed")
	ErrNotAssignedToMe    = errors.New("task not assigned to me")
	ErrInvalidProof       = errors.New("invalid delivery proof")
	ErrQuotaExceeded      = errors.New("task quota exceeded")
)

// TaskManagerConfig 任务管理器配置
type TaskManagerConfig struct {
	DataDir           string        // 数据目录
	DefaultBidding    time.Duration // 默认竞标期
	MaxTasksPerHour   int           // 每小时最大任务数（基础）
	ReputationBonus   float64       // 声誉加成系数
	MinRepToPublish   float64       // 发布任务最低声誉
	DepositMultiplier float64       // 押金倍数（相对于奖励）
	ResponseTimeout   time.Duration // 响应超时
}

// DefaultConfig 返回默认配置
func DefaultConfig() *TaskManagerConfig {
	return &TaskManagerConfig{
		DataDir:           "data/tasks",
		DefaultBidding:    10 * time.Minute,
		MaxTasksPerHour:   2,
		ReputationBonus:   0.05, // 每点声誉增加5%配额
		MinRepToPublish:   30.0,
		DepositMultiplier: 1.2, // 押金 = 奖励 * 1.2
		ResponseTimeout:   24 * time.Hour,
	}
}

// TaskManager 任务管理器
type TaskManager struct {
	mu     sync.RWMutex
	config *TaskManagerConfig

	// 任务存储
	tasks map[string]*Task // taskID -> task

	// 索引
	tasksByRequester map[string][]string // requesterID -> []taskID
	tasksByExecutor  map[string][]string // executorID -> []taskID
	tasksByStatus    map[TaskStatus][]string
	tasksByType      map[TaskType][]string

	// 能力注册表
	capabilities map[string]*AgentCapability // agentID -> capabilities
	capIndex     map[string][]string         // capability -> []agentID

	// 速率限制
	publishCount map[string]*rateLimitRecord // nodeID -> count

	// 交付证明
	deliveryProofs map[string]*DeliveryProof // taskID -> proof

	// 承诺-揭示
	commitReveals map[string]*CommitReveal // taskID -> commit-reveal
}

type rateLimitRecord struct {
	Count     int
	ResetTime time.Time
}

// NewTaskManager 创建任务管理器
func NewTaskManager(config *TaskManagerConfig) *TaskManager {
	if config == nil {
		config = DefaultConfig()
	}

	tm := &TaskManager{
		config:           config,
		tasks:            make(map[string]*Task),
		tasksByRequester: make(map[string][]string),
		tasksByExecutor:  make(map[string][]string),
		tasksByStatus:    make(map[TaskStatus][]string),
		tasksByType:      make(map[TaskType][]string),
		capabilities:     make(map[string]*AgentCapability),
		capIndex:         make(map[string][]string),
		publishCount:     make(map[string]*rateLimitRecord),
		deliveryProofs:   make(map[string]*DeliveryProof),
		commitReveals:    make(map[string]*CommitReveal),
	}

	// 尝试加载持久化数据
	tm.load()

	return tm
}

// PublishTask 发布任务
func (tm *TaskManager) PublishTask(task *Task, requesterRep float64) error {
	tm.mu.Lock()
	defer tm.mu.Unlock()

	// 检查声誉要求
	if requesterRep < tm.config.MinRepToPublish {
		return fmt.Errorf("%w: need %.1f, have %.1f", ErrInsufficientRep, tm.config.MinRepToPublish, requesterRep)
	}

	// 检查速率限制
	if !tm.checkAndUpdateQuota(task.RequesterID, requesterRep) {
		return ErrQuotaExceeded
	}

	// 验证任务
	if !task.IsValid() {
		return errors.New("invalid task")
	}

	// 生成ID
	if task.ID == "" {
		task.ID = tm.generateID()
	}

	// 设置默认值
	task.CreatedAt = time.Now().Unix()
	task.Status = StatusPublished

	// 设置竞标截止时间
	if task.BiddingPeriod > 0 {
		task.BiddingEndsAt = time.Now().Unix() + task.BiddingPeriod
	} else if task.PublishMode == ModeBroadcast {
		// 广播模式默认竞标期
		task.BiddingPeriod = int64(tm.config.DefaultBidding.Seconds())
		task.BiddingEndsAt = time.Now().Unix() + task.BiddingPeriod
	}

	// 计算押金
	if task.RequesterDeposit == 0 {
		task.RequesterDeposit = task.Reward * tm.config.DepositMultiplier
	}

	// 存储
	tm.tasks[task.ID] = task
	tm.addToIndex(task)

	// 持久化
	tm.save()

	return nil
}

// SubmitBid 提交竞标
func (tm *TaskManager) SubmitBid(bid *TaskBid) error {
	tm.mu.Lock()
	defer tm.mu.Unlock()

	task, exists := tm.tasks[bid.TaskID]
	if !exists {
		return ErrTaskNotFound
	}

	if !task.IsBiddingOpen() {
		return ErrBiddingClosed
	}

	// 检查声誉要求
	if task.MinReputation > 0 && bid.Reputation < task.MinReputation {
		return fmt.Errorf("%w: need %.1f, have %.1f", ErrInsufficientRep, task.MinReputation, bid.Reputation)
	}

	// 设置时间
	bid.BidTime = time.Now().Unix()

	// 添加竞标
	task.Bids = append(task.Bids, *bid)

	tm.save()
	return nil
}

// ClaimTask 抢单（非竞标模式）
func (tm *TaskManager) ClaimTask(claim *TaskClaim, claimerRep float64) error {
	tm.mu.Lock()
	defer tm.mu.Unlock()

	task, exists := tm.tasks[claim.TaskID]
	if !exists {
		return ErrTaskNotFound
	}

	if task.Status != StatusPublished {
		return ErrTaskAlreadyAssigned
	}

	// 竞标模式不允许抢单
	if task.BiddingPeriod > 0 {
		return errors.New("bidding mode, cannot claim directly")
	}

	// 定向委托检查
	if task.PublishMode == ModeDirect && task.TargetExecutorID != claim.ClaimerID {
		return errors.New("task is targeted to another agent")
	}

	// 检查声誉
	if task.MinReputation > 0 && claimerRep < task.MinReputation {
		return fmt.Errorf("%w: need %.1f, have %.1f", ErrInsufficientRep, task.MinReputation, claimerRep)
	}

	// 分配任务
	task.ExecutorID = claim.ClaimerID
	task.Status = StatusAccepted

	tm.addExecutorIndex(task)
	tm.save()

	return nil
}

// AssignTask 分配任务（竞标模式下由发布者选择）
func (tm *TaskManager) AssignTask(assignment *TaskAssignment) error {
	tm.mu.Lock()
	defer tm.mu.Unlock()

	task, exists := tm.tasks[assignment.TaskID]
	if !exists {
		return ErrTaskNotFound
	}

	if task.Status != StatusPublished {
		return ErrTaskAlreadyAssigned
	}

	if task.ExecutorID != "" {
		return ErrTaskAlreadyAssigned
	}

	// 分配
	task.ExecutorID = assignment.AssignedTo
	task.Status = StatusAccepted

	tm.addExecutorIndex(task)
	tm.save()

	return nil
}

// StartExecution 开始执行
func (tm *TaskManager) StartExecution(taskID, executorID string) error {
	tm.mu.Lock()
	defer tm.mu.Unlock()

	task, exists := tm.tasks[taskID]
	if !exists {
		return ErrTaskNotFound
	}

	if task.ExecutorID != executorID {
		return ErrNotAssignedToMe
	}

	if !task.CanTransition(StatusInProgress) {
		return ErrInvalidTransition
	}

	task.Status = StatusInProgress
	tm.save()

	return nil
}

// SubmitDelivery 提交交付
func (tm *TaskManager) SubmitDelivery(taskID, executorID, deliverableHash, signature string) error {
	tm.mu.Lock()
	defer tm.mu.Unlock()

	task, exists := tm.tasks[taskID]
	if !exists {
		return ErrTaskNotFound
	}

	if task.ExecutorID != executorID {
		return ErrNotAssignedToMe
	}

	if !task.CanTransition(StatusDelivered) {
		return ErrInvalidTransition
	}

	// 创建交付证明
	proof := &DeliveryProof{
		TaskID:          taskID,
		DeliverableHash: deliverableHash,
		ExecutorSig:     signature,
		DeliveryTime:    time.Now().Unix(),
	}

	tm.deliveryProofs[taskID] = proof
	task.DeliverableHash = deliverableHash
	task.Status = StatusDelivered

	tm.save()
	return nil
}

// ConfirmDelivery 确认收到交付
func (tm *TaskManager) ConfirmDelivery(taskID, requesterID, signature string) error {
	tm.mu.Lock()
	defer tm.mu.Unlock()

	task, exists := tm.tasks[taskID]
	if !exists {
		return ErrTaskNotFound
	}

	if task.RequesterID != requesterID {
		return errors.New("not the task requester")
	}

	if !task.CanTransition(StatusVerified) {
		return ErrInvalidTransition
	}

	// 更新交付证明
	proof, exists := tm.deliveryProofs[taskID]
	if !exists {
		return ErrInvalidProof
	}

	proof.RequesterSig = signature
	proof.ReceiveTime = time.Now().Unix()

	task.ExecutorSig = signature
	task.Status = StatusVerified

	tm.save()
	return nil
}

// SettleTask 结算任务
func (tm *TaskManager) SettleTask(taskID string) (*SettlementResult, error) {
	tm.mu.Lock()
	defer tm.mu.Unlock()

	task, exists := tm.tasks[taskID]
	if !exists {
		return nil, ErrTaskNotFound
	}

	if !task.CanTransition(StatusSettled) {
		return nil, ErrInvalidTransition
	}

	result := &SettlementResult{
		TaskID:        taskID,
		RequesterID:   task.RequesterID,
		ExecutorID:    task.ExecutorID,
		RewardAmount:  task.Reward,
		DepositReturn: task.RequesterDeposit,
		SettledAt:     time.Now().Unix(),
	}

	task.Status = StatusSettled
	tm.save()

	return result, nil
}

// CompleteTask 完成任务（归档）
func (tm *TaskManager) CompleteTask(taskID string) error {
	tm.mu.Lock()
	defer tm.mu.Unlock()

	task, exists := tm.tasks[taskID]
	if !exists {
		return ErrTaskNotFound
	}

	if !task.CanTransition(StatusCompleted) {
		return ErrInvalidTransition
	}

	task.Status = StatusCompleted
	tm.save()

	return nil
}

// DisputeTask 发起争议
func (tm *TaskManager) DisputeTask(taskID, disputerID, reason string) error {
	tm.mu.Lock()
	defer tm.mu.Unlock()

	task, exists := tm.tasks[taskID]
	if !exists {
		return ErrTaskNotFound
	}

	// 只有参与方可以发起争议
	if task.RequesterID != disputerID && task.ExecutorID != disputerID {
		return errors.New("only participants can dispute")
	}

	if !task.CanTransition(StatusDisputed) {
		return ErrInvalidTransition
	}

	task.Status = StatusDisputed
	tm.save()

	return nil
}

// CancelTask 取消任务
func (tm *TaskManager) CancelTask(taskID, requesterID string) error {
	tm.mu.Lock()
	defer tm.mu.Unlock()

	task, exists := tm.tasks[taskID]
	if !exists {
		return ErrTaskNotFound
	}

	if task.RequesterID != requesterID {
		return errors.New("only requester can cancel")
	}

	if !task.CanTransition(StatusCancelled) {
		return ErrInvalidTransition
	}

	task.Status = StatusCancelled
	tm.save()

	return nil
}

// GetTask 获取任务
func (tm *TaskManager) GetTask(taskID string) (*Task, error) {
	tm.mu.RLock()
	defer tm.mu.RUnlock()

	task, exists := tm.tasks[taskID]
	if !exists {
		return nil, ErrTaskNotFound
	}
	return task, nil
}

// GetTasksByRequester 获取发布者的任务
func (tm *TaskManager) GetTasksByRequester(requesterID string) []*Task {
	tm.mu.RLock()
	defer tm.mu.RUnlock()

	ids := tm.tasksByRequester[requesterID]
	tasks := make([]*Task, 0, len(ids))
	for _, id := range ids {
		if task, exists := tm.tasks[id]; exists {
			tasks = append(tasks, task)
		}
	}
	return tasks
}

// GetTasksByExecutor 获取执行者的任务
func (tm *TaskManager) GetTasksByExecutor(executorID string) []*Task {
	tm.mu.RLock()
	defer tm.mu.RUnlock()

	ids := tm.tasksByExecutor[executorID]
	tasks := make([]*Task, 0, len(ids))
	for _, id := range ids {
		if task, exists := tm.tasks[id]; exists {
			tasks = append(tasks, task)
		}
	}
	return tasks
}

// GetTasksByStatus 获取指定状态的任务
func (tm *TaskManager) GetTasksByStatus(status TaskStatus) []*Task {
	tm.mu.RLock()
	defer tm.mu.RUnlock()

	ids := tm.tasksByStatus[status]
	tasks := make([]*Task, 0, len(ids))
	for _, id := range ids {
		if task, exists := tm.tasks[id]; exists {
			tasks = append(tasks, task)
		}
	}
	return tasks
}

// GetOpenTasks 获取开放的任务（可接单）
func (tm *TaskManager) GetOpenTasks() []*Task {
	return tm.GetTasksByStatus(StatusPublished)
}

// GetDeliveryProof 获取交付证明
func (tm *TaskManager) GetDeliveryProof(taskID string) (*DeliveryProof, error) {
	tm.mu.RLock()
	defer tm.mu.RUnlock()

	proof, exists := tm.deliveryProofs[taskID]
	if !exists {
		return nil, ErrInvalidProof
	}
	return proof, nil
}

// RegisterCapability 注册Agent能力
func (tm *TaskManager) RegisterCapability(cap *AgentCapability) {
	tm.mu.Lock()
	defer tm.mu.Unlock()

	cap.UpdatedAt = time.Now().Unix()
	tm.capabilities[cap.AgentID] = cap

	// 更新能力索引
	for _, c := range cap.Capabilities {
		tm.capIndex[c] = append(tm.capIndex[c], cap.AgentID)
	}

	tm.save()
}

// FindAgentsByCapability 根据能力查找Agent
func (tm *TaskManager) FindAgentsByCapability(capabilities []string) []string {
	tm.mu.RLock()
	defer tm.mu.RUnlock()

	// 统计每个agent匹配的能力数
	matchCount := make(map[string]int)
	for _, cap := range capabilities {
		for _, agentID := range tm.capIndex[cap] {
			matchCount[agentID]++
		}
	}

	// 返回全部匹配的agent
	var result []string
	for agentID, count := range matchCount {
		if count == len(capabilities) {
			result = append(result, agentID)
		}
	}

	return result
}

// CheckExpiredTasks 检查并更新过期任务
func (tm *TaskManager) CheckExpiredTasks() []string {
	tm.mu.Lock()
	defer tm.mu.Unlock()

	var expired []string
	now := time.Now().Unix()

	for id, task := range tm.tasks {
		if task.Status == StatusPublished && task.ExpiresAt > 0 && now > task.ExpiresAt {
			task.Status = StatusExpired
			expired = append(expired, id)
		}
	}

	if len(expired) > 0 {
		tm.save()
	}

	return expired
}

// GetStatistics 获取统计信息
func (tm *TaskManager) GetStatistics() *TaskStatistics {
	tm.mu.RLock()
	defer tm.mu.RUnlock()

	stats := &TaskStatistics{
		TotalTasks: len(tm.tasks),
		ByStatus:   make(map[TaskStatus]int),
		ByType:     make(map[TaskType]int),
	}

	for _, task := range tm.tasks {
		stats.ByStatus[task.Status]++
		stats.ByType[task.Type]++
	}

	return stats
}

// SettlementResult 结算结果
type SettlementResult struct {
	TaskID        string
	RequesterID   string
	ExecutorID    string
	RewardAmount  float64
	DepositReturn float64
	SettledAt     int64
}

// TaskStatistics 任务统计
type TaskStatistics struct {
	TotalTasks int
	ByStatus   map[TaskStatus]int
	ByType     map[TaskType]int
}

// 内部方法

func (tm *TaskManager) generateID() string {
	bytes := make([]byte, 16)
	rand.Read(bytes)
	return "task_" + hex.EncodeToString(bytes)
}

func (tm *TaskManager) checkAndUpdateQuota(nodeID string, reputation float64) bool {
	record, exists := tm.publishCount[nodeID]
	if !exists {
		record = &rateLimitRecord{
			Count:     0,
			ResetTime: time.Now().Add(time.Hour),
		}
		tm.publishCount[nodeID] = record
	}

	// 检查是否需要重置
	if time.Now().After(record.ResetTime) {
		record.Count = 0
		record.ResetTime = time.Now().Add(time.Hour)
	}

	// 计算配额
	quota := tm.config.MaxTasksPerHour + int(reputation*tm.config.ReputationBonus)
	if record.Count >= quota {
		return false
	}

	record.Count++
	return true
}

func (tm *TaskManager) addToIndex(task *Task) {
	tm.tasksByRequester[task.RequesterID] = append(tm.tasksByRequester[task.RequesterID], task.ID)
	tm.tasksByStatus[task.Status] = append(tm.tasksByStatus[task.Status], task.ID)
	tm.tasksByType[task.Type] = append(tm.tasksByType[task.Type], task.ID)
}

func (tm *TaskManager) addExecutorIndex(task *Task) {
	if task.ExecutorID != "" {
		tm.tasksByExecutor[task.ExecutorID] = append(tm.tasksByExecutor[task.ExecutorID], task.ID)
	}
}

func (tm *TaskManager) load() {
	filePath := filepath.Join(tm.config.DataDir, "tasks.json")
	data, err := os.ReadFile(filePath)
	if err != nil {
		return
	}

	var stored struct {
		Tasks        map[string]*Task            `json:"tasks"`
		Capabilities map[string]*AgentCapability `json:"capabilities"`
		Proofs       map[string]*DeliveryProof   `json:"proofs"`
	}

	if err := json.Unmarshal(data, &stored); err != nil {
		return
	}

	if stored.Tasks != nil {
		tm.tasks = stored.Tasks
		// 重建索引
		for _, task := range tm.tasks {
			tm.addToIndex(task)
			tm.addExecutorIndex(task)
		}
	}

	if stored.Capabilities != nil {
		tm.capabilities = stored.Capabilities
		for _, cap := range tm.capabilities {
			for _, c := range cap.Capabilities {
				tm.capIndex[c] = append(tm.capIndex[c], cap.AgentID)
			}
		}
	}

	if stored.Proofs != nil {
		tm.deliveryProofs = stored.Proofs
	}
}

func (tm *TaskManager) save() {
	if err := os.MkdirAll(tm.config.DataDir, 0755); err != nil {
		return
	}

	stored := struct {
		Tasks        map[string]*Task            `json:"tasks"`
		Capabilities map[string]*AgentCapability `json:"capabilities"`
		Proofs       map[string]*DeliveryProof   `json:"proofs"`
	}{
		Tasks:        tm.tasks,
		Capabilities: tm.capabilities,
		Proofs:       tm.deliveryProofs,
	}

	data, err := json.MarshalIndent(stored, "", "  ")
	if err != nil {
		return
	}

	filePath := filepath.Join(tm.config.DataDir, "tasks.json")
	os.WriteFile(filePath, data, 0644)
}
