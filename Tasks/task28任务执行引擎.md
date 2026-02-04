# Task 28: 任务执行引擎

> **状态**: ✅ 实现完成  
> **优先级**: P1 (核心功能)  
> **实际工作量**: 1 天  
> **依赖**: Task 27 委托任务  
> **测试通过**: 36 个测试

---

## 📊 实现总结

### 已实现文件

| 文件 | 功能 | 行数 |
|------|------|------|
| `internal/execution/job.go` | 任务结构、执行器接口、基础执行器 | ~360 |
| `internal/execution/queue.go` | 优先级队列（堆实现） | ~170 |
| `internal/execution/registry.go` | 执行器注册表 | ~190 |
| `internal/execution/engine.go` | 执行引擎（调度、工作池） | ~510 |
| `internal/execution/executors/types.go` | 类型重导出 | ~35 |
| `internal/execution/executors/search.go` | 搜索执行器 | ~230 |
| `internal/execution/executors/compute.go` | 计算执行器 | ~315 |
| `internal/execution/executors/llm.go` | LLM执行器 | ~335 |
| `internal/execution/execution_test.go` | Job和Queue测试 | ~200 |
| `internal/execution/engine_test.go` | Engine和Registry测试 | ~360 |
| `internal/execution/executors/executors_test.go` | 执行器测试 | ~400 |

### 核心特性

✅ **执行引擎**: 调度器 + 工作池 + 结果处理  
✅ **优先级队列**: 堆排序、优先级更新  
✅ **执行器接口**: 插件化设计，支持扩展  
✅ **内置执行器**: 搜索、计算、LLM  
✅ **任务生命周期**: Pending → Queued → Running → Completed/Failed  
✅ **重试机制**: 失败自动重试（可配置次数）  
✅ **超时控制**: Context 超时取消  
✅ **回调通知**: 任务完成时回调  

---

## 🎯 设计目标

### 核心问题

```
Q1: Agent 收到任务后如何执行？
Q2: 如何调度和管理任务执行？
Q3: 如何对接外部 LLM 或工具？
Q4: 如何处理任务超时和失败？
```

### 设计原则

1. **插件化执行器**: 支持多种任务类型的执行器
2. **资源隔离**: 任务执行不影响节点稳定性
3. **可观测性**: 任务执行状态实时可查
4. **容错机制**: 支持重试、超时、回滚

---

## 🏗️ 核心架构

### 系统组件

```
┌─────────────────────────────────────────────────────────────────┐
│                     Task Execution Engine                        │
├─────────────────────────────────────────────────────────────────┤
│                                                                 │
│  ┌─────────────┐   ┌─────────────┐   ┌─────────────┐           │
│  │  Scheduler  │   │   Worker    │   │  Registry   │           │
│  │  任务调度    │──▶│   工作池    │◀──│  执行器注册  │           │
│  └─────────────┘   └─────────────┘   └─────────────┘           │
│         │                │                   │                  │
│         ▼                ▼                   ▼                  │
│  ┌─────────────┐   ┌─────────────┐   ┌─────────────┐           │
│  │   Queue     │   │  Executor   │   │   Plugin    │           │
│  │  任务队列    │   │   执行器    │   │   插件系统   │           │
│  └─────────────┘   └─────────────┘   └─────────────┘           │
│                          │                                      │
│                          ▼                                      │
│  ┌──────────────────────────────────────────────────────────┐  │
│  │                    Executors                              │  │
│  │  ┌──────────┐ ┌──────────┐ ┌──────────┐ ┌──────────┐     │  │
│  │  │  Search  │ │  Compute │ │   LLM    │ │  Custom  │     │  │
│  │  │  Executor│ │  Executor│ │  Executor│ │  Executor│     │  │
│  │  └──────────┘ └──────────┘ └──────────┘ └──────────┘     │  │
│  └──────────────────────────────────────────────────────────┘  │
│                                                                 │
└─────────────────────────────────────────────────────────────────┘
```

---

## 📋 数据结构

### 执行任务

