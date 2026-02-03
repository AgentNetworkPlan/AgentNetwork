#!/usr/bin/env python3
"""
AgentNetwork API 测试脚本
测试HTTP API的各种端点

使用方法:
    python api_test.py                    # 测试默认端口
    python api_test.py --port 18000       # 指定端口
    python api_test.py --all              # 运行所有测试
"""

import json
import time
import argparse
import urllib.request
import urllib.error
from dataclasses import dataclass
from typing import Optional, Dict, Any, List
from datetime import datetime

# ============ 配置 ============

DEFAULT_HOST = "127.0.0.1"
DEFAULT_PORT = 18000
TIMEOUT = 10

# ============ 测试框架 ============

@dataclass
class TestResult:
    name: str
    passed: bool
    message: str
    duration: float
    response: Optional[dict] = None

class APIClient:
    """API客户端"""
    
    def __init__(self, host: str, port: int):
        self.base_url = f"http://{host}:{port}"
        self.headers = {
            "Content-Type": "application/json",
            "X-NodeID": "test-client",
            "X-Timestamp": str(int(time.time()))
        }
    
    def request(self, method: str, endpoint: str, data: dict = None) -> tuple:
        """发送HTTP请求"""
        url = f"{self.base_url}{endpoint}"
        
        try:
            if data:
                body = json.dumps(data).encode('utf-8')
            else:
                body = None
            
            req = urllib.request.Request(url, data=body, headers=self.headers, method=method)
            
            with urllib.request.urlopen(req, timeout=TIMEOUT) as resp:
                status = resp.status
                response = json.loads(resp.read().decode('utf-8'))
                return status, response, None
                
        except urllib.error.HTTPError as e:
            try:
                response = json.loads(e.read().decode('utf-8'))
            except:
                response = {"error": str(e)}
            return e.code, response, str(e)
            
        except urllib.error.URLError as e:
            return 0, None, f"Connection failed: {e}"
            
        except Exception as e:
            return 0, None, str(e)
    
    def get(self, endpoint: str) -> tuple:
        return self.request("GET", endpoint)
    
    def post(self, endpoint: str, data: dict = None) -> tuple:
        return self.request("POST", endpoint, data)
    
    def delete(self, endpoint: str) -> tuple:
        return self.request("DELETE", endpoint)

class TestRunner:
    """测试运行器"""
    
    def __init__(self, client: APIClient):
        self.client = client
        self.results: List[TestResult] = []
    
    def run_test(self, name: str, test_func) -> TestResult:
        """运行单个测试"""
        start = time.time()
        try:
            passed, message, response = test_func()
            result = TestResult(
                name=name,
                passed=passed,
                message=message,
                duration=time.time() - start,
                response=response
            )
        except Exception as e:
            result = TestResult(
                name=name,
                passed=False,
                message=f"Exception: {str(e)}",
                duration=time.time() - start
            )
        
        self.results.append(result)
        
        # 打印结果
        status = "✓" if result.passed else "✗"
        color = "\033[32m" if result.passed else "\033[31m"
        reset = "\033[0m"
        print(f"  {color}{status}{reset} {name} ({result.duration:.3f}s)")
        if not result.passed:
            print(f"    └─ {result.message}")
        
        return result
    
    def summary(self) -> tuple:
        """汇总测试结果"""
        passed = sum(1 for r in self.results if r.passed)
        total = len(self.results)
        return passed, total

# ============ 测试用例 ============

def test_health_check(client: APIClient):
    """测试健康检查"""
    status, resp, err = client.get("/api/v1/health")
    if err:
        return False, err, None
    if status != 200:
        return False, f"Status: {status}", resp
    return True, "OK", resp

def test_node_info(client: APIClient):
    """测试节点信息"""
    status, resp, err = client.get("/api/v1/node/info")
    if err:
        return False, err, None
    if status != 200:
        return False, f"Status: {status}", resp
    if "node_id" not in resp.get("data", {}):
        return False, "Missing node_id in response", resp
    return True, "OK", resp

def test_peers_list(client: APIClient):
    """测试对等节点列表"""
    status, resp, err = client.get("/api/v1/node/peers")
    if err:
        return False, err, None
    if status != 200:
        return False, f"Status: {status}", resp
    return True, "OK", resp

def test_neighbor_list(client: APIClient):
    """测试邻居列表"""
    status, resp, err = client.get("/api/v1/neighbor/list")
    if err:
        return False, err, None
    if status != 200:
        return False, f"Status: {status}", resp
    return True, "OK", resp

