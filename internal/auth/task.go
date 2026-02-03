package auth

import (
	"encoding/json"
	"errors"
	"fmt"
	"sync"
	"time"
)

// TaskStatus 任务状态
type TaskStatus string

const (
	TaskPending    TaskStatus = "pending"    // 待处理
	TaskAssigned   TaskStatus = "assigned"   // 已分配
	TaskInProgress TaskStatus = "in_progress" // 处理中
	TaskCompleted  TaskStatus = "completed"  // 已完成
	TaskFailed     TaskStatus = "failed"     // 失败
	TaskVerified   TaskStatus = "verified"   // 已验证
	TaskRejected   TaskStatus = "rejected"   // 被拒绝
)

// Task 任务定义
type Task struct {
	TaskID       string            `json:"task_id"`
	RequesterID  string            `json:"requester_id"`   // 发起者节点 ID
	WorkerID     string            `json:"worker_id"`      // 执行者节点 ID (分配后填写)
	TaskType     string            `json:"task_type"`      // 任务类型
	Payload      []byte            `json:"payload"`        // 任务数据
	Difficulty   int               `json:"difficulty"`     // 任务难度 (1-10)
	Status       TaskStatus        `json:"status"`
	CreatedAt    time.Time         `json:"created_at"`
	AssignedAt   *time.Time        `json:"assigned_at,omitempty"`
	CompletedAt  *time.Time        `json:"completed_at,omitempty"`
	Deadline     time.Time         `json:"deadline"`
	Metadata     map[string]string `json:"metadata,omitempty"`
	RequesterSig []byte            `json:"requester_sig"`  // 发起者签名
}

// ProofOfTask 任务完成证明
type ProofOfTask struct {
	TaskID          string    `json:"task_id"`
	WorkerID        string    `json:"worker_id"`
	Result          []byte    `json:"result"`           // 任务结果
	ResultHash      string    `json:"result_hash"`      // 结果哈希
	ExecutionTime   int64     `json:"execution_time"`   // 执行时间 (毫秒)
	IntermediateHash []string `json:"intermediate_hash"` // 中间状态哈希 (可选)
	Timestamp       time.Time `json:"timestamp"`
	WorkerSignature []byte    `json:"worker_signature"` // 执行者签名
}

// TaskVerification 任务验证结果
type TaskVerification struct {
	TaskID        string    `json:"task_id"`
	VerifierID    string    `json:"verifier_id"`    // 验证者节点 ID
	IsValid       bool      `json:"is_valid"`
	Reason        string    `json:"reason,omitempty"`
	Timestamp     time.Time `json:"timestamp"`
	VerifierSig   []byte    `json:"verifier_sig"`
}

// NewTask 创建新任务
func NewTask(requesterID, taskType string, payload []byte, difficulty int, deadline time.Time) *Task {
	taskID := generateID()
	return &Task{
		TaskID:      taskID,
		RequesterID: requesterID,
		TaskType:    taskType,
		Payload:     payload,
		Difficulty:  difficulty,
		Status:      TaskPending,
		CreatedAt:   time.Now(),
		Deadline:    deadline,
		Metadata:    make(map[string]string),
	}
}

// SignableData 返回任务的可签名数据
func (t *Task) SignableData() []byte {
	data, _ := json.Marshal(map[string]interface{}{
		"task_id":      t.TaskID,
		"requester_id": t.RequesterID,
		"task_type":    t.TaskType,
		"payload":      t.Payload,
		"difficulty":   t.Difficulty,
		"deadline":     t.Deadline.Unix(),
	})
	return data
}

// SignTask 发起者签名任务
func (t *Task) SignTask(identity *NodeIdentity) error {
	if identity.NodeID() != t.RequesterID {
		return errors.New("签名者不是任务发起者")
	}

	sig, err := identity.Sign(t.SignableData())
	if err != nil {
		return err
	}

	t.RequesterSig = sig
	return nil
}

// VerifyRequesterSignature 验证发起者签名
func (t *Task) VerifyRequesterSignature(pubKeyHex string) (bool, error) {
	if t.RequesterSig == nil {
		return false, errors.New("任务未签名")
	}
	return VerifyWithPublicKey(pubKeyHex, t.SignableData(), t.RequesterSig)
}

// AssignTo 分配任务给执行者
func (t *Task) AssignTo(workerID string) error {
	if t.Status != TaskPending {
		return fmt.Errorf("任务状态不允许分配: %s", t.Status)
	}
	
	now := time.Now()
	t.WorkerID = workerID
	t.Status = TaskAssigned
	t.AssignedAt = &now
	return nil
}

// IsExpired 检查任务是否过期
func (t *Task) IsExpired() bool {
	return time.Now().After(t.Deadline)
}

// NewProofOfTask 创建任务完成证明
func NewProofOfTask(taskID, workerID string, result []byte, executionTime int64) *ProofOfTask {
	hash := computeHash(result)
	return &ProofOfTask{
		TaskID:        taskID,
		WorkerID:      workerID,
		Result:        result,
		ResultHash:    hash,
		ExecutionTime: executionTime,
		Timestamp:     time.Now(),
	}
}

// SignableData 返回证明的可签名数据
func (p *ProofOfTask) SignableData() []byte {
	data, _ := json.Marshal(map[string]interface{}{
		"task_id":       p.TaskID,
		"worker_id":     p.WorkerID,
		"result_hash":   p.ResultHash,
		"execution_time": p.ExecutionTime,
		"timestamp":     p.Timestamp.Unix(),
	})
	return data
}

// Sign 执行者签名证明
func (p *ProofOfTask) Sign(identity *NodeIdentity) error {
	if identity.NodeID() != p.WorkerID {
		return errors.New("签名者不是任务执行者")
	}

	sig, err := identity.Sign(p.SignableData())
	if err != nil {
		return err
	}

	p.WorkerSignature = sig
	return nil
}

