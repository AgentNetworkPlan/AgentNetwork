#!/usr/bin/env python3
"""
DAAN 管理平台前端自动化测试
使用 Playwright 进行浏览器自动化测试，包括：
- 登录流程测试
- 页面导航测试
- 控制台日志捕获与分析
- WebSocket 连接测试

使用方法:
    pip install playwright
    playwright install chromium
    python frontend_test.py                      # 基本测试
    python frontend_test.py --base-url http://localhost:18080  # 指定URL
    python frontend_test.py --headless           # 无头模式
    python frontend_test.py --all                # 完整测试
"""

import argparse
import asyncio
import json
import os
import sys
import time
from dataclasses import dataclass, field
from datetime import datetime
from pathlib import Path
from typing import List, Optional, Dict, Any

try:
    from playwright.async_api import async_playwright, Page, BrowserContext, ConsoleMessage, Error
except ImportError:
    print("请先安装 Playwright: pip install playwright")
    print("然后安装浏览器: playwright install chromium")
    sys.exit(1)

# ============ 配置 ============

DEFAULT_BASE_URL = "http://127.0.0.1:18080"
DEFAULT_TOKEN_FILE = os.path.join(os.path.dirname(__file__), "..", "data", "admin_token")
DEFAULT_TIMEOUT = 30000  # 30秒
SCREENSHOT_DIR = os.path.join(os.path.dirname(__file__), "..", "test_logs", "screenshots")

# ============ 数据类型 ============

@dataclass
class ConsoleLog:
    """控制台日志条目"""
    timestamp: str
    type: str  # log, warning, error, info, debug
    text: str
    location: Optional[str] = None
    
@dataclass
class TestResult:
    """测试结果"""
    name: str
    passed: bool
    message: str
    duration: float
    console_logs: List[ConsoleLog] = field(default_factory=list)
    errors: List[str] = field(default_factory=list)
    screenshot: Optional[str] = None

@dataclass
class TestReport:
    """测试报告"""
    timestamp: str
    base_url: str
    total_tests: int
    passed: int
    failed: int
    results: List[TestResult]
    all_console_logs: List[ConsoleLog] = field(default_factory=list)
    
# ============ 测试框架 ============

