package auth

import (
	"crypto/rand"
	"encoding/hex"
	"errors"
	"fmt"
	"math/big"
	"os"
	"path/filepath"
	"sync"
	"time"

	"github.com/tjfoc/gmsm/sm2"
	"github.com/tjfoc/gmsm/sm3"
)

// NodeIdentity 节点身份，基于 SM2 公钥
type NodeIdentity struct {
	privateKey *sm2.PrivateKey
	publicKey  *sm2.PublicKey
	nodeID     string // 公钥的哈希，作为唯一标识
	mu         sync.RWMutex
}

// NewNodeIdentity 创建新的节点身份
func NewNodeIdentity() (*NodeIdentity, error) {
	priv, err := sm2.GenerateKey(rand.Reader)
	if err != nil {
		return nil, fmt.Errorf("生成 SM2 密钥失败: %w", err)
	}

	nodeID := computeNodeID(&priv.PublicKey)

	return &NodeIdentity{
		privateKey: priv,
		publicKey:  &priv.PublicKey,
		nodeID:     nodeID,
	}, nil
}

// LoadIdentity 从文件加载节点身份
func LoadIdentity(keyPath string) (*NodeIdentity, error) {
	data, err := os.ReadFile(keyPath)
	if err != nil {
		return nil, fmt.Errorf("读取密钥文件失败: %w", err)
	}

	// 解析私钥 (hex 格式)
	privBytes, err := hex.DecodeString(string(data))
	if err != nil {
		return nil, fmt.Errorf("解析密钥失败: %w", err)
	}

	// 重建私钥
	priv := new(sm2.PrivateKey)
	priv.Curve = sm2.P256Sm2()
	priv.D = new(big.Int).SetBytes(privBytes)
	priv.PublicKey.X, priv.PublicKey.Y = priv.Curve.ScalarBaseMult(privBytes)

	nodeID := computeNodeID(&priv.PublicKey)

	return &NodeIdentity{
		privateKey: priv,
		publicKey:  &priv.PublicKey,
		nodeID:     nodeID,
	}, nil
}

// LoadOrCreate 加载或创建节点身份
func LoadOrCreate(keyPath string) (*NodeIdentity, error) {
	if _, err := os.Stat(keyPath); os.IsNotExist(err) {
		// 创建新身份
		identity, err := NewNodeIdentity()
		if err != nil {
			return nil, err
		}
		// 保存到文件
		if err := identity.Save(keyPath); err != nil {
			return nil, err
		}
		return identity, nil
	}
	return LoadIdentity(keyPath)
}

// Save 保存节点身份到文件
func (n *NodeIdentity) Save(keyPath string) error {
	n.mu.RLock()
	defer n.mu.RUnlock()

	// 确保目录存在
	dir := filepath.Dir(keyPath)
	if err := os.MkdirAll(dir, 0700); err != nil {
		return fmt.Errorf("创建目录失败: %w", err)
	}

	// 保存私钥 (hex 格式)
	privHex := hex.EncodeToString(n.privateKey.D.Bytes())

	if err := os.WriteFile(keyPath, []byte(privHex), 0600); err != nil {
		return fmt.Errorf("写入密钥文件失败: %w", err)
	}

	return nil
}

// NodeID 返回节点 ID
func (n *NodeIdentity) NodeID() string {
	n.mu.RLock()
	defer n.mu.RUnlock()
	return n.nodeID
}

// ShortID 返回短格式节点 ID
func (n *NodeIdentity) ShortID() string {
	n.mu.RLock()
	defer n.mu.RUnlock()
	if len(n.nodeID) > 12 {
		return n.nodeID[:12]
	}
	return n.nodeID
}

// PublicKeyHex 返回公钥的 hex 编码
func (n *NodeIdentity) PublicKeyHex() string {
	n.mu.RLock()
	defer n.mu.RUnlock()
	return hex.EncodeToString(sm2.Compress(n.publicKey))
}

// Sign 对数据进行签名
func (n *NodeIdentity) Sign(data []byte) ([]byte, error) {
	n.mu.RLock()
	defer n.mu.RUnlock()

	// 使用 SM3 哈希
	hash := sm3.Sm3Sum(data)
	
	sig, err := n.privateKey.Sign(rand.Reader, hash[:], nil)
	if err != nil {
		return nil, fmt.Errorf("签名失败: %w", err)
	}

	return sig, nil
}

