# AgentNetwork 测试指南

本文档描述 AgentNetwork 项目的测试体系和测试方法。

## 📋 测试概览

AgentNetwork 包含三层测试体系：

| 测试类型 | 工具 | 覆盖范围 | 测试数量 |
|---------|------|---------|---------|
| **单元测试** | Go test | 各模块独立功能 | 35+ 模块 |
| **集成测试** | Python | HTTP API / 节点生命周期 | 16 场景 |
| **网络模拟** | Go test | P2P 网络多节点协作 | 网络拓扑 |

### 测试统计

- ✅ **Go 单元测试**: 26+ 模块，200+ 测试用例
- ✅ **生命周期集成测试**: 16 个场景全部通过
- ✅ **存储模块测试**: 9 个测试全部通过
- ✅ **跨平台支持**: Windows/Linux/macOS 编译测试通过

---

## 🧪 单元测试 (Go)

### 快速开始

```bash
# 运行所有测试
go test -v ./...

# 运行特定模块
go test -v ./internal/p2p/identity/...
go test -v ./internal/storage/...
go test -v ./internal/daemon/...

# 生成覆盖率报告
go test -coverprofile=coverage.out ./...
go tool cover -html=coverage.out -o coverage.html
```

### 核心模块测试

#### P2P 网络测试

```bash
# 节点身份管理
go test -v ./internal/p2p/identity/...

# libp2p 主机封装
go test -v ./internal/p2p/host/...

# DHT 节点发现
go test -v ./internal/p2p/discovery/...

# 节点生命周期
go test -v ./internal/p2p/node/...
```

#### 存储模块测试

```bash
# 本地键值存储
go test -v ./internal/storage/...

# 邮箱系统
go test -v ./internal/mailbox/...

# 公告板
go test -v ./internal/bulletin/...
```

#### 信誉与治理测试

```bash
# 信誉系统
go test -v ./internal/reputation/...

# 投票机制
go test -v ./internal/voting/...

# 指控系统
go test -v ./internal/accusation/...

# 激励机制
go test -v ./internal/incentive/...
```

#### 网络通信测试

```bash
# 消息传递
go test -v ./internal/network/messenger/...

# 可靠传输
go test -v ./internal/network/reliable_transport/...

# 连接管理
go test -v ./internal/network/connection_manager/...

# 广播机制
go test -v ./internal/network/broadcaster/...
```

---

## 🔄 集成测试 (Python)

### 生命周期测试

完整模拟节点从启动到关闭的全生命周期。

```bash
# 基础测试（5个节点）
python scripts/lifecycle_test.py

# 自定义节点数量
python scripts/lifecycle_test.py -n 10

# 跳过编译（使用已有二进制）
python scripts/lifecycle_test.py --skip-build

# 保留日志文件
python scripts/lifecycle_test.py --keep-logs

# 详细输出
python scripts/lifecycle_test.py -v
```

#### 测试场景

生命周期测试涵盖 **16 个核心场景**：

1. **环境准备**
   - ✅ 编译二进制文件
   - ✅ 清理旧数据目录

2. **节点启动** (5 节点)
   - ✅ Genesis 节点启动
   - ✅ 普通节点加入网络
   - ✅ 健康检查通过

3. **网络发现**
   - ✅ DHT 节点发现
   - ✅ 邻居列表构建

4. **存储操作**
   - ✅ 数据存储 (PUT)
   - ✅ 数据获取 (GET)
   - ✅ 跨节点同步

5. **任务系统**
   - ✅ 任务创建与分发
   - ✅ 任务执行与完成

6. **信誉机制**
   - ✅ 信誉值查询
   - ✅ 信誉更新

7. **指控系统**
   - ✅ 提交指控
   - ✅ 指控传播

8. **节点停止**
   - ✅ 优雅关闭
   - ✅ 日志收集

### API 测试

```bash
# HTTP API 接口测试
python scripts/api_test.py
```

---

## 🌐 网络模拟测试

### 多节点网络测试

```bash
# 基础网络模拟（8个节点）
go test -v ./test/integration/ -run TestNetworkSimulation

# 增强版协作测试（6个节点 + HTTP API测试）
go test -v ./test/integration/ -run TestEnhancedNetworkBehaviors

# 网络可扩展性测试（10个节点）
go test -v ./test/integration/ -run TestNetworkScalability

# API接口覆盖完整性评估
go test -v ./test/integration/ -run TestAPICompleteness
```

### 测试场景

#### 基础网络模拟 (TestNetworkSimulation)
- 节点加入/退出
- P2P 连接建立
- DHT 节点发现
- 网络稳定性监控

#### 增强版协作测试 (TestEnhancedNetworkBehaviors)
包含 **10+ 核心行为场景**：

1. **节点与HTTP API启动**
   - ✅ 引导节点 + HTTP API
   - ✅ 普通节点 + HTTP API服务器

2. **节点信息API** (4个接口)
   - ✅ 健康检查 `/health`
   - ✅ 节点状态 `/status`
   - ✅ 节点信息 `/api/v1/node/info`
   - ✅ 对等节点列表 `/api/v1/node/peers`

3. **邻居管理API** (2个接口)
   - ✅ 邻居列表 `/api/v1/neighbor/list`
   - ✅ 最佳邻居 `/api/v1/neighbor/best`

