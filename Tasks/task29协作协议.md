# Task 29: 多Agent协作协议

> **状态**: 📋 设计中  
> **优先级**: P1 (核心功能)  
> **预计工作量**: 3-5 天  
> **依赖**: Task 27 委托任务, Task 28 任务执行引擎

---

## 🎯 设计目标

### 核心问题

```
Q1: 多个 Agent 如何协作完成复杂任务？
Q2: 如何分解和分配子任务？
Q3: 如何同步协作进度？
Q4: 如何处理协作中的冲突和失败？
```

### 设计原则

1. **松耦合协作**: Agent 之间通过消息协作，不共享状态
2. **任务分解**: 支持将大任务分解为子任务
3. **进度同步**: 实时同步协作进度
4. **容错处理**: 支持成员退出、超时、失败恢复

---

## 🏗️ 核心架构

### 系统组件

```
┌─────────────────────────────────────────────────────────────────┐
│                    Collaboration Protocol                        │
├─────────────────────────────────────────────────────────────────┤
│                                                                 │
│  ┌─────────────┐   ┌─────────────┐   ┌─────────────┐           │
│  │  Workflow   │   │   Session   │   │   Sync      │           │
│  │  工作流定义  │   │   协作会话  │   │   状态同步  │           │
│  └─────────────┘   └─────────────┘   └─────────────┘           │
│         │                │                   │                  │
│         ▼                ▼                   ▼                  │
│  ┌─────────────┐   ┌─────────────┐   ┌─────────────┐           │
│  │    DAG      │   │   Member    │   │   Message   │           │
│  │  任务依赖图  │   │   成员管理  │   │   协作消息  │           │
│  └─────────────┘   └─────────────┘   └─────────────┘           │
│                                                                 │
└─────────────────────────────────────────────────────────────────┘
```

### 协作模式

```
1. 串行协作 (Sequential)
   A → B → C
   每个 Agent 完成后，下一个接续

2. 并行协作 (Parallel)
   ┌→ B ─┐
   A       D
   └→ C ─┘
   多个 Agent 同时执行

3. 管道协作 (Pipeline)
   A →→→ B →→→ C
   流式处理，边输出边输入

4. 主从协作 (Master-Worker)
        ┌→ W1
   M ──→├→ W2
        └→ W3
   主节点分配，工作节点执行
```

---

## 📁 文件结构

```
internal/collaboration/
├── workflow.go       # 工作流定义
├── session.go        # 协作会话管理
├── member.go         # 成员管理
├── sync.go           # 状态同步
├── message.go        # 协作消息
├── dag.go            # 任务依赖图
└── collaboration_test.go
```

---

## 📋 数据结构

### 工作流定义

```go
// Workflow 工作流定义
type Workflow struct {
    ID          string            `json:"id"`
    Name        string            `json:"name"`
    Description string            `json:"description"`
    Version     string            `json:"version"`
    
    // 节点和边
    Nodes       []WorkflowNode    `json:"nodes"`      // 任务节点
    Edges       []WorkflowEdge    `json:"edges"`      // 依赖关系
    
    // 输入输出
    Inputs      []ParamDef        `json:"inputs"`     // 输入参数定义
    Outputs     []ParamDef        `json:"outputs"`    // 输出参数定义
    
    // 元数据
    CreatedAt   int64             `json:"created_at"`
    CreatedBy   string            `json:"created_by"`
    Tags        []string          `json:"tags"`
}

// WorkflowNode 工作流节点
type WorkflowNode struct {
    ID          string            `json:"id"`
    Type        NodeType          `json:"type"`       // task/subflow/decision/join
    Name        string            `json:"name"`
    
    // 执行配置
    TaskType    string            `json:"task_type,omitempty"`  // 对应的任务类型
    Executor    string            `json:"executor,omitempty"`   // 指定执行器
    Input       map[string]any    `json:"input,omitempty"`      // 输入参数映射
    
    // 约束
    Timeout     int64             `json:"timeout,omitempty"`    // 超时秒数
    Retries     int               `json:"retries,omitempty"`    // 重试次数
    Condition   string            `json:"condition,omitempty"`  // 执行条件表达式
}

// WorkflowEdge 工作流边（依赖关系）
type WorkflowEdge struct {
    From        string            `json:"from"`       // 源节点ID
    To          string            `json:"to"`         // 目标节点ID
    Condition   string            `json:"condition,omitempty"`  // 条件表达式
    DataMapping map[string]string `json:"data_mapping,omitempty"` // 数据映射
}
```