// Verify 验证签名
func (n *NodeIdentity) Verify(data, signature []byte) bool {
	n.mu.RLock()
	defer n.mu.RUnlock()

	hash := sm3.Sm3Sum(data)
	return n.publicKey.Verify(hash[:], signature)
}

// VerifyWithPublicKey 使用指定公钥验证签名
func VerifyWithPublicKey(pubKeyHex string, data, signature []byte) (bool, error) {
	pubKeyBytes, err := hex.DecodeString(pubKeyHex)
	if err != nil {
		return false, fmt.Errorf("解析公钥失败: %w", err)
	}

	pubKey := sm2.Decompress(pubKeyBytes)
	if pubKey == nil {
		return false, errors.New("解压公钥失败")
	}

	hash := sm3.Sm3Sum(data)
	return pubKey.Verify(hash[:], signature), nil
}

// computeNodeID 计算节点 ID (公钥的 SM3 哈希)
func computeNodeID(pubKey *sm2.PublicKey) string {
	pubBytes := sm2.Compress(pubKey)
	hash := sm3.Sm3Sum(pubBytes)
	return hex.EncodeToString(hash[:16]) // 取前 16 字节作为 NodeID
}

// ========== 认证挑战机制 ==========

// Challenge 认证挑战
type Challenge struct {
	ChallengeID string    `json:"challenge_id"`
	NodeID      string    `json:"node_id"`
	Nonce       []byte    `json:"nonce"`
	Timestamp   time.Time `json:"timestamp"`
	ExpiresAt   time.Time `json:"expires_at"`
}

// ChallengeResponse 挑战响应
type ChallengeResponse struct {
	ChallengeID string `json:"challenge_id"`
	NodeID      string `json:"node_id"`
	Signature   []byte `json:"signature"`
	PublicKey   string `json:"public_key"`
}

// NewChallenge 创建新的认证挑战
func NewChallenge(nodeID string, ttl time.Duration) (*Challenge, error) {
	nonce := make([]byte, 32)
	if _, err := rand.Read(nonce); err != nil {
		return nil, fmt.Errorf("生成随机数失败: %w", err)
	}

	challengeID := make([]byte, 16)
	rand.Read(challengeID)

	now := time.Now()

	return &Challenge{
		ChallengeID: hex.EncodeToString(challengeID),
		NodeID:      nodeID,
		Nonce:       nonce,
		Timestamp:   now,
		ExpiresAt:   now.Add(ttl),
	}, nil
}

// IsExpired 检查挑战是否过期
func (c *Challenge) IsExpired() bool {
	return time.Now().After(c.ExpiresAt)
}

// SignableData 返回用于签名的数据
func (c *Challenge) SignableData() []byte {
	data := append([]byte(c.ChallengeID), c.Nonce...)
	data = append(data, []byte(c.NodeID)...)
	return data
}

// RespondToChallenge 响应认证挑战
func (n *NodeIdentity) RespondToChallenge(challenge *Challenge) (*ChallengeResponse, error) {
	if challenge.IsExpired() {
		return nil, errors.New("挑战已过期")
	}

	if challenge.NodeID != n.nodeID {
		return nil, errors.New("挑战不属于此节点")
	}

	sig, err := n.Sign(challenge.SignableData())
	if err != nil {
		return nil, fmt.Errorf("签名失败: %w", err)
	}

	return &ChallengeResponse{
		ChallengeID: challenge.ChallengeID,
		NodeID:      n.nodeID,
		Signature:   sig,
		PublicKey:   n.PublicKeyHex(),
	}, nil
}

// VerifyChallengeResponse 验证挑战响应
func VerifyChallengeResponse(challenge *Challenge, response *ChallengeResponse) (bool, error) {
	if challenge.IsExpired() {
		return false, errors.New("挑战已过期")
	}

	if challenge.ChallengeID != response.ChallengeID {
		return false, errors.New("挑战 ID 不匹配")
	}

	if challenge.NodeID != response.NodeID {
		return false, errors.New("节点 ID 不匹配")
	}

	// 验证签名
	return VerifyWithPublicKey(response.PublicKey, challenge.SignableData(), response.Signature)
}
