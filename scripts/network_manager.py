#!/usr/bin/env python3
"""
AgentNetwork 网络管理脚本
用于启动、测试、重启和清理网络环境

功能:
- start: 启动多节点网络
- stop: 停止所有节点
- restart: 重启网络
- test: 运行测试
- clear: 清理环境
- status: 查看节点状态
"""

import os
import sys
import json
import time
import signal
import shutil
import socket
import argparse
import subprocess
import platform
from pathlib import Path
from datetime import datetime
from typing import List, Dict, Optional
from dataclasses import dataclass, asdict
from concurrent.futures import ThreadPoolExecutor, as_completed

# ============ 配置 ============

@dataclass
class NodeConfig:
    """节点配置"""
    node_id: str
    p2p_port: int
    http_port: int
    data_dir: str
    bootstrap: str = ""
    is_genesis: bool = False
    is_supernode: bool = False

@dataclass 
class NetworkConfig:
    """网络配置"""
    name: str = "testnet"
    base_dir: str = "./testnet"
    node_count: int = 5
    base_p2p_port: int = 9000
    base_http_port: int = 18000
    genesis_node_id: str = "genesis-node-001"

# ============ 全局变量 ============

SCRIPT_DIR = Path(__file__).parent.absolute()
PROJECT_ROOT = SCRIPT_DIR.parent
DATA_DIR = PROJECT_ROOT / "testnet"
PID_FILE = DATA_DIR / "pids.json"
CONFIG_FILE = DATA_DIR / "network.json"
LOG_DIR = DATA_DIR / "logs"

# 进程列表
processes: Dict[str, subprocess.Popen] = {}

# ============ 工具函数 ============

def log(msg: str, level: str = "INFO"):
    """打印日志"""
    timestamp = datetime.now().strftime("%Y-%m-%d %H:%M:%S")
    colors = {
        "INFO": "\033[32m",    # 绿色
        "WARN": "\033[33m",    # 黄色
        "ERROR": "\033[31m",   # 红色
        "DEBUG": "\033[36m",   # 青色
    }
    reset = "\033[0m"
    color = colors.get(level, "")
    print(f"{color}[{timestamp}] [{level}] {msg}{reset}")

def is_port_in_use(port: int) -> bool:
    """检查端口是否被占用"""
    with socket.socket(socket.AF_INET, socket.SOCK_STREAM) as s:
        return s.connect_ex(('127.0.0.1', port)) == 0

def find_available_port(start_port: int, count: int = 1) -> List[int]:
    """查找可用端口"""
    ports = []
    port = start_port
    while len(ports) < count:
        if not is_port_in_use(port):
            ports.append(port)
        port += 1
    return ports

def ensure_dir(path: Path):
    """确保目录存在"""
    path.mkdir(parents=True, exist_ok=True)

def load_json(path: Path) -> dict:
    """加载JSON文件"""
    if path.exists():
        with open(path, 'r', encoding='utf-8') as f:
            return json.load(f)
    return {}

def save_json(path: Path, data: dict):
    """保存JSON文件"""
    ensure_dir(path.parent)
    with open(path, 'w', encoding='utf-8') as f:
        json.dump(data, f, indent=2, ensure_ascii=False)

def run_command(cmd: List[str], cwd: str = None, capture: bool = True) -> tuple:
    """运行命令"""
    try:
        result = subprocess.run(
            cmd,
            cwd=cwd or str(PROJECT_ROOT),
            capture_output=capture,
            text=True,
            timeout=300
        )
        return result.returncode, result.stdout, result.stderr
    except subprocess.TimeoutExpired:
        return -1, "", "Command timeout"
    except Exception as e:
        return -1, "", str(e)

# ============ 节点管理 ============

def generate_node_configs(config: NetworkConfig) -> List[NodeConfig]:
    """生成节点配置列表"""
    nodes = []
    
    # 创世节点
    genesis = NodeConfig(
        node_id=config.genesis_node_id,
        p2p_port=config.base_p2p_port,
        http_port=config.base_http_port,
        data_dir=str(Path(config.base_dir) / "node-genesis"),
        is_genesis=True,
        is_supernode=True
    )
    nodes.append(genesis)
    
    # 普通节点
    bootstrap_addr = f"/ip4/127.0.0.1/tcp/{config.base_p2p_port}/p2p/{config.genesis_node_id}"
    
    for i in range(1, config.node_count):
        node = NodeConfig(
            node_id=f"node-{i:03d}",
            p2p_port=config.base_p2p_port + i,
            http_port=config.base_http_port + i,
            data_dir=str(Path(config.base_dir) / f"node-{i:03d}"),
            bootstrap=bootstrap_addr
        )
        nodes.append(node)
    
    return nodes

