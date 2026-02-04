// Package execution 提供任务执行引擎功能
package execution

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"errors"
	"sync"
	"time"
)

var (
	ErrQueueFull     = errors.New("job queue is full")
	ErrJobNotFound   = errors.New("job not found")
	ErrEngineNotRunning = errors.New("engine is not running")
	ErrEngineStopped = errors.New("engine has been stopped")
)

// EngineConfig 引擎配置
type EngineConfig struct {
	MaxConcurrent  int           // 最大并发任务数
	QueueSize      int           // 队列最大大小
	DefaultTimeout time.Duration // 默认超时时间
	CheckInterval  time.Duration // 状态检查间隔
	WorkerCount    int           // 工作者数量
}

// DefaultEngineConfig 默认配置
func DefaultEngineConfig() *EngineConfig {
	return &EngineConfig{
		MaxConcurrent:  10,
		QueueSize:      1000,
		DefaultTimeout: 5 * time.Minute,
		CheckInterval:  1 * time.Second,
		WorkerCount:    5,
	}
}

// Engine 执行引擎
type Engine struct {
	mu       sync.RWMutex
	config   *EngineConfig
	registry *ExecutorRegistry
	queue    *PriorityQueue

	// 任务存储
	jobs       map[string]*ExecutionJob // jobID -> job
	runningJobs map[string]struct{}     // 运行中的任务ID

	// 工作控制
	workers   []*worker
	jobChan   chan *ExecutionJob
	resultCh  chan *jobResult
	stopChan  chan struct{}
	running   bool

	// 回调
	callbacks []JobCallback

	// 指标
	metrics *EngineMetrics
}

type jobResult struct {
	job    *ExecutionJob
	result *ExecutionResult
	err    error
}

// EngineMetrics 引擎指标
type EngineMetrics struct {
	mu              sync.RWMutex
	TotalSubmitted  int64 `json:"total_submitted"`
	TotalCompleted  int64 `json:"total_completed"`
	TotalFailed     int64 `json:"total_failed"`
	TotalCancelled  int64 `json:"total_cancelled"`
	TotalTimeout    int64 `json:"total_timeout"`
	CurrentQueued   int   `json:"current_queued"`
	CurrentRunning  int   `json:"current_running"`
}

// NewEngine 创建执行引擎
func NewEngine(config *EngineConfig) *Engine {
	if config == nil {
		config = DefaultEngineConfig()
	}

	return &Engine{
		config:      config,
		registry:    NewExecutorRegistry(),
		queue:       NewPriorityQueue(),
		jobs:        make(map[string]*ExecutionJob),
		runningJobs: make(map[string]struct{}),
		jobChan:     make(chan *ExecutionJob, config.MaxConcurrent),
		resultCh:    make(chan *jobResult, config.MaxConcurrent),
		stopChan:    make(chan struct{}),
		callbacks:   make([]JobCallback, 0),
		metrics:     &EngineMetrics{},
	}
}

// Start 启动引擎
func (e *Engine) Start() error {
	e.mu.Lock()
	defer e.mu.Unlock()

	if e.running {
		return nil
	}

	e.stopChan = make(chan struct{})
	e.running = true

	// 启动工作者
	e.workers = make([]*worker, e.config.WorkerCount)
	for i := 0; i < e.config.WorkerCount; i++ {
		w := &worker{
			id:       i,
			engine:   e,
			stopChan: e.stopChan,
		}
		e.workers[i] = w
		go w.run()
	}

	// 启动调度器
	go e.scheduler()

	// 启动结果处理器
	go e.resultHandler()

	return nil
}

// Stop 停止引擎
func (e *Engine) Stop() error {
	e.mu.Lock()
	defer e.mu.Unlock()

	if !e.running {
		return nil
	}

	close(e.stopChan)
	e.running = false

	// 关闭所有执行器
	e.registry.ShutdownAll()

	return nil
}

// IsRunning 检查引擎是否运行中
func (e *Engine) IsRunning() bool {
	e.mu.RLock()
	defer e.mu.RUnlock()
	return e.running
}

// RegisterExecutor 注册执行器
func (e *Engine) RegisterExecutor(executor Executor) error {
	return e.registry.Register(executor)
}

