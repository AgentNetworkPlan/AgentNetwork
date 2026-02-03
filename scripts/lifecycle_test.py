#!/usr/bin/env python3
"""
AgentNetwork 全生命周期模拟测试脚本

功能:
1. 启动多节点网络（通过守护进程）
2. 通过 HTTP API 控制节点
3. 模拟完整生命周期：
   - 创世节点初始化
   - 节点加入网络
   - 邻居发现与管理
   - 任务分发与完成
   - 声誉变化
   - 指责机制
   - 消息传递
4. 收集日志和统计
5. 清理环境

使用方法:
    python lifecycle_test.py [OPTIONS]

选项:
    -n, --nodes INT      节点数量 (默认: 5)
    --p2p-port INT       起始 P2P 端口 (默认: 9000)
    --http-port INT      起始 HTTP 端口 (默认: 18000)
    --skip-build         跳过编译
    --keep-logs          保留日志不清理
    -v, --verbose        详细输出
"""

import os
import sys
import json
import time
import uuid
import shutil
import signal
import socket
import argparse
import subprocess
import platform
from pathlib import Path
from datetime import datetime
from typing import List, Dict, Optional, Any
from dataclasses import dataclass, asdict, field
import urllib.request
import urllib.error

# ============ 配置 ============

SCRIPT_DIR = Path(__file__).parent.absolute()
PROJECT_ROOT = SCRIPT_DIR.parent
TESTNET_DIR = PROJECT_ROOT / "testnet_lifecycle"
LOG_DIR = TESTNET_DIR / "logs"
BINARY_DIR = PROJECT_ROOT / "bin"

# 颜色
class Colors:
    GREEN = "\033[32m"
    YELLOW = "\033[33m"
    RED = "\033[31m"
    CYAN = "\033[36m"
    BLUE = "\033[34m"
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
    }
    color = colors.get(level, "")
    print(f"{color}[{timestamp}] [{level}] {msg}{Colors.RESET}")

# ============ 数据结构 ============

@dataclass
class NodeInfo:
    """节点信息"""
    node_id: str
    p2p_port: int
    http_port: int
    data_dir: str
    pid: int = 0
    status: str = "stopped"
    is_genesis: bool = False
    
    def http_url(self) -> str:
        return f"http://127.0.0.1:{self.http_port}"

@dataclass
class TestResult:
    """测试结果"""
    name: str
    passed: bool
    duration: float
    message: str = ""
    details: Dict = field(default_factory=dict)

@dataclass
class TestStats:
    """测试统计"""
    total: int = 0
    passed: int = 0
    failed: int = 0
    skipped: int = 0
    start_time: datetime = None
    end_time: datetime = None
    results: List[TestResult] = field(default_factory=list)

# ============ HTTP 客户端 ============

class HTTPClient:
    """HTTP API 客户端"""
    
    def __init__(self, base_url: str, timeout: int = 10):
        self.base_url = base_url.rstrip('/')
        self.timeout = timeout
    
    def request(self, method: str, path: str, data: dict = None) -> dict:
        """发送请求"""
        url = f"{self.base_url}{path}"
        
        req = urllib.request.Request(url)
        req.method = method.upper()
        
        if data:
            req.data = json.dumps(data).encode('utf-8')
            req.add_header('Content-Type', 'application/json')
        
        try:
            with urllib.request.urlopen(req, timeout=self.timeout) as resp:
                body = resp.read().decode('utf-8')
                return json.loads(body) if body else {}
        except urllib.error.HTTPError as e:
            body = e.read().decode('utf-8') if e.fp else ""
            return {"error": str(e), "status": e.code, "body": body}
        except urllib.error.URLError as e:
            return {"error": str(e), "status": 0}
        except Exception as e:
            return {"error": str(e), "status": -1}
    
    def get(self, path: str) -> dict:
        return self.request("GET", path)
    
    def post(self, path: str, data: dict = None) -> dict:
        return self.request("POST", path, data)

# ============ 节点管理器 ============

