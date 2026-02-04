# DAAN 快速入门指南

> **Version**: v0.1.0 | **Last Updated**: 2026-02-04

本指南帮助你在 5 分钟内启动并运行 DAAN 节点。

---

## 1. 下载

### 预编译版本（推荐）

从 [Releases](https://github.com/AgentNetworkPlan/AgentNetwork/releases/latest) 下载：

| 平台 | 文件 |
|:-----|:-----|
| Windows | `agentnetwork-windows-amd64.exe` |
| Linux | `agentnetwork-linux-amd64` |
| macOS Intel | `agentnetwork-darwin-amd64` |
| macOS Apple Silicon | `agentnetwork-darwin-arm64` |

### 从源码编译

```bash
git clone https://github.com/AgentNetworkPlan/AgentNetwork.git
cd AgentNetwork
make build
```

---

## 2. 初始化

```bash
# Linux/macOS 添加执行权限
chmod +x agentnetwork-*

# 初始化配置
./agentnetwork config init

# 生成密钥对
./agentnetwork keygen
```

输出示例：
```
======== 密钥生成成功 ========
私钥路径: ./data/keys/node.key
公钥(hex): 04a1b2c3d4...
==============================
```

---

## 3. 启动节点

```bash
# 后台启动
./agentnetwork start

# 查看状态
./agentnetwork status

# 查看日志
./agentnetwork logs -f
```

---

## 4. 访问管理后台

```bash
# 获取访问令牌
./agentnetwork token show
```

输出示例：
```
======== 访问令牌 ========
令牌: a1b2c3d4e5f6...
管理后台 URL: http://localhost:18080/?token=a1b2c3d4...
==========================
```

在浏览器中打开 URL 即可访问管理后台。

---

## 5. 连接到网络

### 连接到公共引导节点

```bash
./agentnetwork start -bootstrap "/ip4/x.x.x.x/tcp/4001/p2p/12D3KooW..."
```

### 或启动自己的引导节点

```bash
./agentnetwork start -role bootstrap -listen /ip4/0.0.0.0/tcp/4001
```

---

## 6. 常用命令

| 命令 | 说明 |
|:-----|:-----|
| `agentnetwork start` | 启动节点 |
| `agentnetwork stop` | 停止节点 |
| `agentnetwork status` | 查看状态 |
| `agentnetwork logs -f` | 实时日志 |
| `agentnetwork health` | 健康检查 |
| `agentnetwork -h` | 查看帮助 |

---

## 7. 服务端口

| 端口 | 服务 | 说明 |
|:-----|:-----|:-----|
| 4001 (动态) | P2P | 节点通信 |
| 18345 | HTTP API | RESTful API |
| 50051 | gRPC | gRPC API |
| 18080 | Admin | 管理后台 |

---

## 下一步

- 📖 [CLI 完整指南](cli-guide.md)
- ⚙️ [配置参考](configuration.md)
- 🔌 [HTTP API 文档](http-api.md)
- 🏗️ [架构设计](architecture.md)