// Submit 提交任务
func (e *Engine) Submit(job *ExecutionJob) error {
	e.mu.Lock()
	defer e.mu.Unlock()

	if !e.running {
		return ErrEngineNotRunning
	}

	// 检查队列大小
	if e.queue.Size() >= e.config.QueueSize {
		return ErrQueueFull
	}

	// 生成ID
	if job.ID == "" {
		job.ID = e.generateID()
	}

	// 设置默认超时
	if job.Timeout == 0 {
		job.Timeout = int64(e.config.DefaultTimeout.Seconds())
	}

	// 存储任务
	e.jobs[job.ID] = job

	// 入队
	e.queue.Enqueue(job)

	// 更新指标
	e.metrics.mu.Lock()
	e.metrics.TotalSubmitted++
	e.metrics.CurrentQueued = e.queue.Size()
	e.metrics.mu.Unlock()

	return nil
}

// Cancel 取消任务
func (e *Engine) Cancel(jobID string) error {
	e.mu.Lock()
	defer e.mu.Unlock()

	job, exists := e.jobs[jobID]
	if !exists {
		return ErrJobNotFound
	}

	// 如果在队列中，直接移除
	if e.queue.Contains(jobID) {
		e.queue.Remove(jobID)
		job.SetCancelled()
		e.notifyCallbacks(job)
		
		e.metrics.mu.Lock()
		e.metrics.TotalCancelled++
		e.metrics.CurrentQueued = e.queue.Size()
		e.metrics.mu.Unlock()
		
		return nil
	}

	// 如果正在运行，标记为取消（工作者会检查）
	if _, running := e.runningJobs[jobID]; running {
		job.Status = JobCancelled
		return nil
	}

	// 已完成的任务不能取消
	if job.IsTerminal() {
		return errors.New("cannot cancel terminal job")
	}

	return nil
}

// GetJob 获取任务
func (e *Engine) GetJob(jobID string) (*ExecutionJob, error) {
	e.mu.RLock()
	defer e.mu.RUnlock()

	job, exists := e.jobs[jobID]
	if !exists {
		return nil, ErrJobNotFound
	}
	return job, nil
}

// ListJobs 列出任务
func (e *Engine) ListJobs(filter *JobFilter) []*ExecutionJob {
	e.mu.RLock()
	defer e.mu.RUnlock()

	result := make([]*ExecutionJob, 0)
	for _, job := range e.jobs {
		if e.matchFilter(job, filter) {
			result = append(result, job)
		}
	}

	// 应用分页
	if filter != nil && filter.Limit > 0 {
		start := filter.Offset
		if start >= len(result) {
			return []*ExecutionJob{}
		}
		end := start + filter.Limit
		if end > len(result) {
			end = len(result)
		}
		result = result[start:end]
	}

	return result
}

// AddCallback 添加回调
func (e *Engine) AddCallback(callback JobCallback) {
	e.mu.Lock()
	defer e.mu.Unlock()
	e.callbacks = append(e.callbacks, callback)
}

// GetMetrics 获取指标
func (e *Engine) GetMetrics() *EngineMetrics {
	e.metrics.mu.RLock()
	defer e.metrics.mu.RUnlock()
	
	return &EngineMetrics{
		TotalSubmitted: e.metrics.TotalSubmitted,
		TotalCompleted: e.metrics.TotalCompleted,
		TotalFailed:    e.metrics.TotalFailed,
		TotalCancelled: e.metrics.TotalCancelled,
		TotalTimeout:   e.metrics.TotalTimeout,
		CurrentQueued:  e.metrics.CurrentQueued,
		CurrentRunning: e.metrics.CurrentRunning,
	}
}

// scheduler 调度器
func (e *Engine) scheduler() {
	ticker := time.NewTicker(100 * time.Millisecond)
	defer ticker.Stop()

	for {
		select {
		case <-e.stopChan:
			return
		case <-ticker.C:
			e.scheduleJobs()
		}
	}
}

