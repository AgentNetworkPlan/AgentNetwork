#!/usr/bin/env python3
"""
DAAN 管理平台前端自动化测试 (基于 Selenium)
测试前端的全流程，捕获控制台日志

使用方法:
    pip install selenium webdriver-manager requests
    python frontend_test_selenium.py                      # 快速测试
    python frontend_test_selenium.py --all                # 完整测试
    python frontend_test_selenium.py --headless           # 无头模式
    python frontend_test_selenium.py --base-url http://localhost:18080
"""

import argparse
import json
import os
import sys
import time
from dataclasses import dataclass, field
from datetime import datetime
from typing import List, Optional, Dict, Any
import traceback

try:
    from selenium import webdriver
    from selenium.webdriver.common.by import By
    from selenium.webdriver.support.ui import WebDriverWait
    from selenium.webdriver.support import expected_conditions as EC
    from selenium.webdriver.chrome.options import Options
    from selenium.webdriver.chrome.service import Service
    from selenium.common.exceptions import TimeoutException, WebDriverException
except ImportError:
    print("请先安装 Selenium: pip install selenium")
    sys.exit(1)

try:
    from webdriver_manager.chrome import ChromeDriverManager
    USE_WEBDRIVER_MANAGER = True
except ImportError:
    USE_WEBDRIVER_MANAGER = False
    print("提示: 安装 webdriver-manager 可自动管理 ChromeDriver: pip install webdriver-manager")

try:
    import requests
except ImportError:
    print("请先安装 requests: pip install requests")
    sys.exit(1)

# ============ 配置 ============

DEFAULT_BASE_URL = "http://127.0.0.1:18080"
DEFAULT_TOKEN_FILE = os.path.join(os.path.dirname(__file__), "..", "data", "admin_token")
DEFAULT_TIMEOUT = 30
SCREENSHOT_DIR = os.path.join(os.path.dirname(__file__), "..", "test_logs", "screenshots")

# ============ 数据类型 ============

@dataclass
class ConsoleLog:
    """控制台日志条目"""
    timestamp: str
    level: str  # SEVERE, WARNING, INFO, etc.
    message: str
    source: Optional[str] = None
    
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

# ============ 颜色输出 ============

class Colors:
    RED = '\033[91m'
    GREEN = '\033[92m'
    YELLOW = '\033[93m'
    BLUE = '\033[94m'
    MAGENTA = '\033[95m'
    CYAN = '\033[96m'
    RESET = '\033[0m'

def print_color(text: str, color: str = Colors.RESET):
    print(f"{color}{text}{Colors.RESET}")

# ============ 测试框架 ============