// VerifySignature 验证执行者签名
func (p *ProofOfTask) VerifySignature(pubKeyHex string) (bool, error) {
	if p.WorkerSignature == nil {
		return false, errors.New("证明未签名")
	}
	return VerifyWithPublicKey(pubKeyHex, p.SignableData(), p.WorkerSignature)
}

// AddIntermediateHash 添加中间状态哈希
func (p *ProofOfTask) AddIntermediateHash(data []byte) {
	hash := computeHash(data)
	p.IntermediateHash = append(p.IntermediateHash, hash)
}

// ========== 任务管理器 ==========

// TaskManager 任务管理器
type TaskManager struct {
	tasks      map[string]*Task
	proofs     map[string]*ProofOfTask
	verifications map[string][]*TaskVerification
	mu         sync.RWMutex
}

// NewTaskManager 创建任务管理器
func NewTaskManager() *TaskManager {
	return &TaskManager{
		tasks:         make(map[string]*Task),
		proofs:        make(map[string]*ProofOfTask),
		verifications: make(map[string][]*TaskVerification),
	}
}

// SubmitTask 提交任务
func (tm *TaskManager) SubmitTask(task *Task) error {
	tm.mu.Lock()
	defer tm.mu.Unlock()

	if task.RequesterSig == nil {
		return errors.New("任务必须签名")
	}

	if _, exists := tm.tasks[task.TaskID]; exists {
		return errors.New("任务已存在")
	}

	tm.tasks[task.TaskID] = task
	return nil
}

// GetTask 获取任务
func (tm *TaskManager) GetTask(taskID string) (*Task, error) {
	tm.mu.RLock()
	defer tm.mu.RUnlock()

	task, exists := tm.tasks[taskID]
	if !exists {
		return nil, errors.New("任务不存在")
	}
	return task, nil
}

// AssignTask 分配任务
func (tm *TaskManager) AssignTask(taskID, workerID string) error {
	tm.mu.Lock()
	defer tm.mu.Unlock()

	task, exists := tm.tasks[taskID]
	if !exists {
		return errors.New("任务不存在")
	}

	return task.AssignTo(workerID)
}

// SubmitProof 提交任务证明
func (tm *TaskManager) SubmitProof(proof *ProofOfTask) error {
	tm.mu.Lock()
	defer tm.mu.Unlock()

	task, exists := tm.tasks[proof.TaskID]
	if !exists {
		return errors.New("任务不存在")
	}

	if task.WorkerID != proof.WorkerID {
		return errors.New("执行者不匹配")
	}

	if proof.WorkerSignature == nil {
		return errors.New("证明必须签名")
	}

	// 更新任务状态
	now := time.Now()
	task.Status = TaskCompleted
	task.CompletedAt = &now

	tm.proofs[proof.TaskID] = proof
	return nil
}

// GetProof 获取任务证明
func (tm *TaskManager) GetProof(taskID string) (*ProofOfTask, error) {
	tm.mu.RLock()
	defer tm.mu.RUnlock()

	proof, exists := tm.proofs[taskID]
	if !exists {
		return nil, errors.New("证明不存在")
	}
	return proof, nil
}

// AddVerification 添加验证结果
func (tm *TaskManager) AddVerification(verification *TaskVerification) error {
	tm.mu.Lock()
	defer tm.mu.Unlock()

	task, exists := tm.tasks[verification.TaskID]
	if !exists {
		return errors.New("任务不存在")
	}

	if verification.VerifierSig == nil {
		return errors.New("验证结果必须签名")
	}

	tm.verifications[verification.TaskID] = append(
		tm.verifications[verification.TaskID],
		verification,
	)

	// 检查是否达到验证共识
	verifications := tm.verifications[verification.TaskID]
	validCount := 0
	for _, v := range verifications {
		if v.IsValid {
			validCount++
		}
	}

	// 简单多数决定 (可以根据需求调整)
	if len(verifications) >= 3 {
		if validCount > len(verifications)/2 {
			task.Status = TaskVerified
		} else {
			task.Status = TaskRejected
		}
	}

	return nil
}

// GetVerifications 获取任务的所有验证结果
func (tm *TaskManager) GetVerifications(taskID string) ([]*TaskVerification, error) {
	tm.mu.RLock()
	defer tm.mu.RUnlock()

	verifications, exists := tm.verifications[taskID]
	if !exists {
		return nil, errors.New("没有验证结果")
	}
	return verifications, nil
}

// GetPendingTasks 获取待处理的任务
func (tm *TaskManager) GetPendingTasks() []*Task {
	tm.mu.RLock()
	defer tm.mu.RUnlock()

	var pending []*Task
	for _, task := range tm.tasks {
		if task.Status == TaskPending && !task.IsExpired() {
			pending = append(pending, task)
		}
	}
	return pending
}

// GetTasksByWorker 获取指定执行者的任务
func (tm *TaskManager) GetTasksByWorker(workerID string) []*Task {
	tm.mu.RLock()
	defer tm.mu.RUnlock()

	var tasks []*Task
	for _, task := range tm.tasks {
		if task.WorkerID == workerID {
			tasks = append(tasks, task)
		}
	}
	return tasks
}

// GetTasksByRequester 获取指定发起者的任务
func (tm *TaskManager) GetTasksByRequester(requesterID string) []*Task {
	tm.mu.RLock()
	defer tm.mu.RUnlock()

	var tasks []*Task
	for _, task := range tm.tasks {
		if task.RequesterID == requesterID {
			tasks = append(tasks, task)
		}
	}
	return tasks
}
