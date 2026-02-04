// Package sync - 端到端加密模块
// 实现消息的端到端加密，支持ECDH密钥交换和AES-GCM加密
package sync

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"sync"
	"time"
)

var (
	ErrKeyNotFound    = errors.New("encryption key not found")
	ErrDecryptFailed  = errors.New("decryption failed")
	ErrInvalidKey     = errors.New("invalid key")
	ErrKeyExpired     = errors.New("key has expired")
)

// KeyPair 密钥对
type KeyPair struct {
	PrivateKey *ecdsa.PrivateKey
	PublicKey  *ecdsa.PublicKey
	PublicKeyHex string // 公钥的十六进制表示
	CreatedAt  time.Time
}

// SharedSecret 共享密钥
type SharedSecret struct {
	PeerID    string    // 对方节点ID
	Secret    []byte    // 共享密钥
	CreatedAt time.Time // 创建时间
	ExpiresAt time.Time // 过期时间
}

// EncryptorConfig 加密器配置
type EncryptorConfig struct {
	NodeID        string
	KeyExpiry     time.Duration // 密钥过期时间
	EnablePFS     bool          // 启用前向保密
	KeyRotation   time.Duration // 密钥轮换周期
}

// DefaultEncryptorConfig 默认加密器配置
func DefaultEncryptorConfig(nodeID string) *EncryptorConfig {
	return &EncryptorConfig{
		NodeID:      nodeID,
		KeyExpiry:   24 * time.Hour,
		EnablePFS:   true,
		KeyRotation: 1 * time.Hour,
	}
}

// E2EEncryptor 端到端加密器
type E2EEncryptor struct {
	config *EncryptorConfig
	
	// 本节点密钥对
	keyPair *KeyPair
	
	// 对方公钥缓存: peerID -> publicKey
	peerKeys map[string][]byte
	
	// 共享密钥缓存: peerID -> SharedSecret
	sharedSecrets map[string]*SharedSecret
	
	// 临时密钥（用于前向保密）
	ephemeralKeys map[string]*KeyPair
	
	mu sync.RWMutex
}

// NewE2EEncryptor 创建端到端加密器
func NewE2EEncryptor(config *EncryptorConfig) (*E2EEncryptor, error) {
	if config == nil {
		config = DefaultEncryptorConfig("")
	}
	
	e := &E2EEncryptor{
		config:        config,
		peerKeys:      make(map[string][]byte),
		sharedSecrets: make(map[string]*SharedSecret),
		ephemeralKeys: make(map[string]*KeyPair),
	}
	
	// 生成主密钥对
	if err := e.generateKeyPair(); err != nil {
		return nil, err
	}
	
	return e, nil
}

// generateKeyPair 生成密钥对
func (e *E2EEncryptor) generateKeyPair() error {
	privateKey, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		return fmt.Errorf("generate key pair: %w", err)
	}
	
	pubKeyBytes := elliptic.Marshal(elliptic.P256(), privateKey.PublicKey.X, privateKey.PublicKey.Y)
	
	e.keyPair = &KeyPair{
		PrivateKey:   privateKey,
		PublicKey:    &privateKey.PublicKey,
		PublicKeyHex: hex.EncodeToString(pubKeyBytes),
		CreatedAt:    time.Now(),
	}
	
	return nil
}

// GetPublicKey 获取本节点公钥
func (e *E2EEncryptor) GetPublicKey() string {
	e.mu.RLock()
	defer e.mu.RUnlock()
	return e.keyPair.PublicKeyHex
}

// SetPeerPublicKey 设置对方公钥
func (e *E2EEncryptor) SetPeerPublicKey(peerID string, publicKeyHex string) error {
	pubKeyBytes, err := hex.DecodeString(publicKeyHex)
	if err != nil {
		return fmt.Errorf("decode public key: %w", err)
	}
	
	// 验证公钥格式
	x, _ := elliptic.Unmarshal(elliptic.P256(), pubKeyBytes)
	if x == nil {
		return ErrInvalidKey
	}
	
	e.mu.Lock()
	e.peerKeys[peerID] = pubKeyBytes
	// 清除旧的共享密钥
	delete(e.sharedSecrets, peerID)
	e.mu.Unlock()
	
	return nil
}