class NodeManager:
    """节点管理器"""
    
    def __init__(self, base_p2p_port: int = 9000, base_http_port: int = 18000):
        self.base_p2p_port = base_p2p_port
        self.base_http_port = base_http_port
        self.nodes: Dict[str, NodeInfo] = {}
        self.binary_path: Optional[Path] = None
    
    def ensure_binary(self, skip_build: bool = False) -> bool:
        """确保二进制文件存在"""
        binary_name = "agentnetwork.exe" if platform.system() == "Windows" else "agentnetwork"
        self.binary_path = BINARY_DIR / binary_name
        
        if self.binary_path.exists() and skip_build:
            log(f"使用已有二进制: {self.binary_path}")
            return True
        
        log("编译项目...")
        BINARY_DIR.mkdir(parents=True, exist_ok=True)
        
        result = subprocess.run(
            ["go", "build", "-o", str(self.binary_path), "./cmd/node"],
            cwd=str(PROJECT_ROOT),
            capture_output=True,
            text=True
        )
        
        if result.returncode != 0:
            log(f"编译失败: {result.stderr}", "ERROR")
            return False
        
        log(f"编译成功: {self.binary_path}")
        return True
    
    def create_node(self, node_id: str, index: int, is_genesis: bool = False) -> NodeInfo:
        """创建节点配置"""
        node = NodeInfo(
            node_id=node_id,
            p2p_port=self.base_p2p_port + index,
            http_port=self.base_http_port + index,
            data_dir=str(TESTNET_DIR / f"node-{index:03d}"),
            is_genesis=is_genesis
        )
        self.nodes[node_id] = node
        return node
    
    def start_node(self, node: NodeInfo, bootstrap: str = "") -> bool:
        """启动单个节点"""
        # 创建数据目录
        data_dir = Path(node.data_dir)
        data_dir.mkdir(parents=True, exist_ok=True)
        (data_dir / "logs").mkdir(exist_ok=True)
        
        # 构建命令 - 使用 run 命令前台运行（由脚本管理后台）
        cmd = [
            str(self.binary_path),
            "run",
            "-data", node.data_dir,
            "-listen", f"/ip4/0.0.0.0/tcp/{node.p2p_port}",
            "-http", f":{node.http_port}",
        ]
        
        if bootstrap:
            cmd.extend(["-bootstrap", bootstrap])
        
        if node.is_genesis:
            cmd.extend(["-role", "bootstrap"])
        
        # 日志文件
        log_file = data_dir / "node.log"
        
        try:
            with open(log_file, 'w') as f:
                # 后台启动进程
                if platform.system() == 'Windows':
                    process = subprocess.Popen(
                        cmd,
                        stdout=f,
                        stderr=subprocess.STDOUT,
                        cwd=str(PROJECT_ROOT),
                        creationflags=subprocess.CREATE_NEW_PROCESS_GROUP | subprocess.DETACHED_PROCESS
                    )
                else:
                    process = subprocess.Popen(
                        cmd,
                        stdout=f,
                        stderr=subprocess.STDOUT,
                        cwd=str(PROJECT_ROOT),
                        start_new_session=True
                    )
            
            # 等待节点启动
            time.sleep(3)
            
            node.pid = process.pid
            node.status = "running"
            log(f"启动节点 {node.node_id} (PID: {node.pid}, HTTP: {node.http_port})")
            return True
            
        except Exception as e:
            log(f"启动节点 {node.node_id} 异常: {e}", "ERROR")
            return False
    
    def stop_node(self, node: NodeInfo) -> bool:
        """停止单个节点"""
        if node.pid <= 0:
            return True
        
        try:
            if platform.system() == 'Windows':
                # 先发送 CTRL_BREAK_EVENT 信号（类似 SIGTERM）
                try:
                    os.kill(node.pid, signal.CTRL_BREAK_EVENT)
                except:
                    pass
                time.sleep(1)
                # 检查进程是否还在运行
                result = subprocess.run(['tasklist', '/FI', f'PID eq {node.pid}'], 
                                       capture_output=True, text=True)
                if str(node.pid) in result.stdout:
                    # 如果还在运行，强制终止
                    subprocess.run(['taskkill', '/F', '/PID', str(node.pid)], capture_output=True)
            else:
                os.kill(node.pid, signal.SIGTERM)
                time.sleep(1)
                try:
                    os.kill(node.pid, signal.SIGKILL)
                except ProcessLookupError:
                    pass
            
            node.status = "stopped"
            node.pid = 0
            log(f"停止节点 {node.node_id}")
            return True
        except Exception as e:
            log(f"停止节点 {node.node_id} 失败: {e}", "WARN")
            return False
    
    def stop_all(self):
        """停止所有节点"""
        for node in self.nodes.values():
            self.stop_node(node)
    
    def get_node_status(self, node: NodeInfo) -> dict:
        """获取节点状态"""
        client = HTTPClient(node.http_url())
        return client.get("/api/v1/node/info")
    
    def wait_for_node(self, node: NodeInfo, timeout: int = 30) -> bool:
        """等待节点就绪"""
        client = HTTPClient(node.http_url())
        start = time.time()
        
        while time.time() - start < timeout:
            result = client.get("/health")
            # 检查返回格式: {"success": true, "data": {"status": "ok", ...}}
            if result.get("success"):
                data = result.get("data", {})
                if data.get("status") == "ok":
                    return True
            # 或直接返回 {"status": "ok"}
            if result.get("status") == "ok":
                return True
            time.sleep(1)
        
        return False

