package config

import (
	"encoding/json"
	"os"
	"path/filepath"
)

// Config 应用程序配置
type Config struct {
	// Agent 基础配置
	AgentID   string `json:"agent_id"`
	Version   string `json:"version"`
	BaseDir   string `json:"base_dir"`

	// 密钥配置
	PrivateKeyPath string `json:"private_key_path"`
	PublicKeyPath  string `json:"public_key_path"`
	KeyAlgorithm   string `json:"key_algorithm"` // "sm2" 或 "ecc"

	// 网络配置
	Network NetworkConfig `json:"network"`

	// GitHub 配置
	GitHub GitHubConfig `json:"github"`
}

// NetworkConfig 网络相关配置
type NetworkConfig struct {
	ListenAddr    string   `json:"listen_addr"`
	BootstrapNodes []string `json:"bootstrap_nodes"`
	EnableDHT     bool     `json:"enable_dht"`
}

// GitHubConfig GitHub 相关配置
type GitHubConfig struct {
	Token      string `json:"token"`
	Owner      string `json:"owner"`
	Repo       string `json:"repo"`
	KeysPath   string `json:"keys_path"`
}

// DefaultConfig 返回默认配置
func DefaultConfig() *Config {
	return &Config{
		Version:      "0.1.0",
		KeyAlgorithm: "sm2",
		Network: NetworkConfig{
			ListenAddr: ":8080",
			EnableDHT:  true,
		},
		GitHub: GitHubConfig{
			Owner:    "AgentNetworkPlan",
			Repo:     "AgentNetwork",
			KeysPath: "registry/keys",
		},
	}
}

// Load 从文件或环境变量加载配置
func Load() (*Config, error) {
	cfg := DefaultConfig()

	// 尝试从配置文件加载
	configPath := os.Getenv("DAAN_CONFIG_PATH")
	if configPath == "" {
		configPath = "config.json"
	}

	if data, err := os.ReadFile(configPath); err == nil {
		if err := json.Unmarshal(data, cfg); err != nil {
			return nil, err
		}
	}

	// 环境变量覆盖
	if token := os.Getenv("AGENTS_GITHUB_TOKEN"); token != "" {
		cfg.GitHub.Token = token
	}

	if baseDir := os.Getenv("DAAN_BASE_DIR"); baseDir != "" {
		cfg.BaseDir = baseDir
	} else {
		// 默认使用当前目录
		if wd, err := os.Getwd(); err == nil {
			cfg.BaseDir = wd
		}
	}

	// 设置默认密钥路径
	if cfg.PrivateKeyPath == "" {
		cfg.PrivateKeyPath = filepath.Join(cfg.BaseDir, "keys", "private.pem")
	}
	if cfg.PublicKeyPath == "" {
		cfg.PublicKeyPath = filepath.Join(cfg.BaseDir, "keys", "public.pem")
	}

	return cfg, nil
}

// Save 保存配置到文件
func (c *Config) Save(path string) error {
	data, err := json.MarshalIndent(c, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(path, data, 0644)
}
