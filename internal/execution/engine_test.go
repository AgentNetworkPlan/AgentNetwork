package execution

import (
	"context"
	"testing"
	"time"
)

func TestNewEngine(t *testing.T) {
	engine := NewEngine(nil)

	if engine == nil {
		t.Fatal("Engine should not be nil")
	}
	if engine.config.MaxConcurrent != 10 {
		t.Errorf("Expected default MaxConcurrent 10, got %d", engine.config.MaxConcurrent)
	}
}

func TestEngineStartStop(t *testing.T) {
	engine := NewEngine(&EngineConfig{
		MaxConcurrent: 2,
		QueueSize:     10,
		WorkerCount:   2,
	})

	// Start
	if err := engine.Start(); err != nil {
		t.Fatalf("Start failed: %v", err)
	}
	if !engine.IsRunning() {
		t.Error("Engine should be running")
	}

	// Start again (should be no-op)
	if err := engine.Start(); err != nil {
		t.Fatalf("Second start failed: %v", err)
	}

	// Stop
	if err := engine.Stop(); err != nil {
		t.Fatalf("Stop failed: %v", err)
	}
	if engine.IsRunning() {
		t.Error("Engine should not be running")
	}
}

func TestEngineSubmit(t *testing.T) {
	engine := NewEngine(&EngineConfig{
		MaxConcurrent: 2,
		QueueSize:     10,
		WorkerCount:   2,
	})
	engine.Start()
	defer engine.Stop()

	// Register a mock executor
	mockExecutor := &mockExecutor{
		BaseExecutor: NewBaseExecutor("mock", "1.0.0", []JobType{JobTypeCompute}),
	}
	engine.RegisterExecutor(mockExecutor)

	// Submit a job
	job := NewExecutionJob("task1", JobTypeCompute, map[string]any{"value": 42})

	err := engine.Submit(job)
	if err != nil {
		t.Fatalf("Submit failed: %v", err)
	}

	if job.ID == "" {
		t.Error("Job ID should be generated")
	}
	if job.Status != JobQueued {
		t.Errorf("Expected queued status, got %s", job.Status)
	}

	// Wait for job to complete
	time.Sleep(500 * time.Millisecond)

	retrievedJob, err := engine.GetJob(job.ID)
	if err != nil {
		t.Fatalf("GetJob failed: %v", err)
	}
	if retrievedJob.Status != JobCompleted {
		t.Errorf("Expected completed status, got %s", retrievedJob.Status)
	}
}

func TestEngineSubmitNotRunning(t *testing.T) {
	engine := NewEngine(nil)

	job := NewExecutionJob("task1", JobTypeCompute, nil)
	err := engine.Submit(job)

	if err != ErrEngineNotRunning {
		t.Errorf("Expected ErrEngineNotRunning, got %v", err)
	}
}

func TestEngineQueueFull(t *testing.T) {
	engine := NewEngine(&EngineConfig{
		MaxConcurrent: 1,
		QueueSize:     2,
		WorkerCount:   1,
	})
	engine.Start()
	defer engine.Stop()

	// Fill the queue
	for i := 0; i < 2; i++ {
		job := NewExecutionJob("task", JobTypeCompute, nil)
		engine.Submit(job)
	}

	// Next submission should fail
	job := NewExecutionJob("task", JobTypeCompute, nil)
	err := engine.Submit(job)

	if err != ErrQueueFull {
		t.Errorf("Expected ErrQueueFull, got %v", err)
	}
}

func TestEngineCancel(t *testing.T) {
	engine := NewEngine(&EngineConfig{
		MaxConcurrent: 2,
		QueueSize:     10,
		WorkerCount:   1,
	})
	engine.Start()
	defer engine.Stop()

	// Submit jobs without executor (they will stay in queue)
	job := NewExecutionJob("task1", JobTypeCompute, nil)
	engine.Submit(job)

	// Cancel
	err := engine.Cancel(job.ID)
	if err != nil {
		t.Fatalf("Cancel failed: %v", err)
	}

	retrievedJob, _ := engine.GetJob(job.ID)
	if retrievedJob.Status != JobCancelled {
		t.Errorf("Expected cancelled status, got %s", retrievedJob.Status)
	}
}

func TestEngineCancelNotFound(t *testing.T) {
	engine := NewEngine(nil)
	engine.Start()
	defer engine.Stop()

	err := engine.Cancel("nonexistent")
	if err != ErrJobNotFound {
		t.Errorf("Expected ErrJobNotFound, got %v", err)
	}
}

func TestEngineGetJobNotFound(t *testing.T) {
	engine := NewEngine(nil)

	_, err := engine.GetJob("nonexistent")
	if err != ErrJobNotFound {
		t.Errorf("Expected ErrJobNotFound, got %v", err)
	}
}