4. **消息传递**
   - ✅ 节点间消息发送
   - ✅ 消息元数据传递

5. **邮箱系统API** (2个接口)
   - ✅ 收件箱查询
   - ✅ 发件箱查询

6. **任务系统API** (2个接口)
   - ✅ 任务创建
   - ✅ 任务列表查询

7. **信誉系统API** (2个接口)
   - ✅ 信誉查询
   - ✅ 信誉排名

8. **公告板API** (2个接口)
   - ✅ 公告发布
   - ✅ 公告搜索

9. **投票系统API**
   - ✅ 提案列表查询

10. **网络拓扑验证**
    - ✅ 连接数统计
    - ✅ 平均连接度分析

#### API覆盖完整性评估 (TestAPICompleteness)

全面评估 **65+ HTTP API接口**覆盖情况：

| 模块 | 接口数 | 说明 |
|------|-------|------|
| 节点管理 | 5 | info, peers, register, health, status |
| 邻居管理 | 5 | list, best, add, remove, ping |
| 消息传递 | 2 | send, receive |
| 邮箱系统 | 6 | send, inbox, outbox, read, mark-read, delete |
| 公告板 | 8 | publish, get, search, subscribe, revoke |
| 任务系统 | 5 | create, status, accept, submit, list |
| 信誉系统 | 4 | query, update, ranking, history |
| 指控系统 | 4 | create, list, detail, analyze |
| 激励机制 | 4 | award, propagate, history, tolerance |
| 投票治理 | 5 | create, list, vote, finalize |
| 超级节点 | 4 | list, candidates, apply, heartbeat |
| 存储系统 | 5 | put, get, delete, list, has |
| 日志系统 | 2 | tail, stream |

**当前覆盖率**: 16/65 核心接口 (~25%)

**未覆盖核心接口**:
- 存储系统 (put/get/delete)
- 指控系统 (create/list)
- 投票治理 (vote/finalize)
- 超级节点管理

---

## 📊 测试报告

### 查看覆盖率

```bash
# 生成覆盖率
go test -coverprofile=coverage.out ./...

# 按包查看覆盖率
go tool cover -func=coverage.out

# 生成 HTML 报告
go tool cover -html=coverage.out -o coverage.html

# 在浏览器中打开
start coverage.html  # Windows
open coverage.html   # macOS
xdg-open coverage.html  # Linux
```

### 查看生命周期测试日志

```bash
# 测试日志位置
test_logs_YYYYMMDD_HHMMSS/
├── test_summary.json          # 测试摘要
├── node_0.log                 # Genesis 节点日志
├── node_1.log                 # 普通节点日志
├── node_2.log
├── node_3.log
└── node_4.log
```

---

## 🐛 调试测试

### 详细日志模式

```bash
# Go 测试详细输出
go test -v -run TestSpecificCase ./internal/module/...

# Python 测试详细输出
python scripts/lifecycle_test.py -v
```

### 单独运行某个测试

```bash
# Go - 运行特定测试函数
go test -v -run TestNodeStart ./internal/p2p/node/

# Go - 运行匹配模式的测试
go test -v -run "TestReputatio.*" ./internal/reputation/
```

### 保留测试环境

```bash
# 生命周期测试保留日志
python scripts/lifecycle_test.py --keep-logs

# 手动启动节点进行调试
./build/node run --data-dir ./debug_data --http-port 18000
```

---

## ✅ 测试最佳实践

### 1. 运行测试前

```bash
# 确保依赖最新
go mod tidy

# 清理旧的测试数据
rm -rf test_data/ test_logs_*/
```

### 2. 提交代码前

```bash
# 运行完整测试套件
go test ./...

# 运行生命周期测试
python scripts/lifecycle_test.py

# 检查覆盖率
go test -cover ./...
```

### 3. 编写新功能时

- 为新模块添加单元测试（`*_test.go`）
- 测试覆盖率应 > 60%
- 集成测试验证端到端功能

### 4. CI/CD 集成

```yaml
# GitHub Actions 示例
- name: Run Go Tests
  run: go test -v -coverprofile=coverage.out ./...

- name: Run Integration Tests
  run: python scripts/lifecycle_test.py --skip-build
```

---

## 🔧 常见问题

### Q: 测试失败 "port already in use"

**A:** 清理残留进程
```bash
# Windows
taskkill /F /IM agentnetwork*.exe

# Linux/macOS
pkill -9 agentnetwork
```

### Q: 生命周期测试超时

**A:** 增加节点启动等待时间
```python
# lifecycle_test.py
NODE_START_TIMEOUT = 30  # 默认 15 秒
```

### Q: DHT 测试失败

**A:** 检查防火墙设置，确保本地环路可用
```bash
# 允许 localhost 通信
# 检查端口范围 9000-9100, 18000-18100
```

### Q: 跨平台测试失败

**A:** 确保平台特定文件存在
```bash
internal/daemon/
├── daemon.go           # 通用逻辑
├── daemon_windows.go   # Windows 特定
└── daemon_unix.go      # Unix 特定
```

---

## 📚 相关资源

- [Go Testing 文档](https://golang.org/pkg/testing/)
- [项目架构](README.md#项目结构)
- [API 文档](api/proto/)
- [开发指南](CONTRIBUTING.md)

---

**测试驱动开发，质量第一！** ✨