def test_neighbor_best(client: APIClient):
    """测试最佳邻居"""
    status, resp, err = client.get("/api/v1/neighbor/best?limit=5")
    if err:
        return False, err, None
    if status != 200:
        return False, f"Status: {status}", resp
    return True, "OK", resp

def test_mailbox_inbox(client: APIClient):
    """测试收件箱"""
    status, resp, err = client.get("/api/v1/mailbox/inbox")
    if err:
        return False, err, None
    if status != 200:
        return False, f"Status: {status}", resp
    return True, "OK", resp

def test_mailbox_send(client: APIClient):
    """测试发送邮件"""
    data = {
        "to": "test-recipient",
        "subject": "Test Message",
        "content": "This is a test message from API test"
    }
    status, resp, err = client.post("/api/v1/mailbox/send", data)
    if err:
        return False, err, None
    # 允许200或201
    if status not in [200, 201]:
        return False, f"Status: {status}", resp
    return True, "OK", resp

def test_bulletin_list(client: APIClient):
    """测试公告列表"""
    status, resp, err = client.get("/api/v1/bulletin/list?limit=10")
    if err:
        return False, err, None
    if status != 200:
        return False, f"Status: {status}", resp
    return True, "OK", resp

def test_bulletin_publish(client: APIClient):
    """测试发布公告"""
    data = {
        "topic": "test",
        "title": "Test Bulletin",
        "content": "This is a test bulletin from API test",
        "tags": ["test", "api"]
    }
    status, resp, err = client.post("/api/v1/bulletin/publish", data)
    if err:
        return False, err, None
    if status not in [200, 201]:
        return False, f"Status: {status}", resp
    return True, "OK", resp

def test_reputation_query(client: APIClient):
    """测试信誉查询"""
    status, resp, err = client.get("/api/v1/reputation/query?node_id=test-node")
    if err:
        return False, err, None
    if status not in [200, 404]:  # 404也是有效响应（节点不存在）
        return False, f"Status: {status}", resp
    return True, "OK", resp

def test_reputation_ranking(client: APIClient):
    """测试信誉排名"""
    status, resp, err = client.get("/api/v1/reputation/ranking?limit=10")
    if err:
        return False, err, None
    if status != 200:
        return False, f"Status: {status}", resp
    return True, "OK", resp

def test_voting_list(client: APIClient):
    """测试投票列表"""
    status, resp, err = client.get("/api/v1/voting/list")
    if err:
        return False, err, None
    if status != 200:
        return False, f"Status: {status}", resp
    return True, "OK", resp

def test_supernode_list(client: APIClient):
    """测试超级节点列表"""
    status, resp, err = client.get("/api/v1/supernode/list")
    if err:
        return False, err, None
    if status != 200:
        return False, f"Status: {status}", resp
    return True, "OK", resp

def test_genesis_info(client: APIClient):
    """测试创世信息"""
    status, resp, err = client.get("/api/v1/genesis/info")
    if err:
        return False, err, None
    if status != 200:
        return False, f"Status: {status}", resp
    return True, "OK", resp

def test_incentive_history(client: APIClient):
    """测试激励历史"""
    status, resp, err = client.get("/api/v1/incentive/history?node_id=test-node")
    if err:
        return False, err, None
    if status not in [200, 404]:
        return False, f"Status: {status}", resp
    return True, "OK", resp

def test_log_query(client: APIClient):
    """测试日志查询"""
    status, resp, err = client.get("/api/v1/log/query?level=info&limit=10")
    if err:
        return False, err, None
    if status != 200:
        return False, f"Status: {status}", resp
    return True, "OK", resp

def test_invalid_endpoint(client: APIClient):
    """测试无效端点"""
    status, resp, err = client.get("/api/v1/invalid/endpoint")
    if err and "Connection failed" in err:
        return False, err, None
    # 期望404
    if status == 404:
        return True, "Correctly returns 404", resp
    return False, f"Expected 404, got {status}", resp

def test_message_send(client: APIClient):
    """测试发送消息"""
    data = {
        "to": "test-peer",
        "type": "test",
        "payload": {"message": "hello"}
    }
    status, resp, err = client.post("/api/v1/message/send", data)
    if err:
        return False, err, None
    # 可能返回各种状态（peer不存在等）
    if status in [200, 201, 404, 400]:
        return True, f"Status: {status}", resp
    return False, f"Unexpected status: {status}", resp

def test_accusation_list(client: APIClient):
    """测试指控列表"""
    status, resp, err = client.get("/api/v1/accusation/list")
    if err:
        return False, err, None
    if status != 200:
        return False, f"Status: {status}", resp
    return True, "OK", resp