// GetOrCreateSharedSecret 获取或创建共享密钥
func (e *E2EEncryptor) GetOrCreateSharedSecret(peerID string) (*SharedSecret, error) {
	e.mu.Lock()
	defer e.mu.Unlock()
	
	// 检查现有共享密钥
	if secret, ok := e.sharedSecrets[peerID]; ok {
		if time.Now().Before(secret.ExpiresAt) {
			return secret, nil
		}
		// 密钥已过期，删除
		delete(e.sharedSecrets, peerID)
	}
	
	// 获取对方公钥
	peerPubKeyBytes, ok := e.peerKeys[peerID]
	if !ok {
		return nil, ErrKeyNotFound
	}
	
	// 解析对方公钥
	x, peerY := elliptic.Unmarshal(elliptic.P256(), peerPubKeyBytes)
	if x == nil {
		return nil, ErrInvalidKey
	}
	peerPubKey := &ecdsa.PublicKey{
		Curve: elliptic.P256(),
		X:     x,
		Y:     peerY,
	}
	
	// ECDH密钥交换
	sharedX, _ := elliptic.P256().ScalarMult(peerPubKey.X, peerPubKey.Y, e.keyPair.PrivateKey.D.Bytes())
	
	// 派生AES密钥
	hash := sha256.Sum256(sharedX.Bytes())
	
	secret := &SharedSecret{
		PeerID:    peerID,
		Secret:    hash[:],
		CreatedAt: time.Now(),
		ExpiresAt: time.Now().Add(e.config.KeyExpiry),
	}
	
	e.sharedSecrets[peerID] = secret
	return secret, nil
}

// Encrypt 加密消息
func (e *E2EEncryptor) Encrypt(peerID string, plaintext []byte) ([]byte, error) {
	secret, err := e.GetOrCreateSharedSecret(peerID)
	if err != nil {
		return nil, err
	}
	
	// 创建AES-GCM加密器
	block, err := aes.NewCipher(secret.Secret)
	if err != nil {
		return nil, fmt.Errorf("create cipher: %w", err)
	}
	
	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return nil, fmt.Errorf("create gcm: %w", err)
	}
	
	// 生成随机nonce
	nonce := make([]byte, gcm.NonceSize())
	if _, err := io.ReadFull(rand.Reader, nonce); err != nil {
		return nil, fmt.Errorf("generate nonce: %w", err)
	}
	
	// 加密
	ciphertext := gcm.Seal(nonce, nonce, plaintext, nil)
	return ciphertext, nil
}

// Decrypt 解密消息
func (e *E2EEncryptor) Decrypt(peerID string, ciphertext []byte) ([]byte, error) {
	secret, err := e.GetOrCreateSharedSecret(peerID)
	if err != nil {
		return nil, err
	}
	
	// 创建AES-GCM解密器
	block, err := aes.NewCipher(secret.Secret)
	if err != nil {
		return nil, fmt.Errorf("create cipher: %w", err)
	}
	
	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return nil, fmt.Errorf("create gcm: %w", err)
	}
	
	nonceSize := gcm.NonceSize()
	if len(ciphertext) < nonceSize {
		return nil, ErrDecryptFailed
	}
	
	nonce, ciphertext := ciphertext[:nonceSize], ciphertext[nonceSize:]
	
	// 解密
	plaintext, err := gcm.Open(nil, nonce, ciphertext, nil)
	if err != nil {
		return nil, ErrDecryptFailed
	}
	
	return plaintext, nil
}

// GenerateEphemeralKey 生成临时密钥（用于前向保密）
func (e *E2EEncryptor) GenerateEphemeralKey(sessionID string) (string, error) {
	privateKey, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		return "", fmt.Errorf("generate ephemeral key: %w", err)
	}
	
	pubKeyBytes := elliptic.Marshal(elliptic.P256(), privateKey.PublicKey.X, privateKey.PublicKey.Y)
	pubKeyHex := hex.EncodeToString(pubKeyBytes)
	
	e.mu.Lock()
	e.ephemeralKeys[sessionID] = &KeyPair{
		PrivateKey:   privateKey,
		PublicKey:    &privateKey.PublicKey,
		PublicKeyHex: pubKeyHex,
		CreatedAt:    time.Now(),
	}
	e.mu.Unlock()
	
	return pubKeyHex, nil
}

