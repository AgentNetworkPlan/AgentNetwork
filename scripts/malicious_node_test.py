#!/usr/bin/env python3
"""
恶意节点集群测试脚本

功能:
1. 启动创世节点
2. 逐步扩展到10个节点（包含2个恶意节点）
3. 进一步扩展到20个节点（包含2个恶意节点）
4. 模拟恶意节点的各种攻击行为
5. 监控和记录网络状态
6. 关闭所有节点并生成报告

使用方法:
    python malicious_node_test.py [--nodes 10] [--max-nodes 20] [--verbose]
"""

import os
import sys
import json
import time
import uuid
import signal
import socket
import random
import argparse
import subprocess
import threading
from pathlib import Path
from datetime import datetime
from typing import List, Dict, Optional, Any
from dataclasses import dataclass
from concurrent.futures import ThreadPoolExecutor, as_completed
import urllib.request
import urllib.error

# ============ 配置 ============

SCRIPT_DIR = Path(__file__).parent.absolute()
PROJECT_ROOT = SCRIPT_DIR.parent
TESTNET_DIR = PROJECT_ROOT / "testnet_malicious"
LOG_DIR = TESTNET_DIR / "logs"
DATA_DIR = TESTNET_DIR / "data"

# 颜色
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
    }
    color = colors.get(level, "")
    print(f"{color}[{timestamp}] [{level}] {msg}{Colors.RESET}", flush=True)

# ============ 数据结构 ============

@dataclass
class NodeInfo:
    """节点信息"""
    node_id: str
    index: int
    p2p_port: int
    http_port: int
    admin_port: int
    data_dir: str
    is_genesis: bool = False
    is_malicious: bool = False
    pid: int = 0
    status: str = "stopped"
    
    def http_url(self) -> str:
        return f"http://127.0.0.1:{self.http_port}"
    
    def admin_url(self) -> str:
        return f"http://127.0.0.1:{self.admin_port}"

