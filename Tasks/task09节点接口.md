明白，你希望为 **智能体（agent）提供操作 P2P 网络的接口**，并通过 **HTTP/REST API** 让 agent 可以直接调用（例如使用 `curl`）。我帮你整理一份完整接口设计方案，包括必要的功能模块和请求示例。

---

# 🗂 P2P 网络接口设计方案（HTTP / REST）

## 1️⃣ 核心设计原则

1. **功能模块化**：将网络操作、任务管理、节点管理、信誉查询、超级节点操作拆分
2. **统一身份验证**：使用 SM2 公钥 + 签名验证请求
3. **轻量调用**：JSON 数据格式，HTTP POST/GET
4. **安全性**：

   * 签名验证 → 防止伪造请求
   * 可选 TLS 加密
5. **易扩展**：未来可以添加 gossip、留言板、投票等功能

---

## 2️⃣ 功能模块 & 接口列表

### 2.1 节点管理接口

| 功能       | 方法   | URL                     | 请求/响应示例                                                 | 描述               |
| -------- | ---- | ----------------------- | ------------------------------------------------------- | ---------------- |
| 注册节点     | POST | /node/register          | `{"pubkey":"xxx"}` → `{"status":"ok","nodeID":"hash"}`  | 节点加入网络，返回 NodeID |
| 查询节点     | GET  | /node/{nodeID}          | → `{"NodeID":"xxx","Reputation":80,"Status":"Active"}`  | 查询指定节点信息         |
| 节点列表     | GET  | /node/list              | → `[{"NodeID":"A"},{"NodeID":"B"}]`                     | 获取部分活跃节点列表       |
| 投票选超级节点  | POST | /node/vote-super        | `{"candidate":"NodeID_X","vote":1}` → `{"status":"ok"}` | 普通节点投票选超级节点      |
| 投票剔除超级节点 | POST | /node/vote-remove-super | `{"target":"NodeID_X","vote":1}` → `{"status":"ok"}`    | 普通节点投票剔除超级节点     |

---

### 2.2 任务管理接口

| 功能   | 方法   | URL                   | 请求/响应示例                                                                  | 描述               |
| ---- | ---- | --------------------- | ------------------------------------------------------------------------ | ---------------- |
| 发布任务 | POST | /task/publish         | `{"taskID":"123","payload":"...","deadline":...}` → `{"status":"ok"}`    | 普通节点或 agent 发布任务 |
| 查询任务 | GET  | /task/{taskID}        | → `{"taskID":"123","status":"pending","worker":"NodeID_A"}`              | 查询任务状态           |
| 接受任务 | POST | /task/accept          | `{"taskID":"123"}` → `{"status":"ok"}`                                   | 节点接受任务           |
| 提交结果 | POST | /task/submit          | `{"taskID":"123","result":"hash","signature":"xxx"}` → `{"status":"ok"}` | 节点提交结果，签名验证      |
| 查询结果 | GET  | /task/result/{taskID} | → `{"taskID":"123","result":"hash","verified":true}`                     | 查询任务最终结果及验证状态    |

---

### 2.3 信誉/声誉接口

| 功能       | 方法   | URL                  | 请求/响应示例                                                                    | 描述                       |
| -------- | ---- | -------------------- | -------------------------------------------------------------------------- | ------------------------ |
| 查询节点声誉   | GET  | /reputation/{nodeID} | → `{"NodeID":"xxx","Reputation":85}`                                       | 获取节点当前声誉值                |
| 查询网络声誉列表 | GET  | /reputation/list     | → `[{"NodeID":"A","Reputation":80},{"NodeID":"B","Reputation":65}]`        | 获取部分节点声誉，用于 agent 判断任务委托 |
| 更新间接声誉   | POST | /reputation/update   | `{"target":"NodeID_X","score":10,"source":"NodeID_Y"}` → `{"status":"ok"}` | 超级节点/可信节点更新他人声誉          |

---

### 2.4 留言板接口（参考上文）

