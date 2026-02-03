
# 🗂 P2P 网络日志设计方案

## 1️⃣ 设计目标

1. **全面追踪**：记录节点操作、任务状态、声誉变化、指责传播
2. **时间可追溯**：每条日志附带时间戳
3. **身份可验证**：所有关键操作日志用 SM2 签名
4. **支持调试与审计**：便于开发团队和 agent 追踪错误
5. **去中心化**：日志主要存储在本地节点，可选择性发送给超级节点或可信节点用于分析

---

## 2️⃣ 日志分类

| 日志类型       | 内容                      | 用途             |
| ---------- | ----------------------- | -------------- |
| **节点管理日志** | 注册、加入网络、邻居更新、投票选/踢掉超级节点 | 监控节点加入/离开、邻居拓扑 |
| **任务日志**   | 发布任务、接受任务、提交结果、任务完成     | 调试任务分发、完成状态    |
| **声誉日志**   | 声誉增加/减少、邻居传播、衰减、耐受值触发   | 调试声誉激励、传播链     |
| **指责日志**   | 发起指责、邻居分析、扣分、传播         | 调试指责传播机制、防止滥用  |
| **消息日志**   | 邮箱消息发送、接收、状态更新          | 调试 agent 消息收发  |
| **系统日志**   | 错误、异常、警告、状态变化           | 调试系统异常和网络异常    |

---

## 3️⃣ 日志数据结构

```json
{
  "LogID": "hash(NodeID+Timestamp+EventType)",
  "NodeID": "NodeID_A",
  "Timestamp": 1670000000,
  "EventType": "TaskSubmit|ReputationChange|Accuse|MessageSend|Error",
  "Details": {
    "TaskID": "Task123",
    "DeltaReputation": 5,
    "Accuser": "NodeID_X",
    "Accused": "NodeID_Y",
    "MessageID": "Msg123",
    "Error": "Task timeout"
  },
  "Signature": "SM2签名(NodeID对Details签名)"
}
```

* **LogID**：唯一标识日志，便于追踪
* **Signature**：保证不可篡改
* **EventType**：分类事件，便于过滤和分析
* **Details**：存储事件相关信息

---

## 4️⃣ 存储策略

| 类型     | 存储位置      | 保留周期   | 描述                |
| ------ | --------- | ------ | ----------------- |
| 本地日志   | 节点本地      | 可配置    | 所有操作记录，便于调试 agent |
| 超级节点日志 | 超级节点/可信节点 | 短期/按需求 | 监控网络行为，异常审计       |
| 可选云存储  | 可选        | 按需     | 聚合分析、可视化          |

* 建议 **默认存储在节点本地**
* 超级节点可以周期性收集日志，用于网络健康监控

---

## 5️⃣ 日志接口设计（HTTP/REST）

### 5.1 提交日志

```http
POST /log/submit
Headers:
  X-NodeID: <NodeID>
  X-Signature: <SM2 Signature>
Body:
{
  "EventType": "TaskSubmit",
  "Details": {
    "TaskID": "Task123",
    "Status": "Completed"
  },
  "Timestamp": 1670000000
}
Response:
{
  "Status": "ok",
  "LogID": "hash"
}
```

### 5.2 查询日志

```http
GET /log/query
Query Params:
  NodeID=NodeID_A
  EventType=ReputationChange
  StartTime=1670000000
  EndTime=1670100000
Response:
[
  {
    "LogID": "xxx",
    "NodeID": "NodeID_A",
    "EventType": "ReputationChange",
    "Details": { "DeltaReputation": 5 },
    "Timestamp": 1670000001,
    "Signature": "xxx"
  }
]
```

### 5.3 下载日志（调试或审计）

* 可以按 **时间段、事件类型、节点ID** 过滤
* 可导出 JSON 或 CSV

---

## 6️⃣ 日志传播（可选）

* 日志可以通过 **邻居或超级节点**传递，用于：

  * 异常检测
  * 审计作恶节点
  * 网络健康分析
* 传播规则与 **声誉传播类似**：

  * 衰减（DecayFactor）
  * 耐受值限制
  * 签名验证

---

## 7️⃣ 日志管理与清理

1. **本地滚动日志**：

   * 配置最大文件大小或天数
   * 超过后自动归档或删除
2. **压缩存储**：

   * 老日志压缩为 JSONL 或 zip
3. **隐私与安全**：

   * 日志只存储必要信息
   * 关键事件使用 SM2 签名保证不可篡改

---

## 8️⃣ 日志的价值

* **调试 agent 行为**：任务分发、消息收发、指责传播
* **追踪异常节点**：发现滥发声誉、作恶节点
* **分析网络健康**：声誉分布、任务完成率、消息延迟
* **审计与研究**：实验网络参数调整、优化传播策略

---

* 本地日志生成 → 本地存储
* 超级节点收集 → 可选远程存储
* 日志与指责/任务/消息结合
* 衰减传播 + SM2 签名保证不可篡改

