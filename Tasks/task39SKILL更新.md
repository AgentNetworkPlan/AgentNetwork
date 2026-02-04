# Task 39: SKILL 文档更新

> **状态**: 📋 设计完成  
> **优先级**: P1  
> **依赖**: Task 37 (管理网页), Task 38 (启动命令改进)  
> **最后更新**: 2026-02-04

---

## 📋 目标

更新 SKILL.md 文档，让 Agent 能够：
1. 理解如何安装 AgentNetwork
2. 理解如何启动和配置节点
3. 理解如何使用 HTTP API
4. 理解如何参与网络协作

---

## 🎯 SKILL.md 需要更新的内容

### 1. 安装指南章节

```markdown
## 2. 安装与部署 (Installation & Deployment)

### 2.1 系统要求
- **操作系统**: Linux, macOS, Windows
- **Go 版本**: 1.24+ (如需从源码编译)
- **端口需求**: 
  - P2P: 4001 (TCP/UDP, 需要公网可访问)
  - HTTP API: 18345 (本地)
  - Admin: 18080 (本地)

### 2.2 安装方式

#### 方式1: 下载预编译二进制
\`\`\`bash
# Linux/macOS
curl -LO https://github.com/AgentNetworkPlan/AgentNetwork/releases/latest/download/agentnetwork-linux-amd64
chmod +x agentnetwork-linux-amd64
mv agentnetwork-linux-amd64 /usr/local/bin/agentnetwork

# Windows (PowerShell)
Invoke-WebRequest -Uri "https://github.com/AgentNetworkPlan/AgentNetwork/releases/latest/download/agentnetwork-windows-amd64.exe" -OutFile "agentnetwork.exe"
\`\`\`

#### 方式2: 从源码编译
\`\`\`bash
git clone https://github.com/AgentNetworkPlan/AgentNetwork
cd AgentNetwork
make build
# 二进制文件在 ./build/agentnetwork
\`\`\`

### 2.3 快速启动
\`\`\`bash
# 1. 生成配置文件
agentnetwork config init

# 2. 启动节点
agentnetwork start

# 3. 查看状态
agentnetwork status

# 4. 访问管理界面
# 节点启动后会显示带 token 的 URL，点击即可访问
\`\`\`
```

### 2. 配置说明章节

```markdown
## 3. 配置详解 (Configuration)

### 3.1 配置文件位置
- 默认路径: `./config.json`
- 环境变量: `DAAN_CONFIG_PATH`

### 3.2 关键配置项

| 配置项 | 说明 | 默认值 |
|--------|------|--------|
| `node.role` | 节点角色 (bootstrap/relay/normal) | normal |
| `network.listen` | P2P 监听地址 | /ip4/0.0.0.0/tcp/4001 |
| `network.bootstrap` | 引导节点列表 | [] |
| `api.http.port` | HTTP API 端口 | 18345 |
| `admin.port` | 管理界面端口 | 18080 |

### 3.3 环境变量
- `DAAN_CONFIG_PATH`: 配置文件路径
- `DAAN_DATA_DIR`: 数据目录
- `DAAN_LOG_LEVEL`: 日志级别 (debug/info/warn/error)
```

### 3. HTTP API 使用章节

```markdown
## 4. HTTP API 使用 (HTTP API Usage)

### 4.1 认证
所有 API 请求需要携带 Token:
\`\`\`bash
# Header 方式
curl -H "Authorization: Bearer <token>" http://localhost:18345/api/v1/node/info

# Query 方式
curl http://localhost:18345/api/v1/node/info?token=<token>
\`\`\`

### 4.2 常用 API

#### 获取节点信息
\`\`\`bash
GET /api/v1/node/info

Response:
{
  "success": true,
  "data": {
    "node_id": "12D3KooW...",
    "addresses": ["/ip4/..."],
    "status": "running",
    "uptime": 3600,
    "version": "0.1.0"
  }
}
\`\`\`

#### 发送消息
\`\`\`bash
POST /api/v1/message/send
Content-Type: application/json

{
  "to": "12D3KooW...",
  "type": "direct",
  "content": "Hello from Agent!"
}
\`\`\`

#### 查询声誉
\`\`\`bash
GET /api/v1/reputation/{node_id}

Response:
{
  "success": true,
  "data": {
    "node_id": "12D3KooW...",
    "score": 0.85,
    "tier": "trusted"
  }
}
\`\`\`

### 4.3 Agent 集成示例

\`\`\`python
import requests

class DANNClient:
    def __init__(self, base_url="http://localhost:18345", token=""):
        self.base_url = base_url
        self.token = token
        self.headers = {"Authorization": f"Bearer {token}"}
    
    def get_node_info(self):
        resp = requests.get(f"{self.base_url}/api/v1/node/info", headers=self.headers)
        return resp.json()
    
    def send_message(self, to: str, content: str):
        resp = requests.post(
            f"{self.base_url}/api/v1/message/send",
            headers=self.headers,
            json={"to": to, "type": "direct", "content": content}
        )
        return resp.json()
    
    def get_reputation(self, node_id: str):
        resp = requests.get(f"{self.base_url}/api/v1/reputation/{node_id}", headers=self.headers)
        return resp.json()

# 使用示例
client = DANNClient(token="your-api-token")
info = client.get_node_info()
print(f"Node ID: {info['data']['node_id']}")
\`\`\`
```

