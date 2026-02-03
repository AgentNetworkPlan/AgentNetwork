package identity

import (
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"os"
	"path/filepath"

	"github.com/libp2p/go-libp2p/core/crypto"
	"github.com/libp2p/go-libp2p/core/peer"
)

// Identity 节点身份信息
type Identity struct {
	PrivKey crypto.PrivKey
	PubKey  crypto.PubKey
	PeerID  peer.ID
}

// NewIdentity 生成新的节点身份
func NewIdentity() (*Identity, error) {
	priv, pub, err := crypto.GenerateKeyPairWithReader(crypto.Ed25519, -1, rand.Reader)
	if err != nil {
		return nil, fmt.Errorf("生成密钥对失败: %w", err)
	}

	peerID, err := peer.IDFromPublicKey(pub)
	if err != nil {
		return nil, fmt.Errorf("生成 PeerID 失败: %w", err)
	}

	return &Identity{
		PrivKey: priv,
		PubKey:  pub,
		PeerID:  peerID,
	}, nil
}

// LoadOrCreate 从文件加载或创建新身份
func LoadOrCreate(keyPath string) (*Identity, error) {
	// 尝试加载现有密钥
	if data, err := os.ReadFile(keyPath); err == nil {
		priv, err := crypto.UnmarshalPrivateKey(data)
		if err != nil {
			return nil, fmt.Errorf("解析私钥失败: %w", err)
		}

		pub := priv.GetPublic()
		peerID, err := peer.IDFromPublicKey(pub)
		if err != nil {
			return nil, fmt.Errorf("生成 PeerID 失败: %w", err)
		}

		return &Identity{
			PrivKey: priv,
			PubKey:  pub,
			PeerID:  peerID,
		}, nil
	}

	// 创建新身份
	id, err := NewIdentity()
	if err != nil {
		return nil, err
	}

	// 保存到文件
	if err := id.Save(keyPath); err != nil {
		return nil, fmt.Errorf("保存密钥失败: %w", err)
	}

	return id, nil
}

// Save 保存私钥到文件
func (id *Identity) Save(keyPath string) error {
	// 确保目录存在
	dir := filepath.Dir(keyPath)
	if err := os.MkdirAll(dir, 0700); err != nil {
		return fmt.Errorf("创建目录失败: %w", err)
	}

	data, err := crypto.MarshalPrivateKey(id.PrivKey)
	if err != nil {
		return fmt.Errorf("序列化私钥失败: %w", err)
	}

	if err := os.WriteFile(keyPath, data, 0600); err != nil {
		return fmt.Errorf("写入文件失败: %w", err)
	}

	return nil
}

// String 返回身份的字符串表示
func (id *Identity) String() string {
	return id.PeerID.String()
}

// ShortID 返回短格式的 PeerID
func (id *Identity) ShortID() string {
	s := id.PeerID.String()
	if len(s) > 12 {
		return s[:6] + "..." + s[len(s)-6:]
	}
	return s
}

// PublicKeyHex 返回公钥的十六进制表示
func (id *Identity) PublicKeyHex() (string, error) {
	data, err := crypto.MarshalPublicKey(id.PubKey)
	if err != nil {
		return "", err
	}
	return hex.EncodeToString(data), nil
}
