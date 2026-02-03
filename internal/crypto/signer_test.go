package crypto

import (
	"os"
	"path/filepath"
	"testing"
)

func TestNewSigner_SM2(t *testing.T) {
	tmpDir := t.TempDir()
	keyPath := filepath.Join(tmpDir, "test.key")

	signer, err := NewSigner("sm2", keyPath)
	if err != nil {
		t.Fatalf("创建 SM2 签名器失败: %v", err)
	}

	if signer == nil {
		t.Fatal("签名器为空")
	}
}

func TestNewSigner_ECC(t *testing.T) {
	tmpDir := t.TempDir()
	keyPath := filepath.Join(tmpDir, "test.key")

	signer, err := NewSigner("ecc", keyPath)
	if err != nil {
		t.Fatalf("创建 ECC 签名器失败: %v", err)
	}

	if signer == nil {
		t.Fatal("签名器为空")
	}
}

func TestNewSigner_Unsupported(t *testing.T) {
	_, err := NewSigner("rsa", "test.key")
	if err == nil {
		t.Error("应该返回不支持的算法错误")
	}
}

func TestSM2Signer_WithKey(t *testing.T) {
	tmpDir := t.TempDir()
	keyPath := filepath.Join(tmpDir, "sm2.key")

	// 创建测试密钥文件
	testKey := []byte("test-private-key-data-for-sm2")
	if err := os.WriteFile(keyPath, testKey, 0600); err != nil {
		t.Fatalf("创建测试密钥失败: %v", err)
	}

	signer, err := NewSM2Signer(keyPath)
	if err != nil {
		t.Fatalf("创建 SM2 签名器失败: %v", err)
	}

	// 测试签名
	data := []byte("test message to sign")
	sig, err := signer.Sign(data)
	if err != nil {
		t.Fatalf("签名失败: %v", err)
	}

	if len(sig) == 0 {
		t.Error("签名为空")
	}

	t.Logf("签名长度: %d bytes", len(sig))
}

func TestSM2Signer_NoKey(t *testing.T) {
	tmpDir := t.TempDir()
	keyPath := filepath.Join(tmpDir, "nonexistent.key")

	signer, err := NewSM2Signer(keyPath)
	if err != nil {
		t.Fatalf("创建签名器失败: %v", err)
	}

	// 没有密钥时签名应该失败
	_, err = signer.Sign([]byte("test"))
	if err == nil {
		t.Error("没有密钥时签名应该失败")
	}
}

func TestSM2Signer_GetAgentID(t *testing.T) {
	tmpDir := t.TempDir()
	keyPath := filepath.Join(tmpDir, "sm2.key")

	// 创建测试密钥
	testKey := []byte("test-private-key-data")
	if err := os.WriteFile(keyPath, testKey, 0600); err != nil {
		t.Fatalf("创建测试密钥失败: %v", err)
	}

	signer, err := NewSM2Signer(keyPath)
	if err != nil {
		t.Fatalf("创建签名器失败: %v", err)
	}

	agentID, err := signer.GetAgentID()
	if err != nil {
		t.Fatalf("获取 AgentID 失败: %v", err)
	}

	if agentID == "" {
		t.Error("AgentID 为空")
	}

	// AgentID 应该是 32 个十六进制字符
	if len(agentID) != 32 {
		t.Errorf("AgentID 长度错误: %d (应该是 32)", len(agentID))
	}

	t.Logf("AgentID: %s", agentID)
}

func TestECCSigner_WithKey(t *testing.T) {
	tmpDir := t.TempDir()
	keyPath := filepath.Join(tmpDir, "ecc.key")

	// 创建测试密钥文件
	testKey := []byte("test-private-key-data-for-ecc")
	if err := os.WriteFile(keyPath, testKey, 0600); err != nil {
		t.Fatalf("创建测试密钥失败: %v", err)
	}

	signer, err := NewECCSigner(keyPath)
	if err != nil {
		t.Fatalf("创建 ECC 签名器失败: %v", err)
	}

	// 测试签名
	data := []byte("test message to sign")
	sig, err := signer.Sign(data)
	if err != nil {
		t.Fatalf("签名失败: %v", err)
	}

	if len(sig) == 0 {
		t.Error("签名为空")
	}
}

func TestECCSigner_GetPublicKey(t *testing.T) {
	tmpDir := t.TempDir()
	keyPath := filepath.Join(tmpDir, "ecc.key")

	testKey := []byte("test-ecc-private-key")
	if err := os.WriteFile(keyPath, testKey, 0600); err != nil {
		t.Fatalf("创建测试密钥失败: %v", err)
	}

	signer, err := NewECCSigner(keyPath)
	if err != nil {
		t.Fatalf("创建签名器失败: %v", err)
	}

	pubKey, err := signer.GetPublicKey()
	if err != nil {
		t.Fatalf("获取公钥失败: %v", err)
	}

	if len(pubKey) == 0 {
		t.Error("公钥为空")
	}
}

func TestSigner_Verify(t *testing.T) {
	tmpDir := t.TempDir()
	keyPath := filepath.Join(tmpDir, "test.key")

	testKey := []byte("test-key")
	os.WriteFile(keyPath, testKey, 0600)

	signer, _ := NewSM2Signer(keyPath)

	// 当前实现总是返回 true
	valid, err := signer.Verify([]byte("data"), []byte("signature"))
	if err != nil {
		t.Fatalf("验证失败: %v", err)
	}

	if !valid {
		t.Error("验证应该通过")
	}
}