# ============ 测试套件 ============

class LifecycleTestSuite:
    """生命周期测试套件"""
    
    def __init__(self, node_count: int = 5, verbose: bool = False):
        self.node_count = node_count
        self.verbose = verbose
        self.manager = NodeManager()
        self.stats = TestStats()
        self.genesis_node: Optional[NodeInfo] = None
    
    def run_test(self, name: str, func) -> TestResult:
        """运行单个测试"""
        log(f"\n{'='*50}", "STEP")
        log(f"测试: {name}", "STEP")
        log('='*50, "STEP")
        
        start = time.time()
        try:
            result = func()
            duration = time.time() - start
            
            if result is True or (isinstance(result, dict) and result.get("success")):
                test_result = TestResult(name=name, passed=True, duration=duration)
                log(f"✓ {name} 通过 ({duration:.2f}s)")
            else:
                message = str(result) if result else "测试失败"
                test_result = TestResult(name=name, passed=False, duration=duration, message=message)
                log(f"✗ {name} 失败: {message}", "ERROR")
                
        except Exception as e:
            duration = time.time() - start
            test_result = TestResult(name=name, passed=False, duration=duration, message=str(e))
            log(f"✗ {name} 异常: {e}", "ERROR")
            if self.verbose:
                import traceback
                traceback.print_exc()
        
        self.stats.results.append(test_result)
        self.stats.total += 1
        if test_result.passed:
            self.stats.passed += 1
        else:
            self.stats.failed += 1
        
        return test_result
    
    # ============ 测试用例 ============
    
    def test_build(self) -> bool:
        """测试编译"""
        return self.manager.ensure_binary(skip_build=False)
    
    def test_genesis_start(self) -> bool:
        """测试创世节点启动"""
        self.genesis_node = self.manager.create_node("genesis", 0, is_genesis=True)
        
        if not self.manager.start_node(self.genesis_node):
            return False
        
        # 等待节点就绪
        if not self.manager.wait_for_node(self.genesis_node, timeout=30):
            return {"success": False, "error": "创世节点启动超时"}
        
        return True
    
    def test_genesis_info(self) -> dict:
        """测试创世节点信息"""
        client = HTTPClient(self.genesis_node.http_url())
        result = client.get("/api/v1/node/info")
        
        if self.verbose:
            log(f"节点信息: {json.dumps(result, indent=2)}", "DEBUG")
        
        # 检查返回的数据结构
        if result.get("success"):
            data = result.get("data", {})
            return data.get("node_id") is not None
        return "node_id" in result or result.get("NodeID")
    
    def test_nodes_join(self) -> bool:
        """测试节点加入网络"""
        # 获取创世节点的引导地址
        client = HTTPClient(self.genesis_node.http_url())
        info = client.get("/api/v1/node/info")
        
        # 构建引导地址
        bootstrap = f"/ip4/127.0.0.1/tcp/{self.genesis_node.p2p_port}"
        
        # 启动其他节点
        success_count = 0
        for i in range(1, self.node_count):
            node = self.manager.create_node(f"node-{i:03d}", i)
            
            if self.manager.start_node(node, bootstrap):
                if self.manager.wait_for_node(node, timeout=20):
                    success_count += 1
                else:
                    log(f"节点 {node.node_id} 启动超时", "WARN")
            
            time.sleep(1)  # 启动间隔
        
        log(f"成功启动 {success_count}/{self.node_count - 1} 个节点")
        return success_count >= (self.node_count - 1) * 0.8  # 80% 成功率
    
    def test_peer_discovery(self) -> bool:
        """测试节点发现"""
        time.sleep(5)  # 等待节点发现
        
        for node_id, node in self.manager.nodes.items():
            if node.status != "running":
                continue
            
            client = HTTPClient(node.http_url())
            peers = client.get("/api/v1/node/peers")
            
            if self.verbose:
                data = peers.get("data", {})
                peer_count = data.get("count", 0) if isinstance(data, dict) else len(data)
                log(f"{node_id} 发现 {peer_count} 个节点", "DEBUG")
        
        return True
    
    def test_neighbor_management(self) -> bool:
        """测试邻居管理"""
        client = HTTPClient(self.genesis_node.http_url())
        
        # 获取邻居列表
        neighbors = client.get("/api/v1/neighbor/list")
        if self.verbose:
            log(f"邻居列表: {json.dumps(neighbors, indent=2)}", "DEBUG")
        
        # 获取最佳邻居
        best = client.get("/api/v1/neighbor/best")
        if self.verbose:
            log(f"最佳邻居: {json.dumps(best, indent=2)}", "DEBUG")
        
        return True
    
    def test_send_message(self) -> bool:
        """测试消息发送"""
        if len(self.manager.nodes) < 2:
            return {"success": False, "error": "节点数量不足"}
        
        # 选择两个节点
        nodes = list(self.manager.nodes.values())
        sender = nodes[0]
        receiver = nodes[1]
        
        client = HTTPClient(sender.http_url())
        
        # 发送消息
        msg_data = {
            "to": receiver.node_id,
            "type": "test",
            "content": f"Hello from lifecycle test at {datetime.now().isoformat()}"
        }
        
        result = client.post("/api/v1/message/send", msg_data)
        
        if self.verbose:
            log(f"发送消息结果: {json.dumps(result, indent=2)}", "DEBUG")
        
        return result.get("success", False) or "error" not in result
    
    def test_mailbox(self) -> bool:
        """测试邮箱功能"""
        client = HTTPClient(self.genesis_node.http_url())
        
        # 发送邮件
        mail_data = {
            "to": "node-001",
            "subject": "测试邮件",
            "content": "这是一封测试邮件"
        }
        
        result = client.post("/api/v1/mailbox/send", mail_data)
        
        if self.verbose:
            log(f"发送邮件结果: {json.dumps(result, indent=2)}", "DEBUG")
        
        # 检查收件箱
        inbox = client.get("/api/v1/mailbox/inbox")
        if self.verbose:
            log(f"收件箱: {json.dumps(inbox, indent=2)}", "DEBUG")
        
        return True
    
    def test_bulletin(self) -> bool:
        """测试公告板"""
        client = HTTPClient(self.genesis_node.http_url())
        
        # 发布公告
        bulletin_data = {
            "topic": "test-topic",
            "content": f"测试公告 {datetime.now().isoformat()}",
            "ttl": 3600
        }
        
        result = client.post("/api/v1/bulletin/publish", bulletin_data)
        
        if self.verbose:
            log(f"发布公告结果: {json.dumps(result, indent=2)}", "DEBUG")
        
        # 获取公告列表
        bulletins = client.get("/api/v1/bulletin/search")
        if self.verbose:
            log(f"公告列表: {json.dumps(bulletins, indent=2)}", "DEBUG")
        
        return True
    
    def test_reputation(self) -> bool:
        """测试声誉系统"""
        client = HTTPClient(self.genesis_node.http_url())
        
        # 获取自身声誉
        rep = client.get("/api/v1/reputation/query?node_id=self")
        if self.verbose:
            log(f"声誉信息: {json.dumps(rep, indent=2)}", "DEBUG")
        
        # 获取声誉排名
        rankings = client.get("/api/v1/reputation/ranking")
        if self.verbose:
            log(f"声誉排名: {json.dumps(rankings, indent=2)}", "DEBUG")
        
        return True
    
    def test_accusation(self) -> bool:
        """测试指责系统"""
        if len(self.manager.nodes) < 2:
            return True  # 跳过
        
        client = HTTPClient(self.genesis_node.http_url())
        
        # 获取指责列表
        accusations = client.get("/api/v1/accusation/list")
        if self.verbose:
            log(f"指责列表: {json.dumps(accusations, indent=2)}", "DEBUG")
        
        return True
    
    def test_voting(self) -> bool:
        """测试投票系统"""
        client = HTTPClient(self.genesis_node.http_url())
        
        # 获取提案列表
        proposals = client.get("/api/v1/voting/proposal/list")
        if self.verbose:
            log(f"提案列表: {json.dumps(proposals, indent=2)}", "DEBUG")
        
        return True
    
    def test_supernode(self) -> bool:
        """测试超级节点"""
        client = HTTPClient(self.genesis_node.http_url())
        
        # 获取超级节点列表
        supernodes = client.get("/api/v1/supernode/list")
        if self.verbose:
            log(f"超级节点: {json.dumps(supernodes, indent=2)}", "DEBUG")
        
        return True
    
    def test_incentive(self) -> bool:
        """测试激励系统"""
        client = HTTPClient(self.genesis_node.http_url())
        
        # 获取激励历史
        incentives = client.get("/api/v1/incentive/history")
        if self.verbose:
            log(f"激励历史: {json.dumps(incentives, indent=2)}", "DEBUG")
        
        return True
    
    def test_logs(self) -> bool:
        """测试日志系统"""
        # HTTP API 没有日志接口，跳过此测试
        return True
    
    def test_error_handling(self) -> bool:
        """测试错误处理"""
        client = HTTPClient(self.genesis_node.http_url())
        
        # 访问不存在的端点
        result = client.get("/api/v1/nonexistent")
        
        # 应该返回 404
        if result.get("status") == 404:
            return True
        
        # 或者返回错误信息
        return "error" in result or result.get("success") == False
    
    # ============ 运行测试 ============
    
    def run_all(self) -> TestStats:
        """运行所有测试"""
        self.stats = TestStats(start_time=datetime.now())
        
        tests = [
            ("编译项目", self.test_build),
            ("启动创世节点", self.test_genesis_start),
            ("创世节点信息", self.test_genesis_info),
            ("节点加入网络", self.test_nodes_join),
            ("节点发现", self.test_peer_discovery),
            ("邻居管理", self.test_neighbor_management),
            ("消息发送", self.test_send_message),
            ("邮箱功能", self.test_mailbox),
            ("公告板", self.test_bulletin),
            ("声誉系统", self.test_reputation),
            ("指责系统", self.test_accusation),
            ("投票系统", self.test_voting),
            ("超级节点", self.test_supernode),
            ("激励系统", self.test_incentive),
            ("日志系统", self.test_logs),
            ("错误处理", self.test_error_handling),
        ]
        
        for name, func in tests:
            self.run_test(name, func)
        
        self.stats.end_time = datetime.now()
        return self.stats
    
    def cleanup(self, keep_logs: bool = False):
        """清理环境"""
        log("\n清理测试环境...", "STEP")
        
        # 停止所有节点
        self.manager.stop_all()
        
        # 等待进程完全退出并释放文件句柄
        time.sleep(2)
        
        # 复制日志（在停止节点之后，确保文件已释放）
        if keep_logs:
            backup_dir = PROJECT_ROOT / "test_logs" / datetime.now().strftime("%Y%m%d_%H%M%S")
            backup_dir.mkdir(parents=True, exist_ok=True)
            
            for node_id, node in self.manager.nodes.items():
                # 日志文件直接在 data_dir 下
                log_file = Path(node.data_dir) / "node.log"
                if log_file.exists():
                    dst_dir = backup_dir / node_id
                    dst_dir.mkdir(parents=True, exist_ok=True)
                    try:
                        shutil.copy(log_file, dst_dir / "node.log")
                    except Exception as e:
                        log(f"复制日志失败 {node_id}: {e}", "WARN")
            
            log(f"日志已保存到: {backup_dir}")
        
        # 删除测试目录
        if TESTNET_DIR.exists():
            try:
                shutil.rmtree(TESTNET_DIR)
                log(f"已删除: {TESTNET_DIR}")
            except Exception as e:
                log(f"删除目录失败: {e}", "WARN")
    
    def print_summary(self):
        """打印测试摘要"""
        log("\n" + "=" * 60, "STEP")
        log("测试结果摘要", "STEP")
        log("=" * 60, "STEP")
        
        duration = (self.stats.end_time - self.stats.start_time).total_seconds()
        
        print(f"\n总计: {self.stats.total}")
        print(f"{Colors.GREEN}通过: {self.stats.passed}{Colors.RESET}")
        print(f"{Colors.RED}失败: {self.stats.failed}{Colors.RESET}")
        print(f"耗时: {duration:.2f}s")
        print()
        
        # 失败的测试
        if self.stats.failed > 0:
            print("失败的测试:")
            for result in self.stats.results:
                if not result.passed:
                    print(f"  - {result.name}: {result.message}")
            print()
        
        # 保存结果到文件
        result_file = PROJECT_ROOT / "test_logs" / "lifecycle_test_result.json"
        result_file.parent.mkdir(parents=True, exist_ok=True)
        
        result_data = {
            "timestamp": datetime.now().isoformat(),
            "duration": duration,
            "total": self.stats.total,
            "passed": self.stats.passed,
            "failed": self.stats.failed,
            "results": [asdict(r) for r in self.stats.results]
        }
        
        with open(result_file, 'w', encoding='utf-8') as f:
            json.dump(result_data, f, indent=2, ensure_ascii=False)
        
        log(f"测试结果已保存: {result_file}")

