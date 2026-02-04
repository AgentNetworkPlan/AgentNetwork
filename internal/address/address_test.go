package address

import (
	"testing"
	"time"
)

// 模拟声誉查询
type mockReputationGetter struct {
	reputations map[string]float64
}

func (m *mockReputationGetter) GetReputation(nodeID string) float64 {
	if rep, ok := m.reputations[nodeID]; ok {
		return rep
	}
	return 0
}

// 模拟签名验证（测试时总是通过）
type mockSignatureVerifier struct {
	shouldPass bool
}

func (m *mockSignatureVerifier) Verify(publicKey, message, signature string) bool {
	return m.shouldPass
}

func newTestRegistry() *Registry {
	repGetter := &mockReputationGetter{
		reputations: map[string]float64{
			"guarantor1": 300.0, // 高声誉担保人
			"guarantor2": 100.0, // 低声誉担保人
			"witness1":   200.0,
			"witness2":   200.0,
			"witness3":   200.0,
		},
	}
	sigVerifier := &mockSignatureVerifier{shouldPass: true}
	return NewRegistry(repGetter, sigVerifier)
}

func TestRegistry_Register(t *testing.T) {
	registry := newTestRegistry()

	// 正常注册
	req := &RegistrationRequest{
		PublicKey:    "test_public_key_123",
		NodeID:       "node_123",
		GuarantorID:  "guarantor1",
		GuarantorSig: "valid_sig",
	}

	result, err := registry.Register(req)
	if err != nil {
		t.Fatalf("Register failed: %v", err)
	}
	if !result.Success {
		t.Errorf("Expected success, got failure")
	}
	if result.AddressID == "" {
		t.Error("Expected address ID, got empty")
	}

	// 验证门牌号已创建
	addr, err := registry.GetAddress(result.AddressID)
	if err != nil {
		t.Fatalf("GetAddress failed: %v", err)
	}
	if addr.Status != AddressStatusPending {
		t.Errorf("Expected pending status, got %s", addr.Status)
	}
}

func TestRegistry_Register_DuplicateKey(t *testing.T) {
	registry := newTestRegistry()

	req := &RegistrationRequest{
		PublicKey:    "test_public_key",
		NodeID:       "node_1",
		GuarantorID:  "guarantor1",
		GuarantorSig: "sig",
	}

	// 第一次注册成功
	_, err := registry.Register(req)
	if err != nil {
		t.Fatalf("First register failed: %v", err)
	}

	// 第二次注册应失败（相同公钥）
	_, err = registry.Register(req)
	if err != ErrAddressExists {
		t.Errorf("Expected ErrAddressExists, got %v", err)
	}
}

func TestRegistry_Register_LowReputationGuarantor(t *testing.T) {
	registry := newTestRegistry()

	req := &RegistrationRequest{
		PublicKey:    "test_public_key",
		NodeID:       "node_1",
		GuarantorID:  "guarantor2", // 低声誉担保人
		GuarantorSig: "sig",
	}

	_, err := registry.Register(req)
	if err != ErrGuarantorLowRep {
		t.Errorf("Expected ErrGuarantorLowRep, got %v", err)
	}
}

func TestRegistry_Register_GuarantorLimit(t *testing.T) {
	registry := newTestRegistry()

	// 担保人担保3个节点（达到上限）
	for i := 0; i < MaxGuaranteesPerNode; i++ {
		req := &RegistrationRequest{
			PublicKey:    "pubkey_" + string(rune('A'+i)),
			NodeID:       "node_" + string(rune('A'+i)),
			GuarantorID:  "guarantor1",
			GuarantorSig: "sig",
		}
		_, err := registry.Register(req)
		if err != nil {
			t.Fatalf("Register %d failed: %v", i, err)
		}
	}

	// 第4个应该失败
	req := &RegistrationRequest{
		PublicKey:    "pubkey_D",
		NodeID:       "node_D",
		GuarantorID:  "guarantor1",
		GuarantorSig: "sig",
	}
	_, err := registry.Register(req)
	if err != ErrGuarantorLimitReached {
		t.Errorf("Expected ErrGuarantorLimitReached, got %v", err)
	}
}

func TestRegistry_AddWitness(t *testing.T) {
	registry := newTestRegistry()

	// 注册门牌号
	req := &RegistrationRequest{
		PublicKey:    "test_public_key",
		NodeID:       "node_1",
		GuarantorID:  "guarantor1",
		GuarantorSig: "sig",
	}
	result, _ := registry.Register(req)
	addressID := result.AddressID

	// 检查初始状态
	addr, _ := registry.GetAddress(addressID)
	if addr.Status != AddressStatusPending {
		t.Errorf("Expected pending, got %s", addr.Status)
	}

	// 添加第一个见证
	err := registry.AddWitness(addressID, "witness1", "sig1")
	if err != nil {
		t.Fatalf("AddWitness 1 failed: %v", err)
	}

	// 仍然是待确认状态
	addr, _ = registry.GetAddress(addressID)
	if addr.Status != AddressStatusPending {
		t.Errorf("Expected pending with 1 witness, got %s", addr.Status)
	}

	// 添加第二个见证
	err = registry.AddWitness(addressID, "witness2", "sig2")
	if err != nil {
		t.Fatalf("AddWitness 2 failed: %v", err)
	}

	// 现在应该是活跃状态
	addr, _ = registry.GetAddress(addressID)
	if addr.Status != AddressStatusActive {
		t.Errorf("Expected active with 2 witnesses, got %s", addr.Status)
	}
}