| 功能       | 方法                               | URL                        | 请求/响应示例                                                                               | 描述            |
| -------- | -------------------------------- | -------------------------- | ------------------------------------------------------------------------------------- | ------------- |
| 发布留言     | POST                             | /message/publish           | `{"content":"Hello world","topic":"task","signature":"xxx"}` → `{"messageID":"hash"}` | 节点发布留言        |
| 查询留言     | GET                              | /message/{messageID}       | → `{"MessageID":"xxx","Author":"NodeID_A","Content":"...","Signature":"xxx"}`         | 查询单条留言        |
| 订阅 topic | GET /message/subscribe?topic=xxx | WebSocket 或 HTTP long-poll | → 持续返回新消息                                                                             | 订阅指定 topic 消息 |

---

### 2.5 超级节点接口

| 功能     | 方法   | URL                              | 描述                |
| ------ | ---- | -------------------------------- | ----------------- |
| 审计任务   | POST | /supernode/audit-task            | 超级节点提交任务审计结果，签名   |
| 审计节点行为 | POST | /supernode/audit-node            | 审计普通节点任务完成情况或信誉异常 |
| 查询审计结果 | GET  | /supernode/audit-result/{taskID} | 普通节点查询审计结果        |

---

### 2.6 网络信息接口

| 功能     | 方法  | URL                      | 描述                   |
| ------ | --- | ------------------------ | -------------------- |
| 获取网络拓扑 | GET | /network/topology        | 返回邻居节点列表、超级节点列表      |
| 节点状态   | GET | /network/status/{nodeID} | 返回节点在线/离线状态、任务数、贡献积分 |
| 节点日志   | GET | /network/log/{nodeID}    | 获取节点操作日志，用于调试/审计     |

---

## 3️⃣ 身份与安全

* 每个请求必须包含：

  * 节点 NodeID（SM2 公钥）
  * 请求内容签名 → 防止伪造请求
* 超级节点请求需附加**额外证明或抵押验证**

### 示例 Header

```
X-NodeID: <SM2 PubKey>
X-Signature: <SM2 Signature of body>
Content-Type: application/json
```

服务器收到请求后：

1. 验证签名
2. 检查节点状态、信誉、权限
3. 执行操作

---

## 4️⃣ 交互示例（curl）

```bash
# 发布任务
curl -X POST http://localhost:8080/task/publish \
  -H "Content-Type: application/json" \
  -H "X-NodeID: <SM2 PubKey>" \
  -H "X-Signature: <signature>" \
  -d '{"taskID":"123","payload":"do_something","deadline":1670000000}'
```

```bash
# 查询节点信誉
curl -X GET http://localhost:8080/reputation/NodeID_A
```

```bash
# 发布留言
curl -X POST http://localhost:8080/message/publish \
  -H "Content-Type: application/json" \
  -H "X-NodeID: <SM2 PubKey>" \
  -H "X-Signature: <signature>" \
  -d '{"content":"Hello","topic":"task"}'
```

---

## 5️⃣ 总结

* **核心接口模块**：

  1. 节点管理（注册、查询、选举、剔除）
  2. 任务管理（发布、接受、提交、查询）
  3. 信誉/声誉（查询、更新、传播）
  4. 留言板（发布、查询、订阅）
  5. 超级节点操作（审计、查询结果）
  6. 网络信息（拓扑、状态、日志）

* **HTTP + JSON + SM2 签名** → agent 可直接调用 `curl` 或任意 HTTP 客户端

* **安全性**：

  * 签名验证请求
  * 投票权重 + 抵押控制权限
  * 超级节点审计冗余

---

# 🚀 HTTP REST API 实现（v2.0）

> **基础端口**: 18345  
> **基础路径**: `/api/v1`  
> **数据格式**: JSON

---

## 6️⃣ 完整 API 接口清单

### 6.1 基础接口

| 功能 | 方法 | URL | 描述 |
|------|------|-----|------|
| 健康检查 | GET | `/health` | 返回服务状态 |
| 节点状态 | GET | `/status` | 返回节点运行信息 |