### 协作会话

```go
// Session 协作会话
type Session struct {
    ID          string            `json:"id"`
    WorkflowID  string            `json:"workflow_id"`
    
    // 状态
    Status      SessionStatus     `json:"status"`
    Phase       string            `json:"phase"`      // 当前阶段
    Progress    float64           `json:"progress"`   // 0-100
    
    // 参与者
    Initiator   string            `json:"initiator"`  // 发起者
    Members     []Member          `json:"members"`    // 成员列表
    
    // 数据
    Context     map[string]any    `json:"context"`    // 共享上下文
    Results     map[string]any    `json:"results"`    // 节点结果
    
    // 时间
    CreatedAt   int64             `json:"created_at"`
    StartedAt   int64             `json:"started_at"`
    CompletedAt int64             `json:"completed_at"`
    
    // 节点状态
    NodeStates  map[string]*NodeState `json:"node_states"`
}

// SessionStatus 会话状态
type SessionStatus string

const (
    SessionPending   SessionStatus = "pending"    // 等待启动
    SessionRecruiting SessionStatus = "recruiting" // 招募成员
    SessionRunning   SessionStatus = "running"    // 执行中
    SessionPaused    SessionStatus = "paused"     // 暂停
    SessionCompleted SessionStatus = "completed"  // 完成
    SessionFailed    SessionStatus = "failed"     // 失败
    SessionCancelled SessionStatus = "cancelled"  // 取消
)

// NodeState 节点执行状态
type NodeState struct {
    NodeID      string            `json:"node_id"`
    Status      NodeStatus        `json:"status"`
    AssignedTo  string            `json:"assigned_to,omitempty"` // 分配给谁
    StartedAt   int64             `json:"started_at,omitempty"`
    CompletedAt int64             `json:"completed_at,omitempty"`
    Output      map[string]any    `json:"output,omitempty"`
    Error       string            `json:"error,omitempty"`
    Retries     int               `json:"retries"`
}

// NodeStatus 节点状态
type NodeStatus string

const (
    NodePending   NodeStatus = "pending"
    NodeReady     NodeStatus = "ready"     // 依赖满足，可执行
    NodeAssigned  NodeStatus = "assigned"  // 已分配
    NodeRunning   NodeStatus = "running"
    NodeCompleted NodeStatus = "completed"
    NodeFailed    NodeStatus = "failed"
    NodeSkipped   NodeStatus = "skipped"   // 条件不满足，跳过
)
```

### 成员管理

```go
// Member 协作成员
type Member struct {
    NodeID      string            `json:"node_id"`
    Role        MemberRole        `json:"role"`
    Capabilities []string         `json:"capabilities"` // 能力标签
    Status      MemberStatus      `json:"status"`
    JoinedAt    int64             `json:"joined_at"`
    LastActive  int64             `json:"last_active"`
}

// MemberRole 成员角色
type MemberRole string

const (
    RoleCoordinator MemberRole = "coordinator" // 协调者
    RoleWorker      MemberRole = "worker"      // 工作者
    RoleObserver    MemberRole = "observer"    // 观察者
)

// MemberStatus 成员状态
type MemberStatus string

const (
    MemberActive    MemberStatus = "active"
    MemberIdle      MemberStatus = "idle"
    MemberBusy      MemberStatus = "busy"
    MemberOffline   MemberStatus = "offline"
)
```

### 协作消息