class MaliciousNodeTest:
    """恶意节点测试管理器"""
    
    def __init__(self, initial_nodes: int = 10, max_nodes: int = 20, verbose: bool = False):
        self.initial_nodes = initial_nodes
        self.max_nodes = max_nodes
        self.verbose = verbose
        self.nodes: Dict[str, NodeInfo] = {}
        self.processes: Dict[str, subprocess.Popen] = {}
        self.report = {
            "start_time": datetime.now().isoformat(),
            "stages": [],
            "attack_events": [],
            "network_stats": [],
            "end_time": None
        }
        
        # 初始化目录
        TESTNET_DIR.mkdir(parents=True, exist_ok=True)
        LOG_DIR.mkdir(parents=True, exist_ok=True)
        DATA_DIR.mkdir(parents=True, exist_ok=True)
    
    # ==================== 基础操作 ====================
    
    def find_available_port(self, start_port: int, count: int = 1) -> List[int]:
        """查找可用端口"""
        ports = []
        port = start_port
        
        while len(ports) < count:
            try:
                sock = socket.socket(socket.AF_INET, socket.SOCK_STREAM)
                result = sock.connect_ex(('127.0.0.1', port))
                sock.close()
                
                if result != 0:  # 端口可用
                    ports.append(port)
                port += 1
            except:
                port += 1
        
        return ports
    
    def create_node(self, index: int, is_genesis: bool = False, is_malicious: bool = False) -> NodeInfo:
        """创建节点配置"""
        node_id = f"node{index:02d}"
        
        # 分配端口
        ports = self.find_available_port(9000 + index * 100, 3)
        p2p_port, http_port, admin_port = ports[0], ports[1], ports[2]
        
        # 创建数据目录
        data_dir = DATA_DIR / node_id
        data_dir.mkdir(exist_ok=True)
        
        node = NodeInfo(
            node_id=node_id,
            index=index,
            p2p_port=p2p_port,
            http_port=http_port,
            admin_port=admin_port,
            data_dir=str(data_dir),
            is_genesis=is_genesis,
            is_malicious=is_malicious
        )
        
        self.nodes[node_id] = node
        
        node_type = "GENESIS" if is_genesis else ("MALICIOUS" if is_malicious else "NORMAL")
        log(f"创建节点: {node_id} [类型: {node_type}] [端口: P2P={p2p_port}, HTTP={http_port}, Admin={admin_port}]", "INFO")
        
        return node
    
    def start_node(self, node: NodeInfo, bootstrap_addr: Optional[str] = None) -> bool:
        """启动节点"""
        try:
            cmd = [
                "go", "run", "cmd/node/main.go", "run",
                "-admin", f":{node.admin_port}",
                "-http", f":{node.http_port}",
                "-grpc", f":{node.p2p_port + 1000}",  # 避免端口冲突
                "-listen", f"/ip4/0.0.0.0/tcp/{node.p2p_port}",
                "-data", node.data_dir,
            ]
            
            if bootstrap_addr and not node.is_genesis:
                cmd.extend(["-bootstrap", bootstrap_addr])
            
            log_file = LOG_DIR / f"{node.node_id}.log"
            
            with open(log_file, 'w') as f:
                proc = subprocess.Popen(
                    cmd,
                    cwd=PROJECT_ROOT,
                    stdout=f,
                    stderr=subprocess.STDOUT,
                    text=True
                )
            
            self.processes[node.node_id] = proc
            node.pid = proc.pid
            node.status = "started"
            
            node_type = "[恶意节点]" if node.is_malicious else ""
            log(f"启动节点 {node.node_id} {node_type} (PID: {proc.pid})", "STEP")
            
            # 等待节点启动
            time.sleep(2)
            
            if self.wait_for_node(node, timeout=10):
                node.status = "running"
                log(f"节点 {node.node_id} 已就绪", "INFO")
                return True
            else:
                node.status = "failed"
                log(f"节点 {node.node_id} 启动失败或超时", "ERROR")
                return False
        
        except Exception as e:
            log(f"启动节点 {node.node_id} 异常: {e}", "ERROR")
            node.status = "failed"
            return False
    
    def wait_for_node(self, node: NodeInfo, timeout: int = 30) -> bool:
        """等待节点就绪"""
        start_time = time.time()
        
        while time.time() - start_time < timeout:
            try:
                url = f"{node.http_url()}/health"
                req = urllib.request.Request(url)
                with urllib.request.urlopen(req, timeout=2) as response:
                    if response.status == 200:
                        return True
            except (urllib.error.URLError, Exception):
                pass
            
            time.sleep(0.5)
        
        return False
    
    def stop_node(self, node: NodeInfo) -> bool:
        """停止节点"""
        try:
            if node.node_id in self.processes:
                proc = self.processes[node.node_id]
                proc.terminate()
                
                try:
                    proc.wait(timeout=5)
                except subprocess.TimeoutExpired:
                    proc.kill()
                    proc.wait()
                
                del self.processes[node.node_id]
            
            node.status = "stopped"
            node.pid = 0
            log(f"停止节点 {node.node_id}", "INFO")
            return True
        
        except Exception as e:
            log(f"停止节点 {node.node_id} 异常: {e}", "ERROR")
            return False
    
    # ==================== API 调用 ====================
    
    def api_call(self, node: NodeInfo, endpoint: str, method: str = "GET", data: Optional[Dict] = None) -> Optional[Dict]:
        """调用节点 API"""
        try:
            url = f"{node.http_url()}/api{endpoint}"
            
            if method == "GET":
                req = urllib.request.Request(url, method="GET")
            else:
                body = json.dumps(data).encode('utf-8') if data else None
                req = urllib.request.Request(url, data=body, method=method)
                req.add_header('Content-Type', 'application/json')
            
            with urllib.request.urlopen(req, timeout=5) as response:
                return json.loads(response.read().decode())
        
        except Exception as e:
            if self.verbose:
                log(f"API 调用失败 {node.node_id} {endpoint}: {e}", "DEBUG")
            return None
    
    def get_node_info(self, node: NodeInfo) -> Optional[Dict]:
        """获取节点信息"""
        return self.api_call(node, "/node/info")
    
    def get_neighbors(self, node: NodeInfo) -> Optional[List]:
        """获取邻居列表"""
        result = self.api_call(node, "/neighbor/list")
        if result and isinstance(result, dict):
            return result.get("neighbors", [])
        return []
    
    def broadcast_message(self, node: NodeInfo, message: str, is_malicious: bool = False) -> bool:
        """广播消息"""
        try:
            data = {
                "message": message,
                "type": "malicious" if is_malicious else "normal"
            }
            result = self.api_call(node, "/bulletin/post", "POST", data)
            return result is not None
        except:
            return False
    
    def send_spam(self, node: NodeInfo, count: int = 10) -> int:
        """发送垃圾消息"""
        success = 0
        for i in range(count):
            if self.broadcast_message(node, f"SPAM_MESSAGE_{i}", is_malicious=True):
                success += 1
                time.sleep(0.1)
        return success
    
    # ==================== 恶意行为 ====================
    
    def execute_malicious_behaviors(self, node: NodeInfo):
        """执行恶意行为"""
        if not node.is_malicious:
            return
        
        behaviors = [
            ("发送垃圾消息", lambda: self.send_spam(node, random.randint(5, 15))),
            ("广播虚假信息", lambda: self.broadcast_malicious_false_info(node)),
            ("不规范的邻居请求", lambda: self.malicious_neighbor_request(node)),
        ]
        
        for behavior_name, behavior_func in behaviors:
            try:
                result = behavior_func()
                log(f"[{node.node_id}] {behavior_name}: {result}", "MALICIOUS")
                self.report["attack_events"].append({
                    "timestamp": datetime.now().isoformat(),
                    "node": node.node_id,
                    "behavior": behavior_name,
                    "result": str(result)
                })
            except Exception as e:
                log(f"[{node.node_id}] {behavior_name} 失败: {e}", "WARN")
            
            time.sleep(random.uniform(1, 3))
    
    def broadcast_malicious_false_info(self, node: NodeInfo) -> str:
        """广播虚假信息"""
        messages = [
            "我是超级节点，所有任务都要通过我",
            "该网络已被攻陷，请停止运行",
            "我发现了一个关键漏洞，请转账给我以获得修复",
            "所有节点的声誉都是造假的",
        ]
        msg = random.choice(messages)
        self.broadcast_message(node, msg, is_malicious=True)
        return msg
    
    def malicious_neighbor_request(self, node: NodeInfo) -> str:
        """不规范的邻居请求"""
        # 尝试连接到非常规端口或发送畸形请求
        actions = [
            "尝试连接到未授权的邻居",
            "发送超过限制的连接请求",
            "尝试冒充其他节点",
        ]
        return random.choice(actions)
    
    # ==================== 网络监控 ====================
    
    def monitor_network(self, nodes: List[NodeInfo]) -> Dict:
        """监控网络状态"""
        stats = {
            "timestamp": datetime.now().isoformat(),
            "total_nodes": len(nodes),
            "running_nodes": sum(1 for n in nodes if n.status == "running"),
            "nodes_info": {}
        }
        
        for node in nodes:
            info = {
                "status": node.status,
                "is_malicious": node.is_malicious,
                "neighbors_count": 0,
            }
            
            if node.status == "running":
                neighbors = self.get_neighbors(node)
                info["neighbors_count"] = len(neighbors) if neighbors else 0
            
            stats["nodes_info"][node.node_id] = info
        
        return stats
    
    def print_network_status(self, nodes: List[NodeInfo]):
        """打印网络状态"""
        log("=" * 60, "INFO")
        log(f"网络状态 - 总节点数: {len(nodes)}", "INFO")
        
        for node in nodes:
            status_symbol = "✓" if node.status == "running" else "✗"
            node_type = "[恶意]" if node.is_malicious else "[正常]"
            neighbors = 0
            
            if node.status == "running":
                neighbors = len(self.get_neighbors(node) or [])
            
            log(f"  {status_symbol} {node.node_id} {node_type} 邻居数: {neighbors}", "INFO")
        
        log("=" * 60, "INFO")
    
    # ==================== 测试场景 ====================
    
    def stage_1_genesis(self):
        """阶段1: 启动创世节点"""
        log("\n" + "=" * 60, "STEP")
        log("阶段 1: 启动创世节点", "STEP")
        log("=" * 60, "STEP")
        
        genesis_node = self.create_node(0, is_genesis=True)
        if self.start_node(genesis_node):
            time.sleep(3)
            self.print_network_status([genesis_node])
            self.report["stages"].append({
                "stage": "genesis",
                "nodes": 1,
                "status": "success"
            })
            return genesis_node
        else:
            self.report["stages"].append({
                "stage": "genesis",
                "nodes": 1,
                "status": "failed"
            })
            return None
    
    def stage_2_expand_to_10(self, genesis_node: NodeInfo):
        """阶段2: 扩展到10个节点（包含2个恶意节点）"""
        log("\n" + "=" * 60, "STEP")
        log("阶段 2: 扩展到10个节点（包含2个恶意节点）", "STEP")
        log("=" * 60, "STEP")
        
        nodes = [genesis_node]
        malicious_indices = {2, 7}  # 节点2和7为恶意节点
        
        for i in range(1, self.initial_nodes):
            is_malicious = i in malicious_indices
            node = self.create_node(i, is_malicious=is_malicious)
            
            bootstrap_addr = f"127.0.0.1:{genesis_node.p2p_port}"
            if self.start_node(node, bootstrap_addr):
                nodes.append(node)
                time.sleep(1)
                
                # 定期执行恶意行为
                if is_malicious:
                    threading.Thread(
                        target=self.execute_malicious_behaviors,
                        args=(node,),
                        daemon=True
                    ).start()
        
        time.sleep(5)
        self.print_network_status(nodes)
        
        stats = self.monitor_network(nodes)
        self.report["network_stats"].append(stats)
        self.report["stages"].append({
            "stage": "expand_to_10",
            "nodes": len(nodes),
            "running": stats["running_nodes"],
            "status": "success" if stats["running_nodes"] >= 8 else "partial"
        })
        
        return nodes
    
    def stage_3_expand_to_20(self, nodes: List[NodeInfo]):
        """阶段3: 扩展到20个节点"""
        log("\n" + "=" * 60, "STEP")
        log("阶段 3: 扩展到20个节点（逐步添加）", "STEP")
        log("=" * 60, "STEP")
        
        current_count = len(nodes)
        genesis_node = nodes[0]
        bootstrap_addr = f"127.0.0.1:{genesis_node.p2p_port}"
        
        # 逐步添加节点到20
        for i in range(current_count, self.max_nodes):
            is_malicious = i in {2, 7}  # 保持恶意节点为2和7
            node = self.create_node(i, is_malicious=is_malicious)
            
            if self.start_node(node, bootstrap_addr):
                nodes.append(node)
                
                # 定期执行恶意行为
                if is_malicious:
                    threading.Thread(
                        target=self.execute_malicious_behaviors,
                        args=(node,),
                        daemon=True
                    ).start()
                
                log(f"节点 {node.node_id} 加入网络 (总节点数: {len(nodes)})", "INFO")
                time.sleep(2)
        
        time.sleep(5)
        self.print_network_status(nodes)
        
        stats = self.monitor_network(nodes)
        self.report["network_stats"].append(stats)
        self.report["stages"].append({
            "stage": "expand_to_20",
            "nodes": len(nodes),
            "running": stats["running_nodes"],
            "status": "success" if stats["running_nodes"] >= 18 else "partial"
        })
        
        return nodes
    
    def stage_4_observe_network(self, nodes: List[NodeInfo]):
        """阶段4: 观察网络运行状态"""
        log("\n" + "=" * 60, "STEP")
        log("阶段 4: 观察网络运行状态（持续30秒）", "STEP")
        log("=" * 60, "STEP")
        
        observation_time = 30
        interval = 5
        
        start_time = time.time()
        
        while time.time() - start_time < observation_time:
            elapsed = time.time() - start_time
            log(f"观察进度: {elapsed:.0f}/{observation_time}s", "DEBUG")
            
            stats = self.monitor_network(nodes)
            self.report["network_stats"].append(stats)
            
            malicious_nodes = [n for n in nodes if n.is_malicious and n.status == "running"]
            if malicious_nodes:
                for node in malicious_nodes:
                    if random.random() < 0.5:  # 50% 概率执行恶意行为
                        threading.Thread(
                            target=self.execute_malicious_behaviors,
                            args=(node,),
                            daemon=True
                        ).start()
            
            time.sleep(interval)
    
    def stage_5_shutdown(self, nodes: List[NodeInfo]):
        """阶段5: 逐步关闭所有节点"""
        log("\n" + "=" * 60, "STEP")
        log("阶段 5: 关闭所有节点", "STEP")
        log("=" * 60, "STEP")
        
        # 倒序关闭节点
        for node in reversed(nodes):
            self.stop_node(node)
            time.sleep(1)
        
        log("所有节点已关闭", "INFO")
        self.report["stages"].append({
            "stage": "shutdown",
            "nodes": len(nodes),
            "status": "success"
        })
    
    # ==================== 报告生成 ====================
    
    def generate_report(self):
        """生成测试报告"""
        self.report["end_time"] = datetime.now().isoformat()
        
        report_file = LOG_DIR / "malicious_test_report.json"
        with open(report_file, 'w', encoding='utf-8') as f:
            json.dump(self.report, f, indent=2, ensure_ascii=False)
        
        log(f"\n测试报告已保存到: {report_file}", "INFO")
        
        # 打印摘要
        log("\n" + "=" * 60, "INFO")
        log("测试摘要", "INFO")
        log("=" * 60, "INFO")
        log(f"开始时间: {self.report['start_time']}", "INFO")
        log(f"结束时间: {self.report['end_time']}", "INFO")
        log(f"测试阶段数: {len(self.report['stages'])}", "INFO")
        log(f"恶意事件数: {len(self.report['attack_events'])}", "INFO")
        log(f"网络采样数: {len(self.report['network_stats'])}", "INFO")
        
        if self.report["attack_events"]:
            log("\n恶意事件统计:", "INFO")
            for event in self.report["attack_events"][-5:]:  # 显示最后5个事件
                log(f"  - {event['node']}: {event['behavior']}", "DEBUG")
        
        log("=" * 60, "INFO")
    
    # ==================== 主测试流程 ====================
    
    def run_test(self):
        """运行完整测试"""
        try:
            log("开始恶意节点集群测试", "STEP")
            log(f"测试配置: 初始节点={self.initial_nodes}, 最大节点={self.max_nodes}", "INFO")
            
            # 阶段1: 启动创世节点
            genesis_node = self.stage_1_genesis()
            if not genesis_node:
                log("创世节点启动失败，测试中止", "ERROR")
                return
            
            # 阶段2: 扩展到10个节点
            nodes = self.stage_2_expand_to_10(genesis_node)
            if len(nodes) < 8:
                log("节点启动不足，测试部分中止", "WARN")
            
            # 阶段3: 扩展到20个节点
            nodes = self.stage_3_expand_to_20(nodes)
            
            # 阶段4: 观察网络
            self.stage_4_observe_network(nodes)
            
            # 阶段5: 关闭所有节点
            self.stage_5_shutdown(nodes)
            
            # 生成报告
            self.generate_report()
            
            log("测试完成！", "STEP")
        
        except KeyboardInterrupt:
            log("\n测试被中断，正在清理资源...", "WARN")
            self.cleanup()
            sys.exit(1)
        
        except Exception as e:
            log(f"测试发生异常: {e}", "ERROR")
            self.cleanup()
            sys.exit(1)
    
    def cleanup(self):
        """清理资源"""
        log("清理资源...", "INFO")
        for node_id in list(self.nodes.keys()):
            node = self.nodes[node_id]
            self.stop_node(node)
        
        log("资源清理完成", "INFO")

# ============ 主函数 ============

def main():
    parser = argparse.ArgumentParser(description="恶意节点集群测试")
    parser.add_argument("--nodes", type=int, default=10, help="初始节点数 (默认: 10)")
    parser.add_argument("--max-nodes", type=int, default=20, help="最大节点数 (默认: 20)")
    parser.add_argument("-v", "--verbose", action="store_true", help="详细输出")
    
    args = parser.parse_args()
    
    tester = MaliciousNodeTest(
        initial_nodes=args.nodes,
        max_nodes=args.max_nodes,
        verbose=args.verbose
    )
    
    tester.run_test()

if __name__ == "__main__":
    main()
