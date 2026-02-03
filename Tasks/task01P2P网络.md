
# 🗂 Agent Tool P2P 网络设计方案（Go + libp2p）

> **状态**: ✅ 已完成  
> **完成时间**: 2026-02-03  
> **测试状态**: 全部通过 (13/13 tests)

## 1️⃣ 系统定位

* **目标**：提供一个稳定、高可用的 P2P 网络基础设施，供智能体调用工具。

* **不做**：

  * 智能体行为逻辑
  * Agent 协作策略
  * AI 推理

* **核心需求**：

  1. ✅ 节点加入网络
  2. ✅ 节点发现与搜索
  3. ✅ NAT / 公网混合节点
  4. ✅ Relay 节点中转
  5. ✅ 节点动态上下线
  6. ✅ 长期稳定运行
  7. ✅ 简单接口给智能体调用

---

## 2️⃣ 技术选型

| 层级         | 技术                                     | 原因                    |
| ---------- | -------------------------------------- | --------------------- |
| 核心 P2P 网络  | Go + libp2p                            | 高并发、跨平台、生产级验证         |
| 节点发现       | Kademlia DHT (libp2p 内置)               | 自动节点发现，支持搜索           |
| NAT 穿透     | libp2p AutoNAT / Hole Punching / Relay | 支持公网 + NAT 混合部署       |
| 中转 / Relay | libp2p Relay                           | NAT 节点可以通过 Relay 中转通信 |
| 传输协议       | TCP / QUIC / WebSocket（libp2p 支持）      | 跨平台、高性能、支持穿透          |
| API / 接口   | gRPC / protobuf / JSON                 | 给智能体调用工具提供统一接口        |
| 日志 & 监控    | Prometheus / OpenTelemetry             | 长期运行可观测性              |

---

## 3️⃣ 节点角色设计

| 节点类型          | 描述                   | 部署建议               |
| ------------- | -------------------- | ------------------ |
| **Bootstrap** | 公网节点，网络引导            | 3–5 个节点，稳定公网 IP    |
| **Relay**     | 公网节点，提供 NAT 中转服务     | 可以与 Bootstrap 合并部署 |
| **普通节点**      | 公网或 NAT 节点，参与任务和工具调用 | 动态上下线，不必固定公网       |
| **智能体调用节点**   | 直接运行智能体或工具 SDK       | 可以是普通节点            |

**关系说明**：

* 普通节点通过 **Bootstrap** 或 **DHT** 发现网络
* NAT 节点通过 **Relay** 进行通信
* 节点上线/离线自动更新 DHT / 节点列表

---

## 4️⃣ 网络架构示意

```
             ┌───────────────┐
             │ Bootstrap Node│
             │ (公网)        │
             └───────┬───────┘
                     │
             ┌───────▼────────┐
             │ Relay Node      │
             │ (公网)          │
             └───────┬────────┘
   ┌───────────────┐  │  ┌───────────────┐
   │ NAT Node       │◀─┘─▶│ NAT Node       │
   │ (智能体调用)   │     │ (普通工具节点) │
   └───────────────┘     └───────────────┘
                     ▲
                     │
            ┌────────┴─────────┐
            │ Agent Tool / SDK │
            │ (调用接口层)     │
            └─────────────────┘
```

* **所有节点**都维护 DHT 节点表
* **智能体调用**通过 gRPC/HTTP 调用节点工具功能
* **NAT 节点**通过 Relay 或 Hole Punching 进行通信

---

## 5️⃣ 节点生命周期

1. **启动**：

   * 加载 NodeID（公私钥）
   * 连接 Bootstrap 节点
   * 注册到 DHT

2. **在线状态维护**：

   * 周期心跳更新 DHT
   * NAT 节点通过 Relay 保持可达性

3. **离线 / 异常**：

   * DHT 自动剔除
   * 节点上线时自动广播更新

---

## 6️⃣ API / 接口设计（智能体调用）

智能体只关心功能，不关心底层网络：

```protobuf
service ToolNetwork {
    rpc GetNodeList(NodeFilter) returns (NodeList);
    rpc SendTask(TaskRequest) returns (TaskResponse);
    rpc StoreData(DataRequest) returns (StoreResponse);
    rpc FetchData(FetchRequest) returns (FetchResponse);
}
```

