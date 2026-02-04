# 📖 Task 23: AgentNetwork 架构思考与改进

> **创建时间**: 2026-02-04  
> **状态**: 进行中  
> **优先级**: 高

---

## 1. 项目现状分析

### 1.1 已完成的核心模块

| 模块 | 状态 | 完成度 | 说明 |
|------|:---:|:---:|------|
| P2P 网络 (libp2p) | ✅ | 100% | DHT、NAT穿透、Relay中转 |
| 节点身份 (SM2) | ✅ | 100% | 密钥生成、签名验证 |
| 节点通信 | ✅ | 100% | 点对点消息、PubSub广播 |
| 声誉系统 | ✅ | 100% | 评分、传播、衰减 |
| 投票机制 | ✅ | 100% | 提案、投票、权重计算 |
| 超级节点 | ✅ | 100% | 选举、审计、任期管理 |
| 邮箱功能 | ✅ | 100% | 消息收发、中继存储 |
| 留言板 | ✅ | 100% | 发布、订阅、Gossip传播 |
| 激励机制 | ✅ | 100% | 任务奖励、声誉传播 |
| 声誉指责 | ✅ | 100% | 指责、验证、耐受值 |
| 创世节点 | ✅ | 100% | 邀请、加入、邻居推荐 |
| HTTP API | ✅ | 100% | REST接口、CORS支持 |
| 存储模块 | ✅ | 100% | 多类型数据持久化 |
| 守护进程 | ✅ | 100% | start/stop/restart |

### 1.2 当前架构图

```
┌─────────────────────────────────────────────────────────────────┐
│                      AgentNetwork 架构                           │
├─────────────────────────────────────────────────────────────────┤
│                                                                  │
│  ┌─────────────┐    ┌─────────────┐    ┌─────────────┐          │
│  │   HTTP API  │    │  gRPC API   │    │    CLI      │          │
│  │   :18345    │    │   :50051    │    │  Commands   │          │
│  └──────┬──────┘    └──────┬──────┘    └──────┬──────┘          │
│         │                  │                  │                  │
│  ┌──────┴──────────────────┴──────────────────┴──────┐          │
│  │                    Node Manager                    │          │
│  │  (identity, config, lifecycle, daemon)            │          │
│  └──────────────────────┬────────────────────────────┘          │
│                         │                                        │
│  ┌──────────────────────┴────────────────────────────┐          │
│  │                  Core Services                     │          │
│  ├────────────┬────────────┬────────────┬────────────┤          │
│  │ Reputation │  Voting    │ SuperNode  │ Incentive  │          │
│  │  System    │  System    │  Manager   │  System    │          │
│  ├────────────┼────────────┼────────────┼────────────┤          │
│  │ Accusation │  Bulletin  │  Mailbox   │  Neighbor  │          │
│  │  System    │   Board    │  System    │  Manager   │          │
│  └────────────┴────────────┴────────────┴────────────┘          │
│                         │                                        │
│  ┌──────────────────────┴────────────────────────────┐          │
│  │                 Network Layer                      │          │
│  │  ┌──────────┐  ┌──────────┐  ┌──────────┐        │          │
│  │  │Messenger │  │Broadcast │  │ Reliable │        │          │
│  │  │  (P2P)   │  │ (PubSub) │  │Transport │        │          │
│  │  └──────────┘  └──────────┘  └──────────┘        │          │
│  └──────────────────────┬────────────────────────────┘          │
│                         │                                        │
│  ┌──────────────────────┴────────────────────────────┐          │
│  │              P2P Infrastructure (libp2p)          │          │
│  │  ┌──────────┐  ┌──────────┐  ┌──────────┐        │          │
│  │  │   DHT    │  │   NAT    │  │  Relay   │        │          │
│  │  │ Kademlia │  │ Traversal│  │ Circuit  │        │          │
│  │  └──────────┘  └──────────┘  └──────────┘        │          │
│  └──────────────────────┬────────────────────────────┘          │
│                         │                                        │
│  ┌──────────────────────┴────────────────────────────┐          │
│  │                   Storage Layer                    │          │
│  │   (neighbors, tasks, reputation, messages, etc.)  │          │
│  └───────────────────────────────────────────────────┘          │
│                                                                  │
└─────────────────────────────────────────────────────────────────┘
```

---

## 2. 核心问题与改进方向

### 2.1 架构层面问题

#### 问题 A: 缺乏统一的服务编排层

**现状**: 各模块（reputation, voting, supernode等）相对独立，没有统一的服务协调机制。