```go
// ExecutionJob 执行任务
type ExecutionJob struct {
    ID          string            `json:"id"`
    TaskID      string            `json:"task_id"`      // 关联的委托任务ID
    ExecutorID  string            `json:"executor_id"`  // 执行者节点ID
    Type        string            `json:"type"`         // 任务类型 (search/compute/llm/...)
    
    // 输入输出
    Input       map[string]any    `json:"input"`        // 任务输入参数
    Output      map[string]any    `json:"output"`       // 执行结果
    Artifacts   []Artifact        `json:"artifacts"`    // 产出文件/数据
    
    // 执行状态
    Status      JobStatus         `json:"status"`
    Progress    float64           `json:"progress"`     // 0-100
    Message     string            `json:"message"`      // 状态消息
    
    // 资源使用
    Resources   ResourceUsage     `json:"resources"`
    
    // 时间
    CreatedAt   int64             `json:"created_at"`
    StartedAt   int64             `json:"started_at"`
    CompletedAt int64             `json:"completed_at"`
    Timeout     int64             `json:"timeout"`      // 超时时间（秒）
    
    // 重试
    RetryCount  int               `json:"retry_count"`
    MaxRetries  int               `json:"max_retries"`
}

// JobStatus 任务状态
type JobStatus string

const (
    JobPending   JobStatus = "pending"    // 等待执行
    JobQueued    JobStatus = "queued"     // 已入队
    JobRunning   JobStatus = "running"    // 执行中
    JobCompleted JobStatus = "completed"  // 已完成
    JobFailed    JobStatus = "failed"     // 失败
    JobCancelled JobStatus = "cancelled"  // 已取消
    JobTimeout   JobStatus = "timeout"    // 超时
)

// Artifact 产出物
type Artifact struct {
    ID        string `json:"id"`
    Type      string `json:"type"`      // file/data/hash
    Name      string `json:"name"`
    Size      int64  `json:"size"`
    Hash      string `json:"hash"`      // SHA256
    Location  string `json:"location"`  // 存储位置
}

// ResourceUsage 资源使用情况
type ResourceUsage struct {
    CPUTime     int64 `json:"cpu_time"`      // CPU时间（毫秒）
    MemoryPeak  int64 `json:"memory_peak"`   // 内存峰值（字节）
    DiskRead    int64 `json:"disk_read"`     // 磁盘读取（字节）
    DiskWrite   int64 `json:"disk_write"`    // 磁盘写入（字节）
    NetworkIn   int64 `json:"network_in"`    // 网络接收（字节）
    NetworkOut  int64 `json:"network_out"`   // 网络发送（字节）
}
```

### 执行器接口

```go
// Executor 执行器接口
type Executor interface {
    // 基础信息
    Name() string
    Version() string
    SupportedTypes() []string
    
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
    Success   bool              `json:"success"`
    Output    map[string]any    `json:"output"`
    Artifacts []Artifact        `json:"artifacts"`
    Error     string            `json:"error,omitempty"`
    Resources ResourceUsage     `json:"resources"`
}

// ResourceEstimate 资源估算
type ResourceEstimate struct {
    CPUTime      int64 `json:"cpu_time"`       // 预计CPU时间
    MemoryBytes  int64 `json:"memory_bytes"`   // 预计内存使用
    DurationSec  int64 `json:"duration_sec"`   // 预计执行时间
}
```

---

## 🔧 核心模块

### 1. 任务调度器 (Scheduler)

```go
// Scheduler 任务调度器
type Scheduler struct {
    config      *SchedulerConfig
    queue       *PriorityQueue    // 优先级队列
    workers     *WorkerPool       // 工作池
    registry    *ExecutorRegistry // 执行器注册表
    
    // 运行中的任务
    runningJobs map[string]*ExecutionJob
    
    // 指标
    metrics     *SchedulerMetrics
}

// SchedulerConfig 调度器配置
type SchedulerConfig struct {
    MaxConcurrent     int           // 最大并发任务数
    QueueSize         int           // 队列大小
    DefaultTimeout    time.Duration // 默认超时时间
    CheckInterval     time.Duration // 状态检查间隔
    PriorityLevels    int           // 优先级级别数
}

// 核心方法
func (s *Scheduler) Submit(job *ExecutionJob) error
func (s *Scheduler) Cancel(jobID string) error
func (s *Scheduler) GetJob(jobID string) (*ExecutionJob, error)
func (s *Scheduler) ListJobs(filter JobFilter) []*ExecutionJob
func (s *Scheduler) GetMetrics() *SchedulerMetrics
```

### 2. 工作池 (WorkerPool)

