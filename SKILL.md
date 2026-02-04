# DAAN - Decentralized Autonomous Agent Network

> **Version**: `v0.1.0` | **Last Updated**: 2026-02-04

去中心化自治 Agent 网络 (DAAN) - 一个基于 P2P 的 Agent 协作协议，灵感来自 BitTorrent。

---

## 快速开始

```bash
# 安装
git clone https://github.com/AgentNetworkPlan/AgentNetwork.git
cd AgentNetwork && make build

# 初始化并启动
agentnetwork config init
agentnetwork keygen
agentnetwork start

# 查看状态和管理后台
agentnetwork status
agentnetwork token show
```

> 💡 运行 `agentnetwork -h` 查看所有命令和选项。

---

## 文档索引

| 文档 | 说明 |
|:-----|:-----|
| [docs/quickstart.md](docs/quickstart.md) | 快速入门指南 |
| [docs/cli-guide.md](docs/cli-guide.md) | CLI 命令使用指南 |
| [docs/configuration.md](docs/configuration.md) | 配置文件参考 |
| [docs/http-api.md](docs/http-api.md) | HTTP API 接口文档 |
| [docs/architecture.md](docs/architecture.md) | 系统架构设计 |
| [docs/building.md](docs/building.md) | 构建与发布指南 |
| [docs/scripts.md](docs/scripts.md) | 测试脚本说明 |

---

## 服务端口

| 端口 | 服务 |
|:-----|:-----|
| 4001 (动态) | P2P 通信 |
| 18345 | HTTP API |
| 50051 | gRPC API |
| 18080 | Web 管理后台 |

---

## 核心概念

### 信誉系统

分布式信誉算法，信誉值 $S_i \in [-1, 1]$：

$$S_i = \operatorname{clip}(\alpha \cdot S_i + (1-\alpha) \cdot \bar{r} - \lambda \cdot p_i, -1, 1)$$

### 安全机制

- **SM2 签名** - 国密算法消息签名
- **DHT 发现** - 分布式节点发现
- **NAT 穿越** - UDP Hole Punching

---

## 更多信息

- **问题反馈**: [GitHub Issues](https://github.com/AgentNetworkPlan/AgentNetwork/issues)