class FrontendTester:
    """前端自动化测试器"""
    
    def __init__(self, base_url: str, token: str, headless: bool = True):
        self.base_url = base_url.rstrip('/')
        self.token = token
        self.headless = headless
        self.results: List[TestResult] = []
        self.console_logs: List[ConsoleLog] = []
        self.driver: Optional[webdriver.Chrome] = None
        
    def _get_console_logs(self) -> List[ConsoleLog]:
        """获取浏览器控制台日志"""
        logs = []
        try:
            browser_logs = self.driver.get_log('browser')
            for entry in browser_logs:
                log = ConsoleLog(
                    timestamp=datetime.fromtimestamp(entry['timestamp'] / 1000).isoformat(),
                    level=entry['level'],
                    message=entry['message'],
                    source=entry.get('source')
                )
                logs.append(log)
                
                # 实时打印
                level_color = {
                    'SEVERE': Colors.RED,
                    'WARNING': Colors.YELLOW,
                    'INFO': Colors.BLUE,
                }.get(entry['level'], Colors.RESET)
                msg = entry['message'][:150] + '...' if len(entry['message']) > 150 else entry['message']
                print(f"  {level_color}[{entry['level']}]{Colors.RESET} {msg}")
        except Exception as e:
            # 某些浏览器可能不支持获取日志
            pass
        return logs
        
    def _take_screenshot(self, name: str) -> str:
        """截图"""
        os.makedirs(SCREENSHOT_DIR, exist_ok=True)
        timestamp = datetime.now().strftime("%Y%m%d_%H%M%S")
        filename = f"{name}_{timestamp}.png"
        filepath = os.path.join(SCREENSHOT_DIR, filename)
        self.driver.save_screenshot(filepath)
        return filepath
        
    def setup(self):
        """设置浏览器"""
        options = Options()
        
        if self.headless:
            options.add_argument('--headless=new')
            
        options.add_argument('--window-size=1920,1080')
        options.add_argument('--no-sandbox')
        options.add_argument('--disable-dev-shm-usage')
        options.add_argument('--disable-gpu')
        options.add_argument('--lang=zh-CN')
        
        # 启用日志记录
        options.set_capability('goog:loggingPrefs', {'browser': 'ALL'})
        
        try:
            if USE_WEBDRIVER_MANAGER:
                service = Service(ChromeDriverManager().install())
                self.driver = webdriver.Chrome(service=service, options=options)
            else:
                self.driver = webdriver.Chrome(options=options)
        except Exception as e:
            print_color(f"无法启动 Chrome 浏览器: {e}", Colors.RED)
            print_color("请确保已安装 Chrome 浏览器", Colors.YELLOW)
            raise
            
        self.driver.implicitly_wait(10)
        
    def teardown(self):
        """清理"""
        if self.driver:
            self.driver.quit()
            
    def run_test(self, name: str, test_func) -> TestResult:
        """运行单个测试"""
        print(f"\n{'='*60}")
        print_color(f"测试: {name}", Colors.CYAN)
        print('='*60)
        
        start_time = time.time()
        errors = []
        passed = False
        message = ""
        screenshot = None
        
        try:
            test_func()
            passed = True
            message = "测试通过"
            print_color(f"✅ {message}", Colors.GREEN)
        except AssertionError as e:
            message = f"断言失败: {e}"
            errors.append(str(e))
            print_color(f"❌ {message}", Colors.RED)
        except TimeoutException as e:
            message = f"超时: {e}"
            errors.append(str(e))
            print_color(f"❌ {message}", Colors.RED)
        except Exception as e:
            message = f"异常: {e}"
            errors.append(str(e))
            print_color(f"❌ {message}", Colors.RED)
            traceback.print_exc()
            
        # 获取控制台日志
        test_logs = self._get_console_logs()
        self.console_logs.extend(test_logs)
        
        # 失败时截图
        if not passed:
            try:
                screenshot = self._take_screenshot(name.replace(' ', '_'))
                print_color(f"📸 截图已保存: {screenshot}", Colors.YELLOW)
            except Exception as e:
                print_color(f"⚠️ 截图失败: {e}", Colors.YELLOW)
                
        duration = time.time() - start_time
        
        # 检查控制台错误
        console_errors = [log for log in test_logs if log.level == 'SEVERE']
        if console_errors:
            print_color(f"⚠️ 发现 {len(console_errors)} 个控制台错误", Colors.YELLOW)
            for err in console_errors:
                errors.append(f"Console Error: {err.message[:200]}")
        
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
    
    def test_api_health(self):
        """测试健康检查 API (不通过浏览器)"""
        response = requests.get(f"{self.base_url}/api/health", timeout=10)
        assert response.status_code == 200, f"健康检查失败: status={response.status_code}"
        
        data = response.json()
        assert data.get('status') == 'healthy', f"状态不正确: {data}"
        print(f"  健康检查响应: {data}")
        
    def test_login_page_loads(self):
        """测试登录页面加载"""
        self.driver.get(f"{self.base_url}/login")
        
        # 等待页面加载
        WebDriverWait(self.driver, DEFAULT_TIMEOUT).until(
            EC.presence_of_element_located((By.CSS_SELECTOR, "input[type='password'], .el-input"))
        )
        
        title = self.driver.title
        print(f"  页面标题: {title}")
        
        # 检查登录表单
        try:
            token_input = self.driver.find_element(By.CSS_SELECTOR, "input[type='password']")
            assert token_input, "找不到令牌输入框"
        except:
            # 尝试其他选择器
            token_input = self.driver.find_element(By.CSS_SELECTOR, ".el-input__inner")
            assert token_input, "找不到令牌输入框"
        
        # 检查登录按钮
        try:
            login_button = self.driver.find_element(By.CSS_SELECTOR, "button[type='submit']")
        except:
            login_button = self.driver.find_element(By.CSS_SELECTOR, ".el-button--primary")
        assert login_button, "找不到登录按钮"
        
        print("  ✓ 登录页面元素完整")
        
    def test_login_with_invalid_token(self):
        """测试无效令牌登录"""
        self.driver.get(f"{self.base_url}/login")
        
        # 等待页面加载
        WebDriverWait(self.driver, DEFAULT_TIMEOUT).until(
            EC.presence_of_element_located((By.CSS_SELECTOR, "input[type='password'], .el-input__inner"))
        )
        
        # 输入无效令牌
        try:
            token_input = self.driver.find_element(By.CSS_SELECTOR, "input[type='password']")
        except:
            token_input = self.driver.find_element(By.CSS_SELECTOR, ".el-input__inner")
            
        token_input.clear()
        token_input.send_keys('invalid_token_12345')
        
        # 点击登录按钮
        try:
            login_button = self.driver.find_element(By.CSS_SELECTOR, "button[type='submit']")
        except:
            login_button = self.driver.find_element(By.CSS_SELECTOR, ".el-button--primary")
        login_button.click()
        
        # 等待响应
        time.sleep(2)
        
        # 检查是否仍在登录页
        assert "/login" in self.driver.current_url, f"应该停留在登录页，当前URL: {self.driver.current_url}"
        
        # 检查错误消息
        try:
            error_alert = self.driver.find_element(By.CSS_SELECTOR, ".el-alert--error, .el-message--error")
            error_text = error_alert.text
            print(f"  错误提示: {error_text}")
        except:
            print("  未检测到错误提示元素（可能是其他形式的反馈）")
            
        print("  ✓ 无效令牌登录正确处理")
        
    def test_login_with_valid_token(self):
        """测试有效令牌登录"""
        self.driver.get(f"{self.base_url}/login")
        
        # 等待页面加载
        WebDriverWait(self.driver, DEFAULT_TIMEOUT).until(
            EC.presence_of_element_located((By.CSS_SELECTOR, "input[type='password'], .el-input__inner"))
        )
        
        # 输入有效令牌
        try:
            token_input = self.driver.find_element(By.CSS_SELECTOR, "input[type='password']")
        except:
            token_input = self.driver.find_element(By.CSS_SELECTOR, ".el-input__inner")
            
        token_input.clear()
        token_input.send_keys(self.token)
        
        # 点击登录按钮
        try:
            login_button = self.driver.find_element(By.CSS_SELECTOR, "button[type='submit']")
        except:
            login_button = self.driver.find_element(By.CSS_SELECTOR, ".el-button--primary")
        login_button.click()
        
        # 等待跳转到仪表盘
        WebDriverWait(self.driver, DEFAULT_TIMEOUT).until(
            EC.url_contains("/dashboard")
        )
        
        print(f"  ✓ 成功跳转到: {self.driver.current_url}")
        
    def test_dashboard_loads(self):
        """测试仪表盘页面加载"""
        self._ensure_logged_in()
        
        self.driver.get(f"{self.base_url}/dashboard")
        
        # 等待页面加载
        WebDriverWait(self.driver, DEFAULT_TIMEOUT).until(
            EC.presence_of_element_located((By.CSS_SELECTOR, ".el-card, .dashboard, .info-card"))
        )
        
        time.sleep(2)  # 等待数据加载
        
        # 检查节点信息卡片
        cards = self.driver.find_elements(By.CSS_SELECTOR, ".el-card")
        print(f"  找到 {len(cards)} 个卡片")
        
        # 检查统计数据
        stat_cards = self.driver.find_elements(By.CSS_SELECTOR, ".stat-card")
        print(f"  找到 {len(stat_cards)} 个统计卡片")
        
        # 检查页面内容
        page_source = self.driver.page_source
        assert "节点" in page_source or "Node" in page_source, "页面应显示节点信息"
        
        print("  ✓ 仪表盘页面加载正常")
        
    def test_topology_page(self):
        """测试网络拓扑页面"""
        self._ensure_logged_in()
        
        self.driver.get(f"{self.base_url}/topology")
        
        # 等待页面加载
        WebDriverWait(self.driver, DEFAULT_TIMEOUT).until(
            EC.presence_of_element_located((By.CSS_SELECTOR, ".el-card, canvas, .topology, svg"))
        )
        
        time.sleep(3)  # 等待图表渲染
        
        print("  ✓ 网络拓扑页面加载正常")
        
    def test_endpoints_page(self):
        """测试 API 浏览器页面"""
        self._ensure_logged_in()
        
        self.driver.get(f"{self.base_url}/endpoints")
        
        # 等待页面加载
        WebDriverWait(self.driver, DEFAULT_TIMEOUT).until(
            EC.presence_of_element_located((By.CSS_SELECTOR, ".el-card, .el-table, .endpoint"))
        )
        
        time.sleep(2)
        
        # 查找端点列表
        rows = self.driver.find_elements(By.CSS_SELECTOR, ".el-table__row, .endpoint-item")
        print(f"  找到 {len(rows)} 个 API 端点")
        
        print("  ✓ API 浏览器页面加载正常")
        
    def test_logs_page(self):
        """测试日志页面"""
        self._ensure_logged_in()
        
        self.driver.get(f"{self.base_url}/logs")
        
        # 等待页面加载
        WebDriverWait(self.driver, DEFAULT_TIMEOUT).until(
            EC.presence_of_element_located((By.CSS_SELECTOR, ".el-card, .log-viewer, .logs"))
        )
        
        time.sleep(2)
        
        page_source = self.driver.page_source
        assert "日志" in page_source or "Log" in page_source or "log" in page_source.lower(), "日志页面未正确加载"
        
        print("  ✓ 日志页面加载正常")
        
    def test_about_page(self):
        """测试关于页面"""
        self._ensure_logged_in()
        
        self.driver.get(f"{self.base_url}/about")
        
        # 等待页面加载
        time.sleep(2)
        
        page_source = self.driver.page_source
        assert "关于" in page_source or "DAAN" in page_source or "About" in page_source, "关于页面未正确加载"
        
        print("  ✓ 关于页面加载正常")
        
    def test_navigation_menu(self):
        """测试导航菜单"""
        self._ensure_logged_in()
        
        self.driver.get(f"{self.base_url}/dashboard")
        time.sleep(2)
        
        # 查找导航菜单项
        nav_items = self.driver.find_elements(By.CSS_SELECTOR, ".el-menu-item, .nav-item, nav a")
        print(f"  找到 {len(nav_items)} 个导航项")
        
        # 测试导航
        pages = [
            ('topology', '拓扑'),
            ('endpoints', 'API'),
            ('logs', '日志'),
            ('about', '关于'),
        ]
        
        for path, name in pages:
            try:
                # 尝试点击导航
                nav_link = self.driver.find_element(By.CSS_SELECTOR, f'a[href*="{path}"]')
                nav_link.click()
                time.sleep(1)
                print(f"  ✓ 导航到 {name}: {self.driver.current_url}")
            except:
                # 直接访问
                self.driver.get(f"{self.base_url}/{path}")
                time.sleep(1)
                print(f"  ✓ 直接访问 {name}: {self.driver.current_url}")
                
        print("  ✓ 导航测试完成")
        
    def test_api_response_times(self):
        """测试 API 响应时间"""
        session = requests.Session()
        
        # 先登录获取 cookie
        login_response = session.post(
            f"{self.base_url}/api/auth/login",
            json={"token": self.token},
            timeout=10
        )
        
        apis = [
            ("/api/health", "健康检查"),
            ("/api/node/status", "节点状态"),
            ("/api/node/peers", "节点列表"),
            ("/api/stats", "网络统计"),
            ("/api/topology", "网络拓扑"),
            ("/api/endpoints", "API列表"),
        ]
        
        for endpoint, name in apis:
            start = time.time()
            response = session.get(f"{self.base_url}{endpoint}", timeout=10)
            duration = (time.time() - start) * 1000  # ms
            
            status_emoji = "✓" if response.status_code == 200 else "✗"
            print(f"  {status_emoji} {name}: {response.status_code} ({duration:.0f}ms)")
            
            assert duration < 5000, f"{name} 响应过慢: {duration}ms"
            
    def test_url_token_login(self):
        """测试 URL 令牌登录"""
        # 清除之前的会话
        self.driver.delete_all_cookies()
        
        # 直接用 token 参数访问登录页
        url_with_token = f"{self.base_url}/login?token={self.token}"
        self.driver.get(url_with_token)
        
        # 等待可能的自动登录和跳转
        time.sleep(5)
        
        current_url = self.driver.current_url
        print(f"  当前URL: {current_url}")
        
        if "/dashboard" in current_url:
            print("  ✓ URL 令牌自动登录成功")
        else:
            print("  ⚠️ URL 令牌自动登录未生效，可能需要手动确认")
            
    def test_responsive_layout(self):
        """测试响应式布局"""
        self._ensure_logged_in()
        
        viewports = [
            (1920, 1080, "桌面"),
            (1366, 768, "笔记本"),
            (768, 1024, "平板"),
        ]
        
        for width, height, device in viewports:
            self.driver.set_window_size(width, height)
            self.driver.get(f"{self.base_url}/dashboard")
            time.sleep(2)
            
            # 检查页面是否正常显示
            try:
                WebDriverWait(self.driver, 10).until(
                    EC.presence_of_element_located((By.CSS_SELECTOR, ".el-card, .dashboard"))
                )
                print(f"  ✓ {device} ({width}x{height})")
            except:
                print(f"  ✗ {device} ({width}x{height}) - 页面加载异常")
                
        # 恢复默认大小
        self.driver.set_window_size(1920, 1080)
        print("  ✓ 响应式布局测试完成")
            
    # ============ 辅助方法 ============
    
    def _ensure_logged_in(self):
        """确保已登录状态"""
        self.driver.get(f"{self.base_url}/dashboard")
        time.sleep(1)
        
        if "/login" in self.driver.current_url:
            # 需要登录
            try:
                token_input = self.driver.find_element(By.CSS_SELECTOR, "input[type='password']")
            except:
                token_input = self.driver.find_element(By.CSS_SELECTOR, ".el-input__inner")
                
            token_input.clear()
            token_input.send_keys(self.token)
            
            try:
                login_button = self.driver.find_element(By.CSS_SELECTOR, "button[type='submit']")
            except:
                login_button = self.driver.find_element(By.CSS_SELECTOR, ".el-button--primary")
            login_button.click()
            
            WebDriverWait(self.driver, DEFAULT_TIMEOUT).until(
                EC.url_contains("/dashboard")
            )
            
    def run_all_tests(self):
        """运行所有测试"""
        print("\n" + "="*60)
        print_color("DAAN 管理平台前端自动化测试", Colors.CYAN)
        print(f"目标: {self.base_url}")
        print(f"时间: {datetime.now().strftime('%Y-%m-%d %H:%M:%S')}")
        print("="*60)
        
        tests = [
            ("API 健康检查", self.test_api_health),
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
            ("URL 令牌登录", self.test_url_token_login),
            ("响应式布局", self.test_responsive_layout),
        ]
        
        self.setup()
        
        try:
            for name, test_func in tests:
                self.run_test(name, test_func)
        finally:
            self.teardown()
            
        return self.generate_report()
        
    def run_quick_tests(self):
        """运行快速测试（基本功能）"""
        print("\n" + "="*60)
        print_color("DAAN 管理平台前端快速测试", Colors.CYAN)
        print(f"目标: {self.base_url}")
        print("="*60)
        
        tests = [
            ("API 健康检查", self.test_api_health),
            ("登录页面加载", self.test_login_page_loads),
            ("有效令牌登录", self.test_login_with_valid_token),
            ("仪表盘页面", self.test_dashboard_loads),
        ]
        
        self.setup()
        
        try:
            for name, test_func in tests:
                self.run_test(name, test_func)
        finally:
            self.teardown()
            
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
        print_color("测试报告摘要", Colors.CYAN)
        print("="*60)
        print(f"总计: {report.total_tests} 个测试")
        print_color(f"通过: {report.passed} ✅", Colors.GREEN)
        if report.failed > 0:
            print_color(f"失败: {report.failed} ❌", Colors.RED)
        print(f"控制台日志: {len(self.console_logs)} 条")
        
        # 统计控制台日志类型
        log_types = {}
        for log in self.console_logs:
            log_types[log.level] = log_types.get(log.level, 0) + 1
        if log_types:
            print(f"日志类型分布: {log_types}")
        
        # 列出失败的测试
        if failed > 0:
            print_color("\n失败的测试:", Colors.RED)
            for r in self.results:
                if not r.passed:
                    print(f"  ❌ {r.name}: {r.message}")
                    for err in r.errors[:3]:  # 只显示前3个错误
                        print(f"     - {err[:100]}...")
                        
        # 列出所有 SEVERE 级别日志
        severe_logs = [log for log in self.console_logs if log.level == 'SEVERE']
        if severe_logs:
            print_color(f"\n控制台严重错误 ({len(severe_logs)} 条):", Colors.RED)
            for log in severe_logs[:5]:  # 只显示前5条
                print(f"  [{log.timestamp}] {log.message[:100]}...")
                
        return report