def create_node_config_file(node: NodeConfig) -> str:
    """为节点创建配置文件"""
    config_dir = Path(node.data_dir)
    ensure_dir(config_dir)
    
    config = {
        "node_id": node.node_id,
        "p2p": {
            "listen_addr": f"/ip4/0.0.0.0/tcp/{node.p2p_port}",
            "bootstrap_peers": [node.bootstrap] if node.bootstrap else []
        },
        "http": {
            "enabled": True,
            "port": node.http_port,
            "host": "127.0.0.1"
        },
        "data_dir": node.data_dir,
        "is_genesis": node.is_genesis,
        "is_supernode": node.is_supernode,
        "log": {
            "level": "debug",
            "file": str(config_dir / "node.log")
        }
    }
    
    config_path = config_dir / "config.json"
    save_json(config_path, config)
    return str(config_path)

def start_node(node: NodeConfig, binary_path: str) -> Optional[subprocess.Popen]:
    """启动单个节点"""
    config_path = create_node_config_file(node)
    log_file = Path(node.data_dir) / "node.log"
    
    # 检查端口
    if is_port_in_use(node.p2p_port):
        log(f"P2P端口 {node.p2p_port} 已被占用", "ERROR")
        return None
    if is_port_in_use(node.http_port):
        log(f"HTTP端口 {node.http_port} 已被占用", "ERROR")
        return None
    
    # 构建启动命令
    cmd = [
        binary_path,
        "--config", config_path,
    ]
    
    # 打开日志文件
    ensure_dir(log_file.parent)
    log_handle = open(log_file, 'a', encoding='utf-8')
    
    try:
        # 启动进程
        process = subprocess.Popen(
            cmd,
            stdout=log_handle,
            stderr=subprocess.STDOUT,
            cwd=str(PROJECT_ROOT),
            creationflags=subprocess.CREATE_NEW_PROCESS_GROUP if platform.system() == 'Windows' else 0
        )
        
        log(f"启动节点 {node.node_id} (PID: {process.pid}, P2P: {node.p2p_port}, HTTP: {node.http_port})")
        return process
        
    except Exception as e:
        log(f"启动节点 {node.node_id} 失败: {e}", "ERROR")
        return None

def stop_node(node_id: str, pid: int):
    """停止单个节点"""
    try:
        if platform.system() == 'Windows':
            subprocess.run(['taskkill', '/F', '/PID', str(pid)], 
                         capture_output=True)
        else:
            os.kill(pid, signal.SIGTERM)
            time.sleep(1)
            try:
                os.kill(pid, signal.SIGKILL)
            except ProcessLookupError:
                pass
        log(f"停止节点 {node_id} (PID: {pid})")
    except ProcessLookupError:
        log(f"节点 {node_id} (PID: {pid}) 已不存在", "WARN")
    except Exception as e:
        log(f"停止节点 {node_id} 失败: {e}", "ERROR")

# ============ 网络管理命令 ============

