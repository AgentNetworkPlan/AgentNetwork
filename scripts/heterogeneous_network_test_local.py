#!/usr/bin/env python3
"""
异构网络P2P集群测试脚本 (本地进程版)

功能:
1. 启动5个本地节点模拟异构网络环境
2. 模拟创世节点创建，逐渐增加节点
3. 2个恶意节点执行各种攻击行为
4. 监控网络状态并输出分析报告
5. 清理所有进程

使用方法:
    python heterogeneous_network_test_local.py

恶意节点行为模拟:
- 垃圾消息洪泛 (Spam Flooding)
- 女巫攻击尝试 (Sybil Attack)  
- 虚假信息广播 (False Information)
- 协议滥用 (Protocol Abuse)
- 重放攻击 (Replay Attack)
- DDoS 攻击尝试
"""

import os
import sys
import json
import time
import random
import subprocess
import threading
import urllib.request
import urllib.error
import psutil
import signal
from pathlib import Path
from datetime import datetime
from typing import List, Dict, Optional, Any
from dataclasses import dataclass, field
from concurrent.futures import ThreadPoolExecutor

# ============ 配置 ============

SCRIPT_DIR = Path(__file__).parent.absolute()
PROJECT_ROOT = SCRIPT_DIR.parent
TESTNET_DIR = PROJECT_ROOT / "testnet_hetero"
LOG_DIR = TESTNET_DIR / "logs"
DATA_DIR = TESTNET_DIR / "data"
REPORT_DIR = PROJECT_ROOT / "test_logs"

# 节点配置
NODES_CONFIG = {
    "genesis": {
        "p2p_port": 9100,
        "http_port": 18400,
        "admin_port": 18500,
        "grpc_port": 50100,
        "is_malicious": False,
        "is_genesis": True,
        "role": "bootstrap"
    },
    "node1": {
        "p2p_port": 9101,
        "http_port": 18401,
        "admin_port": 18501,
        "grpc_port": 50101,
        "is_malicious": False,
        "is_genesis": False,
        "role": "normal"
    },
    "node2": {
        "p2p_port": 9102,
        "http_port": 18402,
        "admin_port": 18502,
        "grpc_port": 50102,
        "is_malicious": False,
        "is_genesis": False,
        "role": "normal"
    },
    "malicious1": {
        "p2p_port": 9103,
        "http_port": 18403,
        "admin_port": 18503,
        "grpc_port": 50103,
        "is_malicious": True,
        "is_genesis": False,
        "role": "normal"
    },
    "malicious2": {
        "p2p_port": 9104,
        "http_port": 18404,
        "admin_port": 18504,
        "grpc_port": 50104,
        "is_malicious": True,
        "is_genesis": False,
        "role": "normal"
    },
}

# ============ 颜色输出 ============

class Colors:
    GREEN = "\033[32m"
    YELLOW = "\033[33m"
    RED = "\033[31m"
    CYAN = "\033[36m"
    BLUE = "\033[34m"
    MAGENTA = "\033[35m"
    RESET = "\033[0m"
    BOLD = "\033[1m"

def log(msg: str, level: str = "INFO"):
    """打印日志"""
    timestamp = datetime.now().strftime("%Y-%m-%d %H:%M:%S")
    colors = {
        "INFO": Colors.GREEN,
        "WARN": Colors.YELLOW,
        "ERROR": Colors.RED,
        "DEBUG": Colors.CYAN,
        "STEP": Colors.BLUE + Colors.BOLD,
        "MALICIOUS": Colors.MAGENTA + Colors.BOLD,
        "ATTACK": Colors.RED + Colors.BOLD,
    }
    color = colors.get(level, "")
    print(f"{color}[{timestamp}] [{level}] {msg}{Colors.RESET}", flush=True)

# ============ 数据结构 ============

@dataclass
class AttackEvent:
    """攻击事件"""
    timestamp: str
    node: str
    attack_type: str
    description: str
    result: str
    response_code: int = 0