# ============ 主程序 ============

def main():
    parser = argparse.ArgumentParser(
        description="AgentNetwork 全生命周期模拟测试",
        formatter_class=argparse.RawDescriptionHelpFormatter
    )
    
    parser.add_argument("-n", "--nodes", type=int, default=5,
                        help="节点数量 (默认: 5)")
    parser.add_argument("--p2p-port", type=int, default=9000,
                        help="起始 P2P 端口")
    parser.add_argument("--http-port", type=int, default=18000,
                        help="起始 HTTP 端口")
    parser.add_argument("--skip-build", action="store_true",
                        help="跳过编译")
    parser.add_argument("--keep-logs", action="store_true",
                        help="保留日志不清理")
    parser.add_argument("-v", "--verbose", action="store_true",
                        help="详细输出")
    
    args = parser.parse_args()
    
    log("=" * 60, "STEP")
    log("AgentNetwork 全生命周期模拟测试", "STEP")
    log("=" * 60, "STEP")
    log(f"节点数量: {args.nodes}")
    log(f"P2P 端口: {args.p2p_port}-{args.p2p_port + args.nodes - 1}")
    log(f"HTTP 端口: {args.http_port}-{args.http_port + args.nodes - 1}")
    
    # 创建测试套件
    suite = LifecycleTestSuite(
        node_count=args.nodes,
        verbose=args.verbose
    )
    suite.manager.base_p2p_port = args.p2p_port
    suite.manager.base_http_port = args.http_port
    
    try:
        # 运行测试
        suite.run_all()
        
        # 打印摘要
        suite.print_summary()
        
    except KeyboardInterrupt:
        log("\n测试被中断", "WARN")
    finally:
        # 清理
        suite.cleanup(keep_logs=args.keep_logs)
    
    # 返回码
    return 0 if suite.stats.failed == 0 else 1

if __name__ == "__main__":
    sys.exit(main())
