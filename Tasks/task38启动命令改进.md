# Task 38: 启动命令与脚本改进

> **状态**: 📋 设计完成  
> **优先级**: P1  
> **依赖**: Task 37 (管理网页)  
> **最后更新**: 2026-02-04

---

## 📋 目标

改进节点启动命令和管理脚本，提供更好的用户体验和运维能力。

---

## 🎯 改进内容

### 1. 命令行参数重构

#### 当前参数
```
-data       数据目录
-key        密钥文件路径
-listen     P2P监听地址
-bootstrap  引导节点地址
-role       节点角色
-grpc       gRPC服务地址
-http       HTTP服务地址
```

#### 新增参数
```
-admin      管理界面地址 (默认: 127.0.0.1:18080)
-admin-bind 管理界面绑定地址 (默认: 127.0.0.1，可设为 0.0.0.0)
-http-bind  HTTP API绑定地址 (默认: 127.0.0.1)
-log-level  日志级别 (debug/info/warn/error)
-log-file   日志文件路径
-config     配置文件路径
```

### 2. 新增命令

#### Token 管理命令
```bash
# 显示当前 Token
agentnetwork token show
agentnetwork token show --type admin   # 显示管理 Token
agentnetwork token show --type api     # 显示 API Token

# 刷新 Token
agentnetwork token refresh             # 刷新所有 Token
agentnetwork token refresh --type admin

# 输出示例
$ agentnetwork token show
━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━
  Admin Token: a1b2c3d4-e5f6-7890-abcd-ef1234567890
  API Token:   x9y8z7w6-v5u4-3210-fedc-ba0987654321
━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━
```

#### 配置管理命令
```bash
# 生成默认配置文件
agentnetwork config init
agentnetwork config init --output config.json

# 验证配置文件
agentnetwork config validate
agentnetwork config validate --config ./config.json

# 显示当前配置
agentnetwork config show

# 输出示例
$ agentnetwork config init
✓ 配置文件已生成: ./config.json
  请编辑配置文件后启动节点
```

#### 密钥管理命令
```bash
# 生成新密钥对
agentnetwork keygen
agentnetwork keygen --output ./keys/node.key
agentnetwork keygen --algorithm sm2  # 或 ed25519

# 显示公钥
agentnetwork key show
agentnetwork key show --format hex   # 十六进制格式
agentnetwork key show --format pem   # PEM 格式

# 输出示例
$ agentnetwork keygen
✓ 密钥对已生成
  私钥: ./data/keys/node.key
  公钥: ./data/keys/node.pub
  Node ID: 12D3KooWAbCdEfGhIjKlMnOpQrStUvWxYz...
```

#### 健康检查命令
```bash
# 检查节点健康状态
agentnetwork health
agentnetwork health --json  # JSON 格式输出

# 输出示例
$ agentnetwork health
━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━
  Status:     ✓ Running
  Node ID:    12D3KooWAbC...
  Uptime:     2h 35m 12s
  Peers:      8 connected
  Messages:   1,234 sent / 5,678 received
  Reputation: 0.85
━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━
```

### 3. 启动输出优化

```
$ agentnetwork start

  ██████╗  █████╗  █████╗ ███╗   ██╗
  ██╔══██╗██╔══██╗██╔══██╗████╗  ██║
  ██║  ██║███████║███████║██╔██╗ ██║
  ██║  ██║██╔══██║██╔══██║██║╚██╗██║
  ██████╔╝██║  ██║██║  ██║██║ ╚████║
  ╚═════╝ ╚═╝  ╚═╝╚═╝  ╚═╝╚═╝  ╚═══╝
  Decentralized Autonomous Agent Network

━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━
  Version:    v0.1.0 (build: 2026-02-04)
  Node ID:    12D3KooWAbCdEfGhIjKlMnOpQrStUvWxYz...
  Role:       normal
  Data Dir:   ./data
━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━
  Endpoints:
    P2P:      /ip4/0.0.0.0/tcp/4001
    HTTP API: http://127.0.0.1:18345
    gRPC:     127.0.0.1:50051
    Admin:    http://127.0.0.1:18080
━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━
  🔗 Quick Access (Ctrl+Click to open):
     http://127.0.0.1:18080/admin?token=a1b2c3d4-e5f6-...
━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━
  Press Ctrl+C to stop
━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━
```

### 4. 完整命令列表

```
agentnetwork <command> [options]

Commands:
  节点控制:
    start         启动节点（后台运行）
    stop          停止节点
    restart       重启节点
    run           前台运行节点（调试用）
    status        查看节点状态
    health        健康检查

  配置管理:
    config init       生成默认配置文件
    config validate   验证配置文件
    config show       显示当前配置

  密钥管理:
    keygen        生成新密钥对
    key show      显示公钥信息

  Token 管理:
    token show    显示 Token
    token refresh 刷新 Token

  日志:
    logs          查看节点日志
    logs -f       实时查看日志（类似 tail -f）

  其他:
    version       显示版本信息
    help          显示帮助信息
```