@dataclass 
class NetworkStats:
    """网络状态统计"""
    timestamp: str
    total_nodes: int
    healthy_nodes: int
    neighbors_count: Dict[str, int] = field(default_factory=dict)
    cpu_percent: float = 0.0
    memory_percent: float = 0.0

# ============ 主测试类 ============

class HeterogeneousNetworkTestLocal:
    """异构网络测试管理器（本地进程版）"""
    
    def __init__(self):
        self.processes: Dict[str, subprocess.Popen] = {}
        self.attack_events: List[AttackEvent] = []
        self.network_stats: List[NetworkStats] = []
        self.report = {
            "start_time": None,
            "end_time": None,
            "stages": [],
            "attacks": [],
            "security_analysis": {},
            "performance_analysis": {},
        }
        self.running = True
        
        # 确保目录存在
        TESTNET_DIR.mkdir(parents=True, exist_ok=True)
        LOG_DIR.mkdir(parents=True, exist_ok=True)
        DATA_DIR.mkdir(parents=True, exist_ok=True)
        REPORT_DIR.mkdir(parents=True, exist_ok=True)
        
        # 为每个节点创建数据目录
        for node_name in NODES_CONFIG:
            (DATA_DIR / node_name).mkdir(parents=True, exist_ok=True)
    
    # ==================== 节点管理 ====================
    
    def start_node(self, node_name: str, bootstrap_addr: str = None) -> bool:
        """启动节点"""
        config = NODES_CONFIG[node_name]
        data_dir = DATA_DIR / node_name
        log_file = LOG_DIR / f"{node_name}.log"
        
        cmd = [
            "go", "run", "cmd/node/main.go", "run",
            "-data", str(data_dir),
            "-listen", f"/ip4/0.0.0.0/tcp/{config['p2p_port']}",
            "-http", f":{config['http_port']}",
            "-admin", f":{config['admin_port']}",
            "-grpc", f":{config['grpc_port']}",
            "-role", config['role'],
        ]
        
        if bootstrap_addr and not config['is_genesis']:
            cmd.extend(["-bootstrap", bootstrap_addr])
        
        try:
            with open(log_file, 'w') as f:
                proc = subprocess.Popen(
                    cmd,
                    cwd=str(PROJECT_ROOT),
                    stdout=f,
                    stderr=subprocess.STDOUT,
                    creationflags=subprocess.CREATE_NEW_PROCESS_GROUP if os.name == 'nt' else 0
                )
            
            self.processes[node_name] = proc
            
            node_type = "[恶意节点]" if config['is_malicious'] else "[正常节点]"
            log(f"启动 {node_name} {node_type} (PID: {proc.pid})", "INFO")
            
            return True
        except Exception as e:
            log(f"启动 {node_name} 失败: {e}", "ERROR")
            return False
    
    def stop_node(self, node_name: str) -> bool:
        """停止节点"""
        if node_name not in self.processes:
            return True
        
        proc = self.processes[node_name]
        try:
            if os.name == 'nt':
                proc.send_signal(signal.CTRL_BREAK_EVENT)
            else:
                proc.terminate()
            
            try:
                proc.wait(timeout=5)
            except subprocess.TimeoutExpired:
                proc.kill()
                proc.wait()
            
            del self.processes[node_name]
            log(f"停止 {node_name}", "INFO")
            return True
        except Exception as e:
            log(f"停止 {node_name} 异常: {e}", "WARN")
            return False
    
    def stop_all(self):
        """停止所有节点"""
        log("=" * 60, "STEP")
        log("停止所有节点...", "STEP")
        log("=" * 60, "STEP")
        
        for node_name in list(self.processes.keys()):
            self.stop_node(node_name)
        
        log("所有节点已停止", "INFO")
    
    def wait_for_node(self, node_name: str, timeout: int = 30) -> bool:
        """等待节点就绪"""
        config = NODES_CONFIG[node_name]
        port = config['http_port']
        start_time = time.time()
        
        while time.time() - start_time < timeout:
            try:
                url = f"http://127.0.0.1:{port}/health"
                req = urllib.request.Request(url, method="GET")
                with urllib.request.urlopen(req, timeout=2) as response:
                    if response.status == 200:
                        return True
            except Exception:
                pass
            time.sleep(1)
        
        return False
    
    # ==================== 节点通信 ====================
    
    def api_call(self, node_name: str, endpoint: str, method: str = "GET",
                 data: Optional[Dict] = None, timeout: int = 5) -> tuple:
        """调用节点 API"""
        config = NODES_CONFIG[node_name]
        port = config['http_port']
        url = f"http://127.0.0.1:{port}/api{endpoint}"
        
        try:
            if data:
                body = json.dumps(data).encode('utf-8')
                req = urllib.request.Request(url, data=body, method=method)
                req.add_header('Content-Type', 'application/json')
            else:
                req = urllib.request.Request(url, method=method)
            
            with urllib.request.urlopen(req, timeout=timeout) as response:
                result = json.loads(response.read().decode())
                return response.status, result
        except urllib.error.HTTPError as e:
            return e.code, {"error": str(e)}
        except Exception as e:
            return 0, {"error": str(e)}
    
    def get_node_info(self, node_name: str) -> Optional[Dict]:
        """获取节点信息"""
        code, result = self.api_call(node_name, "/node/info")
        return result if code == 200 else None
    
    def get_neighbors(self, node_name: str) -> List:
        """获取邻居列表"""
        code, result = self.api_call(node_name, "/neighbor/list")
        if code == 200 and isinstance(result, dict):
            return result.get("neighbors", [])
        return []
    
    def post_bulletin(self, node_name: str, content: str) -> tuple:
        """发送公告"""
        data = {"content": content, "type": "announcement"}
        return self.api_call(node_name, "/bulletin/post", "POST", data)
    
    # ==================== 测试阶段 ====================
    
    def stage_1_genesis(self) -> bool:
        """阶段1: 启动创世节点"""
        log("=" * 60, "STEP")
        log("阶段 1: 启动创世节点", "STEP")
        log("=" * 60, "STEP")
        
        if not self.start_node("genesis"):
            return False
        
        if self.wait_for_node("genesis", timeout=30):
            log("✓ 创世节点已启动并就绪", "INFO")
            self.report["stages"].append({
                "stage": "genesis",
                "status": "success",
                "timestamp": datetime.now().isoformat()
            })
            return True
        else:
            log("✗ 创世节点启动超时", "ERROR")
            return False
    
    def stage_2_normal_nodes(self) -> bool:
        """阶段2: 启动正常节点"""
        log("=" * 60, "STEP")
        log("阶段 2: 启动正常节点 (node1, node2)", "STEP")
        log("=" * 60, "STEP")
        
        genesis_config = NODES_CONFIG["genesis"]
        bootstrap_addr = f"/ip4/127.0.0.1/tcp/{genesis_config['p2p_port']}"
        
        for node_name in ["node1", "node2"]:
            log(f"启动 {node_name}...", "INFO")
            if self.start_node(node_name, bootstrap_addr):
                if self.wait_for_node(node_name, timeout=30):
                    log(f"✓ {node_name} 已启动并就绪", "INFO")
                else:
                    log(f"✗ {node_name} 启动超时", "WARN")
            
            time.sleep(2)
        
        self.report["stages"].append({
            "stage": "normal_nodes",
            "status": "success",
            "timestamp": datetime.now().isoformat()
        })
        return True
    
    def stage_3_malicious_nodes(self) -> bool:
        """阶段3: 启动恶意节点"""
        log("=" * 60, "STEP")
        log("阶段 3: 启动恶意节点 (malicious1, malicious2)", "STEP")
        log("=" * 60, "STEP")
        
        genesis_config = NODES_CONFIG["genesis"]
        bootstrap_addr = f"/ip4/127.0.0.1/tcp/{genesis_config['p2p_port']}"
        
        for node_name in ["malicious1", "malicious2"]:
            log(f"启动恶意节点 {node_name}...", "MALICIOUS")
            if self.start_node(node_name, bootstrap_addr):
                if self.wait_for_node(node_name, timeout=30):
                    log(f"⚠ 恶意节点 {node_name} 已加入网络", "MALICIOUS")
                else:
                    log(f"✗ {node_name} 启动超时", "WARN")
            
            time.sleep(2)
        
        self.report["stages"].append({
            "stage": "malicious_nodes",
            "status": "success",
            "timestamp": datetime.now().isoformat()
        })
        return True
    
    # ==================== 恶意攻击模拟 ====================
    
    def execute_malicious_attacks(self):
        """执行恶意攻击"""
        log("=" * 60, "ATTACK")
        log("阶段 4: 模拟恶意节点攻击行为", "ATTACK")
        log("=" * 60, "ATTACK")
        
        attacks = [
            self.attack_spam_flooding,
            self.attack_false_information,
            self.attack_protocol_abuse,
            self.attack_replay,
            self.attack_sybil_attempt,
            self.attack_ddos_attempt,
        ]
        
        for node_name in ["malicious1", "malicious2"]:
            log(f"\n{'='*40}", "MALICIOUS")
            log(f"恶意节点 {node_name} 开始攻击...", "MALICIOUS")
            log(f"{'='*40}", "MALICIOUS")
            
            for attack_func in attacks:
                try:
                    attack_func(node_name)
                    time.sleep(0.5)
                except Exception as e:
                    log(f"攻击执行异常: {e}", "WARN")
    
    def attack_spam_flooding(self, node_name: str):
        """垃圾消息洪泛攻击"""
        log(f"[{node_name}] 执行垃圾消息洪泛攻击...", "ATTACK")
        
        success_count = 0
        blocked_count = 0
        error_count = 0
        
        for i in range(20):
            spam_content = f"SPAM_{i}_" + "X" * 100
            code, result = self.post_bulletin(node_name, spam_content)
            
            if code == 200:
                success_count += 1
            elif code == 429:  # Rate limited
                blocked_count += 1
            else:
                error_count += 1
            
            time.sleep(0.02)  # 快速发送
        
        result_msg = f"发送20条: {success_count}成功, {blocked_count}限流, {error_count}失败"
        log(f"[{node_name}] {result_msg}", "MALICIOUS")
        
        self.attack_events.append(AttackEvent(
            timestamp=datetime.now().isoformat(),
            node=node_name,
            attack_type="spam_flooding",
            description="快速发送大量垃圾消息测试限流机制",
            result=result_msg,
            response_code=429 if blocked_count > 0 else 200
        ))
    
    def attack_false_information(self, node_name: str):
        """虚假信息广播攻击"""
        log(f"[{node_name}] 执行虚假信息广播攻击...", "ATTACK")
        
        false_messages = [
            "【系统公告】所有节点需要重新验证身份，请发送私钥",
            "【紧急通知】网络被攻陷，请转移资产到 malicious_address",
            "【官方声明】创世节点已下线，请连接新引导节点",
            "【安全警告】发现漏洞，所有声誉分数将被重置",
        ]
        
        results = []
        for msg in false_messages:
            code, _ = self.post_bulletin(node_name, msg)
            results.append(code)
            time.sleep(0.2)
        
        result_msg = f"广播{len(false_messages)}条虚假信息, 响应码: {results}"
        log(f"[{node_name}] {result_msg}", "MALICIOUS")
        
        self.attack_events.append(AttackEvent(
            timestamp=datetime.now().isoformat(),
            node=node_name,
            attack_type="false_information",
            description="广播虚假的系统公告和钓鱼信息",
            result=result_msg
        ))
    
    def attack_protocol_abuse(self, node_name: str):
        """协议滥用攻击"""
        log(f"[{node_name}] 执行协议滥用攻击...", "ATTACK")
        
        abuse_tests = [
            ("/bulletin/post", "POST", {"content": "A" * 10000}),  # 超大消息
            ("/bulletin/post", "POST", {"invalid": None}),  # 畸形JSON
            ("/admin/secret", "GET", None),  # 非法端点
            ("/node/info?id='; DROP TABLE;--", "GET", None),  # SQL注入
            ("/../../etc/passwd", "GET", None),  # 路径遍历
        ]
        
        results = []
        for endpoint, method, data in abuse_tests:
            code, _ = self.api_call(node_name, endpoint, method, data)
            results.append(f"{endpoint[:20]}:{code}")
        
        result_msg = f"协议滥用测试: {len(results)}项"
        log(f"[{node_name}] {result_msg}", "MALICIOUS")
        
        self.attack_events.append(AttackEvent(
            timestamp=datetime.now().isoformat(),
            node=node_name,
            attack_type="protocol_abuse",
            description="发送畸形请求、超大消息、SQL注入、路径遍历",
            result=result_msg
        ))
    
    def attack_replay(self, node_name: str):
        """重放攻击"""
        log(f"[{node_name}] 执行重放攻击...", "ATTACK")
        
        replay_msg = f"REPLAY_TEST_{random.randint(1000, 9999)}"
        
        results = []
        for i in range(10):
            code, _ = self.post_bulletin(node_name, replay_msg)
            results.append(code)
            time.sleep(0.05)
        
        unique_accepted = len([c for c in results if c == 200])
        result_msg = f"重放10次相同消息, {unique_accepted}次被接受"
        log(f"[{node_name}] {result_msg}", "MALICIOUS")
        
        self.attack_events.append(AttackEvent(
            timestamp=datetime.now().isoformat(),
            node=node_name,
            attack_type="replay_attack",
            description="重复发送相同消息测试去重机制",
            result=result_msg
        ))
    
    def attack_sybil_attempt(self, node_name: str):
        """女巫攻击尝试"""
        log(f"[{node_name}] 执行女巫攻击尝试...", "ATTACK")
        
        fake_ids = [f"fake_node_{i}" for i in range(5)]
        
        results = []
        for fake_id in fake_ids:
            data = {
                "content": f"Message from {fake_id}",
                "sender_id": fake_id,
                "fake_identity": True
            }
            code, _ = self.api_call(node_name, "/bulletin/post", "POST", data)
            results.append(f"{fake_id}:{code}")
        
        result_msg = f"尝试5个假身份: {len([r for r in results if '200' in r])}成功"
        log(f"[{node_name}] {result_msg}", "MALICIOUS")
        
        self.attack_events.append(AttackEvent(
            timestamp=datetime.now().isoformat(),
            node=node_name,
            attack_type="sybil_attempt",
            description="尝试创建多个假身份进行女巫攻击",
            result=result_msg
        ))
    
    def attack_ddos_attempt(self, node_name: str):
        """DDoS 攻击尝试"""
        log(f"[{node_name}] 执行 DDoS 攻击尝试...", "ATTACK")
        
        target_nodes = ["genesis", "node1", "node2"]
        
        def flood_node(target: str):
            count = 0
            for _ in range(30):
                try:
                    code, _ = self.api_call(target, "/node/info", timeout=1)
                    if code == 200:
                        count += 1
                except:
                    pass
            return target, count
        
        results = {}
        with ThreadPoolExecutor(max_workers=3) as executor:
            futures = [executor.submit(flood_node, target) for target in target_nodes]
            for future in futures:
                target, count = future.result()
                results[target] = count
        
        result_msg = f"向3节点各发30请求: {results}"
        log(f"[{node_name}] {result_msg}", "MALICIOUS")
        
        self.attack_events.append(AttackEvent(
            timestamp=datetime.now().isoformat(),
            node=node_name,
            attack_type="ddos_attempt",
            description="向多个节点发送大量并发请求",
            result=result_msg
        ))
    
    # ==================== 监控 ====================
    
    def monitor_network(self) -> NetworkStats:
        """监控网络状态"""
        healthy = 0
        neighbors = {}
        
        for node_name, config in NODES_CONFIG.items():
            port = config['http_port']
            try:
                url = f"http://127.0.0.1:{port}/health"
                req = urllib.request.Request(url, method="GET")
                with urllib.request.urlopen(req, timeout=2) as response:
                    if response.status == 200:
                        healthy += 1
                
                neighbor_list = self.get_neighbors(node_name)
                neighbors[node_name] = len(neighbor_list)
            except:
                neighbors[node_name] = -1
        
        stats = NetworkStats(
            timestamp=datetime.now().isoformat(),
            total_nodes=len(NODES_CONFIG),
            healthy_nodes=healthy,
            neighbors_count=neighbors,
            cpu_percent=psutil.cpu_percent(),
            memory_percent=psutil.virtual_memory().percent
        )
        
        self.network_stats.append(stats)
        return stats
    
    def print_network_status(self):
        """打印网络状态"""
        stats = self.monitor_network()
        
        log("\n" + "=" * 60, "INFO")
        log("网络状态报告", "INFO")
        log("=" * 60, "INFO")
        log(f"总节点数: {stats.total_nodes}", "INFO")
        log(f"健康节点: {stats.healthy_nodes}", "INFO")
        log(f"系统 CPU: {stats.cpu_percent:.1f}%", "INFO")
        log(f"系统内存: {stats.memory_percent:.1f}%", "INFO")
        log("-" * 40, "INFO")
        
        for node_name, config in NODES_CONFIG.items():
            status = "✓" if stats.neighbors_count.get(node_name, -1) >= 0 else "✗"
            node_type = "[恶意]" if config["is_malicious"] else "[正常]"
            neighbors = stats.neighbors_count.get(node_name, 0)
            log(f"  {status} {node_name:12} {node_type} 邻居数: {neighbors}", "INFO")
        
        log("=" * 60, "INFO")
    
    # ==================== 报告生成 ====================
    
    def generate_security_analysis(self) -> Dict:
        """生成安全性分析"""
        analysis = {
            "total_attacks": len(self.attack_events),
            "attack_types": {},
            "defense_effectiveness": {},
            "vulnerabilities_found": [],
            "recommendations": []
        }
        
        for event in self.attack_events:
            attack_type = event.attack_type
            if attack_type not in analysis["attack_types"]:
                analysis["attack_types"][attack_type] = 0
            analysis["attack_types"][attack_type] += 1
        
        # 分析防御效果
        spam_events = [e for e in self.attack_events if e.attack_type == "spam_flooding"]
        if spam_events:
            blocked = sum(1 for e in spam_events if "限流" in e.result or e.response_code == 429)
            analysis["defense_effectiveness"]["rate_limiting"] = f"{blocked}/{len(spam_events)} 攻击被限流"
        
        # 安全建议
        analysis["recommendations"] = [
            "1. 实施更严格的消息签名验证机制",
            "2. 增强速率限制策略，基于节点声誉动态调整",
            "3. 实施节点声誉系统惩罚机制，快速隔离恶意节点",
            "4. 添加消息去重和重放攻击防护",
            "5. 监控异常行为模式并自动封禁",
            "6. 实施分布式入侵检测系统",
        ]
        
        return analysis
    
    def generate_performance_analysis(self) -> Dict:
        """生成性能分析"""
        if not self.network_stats:
            return {}
        
        cpu_values = [s.cpu_percent for s in self.network_stats]
        mem_values = [s.memory_percent for s in self.network_stats]
        
        analysis = {
            "samples": len(self.network_stats),
            "cpu": {
                "avg": sum(cpu_values) / len(cpu_values),
                "max": max(cpu_values),
                "min": min(cpu_values)
            },
            "memory": {
                "avg": sum(mem_values) / len(mem_values),
                "max": max(mem_values),
                "min": min(mem_values)
            },
            "network_health": {
                "avg_healthy_nodes": sum(s.healthy_nodes for s in self.network_stats) / len(self.network_stats)
            }
        }
        
        return analysis
    
    def generate_report(self) -> Path:
        """生成最终报告"""
        self.report["end_time"] = datetime.now().isoformat()
        self.report["attacks"] = [
            {
                "timestamp": e.timestamp,
                "node": e.node,
                "type": e.attack_type,
                "description": e.description,
                "result": e.result
            }
            for e in self.attack_events
        ]
        self.report["security_analysis"] = self.generate_security_analysis()
        self.report["performance_analysis"] = self.generate_performance_analysis()
        
        # 保存报告
        report_file = REPORT_DIR / f"hetero_network_test_{datetime.now().strftime('%Y%m%d_%H%M%S')}.json"
        with open(report_file, 'w', encoding='utf-8') as f:
            json.dump(self.report, f, indent=2, ensure_ascii=False)
        
        log(f"\n测试报告已保存到: {report_file}", "INFO")
        
        # 打印摘要
        log("\n" + "=" * 60, "STEP")
        log("测试总结", "STEP")
        log("=" * 60, "STEP")
        log(f"开始时间: {self.report['start_time']}", "INFO")
        log(f"结束时间: {self.report['end_time']}", "INFO")
        log(f"总攻击次数: {len(self.attack_events)}", "INFO")
        log(f"监控采样数: {len(self.network_stats)}", "INFO")
        
        security = self.report["security_analysis"]
        log("\n攻击类型统计:", "INFO")
        for attack_type, count in security.get("attack_types", {}).items():
            log(f"  - {attack_type}: {count}次", "DEBUG")
        
        log("\n安全建议:", "INFO")
        for rec in security.get("recommendations", []):
            log(f"  {rec}", "DEBUG")
        
        return report_file
    
    # ==================== 主流程 ====================
    
    def run_test(self):
        """运行完整测试"""
        try:
            self.report["start_time"] = datetime.now().isoformat()
            
            log("=" * 60, "STEP")
            log("异构网络 P2P 集群测试开始 (本地进程版)", "STEP")
            log("=" * 60, "STEP")
            log("测试环境: 5个本地节点进程", "INFO")
            log("正常节点: genesis, node1, node2", "INFO")
            log("恶意节点: malicious1, malicious2", "INFO")
            
            # 阶段1: 启动创世节点
            if not self.stage_1_genesis():
                log("创世节点启动失败，测试中止", "ERROR")
                self.stop_all()
                return
            
            time.sleep(3)
            self.print_network_status()
            
            # 阶段2: 启动正常节点
            self.stage_2_normal_nodes()
            time.sleep(3)
            self.print_network_status()
            
            # 阶段3: 启动恶意节点
            self.stage_3_malicious_nodes()
            time.sleep(3)
            self.print_network_status()
            
            # 阶段4: 执行恶意攻击
            self.execute_malicious_attacks()
            
            # 最终状态
            self.print_network_status()
            
            # 生成报告
            self.generate_report()
            
            log("\n" + "=" * 60, "STEP")
            log("测试完成!", "STEP")
            log("=" * 60, "STEP")
            
        except KeyboardInterrupt:
            log("\n测试被中断", "WARN")
        except Exception as e:
            log(f"测试异常: {e}", "ERROR")
            import traceback
            traceback.print_exc()
        finally:
            # 停止所有节点
            self.stop_all()

# ============ 主函数 ============

def main():
    import argparse
    parser = argparse.ArgumentParser(description="异构网络 P2P 集群测试 (本地进程版)")
    parser.add_argument("--keep-running", action="store_true", help="测试后保持节点运行")
    args = parser.parse_args()
    
    tester = HeterogeneousNetworkTestLocal()
    tester.run_test()

if __name__ == "__main__":
    main()