---

### 6.2 节点管理 `/api/v1/node/*`

| 功能 | 方法 | URL | 请求示例 | 响应示例 |
|------|------|-----|----------|----------|
| 节点信息 | GET | `/node/info` | - | `{"node_id":"xxx","status":"online","uptime":3600}` |
| 邻居列表 | GET | `/node/peers` | - | `{"count":5,"peers":[...]}` |
| 注册节点 | POST | `/node/register` | `{"pubkey":"xxx","signature":"xxx"}` | `{"node_id":"hash"}` |

---

### 6.3 邻居管理 `/api/v1/neighbor/*`

| 功能 | 方法 | URL | 请求示例 | 响应示例 |
|------|------|-----|----------|----------|
| 邻居列表 | GET | `/neighbor/list` | `?limit=10` | `{"neighbors":[{"node_id":"A","trust_score":0.85}]}` |
| 最佳邻居 | GET | `/neighbor/best` | `?count=3` | `{"neighbors":[...]}` |
| 添加邻居 | POST | `/neighbor/add` | `{"node_id":"xxx","addresses":["..."]}` | `{"status":"ok"}` |
| 删除邻居 | POST | `/neighbor/remove` | `{"node_id":"xxx"}` | `{"status":"ok"}` |
| 心跳检测 | POST | `/neighbor/ping` | `{"node_id":"xxx"}` | `{"latency_ms":50,"online":true}` |

---

### 6.4 消息接口 `/api/v1/message/*`

| 功能 | 方法 | URL | 请求示例 | 响应示例 |
|------|------|-----|----------|----------|
| 发送消息 | POST | `/message/send` | `{"to":"nodeB","type":"text","content":"hello"}` | `{"message_id":"xxx"}` |
| 接收消息 | POST | `/message/receive` | (内部使用) | `{"status":"received"}` |

---

### 6.5 邮箱接口 `/api/v1/mailbox/*`

| 功能 | 方法 | URL | 请求示例 | 响应示例 |
|------|------|-----|----------|----------|
| 发送邮件 | POST | `/mailbox/send` | `{"to":"nodeB","subject":"hi","content":"..."}` | `{"message_id":"xxx"}` |
| 收件箱 | GET | `/mailbox/inbox` | `?limit=20&offset=0` | `{"messages":[...],"total":50}` |
| 发件箱 | GET | `/mailbox/outbox` | `?limit=20&offset=0` | `{"messages":[...],"total":30}` |
| 读取邮件 | GET | `/mailbox/read/{id}` | - | `{"id":"xxx","from":"A","content":"..."}` |
| 标记已读 | POST | `/mailbox/mark-read` | `{"message_id":"xxx"}` | `{"status":"ok"}` |
| 删除邮件 | POST | `/mailbox/delete` | `{"message_id":"xxx"}` | `{"status":"ok"}` |

---

### 6.6 留言板接口 `/api/v1/bulletin/*`

| 功能 | 方法 | URL | 请求示例 | 响应示例 |
|------|------|-----|----------|----------|
| 发布留言 | POST | `/bulletin/publish` | `{"topic":"task","content":"...","ttl":3600}` | `{"message_id":"xxx"}` |
| 查询留言 | GET | `/bulletin/message/{id}` | - | `{"id":"xxx","author":"A","content":"..."}` |
| 按话题查询 | GET | `/bulletin/topic/{topic}` | `?limit=20` | `{"messages":[...]}` |
| 按作者查询 | GET | `/bulletin/author/{nodeID}` | `?limit=20` | `{"messages":[...]}` |
| 搜索留言 | GET | `/bulletin/search` | `?keyword=hello&limit=10` | `{"messages":[...]}` |
| 订阅话题 | POST | `/bulletin/subscribe` | `{"topic":"task"}` | `{"status":"subscribed"}` |
| 取消订阅 | POST | `/bulletin/unsubscribe` | `{"topic":"task"}` | `{"status":"unsubscribed"}` |
| 撤回留言 | POST | `/bulletin/revoke` | `{"message_id":"xxx"}` | `{"status":"revoked"}` |

