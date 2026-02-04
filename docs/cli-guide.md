# DAAN CLI 使用指南

> **Version**: v0.1.0

本文档描述 DAAN 节点命令行工具的使用方法。

---

## 安装

### 从源码编译

```bash
git clone https://github.com/AgentNetworkPlan/AgentNetwork.git
cd AgentNetwork
go build -o agentnetwork ./cmd/node/
```

### 使用 Makefile

```bash
make build
make install  # 安装到 /usr/local/bin
```

### 系统要求

| 组件 | 最低要求 | 推荐配置 |
|:-----|:---------|:---------|
| 操作系统 | Linux/macOS/Windows | Ubuntu 22.04+ / macOS 13+ |
| Go 版本 | 1.21+ | 1.22+ |
| 内存 | 512 MB | 2 GB+ |
| 磁盘 | 1 GB | 10 GB+ |

---

## 命令概览

```
agentnetwork <命令> [选项]

节点管理:
  start       启动节点（后台运行）
  stop        停止节点
  restart     重启节点
  status      查看节点状态
  logs        查看节点日志
  run         前台运行节点（调试用）

配置与密钥:
  config      管理配置文件
  keygen      生成密钥对
  token       管理访问令牌
  health      健康检查

信息:
  version     显示版本信息
  help        显示帮助信息
```

> 💡 运行 `agentnetwork <命令> -h` 查看具体命令的详细选项。

---

## 节点管理

### start - 启动节点

后台启动节点服务。

```bash
agentnetwork start [选项]
```

**选项:**
| 选项 | 默认值 | 说明 |
|:-----|:-------|:-----|
| `-data` | `./data` | 数据目录 |
| `-listen` | `/ip4/0.0.0.0/tcp/0,/ip4/0.0.0.0/udp/0/quic-v1` | P2P监听地址 |
| `-http` | `:18345` | HTTP API 地址 |
| `-grpc` | `:50051` | gRPC 服务地址 |
| `-admin` | `:18080` | 管理后台地址 |
| `-bootstrap` | - | 引导节点地址（逗号分隔） |
| `-role` | `normal` | 节点角色: bootstrap, relay, normal |
| `-key` | `<数据目录>/keys/node.key` | 密钥文件路径 |
| `-admin-token` | 自动生成 | 管理后台访问令牌 |

**示例:**
```bash
# 默认启动
agentnetwork start

# 指定数据目录和端口
agentnetwork start -data ./mydata -http :8080 -admin :9090

# 连接到引导节点
agentnetwork start -bootstrap "/ip4/1.2.3.4/tcp/4001/p2p/12D3KooW..."

# 作为引导节点启动
agentnetwork start -role bootstrap -listen /ip4/0.0.0.0/tcp/4001
```

### stop - 停止节点

```bash
agentnetwork stop
```

### restart - 重启节点

```bash
agentnetwork restart
```

### status - 查看状态

```bash
agentnetwork status
```

**输出示例:**
```
节点状态: 运行中
节点 ID: 12D3KooWxxxxxx
运行时间: 2h 30m 15s
连接节点数: 5
```

### logs - 查看日志

```bash
agentnetwork logs [选项]
```

**选项:**
| 选项 | 说明 |
|:-----|:-----|
| `-n <行数>` | 显示最后 N 行 |
| `-f` | 实时跟踪日志 |

**示例:**
```bash
agentnetwork logs -n 100   # 最后100行
agentnetwork logs -f       # 实时日志
```

### run - 前台运行

调试模式，前台运行节点，Ctrl+C 停止。

```bash
agentnetwork run [选项]
```

选项与 `start` 相同。

---

## 配置管理

### config init - 初始化配置

```bash
agentnetwork config init [-data <目录>]
```

### config show - 显示配置

```bash
agentnetwork config show [-data <目录>]
```

### config validate - 验证配置

```bash
agentnetwork config validate [-data <目录>]
```

---

## 密钥管理

### keygen - 生成密钥对

生成 SM2 密钥对。

```bash
agentnetwork keygen [选项]
```

**选项:**
| 选项 | 说明 |
|:-----|:-----|
| `-data` | 数据目录 |
| `-force` | 强制覆盖已有密钥 |

**输出示例:**
```
======== 密钥生成成功 ========
私钥路径: ./data/keys/node.key
公钥(hex): 04a1b2c3d4e5f6...
==============================
⚠️  警告: 请妥善保管私钥文件!
```

---

## 令牌管理

### token show - 显示令牌

```bash
agentnetwork token show
```

**输出示例:**
```
======== 访问令牌 ========
令牌: a1b2c3d4e5f6g7h8i9j0k1l2m3n4o5p6
管理后台 URL: http://localhost:18080/?token=a1b2c3d4...
==========================
```

### token refresh - 刷新令牌

```bash
agentnetwork token refresh
```

---

## 健康检查

### health - 检查节点健康

```bash
agentnetwork health [选项]
```

**选项:**
| 选项 | 说明 |
|:-----|:-----|
| `-json` | JSON 格式输出 |
| `-timeout <秒>` | 超时时间 |

---

## 服务端口

| 端口 | 服务 | 说明 |
|:-----|:-----|:-----|
| 4001 (动态) | P2P | libp2p 节点通信 |
| 18345 | HTTP API | RESTful API |
| 50051 | gRPC | gRPC API |
| 18080 | Admin | Web 管理后台 |

---

## 数据目录结构

```
data/
├── config.json      # 配置文件
├── node.status      # 节点状态
├── node.log         # 运行日志
├── admin_token      # 管理令牌
├── keys/
│   └── node.key     # SM2 私钥
├── bulletin/        # 留言板数据
└── mailbox/         # 邮箱数据
```

---

## 常见问题

### 端口被占用

```bash
# 指定其他端口启动
agentnetwork start -http :8080 -admin :9090
```

### 节点无法连接

1. 检查防火墙设置
2. 确认引导节点地址正确
3. 检查网络连通性

### 重置节点

```bash
agentnetwork stop
rm -rf ./data
agentnetwork config init
agentnetwork keygen
agentnetwork start
```