func TestEngineListJobs(t *testing.T) {
	engine := NewEngine(&EngineConfig{
		MaxConcurrent: 1,
		QueueSize:     10,
		WorkerCount:   1,
	})
	engine.Start()
	defer engine.Stop()

	// Submit multiple jobs
	for i := 0; i < 5; i++ {
		job := NewExecutionJob("task", JobTypeCompute, nil)
		engine.Submit(job)
	}

	// List all
	jobs := engine.ListJobs(nil)
	if len(jobs) != 5 {
		t.Errorf("Expected 5 jobs, got %d", len(jobs))
	}

	// List with filter
	jobs = engine.ListJobs(&JobFilter{Limit: 2})
	if len(jobs) != 2 {
		t.Errorf("Expected 2 jobs with limit, got %d", len(jobs))
	}
}

func TestEngineMetrics(t *testing.T) {
	engine := NewEngine(&EngineConfig{
		MaxConcurrent: 2,
		QueueSize:     10,
		WorkerCount:   2,
	})
	engine.Start()
	defer engine.Stop()

	// Register executor
	mockExecutor := &mockExecutor{
		BaseExecutor: NewBaseExecutor("mock", "1.0.0", []JobType{JobTypeCompute}),
	}
	engine.RegisterExecutor(mockExecutor)

	// Submit job
	job := NewExecutionJob("task1", JobTypeCompute, nil)
	engine.Submit(job)

	time.Sleep(500 * time.Millisecond)

	metrics := engine.GetMetrics()
	if metrics.TotalSubmitted != 1 {
		t.Errorf("Expected 1 submitted, got %d", metrics.TotalSubmitted)
	}
	if metrics.TotalCompleted != 1 {
		t.Errorf("Expected 1 completed, got %d", metrics.TotalCompleted)
	}
}

func TestEngineCallback(t *testing.T) {
	engine := NewEngine(&EngineConfig{
		MaxConcurrent: 2,
		QueueSize:     10,
		WorkerCount:   2,
	})
	engine.Start()
	defer engine.Stop()

	// Register executor
	mockExecutor := &mockExecutor{
		BaseExecutor: NewBaseExecutor("mock", "1.0.0", []JobType{JobTypeCompute}),
	}
	engine.RegisterExecutor(mockExecutor)

	// Add callback
	callbackReceived := make(chan bool, 1)
	engine.AddCallback(func(job *ExecutionJob) {
		callbackReceived <- true
	})

	// Submit job
	job := NewExecutionJob("task1", JobTypeCompute, nil)
	engine.Submit(job)

	// Wait for callback
	select {
	case <-callbackReceived:
		// OK
	case <-time.After(2 * time.Second):
		t.Error("Callback not received")
	}
}

func TestExecutorRegistry(t *testing.T) {
	registry := NewExecutorRegistry()

	// Register
	executor := &mockExecutor{
		BaseExecutor: NewBaseExecutor("test", "1.0.0", []JobType{JobTypeCompute}),
	}

	err := registry.Register(executor)
	if err != nil {
		t.Fatalf("Register failed: %v", err)
	}

	if registry.Count() != 1 {
		t.Errorf("Expected 1 executor, got %d", registry.Count())
	}

	// Get by name
	retrieved, ok := registry.Get("test")
	if !ok {
		t.Error("Executor should be found")
	}
	if retrieved.Name() != "test" {
		t.Errorf("Expected 'test', got %s", retrieved.Name())
	}

	// Get for type
	retrieved, err = registry.GetForType(JobTypeCompute)
	if err != nil {
		t.Fatalf("GetForType failed: %v", err)
	}
	if retrieved.Name() != "test" {
		t.Errorf("Expected 'test', got %s", retrieved.Name())
	}

	// Get for unsupported type
	_, err = registry.GetForType(JobTypeLLM)
	if err != ErrNoExecutorForType {
		t.Errorf("Expected ErrNoExecutorForType, got %v", err)
	}

	// Unregister
	err = registry.Unregister("test")
	if err != nil {
		t.Fatalf("Unregister failed: %v", err)
	}

	if registry.Count() != 0 {
		t.Errorf("Expected 0 executors, got %d", registry.Count())
	}
}

func TestExecutorRegistryDuplicate(t *testing.T) {
	registry := NewExecutorRegistry()

	executor := &mockExecutor{
		BaseExecutor: NewBaseExecutor("test", "1.0.0", []JobType{JobTypeCompute}),
	}

	registry.Register(executor)
	err := registry.Register(executor)

	if err != ErrExecutorExists {
		t.Errorf("Expected ErrExecutorExists, got %v", err)
	}
}

func TestExecutorRegistryUnregisterNotFound(t *testing.T) {
	registry := NewExecutorRegistry()

	err := registry.Unregister("nonexistent")
	if err != ErrExecutorNotFound {
		t.Errorf("Expected ErrExecutorNotFound, got %v", err)
	}
}

// mockExecutor 模拟执行器
type mockExecutor struct {
	*BaseExecutor
	executeDelay time.Duration
	shouldFail   bool
}

func (e *mockExecutor) Execute(ctx context.Context, job *ExecutionJob) (*ExecutionResult, error) {
	if e.executeDelay > 0 {
		select {
		case <-ctx.Done():
			return nil, ctx.Err()
		case <-time.After(e.executeDelay):
		}
	}

	if e.shouldFail {
		return NewErrorResult("mock error"), nil
	}

	return NewSuccessResult(map[string]any{
		"message": "mock execution completed",
	}, nil), nil
}