def cmd_start(args):
    """启动网络"""
    log("=" * 50)
    log("启动 AgentNetwork 测试网络")
    log("=" * 50)
    
    # 创建网络配置
    net_config = NetworkConfig(
        name=args.name,
        base_dir=str(DATA_DIR),
        node_count=args.nodes,
        base_p2p_port=args.p2p_port,
        base_http_port=args.http_port
    )
    
    # 生成节点配置
    nodes = generate_node_configs(net_config)
    
    # 检查/编译二进制
    binary_name = "agentnetwork.exe" if platform.system() == 'Windows' else "agentnetwork"
    binary_path = PROJECT_ROOT / "bin" / binary_name
    
    if not binary_path.exists() or args.rebuild:
        log("编译项目...")
        ensure_dir(binary_path.parent)
        code, stdout, stderr = run_command([
            "go", "build", "-o", str(binary_path), "./cmd/node"
        ])
        if code != 0:
            log(f"编译失败: {stderr}", "ERROR")
            # 如果没有 cmd/node，尝试模拟模式
            log("使用模拟模式启动...", "WARN")
            binary_path = None
    
    # 保存网络配置
    network_data = {
        "config": asdict(net_config),
        "nodes": [asdict(n) for n in nodes],
        "started_at": datetime.now().isoformat()
    }
    save_json(CONFIG_FILE, network_data)
    
    # 启动节点
    pids = {}
    
    if binary_path and binary_path.exists():
        # 真实模式：启动实际进程
        for node in nodes:
            process = start_node(node, str(binary_path))
            if process:
                pids[node.node_id] = process.pid
                processes[node.node_id] = process
            time.sleep(0.5)  # 启动间隔
    else:
        # 模拟模式：只创建配置文件
        log("模拟模式：仅创建配置文件，不启动实际进程", "WARN")
        for node in nodes:
            create_node_config_file(node)
            pids[node.node_id] = -1  # 模拟PID
            log(f"[模拟] 节点 {node.node_id} 配置已创建")
    
    # 保存PID
    save_json(PID_FILE, pids)
    
    log("=" * 50)
    log(f"网络启动完成: {len(pids)} 个节点")
    log(f"数据目录: {DATA_DIR}")
    log(f"创世节点 HTTP: http://127.0.0.1:{net_config.base_http_port}")
    log("=" * 50)
    
    return 0

def cmd_stop(args):
    """停止网络"""
    log("停止 AgentNetwork 测试网络...")
    
    pids = load_json(PID_FILE)
    if not pids:
        log("没有运行中的节点", "WARN")
        return 0
    
    for node_id, pid in pids.items():
        if pid > 0:  # 忽略模拟模式的PID
            stop_node(node_id, pid)
    
    # 清空PID文件
    save_json(PID_FILE, {})
    
    log("所有节点已停止")
    return 0

def cmd_restart(args):
    """重启网络"""
    log("重启 AgentNetwork 测试网络...")
    
    # 加载原配置
    network_data = load_json(CONFIG_FILE)
    if not network_data:
        log("没有找到网络配置，请先启动网络", "ERROR")
        return 1
    
    # 停止
    cmd_stop(args)
    time.sleep(2)
    
    # 使用原配置重新启动
    config = network_data.get("config", {})
    args.name = config.get("name", "testnet")
    args.nodes = config.get("node_count", 5)
    args.p2p_port = config.get("base_p2p_port", 9000)
    args.http_port = config.get("base_http_port", 18000)
    args.rebuild = False
    
    return cmd_start(args)

def cmd_status(args):
    """查看网络状态"""
    log("AgentNetwork 网络状态")
    log("=" * 60)
    
    network_data = load_json(CONFIG_FILE)
    pids = load_json(PID_FILE)
    
    if not network_data:
        log("网络未配置", "WARN")
        return 0
    
    nodes = network_data.get("nodes", [])
    started_at = network_data.get("started_at", "未知")
    
    print(f"\n启动时间: {started_at}")
    print(f"节点数量: {len(nodes)}")
    print()
    
    # 表头
    print(f"{'节点ID':<20} {'P2P端口':<10} {'HTTP端口':<10} {'PID':<10} {'状态':<10}")
    print("-" * 60)
    
    for node in nodes:
        node_id = node['node_id']
        p2p_port = node['p2p_port']
        http_port = node['http_port']
        pid = pids.get(node_id, -1)
        
        # 检查进程状态
        if pid > 0:
            try:
                if platform.system() == 'Windows':
                    result = subprocess.run(
                        ['tasklist', '/FI', f'PID eq {pid}'],
                        capture_output=True, text=True
                    )
                    is_running = str(pid) in result.stdout
                else:
                    os.kill(pid, 0)
                    is_running = True
            except (ProcessLookupError, PermissionError):
                is_running = False
            
            status = "\033[32m运行中\033[0m" if is_running else "\033[31m已停止\033[0m"
        else:
            status = "\033[33m模拟\033[0m"
        
        print(f"{node_id:<20} {p2p_port:<10} {http_port:<10} {pid:<10} {status}")
    
    print()
    return 0