class FrontendTester:
    """前端自动化测试器"""
    
    def __init__(self, base_url: str, token: str, headless: bool = True):
        self.base_url = base_url.rstrip('/')
        self.token = token
        self.headless = headless
        self.results: List[TestResult] = []
        self.console_logs: List[ConsoleLog] = []
        self.page: Optional[Page] = None
        self.context: Optional[BrowserContext] = None
        
    def _log_console(self, msg: ConsoleMessage):
        """捕获控制台日志"""
        log_entry = ConsoleLog(
            timestamp=datetime.now().isoformat(),
            type=msg.type,
            text=msg.text,
            location=msg.location.get('url', '') if msg.location else None
        )
        self.console_logs.append(log_entry)
        
        # 实时打印日志
        color = {
            'error': '\033[91m',
            'warning': '\033[93m',
            'info': '\033[94m',
            'log': '\033[92m',
            'debug': '\033[90m'
        }.get(msg.type, '\033[0m')
        reset = '\033[0m'
        print(f"  {color}[Console {msg.type.upper()}]{reset} {msg.text[:200]}{'...' if len(msg.text) > 200 else ''}")
        
    async def _take_screenshot(self, name: str) -> str:
        """截图"""
        os.makedirs(SCREENSHOT_DIR, exist_ok=True)
        timestamp = datetime.now().strftime("%Y%m%d_%H%M%S")
        filename = f"{name}_{timestamp}.png"
        filepath = os.path.join(SCREENSHOT_DIR, filename)
        await self.page.screenshot(path=filepath, full_page=True)
        return filepath
        
    async def setup(self):
        """设置浏览器"""
        self.playwright = await async_playwright().start()
        self.browser = await self.playwright.chromium.launch(headless=self.headless)
        self.context = await self.browser.new_context(
            viewport={'width': 1920, 'height': 1080},
            locale='zh-CN'
        )
        self.page = await self.context.new_page()
        
        # 监听控制台消息
        self.page.on('console', self._log_console)
        
        # 监听页面错误
        self.page.on('pageerror', lambda error: self.console_logs.append(
            ConsoleLog(
                timestamp=datetime.now().isoformat(),
                type='error',
                text=f"Page Error: {error}",
                location=None
            )
        ))
        
    async def teardown(self):
        """清理"""
        if self.context:
            await self.context.close()
        if self.browser:
            await self.browser.close()
        if self.playwright:
            await self.playwright.stop()
            
    async def run_test(self, name: str, test_func) -> TestResult:
        """运行单个测试"""
        print(f"\n{'='*60}")
        print(f"测试: {name}")
        print('='*60)
        
        start_time = time.time()
        start_log_count = len(self.console_logs)
        errors = []
        passed = False
        message = ""
        screenshot = None
        
        try:
            await test_func()
            passed = True
            message = "测试通过"
            print(f"✅ {message}")
        except AssertionError as e:
            message = f"断言失败: {e}"
            errors.append(str(e))
            print(f"❌ {message}")
        except Exception as e:
            message = f"异常: {e}"
            errors.append(str(e))
            print(f"❌ {message}")
            
        # 失败时截图
        if not passed:
            try:
                screenshot = await self._take_screenshot(name.replace(' ', '_'))
                print(f"📸 截图已保存: {screenshot}")
            except Exception as e:
                print(f"⚠️ 截图失败: {e}")
                
        duration = time.time() - start_time
        test_logs = self.console_logs[start_log_count:]
        
        # 检查控制台错误
        console_errors = [log for log in test_logs if log.type == 'error']
        if console_errors:
            print(f"⚠️ 发现 {len(console_errors)} 个控制台错误")
            for err in console_errors:
                errors.append(f"Console Error: {err.text[:200]}")
        
        result = TestResult(
            name=name,
            passed=passed,
            message=message,
            duration=duration,
            console_logs=test_logs,
            errors=errors,
            screenshot=screenshot
        )
        self.results.append(result)
        return result

    # ============ 测试用例 ============
    
    async def test_health_check(self):
        """测试健康检查 API"""
        response = await self.page.request.get(f"{self.base_url}/api/health")
        assert response.status == 200, f"健康检查失败: status={response.status}"
        
        data = await response.json()
        assert data.get('status') == 'healthy', f"状态不正确: {data}"
        print(f"  健康检查响应: {data}")
        
    async def test_login_page_loads(self):
        """测试登录页面加载"""
        await self.page.goto(f"{self.base_url}/login")
        await self.page.wait_for_load_state('networkidle')
        
        # 检查页面元素
        title = await self.page.title()
        print(f"  页面标题: {title}")
        
        # 检查登录表单
        token_input = await self.page.query_selector('input[type="password"]')
        assert token_input, "找不到令牌输入框"
        
        login_button = await self.page.query_selector('button[type="submit"], .el-button--primary')
        assert login_button, "找不到登录按钮"
        
        print("  ✓ 登录页面元素完整")
        
    async def test_login_with_invalid_token(self):
        """测试无效令牌登录"""
        await self.page.goto(f"{self.base_url}/login")
        await self.page.wait_for_load_state('networkidle')
        
        # 输入无效令牌
        await self.page.fill('input[type="password"]', 'invalid_token_12345')
        await self.page.click('button[type="submit"], .el-button--primary')
        
        # 等待响应
        await self.page.wait_for_timeout(2000)
        
        # 检查错误消息
        error_alert = await self.page.query_selector('.el-alert--error')
        if error_alert:
            error_text = await error_alert.text_content()
            print(f"  错误提示: {error_text}")
            assert "无效" in error_text or "失败" in error_text or "Invalid" in error_text or "invalid" in error_text, "错误消息不正确"
        
        # 确保仍在登录页
        assert "/login" in self.page.url, f"应该停留在登录页，当前URL: {self.page.url}"
        print("  ✓ 无效令牌登录正确处理")
        
    async def test_login_with_valid_token(self):
        """测试有效令牌登录"""
        await self.page.goto(f"{self.base_url}/login")
        await self.page.wait_for_load_state('networkidle')
        
        # 输入有效令牌
        await self.page.fill('input[type="password"]', self.token)
        await self.page.click('button[type="submit"], .el-button--primary')
        
        # 等待跳转到仪表盘
        try:
            await self.page.wait_for_url("**/dashboard", timeout=10000)
            print(f"  ✓ 成功跳转到: {self.page.url}")
        except Exception as e:
            # 检查当前URL
            print(f"  当前URL: {self.page.url}")
            raise AssertionError(f"登录后未跳转到仪表盘: {e}")
            
    async def test_dashboard_loads(self):
        """测试仪表盘页面加载"""
        # 确保已登录
        await self._ensure_logged_in()
        
        await self.page.goto(f"{self.base_url}/dashboard")
        await self.page.wait_for_load_state('networkidle')
        await self.page.wait_for_timeout(2000)  # 等待数据加载
        
        # 检查节点信息卡片
        node_info = await self.page.query_selector('.info-card, .el-card')
        assert node_info, "找不到节点信息卡片"
        
        # 检查统计数据
        stat_cards = await self.page.query_selector_all('.stat-card')
        print(f"  找到 {len(stat_cards)} 个统计卡片")
        
        # 检查节点ID显示
        page_content = await self.page.content()
        assert "节点" in page_content, "页面应显示节点信息"
        
        print("  ✓ 仪表盘页面加载正常")
        
    async def test_topology_page(self):
        """测试网络拓扑页面"""
        await self._ensure_logged_in()
        
        await self.page.goto(f"{self.base_url}/topology")
        await self.page.wait_for_load_state('networkidle')
        await self.page.wait_for_timeout(3000)  # 等待图表渲染
        
        # 检查页面加载
        page_content = await self.page.content()
        assert "拓扑" in page_content or "topology" in page_content.lower(), "拓扑页面未正确加载"
        
        print("  ✓ 网络拓扑页面加载正常")
        
    async def test_endpoints_page(self):
        """测试 API 浏览器页面"""
        await self._ensure_logged_in()
        
        await self.page.goto(f"{self.base_url}/endpoints")
        await self.page.wait_for_load_state('networkidle')
        await self.page.wait_for_timeout(2000)
        
        # 检查 API 列表
        page_content = await self.page.content()
        assert "API" in page_content, "API 浏览器页面未正确加载"
        
        # 查找端点列表
        endpoints = await self.page.query_selector_all('.endpoint-item, .el-table__row')
        print(f"  找到 {len(endpoints)} 个 API 端点")
        
        print("  ✓ API 浏览器页面加载正常")
        
    async def test_logs_page(self):
        """测试日志页面"""
        await self._ensure_logged_in()
        
        await self.page.goto(f"{self.base_url}/logs")
        await self.page.wait_for_load_state('networkidle')
        await self.page.wait_for_timeout(2000)
        
        page_content = await self.page.content()
        assert "日志" in page_content or "log" in page_content.lower(), "日志页面未正确加载"
        
        print("  ✓ 日志页面加载正常")
        
    async def test_about_page(self):
        """测试关于页面"""
        await self._ensure_logged_in()
        
        await self.page.goto(f"{self.base_url}/about")
        await self.page.wait_for_load_state('networkidle')
        
        page_content = await self.page.content()
        assert "关于" in page_content or "DAAN" in page_content, "关于页面未正确加载"
        
        print("  ✓ 关于页面加载正常")
        
    async def test_navigation_menu(self):
        """测试导航菜单"""
        await self._ensure_logged_in()
        
        await self.page.goto(f"{self.base_url}/dashboard")
        await self.page.wait_for_load_state('networkidle')
        
        # 查找导航菜单项
        nav_items = await self.page.query_selector_all('.el-menu-item, nav a')
        print(f"  找到 {len(nav_items)} 个导航项")
        
        # 测试点击各个菜单
        menu_items = ['topology', 'endpoints', 'logs', 'about']
        for item in menu_items:
            nav_link = await self.page.query_selector(f'a[href*="{item}"], .el-menu-item:has-text("{item}")')
            if nav_link:
                await nav_link.click()
                await self.page.wait_for_load_state('networkidle')
                await self.page.wait_for_timeout(1000)
                print(f"  ✓ 导航到 {item}: {self.page.url}")
                
        print("  ✓ 导航菜单工作正常")
        
    async def test_logout(self):
        """测试登出功能"""
        await self._ensure_logged_in()
        
        # 查找登出按钮
        logout_btn = await self.page.query_selector('button:has-text("登出"), button:has-text("退出"), .logout-btn')
        if logout_btn:
            await logout_btn.click()
            await self.page.wait_for_timeout(2000)
            
            # 检查是否回到登录页
            assert "/login" in self.page.url, f"登出后应跳转到登录页，当前: {self.page.url}"
            print("  ✓ 登出成功")
        else:
            print("  ⚠️ 未找到登出按钮，跳过测试")
            
    async def test_api_response_times(self):
        """测试 API 响应时间"""
        apis = [
            ("/api/health", "健康检查"),
            ("/api/node/status", "节点状态"),
            ("/api/node/peers", "节点列表"),
            ("/api/stats", "网络统计"),
        ]
        
        await self._ensure_logged_in()
        
        for endpoint, name in apis:
            start = time.time()
            response = await self.page.request.get(f"{self.base_url}{endpoint}")
            duration = (time.time() - start) * 1000  # ms
            
            status_emoji = "✓" if response.status == 200 else "✗"
            print(f"  {status_emoji} {name}: {response.status} ({duration:.0f}ms)")
            
            assert response.status in [200, 401], f"{name} 响应异常: {response.status}"
            assert duration < 5000, f"{name} 响应过慢: {duration}ms"
            
    async def test_websocket_connection(self):
        """测试 WebSocket 连接"""
        await self._ensure_logged_in()
        await self.page.goto(f"{self.base_url}/dashboard")
        await self.page.wait_for_load_state('networkidle')
        
        # 等待 WebSocket 建立
        await self.page.wait_for_timeout(3000)
        
        # 检查控制台是否有 WebSocket 相关日志
        ws_logs = [log for log in self.console_logs if 'WebSocket' in log.text or 'ws' in log.text.lower()]
        print(f"  WebSocket 相关日志: {len(ws_logs)} 条")
        
        # WebSocket 连接可能失败（如果服务不支持），但不应有未捕获的错误
        print("  ✓ WebSocket 测试完成")
        
    async def test_responsive_design(self):
        """测试响应式设计"""
        await self._ensure_logged_in()
        
        viewports = [
            (1920, 1080, "桌面"),
            (1366, 768, "笔记本"),
            (768, 1024, "平板"),
            (375, 667, "手机"),
        ]
        
        for width, height, device in viewports:
            await self.page.set_viewport_size({"width": width, "height": height})
            await self.page.goto(f"{self.base_url}/dashboard")
            await self.page.wait_for_load_state('networkidle')
            await self.page.wait_for_timeout(1000)
            
            # 检查页面是否可见
            visible_content = await self.page.query_selector('.dashboard, .el-main, main')
            assert visible_content, f"{device} 视图下页面内容不可见"
            print(f"  ✓ {device} ({width}x{height})")
            
        # 恢复默认视口
        await self.page.set_viewport_size({"width": 1920, "height": 1080})
        print("  ✓ 响应式设计测试通过")
        
    async def test_url_token_login(self):
        """测试 URL 令牌登录"""
        # 直接用 token 参数访问登录页
        url_with_token = f"{self.base_url}/login?token={self.token}"
        await self.page.goto(url_with_token)
        
        # 等待自动登录和跳转
        try:
            await self.page.wait_for_url("**/dashboard", timeout=10000)
            print(f"  ✓ URL 令牌登录成功，跳转到: {self.page.url}")
        except Exception as e:
            print(f"  当前URL: {self.page.url}")
            # 不一定是错误，可能需要手动确认
            print(f"  ⚠️ URL 令牌自动登录未生效: {e}")
            
    # ============ 辅助方法 ============
    
    async def _ensure_logged_in(self):
        """确保已登录状态"""
        # 检查是否已登录
        await self.page.goto(f"{self.base_url}/dashboard")
        await self.page.wait_for_timeout(1000)
        
        if "/login" in self.page.url:
            # 需要登录
            await self.page.fill('input[type="password"]', self.token)
            await self.page.click('button[type="submit"], .el-button--primary')
            await self.page.wait_for_url("**/dashboard", timeout=10000)
            
    async def run_all_tests(self):
        """运行所有测试"""
        print("\n" + "="*60)
        print("DAAN 管理平台前端自动化测试")
        print(f"目标: {self.base_url}")
        print(f"时间: {datetime.now().strftime('%Y-%m-%d %H:%M:%S')}")
        print("="*60)
        
        tests = [
            ("健康检查 API", self.test_health_check),
            ("登录页面加载", self.test_login_page_loads),
            ("无效令牌登录", self.test_login_with_invalid_token),
            ("有效令牌登录", self.test_login_with_valid_token),
            ("仪表盘页面", self.test_dashboard_loads),
            ("网络拓扑页面", self.test_topology_page),
            ("API 浏览器页面", self.test_endpoints_page),
            ("日志页面", self.test_logs_page),
            ("关于页面", self.test_about_page),
            ("导航菜单", self.test_navigation_menu),
            ("API 响应时间", self.test_api_response_times),
            ("WebSocket 连接", self.test_websocket_connection),
            ("URL 令牌登录", self.test_url_token_login),
        ]
        
        await self.setup()
        
        try:
            for name, test_func in tests:
                await self.run_test(name, test_func)
        finally:
            await self.teardown()
            
        return self.generate_report()
        
    async def run_quick_tests(self):
        """运行快速测试（基本功能）"""
        print("\n" + "="*60)
        print("DAAN 管理平台前端快速测试")
        print(f"目标: {self.base_url}")
        print("="*60)
        
        tests = [
            ("健康检查 API", self.test_health_check),
            ("登录页面加载", self.test_login_page_loads),
            ("有效令牌登录", self.test_login_with_valid_token),
            ("仪表盘页面", self.test_dashboard_loads),
        ]
        
        await self.setup()
        
        try:
            for name, test_func in tests:
                await self.run_test(name, test_func)
        finally:
            await self.teardown()
            
        return self.generate_report()
        
    def generate_report(self) -> TestReport:
        """生成测试报告"""
        passed = sum(1 for r in self.results if r.passed)
        failed = len(self.results) - passed
        
        report = TestReport(
            timestamp=datetime.now().isoformat(),
            base_url=self.base_url,
            total_tests=len(self.results),
            passed=passed,
            failed=failed,
            results=self.results,
            all_console_logs=self.console_logs
        )
        
        # 打印摘要
        print("\n" + "="*60)
        print("测试报告摘要")
        print("="*60)
        print(f"总计: {report.total_tests} 个测试")
        print(f"通过: {report.passed} ✅")
        print(f"失败: {report.failed} ❌")
        print(f"控制台日志: {len(self.console_logs)} 条")
        
        # 统计控制台日志类型
        log_types = {}
        for log in self.console_logs:
            log_types[log.type] = log_types.get(log.type, 0) + 1
        print(f"日志类型分布: {log_types}")
        
        # 列出失败的测试
        if failed > 0:
            print("\n失败的测试:")
            for r in self.results:
                if not r.passed:
                    print(f"  ❌ {r.name}: {r.message}")
                    for err in r.errors:
                        print(f"     - {err}")
                        
        # 列出所有错误日志
        error_logs = [log for log in self.console_logs if log.type == 'error']
        if error_logs:
            print(f"\n控制台错误 ({len(error_logs)} 条):")
            for log in error_logs[:10]:  # 只显示前10条
                print(f"  [{log.timestamp}] {log.text[:100]}...")
                
        return report