// scheduleJobs 调度任务
func (e *Engine) scheduleJobs() {
	e.mu.Lock()
	defer e.mu.Unlock()

	// 检查是否可以调度更多任务
	for len(e.runningJobs) < e.config.MaxConcurrent {
		job := e.queue.Dequeue()
		if job == nil {
			break
		}

		// 查找执行器
		executor, err := e.registry.FindExecutor(job)
		if err != nil {
			job.SetFailed("no executor available: " + err.Error())
			e.notifyCallbacks(job)
			
			e.metrics.mu.Lock()
			e.metrics.TotalFailed++
			e.metrics.mu.Unlock()
			continue
		}

		// 标记为运行中
		e.runningJobs[job.ID] = struct{}{}
		job.SetRunning()

		e.metrics.mu.Lock()
		e.metrics.CurrentQueued = e.queue.Size()
		e.metrics.CurrentRunning = len(e.runningJobs)
		e.metrics.mu.Unlock()

		// 发送到工作者
		select {
		case e.jobChan <- job:
			// 存储执行器引用用于后续执行
			go e.executeJob(job, executor)
		default:
			// 工作者通道满了，放回队列
			delete(e.runningJobs, job.ID)
			job.Status = JobQueued
			e.queue.Enqueue(job)
		}
	}
}

// executeJob 执行任务
func (e *Engine) executeJob(job *ExecutionJob, executor Executor) {
	// 创建带超时的上下文
	ctx, cancel := context.WithTimeout(context.Background(), time.Duration(job.Timeout)*time.Second)
	defer cancel()

	// 执行任务
	result, err := executor.Execute(ctx, job)

	// 发送结果
	e.resultCh <- &jobResult{
		job:    job,
		result: result,
		err:    err,
	}
}

// resultHandler 结果处理器
func (e *Engine) resultHandler() {
	for {
		select {
		case <-e.stopChan:
			return
		case jr := <-e.resultCh:
			e.handleResult(jr)
		}
	}
}

// handleResult 处理结果
func (e *Engine) handleResult(jr *jobResult) {
	e.mu.Lock()
	defer e.mu.Unlock()

	job := jr.job
	delete(e.runningJobs, job.ID)

	if jr.err != nil {
		// 检查是否超时
		if errors.Is(jr.err, context.DeadlineExceeded) {
			job.SetTimeout()
			e.metrics.mu.Lock()
			e.metrics.TotalTimeout++
			e.metrics.mu.Unlock()
		} else if job.CanRetry() {
			// 重试
			job.RetryCount++
			job.Status = JobPending
			e.queue.Enqueue(job)
			return
		} else {
			job.SetFailed(jr.err.Error())
			e.metrics.mu.Lock()
			e.metrics.TotalFailed++
			e.metrics.mu.Unlock()
		}
	} else if jr.result != nil {
		if jr.result.Success {
			job.SetCompleted(jr.result.Output, jr.result.Artifacts)
			job.Resources = jr.result.Resources
			e.metrics.mu.Lock()
			e.metrics.TotalCompleted++
			e.metrics.mu.Unlock()
		} else {
			if job.CanRetry() {
				job.RetryCount++
				job.Status = JobPending
				e.queue.Enqueue(job)
				return
			}
			job.SetFailed(jr.result.Error)
			e.metrics.mu.Lock()
			e.metrics.TotalFailed++
			e.metrics.mu.Unlock()
		}
	}

	e.metrics.mu.Lock()
	e.metrics.CurrentRunning = len(e.runningJobs)
	e.metrics.mu.Unlock()

	e.notifyCallbacks(job)
}

// notifyCallbacks 通知回调
func (e *Engine) notifyCallbacks(job *ExecutionJob) {
	for _, cb := range e.callbacks {
		go cb(job)
	}
}

// matchFilter 匹配过滤器
func (e *Engine) matchFilter(job *ExecutionJob, filter *JobFilter) bool {
	if filter == nil {
		return true
	}
	if filter.TaskID != "" && job.TaskID != filter.TaskID {
		return false
	}
	if filter.ExecutorID != "" && job.ExecutorID != filter.ExecutorID {
		return false
	}
	if filter.Type != "" && job.Type != filter.Type {
		return false
	}
	if filter.Status != "" && job.Status != filter.Status {
		return false
	}
	if filter.Priority > 0 && job.Priority != filter.Priority {
		return false
	}
	return true
}

// generateID 生成ID
func (e *Engine) generateID() string {
	b := make([]byte, 16)
	rand.Read(b)
	return "job_" + hex.EncodeToString(b)
}

// worker 工作者
type worker struct {
	id       int
	engine   *Engine
	stopChan <-chan struct{}
}

// run 工作者运行循环
func (w *worker) run() {
	for {
		select {
		case <-w.stopChan:
			return
		case <-w.engine.jobChan:
			// 任务已经在 executeJob 中处理
		}
	}
}