**建议**:
```go
// 新增: internal/orchestrator/orchestrator.go
type Orchestrator struct {
    node       *p2p.Node
    reputation *reputation.Manager
    voting     *voting.Manager
    supernode  *supernode.Manager
    incentive  *incentive.Manager
    // ... 其他服务
    
    eventBus   *EventBus  // 统一事件总线
}

func (o *Orchestrator) Start(ctx context.Context) error
func (o *Orchestrator) Stop() error
func (o *Orchestrator) Health() HealthStatus
```

#### 问题 B: Agent 与 Node 概念混淆

**现状**: 
- `cmd/agent/` 和 `cmd/node/` 两套入口
- `internal/agent/` 模块较为简单
- SKILL.md 描述的是 Agent 协作网络，但代码实现更偏向 Node

**建议**: 明确定义

| 概念 | 定义 | 职责 |
|------|------|------|
| **Node** | P2P 网络节点 | 网络通信、数据存储、消息传递 |
| **Agent** | 智能代理 | 任务执行、协作决策、贡献追踪 |

```go
// 关系: Agent 运行在 Node 之上
type Agent struct {
    node       *node.Node      // 底层网络节点
    identity   *Identity       // Agent 身份
    tasks      *TaskManager    // 任务管理
    contrib    *ContribTracker // 贡献追踪
}
```

#### 问题 C: 缺乏插件/模块化机制

**建议**: 引入模块注册机制

```go
// internal/module/module.go
type Module interface {
    Name() string
    Version() string
    Init(ctx context.Context, deps Dependencies) error
    Start(ctx context.Context) error
    Stop(ctx context.Context) error
    Health() ModuleHealth
}

type ModuleRegistry struct {
    modules map[string]Module
}
```

---

### 2.2 功能层面问题与改进

#### ✅ 需要增加的功能

| ID | 功能 | 优先级 | 说明 |
|:--:|------|:---:|------|
| F-01 | **任务执行引擎** | 高 | 接收任务、执行、上报结果 |
| F-02 | **协作协议** | 高 | Agent 间协作请求/响应流程 |
| F-03 | **贡献追踪** | 高 | PR/Review/Discussion 追踪 |
| F-04 | **协议版本管理** | 中 | protocol_hash 校验与同步 |
| F-05 | **限流与熔断** | 中 | 防止恶意请求、网络风暴 |
| F-06 | **指标监控** | 中 | Prometheus metrics 导出 |
| F-07 | **分布式锁** | 中 | 跨节点资源协调 |
| F-08 | **消息队列** | 低 | 异步任务处理 |
| F-09 | **插件系统** | 低 | 动态加载扩展功能 |

#### ⚠️ 需要改进的功能

| ID | 模块 | 问题 | 建议 |
|:--:|------|------|------|
| I-01 | httpapi | 接口测试覆盖率仅 27.1% | 增加 API 集成测试 |
| I-02 | reputation | 缺乏分布式一致性 | 引入 CRDT 或 Raft |
| I-03 | voting | 无法处理网络分区 | 增加分区容忍机制 |
| I-04 | supernode | 审计仅限单节点 | 实现真正的多节点交叉审计 |
| I-05 | genesis | 邀请函无撤销机制 | 增加邀请撤销功能 |
| I-06 | storage | 无备份恢复机制 | 增加自动备份与恢复 |
| I-07 | logging | 日志无集中收集 | 支持日志上报到超级节点 |

#### ❌ 应该移除或合并的功能

| ID | 内容 | 原因 | 建议 |
|:--:|------|------|------|
| R-01 | `cmd/agent/` | 与 `cmd/node/` 功能重叠 | 合并为统一入口 |
| R-02 | 重复的配置定义 | 各模块各有 Config | 统一配置管理 |
| R-03 | gRPC + HTTP 双接口 | 增加维护成本 | 保留 HTTP，gRPC 可选 |

---

### 2.3 使用流程设计

#### 2.3.1 新用户引导流程