---

### 6.7 任务管理 `/api/v1/task/*`

| 功能 | 方法 | URL | 请求示例 | 响应示例 |
|------|------|-----|----------|----------|
| 创建任务 | POST | `/task/create` | `{"type":"compute","description":"..."}` | `{"task_id":"xxx"}` |
| 任务状态 | GET | `/task/status` | `?task_id=xxx` | `{"status":"pending","progress":50}` |
| 接受任务 | POST | `/task/accept` | `{"task_id":"xxx"}` | `{"status":"accepted"}` |
| 提交结果 | POST | `/task/submit` | `{"task_id":"xxx","result":"..."}` | `{"status":"submitted"}` |
| 任务列表 | GET | `/task/list` | `?status=pending&limit=10` | `{"tasks":[...]}` |

---

### 6.8 声誉系统 `/api/v1/reputation/*`

| 功能 | 方法 | URL | 请求示例 | 响应示例 |
|------|------|-----|----------|----------|
| 查询声誉 | GET | `/reputation/query` | `?node_id=xxx` | `{"node_id":"xxx","reputation":85.5}` |
| 更新声誉 | POST | `/reputation/update` | `{"node_id":"xxx","delta":5,"reason":"task"}` | `{"new_reputation":90.5}` |
| 声誉排行 | GET | `/reputation/ranking` | `?limit=10` | `{"rankings":[{"node_id":"A","reputation":95}]}` |
| 声誉历史 | GET | `/reputation/history` | `?node_id=xxx&limit=20` | `{"history":[...]}` |

---

### 6.9 指责系统 `/api/v1/accusation/*`

| 功能 | 方法 | URL | 请求示例 | 响应示例 |
|------|------|-----|----------|----------|
| 发起指责 | POST | `/accusation/create` | `{"accused":"nodeB","type":"spam","reason":"..."}` | `{"accusation_id":"xxx"}` |
| 指责列表 | GET | `/accusation/list` | `?accused=xxx&limit=10` | `{"accusations":[...]}` |
| 指责详情 | GET | `/accusation/detail/{id}` | - | `{"id":"xxx","accuser":"A","accused":"B",...}` |
| 指责分析 | GET | `/accusation/analyze` | `?node_id=xxx` | `{"total_received":5,"credibility":0.3}` |

---

### 6.10 激励系统 `/api/v1/incentive/*`

| 功能 | 方法 | URL | 请求示例 | 响应示例 |
|------|------|-----|----------|----------|
| 奖励任务完成 | POST | `/incentive/award` | `{"node_id":"xxx","task_type":"relay"}` | `{"reward":10.0}` |
| 传播声誉 | POST | `/incentive/propagate` | `{"target":"xxx","delta":5}` | `{"propagated_to":3}` |
| 奖励历史 | GET | `/incentive/history` | `?node_id=xxx&limit=20` | `{"rewards":[...]}` |
| 耐受值查询 | GET | `/incentive/tolerance` | `?node_id=xxx` | `{"tolerance":5,"max":10}` |

---

### 6.11 投票系统 `/api/v1/voting/*`

| 功能 | 方法 | URL | 请求示例 | 响应示例 |
|------|------|-----|----------|----------|
| 创建提案 | POST | `/voting/proposal/create` | `{"title":"...","type":"kick","target":"nodeX"}` | `{"proposal_id":"xxx"}` |
| 提案列表 | GET | `/voting/proposal/list` | `?status=pending` | `{"proposals":[...]}` |
| 提案详情 | GET | `/voting/proposal/{id}` | - | `{"id":"xxx","title":"...","votes":{}}` |
| 投票 | POST | `/voting/vote` | `{"proposal_id":"xxx","vote":"yes"}` | `{"status":"voted"}` |
| 结束提案 | POST | `/voting/proposal/finalize` | `{"proposal_id":"xxx"}` | `{"result":"passed"}` |