# ============ 测试套件 ============

BASIC_TESTS = [
    ("健康检查", test_health_check),
    ("节点信息", test_node_info),
    ("对等节点列表", test_peers_list),
]

NEIGHBOR_TESTS = [
    ("邻居列表", test_neighbor_list),
    ("最佳邻居", test_neighbor_best),
]

MAILBOX_TESTS = [
    ("收件箱", test_mailbox_inbox),
    ("发送邮件", test_mailbox_send),
]

BULLETIN_TESTS = [
    ("公告列表", test_bulletin_list),
    ("发布公告", test_bulletin_publish),
]

REPUTATION_TESTS = [
    ("信誉查询", test_reputation_query),
    ("信誉排名", test_reputation_ranking),
]

VOTING_TESTS = [
    ("投票列表", test_voting_list),
]

SUPERNODE_TESTS = [
    ("超级节点列表", test_supernode_list),
]

GENESIS_TESTS = [
    ("创世信息", test_genesis_info),
]

INCENTIVE_TESTS = [
    ("激励历史", test_incentive_history),
]

LOG_TESTS = [
    ("日志查询", test_log_query),
]

MESSAGE_TESTS = [
    ("发送消息", test_message_send),
]

ACCUSATION_TESTS = [
    ("指控列表", test_accusation_list),
]

ERROR_TESTS = [
    ("无效端点(404)", test_invalid_endpoint),
]

ALL_TEST_SUITES = [
    ("基础API", BASIC_TESTS),
    ("邻居管理", NEIGHBOR_TESTS),
    ("邮箱功能", MAILBOX_TESTS),
    ("公告板", BULLETIN_TESTS),
    ("信誉系统", REPUTATION_TESTS),
    ("投票系统", VOTING_TESTS),
    ("超级节点", SUPERNODE_TESTS),
    ("创世节点", GENESIS_TESTS),
    ("激励机制", INCENTIVE_TESTS),
    ("日志管理", LOG_TESTS),
    ("消息传递", MESSAGE_TESTS),
    ("指控系统", ACCUSATION_TESTS),
    ("错误处理", ERROR_TESTS),
]

# ============ 主程序 ============

def main():
    parser = argparse.ArgumentParser(description="AgentNetwork API 测试")
    parser.add_argument("--host", default=DEFAULT_HOST, help="API主机")
    parser.add_argument("--port", type=int, default=DEFAULT_PORT, help="API端口")
    parser.add_argument("--all", action="store_true", help="运行所有测试")
    parser.add_argument("--suite", help="运行特定测试套件")
    parser.add_argument("--list", action="store_true", help="列出测试套件")
    args = parser.parse_args()
    
    if args.list:
        print("可用的测试套件:")
        for name, tests in ALL_TEST_SUITES:
            print(f"  - {name} ({len(tests)} 个测试)")
        return 0
    
    # 创建客户端
    client = APIClient(args.host, args.port)
    runner = TestRunner(client)
    
    print("=" * 60)
    print(f"AgentNetwork API 测试")
    print(f"目标: {client.base_url}")
    print(f"时间: {datetime.now().strftime('%Y-%m-%d %H:%M:%S')}")
    print("=" * 60)
    
    # 检查连接
    print("\n检查连接...")
    status, resp, err = client.get("/api/v1/health")
    if err and "Connection failed" in str(err):
        print(f"\033[31m无法连接到 {client.base_url}\033[0m")
        print("请确保节点已启动并监听正确的端口")
        return 1
    
    print(f"\033[32m连接成功\033[0m\n")
    
    # 选择测试套件
    if args.suite:
        suites = [(name, tests) for name, tests in ALL_TEST_SUITES 
                  if args.suite.lower() in name.lower()]
        if not suites:
            print(f"未找到测试套件: {args.suite}")
            return 1
    else:
        suites = ALL_TEST_SUITES
    
    # 运行测试
    for suite_name, tests in suites:
        print(f"\n[{suite_name}]")
        for test_name, test_func in tests:
            runner.run_test(test_name, lambda f=test_func: f(client))
    
    # 汇总
    passed, total = runner.summary()
    
    print("\n" + "=" * 60)
    print(f"测试完成: {passed}/{total} 通过")
    
    if passed == total:
        print("\033[32m所有测试通过!\033[0m")
        return 0
    else:
        print(f"\033[31m{total - passed} 个测试失败\033[0m")
        return 1

if __name__ == "__main__":
    exit(main())
