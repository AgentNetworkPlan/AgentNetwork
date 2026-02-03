package auth

import (
	"encoding/hex"
	"testing"
	"time"
)

func TestNewNodeIdentity(t *testing.T) {
	identity, err := NewNodeIdentity()
	if err != nil {
		t.Fatalf("创建节点身份失败: %v", err)
	}

	if identity.NodeID() == "" {
		t.Error("节点 ID 不应为空")
	}

	t.Logf("节点 ID: %s", identity.NodeID())
}

func TestNodeIdentity_Sign(t *testing.T) {
	identity, err := NewNodeIdentity()
	if err != nil {
		t.Fatalf("创建节点身份失败: %v", err)
	}

	data := []byte("test data for signing")
	signature, err := identity.Sign(data)
	if err != nil {
		t.Fatalf("签名失败: %v", err)
	}

	if len(signature) == 0 {
		t.Error("签名不应为空")
	}

	t.Logf("签名长度: %d bytes", len(signature))
}

func TestNodeIdentity_Verify(t *testing.T) {
	identity, err := NewNodeIdentity()
	if err != nil {
		t.Fatalf("创建节点身份失败: %v", err)
	}

	data := []byte("test data for verification")
	signature, err := identity.Sign(data)
	if err != nil {
		t.Fatalf("签名失败: %v", err)
	}

	// 正确数据验证
	if !identity.Verify(data, signature) {
		t.Error("有效签名验证失败")
	}

	// 篡改数据验证
	tamperedData := []byte("tampered data")
	if identity.Verify(tamperedData, signature) {
		t.Error("篡改数据不应通过验证")
	}
}

func TestNodeIdentity_VerifyWithPublicKey(t *testing.T) {
	identity, err := NewNodeIdentity()
	if err != nil {
		t.Fatalf("创建节点身份失败: %v", err)
	}

	data := []byte("test data")
	signature, err := identity.Sign(data)
	if err != nil {
		t.Fatalf("签名失败: %v", err)
	}

	// 使用公钥 hex 验证
	pubKeyHex := identity.PublicKeyHex()
	valid, err := VerifyWithPublicKey(pubKeyHex, data, signature)
	if err != nil {
		t.Fatalf("验证失败: %v", err)
	}
	if !valid {
		t.Error("使用公钥验证失败")
	}
}

func TestNodeIdentity_ChallengeResponse(t *testing.T) {
	identity, err := NewNodeIdentity()
	if err != nil {
		t.Fatalf("创建节点身份失败: %v", err)
	}

	// 创建挑战
	challenge, err := NewChallenge(identity.NodeID(), 5*time.Minute)
	if err != nil {
		t.Fatalf("创建挑战失败: %v", err)
	}

	if len(challenge.Nonce) != 32 {
		t.Errorf("挑战 Nonce 长度应为 32，实际为 %d", len(challenge.Nonce))
	}

	// 响应挑战
	response, err := identity.RespondToChallenge(challenge)
	if err != nil {
		t.Fatalf("响应挑战失败: %v", err)
	}

	// 验证响应
	valid, err := VerifyChallengeResponse(challenge, response)
	if err != nil {
		t.Fatalf("验证挑战响应失败: %v", err)
	}
	if !valid {
		t.Error("挑战响应验证失败")
	}
}

func TestNodeIdentity_ShortID(t *testing.T) {
	identity, err := NewNodeIdentity()
	if err != nil {
		t.Fatalf("创建节点身份失败: %v", err)
	}

	shortID := identity.ShortID()
	if len(shortID) > 12 {
		t.Errorf("ShortID 长度应不超过 12，实际为 %d", len(shortID))
	}
}

func TestGenerateNodeID(t *testing.T) {
	identity1, _ := NewNodeIdentity()
	identity2, _ := NewNodeIdentity()

	if identity1.NodeID() == identity2.NodeID() {
		t.Error("两个不同身份的节点 ID 应该不同")
	}

	// 节点 ID 应该是十六进制字符串
	_, err := hex.DecodeString(identity1.NodeID())
	if err != nil {
		t.Errorf("节点 ID 应该是有效的十六进制字符串: %v", err)
	}
}

func TestChallenge_IsExpired(t *testing.T) {
	identity, _ := NewNodeIdentity()
	
	// 创建一个很短有效期的挑战
	challenge, _ := NewChallenge(identity.NodeID(), 1*time.Millisecond)
	
	// 等待过期
	time.Sleep(10 * time.Millisecond)
	
	if !challenge.IsExpired() {
		t.Error("挑战应该已过期")
	}
}
