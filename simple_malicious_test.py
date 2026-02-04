#!/usr/bin/env python3
"""
简化版恶意节点测试脚本
手动启动节点并执行恶意行为测试
"""

import os
import sys
import json
import time
import random
import requests
import subprocess
from datetime import datetime

# 颜色输出
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

class SimpleNodeTester:
    def __init__(self):
        self.nodes = []
        self.test_results = []
        self.attack_results = []
        
    def test_node_api(self, node_url, api_token=None, is_malicious=False):
        """测试节点API"""
        try:
            headers = {}
            if api_token:
                headers["X-API-Token"] = api_token
            
            # 测试健康检查
            response = requests.get(f"{node_url}/health", headers=headers, timeout=5)
            health_ok = response.status_code == 200
            
            # 测试节点信息
            response = requests.get(f"{node_url}/api/v1/node/info", headers=headers, timeout=5)
            info_ok = response.status_code == 200
            
            if is_malicious:
                # 恶意行为1: 尝试超量请求
                self.perform_flood_attack(node_url, headers)
                # 恶意行为2: 尝试无效数据
                self.perform_invalid_data_attack(node_url, headers)
                # 恶意行为3: 尝试未授权访问
                self.perform_unauthorized_attack(node_url)
            
            return {
                "health": health_ok,
                "info": info_ok,
                "response_time": response.elapsed.total_seconds() if 'response' in locals() else None
            }
            
        except Exception as e:
            log(f"API测试失败: {e}", "ERROR")
            return {"health": False, "info": False, "error": str(e)}
    
    def perform_flood_attack(self, node_url, headers):
        """执行洪水攻击"""
        log("执行洪水攻击 - 发送大量请求", "MALICIOUS")
        attack_result = {"type": "flood", "requests_sent": 0, "failed": 0, "success": 0}
        
        for i in range(50):  # 发送50个快速请求
            try:
                response = requests.get(f"{node_url}/health", headers=headers, timeout=1)
                attack_result["requests_sent"] += 1
                if response.status_code == 200:
                    attack_result["success"] += 1
                else:
                    attack_result["failed"] += 1
            except:
                attack_result["failed"] += 1
            
        self.attack_results.append(attack_result)
        log(f"洪水攻击完成: {attack_result}", "MALICIOUS")
    
    def perform_invalid_data_attack(self, node_url, headers):
        """执行无效数据攻击"""
        log("执行无效数据攻击 - 发送格式错误的数据", "MALICIOUS")
        attack_result = {"type": "invalid_data", "attempts": 0, "responses": []}
        
        invalid_payloads = [
            {"type": "oversized", "data": "A" * 10000},  # 超大数据
            {"type": "malformed_json", "data": "{'invalid': json}"},  # 格式错误JSON
            {"type": "sql_injection", "data": "'; DROP TABLE nodes; --"},  # SQL注入尝试
            {"type": "script_injection", "data": "<script>alert('xss')</script>"},  # XSS尝试
        ]
        
        for payload in invalid_payloads:
            try:
                response = requests.post(f"{node_url}/api/v1/message", 
                                       headers=headers, 
                                       json=payload["data"], 
                                       timeout=5)
                attack_result["attempts"] += 1
                attack_result["responses"].append({
                    "type": payload["type"],
                    "status": response.status_code,
                    "handled": response.status_code in [400, 422, 429]  # 正确的错误响应
                })
            except Exception as e:
                attack_result["responses"].append({
                    "type": payload["type"],
                    "error": str(e)
                })
        
        self.attack_results.append(attack_result)
        log(f"无效数据攻击完成: {len(attack_result['responses'])} 次尝试", "MALICIOUS")
    
    def perform_unauthorized_attack(self, node_url):
        """执行未授权访问攻击"""
        log("执行未授权访问攻击 - 尝试访问受保护的接口", "MALICIOUS")
        attack_result = {"type": "unauthorized", "attempts": 0, "blocked": 0}
        
        protected_endpoints = [
            "/api/v1/admin/config",
            "/api/v1/admin/tokens", 
            "/api/v1/admin/shutdown",
            "/api/v1/node/neighbors/add",
            "/api/v1/node/neighbors/remove"
        ]
        
        for endpoint in protected_endpoints:
            try:
                # 尝试无token访问
                response = requests.get(f"{node_url}{endpoint}", timeout=5)
                attack_result["attempts"] += 1
                if response.status_code in [401, 403]:
                    attack_result["blocked"] += 1
                    
                # 尝试伪造token访问
                fake_headers = {"X-API-Token": "fake_token_12345"}
                response = requests.get(f"{node_url}{endpoint}", headers=fake_headers, timeout=5)
                attack_result["attempts"] += 1
                if response.status_code in [401, 403]:
                    attack_result["blocked"] += 1
                    
            except Exception as e:
                pass
        
        self.attack_results.append(attack_result)
        log(f"未授权攻击完成: {attack_result['blocked']}/{attack_result['attempts']} 被正确阻止", "MALICIOUS")
    
    def analyze_network_resilience(self):
        """分析网络韧性"""
        log("分析网络安全性和韧性...", "STEP")
        
        analysis = {
            "timestamp": datetime.now().isoformat(),
            "total_attacks": len(self.attack_results),
            "attack_types": {},
            "security_score": 0
        }
        
        for attack in self.attack_results:
            attack_type = attack["type"]
            if attack_type not in analysis["attack_types"]:
                analysis["attack_types"][attack_type] = []
            analysis["attack_types"][attack_type].append(attack)
        
        # 计算安全得分
        total_score = 0
        max_score = 0
        
        # 洪水攻击防护评分
        if "flood" in analysis["attack_types"]:
            flood_attacks = analysis["attack_types"]["flood"]
            for attack in flood_attacks:
                max_score += 100
                # 如果失败率高，说明有防护机制
                fail_rate = attack["failed"] / attack["requests_sent"] if attack["requests_sent"] > 0 else 0
                if fail_rate > 0.5:  # 50%以上请求失败
                    total_score += 80
                elif fail_rate > 0.2:  # 20%以上请求失败
                    total_score += 60
                else:
                    total_score += 20
        
        # 无效数据防护评分
        if "invalid_data" in analysis["attack_types"]:
            invalid_attacks = analysis["attack_types"]["invalid_data"]
            for attack in invalid_attacks:
                max_score += 100
                handled_count = sum(1 for resp in attack["responses"] if resp.get("handled", False))
                handle_rate = handled_count / len(attack["responses"]) if attack["responses"] else 0
                total_score += int(handle_rate * 100)
        
        # 未授权访问防护评分
        if "unauthorized" in analysis["attack_types"]:
            unauth_attacks = analysis["attack_types"]["unauthorized"]
            for attack in unauth_attacks:
                max_score += 100
                block_rate = attack["blocked"] / attack["attempts"] if attack["attempts"] > 0 else 0
                total_score += int(block_rate * 100)
        
        analysis["security_score"] = (total_score / max_score * 100) if max_score > 0 else 0
        
        return analysis
    
    def generate_report(self):
        """生成测试报告"""
        log("生成恶意节点测试报告...", "STEP")
        
        analysis = self.analyze_network_resilience()
        
        report = {
            "test_summary": {
                "test_time": datetime.now().isoformat(),
                "total_nodes": len(self.nodes),
                "total_attacks": len(self.attack_results),
                "security_score": analysis["security_score"]
            },
            "attack_analysis": analysis,
            "detailed_results": self.attack_results,
            "recommendations": self.generate_recommendations(analysis)
        }
        
        # 保存报告
        report_file = f"malicious_test_report_{datetime.now().strftime('%Y%m%d_%H%M%S')}.json"
        with open(report_file, 'w', encoding='utf-8') as f:
            json.dump(report, f, indent=2, ensure_ascii=False)
        
        log(f"测试报告已保存: {report_file}", "INFO")
        return report
    
    def generate_recommendations(self, analysis):
        """生成安全建议"""
        recommendations = []
        
        if analysis["security_score"] < 70:
            recommendations.append("网络安全防护需要加强")
        
        if "flood" in analysis["attack_types"]:
            recommendations.append("建议增加请求频率限制和DDoS防护")
        
        if "invalid_data" in analysis["attack_types"]:
            recommendations.append("建议增强输入验证和数据清理")
        
        if "unauthorized" in analysis["attack_types"]:
            recommendations.append("建议加强身份验证和访问控制")
        
        if analysis["security_score"] > 80:
            recommendations.append("网络安全防护表现良好")
        
        return recommendations