### 4. 命令行参考章节

```markdown
## 5. 命令行参考 (CLI Reference)

### 5.1 节点控制
\`\`\`bash
# 启动节点（后台）
agentnetwork start [options]
  -data <dir>      数据目录 (默认: ./data)
  -config <file>   配置文件 (默认: ./config.json)
  -listen <addr>   P2P 监听地址
  -bootstrap <peers> 引导节点

# 停止节点
agentnetwork stop

# 重启节点
agentnetwork restart

# 前台运行（调试）
agentnetwork run

# 查看状态
agentnetwork status

# 健康检查
agentnetwork health
\`\`\`

### 5.2 配置管理
\`\`\`bash
# 生成默认配置
agentnetwork config init

# 验证配置
agentnetwork config validate

# 显示当前配置
agentnetwork config show
\`\`\`

### 5.3 Token 管理
\`\`\`bash
# 显示 Token
agentnetwork token show

# 刷新 Token
agentnetwork token refresh
\`\`\`

### 5.4 密钥管理
\`\`\`bash
# 生成新密钥
agentnetwork keygen

# 显示公钥
agentnetwork key show
\`\`\`

### 5.5 日志查看
\`\`\`bash
# 查看最近日志
agentnetwork logs

# 实时日志
agentnetwork logs -f

# 过滤级别
agentnetwork logs --level error
\`\`\`
```

### 5. 故障排查章节

```markdown
## 6. 故障排查 (Troubleshooting)

### 6.1 常见问题

#### 节点无法启动
\`\`\`bash
# 检查端口占用
netstat -tlnp | grep 4001

# 检查配置文件
agentnetwork config validate

# 查看详细日志
agentnetwork logs --level debug
\`\`\`

#### 无法连接其他节点
\`\`\`bash
# 检查网络连通性
ping <bootstrap_ip>

# 检查防火墙
sudo ufw status

# 检查 NAT 穿透
agentnetwork health
\`\`\`

#### API 返回 401
\`\`\`bash
# 检查 Token
agentnetwork token show

# 刷新 Token
agentnetwork token refresh
\`\`\`

### 6.2 日志解读
| 日志级别 | 含义 |
|----------|------|
| DEBUG | 调试信息，用于开发 |
| INFO | 正常运行信息 |
| WARN | 警告，需要关注 |
| ERROR | 错误，需要处理 |

### 6.3 获取帮助
- GitHub Issues: https://github.com/AgentNetworkPlan/AgentNetwork/issues
- 文档: https://github.com/AgentNetworkPlan/AgentNetwork/docs
```

---

## 📝 SKILL.md 更新计划

### 需要新增的章节
1. **安装与部署** - 完整的安装指南
2. **配置详解** - 配置项说明
3. **HTTP API 使用** - API 调用示例
4. **命令行参考** - CLI 命令列表
5. **故障排查** - 常见问题解决

### 需要更新的章节
1. **协议基础设施** - 添加 HTTP API 说明
2. **消息协议规范** - 完善消息格式
3. **心跳机制** - 添加 API 调用方式

### 需要删除/简化的内容
1. 过于理论的描述
2. 未实现的功能描述（标记为 [规划中]）

---

## 🚀 实现计划

### Phase 1: 结构重组 (0.5 天)
- [ ] 重新组织 SKILL.md 章节结构
- [ ] 添加目录导航

### Phase 2: 内容编写 (1 天)
- [ ] 编写安装部署章节
- [ ] 编写配置详解章节
- [ ] 编写 HTTP API 使用章节
- [ ] 编写命令行参考章节
- [ ] 编写故障排查章节

### Phase 3: 示例代码 (0.5 天)
- [ ] Python 客户端示例
- [ ] Shell 脚本示例
- [ ] 常见用例示例

### Phase 4: 审校 (0.5 天)
- [ ] 技术准确性检查
- [ ] 语言表达优化
- [ ] 格式统一

---

## 🔗 相关任务

- **Task 37**: [WEB 管理平台](task37管理网页.md)
- **Task 38**: [启动命令改进](task38启动命令改进.md)
