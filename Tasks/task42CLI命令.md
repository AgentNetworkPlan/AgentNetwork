# DAAN CLI 命令参考

## 概述

可执行程序名称: `agentnetwork`

支持多种子命令模式，提供完整的节点生命周期管理。

---

## 命令一览

```
agentnetwork <命令> [选项]
```

| 命令 | 说明 |
|------|------|
| `start` | 启动节点（后台运行） |
| `stop` | 停止节点 |
| `restart` | 重启节点 |
| `status` | 查看节点状态 |
| `logs` | 查看节点日志 |
| `run` | 前台运行节点（调试用） |
| `token` | 管理访问令牌 |
| `config` | 管理配置文件 |
| `keygen` | 生成密钥对 |
| `health` | 健康检查 |
| `version` | 显示版本信息 |
| `help` | 显示帮助信息 |

---

## 公共选项

以下选项在 `start`, `run`, `restart` 命令中通用：

| 选项 | 默认值 | 说明 |
|------|--------|------|
| `-data` | `./data` | 数据目录 |
| `-key` | `<数据目录>/keys/node.key` | 密钥文件路径 |
| `-listen` | `/ip4/0.0.0.0/tcp/0,/ip4/0.0.0.0/udp/0/quic-v1` | P2P监听地址（逗号分隔） |
| `-bootstrap` | 空 | 引导节点地址（逗号分隔） |
| `-role` | `normal` | 节点角色: `bootstrap`, `relay`, `normal` |
| `-grpc` | `:50051` | gRPC 服务地址 |
| `-http` | `:18345` | HTTP API 服务地址 |
| `-admin` | `:18080` | 管理后台地址 |
| `-admin-token` | 自动生成 | 管理后台访问令牌 |

---

## 命令详解

### 1. start - 启动节点（后台）

在后台启动守护进程模式运行节点。

```bash
agentnetwork start [选项]
```

**示例：**
```bash
# 默认启动
agentnetwork start

# 指定数据目录
agentnetwork start -data ./mydata

# 指定监听地址
agentnetwork start -listen /ip4/0.0.0.0/tcp/9000

# 作为引导节点启动
agentnetwork start -role bootstrap -listen /ip4/0.0.0.0/tcp/4001

# 连接到已有网络
agentnetwork start -bootstrap /ip4/x.x.x.x/tcp/4001/p2p/12D3KooW...

# 指定所有端口
agentnetwork start -grpc :50051 -http :18345 -admin :18080
```

---

### 2. stop - 停止节点

停止正在运行的后台节点。

```bash
agentnetwork stop [-data <数据目录>]
```

**示例：**
```bash
agentnetwork stop
agentnetwork stop -data ./mydata
```

---

### 3. restart - 重启节点

重启节点（相当于 stop + start）。

```bash
agentnetwork restart [选项]
```

**示例：**
```bash
agentnetwork restart
agentnetwork restart -data ./mydata
```

---

### 4. status - 查看状态

查看节点运行状态。

```bash
agentnetwork status [-data <数据目录>] [-json]
```

**选项：**
| 选项 | 说明 |
|------|------|
| `-data` | 数据目录 (默认: ./data) |
| `-json` | JSON 格式输出 |

**示例：**
```bash
agentnetwork status
agentnetwork status -json
```

**输出示例：**
```
======== 节点状态 ========
状态:     运行中
PID:      12345
节点ID:   12D3KooW...
版本:     0.1.0
运行时间: 2h30m
监听地址:
  - /ip4/192.168.1.100/tcp/50001/p2p/12D3KooW...
连接节点: 5
数据目录: ./data
日志文件: ./data/node.log
==========================
```

---

### 5. logs - 查看日志

查看节点运行日志。

```bash
agentnetwork logs [-data <数据目录>] [-n <行数>] [-f]
```

**选项：**
| 选项 | 说明 |
|------|------|
| `-data` | 数据目录 (默认: ./data) |
| `-n` | 显示行数 (默认: 50) |
| `-f` | 实时跟踪 |

**示例：**
```bash
agentnetwork logs              # 显示最后50行
agentnetwork logs -n 100       # 显示最后100行
agentnetwork logs -f           # 实时跟踪日志
```

---

### 6. run - 前台运行

在前台运行节点（调试用），按 Ctrl+C 停止。

```bash
agentnetwork run [选项]
```

**示例：**
```bash
agentnetwork run
agentnetwork run -data ./mydata
agentnetwork run -grpc :50052 -http :18346 -admin :18081
```

---

### 7. token - 令牌管理

管理管理后台的访问令牌。

```bash
agentnetwork token <子命令> [-data <数据目录>]
```

**子命令：**
| 子命令 | 说明 |
|--------|------|
| `show` | 显示当前访问令牌 |
| `refresh` | 刷新（重新生成）访问令牌 |

**示例：**
```bash
agentnetwork token show
agentnetwork token refresh
agentnetwork token refresh -data ./mydata
```

**输出示例：**
```
======== 访问令牌 ========
令牌: 7995fc8815d0c447bfa51d9f0d8a6bdd
管理后台 URL: http://localhost:18080/?token=7995fc8815d0c447bfa51d9f0d8a6bdd
==========================
```

---

### 8. config - 配置管理

管理配置文件。

```bash
agentnetwork config <子命令> [-data <数据目录>] [-force]
```

**子命令：**
| 子命令 | 说明 |
|--------|------|
| `init` | 初始化配置文件 |
| `show` | 显示当前配置 |
| `validate` | 验证配置文件 |