def main():
    log("开始简化版恶意节点测试", "STEP")
    
    tester = SimpleNodeTester()
    
    # 测试配置
    test_nodes = [
        {"url": "http://localhost:9001", "name": "node01", "malicious": False},
        {"url": "http://localhost:9011", "name": "node02", "malicious": True},  # 恶意节点
        {"url": "http://localhost:9021", "name": "node03", "malicious": False},
    ]
    
    log("提示: 请确保已经手动启动了测试节点", "WARN")
    log("节点端口: 9001, 9011, 9021", "INFO")
    
    input("按 Enter 键开始测试...")
    
    # 测试每个节点
    for node_config in test_nodes:
        log(f"测试节点 {node_config['name']} ({'恶意' if node_config['malicious'] else '正常'})", "STEP")
        
        # 假设的API token（实际需要从节点获取）
        api_token = "test_token_123"  
        
        result = tester.test_node_api(
            node_config["url"], 
            api_token, 
            node_config["malicious"]
        )
        
        log(f"节点 {node_config['name']} 测试完成: {result}", "INFO")
        tester.nodes.append({
            "name": node_config["name"],
            "url": node_config["url"],
            "malicious": node_config["malicious"],
            "test_result": result
        })
        
        time.sleep(2)  # 间隔时间
    
    # 生成报告
    report = tester.generate_report()
    
    # 显示摘要
    log("测试摘要:", "STEP")
    log(f"总节点数: {report['test_summary']['total_nodes']}", "INFO")
    log(f"总攻击次数: {report['test_summary']['total_attacks']}", "INFO")
    log(f"安全得分: {report['test_summary']['security_score']:.1f}/100", "INFO")
    
    log("安全建议:", "STEP")
    for rec in report["recommendations"]:
        log(f"- {rec}", "INFO")
    
    log("恶意节点测试完成!", "STEP")

if __name__ == "__main__":
    main()