```
┌─────────────────────────────────────────────────────────────┐
│                    AgentNetwork 使用流程                     │
└─────────────────────────────────────────────────────────────┘

1️⃣ 安装
   ├── 下载预编译二进制
   │   └── agentnetwork-{os}-{arch}
   └── 或从源码构建
       └── go build -o agentnetwork ./cmd/node

2️⃣ 初始化
   ├── agentnetwork init
   │   ├── 生成 SM2 密钥对
   │   ├── 创建配置文件
   │   └── 创建数据目录
   └── 输出: ./data/keys/node.key, ./data/config.json

3️⃣ 获取邀请 (首次加入网络)
   ├── 方式A: 从创世节点获取邀请
   │   └── curl http://genesis-node:18345/api/v1/genesis/invite
   └── 方式B: 从已有节点获取邀请
       └── 该节点需要有足够声誉

4️⃣ 加入网络
   ├── agentnetwork join --invitation <invitation_token>
   │   ├── 验证邀请有效性
   │   ├── 注册节点身份
   │   ├── 获取邻居推荐
   │   └── 初始化声誉
   └── 输出: 节点 ID, 初始邻居列表

5️⃣ 启动节点
   ├── agentnetwork start
   │   ├── 守护进程模式运行
   │   ├── 连接 bootstrap 节点
   │   ├── 加入 DHT 网络
   │   └── 启动 HTTP API
   └── agentnetwork run (前台调试)

6️⃣ 参与协作
   ├── 心跳广播 (自动)
   ├── 任务接收与执行
   ├── 代码审查与贡献
   └── 投票与治理参与

7️⃣ 管理与监控
   ├── agentnetwork status  - 查看状态
   ├── agentnetwork logs    - 查看日志
   ├── agentnetwork peers   - 查看邻居
   └── agentnetwork stop    - 停止节点
```

#### 2.3.2 命令行接口设计（改进）

```bash
# 当前命令
agentnetwork start|stop|restart|status|logs|run

# 建议增加的命令
agentnetwork init                    # 初始化节点
agentnetwork join <invitation>       # 加入网络
agentnetwork invite <pubkey>         # 邀请新节点
agentnetwork peers                   # 列出邻居
agentnetwork reputation [node_id]    # 查看声誉
agentnetwork task list|create|status # 任务管理
agentnetwork vote list|cast|create   # 投票管理
agentnetwork config get|set          # 配置管理
agentnetwork export                  # 导出数据
agentnetwork import                  # 导入数据
agentnetwork upgrade                 # 升级版本
```

---

## 3. 详细设计建议

### 3.1 任务执行引擎 (Task Engine)

```go
// internal/taskengine/engine.go
package taskengine

type TaskType string

const (
    TaskCodeReview   TaskType = "code_review"
    TaskPairCoding   TaskType = "pair_coding"
    TaskAudit        TaskType = "audit"
    TaskComputation  TaskType = "computation"
    TaskDataProcess  TaskType = "data_process"
)

type Task struct {
    ID          string
    Type        TaskType
    From        string          // 请求者
    To          string          // 执行者
    Payload     json.RawMessage // 任务数据
    Priority    int
    Deadline    time.Time
    CreatedAt   time.Time
    Status      TaskStatus
}

type TaskEngine struct {
    queue       *PriorityQueue
    workers     int
    resultChan  chan *TaskResult
    
    // 回调
    OnTaskReceived  func(*Task)
    OnTaskCompleted func(*TaskResult)
    OnTaskFailed    func(*Task, error)
}

// 核心方法
func (e *TaskEngine) Submit(task *Task) error
func (e *TaskEngine) Execute(ctx context.Context, task *Task) (*TaskResult, error)
func (e *TaskEngine) Cancel(taskID string) error
func (e *TaskEngine) GetStatus(taskID string) TaskStatus
```

### 3.2 协作协议 (Collaboration Protocol)

```go
// internal/collab/protocol.go
package collab

type CollabRequest struct {
    Version   string          `json:"version"`
    Type      string          `json:"type"`  // "collab_request"
    From      string          `json:"from"`
    To        string          `json:"to"`
    TaskType  string          `json:"task_type"`
    Payload   json.RawMessage `json:"payload"`
    Nonce     string          `json:"nonce"`
    Signature string          `json:"signature"`
}

type CollabResponse struct {
    Version      string `json:"version"`
    Type         string `json:"type"`  // "collab_response"
    RequestNonce string `json:"request_nonce"`
    From         string `json:"from"`
    Status       string `json:"status"`  // accepted|rejected|busy
    Reason       string `json:"reason,omitempty"`
    Signature    string `json:"signature"`
}

type CollabManager struct {
    messenger   *network.Messenger
    taskEngine  *taskengine.Engine
    reputation  *reputation.Manager
    
    // 协作策略 (Tit-for-Tat)
    strategy    CollabStrategy
}

// 核心方法
func (m *CollabManager) RequestCollab(req *CollabRequest) (*CollabResponse, error)
func (m *CollabManager) HandleRequest(req *CollabRequest) *CollabResponse
func (m *CollabManager) ShouldAccept(fromNodeID string) bool  // 基于信誉决策
```

### 3.3 贡献追踪 (Contribution Tracker)

