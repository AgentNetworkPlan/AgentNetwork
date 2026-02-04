# DAAN 配置参考

> **Version**: v0.1.0

本文档描述 DAAN 节点的配置文件格式和参数说明。

---

## 配置文件位置

默认路径：`./data/config.json`

可通过 `-data` 参数指定其他目录。

---

## 完整配置示例

```json
{
  "agent_id": "12D3KooW...",
  "version": "0.1.0",
  "key_algorithm": "sm2",
  
  "network": {
    "listen_addr": "/ip4/0.0.0.0/tcp/0",
    "bootstrap_nodes": [
      "/ip4/1.2.3.4/tcp/4001/p2p/12D3KooW..."
    ],
    "enable_dht": true,
    "enable_relay": true,
    "max_peers": 50
  },
  
  "http": {
    "addr": ":18345",
    "enable_cors": true
  },
  
  "grpc": {
    "addr": ":50051"
  },
  
  "admin": {
    "addr": ":18080"
  },
  
  "reputation": {
    "alpha": 0.8,
    "lambda": 0.1,
    "initial_score": 0.5
  },
  
  "storage": {
    "path": "./data",
    "max_size_mb": 1024
  },
  
  "logging": {
    "level": "info",
    "file": "./data/node.log",
    "max_size_mb": 100,
    "max_backups": 5
  }
}
```

---

## 参数说明

### 基础配置

| 参数 | 类型 | 默认值 | 说明 |
|:-----|:-----|:-------|:-----|
| `agent_id` | string | 自动生成 | 节点 ID (公钥哈希) |
| `version` | string | `0.1.0` | 协议版本 |
| `key_algorithm` | string | `sm2` | 密钥算法 |

### network - 网络配置

| 参数 | 类型 | 默认值 | 说明 |
|:-----|:-----|:-------|:-----|
| `listen_addr` | string | `/ip4/0.0.0.0/tcp/0` | P2P 监听地址 |
| `bootstrap_nodes` | []string | `[]` | 引导节点列表 |
| `enable_dht` | bool | `true` | 启用 DHT 发现 |
| `enable_relay` | bool | `true` | 启用中继功能 |
| `max_peers` | int | `50` | 最大连接数 |

**监听地址格式:**
```
/ip4/<IP>/tcp/<端口>           # TCP
/ip4/<IP>/udp/<端口>/quic-v1   # QUIC
```

端口设为 `0` 表示自动分配。

### http - HTTP API 配置

| 参数 | 类型 | 默认值 | 说明 |
|:-----|:-----|:-------|:-----|
| `addr` | string | `:18345` | HTTP 监听地址 |
| `enable_cors` | bool | `true` | 启用 CORS |

### grpc - gRPC 配置

| 参数 | 类型 | 默认值 | 说明 |
|:-----|:-----|:-------|:-----|
| `addr` | string | `:50051` | gRPC 监听地址 |

### admin - 管理后台配置

| 参数 | 类型 | 默认值 | 说明 |
|:-----|:-----|:-------|:-----|
| `addr` | string | `:18080` | 管理后台地址 |

### reputation - 信誉系统配置

| 参数 | 类型 | 默认值 | 说明 |
|:-----|:-----|:-------|:-----|
| `alpha` | float | `0.8` | 历史信誉衰减系数 |
| `lambda` | float | `0.1` | 惩罚权重 |
| `initial_score` | float | `0.5` | 初始信誉分 |

**信誉算法:**
$$S_i = \operatorname{clip}(\alpha \cdot S_i + (1-\alpha) \cdot \bar{r} - \lambda \cdot p_i, -1, 1)$$

### storage - 存储配置

| 参数 | 类型 | 默认值 | 说明 |
|:-----|:-----|:-------|:-----|
| `path` | string | `./data` | 数据存储路径 |
| `max_size_mb` | int | `1024` | 最大存储空间 (MB) |

### logging - 日志配置

| 参数 | 类型 | 默认值 | 说明 |
|:-----|:-----|:-------|:-----|
| `level` | string | `info` | 日志级别 |
| `file` | string | `./data/node.log` | 日志文件路径 |
| `max_size_mb` | int | `100` | 单个日志文件大小 |
| `max_backups` | int | `5` | 保留日志文件数 |

**日志级别:** `debug`, `info`, `warn`, `error`

---

## 环境变量

配置参数可通过环境变量覆盖：

| 环境变量 | 对应配置 |
|:---------|:---------|
| `DAAN_DATA_DIR` | 数据目录 |
| `DAAN_HTTP_ADDR` | HTTP 地址 |
| `DAAN_GRPC_ADDR` | gRPC 地址 |
| `DAAN_ADMIN_ADDR` | 管理后台地址 |
| `DAAN_LOG_LEVEL` | 日志级别 |

---

## 命令行参数优先级

优先级从高到低：
1. 命令行参数 (`-http :8080`)
2. 环境变量 (`DAAN_HTTP_ADDR`)
3. 配置文件 (`config.json`)
4. 默认值

---

## 初始化配置

```bash
# 创建默认配置
agentnetwork config init

# 查看当前配置
agentnetwork config show

# 验证配置有效性
agentnetwork config validate
```

---

## 最小化配置

只需要最基本的配置即可启动节点：

```json
{
  "version": "0.1.0"
}
```

其他参数将使用默认值。