```go
// WorkerPool 工作池
type WorkerPool struct {
    size      int
    workers   []*Worker
    jobChan   chan *ExecutionJob
    resultCh  chan *ExecutionResult
    stopChan  chan struct{}
}

// Worker 工作者
type Worker struct {
    id         int
    pool       *WorkerPool
    executor   Executor
    currentJob *ExecutionJob
}

// 工作者执行循环
func (w *Worker) run() {
    for {
        select {
        case job := <-w.pool.jobChan:
            result := w.execute(job)
            w.pool.resultCh <- result
        case <-w.pool.stopChan:
            return
        }
    }
}
```

### 3. 执行器注册表 (ExecutorRegistry)

```go
// ExecutorRegistry 执行器注册表
type ExecutorRegistry struct {
    executors map[string]Executor // type -> executor
    mu        sync.RWMutex
}

func (r *ExecutorRegistry) Register(executor Executor) error
func (r *ExecutorRegistry) Unregister(name string) error
func (r *ExecutorRegistry) Get(taskType string) (Executor, bool)
func (r *ExecutorRegistry) List() []ExecutorInfo
```

---

## 🔌 内置执行器

### 1. 搜索执行器 (SearchExecutor)

```go
// SearchExecutor 搜索任务执行器
type SearchExecutor struct {
    searchEngines map[string]SearchEngine
}

// 支持的搜索类型
// - file_search: 本地文件搜索
// - network_search: 网络资源搜索
// - content_search: 内容检索

func (e *SearchExecutor) Execute(ctx context.Context, job *ExecutionJob) (*ExecutionResult, error) {
    searchType := job.Input["search_type"].(string)
    query := job.Input["query"].(string)
    
    engine := e.searchEngines[searchType]
    results, err := engine.Search(ctx, query)
    if err != nil {
        return nil, err
    }
    
    return &ExecutionResult{
        Success: true,
        Output: map[string]any{
            "results": results,
            "count":   len(results),
        },
    }, nil
}
```

### 2. 计算执行器 (ComputeExecutor)

```go
// ComputeExecutor 计算任务执行器
type ComputeExecutor struct {
    sandbox    *Sandbox     // 沙箱环境
    runtimes   map[string]Runtime // 运行时（python/node/wasm）
}

// 支持的计算类型
// - script: 脚本执行（受限沙箱）
// - wasm: WebAssembly 执行
// - transform: 数据转换

func (e *ComputeExecutor) Execute(ctx context.Context, job *ExecutionJob) (*ExecutionResult, error) {
    computeType := job.Input["compute_type"].(string)
    
    switch computeType {
    case "wasm":
        return e.executeWasm(ctx, job)
    case "transform":
        return e.executeTransform(ctx, job)
    default:
        return nil, fmt.Errorf("unsupported compute type: %s", computeType)
    }
}
```

### 3. LLM 执行器 (LLMExecutor)

```go
// LLMExecutor LLM任务执行器
type LLMExecutor struct {
    providers map[string]LLMProvider // openai/anthropic/local
    config    *LLMConfig
}

// LLMProvider LLM提供者接口
type LLMProvider interface {
    Chat(ctx context.Context, messages []Message) (*Response, error)
    Complete(ctx context.Context, prompt string) (*Response, error)
}

// 支持的LLM任务
// - chat: 对话生成
// - completion: 文本补全
// - analysis: 内容分析
// - summary: 文本摘要

func (e *LLMExecutor) Execute(ctx context.Context, job *ExecutionJob) (*ExecutionResult, error) {
    llmType := job.Input["llm_type"].(string)
    provider := job.Input["provider"].(string)
    
    p := e.providers[provider]
    if p == nil {
        return nil, fmt.Errorf("unknown provider: %s", provider)
    }
    
    switch llmType {
    case "chat":
        return e.executeChat(ctx, p, job)
    case "completion":
        return e.executeCompletion(ctx, p, job)
    case "analysis":
        return e.executeAnalysis(ctx, p, job)
    default:
        return nil, fmt.Errorf("unsupported llm type: %s", llmType)
    }
}
```

---

## 📊 任务执行流程

### 执行流程

