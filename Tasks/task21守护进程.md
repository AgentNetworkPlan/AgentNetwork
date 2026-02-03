# ⚙️ 节点守护进程管理 - 已完成 ✅

> **状态**: 已实现  
> **实现方式**: Go原生实现，不依赖Python  
> **文件位置**: 
> - `internal/daemon/daemon.go` - 守护进程核心模块
> - `internal/daemon/daemon_test.go` - 单元测试
> - `cmd/node/main.go` - 命令行入口

---

## 1️⃣ 设计目标

1. **后台运行**：节点可作为守护进程运行，不占用命令行 ✅
2. **命令管理**：提供 `start/stop/status/restart/logs` 命令 ✅
3. **日志记录**：节点运行日志持久化，支持轮转 ✅
4. **安全启动**：确保节点启动前验证配置和密钥 ✅
5. **跨平台**：支持 Linux、Windows、MacOS ✅
6. **无Python依赖**：纯Go实现，节点程序自行管理 ✅

---

## 2️⃣ 命令使用

### 基本命令

```bash
# 启动节点（后台运行）
agentnetwork start

# 停止节点
agentnetwork stop

# 重启节点
agentnetwork restart

# 查看状态
agentnetwork status
agentnetwork status -json  # JSON格式输出

# 查看日志
agentnetwork logs          # 最后50行
agentnetwork logs -n 100   # 最后100行
agentnetwork logs -f       # 实时跟踪

# 前台运行（调试用）
agentnetwork run

# 版本信息
agentnetwork version
```

### 高级选项

```bash
# 指定数据目录
agentnetwork start -data /path/to/data

# 指定监听地址
agentnetwork start -listen /ip4/0.0.0.0/tcp/9000

# 指定引导节点
agentnetwork start -bootstrap /ip4/1.2.3.4/tcp/9000/p2p/QmXXX

# 指定节点角色
agentnetwork start -role bootstrap  # bootstrap/relay/normal

# 组合使用
agentnetwork start -data ./node1 -listen /ip4/0.0.0.0/tcp/9001 -role relay
```

---

## 3️⃣ 实现架构

### 3.1 目录结构

```
data/
├── logs/
│   ├── node.log           # 当前日志
│   ├── node.log.1         # 轮转日志
│   └── node.log.2
├── node.pid               # PID文件
├── node.status            # 状态文件(JSON)
└── keys/
    └── node.key           # 节点密钥
```

### 3.2 核心模块 (daemon.go)

```go
// 配置
type Config struct {
    DataDir      string  // 数据目录
    LogFile      string  // 日志文件名
    PidFile      string  // PID文件名
    MaxLogSizeMB int     // 最大日志大小(MB)
    MaxLogFiles  int     // 最大日志文件数
}

// 状态信息
type NodeStatus struct {
    Running     bool      // 是否运行中
    PID         int       // 进程ID
    StartTime   time.Time // 启动时间
    Uptime      string    // 运行时长
    NodeID      string    // 节点ID
    Version     string    // 版本
    ListenAddrs []string  // 监听地址
    PeerCount   int       // 连接节点数
    DataDir     string    // 数据目录
    LogFile     string    // 日志文件
}

// 主要方法
func (d *Daemon) Start() (bool, error)  // 启动守护进程
func (d *Daemon) Stop() error           // 停止守护进程
func (d *Daemon) Restart() error        // 重启
func (d *Daemon) Status() *NodeStatus   // 获取状态
func (d *Daemon) Logs(n int, follow bool) error  // 查看日志
func (d *Daemon) RotateLogs() error     // 轮转日志
func (d *Daemon) WriteStatus(s *NodeStatus)  // 写入状态
func (d *Daemon) Cleanup()              // 清理资源
```

### 3.3 跨平台实现

| 功能 | Linux/MacOS | Windows |
|-----|-------------|---------|
| 进程分离 | fork + setsid | CREATE_NEW_PROCESS_GROUP |
| 信号停止 | SIGTERM | taskkill /PID |
| 进程检测 | kill -0 | tasklist /FI |
| 守护标记 | AGENTNETWORK_DAEMON=1 环境变量 |

