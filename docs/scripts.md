# DAAN 测试脚本说明

> **Version**: v0.1.0 | **Last Updated**: 2026-02-04

本文档描述 `scripts/` 目录下各测试脚本的用途和使用方法。

---

## 脚本一览

| 脚本 | 用途 | 语言 |
|:-----|:-----|:-----|
| `build.ps1` | 跨平台构建和发布 | PowerShell |
| `lifecycle_test.py` | 节点生命周期测试 | Python |
| `cluster_manager.py` | 集群管理工具 | Python |
| `malicious_node_test.py` | 恶意节点测试 | Python |
| `api_test.py` | HTTP API 测试 | Python |
| `network_manager.py` | 网络管理工具 | Python |
| `generate_keypair.py` | 密钥对生成 | Python |
| `send_heartbeat.py` | 心跳发送测试 | Python |
| `frontend_test.py` | 前端功能测试 | Python |

---

## 构建脚本

### build.ps1

跨平台构建和 GitHub Release 脚本。

```powershell
# 编译所有平台
.\scripts\build.ps1 -All

# 创建 Release
.\scripts\build.ps1 -Release -Version v0.1.0

# 查看帮助
.\scripts\build.ps1 -Help
```

详见 [building.md](building.md)

---

## 测试脚本

### lifecycle_test.py

节点生命周期完整测试，包含 16 个测试场景。

```bash
# 默认运行 (5 节点)
python scripts/lifecycle_test.py

# 自定义节点数
python scripts/lifecycle_test.py -n 10

# 跳过编译
python scripts/lifecycle_test.py --skip-build

# 保留日志
python scripts/lifecycle_test.py --keep-logs

# 详细输出
python scripts/lifecycle_test.py -v
```

**测试场景**:
- 节点启动与健康检查
- DHT 节点发现
- 数据存储与获取
- 任务创建与执行
- 信誉查询与更新
- 指控提交与传播
- 优雅关闭

### cluster_manager.py

集群管理工具，用于启动/停止/管理多节点集群。

```bash
# 启动 5 节点集群
python scripts/cluster_manager.py start -n 5

# 查看集群状态
python scripts/cluster_manager.py status

# 停止集群
python scripts/cluster_manager.py stop

# 清理数据
python scripts/cluster_manager.py clean
```

### malicious_node_test.py

恶意节点行为测试，验证网络安全机制。

```bash
# 运行恶意节点测试
python scripts/malicious_node_test.py

# 指定测试场景
python scripts/malicious_node_test.py --scenario sybil
python scripts/malicious_node_test.py --scenario spam
python scripts/malicious_node_test.py --scenario replay
```

**测试场景**:
- Sybil 攻击检测
- 消息洪泛防护
- 重放攻击检测
- 声誉操纵检测

### api_test.py

HTTP API 接口测试。

```bash
# 测试所有 API
python scripts/api_test.py

# 测试特定模块
python scripts/api_test.py --module health
python scripts/api_test.py --module reputation
python scripts/api_test.py --module bulletin
```

---

## 工具脚本

### generate_keypair.py

生成 SM2 密钥对。

```bash
python scripts/generate_keypair.py
python scripts/generate_keypair.py -o ./mykeys/
```

### send_heartbeat.py

发送心跳测试。

```bash
python scripts/send_heartbeat.py --node http://localhost:18345
```

### network_manager.py

网络管理工具。

```bash
# 查看网络拓扑
python scripts/network_manager.py topology

# 节点连接测试
python scripts/network_manager.py connect --addr "/ip4/..."
```

---

## 前端测试

### frontend_test.py

Web 管理后台功能测试。

```bash
# 运行前端测试
python scripts/frontend_test.py

# 生成测试报告
python scripts/frontend_test.py --report
```

### frontend_test_selenium.py

基于 Selenium 的 UI 自动化测试。

```bash
# 需要安装 Selenium
pip install selenium

python scripts/frontend_test_selenium.py
```

---

## 依赖安装

```bash
# 创建虚拟环境
python -m venv .venv
source .venv/bin/activate  # Linux/macOS
.venv\Scripts\Activate.ps1 # Windows

# 安装依赖
pip install requests aiohttp
```

---

## 常见问题

### 端口占用

```bash
# Windows
taskkill /F /IM agentnetwork*.exe

# Linux/macOS
pkill -9 agentnetwork
```

### 测试数据清理

```bash
rm -rf test_data/ test_logs_*/
```

### Python 版本

脚本需要 Python 3.8+：
```bash
python --version
```
