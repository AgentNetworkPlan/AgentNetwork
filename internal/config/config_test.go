package config

import (
	"os"
	"path/filepath"
	"testing"
)

func TestDefaultConfig(t *testing.T) {
	cfg := DefaultConfig()

	if cfg == nil {
		t.Fatal("DefaultConfig 返回 nil")
	}

	if cfg.Version != "0.1.0" {
		t.Errorf("版本号错误: %s", cfg.Version)
	}

	if cfg.KeyAlgorithm != "sm2" {
		t.Errorf("密钥算法错误: %s", cfg.KeyAlgorithm)
	}

	if cfg.Network.ListenAddr != ":8080" {
		t.Errorf("监听地址错误: %s", cfg.Network.ListenAddr)
	}

	if !cfg.Network.EnableDHT {
		t.Error("DHT 应该默认启用")
	}

	if cfg.GitHub.Owner != "AgentNetworkPlan" {
		t.Errorf("GitHub Owner 错误: %s", cfg.GitHub.Owner)
	}
}

func TestConfig_Load(t *testing.T) {
	// 测试默认加载
	cfg, err := Load()
	if err != nil {
		t.Fatalf("加载配置失败: %v", err)
	}

	if cfg == nil {
		t.Fatal("配置为空")
	}

	// 验证默认值
	if cfg.Version == "" {
		t.Error("版本号为空")
	}
}

func TestConfig_LoadWithEnv(t *testing.T) {
	// 设置环境变量
	os.Setenv("AGENTS_GITHUB_TOKEN", "test-token-123")
	os.Setenv("DAAN_BASE_DIR", "/tmp/test-daan")
	defer func() {
		os.Unsetenv("AGENTS_GITHUB_TOKEN")
		os.Unsetenv("DAAN_BASE_DIR")
	}()

	cfg, err := Load()
	if err != nil {
		t.Fatalf("加载配置失败: %v", err)
	}

	if cfg.GitHub.Token != "test-token-123" {
		t.Errorf("GitHub Token 未从环境变量加载: %s", cfg.GitHub.Token)
	}

	if cfg.BaseDir != "/tmp/test-daan" {
		t.Errorf("BaseDir 未从环境变量加载: %s", cfg.BaseDir)
	}
}

func TestConfig_Save(t *testing.T) {
	tmpDir := t.TempDir()
	configPath := filepath.Join(tmpDir, "test-config.json")

	cfg := DefaultConfig()
	cfg.AgentID = "test-agent-id"
	cfg.GitHub.Token = "test-token"

	if err := cfg.Save(configPath); err != nil {
		t.Fatalf("保存配置失败: %v", err)
	}

	// 验证文件存在
	if _, err := os.Stat(configPath); os.IsNotExist(err) {
		t.Fatal("配置文件未创建")
	}

	// 读取并验证
	data, err := os.ReadFile(configPath)
	if err != nil {
		t.Fatalf("读取配置文件失败: %v", err)
	}

	if len(data) == 0 {
		t.Error("配置文件为空")
	}

	t.Logf("保存的配置大小: %d bytes", len(data))
}

func TestConfig_LoadFromFile(t *testing.T) {
	tmpDir := t.TempDir()
	configPath := filepath.Join(tmpDir, "config.json")

	// 创建测试配置文件
	configContent := `{
		"agent_id": "test-agent",
		"version": "1.0.0",
		"key_algorithm": "ecc",
		"network": {
			"listen_addr": ":9090",
			"enable_dht": false
		}
	}`

	if err := os.WriteFile(configPath, []byte(configContent), 0644); err != nil {
		t.Fatalf("创建测试配置文件失败: %v", err)
	}

	// 设置配置路径
	os.Setenv("DAAN_CONFIG_PATH", configPath)
	defer os.Unsetenv("DAAN_CONFIG_PATH")

	cfg, err := Load()
	if err != nil {
		t.Fatalf("加载配置失败: %v", err)
	}

	if cfg.AgentID != "test-agent" {
		t.Errorf("AgentID 错误: %s", cfg.AgentID)
	}

	if cfg.Version != "1.0.0" {
		t.Errorf("版本号错误: %s", cfg.Version)
	}

	if cfg.KeyAlgorithm != "ecc" {
		t.Errorf("密钥算法错误: %s", cfg.KeyAlgorithm)
	}

	if cfg.Network.ListenAddr != ":9090" {
		t.Errorf("监听地址错误: %s", cfg.Network.ListenAddr)
	}

	if cfg.Network.EnableDHT != false {
		t.Error("DHT 状态错误")
	}
}
