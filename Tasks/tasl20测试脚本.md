# 🧪 AgentNetwork 测试脚本

> 端到端自动化测试框架，模拟 **10 个节点从创世到正常运行**，包括任务、声誉、指责、消息、邻居和超级节点功能的全流程测试。

---

## 🚀 快速开始

### 安装依赖
```bash
# Python 3.8+
pip install requests  # 可选，脚本使用内置urllib
```

### 启动测试网络
```bash
cd scripts/

# 启动5节点网络（默认）
python network_manager.py start

# 启动10节点网络
python network_manager.py start -n 10

# 查看状态
python network_manager.py status

# 运行API测试
python api_test.py

# 停止网络
python network_manager.py stop

# 清理环境
python network_manager.py clear --all
```

---

## 📁 脚本文件

| 脚本 | 功能 |
|------|------|
| `network_manager.py` | 网络启动/停止/重启/状态管理 |
| `api_test.py` | HTTP API 端点测试 |
| `generate_keypair.py` | SM2 密钥对生成 |
| `send_heartbeat.py` | 心跳发送测试 |

---

## 🛠️ network_manager.py 命令

```bash
# 启动网络
python network_manager.py start [OPTIONS]
  -n, --nodes INT       节点数量 (默认: 5)
  --name STR            网络名称 (默认: testnet)
  --p2p-port INT        起始P2P端口 (默认: 9000)
  --http-port INT       起始HTTP端口 (默认: 18000)
  --rebuild             重新编译项目

# 停止网络
python network_manager.py stop

# 重启网络
python network_manager.py restart

# 查看状态
python network_manager.py status

# 运行测试
python network_manager.py test [OPTIONS]
  --all                 运行所有测试
  --unit                运行单元测试
  --integration         运行集成测试
  --api                 运行API测试
  -v, --verbose         详细输出

# 清理环境
python network_manager.py clear [OPTIONS]
  --all                 完全清理（包括配置）

# 查看日志
python network_manager.py logs [OPTIONS]
  --node STR            节点ID
  -n, --lines INT       显示行数 (默认: 50)

# 执行API调用
python network_manager.py exec [OPTIONS]
  --node STR            目标节点
  -e, --endpoint STR    API端点
  -m, --method STR      HTTP方法 (默认: GET)
  -d, --data STR        请求数据(JSON)
```

---

## 🧪 api_test.py 测试套件

```bash
# 运行所有测试
python api_test.py --all

# 指定端口
python api_test.py --port 18000

# 运行特定套件
python api_test.py --suite 邻居

# 列出测试套件
python api_test.py --list
```

### 测试覆盖

| 套件 | 测试项 |
|------|--------|
| 基础API | 健康检查、节点信息、对等节点列表 |
| 邻居管理 | 邻居列表、最佳邻居 |
| 邮箱功能 | 收件箱、发送邮件 |
| 公告板 | 公告列表、发布公告 |
| 信誉系统 | 信誉查询、信誉排名 |
| 投票系统 | 投票列表 |
| 超级节点 | 超级节点列表 |
| 创世节点 | 创世信息 |
| 激励机制 | 激励历史 |
| 日志管理 | 日志查询 |
| 消息传递 | 发送消息 |
| 指控系统 | 指控列表 |
| 错误处理 | 无效端点(404) |

---

# 📋 测试设计方案

## 1️⃣ 测试目标

1. 启动 10 个节点（Node1…Node10）
2. 完整验证：

   * 创世节点初始化
   * 节点加入（邀请 + 思考证明）
   * 邻居选择与维护
   * 任务分发、提交与验证
   * 声誉奖励与传播
   * 指责机制
   * 消息系统（类似邮箱）
   * 超级节点选举与投票
3. 测试节点异常：

   * 离线/延迟/作恶节点
   * 验证指责与剔除机制
4. 输出日志、声誉变化、消息流和任务完成情况

---

## 2️⃣ 测试架构

```
+---------------------------+
| Test Controller (脚本)    |
| - 启动节点进程            |
| - 调用HTTP接口            |
| - 收集日志 & 统计数据     |
+------------+--------------+
             |
             v
+---------------------------+
| Node1 ... Node10          |
| - HTTP接口监听端口        |
| - 邻居管理                |
| - 任务/声誉/指责/消息    |
| - SM2签名/验证            |
+---------------------------+
```

