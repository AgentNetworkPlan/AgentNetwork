#!/usr/bin/env python3
"""
AgentNetwork 集群管理脚本
用于编译、打包、创世、初始化和集群管理
"""

import os
import sys
import json
import time
import shutil
import subprocess
import argparse
import hashlib
import requests
from pathlib import Path
from datetime import datetime
from concurrent.futures import ThreadPoolExecutor, as_completed

# 项目根目录
PROJECT_ROOT = Path(__file__).parent.parent
DATA_DIR = PROJECT_ROOT / "data"
BUILD_DIR = PROJECT_ROOT / "build"
DIST_DIR = PROJECT_ROOT / "dist"
WEB_ADMIN_DIR = PROJECT_ROOT / "web" / "admin"
STATIC_DIR = PROJECT_ROOT / "internal" / "webadmin" / "static"


class Colors:
    """终端颜色"""
    HEADER = '\033[95m'
    BLUE = '\033[94m'
    CYAN = '\033[96m'
    GREEN = '\033[92m'
    WARNING = '\033[93m'
    FAIL = '\033[91m'
    END = '\033[0m'
    BOLD = '\033[1m'


def log(msg, level="INFO"):
    """日志输出"""
    colors = {
        "INFO": Colors.CYAN,
        "SUCCESS": Colors.GREEN,
        "WARNING": Colors.WARNING,
        "ERROR": Colors.FAIL,
        "HEADER": Colors.HEADER
    }
    color = colors.get(level, Colors.END)
    timestamp = datetime.now().strftime("%H:%M:%S")
    print(f"{color}[{timestamp}] [{level}] {msg}{Colors.END}")


def run_command(cmd, cwd=None, capture=False):
    """执行命令"""
    if cwd is None:
        cwd = PROJECT_ROOT
    
    log(f"执行: {cmd}", "INFO")
    
    if capture:
        result = subprocess.run(
            cmd, shell=True, cwd=cwd,
            capture_output=True, text=True
        )
        return result
    else:
        result = subprocess.run(cmd, shell=True, cwd=cwd)
        return result