// DeriveSessionKey 派生会话密钥（前向保密）
func (e *E2EEncryptor) DeriveSessionKey(sessionID, peerEphemeralKeyHex string) ([]byte, error) {
	e.mu.RLock()
	ephemeralKey, ok := e.ephemeralKeys[sessionID]
	e.mu.RUnlock()
	
	if !ok {
		return nil, errors.New("ephemeral key not found")
	}
	
	// 解析对方临时公钥
	peerKeyBytes, err := hex.DecodeString(peerEphemeralKeyHex)
	if err != nil {
		return nil, fmt.Errorf("decode peer key: %w", err)
	}
	
	x, y := elliptic.Unmarshal(elliptic.P256(), peerKeyBytes)
	if x == nil {
		return nil, ErrInvalidKey
	}
	
	// ECDH
	sharedX, _ := elliptic.P256().ScalarMult(x, y, ephemeralKey.PrivateKey.D.Bytes())
	
	// 派生密钥
	hash := sha256.Sum256(sharedX.Bytes())
	
	// 用完即删除（一次性密钥）
	e.mu.Lock()
	delete(e.ephemeralKeys, sessionID)
	e.mu.Unlock()
	
	return hash[:], nil
}

// EncryptWithSessionKey 使用会话密钥加密
func (e *E2EEncryptor) EncryptWithSessionKey(key, plaintext []byte) ([]byte, error) {
	block, err := aes.NewCipher(key)
	if err != nil {
		return nil, err
	}
	
	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return nil, err
	}
	
	nonce := make([]byte, gcm.NonceSize())
	if _, err := io.ReadFull(rand.Reader, nonce); err != nil {
		return nil, err
	}
	
	return gcm.Seal(nonce, nonce, plaintext, nil), nil
}

// DecryptWithSessionKey 使用会话密钥解密
func (e *E2EEncryptor) DecryptWithSessionKey(key, ciphertext []byte) ([]byte, error) {
	block, err := aes.NewCipher(key)
	if err != nil {
		return nil, err
	}
	
	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return nil, err
	}
	
	nonceSize := gcm.NonceSize()
	if len(ciphertext) < nonceSize {
		return nil, ErrDecryptFailed
	}
	
	nonce, ciphertext := ciphertext[:nonceSize], ciphertext[nonceSize:]
	return gcm.Open(nil, nonce, ciphertext, nil)
}

// CleanupExpiredKeys 清理过期密钥
func (e *E2EEncryptor) CleanupExpiredKeys() {
	e.mu.Lock()
	defer e.mu.Unlock()
	
	now := time.Now()
	
	// 清理过期的共享密钥
	for peerID, secret := range e.sharedSecrets {
		if now.After(secret.ExpiresAt) {
			delete(e.sharedSecrets, peerID)
		}
	}
	
	// 清理过期的临时密钥（超过1小时）
	for sessionID, key := range e.ephemeralKeys {
		if now.Sub(key.CreatedAt) > time.Hour {
			delete(e.ephemeralKeys, sessionID)
		}
	}
}

// KeyExchangeMessage 密钥交换消息
type KeyExchangeMessage struct {
	Type      string `json:"type"`       // "request" 或 "response"
	SessionID string `json:"session_id"` // 会话ID
	PublicKey string `json:"public_key"` // 公钥
	Timestamp int64  `json:"timestamp"`  // 时间戳
	Signature string `json:"signature"`  // 签名
}

// CreateKeyExchangeRequest 创建密钥交换请求
func (e *E2EEncryptor) CreateKeyExchangeRequest() (*KeyExchangeMessage, error) {
	sessionID := generateID()
	pubKey, err := e.GenerateEphemeralKey(sessionID)
	if err != nil {
		return nil, err
	}
	
	return &KeyExchangeMessage{
		Type:      "request",
		SessionID: sessionID,
		PublicKey: pubKey,
		Timestamp: time.Now().UnixNano(),
	}, nil
}

// CreateKeyExchangeResponse 创建密钥交换响应
func (e *E2EEncryptor) CreateKeyExchangeResponse(requestSessionID string) (*KeyExchangeMessage, error) {
	pubKey, err := e.GenerateEphemeralKey(requestSessionID)
	if err != nil {
		return nil, err
	}
	
	return &KeyExchangeMessage{
		Type:      "response",
		SessionID: requestSessionID,
		PublicKey: pubKey,
		Timestamp: time.Now().UnixNano(),
	}, nil
}
