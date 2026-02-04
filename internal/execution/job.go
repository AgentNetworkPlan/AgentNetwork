// Package execution 提供任务执行引擎功能
// Task 28: 任务执行引擎
package execution

import (
	"context"
	"time"
)

// JobStatus 任务状态
type JobStatus string

const (
	JobPending   JobStatus = "pending"   // 等待执行
	JobQueued    JobStatus = "queued"    // 已入队
	JobRunning   JobStatus = "running"   // 执行中
	JobCompleted JobStatus = "completed" // 已完成
	JobFailed    JobStatus = "failed"    // 失败
	JobCancelled JobStatus = "cancelled" // 已取消
	JobTimeout   JobStatus = "timeout"   // 超时
)

// JobType 任务类型
type JobType string

const (
	JobTypeSearch  JobType = "search"  // 搜索任务
	JobTypeCompute JobType = "compute" // 计算任务
	JobTypeLLM     JobType = "llm"     // LLM任务
	JobTypeCustom  JobType = "custom"  // 自定义任务
)

// JobPriority 任务优先级
type JobPriority int

const (
	PriorityLow      JobPriority = 1
	PriorityNormal   JobPriority = 5
	PriorityHigh     JobPriority = 10
	PriorityCritical JobPriority = 100
)

// ExecutionJob 执行任务
type ExecutionJob struct {
	ID         string            `json:"id"`
	TaskID     string            `json:"task_id"`     // 关联的委托任务ID
	ExecutorID string            `json:"executor_id"` // 执行者节点ID
	Type       JobType           `json:"type"`        // 任务类型
	Priority   JobPriority       `json:"priority"`    // 优先级

	// 输入输出
	Input     map[string]any `json:"input"`     // 任务输入参数
	Output    map[string]any `json:"output"`    // 执行结果
	Artifacts []Artifact     `json:"artifacts"` // 产出文件/数据

	// 执行状态
	Status   JobStatus `json:"status"`
	Progress float64   `json:"progress"` // 0-100
	Message  string    `json:"message"`  // 状态消息
	Error    string    `json:"error"`    // 错误信息

	// 资源使用
	Resources ResourceUsage `json:"resources"`

	// 时间
	CreatedAt   int64 `json:"created_at"`
	StartedAt   int64 `json:"started_at"`
	CompletedAt int64 `json:"completed_at"`
	Timeout     int64 `json:"timeout"` // 超时时间（秒）

	// 重试
	RetryCount int `json:"retry_count"`
	MaxRetries int `json:"max_retries"`

	// 回调
	CallbackURL string `json:"callback_url,omitempty"` // 完成时回调地址
}

// Artifact 产出物
type Artifact struct {
	ID       string `json:"id"`
	Type     string `json:"type"` // file/data/hash
	Name     string `json:"name"`
	MimeType string `json:"mime_type"`
	Size     int64  `json:"size"`
	Hash     string `json:"hash"`     // SHA256
	Location string `json:"location"` // 存储位置
}

// ResourceUsage 资源使用情况
type ResourceUsage struct {
	CPUTimeMs   int64 `json:"cpu_time_ms"`   // CPU时间（毫秒）
	MemoryPeak  int64 `json:"memory_peak"`   // 内存峰值（字节）
	DiskRead    int64 `json:"disk_read"`     // 磁盘读取（字节）
	DiskWrite   int64 `json:"disk_write"`    // 磁盘写入（字节）
	NetworkIn   int64 `json:"network_in"`    // 网络接收（字节）
	NetworkOut  int64 `json:"network_out"`   // 网络发送（字节）
	DurationMs  int64 `json:"duration_ms"`   // 执行时长（毫秒）
}

// ResourceEstimate 资源估算
type ResourceEstimate struct {
	CPUTimeMs   int64 `json:"cpu_time_ms"`   // 预计CPU时间
	MemoryBytes int64 `json:"memory_bytes"`  // 预计内存使用
	DurationSec int64 `json:"duration_sec"`  // 预计执行时间
}