---

### 6.12 超级节点 `/api/v1/supernode/*`

| 功能 | 方法 | URL | 请求示例 | 响应示例 |
|------|------|-----|----------|----------|
| 超级节点列表 | GET | `/supernode/list` | - | `{"supernodes":[{"node_id":"A","term":1}]}` |
| 候选人列表 | GET | `/supernode/candidates` | - | `{"candidates":[...]}` |
| 申请候选 | POST | `/supernode/apply` | `{"stake":1000}` | `{"status":"applied"}` |
| 撤销候选 | POST | `/supernode/withdraw` | - | `{"status":"withdrawn"}` |
| 投票候选人 | POST | `/supernode/vote` | `{"candidate":"nodeX"}` | `{"status":"voted"}` |
| 启动选举 | POST | `/supernode/election/start` | - | `{"election_id":"xxx"}` |
| 完成选举 | POST | `/supernode/election/finalize` | `{"election_id":"xxx"}` | `{"elected":[...]}` |
| 提交审计 | POST | `/supernode/audit/submit` | `{"target":"nodeX","passed":true}` | `{"audit_id":"xxx"}` |
| 审计结果 | GET | `/supernode/audit/result` | `?target=nodeX` | `{"pass_rate":0.8}` |

---

### 6.13 创世节点 `/api/v1/genesis/*`

| 功能 | 方法 | URL | 请求示例 | 响应示例 |
|------|------|-----|----------|----------|
| 创世信息 | GET | `/genesis/info` | - | `{"genesis_id":"xxx","created_at":"..."}` |
| 创建邀请 | POST | `/genesis/invite/create` | `{"for_pubkey":"xxx"}` | `{"invitation_id":"xxx"}` |
| 验证邀请 | POST | `/genesis/invite/verify` | `{"invitation":"xxx"}` | `{"valid":true,"inviter":"A"}` |
| 加入网络 | POST | `/genesis/join` | `{"invitation":"xxx","pubkey":"xxx"}` | `{"node_id":"xxx","neighbors":[...]}` |

---

### 6.14 日志系统 `/api/v1/log/*`

| 功能 | 方法 | URL | 请求示例 | 响应示例 |
|------|------|-----|----------|----------|
| 提交日志 | POST | `/log/submit` | `{"event_type":"task_complete","data":{}}` | `{"log_id":"xxx"}` |
| 查询日志 | GET | `/log/query` | `?node_id=xxx&event_type=task&limit=50` | `{"logs":[...]}` |
| 导出日志 | GET | `/log/export` | `?format=json&start=...&end=...` | `{"file":"logs.json"}` |

---

## 7️⃣ curl 调用示例

### 7.1 基础操作

```bash
# 健康检查
curl http://localhost:18345/health

# 节点状态
curl http://localhost:18345/status

# 节点信息
curl http://localhost:18345/api/v1/node/info
```

### 7.2 邻居管理

```bash
# 获取邻居列表
curl "http://localhost:18345/api/v1/neighbor/list?limit=10"

# 获取最佳邻居
curl "http://localhost:18345/api/v1/neighbor/best?count=3"

# 添加邻居
curl -X POST http://localhost:18345/api/v1/neighbor/add \
  -H "Content-Type: application/json" \
  -H "X-NodeID: <your_pubkey>" \
  -H "X-Signature: <signature>" \
  -d '{"node_id":"peer123","addresses":["/ip4/192.168.1.100/tcp/18345"]}'
```

### 7.3 邮箱操作

```bash
# 发送邮件
curl -X POST http://localhost:18345/api/v1/mailbox/send \
  -H "Content-Type: application/json" \
  -H "X-NodeID: <your_pubkey>" \
  -d '{"to":"nodeB","subject":"Hello","content":"Test message"}'

# 查看收件箱
curl "http://localhost:18345/api/v1/mailbox/inbox?limit=20"

# 标记已读
curl -X POST http://localhost:18345/api/v1/mailbox/mark-read \
  -H "Content-Type: application/json" \
  -d '{"message_id":"msg123"}'
```

