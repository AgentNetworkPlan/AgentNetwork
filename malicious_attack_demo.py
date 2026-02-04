#!/usr/bin/env python3
"""
恶意节点攻击演示脚本
演示各种攻击场景和网络安全防护
"""

import time
import json
import requests
import threading
import random
from datetime import datetime
from concurrent.futures import ThreadPoolExecutor

# 测试配置
NODE_URL = "http://127.0.0.1:18345"
API_TOKEN = None  # 将在运行时获取

def log(msg, level="INFO"):
    """彩色日志输出"""
    colors = {
        "INFO": "\033[32m",     # 绿色
        "WARN": "\033[33m",     # 黄色  
        "ERROR": "\033[31m",    # 红色
        "ATTACK": "\033[35m\033[1m",  # 紫色粗体
        "SUCCESS": "\033[92m",  # 亮绿色
    }
    reset = "\033[0m"
    timestamp = datetime.now().strftime("%H:%M:%S")
    color = colors.get(level, "")
    print(f"{color}[{timestamp}] [{level}] {msg}{reset}")

class MaliciousAttacks:
    def __init__(self, node_url, api_token=None):
        self.node_url = node_url
        self.api_token = api_token
        self.headers = {"X-API-Token": api_token} if api_token else {}
        self.results = []
        
    def test_basic_connectivity(self):
        """测试基本连接性"""
        log("测试基本连接性...", "INFO")
        try:
            # 测试健康检查
            response = requests.get(f"{self.node_url}/health", timeout=5)
            log(f"健康检查: {response.status_code}", "SUCCESS" if response.status_code == 200 else "ERROR")
            
            # 测试节点信息
            if self.api_token:
                response = requests.get(f"{self.node_url}/api/v1/node/info", headers=self.headers, timeout=5)
                log(f"节点信息: {response.status_code}", "SUCCESS" if response.status_code == 200 else "ERROR")
                if response.status_code == 200:
                    info = response.json()
                    log(f"节点ID: {info.get('peer_id', 'unknown')}", "INFO")
            
            return True
        except Exception as e:
            log(f"基本连接测试失败: {e}", "ERROR")
            return False
    
    def attack_1_ddos_simulation(self):
        """攻击1: DDoS洪水攻击模拟"""
        log("🔥 执行攻击 1: DDoS洪水攻击", "ATTACK")
        
        attack_result = {
            "attack_type": "DDoS洪水攻击",
            "start_time": datetime.now().isoformat(),
            "requests_sent": 0,
            "successful_requests": 0,
            "failed_requests": 0,
            "response_times": []
        }
        
        def send_flood_request():
            try:
                start_time = time.time()
                response = requests.get(f"{self.node_url}/health", timeout=2)
                response_time = time.time() - start_time
                
                attack_result["requests_sent"] += 1
                if response.status_code == 200:
                    attack_result["successful_requests"] += 1
                    attack_result["response_times"].append(response_time)
                else:
                    attack_result["failed_requests"] += 1
                    
            except Exception as e:
                attack_result["failed_requests"] += 1
        
        # 发送100个并发请求
        with ThreadPoolExecutor(max_workers=20) as executor:
            futures = [executor.submit(send_flood_request) for _ in range(100)]
            for future in futures:
                try:
                    future.result(timeout=1)
                except:
                    attack_result["failed_requests"] += 1
        
        attack_result["end_time"] = datetime.now().isoformat()
        attack_result["avg_response_time"] = sum(attack_result["response_times"]) / len(attack_result["response_times"]) if attack_result["response_times"] else 0
        
        self.results.append(attack_result)
        
        log(f"DDoS攻击完成: {attack_result['successful_requests']}/{attack_result['requests_sent']} 成功", "ATTACK")
        log(f"平均响应时间: {attack_result['avg_response_time']:.3f}s", "INFO")
        
        # 分析结果
        if attack_result["failed_requests"] > attack_result["successful_requests"] * 0.5:
            log("✅ 网络具有良好的DDoS防护能力", "SUCCESS")
        else:
            log("⚠️  网络可能容易受到DDoS攻击", "WARN")
    
    def attack_2_malformed_data(self):
        """攻击2: 恶意数据注入"""
        log("🔥 执行攻击 2: 恶意数据注入", "ATTACK")
        
        malicious_payloads = [
            # SQL注入尝试
            {"payload": "'; DROP TABLE users; --", "type": "SQL注入"},
            # XSS尝试
            {"payload": "<script>alert('恶意脚本')</script>", "type": "XSS注入"},
            # 超长数据
            {"payload": "A" * 10000, "type": "缓冲区溢出"},
            # 格式错误JSON
            {"payload": "{'malformed': json}", "type": "格式错误JSON"},
            # Unicode攻击
            {"payload": "\u0000\u0001\u0002恶意Unicode", "type": "Unicode攻击"},
        ]
        
        attack_result = {
            "attack_type": "恶意数据注入",
            "start_time": datetime.now().isoformat(),
            "payloads_tested": 0,
            "blocked_attempts": 0,
            "successful_injections": 0,
            "details": []
        }
        
        for payload_info in malicious_payloads:
            try:
                # 尝试多个端点
                endpoints = ["/api/v1/message", "/api/v1/neighbor/add", "/health"]
                
                for endpoint in endpoints:
                    try:
                        if endpoint == "/health":
                            # GET请求注入
                            response = requests.get(f"{self.node_url}{endpoint}?data={payload_info['payload']}", 
                                                  headers=self.headers, timeout=5)
                        else:
                            # POST请求注入
                            response = requests.post(f"{self.node_url}{endpoint}", 
                                                   headers=self.headers,
                                                   json={"data": payload_info['payload']}, 
                                                   timeout=5)
                        
                        attack_result["payloads_tested"] += 1
                        
                        detail = {
                            "payload_type": payload_info['type'],
                            "endpoint": endpoint,
                            "status_code": response.status_code,
                            "blocked": response.status_code in [400, 401, 403, 422, 429]
                        }
                        
                        if detail["blocked"]:
                            attack_result["blocked_attempts"] += 1
                        elif response.status_code == 200:
                            attack_result["successful_injections"] += 1
                            
                        attack_result["details"].append(detail)
                        
                    except requests.exceptions.Timeout:
                        attack_result["blocked_attempts"] += 1  # 超时也算被阻止
                    except Exception as e:
                        pass  # 忽略连接错误
                        
            except Exception as e:
                pass
        
        attack_result["end_time"] = datetime.now().isoformat()
        self.results.append(attack_result)
        
        log(f"数据注入测试完成: {attack_result['blocked_attempts']}/{attack_result['payloads_tested']} 被阻止", "ATTACK")
        
        block_rate = attack_result["blocked_attempts"] / attack_result["payloads_tested"] if attack_result["payloads_tested"] > 0 else 0
        if block_rate > 0.8:
            log("✅ 网络具有强大的输入验证防护", "SUCCESS")
        elif block_rate > 0.5:
            log("⚠️  网络输入验证需要改进", "WARN")
        else:
            log("❌ 网络存在严重的输入验证漏洞", "ERROR")
    
    def attack_3_unauthorized_access(self):
        """攻击3: 未授权访问尝试"""
        log("🔥 执行攻击 3: 未授权访问尝试", "ATTACK")
        
        attack_result = {
            "attack_type": "未授权访问",
            "start_time": datetime.now().isoformat(),
            "endpoints_tested": 0,
            "blocked_access": 0,
            "unauthorized_success": 0,
            "details": []
        }
        
        # 尝试访问受保护的端点
        protected_endpoints = [
            "/api/v1/admin/config",
            "/api/v1/admin/tokens",
            "/api/v1/admin/shutdown", 
            "/api/v1/node/neighbors/add",
            "/api/v1/node/neighbors/remove",
            "/api/v1/message/send"
        ]
        
        # 无token访问
        for endpoint in protected_endpoints:
            try:
                response = requests.get(f"{self.node_url}{endpoint}", timeout=5)
                attack_result["endpoints_tested"] += 1
                
                detail = {
                    "endpoint": endpoint,
                    "method": "无token",
                    "status_code": response.status_code,
                    "blocked": response.status_code in [401, 403]
                }
                
                if detail["blocked"]:
                    attack_result["blocked_access"] += 1
                else:
                    attack_result["unauthorized_success"] += 1
                    
                attack_result["details"].append(detail)
                
            except:
                pass
        
        # 伪造token访问
        fake_tokens = [
            "fake_token_123",
            "admin",
            "root",
            "password",
            "0" * 64,  # 假的长token
        ]
        
        for token in fake_tokens:
            fake_headers = {"X-API-Token": token}
            for endpoint in protected_endpoints[:3]:  # 只测试前3个以节省时间
                try:
                    response = requests.get(f"{self.node_url}{endpoint}", headers=fake_headers, timeout=5)
                    attack_result["endpoints_tested"] += 1
                    
                    detail = {
                        "endpoint": endpoint,
                        "method": f"伪造token",
                        "status_code": response.status_code,
                        "blocked": response.status_code in [401, 403]
                    }
                    
                    if detail["blocked"]:
                        attack_result["blocked_access"] += 1
                    else:
                        attack_result["unauthorized_success"] += 1
                        
                    attack_result["details"].append(detail)
                    
                except:
                    pass
        
        attack_result["end_time"] = datetime.now().isoformat()
        self.results.append(attack_result)
        
        log(f"未授权访问测试: {attack_result['blocked_access']}/{attack_result['endpoints_tested']} 被正确阻止", "ATTACK")
        
        block_rate = attack_result["blocked_access"] / attack_result["endpoints_tested"] if attack_result["endpoints_tested"] > 0 else 0
        if block_rate > 0.9:
            log("✅ 网络具有优秀的访问控制", "SUCCESS")
        elif block_rate > 0.7:
            log("⚠️  网络访问控制需要改进", "WARN")
        else:
            log("❌ 网络存在严重的访问控制漏洞", "ERROR")
    
    def attack_4_resource_exhaustion(self):
        """攻击4: 资源耗尽攻击"""
        log("🔥 执行攻击 4: 资源耗尽攻击", "ATTACK")
        
        attack_result = {
            "attack_type": "资源耗尽攻击",
            "start_time": datetime.now().isoformat(),
            "long_requests": 0,
            "concurrent_connections": 0,
            "memory_pressure_tests": 0
        }
        
        # 长时间连接测试
        def create_long_connection():
            try:
                requests.get(f"{self.node_url}/health", stream=True, timeout=30)
                attack_result["long_requests"] += 1
            except:
                pass
        
        # 创建多个长连接
        with ThreadPoolExecutor(max_workers=10) as executor:
            futures = [executor.submit(create_long_connection) for _ in range(20)]
            for future in futures:
                try:
                    future.result(timeout=5)
                except:
                    pass
        
        attack_result["end_time"] = datetime.now().isoformat()
        self.results.append(attack_result)
        log("资源耗尽攻击完成", "ATTACK")
        
    def generate_security_report(self):
        """生成安全分析报告"""
        log("📊 生成安全分析报告", "INFO")
        
        report = {
            "test_summary": {
                "test_time": datetime.now().isoformat(),
                "total_attacks": len(self.results),
                "node_url": self.node_url
            },
            "attacks": self.results,
            "security_analysis": {},
            "recommendations": []
        }
        
        # 安全分析
        ddos_attacks = [r for r in self.results if r.get("attack_type") == "DDoS洪水攻击"]
        if ddos_attacks:
            ddos = ddos_attacks[0]
            fail_rate = ddos["failed_requests"] / ddos["requests_sent"] if ddos["requests_sent"] > 0 else 0
            report["security_analysis"]["ddos_resistance"] = {
                "fail_rate": fail_rate,
                "rating": "好" if fail_rate > 0.5 else "中" if fail_rate > 0.2 else "差"
            }
        
        injection_attacks = [r for r in self.results if r.get("attack_type") == "恶意数据注入"]
        if injection_attacks:
            injection = injection_attacks[0]
            block_rate = injection["blocked_attempts"] / injection["payloads_tested"] if injection["payloads_tested"] > 0 else 0
            report["security_analysis"]["input_validation"] = {
                "block_rate": block_rate,
                "rating": "好" if block_rate > 0.8 else "中" if block_rate > 0.5 else "差"
            }
        
        auth_attacks = [r for r in self.results if r.get("attack_type") == "未授权访问"]
        if auth_attacks:
            auth = auth_attacks[0]
            block_rate = auth["blocked_access"] / auth["endpoints_tested"] if auth["endpoints_tested"] > 0 else 0
            report["security_analysis"]["access_control"] = {
                "block_rate": block_rate,
                "rating": "好" if block_rate > 0.9 else "中" if block_rate > 0.7 else "差"
            }
        
        # 生成建议
        for category, analysis in report["security_analysis"].items():
            if analysis["rating"] == "差":
                if category == "ddos_resistance":
                    report["recommendations"].append("建议增加请求频率限制和DDoS防护")
                elif category == "input_validation":
                    report["recommendations"].append("建议加强输入验证和数据清理")
                elif category == "access_control":
                    report["recommendations"].append("建议完善身份验证和访问控制机制")
        
        if not report["recommendations"]:
            report["recommendations"].append("网络安全防护表现良好")
        
        # 计算总体安全得分
        ratings = [analysis["rating"] for analysis in report["security_analysis"].values()]
        score_map = {"好": 100, "中": 60, "差": 20}
        if ratings:
            avg_score = sum(score_map[rating] for rating in ratings) / len(ratings)
            report["security_analysis"]["overall_score"] = avg_score
        
        # 保存报告
        timestamp = datetime.now().strftime("%Y%m%d_%H%M%S")
        report_file = f"security_test_report_{timestamp}.json"
        with open(report_file, 'w', encoding='utf-8') as f:
            json.dump(report, f, indent=2, ensure_ascii=False)
        
        log(f"安全报告已保存: {report_file}", "SUCCESS")
        
        # 显示摘要
        log("=== 安全测试摘要 ===", "INFO")
        for category, analysis in report["security_analysis"].items():
            log(f"{category}: {analysis['rating']}", "INFO")
        
        if "overall_score" in report["security_analysis"]:
            log(f"总体安全得分: {report['security_analysis']['overall_score']:.1f}/100", "INFO")
        
        log("=== 安全建议 ===", "INFO")
        for rec in report["recommendations"]:
            log(f"• {rec}", "INFO")
        
        return report

def main():
    log("🚀 开始恶意节点攻击测试", "ATTACK")
    
    # 初始化攻击器
    attacker = MaliciousAttacks(NODE_URL)
    
    # 测试基本连接
    if not attacker.test_basic_connectivity():
        log("❌ 节点连接失败，测试中止", "ERROR")
        return
    
    log("⚡ 开始执行恶意攻击序列...", "ATTACK")
    
    # 执行各种攻击
    try:
        attacker.attack_1_ddos_simulation()
        time.sleep(2)
        
        attacker.attack_2_malformed_data()
        time.sleep(2)
        
        attacker.attack_3_unauthorized_access()
        time.sleep(2)
        
        attacker.attack_4_resource_exhaustion()
        
    except KeyboardInterrupt:
        log("❌ 攻击测试被中断", "WARN")
    
    # 生成报告
    log("📊 分析攻击结果并生成报告...", "INFO")
    attacker.generate_security_report()
    
    log("🏁 恶意节点测试完成！", "SUCCESS")

if __name__ == "__main__":
    main()