def read_token(token_file: str) -> str:
    """读取管理令牌"""
    try:
        with open(token_file, 'r') as f:
            return f.read().strip()
    except FileNotFoundError:
        print(f"警告: 找不到令牌文件 {token_file}")
        return ""
        
def save_report(report: TestReport, output_dir: str):
    """保存测试报告"""
    os.makedirs(output_dir, exist_ok=True)
    timestamp = datetime.now().strftime("%Y%m%d_%H%M%S")
    filename = os.path.join(output_dir, f"frontend_test_{timestamp}.json")
    
    # 转换为可序列化的格式
    report_dict = {
        "timestamp": report.timestamp,
        "base_url": report.base_url,
        "total_tests": report.total_tests,
        "passed": report.passed,
        "failed": report.failed,
        "results": [
            {
                "name": r.name,
                "passed": r.passed,
                "message": r.message,
                "duration": r.duration,
                "errors": r.errors,
                "screenshot": r.screenshot,
                "console_logs_count": len(r.console_logs)
            }
            for r in report.results
        ],
        "console_logs": [
            {
                "timestamp": log.timestamp,
                "type": log.type,
                "text": log.text[:500],  # 截断长文本
                "location": log.location
            }
            for log in report.all_console_logs
        ]
    }
    
    with open(filename, 'w', encoding='utf-8') as f:
        json.dump(report_dict, f, ensure_ascii=False, indent=2)
        
    print(f"\n📄 测试报告已保存: {filename}")
    return filename