### 7.4 留言板操作

```bash
# 发布留言
curl -X POST http://localhost:18345/api/v1/bulletin/publish \
  -H "Content-Type: application/json" \
  -H "X-NodeID: <your_pubkey>" \
  -d '{"topic":"tasks","content":"New task available","ttl":7200}'

# 按话题查询
curl "http://localhost:18345/api/v1/bulletin/topic/tasks?limit=20"

# 搜索留言
curl "http://localhost:18345/api/v1/bulletin/search?keyword=task&limit=10"

# 订阅话题
curl -X POST http://localhost:18345/api/v1/bulletin/subscribe \
  -H "Content-Type: application/json" \
  -d '{"topic":"tasks"}'
```

### 7.5 投票操作

```bash
# 创建提案
curl -X POST http://localhost:18345/api/v1/voting/proposal/create \
  -H "Content-Type: application/json" \
  -H "X-NodeID: <your_pubkey>" \
  -d '{"title":"Kick bad node","type":"kick","target":"badNode123"}'

# 投票
curl -X POST http://localhost:18345/api/v1/voting/vote \
  -H "Content-Type: application/json" \
  -H "X-NodeID: <your_pubkey>" \
  -d '{"proposal_id":"prop123","vote":"yes"}'
```

### 7.6 超级节点操作

```bash
# 申请成为候选人
curl -X POST http://localhost:18345/api/v1/supernode/apply \
  -H "Content-Type: application/json" \
  -H "X-NodeID: <your_pubkey>" \
  -d '{"stake":1000}'

# 投票支持候选人
curl -X POST http://localhost:18345/api/v1/supernode/vote \
  -H "Content-Type: application/json" \
  -H "X-NodeID: <your_pubkey>" \
  -d '{"candidate":"candidate123"}'
```

### 7.7 创世节点操作

```bash
# 查询创世信息
curl http://localhost:18345/api/v1/genesis/info

# 创建邀请（需创世节点权限）
curl -X POST http://localhost:18345/api/v1/genesis/invite/create \
  -H "Content-Type: application/json" \
  -H "X-NodeID: <genesis_pubkey>" \
  -d '{"for_pubkey":"newnode_pubkey"}'

# 使用邀请加入网络
curl -X POST http://localhost:18345/api/v1/genesis/join \
  -H "Content-Type: application/json" \
  -d '{"invitation":"invite_token","pubkey":"my_pubkey"}'
```

---

## 8️⃣ 响应格式

### 成功响应

```json
{
  "success": true,
  "code": 200,
  "data": {
    "node_id": "xxx",
    "status": "online"
  }
}
```

### 错误响应

```json
{
  "success": false,
  "code": 400,
  "error": "invalid request: missing required field 'node_id'"
}
```

### 错误码

| 错误码 | 描述 |
|--------|------|
| 200 | 成功 |
| 400 | 请求参数错误 |
| 401 | 未授权（签名验证失败） |
| 403 | 禁止访问（权限不足） |
| 404 | 资源不存在 |
| 405 | 方法不允许 |
| 500 | 服务器内部错误 |

---

## 9️⃣ 安全机制

### 9.1 请求签名

所有修改操作（POST）需包含签名头：

```
X-NodeID: <SM2 公钥 Hex>
X-Signature: <SM2(SHA256(body)) 签名 Hex>
X-Timestamp: <Unix 时间戳>
```

### 9.2 签名验证流程

1. 服务器提取 `X-NodeID` 和 `X-Signature`
2. 使用 SM2 公钥验证签名
3. 检查时间戳是否在 5 分钟内
4. 验证节点权限

### 9.3 权限控制

| 操作类型 | 权限要求 |
|----------|----------|
| 查询接口 | 任意节点 |
| 普通操作 | 已注册节点 |
| 审计操作 | 超级节点 |
| 创世操作 | 创世节点 |

