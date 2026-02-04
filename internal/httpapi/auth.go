// Package httpapi 提供 HTTP REST API 接口的认证功能
package httpapi

import (
	"crypto/rand"
	"crypto/subtle"
	"encoding/hex"
	"fmt"
	"net/http"
	"sync"
)

// Token 认证相关常量
const (
	TokenLength      = 32          // Token 长度（32字节 = 256位）
	TokenHeader      = "X-API-Token"
	TokenQueryParam  = "token"
)

// AuthConfig 认证配置
type AuthConfig struct {
	APIToken       string   `json:"api_token"`        // API Token
	TokenGenerated bool     `json:"token_generated"`  // 是否已生成 Token
	AllowedIPs     []string `json:"allowed_ips"`      // 允许的 IP 列表（可选）
	AuthEnabled    bool     `json:"auth_enabled"`     // 是否启用认证（默认启用）
}

// DefaultAuthConfig 返回默认认证配置
func DefaultAuthConfig() *AuthConfig {
	return &AuthConfig{
		AuthEnabled: true,
	}
}

// TokenManager Token 管理器
type TokenManager struct {
	mu     sync.RWMutex
	config *AuthConfig
}

// NewTokenManager 创建 Token 管理器
func NewTokenManager(config *AuthConfig) *TokenManager {
	if config == nil {
		config = DefaultAuthConfig()
	}
	return &TokenManager{
		config: config,
	}
}

// GenerateToken 生成随机 API Token
func GenerateToken() (string, error) {
	bytes := make([]byte, TokenLength)
	if _, err := rand.Read(bytes); err != nil {
		return "", fmt.Errorf("生成 Token 失败: %w", err)
	}
	return hex.EncodeToString(bytes), nil
}

// EnsureToken 确保 Token 存在，如果不存在则生成
// 返回 (token, isNewlyGenerated, error)
func (tm *TokenManager) EnsureToken() (string, bool, error) {
	tm.mu.Lock()
	defer tm.mu.Unlock()

	if tm.config.APIToken != "" && tm.config.TokenGenerated {
		return tm.config.APIToken, false, nil
	}

	// 生成新 Token
	token, err := GenerateToken()
	if err != nil {
		return "", false, err
	}

	tm.config.APIToken = token
	tm.config.TokenGenerated = true

	return token, true, nil
}

// GetToken 获取当前 Token
func (tm *TokenManager) GetToken() string {
	tm.mu.RLock()
	defer tm.mu.RUnlock()
	return tm.config.APIToken
}

// SetToken 设置 Token
func (tm *TokenManager) SetToken(token string) {
	tm.mu.Lock()
	defer tm.mu.Unlock()
	tm.config.APIToken = token
	tm.config.TokenGenerated = true
}

// RegenerateToken 重新生成 Token
func (tm *TokenManager) RegenerateToken() (string, error) {
	tm.mu.Lock()
	defer tm.mu.Unlock()

	token, err := GenerateToken()
	if err != nil {
		return "", err
	}

	tm.config.APIToken = token
	tm.config.TokenGenerated = true

	return token, nil
}

// RevokeToken 撤销 Token（禁用 API 访问）
func (tm *TokenManager) RevokeToken() {
	tm.mu.Lock()
	defer tm.mu.Unlock()
	tm.config.APIToken = ""
	tm.config.TokenGenerated = false
}

// ValidateToken 验证 Token
func (tm *TokenManager) ValidateToken(token string) bool {
	tm.mu.RLock()
	defer tm.mu.RUnlock()

	// 认证未启用时，允许所有请求
	if !tm.config.AuthEnabled {
		return true
	}

	// Token 未配置时，拒绝所有请求
	if tm.config.APIToken == "" {
		return false
	}

	// 空 Token 无效
	if token == "" {
		return false
	}

	// 使用常量时间比较防止时序攻击
	return subtle.ConstantTimeCompare([]byte(token), []byte(tm.config.APIToken)) == 1
}

// IsAuthEnabled 检查是否启用认证
func (tm *TokenManager) IsAuthEnabled() bool {
	tm.mu.RLock()
	defer tm.mu.RUnlock()
	return tm.config.AuthEnabled
}

// SetAuthEnabled 设置是否启用认证
func (tm *TokenManager) SetAuthEnabled(enabled bool) {
	tm.mu.Lock()
	defer tm.mu.Unlock()
	tm.config.AuthEnabled = enabled
}

// GetConfig 获取配置（用于持久化）
func (tm *TokenManager) GetConfig() *AuthConfig {
	tm.mu.RLock()
	defer tm.mu.RUnlock()
	// 返回副本
	return &AuthConfig{
		APIToken:       tm.config.APIToken,
		TokenGenerated: tm.config.TokenGenerated,
		AllowedIPs:     tm.config.AllowedIPs,
		AuthEnabled:    tm.config.AuthEnabled,
	}
}

// TokenAuthMiddleware 创建 Token 认证中间件
func (tm *TokenManager) TokenAuthMiddleware(next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		// 健康检查端点无需认证
		if r.URL.Path == "/health" || r.URL.Path == "/status" {
			next(w, r)
			return
		}

		// 认证未启用时，直接放行
		if !tm.IsAuthEnabled() {
			next(w, r)
			return
		}

		// 获取 Token（优先从 Header，备选从 URL 参数）
		token := r.Header.Get(TokenHeader)
		if token == "" {
			token = r.URL.Query().Get(TokenQueryParam)
		}

		// 验证 Token
		if !tm.ValidateToken(token) {
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusUnauthorized)
			w.Write([]byte(`{"success":false,"error":"invalid or missing API token","code":401}`))
			return
		}

		next(w, r)
	}
}

// PrintTokenInfo 打印 Token 信息到控制台（首次启动时调用）
func PrintTokenInfo(token string, listenAddr string) {
	fmt.Println()
	fmt.Println("════════════════════════════════════════════════════════════════════")
	fmt.Println("🔐 API Token 已生成 (请妥善保管，仅显示一次):")
	fmt.Println()
	fmt.Printf("   %s\n", token)
	fmt.Println()
	fmt.Println("   使用方式:")
	fmt.Printf("   curl -H \"X-API-Token: %s\" http://127.0.0.1%s/api/v1/node/info\n", token, listenAddr)
	fmt.Println()
	fmt.Println("   或使用 URL 参数:")
	fmt.Printf("   curl \"http://127.0.0.1%s/api/v1/node/info?token=%s\"\n", listenAddr, token)
	fmt.Println("════════════════════════════════════════════════════════════════════")
	fmt.Println()
}
