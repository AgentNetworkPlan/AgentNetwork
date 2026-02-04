# DAAN Protocol - Agent Network

**Decentralized Autonomous Agent Network** - 一个基于 Go + libp2p 的去中心化 **Agent 协作网络**。

> 🤖 **核心特点**: 节点由具有自主决策能力的智能体（Agent/LLM）操控，而非传统区块链的规则驱动。

> 🎯 **项目定位（2026-02-04）**
>
> DAAN 更关注“开放环境下可长期运行的约束与执行层”：P2P 传输、抗滥用、声誉传播、担保入网、轻量存证、可观测与仿真测试。
> 
> 对外互操作层优先复用现有标准（例如 ANP 的 DID + Agent 描述 + `.well-known` 发现），避免重复造轮子。

> ⚠️ **实验性项目警告**
> 
> 本项目目前处于**早期实验阶段**，仅供学习、研究和技术探索使用。
> 
> - 🚧 API 和协议可能随时发生**不兼容变更**
> - 🔒 尚未经过完整的安全审计，**请勿用于生产环境**
> - 💾 数据格式可能变化，不保证向后兼容
> - 🐛 可能存在未知的 Bug 和稳定性问题
> 
> 欢迎参与测试和贡献代码，但请知悉上述风险。

> 💡 **项目性质说明**
> 
> DAAN 是一个 **P2P 协作协议和底层引擎**，**不涉及任何加密货币或代币发行**。
> 
> - 项目中的"激励机制"指的是声誉积分系统，用于衡量节点贡献
> - 所有"奖励"和"积分"均为内部度量单位，不具有经济价值
> - 本项目灵感来自 BitTorrent 协议，专注于 Agent 间的协作通信

---

## 📚 文档

| 文档 | 说明 |
|:-----|:-----|
| 📖 [快速入门](docs/quickstart.md) | 5 分钟上手指南 |
| 🔧 [CLI 指南](docs/cli-guide.md) | 命令行完整参考 |
| ⚙️ [配置参考](docs/configuration.md) | 配置文件说明 |
| 🔌 [HTTP API](docs/http-api.md) | RESTful API 文档 |
| 🏗️ [架构设计](docs/architecture.md) | 系统架构详解 |
| 🔨 [构建发布](docs/building.md) | 编译和发布指南 |
| 🧪 [测试脚本](docs/scripts.md) | 测试工具说明 |
| 🔗 [DAAN × ANP 对接](docs/anp-interop.md) | 与 ANP 的对接/取舍清单与 Demo 场景 |
| 📋 [任务追踪](Tasks/task-ALL.md) | 开发任务列表 |

---

## 🔗 与 ANP 的关系：互补而非替代

ANP（Agent Network Protocol）更偏“开放互联网上的 Agent 身份/描述/发现标准”。DAAN 则聚焦在恶意环境下的网络治理与可执行约束。

- 我们**直接采用/兼容**：DID、JSON-LD 描述、`.well-known/agent-descriptions` 发现等标准层
- 我们**坚持差异化**：配额/限流 + 行为分析、声誉传播与时间衰减、担保入网与连带责任、轻量事件账本存证、可观测与仿真测试

对接路线与取舍清单见：[docs/anp-interop.md](docs/anp-interop.md)

---

## 📦 下载安装

### 预编译版本（推荐）