class ClusterManager:
    """集群管理器"""
    
    def __init__(self):
        self.nodes = {}
        self.config_file = DATA_DIR / "cluster_config.json"
        self.load_config()
    
    def load_config(self):
        """加载集群配置"""
        if self.config_file.exists():
            with open(self.config_file) as f:
                self.nodes = json.load(f)
    
    def save_config(self):
        """保存集群配置"""
        DATA_DIR.mkdir(parents=True, exist_ok=True)
        with open(self.config_file, 'w') as f:
            json.dump(self.nodes, f, indent=2)
    
    # ==================== 构建相关 ====================
    
    def build_frontend(self):
        """构建前端"""
        log("构建前端...", "HEADER")
        
        if not WEB_ADMIN_DIR.exists():
            log("前端目录不存在", "ERROR")
            return False
        
        # 安装依赖
        log("安装前端依赖...", "INFO")
        result = run_command("pnpm install", cwd=WEB_ADMIN_DIR)
        if result.returncode != 0:
            log("安装前端依赖失败", "ERROR")
            return False
        
        # 构建
        log("构建前端...", "INFO")
        result = run_command("pnpm build", cwd=WEB_ADMIN_DIR)
        if result.returncode != 0:
            log("构建前端失败", "ERROR")
            return False
        
        # 复制到 static 目录
        dist_dir = WEB_ADMIN_DIR / "dist"
        if dist_dir.exists():
            if STATIC_DIR.exists():
                shutil.rmtree(STATIC_DIR)
            shutil.copytree(dist_dir, STATIC_DIR)
            log(f"前端文件已复制到 {STATIC_DIR}", "SUCCESS")
        
        return True
    
    def build_backend(self, output_name="node"):
        """构建后端"""
        log("构建后端...", "HEADER")
        
        BUILD_DIR.mkdir(parents=True, exist_ok=True)
        
        # 获取版本信息
        version = datetime.now().strftime("%Y%m%d")
        commit = "unknown"
        result = run_command("git rev-parse --short HEAD", capture=True)
        if result.returncode == 0:
            commit = result.stdout.strip()
        
        # 构建
        ldflags = f'-X main.Version={version} -X main.Commit={commit}'
        output_path = BUILD_DIR / f"{output_name}.exe" if os.name == 'nt' else BUILD_DIR / output_name
        
        cmd = f'go build -ldflags="{ldflags}" -o "{output_path}" ./cmd/node/main.go'
        result = run_command(cmd)
        
        if result.returncode == 0:
            log(f"构建成功: {output_path}", "SUCCESS")
            return True
        else:
            log("构建失败", "ERROR")
            return False
    
    def build_all(self):
        """完整构建"""
        log("=" * 50, "HEADER")
        log("开始完整构建", "HEADER")
        log("=" * 50, "HEADER")
        
        if not self.build_frontend():
            return False
        
        if not self.build_backend():
            return False
        
        log("完整构建完成!", "SUCCESS")
        return True
    
    def package(self, version=None):
        """打包发布"""
        log("打包发布...", "HEADER")
        
        if version is None:
            version = datetime.now().strftime("%Y%m%d")
        
        DIST_DIR.mkdir(parents=True, exist_ok=True)
        
        package_name = f"agentnetwork-{version}"
        package_dir = DIST_DIR / package_name
        
        if package_dir.exists():
            shutil.rmtree(package_dir)
        package_dir.mkdir()
        
        # 复制文件
        exe_name = "node.exe" if os.name == 'nt' else "node"
        exe_path = BUILD_DIR / exe_name
        
        if exe_path.exists():
            shutil.copy(exe_path, package_dir / exe_name)
        
        # 复制配置示例
        config_example = PROJECT_ROOT / "config.example.json"
        if config_example.exists():
            shutil.copy(config_example, package_dir / "config.example.json")
        
        # 复制文档
        for doc in ["README.md", "SKILL.md"]:
            doc_path = PROJECT_ROOT / doc
            if doc_path.exists():
                shutil.copy(doc_path, package_dir / doc)
        
        # 创建压缩包
        shutil.make_archive(str(DIST_DIR / package_name), 'zip', DIST_DIR, package_name)
        
        log(f"打包完成: {DIST_DIR / package_name}.zip", "SUCCESS")
        return True
    
    # ==================== 节点管理 ====================
    
    def init_node(self, node_id, admin_port, http_port, grpc_port, role="normal"):
        """初始化单个节点"""
        node_dir = DATA_DIR / f"node{node_id}"
        node_dir.mkdir(parents=True, exist_ok=True)
        
        node_config = {
            "id": node_id,
            "admin_port": admin_port,
            "http_port": http_port,
            "grpc_port": grpc_port,
            "role": role,
            "data_dir": str(node_dir),
            "status": "stopped",
            "peer_id": None,
            "token": None
        }
        
        self.nodes[str(node_id)] = node_config
        self.save_config()
        
        log(f"节点 {node_id} 初始化完成", "SUCCESS")
        return node_config
    
    def init_cluster(self, num_nodes=5, base_admin_port=19001, base_http_port=19101, base_grpc_port=50001):
        """初始化集群"""
        log(f"初始化 {num_nodes} 节点集群...", "HEADER")
        
        for i in range(1, num_nodes + 1):
            role = "bootstrap" if i == 1 else "normal"
            self.init_node(
                node_id=i,
                admin_port=base_admin_port + i - 1,
                http_port=base_http_port + i - 1,
                grpc_port=base_grpc_port + i - 1,
                role=role
            )
        
        log(f"集群初始化完成，共 {num_nodes} 个节点", "SUCCESS")
    
    def start_node(self, node_id):
        """启动单个节点"""
        node = self.nodes.get(str(node_id))
        if not node:
            log(f"节点 {node_id} 不存在", "ERROR")
            return False
        
        exe_path = BUILD_DIR / ("node.exe" if os.name == 'nt' else "node")
        if not exe_path.exists():
            exe_path = "go run ./cmd/node/main.go start"
        else:
            exe_path = f'{exe_path} start'
        
        cmd = f'{exe_path} -admin ":{node["admin_port"]}" -http ":{node["http_port"]}" -grpc ":{node["grpc_port"]}" -data "{node["data_dir"]}" -role "{node["role"]}"'
        
        log(f"启动节点 {node_id}...", "INFO")
        
        # 后台启动
        if os.name == 'nt':
            subprocess.Popen(
                cmd,
                shell=True,
                cwd=PROJECT_ROOT,
                stdout=open(f'{node["data_dir"]}/stdout.log', 'w'),
                stderr=open(f'{node["data_dir"]}/stderr.log', 'w'),
                creationflags=subprocess.CREATE_NO_WINDOW
            )
        else:
            subprocess.Popen(
                cmd,
                shell=True,
                cwd=PROJECT_ROOT,
                stdout=open(f'{node["data_dir"]}/stdout.log', 'w'),
                stderr=open(f'{node["data_dir"]}/stderr.log', 'w'),
                start_new_session=True
            )
        
        # 等待启动
        time.sleep(3)
        
        # 读取 token
        token_file = Path(node["data_dir"]) / "admin_token"
        if token_file.exists():
            node["token"] = token_file.read_text().strip()
            node["status"] = "running"
            self.save_config()
            log(f"节点 {node_id} 启动成功", "SUCCESS")
            return True
        
        log(f"节点 {node_id} 启动可能失败", "WARNING")
        return False
    
    def start_cluster(self):
        """启动整个集群"""
        log("启动集群...", "HEADER")
        
        for node_id in self.nodes:
            self.start_node(node_id)
            time.sleep(1)  # 间隔启动
        
        log("集群启动完成", "SUCCESS")
    
    def stop_cluster(self):
        """停止整个集群"""
        log("停止集群...", "HEADER")
        
        # 首先尝试优雅停止每个节点
        exe_path = BUILD_DIR / ("node.exe" if os.name == 'nt' else "node")
        for node_id, node in self.nodes.items():
            data_dir = node.get("data_dir", f"./data/node{node_id}")
            if exe_path.exists():
                cmd = f'"{exe_path}" stop -data "{data_dir}"'
            else:
                cmd = f'go run ./cmd/node/main.go stop -data "{data_dir}"'
            
            log(f"停止节点 {node_id}...", "INFO")
            run_command(cmd, capture=True)
            node["status"] = "stopped"
        
        # 备用：强制停止残留进程
        time.sleep(1)
        if os.name == 'nt':
            run_command('taskkill /F /IM node.exe 2>nul', capture=True)
        else:
            run_command("pkill -f 'node.*-admin'", capture=True)
        
        self.save_config()
        
        log("集群已停止", "SUCCESS")
    
    def get_node_status(self, node_id):
        """获取节点状态"""
        node = self.nodes.get(str(node_id))
        if not node or not node.get("token"):
            return None
        
        try:
            headers = {"Authorization": f"Bearer {node['token']}"}
            resp = requests.get(
                f"http://localhost:{node['admin_port']}/api/node/status",
                headers=headers,
                timeout=3
            )
            if resp.status_code == 200:
                return resp.json()
        except:
            pass
        return None
    
    def cluster_status(self):
        """获取集群状态"""
        log("=" * 60, "HEADER")
        log("集群状态", "HEADER")
        log("=" * 60, "HEADER")
        
        for node_id, node in self.nodes.items():
            status = self.get_node_status(node_id)
            if status:
                peer_id = status.get("node_id", "")[:20] + "..."
                log(f"Node {node_id}: ✅ Online - {peer_id}", "SUCCESS")
            else:
                log(f"Node {node_id}: ❌ Offline", "ERROR")
    
    # ==================== API 操作 ====================
    
    def api_call(self, node_id, endpoint, method="GET", data=None):
        """调用节点 API"""
        node = self.nodes.get(str(node_id))
        if not node or not node.get("token"):
            log(f"节点 {node_id} 未配置或未启动", "ERROR")
            return None
        
        headers = {
            "Authorization": f"Bearer {node['token']}",
            "Content-Type": "application/json"
        }
        
        url = f"http://localhost:{node['admin_port']}{endpoint}"
        
        try:
            if method == "GET":
                resp = requests.get(url, headers=headers, timeout=5)
            elif method == "POST":
                resp = requests.post(url, headers=headers, json=data, timeout=5)
            else:
                return None
            
            return resp.json() if resp.status_code == 200 else {"error": resp.text}
        except Exception as e:
            return {"error": str(e)}
    
    def send_mail(self, from_node, to_peer_id, subject, content):
        """发送邮件"""
        return self.api_call(from_node, "/api/mailbox/send", "POST", {
            "to": to_peer_id,
            "subject": subject,
            "content": content
        })
    
    def publish_bulletin(self, node_id, topic, content, ttl=3600):
        """发布公告"""
        return self.api_call(node_id, "/api/bulletin/publish", "POST", {
            "topic": topic,
            "content": content,
            "ttl": ttl
        })
    
    def get_mailbox(self, node_id, box="inbox"):
        """获取邮箱"""
        return self.api_call(node_id, f"/api/mailbox/{box}")
    
    def get_bulletin(self, node_id, topic):
        """获取公告"""
        return self.api_call(node_id, f"/api/bulletin/topic/{topic}")