def read_token(token_file: str) -> str:
    """读取管理令牌"""
    try:
        with open(token_file, 'r') as f:
            return f.read().strip()
    except FileNotFoundError:
        print_color(f"警告: 找不到令牌文件 {token_file}", Colors.YELLOW)
        return ""
        
def save_report(report: TestReport, output_dir: str) -> str:
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
                "level": log.level,
                "message": log.message[:500] if log.message else "",
                "source": log.source
            }
            for log in report.all_console_logs
        ]
    }
    
    with open(filename, 'w', encoding='utf-8') as f:
        json.dump(report_dict, f, ensure_ascii=False, indent=2)
        
    print_color(f"\n📄 测试报告已保存: {filename}", Colors.GREEN)
    return filename

def wait_for_server(base_url: str, timeout: int = 60) -> bool:
    """等待服务器启动"""
    print_color(f"等待服务器启动: {base_url}", Colors.YELLOW)
    start = time.time()
    while time.time() - start < timeout:
        try:
            response = requests.get(f"{base_url}/api/health", timeout=5)
            if response.status_code == 200:
                print_color("✓ 服务器已就绪", Colors.GREEN)
                return True
        except:
            pass
        print(".", end="", flush=True)
        time.sleep(2)
    print()
    return False

def main():
    parser = argparse.ArgumentParser(description='DAAN 前端自动化测试 (Selenium)')
    parser.add_argument('--base-url', default=DEFAULT_BASE_URL, help='管理平台URL')
    parser.add_argument('--token', help='管理令牌（默认从文件读取）')
    parser.add_argument('--token-file', default=DEFAULT_TOKEN_FILE, help='令牌文件路径')
    parser.add_argument('--headless', action='store_true', default=False, help='无头模式运行')
    parser.add_argument('--all', action='store_true', help='运行所有测试')
    parser.add_argument('--wait', type=int, default=30, help='等待服务器启动的超时时间(秒)')
    parser.add_argument('--output', default=os.path.join(os.path.dirname(__file__), "..", "test_logs"),
                       help='报告输出目录')
    
    args = parser.parse_args()
    
    # 读取令牌
    token = args.token or read_token(args.token_file)
    if not token:
        print_color("错误: 未提供管理令牌", Colors.RED)
        print("请使用 --token 参数或确保 data/admin_token 文件存在")
        sys.exit(1)
    
    # 等待服务器启动
    if not wait_for_server(args.base_url, args.wait):
        print_color(f"错误: 服务器未在 {args.wait} 秒内启动", Colors.RED)
        print_color("请确保节点已启动: go run cmd/node/main.go run", Colors.YELLOW)
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
            report = tester.run_all_tests()
        else:
            report = tester.run_quick_tests()
            
        # 保存报告
        save_report(report, args.output)
        
        # 返回退出码
        sys.exit(0 if report.failed == 0 else 1)
        
    except WebDriverException as e:
        print_color(f"\n❌ 浏览器错误: {e}", Colors.RED)
        print_color("请确保 Chrome 浏览器已安装", Colors.YELLOW)
        sys.exit(2)
    except Exception as e:
        print_color(f"\n❌ 测试运行失败: {e}", Colors.RED)
        traceback.print_exc()
        sys.exit(2)

if __name__ == "__main__":
    main()
