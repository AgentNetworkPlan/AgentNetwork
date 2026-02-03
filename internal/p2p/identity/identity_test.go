package identity

import (
	"os"
	"path/filepath"
	"testing"
)

func TestNewIdentity(t *testing.T) {
	id, err := NewIdentity()
	if err != nil {
		t.Fatalf("创建身份失败: %v", err)
	}

	if id.PrivKey == nil {
		t.Error("私钥为空")
	}

	if id.PubKey == nil {
		t.Error("公钥为空")
	}

	if id.PeerID == "" {
		t.Error("PeerID 为空")
	}

	t.Logf("生成的 PeerID: %s", id.PeerID)
}

func TestIdentity_SaveAndLoad(t *testing.T) {
	// 创建临时目录
	tmpDir := t.TempDir()
	keyPath := filepath.Join(tmpDir, "test.key")

	// 创建新身份
	id1, err := NewIdentity()
	if err != nil {
		t.Fatalf("创建身份失败: %v", err)
	}

	// 保存
	if err := id1.Save(keyPath); err != nil {
		t.Fatalf("保存身份失败: %v", err)
	}

	// 检查文件是否存在
	if _, err := os.Stat(keyPath); os.IsNotExist(err) {
		t.Fatal("密钥文件不存在")
	}

	// 加载
	id2, err := LoadOrCreate(keyPath)
	if err != nil {
		t.Fatalf("加载身份失败: %v", err)
	}

	// 验证 PeerID 一致
	if id1.PeerID != id2.PeerID {
		t.Errorf("PeerID 不一致: %s != %s", id1.PeerID, id2.PeerID)
	}
}

func TestIdentity_LoadOrCreate_New(t *testing.T) {
	tmpDir := t.TempDir()
	keyPath := filepath.Join(tmpDir, "new.key")

	// 确保文件不存在
	os.Remove(keyPath)

	// LoadOrCreate 应该创建新身份
	id, err := LoadOrCreate(keyPath)
	if err != nil {
		t.Fatalf("LoadOrCreate 失败: %v", err)
	}

	if id.PeerID == "" {
		t.Error("PeerID 为空")
	}

	// 文件应该已创建
	if _, err := os.Stat(keyPath); os.IsNotExist(err) {
		t.Error("密钥文件未创建")
	}
}

func TestIdentity_ShortID(t *testing.T) {
	id, err := NewIdentity()
	if err != nil {
		t.Fatalf("创建身份失败: %v", err)
	}

	shortID := id.ShortID()
	if len(shortID) == 0 {
		t.Error("ShortID 为空")
	}

	// ShortID 应该包含 "..."
	if len(id.PeerID.String()) > 12 && len(shortID) < len(id.PeerID.String()) {
		t.Logf("ShortID: %s (原始: %s)", shortID, id.PeerID.String())
	}
}

func TestIdentity_PublicKeyHex(t *testing.T) {
	id, err := NewIdentity()
	if err != nil {
		t.Fatalf("创建身份失败: %v", err)
	}

	hex, err := id.PublicKeyHex()
	if err != nil {
		t.Fatalf("获取公钥 Hex 失败: %v", err)
	}

	if len(hex) == 0 {
		t.Error("公钥 Hex 为空")
	}

	t.Logf("公钥 Hex 长度: %d", len(hex))
}