**示例：**
```bash
agentnetwork config init           # 创建默认配置
agentnetwork config init -force    # 强制覆盖
agentnetwork config show           # 显示配置
agentnetwork config validate       # 验证配置
```

---

### 9. keygen - 生成密钥

生成节点密钥对。

```bash
agentnetwork keygen [-data <数据目录>] [-force]
```

**选项：**
| 选项 | 说明 |
|------|------|
| `-data` | 数据目录 (默认: ./data) |
| `-force` | 强制覆盖现有密钥 |

**示例：**
```bash
agentnetwork keygen
agentnetwork keygen -force
agentnetwork keygen -data ./mydata
```

**输出示例：**
```
======== 密钥生成成功 ========
私钥路径: ./data/keys/node.key
节点ID:   12D3KooWAbC123...
公钥(hex): 08011220...
==============================
⚠️  警告: 请妥善保管私钥文件!
```

---

### 10. health - 健康检查

检查节点健康状态。

```bash
agentnetwork health [-data <数据目录>] [-http <HTTP地址>] [-timeout <秒>] [-json]
```

**选项：**
| 选项 | 说明 |
|------|------|
| `-data` | 数据目录 (默认: ./data) |
| `-http` | HTTP 服务地址 (默认: :18345) |
| `-timeout` | 超时时间秒数 (默认: 5) |
| `-json` | JSON 格式输出 |

**示例：**
```bash
agentnetwork health
agentnetwork health -json
agentnetwork health -timeout 10
```

**输出示例：**
```
======== 健康检查 ========
状态: ✅ 健康
进程状态: ✅
HTTP服务: ✅
节点ID: 12D3KooW...
运行时间: 2h30m
==========================
```

---

### 11. version - 版本信息

显示版本信息。

```bash
agentnetwork version
# 或
agentnetwork -v
agentnetwork --version
```

---

### 12. help - 帮助信息

显示帮助信息。

```bash
agentnetwork help
# 或
agentnetwork -h
agentnetwork --help
```

---

## 服务端口说明

| 端口 | 默认值 | 服务 | 用途 |
|------|--------|------|------|
| gRPC | 50051 | gRPC API | 程序化访问（Agent 调用） |
| HTTP | 18345 | HTTP API | RESTful 接口 |
| Admin | 18080 | 管理后台 | Web 管理界面 |
| P2P | 随机 | P2P 网络 | 节点间通信 |

---

## 节点角色

| 角色 | 说明 | 部署建议 |
|------|------|----------|
| `bootstrap` | 网络引导节点 | 3-5 个公网节点 |
| `relay` | NAT 中转节点 | 可与 bootstrap 合并 |
| `normal` | 普通参与节点 | 动态上下线 |

---

## 常用场景

### 场景1: 首次启动

```bash
# 1. 生成密钥
agentnetwork keygen

# 2. 初始化配置（可选）
agentnetwork config init

# 3. 启动节点
agentnetwork start

# 4. 查看状态
agentnetwork status

# 5. 获取管理后台令牌
agentnetwork token show
```

### 场景2: 加入现有网络

```bash
agentnetwork start -bootstrap /ip4/x.x.x.x/tcp/4001/p2p/12D3KooW...
```

### 场景3: 作为引导节点

```bash
agentnetwork start -role bootstrap -listen /ip4/0.0.0.0/tcp/4001
```

### 场景4: 开发调试

```bash
# 前台运行，方便查看日志
agentnetwork run

# 或指定不同端口避免冲突
agentnetwork run -grpc :50052 -http :18346 -admin :18081
```

### 场景5: 多节点本地测试

```bash
# 节点1（引导节点）
agentnetwork run -data ./node1 -role bootstrap -listen /ip4/127.0.0.1/tcp/4001 -grpc :50051 -http :18345 -admin :18080

# 节点2
agentnetwork run -data ./node2 -bootstrap /ip4/127.0.0.1/tcp/4001/p2p/<节点1ID> -grpc :50052 -http :18346 -admin :18081

# 节点3
agentnetwork run -data ./node3 -bootstrap /ip4/127.0.0.1/tcp/4001/p2p/<节点1ID> -grpc :50053 -http :18347 -admin :18082
```

---

## 数据目录结构

```
data/
├── keys/
│   └── node.key          # 节点私钥
├── bulletin/
│   └── bulletin.json     # 公告板数据
├── mailbox/
│   └── mailbox.json      # 邮箱数据
├── admin_token           # 管理后台令牌
├── node.status           # 节点状态
├── node.log              # 节点日志
├── node.log.1            # 轮转日志
└── config.json           # 配置文件（可选）
```

---

## 构建与安装

### 从源码构建

```bash
# 克隆仓库
git clone https://github.com/AgentNetworkPlan/AgentNetwork
cd AgentNetwork

# 安装依赖
go mod tidy

# 构建
go build -o agentnetwork ./cmd/node

# 或使用 Makefile
make build
```

### 下载预编译版本

从 [Releases](https://github.com/AgentNetworkPlan/AgentNetwork/releases) 页面下载。

---

## 注意事项

1. **密钥安全**: 私钥文件 `node.key` 是节点身份凭证，请妥善保管
2. **端口冲突**: 运行多节点时需指定不同端口
3. **防火墙**: 如果需要外部连接，请开放相应端口
4. **令牌**: 管理后台令牌存储在 `data/admin_token`，重启后令牌不变
5. **日志轮转**: 日志文件超过 10MB 会自动轮转

---

*最后更新: 2026-02-04*

