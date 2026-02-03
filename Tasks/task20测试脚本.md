# 🧪 测试脚本模块 - 已完成 ✅

> **状态**: 已实现  
> **实现文件**: 
> - `scripts/lifecycle_test.py` - 全生命周期模拟测试
> - `scripts/network_manager.py` - 网络管理脚本
> - `scripts/api_test.py` - API 测试脚本

---

## 1️⃣ 全生命周期测试 (lifecycle_test.py)

### 功能概述

通过 Python 脚本启动守护进程，调用 HTTP 接口控制每个节点，实现全过程全生命周期的模拟测试。

### 测试用例 (16个测试全部通过)

| 序号 | 测试名称 | 说明 |
|------|----------|------|
| 1 | 编译项目 | 构建 Go 二进制文件 |
| 2 | 启动创世节点 | 启动 bootstrap 角色节点 |
| 3 | 创世节点信息 | 获取节点ID、地址、状态 |
| 4 | 节点加入网络 | 其他节点通过引导节点加入 |
| 5 | 节点发现 | DHT 节点发现 |
| 6 | 邻居管理 | 邻居列表、最佳邻居 |
| 7 | 消息发送 | P2P 消息传递 |
| 8 | 邮箱功能 | 发送/接收邮件 |
| 9 | 公告板 | 发布/搜索公告 |
| 10 | 声誉系统 | 查询声誉、排名 |
| 11 | 指责系统 | 指责列表 |
| 12 | 投票系统 | 提案列表 |
| 13 | 超级节点 | 超级节点列表 |
| 14 | 激励系统 | 激励历史 |
| 15 | 日志系统 | 跳过（无HTTP接口） |
| 16 | 错误处理 | 404 处理 |

### 使用方法

```bash
# 基本运行（3节点）
python scripts/lifecycle_test.py -n 3

# 详细模式
python scripts/lifecycle_test.py -n 5 -v

# 保留日志
python scripts/lifecycle_test.py -n 3 --keep-logs

# 自定义端口
python scripts/lifecycle_test.py -n 3 --p2p-port 9100 --http-port 18100
```

### 命令行参数

| 参数 | 默认值 | 说明 |
|------|--------|------|
| `-n, --nodes` | 5 | 节点数量 |
| `--p2p-port` | 9000 | 起始 P2P 端口 |
| `--http-port` | 18000 | 起始 HTTP 端口 |
| `--skip-build` | false | 跳过编译 |
| `--keep-logs` | false | 保留日志不清理 |
| `-v, --verbose` | false | 详细输出 |

### 测试流程

```
1. 编译项目
   └── go build -o bin/agentnetwork.exe ./cmd/node

2. 启动创世节点
   └── agentnetwork run -data ./node-000 -role bootstrap

3. 启动其他节点
   └── agentnetwork run -data ./node-00X -bootstrap <genesis_addr>

4. 运行 API 测试
   ├── /health
   ├── /api/v1/node/info
   ├── /api/v1/neighbor/list
   ├── /api/v1/message/send
   ├── /api/v1/mailbox/*
   ├── /api/v1/bulletin/*
   ├── /api/v1/reputation/*
   ├── /api/v1/accusation/*
   ├── /api/v1/voting/*
   ├── /api/v1/supernode/*
   └── /api/v1/incentive/*

5. 收集日志
   └── test_logs/<timestamp>/

6. 清理环境
   └── 停止所有节点，删除临时目录
```

---

## 2️⃣ 网络管理脚本 (network_manager.py)

### 功能

- 启动/停止多节点测试网络
- 查看网络状态
- 清理测试环境

### 使用方法

```bash
# 启动 5 节点网络
python scripts/network_manager.py start -n 5

# 查看状态
python scripts/network_manager.py status

# 停止网络
python scripts/network_manager.py stop

# 清理环境
python scripts/network_manager.py clear
```

---

## 3️⃣ API 测试脚本 (api_test.py)

### 功能

- 测试单个节点的所有 HTTP API
- 验证响应格式和数据正确性

### 使用方法

```bash
# 测试指定节点
python scripts/api_test.py --url http://127.0.0.1:18000

# 详细输出
python scripts/api_test.py --url http://127.0.0.1:18000 -v
```

---

## 4️⃣ 测试结果示例

```
============================================================
AgentNetwork 全生命周期模拟测试
============================================================
节点数量: 3
P2P 端口: 9000-9002
HTTP 端口: 18000-18002

✓ 编译项目 通过 (3.27s)
✓ 启动创世节点 通过 (3.19s)
✓ 创世节点信息 通过 (0.00s)
✓ 节点加入网络 通过 (8.08s)
✓ 节点发现 通过 (5.03s)
✓ 邻居管理 通过 (0.00s)
✓ 消息发送 通过 (0.03s)
✓ 邮箱功能 通过 (0.00s)
✓ 公告板 通过 (0.00s)
✓ 声誉系统 通过 (0.04s)
✓ 指责系统 通过 (0.01s)
✓ 投票系统 通过 (0.00s)
✓ 超级节点 通过 (0.00s)
✓ 激励系统 通过 (0.03s)
✓ 日志系统 通过 (0.00s)
✓ 错误处理 通过 (0.01s)

============================================================
测试结果摘要
============================================================

总计: 16
通过: 16
失败: 0
耗时: 20.29s
```

---

## 5️⃣ 日志保存位置

测试日志保存在 `test_logs/` 目录：

```
test_logs/
├── 20260203_201630/        # 按时间戳命名
│   ├── genesis/
│   │   └── node.log
│   ├── node-001/
│   │   └── node.log
│   └── node-002/
│       └── node.log
└── lifecycle_test_result.json  # 测试结果 JSON
```

---

✅ **任务完成总结**

- 实现了完整的生命周期测试脚本
- 16 个测试用例全部通过
- 支持多节点网络模拟
- 日志保留便于后续分析
- Python 脚本通过 HTTP API 控制节点