def cmd_clear(args):
    """清理环境"""
    log("清理 AgentNetwork 测试环境...")
    
    # 先停止所有节点
    cmd_stop(args)
    
    if args.all:
        # 完全清理
        if DATA_DIR.exists():
            log(f"删除数据目录: {DATA_DIR}")
            shutil.rmtree(DATA_DIR)
        log("环境已完全清理")
    else:
        # 只清理数据，保留配置
        for item in DATA_DIR.iterdir():
            if item.is_dir() and item.name.startswith("node-"):
                data_path = item / "data"
                if data_path.exists():
                    log(f"清理节点数据: {item.name}")
                    shutil.rmtree(data_path)
        log("节点数据已清理，配置保留")
    
    return 0

def cmd_test(args):
    """运行测试"""
    log("=" * 50)
    log("运行 AgentNetwork 测试")
    log("=" * 50)
    
    test_results = []
    
    # 1. 单元测试
    if args.unit or args.all:
        log("\n[1/3] 运行单元测试...")
        code, stdout, stderr = run_command([
            "go", "test", "./...", "-v", "-count=1"
        ])
        test_results.append(("单元测试", code == 0))
        
        if code == 0:
            # 统计结果
            lines = stdout.split('\n')
            passed = sum(1 for l in lines if 'PASS' in l and 'ok' in l)
            log(f"单元测试通过: {passed} 个包")
        else:
            log(f"单元测试失败", "ERROR")
            if args.verbose:
                print(stderr)
    
    # 2. 集成测试
    if args.integration or args.all:
        log("\n[2/3] 运行集成测试...")
        code, stdout, stderr = run_command([
            "go", "test", "./test/integration/...", "-v", "-count=1", "-timeout=5m"
        ])
        test_results.append(("集成测试", code == 0))
        
        if code != 0:
            log("集成测试失败或不存在", "WARN")
    
    # 3. API测试
    if args.api or args.all:
        log("\n[3/3] 运行API测试...")
        
        # 检查网络是否运行
        network_data = load_json(CONFIG_FILE)
        if not network_data:
            log("API测试需要先启动网络", "WARN")
            test_results.append(("API测试", False))
        else:
            # 运行API测试脚本
            api_test_script = SCRIPT_DIR / "api_test.py"
            if api_test_script.exists():
                code, stdout, stderr = run_command([
                    sys.executable, str(api_test_script)
                ])
                test_results.append(("API测试", code == 0))
            else:
                log("API测试脚本不存在，跳过", "WARN")
                test_results.append(("API测试", None))
    
    # 汇总结果
    log("\n" + "=" * 50)
    log("测试结果汇总")
    log("=" * 50)
    
    all_passed = True
    for name, result in test_results:
        if result is True:
            status = "\033[32m✓ 通过\033[0m"
        elif result is False:
            status = "\033[31m✗ 失败\033[0m"
            all_passed = False
        else:
            status = "\033[33m- 跳过\033[0m"
        print(f"  {name}: {status}")
    
    print()
    return 0 if all_passed else 1

def cmd_logs(args):
    """查看日志"""
    node_id = args.node
    
    if node_id:
        # 查看特定节点日志
        log_file = DATA_DIR / f"node-{node_id}" / "node.log"
        if node_id.startswith("genesis"):
            log_file = DATA_DIR / "node-genesis" / "node.log"
    else:
        # 查看创世节点日志
        log_file = DATA_DIR / "node-genesis" / "node.log"
    
    if not log_file.exists():
        log(f"日志文件不存在: {log_file}", "ERROR")
        return 1
    
    # 显示最后N行
    lines = args.lines or 50
    
    with open(log_file, 'r', encoding='utf-8', errors='ignore') as f:
        all_lines = f.readlines()
        for line in all_lines[-lines:]:
            print(line.rstrip())
    
    return 0

