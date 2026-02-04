// Package execution 提供任务执行引擎功能
package execution

import (
	"errors"
	"sync"
)

var (
	ErrExecutorExists    = errors.New("executor already registered")
	ErrExecutorNotFound  = errors.New("executor not found")
	ErrNoExecutorForType = errors.New("no executor available for job type")
)

// ExecutorRegistry 执行器注册表
type ExecutorRegistry struct {
	mu        sync.RWMutex
	executors map[string]Executor     // name -> executor
	typeIndex map[JobType][]string    // type -> []executor names
}

// NewExecutorRegistry 创建执行器注册表
func NewExecutorRegistry() *ExecutorRegistry {
	return &ExecutorRegistry{
		executors: make(map[string]Executor),
		typeIndex: make(map[JobType][]string),
	}
}

// Register 注册执行器
func (r *ExecutorRegistry) Register(executor Executor) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	name := executor.Name()
	if _, exists := r.executors[name]; exists {
		return ErrExecutorExists
	}

	// 初始化执行器
	if err := executor.Initialize(); err != nil {
		return err
	}

	r.executors[name] = executor

	// 更新类型索引
	for _, t := range executor.SupportedTypes() {
		r.typeIndex[t] = append(r.typeIndex[t], name)
	}

	return nil
}

// Unregister 注销执行器
func (r *ExecutorRegistry) Unregister(name string) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	executor, exists := r.executors[name]
	if !exists {
		return ErrExecutorNotFound
	}

	// 关闭执行器
	if err := executor.Shutdown(); err != nil {
		return err
	}

	// 从类型索引中移除
	for _, t := range executor.SupportedTypes() {
		names := r.typeIndex[t]
		for i, n := range names {
			if n == name {
				r.typeIndex[t] = append(names[:i], names[i+1:]...)
				break
			}
		}
	}

	delete(r.executors, name)
	return nil
}

// Get 获取指定名称的执行器
func (r *ExecutorRegistry) Get(name string) (Executor, bool) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	executor, exists := r.executors[name]
	return executor, exists
}

// GetForType 获取支持指定类型的执行器
func (r *ExecutorRegistry) GetForType(jobType JobType) (Executor, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	names, exists := r.typeIndex[jobType]
	if !exists || len(names) == 0 {
		return nil, ErrNoExecutorForType
	}

	// 返回第一个可用的执行器
	// TODO: 可以实现更复杂的选择策略（负载均衡、能力匹配等）
	return r.executors[names[0]], nil
}

// GetAllForType 获取所有支持指定类型的执行器
func (r *ExecutorRegistry) GetAllForType(jobType JobType) []Executor {
	r.mu.RLock()
	defer r.mu.RUnlock()

	names, exists := r.typeIndex[jobType]
	if !exists {
		return nil
	}

	result := make([]Executor, 0, len(names))
	for _, name := range names {
		if e, ok := r.executors[name]; ok {
			result = append(result, e)
		}
	}
	return result
}

// FindExecutor 查找能执行指定任务的执行器
func (r *ExecutorRegistry) FindExecutor(job *ExecutionJob) (Executor, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	names, exists := r.typeIndex[job.Type]
	if !exists || len(names) == 0 {
		return nil, ErrNoExecutorForType
	}

	// 找到第一个能执行此任务的执行器
	for _, name := range names {
		e := r.executors[name]
		if e.CanExecute(job) {
			return e, nil
		}
	}

	return nil, ErrNoExecutorForType
}

// List 列出所有执行器
func (r *ExecutorRegistry) List() []ExecutorInfo {
	r.mu.RLock()
	defer r.mu.RUnlock()

	result := make([]ExecutorInfo, 0, len(r.executors))
	for _, e := range r.executors {
		info := ExecutorInfo{
			Name:           e.Name(),
			Version:        e.Version(),
			SupportedTypes: e.SupportedTypes(),
			Status:         "ready",
		}
		result = append(result, info)
	}
	return result
}

// Count 返回注册的执行器数量
func (r *ExecutorRegistry) Count() int {
	r.mu.RLock()
	defer r.mu.RUnlock()
	return len(r.executors)
}

// ShutdownAll 关闭所有执行器
func (r *ExecutorRegistry) ShutdownAll() error {
	r.mu.Lock()
	defer r.mu.Unlock()

	var lastErr error
	for _, e := range r.executors {
		if err := e.Shutdown(); err != nil {
			lastErr = err
		}
	}

	r.executors = make(map[string]Executor)
	r.typeIndex = make(map[JobType][]string)
	return lastErr
}