```go
// internal/contrib/tracker.go
package contrib

type ContributionType string

const (
    ContribPRSubmitted    ContributionType = "pr_submitted"
    ContribPRMerged       ContributionType = "pr_merged"
    ContribReview         ContributionType = "review"
    ContribIssueCreated   ContributionType = "issue_created"
    ContribIssueResolved  ContributionType = "issue_resolved"
    ContribDiscussion     ContributionType = "discussion"
)

type Contribution struct {
    ID        string
    NodeID    string
    Type      ContributionType
    Timestamp time.Time
    Evidence  string  // URL 或哈希
    Score     float64
    Verified  bool
}

type ContribTracker struct {
    contributions map[string][]*Contribution  // nodeID -> contributions
    verifier      ContribVerifier
}

// 核心方法
func (t *ContribTracker) Record(contrib *Contribution) error
func (t *ContribTracker) Verify(contrib *Contribution) (bool, error)
func (t *ContribTracker) GetScore(nodeID string, period time.Duration) float64
func (t *ContribTracker) GetRanking(limit int) []*NodeScore
```

### 3.4 事件总线 (Event Bus)

```go
// internal/eventbus/eventbus.go
package eventbus

type EventType string

const (
    EventNodeJoined       EventType = "node.joined"
    EventNodeLeft         EventType = "node.left"
    EventReputationChanged EventType = "reputation.changed"
    EventVoteCreated      EventType = "vote.created"
    EventVoteFinalized    EventType = "vote.finalized"
    EventTaskReceived     EventType = "task.received"
    EventTaskCompleted    EventType = "task.completed"
    EventAccusation       EventType = "accusation.created"
    // ...
)

type Event struct {
    Type      EventType
    NodeID    string
    Timestamp time.Time
    Data      interface{}
}

type EventBus struct {
    subscribers map[EventType][]chan Event
    mu          sync.RWMutex
}

// 核心方法
func (b *EventBus) Publish(event Event)
func (b *EventBus) Subscribe(eventType EventType) <-chan Event
func (b *EventBus) Unsubscribe(eventType EventType, ch <-chan Event)
```

---

## 4. API 接口规划

### 4.1 当前 HTTP API 概览

| 路径 | 方法 | 说明 | 测试状态 |
|------|:---:|------|:---:|
| `/health` | GET | 健康检查 | ✅ |
| `/status` | GET | 节点状态 | ✅ |
| `/api/v1/node/info` | GET | 节点信息 | ✅ |
| `/api/v1/node/peers` | GET | 邻居列表 | ✅ |
| `/api/v1/message/send` | POST | 发送消息 | ✅ |
| `/api/v1/reputation/query` | GET | 查询声誉 | ✅ |
| `/api/v1/task/create` | POST | 创建任务 | ⚠️ |
| ... | ... | ... | ... |

### 4.2 建议增加的 API

| 路径 | 方法 | 说明 |
|------|:---:|------|
| `/api/v1/collab/request` | POST | 发起协作请求 |
| `/api/v1/collab/respond` | POST | 响应协作请求 |
| `/api/v1/contrib/record` | POST | 记录贡献 |
| `/api/v1/contrib/ranking` | GET | 贡献排行榜 |
| `/api/v1/protocol/hash` | GET | 获取协议哈希 |
| `/api/v1/protocol/sync` | POST | 同步协议 |
| `/api/v1/metrics` | GET | Prometheus 指标 |
| `/api/v1/admin/modules` | GET | 模块状态 |

### 4.3 WebSocket 支持

建议增加 WebSocket 支持，用于实时事件推送：

```go
// WebSocket 事件订阅
ws://node:18345/ws/events?types=reputation.changed,task.received

// 事件格式
{
    "type": "reputation.changed",
    "timestamp": "2026-02-04T10:00:00Z",
    "data": {
        "node_id": "12D3KooW...",
        "old_score": 50,
        "new_score": 55,
        "reason": "task_completed"
    }
}
```

---

## 5. 安全性改进

### 5.1 当前安全机制

- ✅ SM2 消息签名
- ✅ SM3 完整性校验
- ✅ 邀请函机制
- ✅ 声誉系统约束

### 5.2 建议增加的安全机制

| 机制 | 说明 | 优先级 |
|------|------|:---:|
| **重放攻击防护** | Nonce + 时间窗口校验 | 高 |
| **DDoS 防护** | 请求限流、IP 黑名单 | 高 |
| **Sybil 攻击防护** | PoW 注册门槛 | 中 |
| **Eclipse 攻击防护** | 邻居多样性检查 | 中 |
| **消息加密** | SM4 对称加密（可选） | 中 |
| **审计日志** | 关键操作不可篡改日志 | 低 |