func TestRegistry_ValidateAddress(t *testing.T) {
	registry := newTestRegistry()

	// 注册并激活门牌号
	req := &RegistrationRequest{
		PublicKey:    "test_public_key",
		NodeID:       "node_1",
		GuarantorID:  "guarantor1",
		GuarantorSig: "sig",
	}
	result, _ := registry.Register(req)
	addressID := result.AddressID

	registry.AddWitness(addressID, "witness1", "sig1")
	registry.AddWitness(addressID, "witness2", "sig2")

	// 验证有效
	err := registry.ValidateAddress(addressID)
	if err != nil {
		t.Errorf("ValidateAddress failed: %v", err)
	}
}

func TestRegistry_RevokeAddress(t *testing.T) {
	registry := newTestRegistry()

	// 注册门牌号
	req := &RegistrationRequest{
		PublicKey:    "test_public_key",
		NodeID:       "node_1",
		GuarantorID:  "guarantor1",
		GuarantorSig: "sig",
	}
	result, _ := registry.Register(req)
	addressID := result.AddressID

	// 撤销
	err := registry.RevokeAddress(addressID, "test revoke")
	if err != nil {
		t.Fatalf("RevokeAddress failed: %v", err)
	}

	// 检查状态
	addr, _ := registry.GetAddress(addressID)
	if addr.Status != AddressStatusRevoked {
		t.Errorf("Expected revoked, got %s", addr.Status)
	}

	// 验证应该失败
	err = registry.ValidateAddress(addressID)
	if err != ErrAddressRevoked {
		t.Errorf("Expected ErrAddressRevoked, got %v", err)
	}
}

func TestRegistry_GetByNodeIDAndPublicKey(t *testing.T) {
	registry := newTestRegistry()

	req := &RegistrationRequest{
		PublicKey:    "test_public_key",
		NodeID:       "node_1",
		GuarantorID:  "guarantor1",
		GuarantorSig: "sig",
	}
	result, _ := registry.Register(req)

	// 通过节点ID查询
	addr1, err := registry.GetAddressByNodeID("node_1")
	if err != nil {
		t.Fatalf("GetAddressByNodeID failed: %v", err)
	}
	if addr1.ID != result.AddressID {
		t.Error("Address ID mismatch")
	}

	// 通过公钥查询
	addr2, err := registry.GetAddressByPublicKey("test_public_key")
	if err != nil {
		t.Fatalf("GetAddressByPublicKey failed: %v", err)
	}
	if addr2.ID != result.AddressID {
		t.Error("Address ID mismatch")
	}
}

func TestRegistry_UpdateLastSeen(t *testing.T) {
	registry := newTestRegistry()

	req := &RegistrationRequest{
		PublicKey:    "test_public_key",
		NodeID:       "node_1",
		GuarantorID:  "guarantor1",
		GuarantorSig: "sig",
	}
	result, _ := registry.Register(req)

	// 更新最后活跃时间
	time.Sleep(10 * time.Millisecond) // 确保时间有变化
	registry.UpdateLastSeen(result.AddressID)

	addr, _ := registry.GetAddress(result.AddressID)
	if addr.LastSeen.Before(addr.RegisteredAt) {
		t.Error("LastSeen should be updated")
	}
}

func TestRegistry_GetAddressInfo(t *testing.T) {
	registry := newTestRegistry()

	req := &RegistrationRequest{
		PublicKey:    "test_public_key",
		NodeID:       "node_1",
		GuarantorID:  "guarantor1",
		GuarantorSig: "sig",
	}
	result, _ := registry.Register(req)
	registry.AddWitness(result.AddressID, "witness1", "sig1")
	registry.AddWitness(result.AddressID, "witness2", "sig2")

	info, err := registry.GetAddressInfo(result.AddressID)
	if err != nil {
		t.Fatalf("GetAddressInfo failed: %v", err)
	}

	if info.Status != AddressStatusActive {
		t.Errorf("Expected active, got %s", info.Status)
	}
	if info.WitnessCount != 2 {
		t.Errorf("Expected 2 witnesses, got %d", info.WitnessCount)
	}
	if !info.IsValid {
		t.Error("Expected valid address")
	}
}

func TestGenerateAddressID(t *testing.T) {
	id1 := generateAddressID("pubkey1")
	id2 := generateAddressID("pubkey2")
	id3 := generateAddressID("pubkey1") // 相同公钥

	if id1 == id2 {
		t.Error("Different public keys should generate different IDs")
	}
	if id1 != id3 {
		t.Error("Same public key should generate same ID")
	}
	if len(id1) != 32 { // 16字节 = 32个十六进制字符
		t.Errorf("Expected 32 char ID, got %d", len(id1))
	}
}