---

## 📁 配置文件格式

### config.json 完整示例

```json
{
  "version": "1.0",
  "node": {
    "id": "",
    "role": "normal",
    "data_dir": "./data"
  },
  "network": {
    "listen": [
      "/ip4/0.0.0.0/tcp/4001",
      "/ip4/0.0.0.0/udp/4001/quic-v1"
    ],
    "bootstrap": [
      "/ip4/x.x.x.x/tcp/4001/p2p/12D3KooW..."
    ],
    "enable_dht": true,
    "enable_relay": true
  },
  "api": {
    "http": {
      "enabled": true,
      "bind": "127.0.0.1",
      "port": 18345,
      "token": ""
    },
    "grpc": {
      "enabled": true,
      "bind": "127.0.0.1",
      "port": 50051
    }
  },
  "admin": {
    "enabled": true,
    "bind": "127.0.0.1",
    "port": 18080,
    "token": ""
  },
  "logging": {
    "level": "info",
    "file": "./logs/node.log",
    "max_size_mb": 100,
    "max_backups": 5,
    "max_age_days": 30
  },
  "security": {
    "key_algorithm": "sm2",
    "private_key_path": "./data/keys/node.key"
  }
}
```

---

## 🛠️ 脚本改进

### Makefile 新增目标

```makefile
# 构建
build:
	go build -o build/agentnetwork ./cmd/node

# 构建（包含版本信息）
build-release:
	go build -ldflags "-X main.version=$(VERSION) -X main.buildTime=$(BUILD_TIME)" \
		-o build/agentnetwork ./cmd/node

# 构建前端
build-web:
	cd web/admin && npm install && npm run build

# 完整构建（后端 + 前端）
build-all: build-web build-release

# 开发模式（前端热重载）
dev-web:
	cd web/admin && npm run dev

# 运行测试
test:
	go test ./... -v

# 生成配置
init-config:
	./build/agentnetwork config init

# 清理
clean:
	rm -rf build/
	rm -rf web/admin/dist/
```

### 快速启动脚本

#### scripts/quick-start.sh (Linux/macOS)
```bash
#!/bin/bash
set -e

# 颜色
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
NC='\033[0m'

echo -e "${GREEN}🚀 DAAN Quick Start${NC}"
echo "━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━"

# 检查是否已编译
if [ ! -f "./build/agentnetwork" ]; then
    echo -e "${YELLOW}Building...${NC}"
    make build
fi

# 生成配置（如果不存在）
if [ ! -f "./config.json" ]; then
    echo -e "${YELLOW}Generating config...${NC}"
    ./build/agentnetwork config init
fi

# 启动节点
echo -e "${GREEN}Starting node...${NC}"
./build/agentnetwork run
```

#### scripts/quick-start.ps1 (Windows)
```powershell
# DAAN Quick Start Script
Write-Host "🚀 DAAN Quick Start" -ForegroundColor Green
Write-Host "━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━" -ForegroundColor Gray

# 检查是否已编译
if (-not (Test-Path ".\build\agentnetwork.exe")) {
    Write-Host "Building..." -ForegroundColor Yellow
    go build -o build/agentnetwork.exe ./cmd/node
}

# 生成配置（如果不存在）
if (-not (Test-Path ".\config.json")) {
    Write-Host "Generating config..." -ForegroundColor Yellow
    .\build\agentnetwork.exe config init
}

# 启动节点
Write-Host "Starting node..." -ForegroundColor Green
.\build\agentnetwork.exe run
```

---

## 📝 实现计划

### Phase 1: 命令行重构 (1 天)
- [ ] 重构 `cmd/node/main.go`
- [ ] 实现 token 命令
- [ ] 实现 config 命令
- [ ] 实现 keygen 命令
- [ ] 实现 health 命令

### Phase 2: 配置系统 (0.5 天)
- [ ] 设计新的配置结构
- [ ] 实现配置加载/保存
- [ ] 配置验证

### Phase 3: 输出优化 (0.5 天)
- [ ] 启动输出美化
- [ ] 添加颜色支持
- [ ] 添加 ASCII Logo

### Phase 4: 文档更新 (0.5 天)
- [ ] 更新 README.md
- [ ] 更新 TESTING.md
- [ ] 添加使用示例

---

## 🔗 相关任务

- **Task 37**: [WEB 管理平台](task37管理网页.md) - 新增 admin 端口
- **Task 39**: [SKILL 更新](task39SKILL更新.md) - Agent 使用文档