```go
// internal/security/ratelimit.go
type RateLimiter struct {
    limits  map[string]*Limit  // endpoint -> limit
    clients map[string]*Client // IP/NodeID -> client state
}

func (r *RateLimiter) Allow(clientID, endpoint string) bool
func (r *RateLimiter) SetLimit(endpoint string, limit *Limit)
```

---

## 6. 测试改进计划

### 6.1 当前测试状态

| 类型 | 数量 | 覆盖率 |
|------|:---:|:---:|
| 单元测试 | 200+ | 高 |
| 集成测试 | 14 | 中 |
| API 测试 | 16/59 | 27.1% |
| 端到端测试 | 有限 | 低 |

### 6.2 测试改进计划

| ID | 任务 | 优先级 |
|:--:|------|:---:|
| T-01 | 补充 API 集成测试到 80%+ | 高 |
| T-02 | 增加混沌测试（网络分区、节点故障） | 中 |
| T-03 | 增加性能基准测试 | 中 |
| T-04 | 增加安全测试（模糊测试、渗透测试） | 低 |

---

## 7. 文档改进

### 7.1 需要创建的文档

| 文档 | 说明 | 优先级 |
|------|------|:---:|
| `docs/ARCHITECTURE.md` | 详细架构说明 | 高 |
| `docs/API.md` | 完整 API 文档 | 高 |
| `docs/PROTOCOL.md` | 协议规范 | 高 |
| `docs/SECURITY.md` | 安全模型 | 中 |
| `docs/DEPLOYMENT.md` | 部署指南 | 中 |
| `docs/CONTRIBUTION.md` | 贡献指南 | 低 |

### 7.2 README 改进建议

- 增加快速开始（5 分钟体验）
- 增加架构图
- 增加使用场景示例
- 增加 FAQ 部分

---

## 8. 优先级排序与行动计划

### Phase 1: 核心功能完善 (1-2 周)

| 优先级 | 任务 | 估时 |
|:---:|------|:---:|
| P0 | 统一 Agent/Node 入口 | 2d |
| P0 | 实现任务执行引擎 | 3d |
| P0 | 实现协作协议 | 3d |
| P1 | 增加事件总线 | 2d |
| P1 | 增加命令行子命令 (init/join/invite) | 2d |

### Phase 2: 稳定性与安全 (1-2 周)

| 优先级 | 任务 | 估时 |
|:---:|------|:---:|
| P0 | 请求限流实现 | 2d |
| P0 | 重放攻击防护 | 1d |
| P1 | API 测试覆盖率提升 | 3d |
| P1 | 混沌测试框架 | 2d |

### Phase 3: 生态与文档 (1 周)

| 优先级 | 任务 | 估时 |
|:---:|------|:---:|
| P1 | 架构文档编写 | 2d |
| P1 | API 文档生成 | 1d |
| P2 | 贡献追踪实现 | 3d |
| P2 | WebSocket 事件推送 | 2d |

---

## 9. 总结

AgentNetwork 已经构建了完整的 P2P 网络基础设施，包括身份、通信、声誉、投票、激励等核心模块。但要实现 SKILL.md 中描述的"去中心化自治 Agent 网络"愿景，还需要：

1. **统一概念模型**: 明确 Agent 与 Node 的关系
2. **增加核心功能**: 任务引擎、协作协议、贡献追踪
3. **提升稳定性**: 安全机制、测试覆盖、错误处理
4. **改善用户体验**: CLI 改进、文档完善、快速上手

当前代码质量良好，模块化清晰，测试覆盖完整。建议按照上述优先级逐步完善，预计 4-6 周可完成核心改进。

---

## 附录: 新增任务清单

以下任务需要添加到任务跟踪系统：

| ID | 任务名称 | 类型 | 优先级 | 关联 |
|:--:|---------|:---:|:---:|------|
| task24 | 统一服务编排层 | 新增 | P0 | - |
| task25 | 任务执行引擎 | 新增 | P0 | task03 |
| task26 | 协作协议实现 | 新增 | P0 | task03 |
| task27 | 贡献追踪系统 | 新增 | P1 | task11 |
| task28 | 事件总线机制 | 新增 | P1 | - |
| task29 | CLI 命令扩展 | 改进 | P1 | task18 |
| task30 | 安全机制增强 | 改进 | P0 | task14 |
| task31 | API 测试完善 | 测试 | P1 | task09/18 |
| task32 | 文档体系建设 | 文档 | P1 | - |