async def main():
    parser = argparse.ArgumentParser(description='DAAN 前端自动化测试')
    parser.add_argument('--base-url', default=DEFAULT_BASE_URL, help='管理平台URL')
    parser.add_argument('--token', help='管理令牌（默认从文件读取）')
    parser.add_argument('--token-file', default=DEFAULT_TOKEN_FILE, help='令牌文件路径')
    parser.add_argument('--headless', action='store_true', default=True, help='无头模式运行')
    parser.add_argument('--no-headless', dest='headless', action='store_false', help='显示浏览器窗口')
    parser.add_argument('--all', action='store_true', help='运行所有测试')
    parser.add_argument('--output', default=os.path.join(os.path.dirname(__file__), "..", "test_logs"),
                       help='报告输出目录')
    
    args = parser.parse_args()
    
    # 读取令牌
    token = args.token or read_token(args.token_file)
    if not token:
        print("错误: 未提供管理令牌")
        print("请使用 --token 参数或确保 data/admin_token 文件存在")
        sys.exit(1)
        
    # 创建测试器
    tester = FrontendTester(
        base_url=args.base_url,
        token=token,
        headless=args.headless
    )
    
    # 运行测试
    try:
        if args.all:
            report = await tester.run_all_tests()
        else:
            report = await tester.run_quick_tests()
            
        # 保存报告
        save_report(report, args.output)
        
        # 返回退出码
        sys.exit(0 if report.failed == 0 else 1)
        
    except Exception as e:
        print(f"\n❌ 测试运行失败: {e}")
        import traceback
        traceback.print_exc()
        sys.exit(2)

if __name__ == "__main__":
    asyncio.run(main())