// ResourceLimit 资源限制
type ResourceLimit struct {
	MaxCPUTimeMs  int64 `json:"max_cpu_time_ms"`  // 最大CPU时间
	MaxMemoryMB   int64 `json:"max_memory_mb"`    // 最大内存（MB）
	MaxDiskMB     int64 `json:"max_disk_mb"`      // 最大磁盘（MB）
	MaxNetworkMB  int64 `json:"max_network_mb"`   // 最大网络（MB）
	MaxDurationMs int64 `json:"max_duration_ms"`  // 最大执行时间
}

// NewExecutionJob 创建新的执行任务
func NewExecutionJob(taskID string, jobType JobType, input map[string]any) *ExecutionJob {
	return &ExecutionJob{
		TaskID:     taskID,
		Type:       jobType,
		Priority:   PriorityNormal,
		Input:      input,
		Output:     make(map[string]any),
		Artifacts:  make([]Artifact, 0),
		Status:     JobPending,
		Progress:   0,
		CreatedAt:  time.Now().Unix(),
		Timeout:    300, // 默认5分钟超时
		MaxRetries: 3,
	}
}

// IsTerminal 检查任务是否处于终态
func (j *ExecutionJob) IsTerminal() bool {
	switch j.Status {
	case JobCompleted, JobFailed, JobCancelled, JobTimeout:
		return true
	default:
		return false
	}
}

// IsRunning 检查任务是否正在运行
func (j *ExecutionJob) IsRunning() bool {
	return j.Status == JobRunning
}

// CanRetry 检查任务是否可以重试
func (j *ExecutionJob) CanRetry() bool {
	return j.RetryCount < j.MaxRetries && j.Status == JobFailed
}

// Duration 获取执行时长
func (j *ExecutionJob) Duration() time.Duration {
	if j.StartedAt == 0 {
		return 0
	}
	endTime := j.CompletedAt
	if endTime == 0 {
		endTime = time.Now().Unix()
	}
	return time.Duration(endTime-j.StartedAt) * time.Second
}

// SetRunning 设置为运行中状态
func (j *ExecutionJob) SetRunning() {
	j.Status = JobRunning
	j.StartedAt = time.Now().Unix()
	j.Progress = 0
}

// SetCompleted 设置为完成状态
func (j *ExecutionJob) SetCompleted(output map[string]any, artifacts []Artifact) {
	j.Status = JobCompleted
	j.CompletedAt = time.Now().Unix()
	j.Progress = 100
	j.Output = output
	j.Artifacts = artifacts
	j.Resources.DurationMs = (j.CompletedAt - j.StartedAt) * 1000
}

// SetFailed 设置为失败状态
func (j *ExecutionJob) SetFailed(err string) {
	j.Status = JobFailed
	j.CompletedAt = time.Now().Unix()
	j.Error = err
	if j.StartedAt > 0 {
		j.Resources.DurationMs = (j.CompletedAt - j.StartedAt) * 1000
	}
}

// SetCancelled 设置为取消状态
func (j *ExecutionJob) SetCancelled() {
	j.Status = JobCancelled
	j.CompletedAt = time.Now().Unix()
	if j.StartedAt > 0 {
		j.Resources.DurationMs = (j.CompletedAt - j.StartedAt) * 1000
	}
}

// SetTimeout 设置为超时状态
func (j *ExecutionJob) SetTimeout() {
	j.Status = JobTimeout
	j.CompletedAt = time.Now().Unix()
	j.Error = "execution timeout"
	if j.StartedAt > 0 {
		j.Resources.DurationMs = (j.CompletedAt - j.StartedAt) * 1000
	}
}

// UpdateProgress 更新进度
func (j *ExecutionJob) UpdateProgress(progress float64, message string) {
	if progress < 0 {
		progress = 0
	}
	if progress > 100 {
		progress = 100
	}
	j.Progress = progress
	j.Message = message
}