```
┌─────────────────────────────────────────────────────────────────┐
│                      任务执行流程                                │
├─────────────────────────────────────────────────────────────────┤
│                                                                 │
│  1. 任务接收                                                    │
│     ┌─────────────────────────────────────────────┐            │
│     │  Task Module  ────▶  Execution Engine       │            │
│     │  (委托任务)         (创建 ExecutionJob)      │            │
│     └─────────────────────────────────────────────┘            │
│                         │                                       │
│                         ▼                                       │
│  2. 资源检查                                                    │
│     ┌─────────────────────────────────────────────┐            │
│     │  检查本地资源 → 估算执行成本 → 决定是否接受  │            │
│     └─────────────────────────────────────────────┘            │
│                         │                                       │
│                         ▼                                       │
│  3. 任务入队                                                    │
│     ┌─────────────────────────────────────────────┐            │
│     │  Priority Queue (按优先级/时间排序)          │            │
│     └─────────────────────────────────────────────┘            │
│                         │                                       │
│                         ▼                                       │
│  4. 任务分发                                                    │
│     ┌─────────────────────────────────────────────┐            │
│     │  Scheduler ────▶ Worker ────▶ Executor      │            │
│     └─────────────────────────────────────────────┘            │
│                         │                                       │
│                         ▼                                       │
│  5. 执行与监控                                                  │
│     ┌─────────────────────────────────────────────┐            │
│     │  Execute → Progress Update → Timeout Check  │            │
│     └─────────────────────────────────────────────┘            │
│                         │                                       │
│                         ▼                                       │
│  6. 结果处理                                                    │
│     ┌─────────────────────────────────────────────┐            │
│     │  Save Artifacts → Update Status → Callback  │            │
│     └─────────────────────────────────────────────┘            │
│                                                                 │
└─────────────────────────────────────────────────────────────────┘
```

---

## 📁 文件结构

```
internal/execution/
├── engine.go           # 执行引擎主入口
├── scheduler.go        # 任务调度器
├── worker.go           # 工作池与工作者
├── registry.go         # 执行器注册表
├── job.go              # 执行任务定义
├── queue.go            # 优先级队列
├── metrics.go          # 指标收集
├── executors/
│   ├── executor.go     # 执行器接口
│   ├── search.go       # 搜索执行器
│   ├── compute.go      # 计算执行器
│   ├── llm.go          # LLM执行器
│   └── custom.go       # 自定义执行器支持
├── sandbox/
│   └── sandbox.go      # 沙箱环境
└── engine_test.go      # 单元测试
```

---

## 🧪 测试计划

### 单元测试

| 测试名称 | 覆盖模块 | 说明 |
|---------|---------|------|
| TestNewEngine | engine.go | 引擎创建 |
| TestSubmitJob | scheduler.go | 任务提交 |
| TestJobExecution | worker.go | 任务执行 |
| TestPriorityQueue | queue.go | 优先级队列 |
| TestExecutorRegistry | registry.go | 执行器注册 |
| TestSearchExecutor | search.go | 搜索执行 |
| TestComputeExecutor | compute.go | 计算执行 |
| TestLLMExecutor | llm.go | LLM执行 |
| TestJobTimeout | scheduler.go | 超时处理 |
| TestJobRetry | scheduler.go | 重试机制 |
| TestConcurrency | worker.go | 并发控制 |
| TestResourceLimit | engine.go | 资源限制 |

---

## 📋 实现清单

### Phase 1: 核心框架
- [ ] 定义数据结构 (job.go)
- [ ] 实现执行器接口 (executors/executor.go)
- [ ] 实现优先级队列 (queue.go)
- [ ] 实现调度器 (scheduler.go)
- [ ] 实现工作池 (worker.go)

### Phase 2: 内置执行器
- [ ] 搜索执行器 (search.go)
- [ ] 计算执行器 (compute.go)
- [ ] LLM执行器框架 (llm.go)

### Phase 3: 集成
- [ ] 与 Task 模块集成
- [ ] 与 Transfer 模块集成
- [ ] 事件回调机制

### Phase 4: 测试
- [ ] 单元测试
- [ ] 集成测试
- [ ] 性能测试

---

## 📝 注意事项

1. **安全性**: 计算任务必须在沙箱中执行
2. **资源控制**: 严格限制CPU/内存/网络使用
3. **超时处理**: 所有任务必须有超时限制
4. **可扩展性**: 支持自定义执行器插件
5. **可观测性**: 完整的日志和指标