```go
// CollaborationMessage 协作消息
type CollaborationMessage struct {
    ID          string            `json:"id"`
    SessionID   string            `json:"session_id"`
    Type        MessageType       `json:"type"`
    From        string            `json:"from"`
    To          string            `json:"to,omitempty"` // 空表示广播
    
    Payload     any               `json:"payload"`
    Timestamp   int64             `json:"timestamp"`
    Signature   string            `json:"signature"`
}

// MessageType 消息类型
type MessageType string

const (
    // 会话管理
    MsgInvite      MessageType = "invite"       // 邀请加入
    MsgJoin        MessageType = "join"         // 加入请求
    MsgAccept      MessageType = "accept"       // 接受加入
    MsgLeave       MessageType = "leave"        // 离开会话
    
    // 任务分配
    MsgAssign      MessageType = "assign"       // 分配任务
    MsgAck         MessageType = "ack"          // 确认接收
    MsgProgress    MessageType = "progress"     // 进度更新
    MsgResult      MessageType = "result"       // 结果提交
    
    // 同步
    MsgSync        MessageType = "sync"         // 状态同步
    MsgHeartbeat   MessageType = "heartbeat"    // 心跳
    
    // 控制
    MsgPause       MessageType = "pause"        // 暂停
    MsgResume      MessageType = "resume"       // 恢复
    MsgCancel      MessageType = "cancel"       // 取消
)
```

---

## 🔧 核心功能

### 1. 工作流解析

```go
// 解析工作流，构建DAG
func (w *Workflow) BuildDAG() (*DAG, error)

// 获取可执行节点（依赖已满足）
func (d *DAG) GetReadyNodes(completedNodes []string) []string

// 拓扑排序
func (d *DAG) TopologicalSort() ([]string, error)

// 检测循环依赖
func (d *DAG) HasCycle() bool
```

### 2. 会话管理

```go
// 创建会话
func (m *SessionManager) CreateSession(workflow *Workflow, initiator string) (*Session, error)

// 招募成员
func (m *SessionManager) RecruitMembers(sessionID string, requirements []Requirement) error

// 启动会话
func (m *SessionManager) StartSession(sessionID string) error

// 分配任务
func (m *SessionManager) AssignNode(sessionID, nodeID, memberID string) error

// 提交结果
func (m *SessionManager) SubmitResult(sessionID, nodeID string, result any) error
```

### 3. 状态同步

```go
// 同步状态到所有成员
func (s *Syncer) BroadcastState(session *Session) error

// 处理状态更新
func (s *Syncer) HandleStateUpdate(msg *CollaborationMessage) error

// 冲突解决
func (s *Syncer) ResolveConflict(session *Session, updates []StateUpdate) error
```

### 4. 容错处理

```go
// 成员超时处理
func (m *SessionManager) HandleMemberTimeout(sessionID, memberID string) error

// 任务失败重试
func (m *SessionManager) RetryNode(sessionID, nodeID string) error

// 成员替换
func (m *SessionManager) ReplaceMember(sessionID, oldMemberID, newMemberID string) error
```

---

## 📝 使用示例

### 定义工作流

```go
workflow := &Workflow{
    ID:   "translation-workflow",
    Name: "翻译工作流",
    Nodes: []WorkflowNode{
        {ID: "split", Type: NodeTypeTask, TaskType: "text_split"},
        {ID: "trans1", Type: NodeTypeTask, TaskType: "translate"},
        {ID: "trans2", Type: NodeTypeTask, TaskType: "translate"},
        {ID: "merge", Type: NodeTypeTask, TaskType: "text_merge"},
    },
    Edges: []WorkflowEdge{
        {From: "split", To: "trans1"},
        {From: "split", To: "trans2"},
        {From: "trans1", To: "merge"},
        {From: "trans2", To: "merge"},
    },
}
```

### 创建协作会话

```go
// 创建会话
session, _ := manager.CreateSession(workflow, "agent-A")

// 招募成员
manager.RecruitMembers(session.ID, []Requirement{
    {Capability: "translate", MinCount: 2},
})

// 启动执行
manager.StartSession(session.ID)
```

---

## ✅ 验收标准

1. **工作流解析**: 能正确解析 DAG，检测循环依赖
2. **会话管理**: 支持创建、启动、暂停、取消会话
3. **成员协作**: 支持邀请、加入、离开
4. **任务分配**: 自动分配就绪任务给空闲成员
5. **状态同步**: 实时同步会话状态
6. **容错处理**: 处理超时、失败、成员离开
7. **测试覆盖**: >80% 核心功能测试