def cmd_exec(args):
    """在节点上执行命令"""
    import urllib.request
    import urllib.error
    
    node_id = args.node or "genesis"
    
    # 获取节点HTTP端口
    network_data = load_json(CONFIG_FILE)
    if not network_data:
        log("网络未配置", "ERROR")
        return 1
    
    nodes = network_data.get("nodes", [])
    target_node = None
    for node in nodes:
        if node_id in node['node_id']:
            target_node = node
            break
    
    if not target_node:
        log(f"节点 {node_id} 不存在", "ERROR")
        return 1
    
    http_port = target_node['http_port']
    
    # 执行API调用
    endpoint = args.endpoint or "/api/v1/node/info"
    url = f"http://127.0.0.1:{http_port}{endpoint}"
    
    log(f"调用: {url}")
    
    try:
        req = urllib.request.Request(url)
        if args.method:
            req.method = args.method.upper()
        if args.data:
            req.data = args.data.encode('utf-8')
            req.add_header('Content-Type', 'application/json')
        
        with urllib.request.urlopen(req, timeout=10) as response:
            result = response.read().decode('utf-8')
            try:
                data = json.loads(result)
                print(json.dumps(data, indent=2, ensure_ascii=False))
            except json.JSONDecodeError:
                print(result)
    except urllib.error.URLError as e:
        log(f"请求失败: {e}", "ERROR")
        return 1
    
    return 0

# ============ 主程序 ============

def main():
    parser = argparse.ArgumentParser(
        description="AgentNetwork 网络管理工具",
        formatter_class=argparse.RawDescriptionHelpFormatter,
        epilog="""
示例:
  python network_manager.py start                    # 启动5节点网络
  python network_manager.py start -n 10              # 启动10节点网络
  python network_manager.py stop                     # 停止网络
  python network_manager.py restart                  # 重启网络
  python network_manager.py status                   # 查看状态
  python network_manager.py test --all               # 运行所有测试
  python network_manager.py clear --all              # 完全清理环境
  python network_manager.py logs                     # 查看创世节点日志
  python network_manager.py exec -e /api/v1/peers   # 调用API
        """
    )
    
    subparsers = parser.add_subparsers(dest="command", help="命令")
    
    # start 命令
    start_parser = subparsers.add_parser("start", help="启动网络")
    start_parser.add_argument("-n", "--nodes", type=int, default=5, help="节点数量 (默认: 5)")
    start_parser.add_argument("--name", default="testnet", help="网络名称")
    start_parser.add_argument("--p2p-port", type=int, default=9000, help="起始P2P端口")
    start_parser.add_argument("--http-port", type=int, default=18000, help="起始HTTP端口")
    start_parser.add_argument("--rebuild", action="store_true", help="重新编译")
    
    # stop 命令
    stop_parser = subparsers.add_parser("stop", help="停止网络")
    
    # restart 命令
    restart_parser = subparsers.add_parser("restart", help="重启网络")
    
    # status 命令
    status_parser = subparsers.add_parser("status", help="查看状态")
    
    # clear 命令
    clear_parser = subparsers.add_parser("clear", help="清理环境")
    clear_parser.add_argument("--all", action="store_true", help="完全清理（包括配置）")
    
    # test 命令
    test_parser = subparsers.add_parser("test", help="运行测试")
    test_parser.add_argument("--all", action="store_true", help="运行所有测试")
    test_parser.add_argument("--unit", action="store_true", help="运行单元测试")
    test_parser.add_argument("--integration", action="store_true", help="运行集成测试")
    test_parser.add_argument("--api", action="store_true", help="运行API测试")
    test_parser.add_argument("-v", "--verbose", action="store_true", help="详细输出")
    
    # logs 命令
    logs_parser = subparsers.add_parser("logs", help="查看日志")
    logs_parser.add_argument("--node", help="节点ID")
    logs_parser.add_argument("-n", "--lines", type=int, default=50, help="显示行数")
    
    # exec 命令
    exec_parser = subparsers.add_parser("exec", help="执行API调用")
    exec_parser.add_argument("--node", help="目标节点")
    exec_parser.add_argument("-e", "--endpoint", help="API端点")
    exec_parser.add_argument("-m", "--method", default="GET", help="HTTP方法")
    exec_parser.add_argument("-d", "--data", help="请求数据(JSON)")
    
    args = parser.parse_args()
    
    if not args.command:
        parser.print_help()
        return 0
    
    # 命令映射
    commands = {
        "start": cmd_start,
        "stop": cmd_stop,
        "restart": cmd_restart,
        "status": cmd_status,
        "clear": cmd_clear,
        "test": cmd_test,
        "logs": cmd_logs,
        "exec": cmd_exec,
    }
    
    handler = commands.get(args.command)
    if handler:
        return handler(args)
    else:
        parser.print_help()
        return 1

if __name__ == "__main__":
    try:
        sys.exit(main())
    except KeyboardInterrupt:
        log("\n操作已取消", "WARN")
        sys.exit(130)