* 每个节点独立进程或线程
* 脚本通过 **HTTP REST API** 调用节点接口
* 可以模拟网络延迟、节点掉线、作恶行为

---

## 3️⃣ 测试流程设计

### Step 1: 创世节点初始化

```python
create_node(node_id="Node1", role="genesis")
```

* Node1 作为创世节点
* 初始化网络状态、邻居列表为空
* 生成创世任务/声誉/超级节点初始状态

---

### Step 2: 新节点加入

```python
for i in range(2, 11):
    invite_node(inviter="Node1", new_node=f"Node{i}")
    perform_proof_of_thought(node=f"Node{i}")
    join_network(node=f"Node{i}")
```

* 邀请机制 + 思考证明
* 邻居选择自动完成
* 初始声誉分配

---

### Step 3: 邻居管理测试

```python
for node in all_nodes:
    neighbors = get_neighbors(node)
    assert len(neighbors) >= MIN_NEIGHBORS
```

* 验证每个节点邻居数量
* 模拟部分节点离线 → 检查动态更新

---

### Step 4: 任务系统测试

```python
# 创世节点分发任务
create_task(creator="Node1", target_node="Node2", task_type="Compute")

# Node2 完成任务并提交
submit_task(node="Node2", task_id="task_001", result="solution_digest")

# 邻居/超级节点验证任务结果
verify_task(node="Node1", task_id="task_001")
```

* 检查声誉增加
* 检查声誉传播到邻居

---

### Step 5: 指责系统测试

```python
# Node3 发现 Node4 作恶
accuse(node="Node3", accused="Node4", reason="未完成任务")

# 邻居 Node5 验证并传播指责
propagate_accuse(node="Node5", accuse_msg="...")
```

* 检查：

  * Node4 声誉扣减
  * Node3 扣除少量声誉（防滥发）
  * 邻居传播有效

---

### Step 6: 消息系统测试

```python
send_message(sender="Node2", receiver="Node5", content="Hello Node5!")
receive_message(node="Node5")
```

* 检查：

  * 消息签名验证
  * 离线节点消息中继
  * 消息顺序与完整性

---

### Step 7: 超级节点选举测试

```python
# 所有节点投票选举超级节点
vote_supernode(voter="Node1", candidate="Node6")
vote_supernode(voter="Node2", candidate="Node6")
...
```

* 验证：

  * 超级节点选举成功
  * 超级节点可审计其他节点
  * 作恶超级节点被剔除

---

### Step 8: 日志与统计收集

```python
for node in all_nodes:
    logs = fetch_logs(node)
    reputation = get_reputation(node)
    neighbors = get_neighbors(node)
    tasks = get_task_status(node)
    print_summary(node, logs, reputation, neighbors, tasks)
```

* 输出每个节点：

  * 声誉变化
  * 任务完成情况
  * 消息发送/接收
  * 指责记录

---

## 4️⃣ 脚本示例 (Python + requests)

```python
import requests

NODES = [f"http://127.0.0.1:{18340+i}" for i in range(10)]

def create_task(node_url, target, task_id):
    r = requests.post(f"{node_url}/task/create", json={
        "TaskID": task_id,
        "TargetNodeID": target
    })
    return r.json()

def submit_task(node_url, task_id, result):
    r = requests.post(f"{node_url}/task/submit", json={
        "TaskID": task_id,
        "ResultDigest": result
    })
    return r.json()

def get_reputation(node_url):
    r = requests.get(f"{node_url}/reputation")
    return r.json()

# 示例: 分发任务
create_task(NODES[0], target="Node2", task_id="task_001")
submit_task(NODES[1], task_id="task_001", result="digest_001")
print(get_reputation(NODES[1]))
```

> 实际测试中，可以循环创建节点、任务、指责和消息，收集日志，实现全流程自动化。

---

## 5️⃣ 建议

1. **使用容器/线程**启动多个节点 → 方便隔离测试
2. **模拟异常节点**：

   * Node作恶
   * Node离线
   * Node滥发声誉/指责
3. **日志收集与分析**：

   * 输出声誉变化、任务完成率、指责传播情况
4. **自动化验证**：

   * 脚本可检查每个节点状态是否符合预期
   * 可以生成报告，标记异常节点

---