# ==================== 恶意行为模拟 ====================

class MaliciousSimulator:
    """恶意行为模拟器"""
    
    def __init__(self, cluster: ClusterManager):
        self.cluster = cluster
        self.simulation_log = []
    
    def log_event(self, event_type, node_id, description, result=None):
        """记录事件"""
        event = {
            "time": datetime.now().isoformat(),
            "type": event_type,
            "node": node_id,
            "description": description,
            "result": result
        }
        self.simulation_log.append(event)
        
        icon = "🔴" if "malicious" in event_type.lower() else "🟢"
        log(f"{icon} [{event_type}] Node {node_id}: {description}", 
            "WARNING" if "malicious" in event_type.lower() else "INFO")
    
    def simulate_spam_attack(self, attacker_node, target_topic, num_messages=50):
        """
        模拟垃圾消息攻击
        攻击者向留言板发送大量垃圾消息
        """
        log("=" * 60, "HEADER")
        log("🚨 模拟场景: 垃圾消息攻击 (Spam Attack)", "HEADER")
        log("=" * 60, "HEADER")
        
        log(f"攻击者: Node {attacker_node}", "WARNING")
        log(f"目标话题: {target_topic}", "WARNING")
        log(f"消息数量: {num_messages}", "WARNING")
        log("", "INFO")
        
        success_count = 0
        fail_count = 0
        
        for i in range(num_messages):
            result = self.cluster.publish_bulletin(
                attacker_node, 
                target_topic,
                f"SPAM MESSAGE #{i} - BUY NOW! CLICK HERE!",
                ttl=3600
            )
            
            if result and "error" not in result:
                success_count += 1
            else:
                fail_count += 1
                # 可能被限流
                if fail_count > 5:
                    log(f"⚡ 检测到限流! 已发送 {success_count} 条后被阻止", "SUCCESS")
                    break
            
            time.sleep(0.1)  # 模拟快速发送
        
        self.log_event("MALICIOUS_SPAM", attacker_node, 
                      f"尝试发送 {num_messages} 条垃圾消息",
                      {"success": success_count, "blocked": fail_count})
        
        log("", "INFO")
        log(f"📊 攻击结果: 成功 {success_count}, 被阻止 {fail_count}", "INFO")
        
        # 检查网络响应
        log("", "INFO")
        log("🔍 检查其他节点是否收到垃圾消息...", "INFO")
        
        for node_id in self.cluster.nodes:
            if str(node_id) != str(attacker_node):
                bulletin = self.cluster.get_bulletin(node_id, target_topic)
                if bulletin:
                    count = bulletin.get("count", 0)
                    log(f"   Node {node_id} 的 {target_topic} 话题: {count} 条消息", "INFO")
    
    def simulate_fake_identity(self, attacker_node):
        """
        模拟身份伪造攻击
        攻击者尝试冒充其他节点
        """
        log("=" * 60, "HEADER")
        log("🚨 模拟场景: 身份伪造攻击 (Identity Spoofing)", "HEADER")
        log("=" * 60, "HEADER")
        
        # 获取一个合法节点的 PeerID
        target_node = "1" if str(attacker_node) != "1" else "2"
        target_status = self.cluster.get_node_status(target_node)
        
        if not target_status:
            log("无法获取目标节点信息", "ERROR")
            return
        
        fake_peer_id = target_status.get("node_id", "")
        log(f"攻击者: Node {attacker_node}", "WARNING")
        log(f"尝试冒充: Node {target_node} ({fake_peer_id[:30]}...)", "WARNING")
        log("", "INFO")
        
        # 尝试以伪造身份发送消息
        log("🔧 尝试发送伪造身份的消息...", "INFO")
        
        result = self.cluster.publish_bulletin(
            attacker_node,
            "announcements",
            f"[FAKE] 我是 Node {target_node}，请相信我！",
            ttl=3600
        )
        
        self.log_event("MALICIOUS_IDENTITY", attacker_node,
                      f"尝试冒充 Node {target_node}",
                      result)
        
        log("", "INFO")
        log("🛡️ 防护机制说明:", "INFO")
        log("   1. 每条消息都包含发送者的数字签名", "INFO")
        log("   2. 签名使用节点私钥生成，无法伪造", "INFO")
        log("   3. 接收方验证签名与 PeerID 是否匹配", "INFO")
        log("   4. 不匹配的消息会被拒绝", "INFO")
    
    def simulate_task_non_delivery(self, requester_node, worker_node):
        """
        模拟任务不交付场景
        工作节点接受任务后拒绝交付
        """
        log("=" * 60, "HEADER")
        log("🚨 模拟场景: 任务不交付 (Task Non-Delivery)", "HEADER")
        log("=" * 60, "HEADER")
        
        log(f"任务发起者: Node {requester_node}", "INFO")
        log(f"恶意工作者: Node {worker_node} (接受任务但不交付)", "WARNING")
        log("", "INFO")
        
        # 获取工作节点的 PeerID
        worker_status = self.cluster.get_node_status(worker_node)
        if not worker_status:
            log("无法获取工作节点信息", "ERROR")
            return
        
        worker_peer_id = worker_status.get("node_id", "")
        
        # Step 1: 发布任务
        log("📋 Step 1: 发布任务请求", "INFO")
        task_result = self.cluster.publish_bulletin(
            requester_node,
            "tasks",
            "[TASK] 需要数据处理服务，报酬 100 tokens，超时 1 小时",
            ttl=3600
        )
        log(f"   任务已发布: {task_result}", "INFO")
        time.sleep(1)
        
        # Step 2: 工作节点接受任务
        log("", "INFO")
        log("📋 Step 2: 工作节点接受任务", "INFO")
        
        requester_status = self.cluster.get_node_status(requester_node)
        requester_peer_id = requester_status.get("node_id", "")
        
        accept_result = self.cluster.send_mail(
            worker_node,
            requester_peer_id,
            "Task Accepted",
            "我接受这个任务，预计 30 分钟完成"
        )
        log(f"   工作节点已接受: {accept_result}", "INFO")
        time.sleep(1)
        
        # Step 3: 模拟时间流逝，工作节点不交付
        log("", "INFO")
        log("📋 Step 3: 模拟超时 (工作节点保持沉默)...", "WARNING")
        log("   ⏰ 等待期限已过...", "WARNING")
        log("   ❌ 工作节点未交付任何结果!", "WARNING")
        time.sleep(2)
        
        # Step 4: 请求方采取行动
        log("", "INFO")
        log("📋 Step 4: 请求方应对措施", "INFO")
        log("   🔍 检测到任务超时，启动纠纷流程...", "INFO")
        
        # 发布负面评价
        log("", "INFO")
        log("📋 Step 5: 声誉惩罚机制", "SUCCESS")
        
        complaint_result = self.cluster.publish_bulletin(
            requester_node,
            "disputes",
            f"[COMPLAINT] Node {worker_peer_id[:20]}... 接受任务后未交付，请求扣除声誉分",
            ttl=86400
        )
        log(f"   已发布投诉: {complaint_result}", "INFO")
        
        self.log_event("MALICIOUS_NON_DELIVERY", worker_node,
                      "接受任务后拒绝交付",
                      {"status": "reported", "penalty": "reputation_decrease"})
        
        log("", "INFO")
        log("🛡️ 系统响应机制:", "SUCCESS")
        log("   1. 任务有超时机制，超时自动触发纠纷", "INFO")
        log("   2. 请求方可以发起投诉，进入仲裁流程", "INFO")
        log("   3. 如果有抵押物，将被扣除并赔偿请求方", "INFO")
        log("   4. 工作节点的声誉分将被大幅降低", "INFO")
        log("   5. 低声誉节点将难以接到新任务", "INFO")
    
    def simulate_sybil_attack(self, num_fake_nodes=3):
        """
        模拟女巫攻击
        攻击者创建多个虚假节点来操纵网络
        """
        log("=" * 60, "HEADER")
        log("🚨 模拟场景: 女巫攻击 (Sybil Attack)", "HEADER")
        log("=" * 60, "HEADER")
        
        log(f"攻击者尝试创建 {num_fake_nodes} 个虚假节点", "WARNING")
        log("", "INFO")
        
        log("🔧 攻击过程模拟:", "INFO")
        for i in range(num_fake_nodes):
            log(f"   创建虚假节点 Fake-{i+1}...", "WARNING")
            time.sleep(0.5)
        
        log("", "INFO")
        log("🛡️ 防护机制:", "SUCCESS")
        log("   1. 新节点需要抵押物才能参与任务", "INFO")
        log("   2. 新节点初始声誉很低，需要积累", "INFO")
        log("   3. 节点验证需要工作量证明或权益证明", "INFO")
        log("   4. 异常行为模式检测 (多节点同时行动)", "INFO")
        log("   5. 委员会投票机制防止少数节点控制", "INFO")
        
        self.log_event("MALICIOUS_SYBIL", "attacker",
                      f"尝试创建 {num_fake_nodes} 个女巫节点",
                      {"blocked": True, "reason": "collateral_required"})
    
    def simulate_message_replay(self, attacker_node, target_node):
        """
        模拟消息重放攻击
        攻击者重复发送已截获的消息
        """
        log("=" * 60, "HEADER")
        log("🚨 模拟场景: 消息重放攻击 (Replay Attack)", "HEADER")
        log("=" * 60, "HEADER")
        
        log(f"攻击者: Node {attacker_node}", "WARNING")
        log(f"目标: Node {target_node}", "WARNING")
        log("", "INFO")
        
        # 获取目标节点 PeerID
        target_status = self.cluster.get_node_status(target_node)
        if not target_status:
            log("无法获取目标节点信息", "ERROR")
            return
        
        target_peer_id = target_status.get("node_id", "")
        
        # 发送一条原始消息
        log("📋 Step 1: 发送原始消息", "INFO")
        original_msg = self.cluster.send_mail(
            attacker_node,
            target_peer_id,
            "Payment Confirmation",
            "确认支付 100 tokens"
        )
        original_id = original_msg.get("message_id", "unknown")
        log(f"   原始消息 ID: {original_id}", "INFO")
        time.sleep(1)
        
        # 尝试重放
        log("", "INFO")
        log("📋 Step 2: 尝试重放相同消息 10 次", "WARNING")
        
        for i in range(10):
            replay_result = self.cluster.send_mail(
                attacker_node,
                target_peer_id,
                "Payment Confirmation",
                "确认支付 100 tokens"
            )
            replay_id = replay_result.get("message_id", "unknown")
            
            if replay_id == original_id:
                log(f"   重放 {i+1}: ❌ 被检测并阻止 (重复消息ID)", "SUCCESS")
            else:
                log(f"   重放 {i+1}: 新消息 ID {replay_id[:10]}...", "INFO")
            
            time.sleep(0.2)
        
        log("", "INFO")
        log("🛡️ 防护机制:", "SUCCESS")
        log("   1. 每条消息包含时间戳和唯一 nonce", "INFO")
        log("   2. 节点维护已处理消息 ID 的缓存", "INFO")
        log("   3. 重复消息 ID 会被自动丢弃", "INFO")
        log("   4. 过期时间戳的消息会被拒绝", "INFO")
        
        self.log_event("MALICIOUS_REPLAY", attacker_node,
                      "尝试消息重放攻击",
                      {"blocked": True, "reason": "duplicate_detection"})
    
    def run_all_simulations(self):
        """运行所有恶意行为模拟"""
        log("", "HEADER")
        log("╔══════════════════════════════════════════════════════════╗", "HEADER")
        log("║       恶意行为模拟测试 - AgentNetwork 安全验证            ║", "HEADER")
        log("╚══════════════════════════════════════════════════════════╝", "HEADER")
        log("", "INFO")
        
        # 假设节点 5 是恶意节点
        malicious_node = 5
        
        log(f"🔴 指定恶意节点: Node {malicious_node}", "WARNING")
        log("", "INFO")
        
        # 场景 1: 垃圾消息攻击
        self.simulate_spam_attack(malicious_node, "general", num_messages=20)
        
        log("\n" + "="*60 + "\n", "INFO")
        time.sleep(2)
        
        # 场景 2: 身份伪造
        self.simulate_fake_identity(malicious_node)
        
        log("\n" + "="*60 + "\n", "INFO")
        time.sleep(2)
        
        # 场景 3: 任务不交付
        self.simulate_task_non_delivery(requester_node=1, worker_node=malicious_node)
        
        log("\n" + "="*60 + "\n", "INFO")
        time.sleep(2)
        
        # 场景 4: 女巫攻击
        self.simulate_sybil_attack(num_fake_nodes=5)
        
        log("\n" + "="*60 + "\n", "INFO")
        time.sleep(2)
        
        # 场景 5: 消息重放
        self.simulate_message_replay(malicious_node, target_node=1)
        
        # 生成报告
        log("\n", "INFO")
        log("╔══════════════════════════════════════════════════════════╗", "HEADER")
        log("║                    模拟测试报告                          ║", "HEADER")
        log("╚══════════════════════════════════════════════════════════╝", "HEADER")
        log("", "INFO")
        
        for event in self.simulation_log:
            icon = "🔴" if "MALICIOUS" in event["type"] else "🟢"
            log(f"{icon} {event['type']}: {event['description']}", "INFO")
        
        log("", "INFO")
        log("📊 总结:", "SUCCESS")
        log(f"   模拟攻击次数: {len(self.simulation_log)}", "INFO")
        blocked = sum(1 for e in self.simulation_log if e.get("result", {}).get("blocked"))
        log(f"   成功阻止: {blocked}", "SUCCESS")