// JobFilter 任务过滤器
type JobFilter struct {
	TaskID     string      `json:"task_id,omitempty"`
	ExecutorID string      `json:"executor_id,omitempty"`
	Type       JobType     `json:"type,omitempty"`
	Status     JobStatus   `json:"status,omitempty"`
	Priority   JobPriority `json:"priority,omitempty"`
	Limit      int         `json:"limit,omitempty"`
	Offset     int         `json:"offset,omitempty"`
}

// JobCallback 任务回调
type JobCallback func(job *ExecutionJob)

// Executor 执行器接口
type Executor interface {
	// 基础信息
	Name() string
	Version() string
	SupportedTypes() []JobType

	// 能力检查
	CanExecute(job *ExecutionJob) bool
	EstimateResources(job *ExecutionJob) (*ResourceEstimate, error)

	// 执行
	Execute(ctx context.Context, job *ExecutionJob) (*ExecutionResult, error)

	// 生命周期
	Initialize() error
	Shutdown() error
}

// ExecutionResult 执行结果
type ExecutionResult struct {
	Success   bool          `json:"success"`
	Output    map[string]any `json:"output"`
	Artifacts []Artifact    `json:"artifacts"`
	Error     string        `json:"error,omitempty"`
	Resources ResourceUsage `json:"resources"`
}

// NewSuccessResult 创建成功结果
func NewSuccessResult(output map[string]any, artifacts []Artifact) *ExecutionResult {
	return &ExecutionResult{
		Success:   true,
		Output:    output,
		Artifacts: artifacts,
	}
}

// NewErrorResult 创建错误结果
func NewErrorResult(err string) *ExecutionResult {
	return &ExecutionResult{
		Success: false,
		Error:   err,
	}
}

// ExecutorInfo 执行器信息
type ExecutorInfo struct {
	Name           string         `json:"name"`
	Version        string         `json:"version"`
	SupportedTypes []JobType      `json:"supported_types"`
	Status         string         `json:"status"` // ready/busy/error
	RunningJobs    int            `json:"running_jobs"`
	Limits         ResourceLimit  `json:"limits"`
}

// BaseExecutor 基础执行器（可嵌入其他执行器）
type BaseExecutor struct {
	name           string
	version        string
	supportedTypes []JobType
	initialized    bool
}

// NewBaseExecutor 创建基础执行器
func NewBaseExecutor(name, version string, types []JobType) *BaseExecutor {
	return &BaseExecutor{
		name:           name,
		version:        version,
		supportedTypes: types,
	}
}

// Name 返回执行器名称
func (b *BaseExecutor) Name() string {
	return b.name
}

// Version 返回版本
func (b *BaseExecutor) Version() string {
	return b.version
}

// SupportedTypes 返回支持的任务类型
func (b *BaseExecutor) SupportedTypes() []JobType {
	return b.supportedTypes
}

// CanExecute 默认实现：检查任务类型是否支持
func (b *BaseExecutor) CanExecute(job *ExecutionJob) bool {
	for _, t := range b.supportedTypes {
		if t == job.Type {
			return true
		}
	}
	return false
}

// EstimateResources 默认资源估算
func (b *BaseExecutor) EstimateResources(job *ExecutionJob) (*ResourceEstimate, error) {
	return &ResourceEstimate{
		CPUTimeMs:   1000,              // 默认1秒
		MemoryBytes: 64 * 1024 * 1024,  // 默认64MB
		DurationSec: 5,                 // 默认5秒
	}, nil
}

// Initialize 初始化
func (b *BaseExecutor) Initialize() error {
	b.initialized = true
	return nil
}

// Shutdown 关闭
func (b *BaseExecutor) Shutdown() error {
	b.initialized = false
	return nil
}

// IsInitialized 检查是否已初始化
func (b *BaseExecutor) IsInitialized() bool {
	return b.initialized
}