---

## 4️⃣ 运行流程

```
用户: agentnetwork start
        │
        ▼
检查 PID 文件 → 是否已运行？
        │
    ┌───┴───┐
    是       否
    │        │
返回错误    创建子进程
            │
            ▼
父进程:  输出启动信息 → 退出
            │
子进程:  设置 AGENTNETWORK_DAEMON=1
            │
            ▼
    写入 PID 文件
            │
            ▼
    初始化节点:
      - 加载密钥
      - 启动P2P网络
      - 启动gRPC服务
            │
            ▼
    写入状态文件 (node.status)
            │
            ▼
    每10秒更新状态
    定期轮转日志
            │
            ▼
    等待 SIGTERM/SIGINT
            │
            ▼
    清理并退出


用户: agentnetwork status
        │
        ▼
读取 PID 文件 → 检查进程存活
        │
        ▼
读取状态文件 → 格式化输出


用户: agentnetwork stop
        │
        ▼
读取 PID 文件
        │
        ▼
发送 SIGTERM (Linux) / taskkill (Windows)
        │
        ▼
等待进程退出 (最多10秒)
        │
        ▼
若未退出 → 强制 SIGKILL / taskkill /F
```

---

## 5️⃣ 状态输出示例

```
======== 节点状态 ========
状态:     运行中
PID:      12345
节点ID:   QmYjNKx...abc123
版本:     0.1.0
运行时间: 2h30m15s
监听地址:
  - /ip4/192.168.1.100/tcp/9000/p2p/QmYjNKx...abc123
  - /ip4/192.168.1.100/udp/9000/quic-v1/p2p/QmYjNKx...abc123
连接节点: 5
数据目录: ./data
日志文件: ./data/logs/node.log
==========================
```

---

## 6️⃣ 日志管理

### 日志轮转策略

- 单个日志文件最大: 10MB (可配置)
- 保留文件数量: 5个 (可配置)
- 轮转触发: 每10秒检查一次

### 日志文件命名

```
node.log      # 当前日志
node.log.1    # 上一个
node.log.2    # 更早的
...
node.log.5    # 最旧的 (超过后删除)
```

---

## 7️⃣ 测试覆盖

单元测试位于 `internal/daemon/daemon_test.go`:

- ✅ TestDefaultConfig - 默认配置
- ✅ TestNew - 创建实例
- ✅ TestPaths - 路径生成
- ✅ TestIsDaemonProcess - 守护进程检测
- ✅ TestIsRunning - 运行状态检测
- ✅ TestStatus - 状态获取
- ✅ TestWriteStatus - 状态写入
- ✅ TestCleanup - 资源清理
- ✅ TestEnsureDir - 目录创建
- ✅ TestTailLines - 日志尾部读取
- ✅ TestRotateLogs - 日志轮转
- ✅ TestLogsFileNotExist - 日志不存在处理
- ✅ TestNodeStatus - 状态结构
- ✅ TestStartAlreadyRunning - 重复启动检测
- ✅ TestStopNotRunning - 停止未运行检测

运行测试:
```bash
go test ./internal/daemon/... -v
```

---

## 8️⃣ 与Python脚本对比

| 特性 | Python脚本 | Go原生实现 |
|-----|-----------|-----------|
| 依赖 | 需要Python环境 | 无外部依赖 |
| 部署 | 需要额外文件 | 单一二进制 |
| 性能 | 启动较慢 | 启动快速 |
| 维护 | 两套代码 | 统一代码库 |
| 跨平台 | 需处理兼容性 | 编译时处理 |

---

✅ **任务完成总结**

- 实现了完整的守护进程管理功能
- 支持 start/stop/restart/status/logs 命令
- 支持 Linux/Windows/MacOS 三平台
- 日志轮转和状态持久化
- 无Python依赖，纯Go实现
- 完整的单元测试覆盖