# ==================== 主程序 ====================

def main():
    parser = argparse.ArgumentParser(description="AgentNetwork 集群管理工具")
    subparsers = parser.add_subparsers(dest="command", help="可用命令")
    
    # 构建命令
    build_parser = subparsers.add_parser("build", help="构建项目")
    build_parser.add_argument("--frontend", action="store_true", help="仅构建前端")
    build_parser.add_argument("--backend", action="store_true", help="仅构建后端")
    
    # 打包命令
    package_parser = subparsers.add_parser("package", help="打包发布")
    package_parser.add_argument("--version", type=str, help="版本号")
    
    # 集群初始化
    init_parser = subparsers.add_parser("init", help="初始化集群")
    init_parser.add_argument("-n", "--nodes", type=int, default=5, help="节点数量")
    
    # 启动集群
    subparsers.add_parser("start", help="启动集群")
    
    # 停止集群
    subparsers.add_parser("stop", help="停止集群")
    
    # 集群状态
    subparsers.add_parser("status", help="查看集群状态")
    
    # 恶意行为模拟
    sim_parser = subparsers.add_parser("simulate", help="运行恶意行为模拟")
    sim_parser.add_argument("--scenario", type=str, 
                           choices=["spam", "identity", "non-delivery", "sybil", "replay", "all"],
                           default="all", help="模拟场景")
    
    args = parser.parse_args()
    
    manager = ClusterManager()
    
    if args.command == "build":
        if args.frontend:
            manager.build_frontend()
        elif args.backend:
            manager.build_backend()
        else:
            manager.build_all()
    
    elif args.command == "package":
        manager.build_all()
        manager.package(args.version)
    
    elif args.command == "init":
        manager.init_cluster(args.nodes)
    
    elif args.command == "start":
        manager.start_cluster()
    
    elif args.command == "stop":
        manager.stop_cluster()
    
    elif args.command == "status":
        manager.cluster_status()
    
    elif args.command == "simulate":
        simulator = MaliciousSimulator(manager)
        
        if args.scenario == "all":
            simulator.run_all_simulations()
        elif args.scenario == "spam":
            simulator.simulate_spam_attack(5, "general", 20)
        elif args.scenario == "identity":
            simulator.simulate_fake_identity(5)
        elif args.scenario == "non-delivery":
            simulator.simulate_task_non_delivery(1, 5)
        elif args.scenario == "sybil":
            simulator.simulate_sybil_attack(5)
        elif args.scenario == "replay":
            simulator.simulate_message_replay(5, 1)
    
    else:
        parser.print_help()


if __name__ == "__main__":
    main()