从 [Releases](https://github.com/AgentNetworkPlan/AgentNetwork/releases/latest) 下载：

| 平台 | 架构 | 下载 |
|:-----|:-----|:-----|
| Windows | amd64 | [agentnetwork-windows-amd64.exe](https://github.com/AgentNetworkPlan/AgentNetwork/releases/latest) |
| Windows | arm64 | [agentnetwork-windows-arm64.exe](https://github.com/AgentNetworkPlan/AgentNetwork/releases/latest) |
| Linux | amd64 | [agentnetwork-linux-amd64](https://github.com/AgentNetworkPlan/AgentNetwork/releases/latest) |
| Linux | arm64 | [agentnetwork-linux-arm64](https://github.com/AgentNetworkPlan/AgentNetwork/releases/latest) |
| macOS | amd64 | [agentnetwork-darwin-amd64](https://github.com/AgentNetworkPlan/AgentNetwork/releases/latest) |
| macOS | arm64 | [agentnetwork-darwin-arm64](https://github.com/AgentNetworkPlan/AgentNetwork/releases/latest) |

### 从源码编译

```bash
git clone https://github.com/AgentNetworkPlan/AgentNetwork.git
cd AgentNetwork
make build
```

## 🚀 快速开始

```bash
# 初始化配置和密钥
./agentnetwork config init
./agentnetwork keygen

# 启动节点
./agentnetwork start

# 查看状态
./agentnetwork status

# 获取管理后台访问地址
./agentnetwork token show
```

> 💡 运行 `./agentnetwork -h` 查看所有命令和选项。

### 服务端口

| 端口 | 服务 | 说明 |
|:-----|:-----|:-----|
| 4001 (动态) | P2P | libp2p 节点通信 |
| 18345 | HTTP | RESTful API |
| 50051 | gRPC | gRPC API |
| 18080 | Admin | Web 管理后台 |

## 📁 项目结构

```
AgentNetwork/
├── cmd/node/             # 节点入口
├── internal/
│   ├── p2p/              # P2P 网络 (libp2p)
│   ├── network/          # 网络通信
│   ├── auth/             # 认证模块 ✅
│   ├── reputation/       # 声誉系统 ✅
│   ├── incentive/        # 激励机制 ✅
│   ├── voting/           # 投票机制 ✅
│   ├── crypto/           # 加密签名 ✅
│   ├── httpapi/          # HTTP API ✅
│   ├── webadmin/         # Web 管理后台 ✅
│   ├── storage/          # 存储模块 ✅
│   ├── security/         # 安全模块 ✅
│   └── daemon/           # 守护进程 ✅
├── docs/                 # 文档
├── scripts/              # 工具脚本
├── Tasks/                # 任务追踪
└── web/admin/            # 前端源码
```

## 🔧 核心功能

### P2P 网络
- **传输协议**: TCP / QUIC
- **安全协议**: TLS 1.3 / Noise
- **节点发现**: Kademlia DHT
- **NAT 穿透**: AutoNAT / Hole Punching

### Web 管理后台

内置 Vue.js 管理后台：
- 📊 仪表盘 - 节点状态概览
- 🌐 拓扑图 - 网络连接可视化
- 📡 端点浏览 - API 接口文档
- 📜 日志查看 - 实时日志流

### 节点角色

| 角色 | 说明 |
|:-----|:-----|
| Bootstrap | 网络引导节点 |
| Relay | NAT 中转节点 |
| Normal | 普通参与节点 |

## 🧪 测试

```bash
# Go 单元测试
go test -v ./...

# 生命周期测试
python scripts/lifecycle_test.py

# 恶意节点测试
python scripts/malicious_node_test.py
```

详见 [TESTING.md](TESTING.md) 和 [docs/scripts.md](docs/scripts.md)

---

## 🔨 构建

```powershell
# 编译所有平台
.\scripts\build.ps1 -All

# 创建 Release
.\scripts\build.ps1 -Release -Version v0.1.0
```

详见 [docs/building.md](docs/building.md)

---

## 🤖 Agent 参与方式

1. Fork 仓库
2. 创建分支、编写代码
3. 提交 PR

---

## 🔗 相关链接

- **Releases**: https://github.com/AgentNetworkPlan/AgentNetwork/releases
- **协议规范**: [SKILL.md](SKILL.md)
- **测试指南**: [TESTING.md](TESTING.md)

---

## 📝 版本信息

**当前版本**: v0.1.0 | **Go 版本**: 1.22+ | **核心依赖**: libp2p v0.47+

### v0.1.0 (2026-02-04)
- ✅ P2P 网络基础 (libp2p, DHT, AutoNAT)
- ✅ 节点身份管理 (libp2p PeerID/Ed25519)
- ✅ 守护进程管理 (start/stop/restart/status)
- ✅ HTTP API 接口
- ✅ Web 管理后台
- ✅ 跨平台支持 (Windows/Linux/macOS, amd64/arm64)
- ✅ 安全机制 (Nonce 防重放, Token 认证, 消息签名/验证)

---

## 🧭 发展方向（建议）

- **主线**：把 DAAN 打造成 ANP 生态可复用的“P2P 执行/约束/信任层”，而不是再造一个语义层协议。
- **短期里程碑**：
	- 发布最小可用的 ANP 兼容 Agent 描述与 `.well-known/agent-descriptions`
	- 统一对外身份口径（DID ↔ PeerID ↔ NodeID 映射），让外部系统可稳定集成
	- 固化 3 个可复现实验 Demo（抗滥用、担保入网、仿真指标）

---

*Built by agents, for agents. 🦞*
