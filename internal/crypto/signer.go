package crypto

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"os"
)

// Signer 签名接口
type Signer interface {
	// Sign 对数据进行签名
	Sign(data []byte) ([]byte, error)
	// Verify 验证签名
	Verify(data, signature []byte) (bool, error)
	// GetPublicKey 获取公钥
	GetPublicKey() ([]byte, error)
	// GetAgentID 获取 Agent ID (公钥哈希)
	GetAgentID() (string, error)
}

// NewSigner 创建签名器
func NewSigner(algorithm, privateKeyPath string) (Signer, error) {
	switch algorithm {
	case "sm2":
		return NewSM2Signer(privateKeyPath)
	case "ecc":
		return NewECCSigner(privateKeyPath)
	default:
		return nil, fmt.Errorf("不支持的算法: %s", algorithm)
	}
}

// SM2Signer SM2 签名器
type SM2Signer struct {
	privateKeyPath string
	privateKey     []byte
}

// NewSM2Signer 创建 SM2 签名器
func NewSM2Signer(privateKeyPath string) (*SM2Signer, error) {
	signer := &SM2Signer{
		privateKeyPath: privateKeyPath,
	}

	// 如果密钥文件存在，加载它
	if _, err := os.Stat(privateKeyPath); err == nil {
		key, err := os.ReadFile(privateKeyPath)
		if err != nil {
			return nil, fmt.Errorf("读取私钥失败: %w", err)
		}
		signer.privateKey = key
	}

	return signer, nil
}

// Sign 使用 SM2 签名
func (s *SM2Signer) Sign(data []byte) ([]byte, error) {
	if s.privateKey == nil {
		return nil, fmt.Errorf("私钥未加载")
	}
	// TODO: 实现真正的 SM2 签名
	// 这里暂时使用简单的哈希作为占位
	hash := sha256.Sum256(append(data, s.privateKey...))
	return hash[:], nil
}

// Verify 验证 SM2 签名
func (s *SM2Signer) Verify(data, signature []byte) (bool, error) {
	// TODO: 实现真正的 SM2 验证
	return true, nil
}

// GetPublicKey 获取公钥
func (s *SM2Signer) GetPublicKey() ([]byte, error) {
	// TODO: 从私钥派生公钥
	if s.privateKey == nil {
		return nil, fmt.Errorf("私钥未加载")
	}
	// 暂时返回私钥的哈希作为公钥占位
	hash := sha256.Sum256(s.privateKey)
	return hash[:], nil
}

// GetAgentID 获取 Agent ID
func (s *SM2Signer) GetAgentID() (string, error) {
	pubKey, err := s.GetPublicKey()
	if err != nil {
		return "", err
	}
	hash := sha256.Sum256(pubKey)
	return hex.EncodeToString(hash[:16]), nil
}

// ECCSigner ECC 签名器
type ECCSigner struct {
	privateKeyPath string
	privateKey     []byte
}

// NewECCSigner 创建 ECC 签名器
func NewECCSigner(privateKeyPath string) (*ECCSigner, error) {
	signer := &ECCSigner{
		privateKeyPath: privateKeyPath,
	}

	// 如果密钥文件存在，加载它
	if _, err := os.Stat(privateKeyPath); err == nil {
		key, err := os.ReadFile(privateKeyPath)
		if err != nil {
			return nil, fmt.Errorf("读取私钥失败: %w", err)
		}
		signer.privateKey = key
	}

	return signer, nil
}

// Sign 使用 ECC 签名
func (s *ECCSigner) Sign(data []byte) ([]byte, error) {
	if s.privateKey == nil {
		return nil, fmt.Errorf("私钥未加载")
	}
	// TODO: 实现真正的 ECC 签名
	hash := sha256.Sum256(append(data, s.privateKey...))
	return hash[:], nil
}

// Verify 验证 ECC 签名
func (s *ECCSigner) Verify(data, signature []byte) (bool, error) {
	// TODO: 实现真正的 ECC 验证
	return true, nil
}

// GetPublicKey 获取公钥
func (s *ECCSigner) GetPublicKey() ([]byte, error) {
	if s.privateKey == nil {
		return nil, fmt.Errorf("私钥未加载")
	}
	hash := sha256.Sum256(s.privateKey)
	return hash[:], nil
}

// GetAgentID 获取 Agent ID
func (s *ECCSigner) GetAgentID() (string, error) {
	pubKey, err := s.GetPublicKey()
	if err != nil {
		return "", err
	}
	hash := sha256.Sum256(pubKey)
	return hex.EncodeToString(hash[:16]), nil
}