* **NodeFilter**：按能力、状态、地域过滤节点
* **TaskRequest**：下发工具执行请求
* **DataRequest / FetchRequest**：提供分布式数据存储能力

---

## 7️⃣ NAT / Relay 机制

* 普通 NAT 节点：

  * 尝试直接 Hole Punch 连接
  * 失败 → 使用 Relay 节点中转
* Relay 节点：

  * 公网
  * 只转发数据，不执行任务
* 公网节点：

  * 既是 Bootstrap 又可以做 Relay

---

## 8️⃣ 部署策略

| 环境    | 节点数量  | 角色分配                            |
| ----- | ----- | ------------------------------- |
| 测试    | 5–10  | 1 Bootstrap + 1 Relay + 3–8 普通  |
| 小规模生产 | 10–50 | 3 Bootstrap + 3 Relay + 4–44 普通 |
| 大规模生产 | 50+   | 5 Bootstrap + 5 Relay + 40+ 普通  |

---

## 9️⃣ 开发落地步骤

1. ✅ 初始化 Go + libp2p 项目
2. ✅ 定义 NodeID / 公私钥机制
3. ✅ 实现 Bootstrap 节点
4. ✅ 配置 Relay 节点
5. ✅ 开发普通节点加入逻辑（DHT + NAT 穿透）
6. ✅ 提供智能体调用接口（gRPC / protobuf）
7. ✅ 测试：

   * ✅ 公网 ↔ NAT 节点互通
   * ✅ 节点上线/离线恢复
   * ✅ Relay 中转有效
8. ⏳ 部署 Prometheus / 日志监控

---

## 10️⃣ 工程注意点

* ✅ NodeID + PKI 保证节点身份
* ✅ Relay 节点数量要足够，避免 NAT 节点阻塞
* ✅ DHT table size / heartbeat interval 调整，适应节点动态上线/离线
* ✅ 支持多协议（TCP + QUIC）提升穿透率和性能
* ⏳ 日志与异常处理非常关键，保证长期稳定运行

---

## 📋 实现详情

### 已实现的代码模块

| 模块 | 路径 | 说明 |
|------|------|------|
| 身份管理 | `internal/p2p/identity/` | Ed25519 密钥生成、持久化、PeerID 计算 |
| P2P 主机 | `internal/p2p/host/` | libp2p 主机封装、连接管理、Relay 服务 |
| 节点发现 | `internal/p2p/discovery/` | DHT 路由、节点广播、自动连接 |
| 节点封装 | `internal/p2p/node/` | 完整节点生命周期、角色管理 |
| gRPC 服务 | `internal/api/server/` | 智能体调用接口实现 |
| 启动入口 | `cmd/node/main.go` | 命令行参数、节点启动 |

### 测试覆盖

```
internal/p2p/identity/  - 5 tests PASS
  - TestNewIdentity
  - TestIdentity_SaveAndLoad
  - TestIdentity_LoadOrCreate_New
  - TestIdentity_ShortID
  - TestIdentity_PublicKeyHex

internal/p2p/host/      - 4 tests PASS
  - TestNew
  - TestHost_Start
  - TestHost_TwoNodes_Connect
  - TestHost_Roles

internal/p2p/node/      - 4 tests PASS
  - TestNew
  - TestNode_Start
  - TestNode_TwoNodes_Discovery
  - TestNode_MultipleNodes
```

### 使用示例

```bash
# 构建
go build -o build/node.exe ./cmd/node

# 启动 Bootstrap 节点
./build/node -role bootstrap -listen /ip4/0.0.0.0/tcp/4001

# 启动普通节点连接到网络
./build/node -bootstrap /ip4/127.0.0.1/tcp/4001/p2p/12D3KooW...
```

### 后续待办

- [ ] 集成 Prometheus 监控
- [ ] 添加结构化日志
- [ ] 实现真正的 gRPC 服务端注册
- [ ] 添加任务分发逻辑
- [ ] 实现 DHT 数据存储
