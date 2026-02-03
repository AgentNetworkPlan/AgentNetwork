package genesis

import (
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/tjfoc/gmsm/sm2"
)

func TestNewGenesisManager(t *testing.T) {
	tempDir := t.TempDir()

	gm, err := NewGenesisManager(tempDir)
	if err != nil {
		t.Fatalf("创建 GenesisManager 失败: %v", err)
	}

	if gm == nil {
		t.Fatal("GenesisManager 为 nil")
	}

	if gm.genesis != nil {
		t.Error("新建的 GenesisManager 不应有创世信息")
	}
}

func TestInitGenesis(t *testing.T) {
	tempDir := t.TempDir()

	gm, err := NewGenesisManager(tempDir)
	if err != nil {
		t.Fatalf("创建 GenesisManager 失败: %v", err)
	}

	genesis, err := gm.InitGenesis("TestNetwork", "1.0.0")
	if err != nil {
		t.Fatalf("初始化创世信息失败: %v", err)
	}

	// 验证创世信息
	if genesis.NetworkName != "TestNetwork" {
		t.Errorf("网络名称错误: got %s, want TestNetwork", genesis.NetworkName)
	}

	if genesis.NetworkVersion != "1.0.0" {
		t.Errorf("网络版本错误: got %s, want 1.0.0", genesis.NetworkVersion)
	}

	if genesis.GenesisNodeID == "" {
		t.Error("创世节点ID为空")
	}

	if genesis.GenesisKey == "" {
		t.Error("创世节点公钥为空")
	}

	if genesis.Signature == "" {
		t.Error("创世签名为空")
	}

	// 验证默认配置
	if genesis.InitialReputation != 1 {
		t.Errorf("初始声誉错误: got %d, want 1", genesis.InitialReputation)
	}

	if genesis.MinInviterReputation != 10 {
		t.Errorf("最低邀请声誉错误: got %d, want 10", genesis.MinInviterReputation)
	}

	// 验证创世节点已加入
	if !gm.IsNodeJoined(genesis.GenesisNodeID) {
		t.Error("创世节点应该已加入网络")
	}

	// 验证创世节点声誉
	rep, err := gm.GetNodeReputation(genesis.GenesisNodeID)
	if err != nil {
		t.Fatalf("获取创世节点声誉失败: %v", err)
	}
	if rep != 100 {
		t.Errorf("创世节点声誉错误: got %d, want 100", rep)
	}

	// 验证文件已保存
	genesisPath := filepath.Join(tempDir, "genesis.json")
	if _, err := os.Stat(genesisPath); os.IsNotExist(err) {
		t.Error("创世信息文件未保存")
	}

	keyPath := filepath.Join(tempDir, "node_key.hex")
	if _, err := os.Stat(keyPath); os.IsNotExist(err) {
		t.Error("节点密钥文件未保存")
	}
}

func TestInitGenesisAlreadyExists(t *testing.T) {
	tempDir := t.TempDir()

	gm, _ := NewGenesisManager(tempDir)
	gm.InitGenesis("TestNetwork", "1.0.0")

	// 再次初始化应该失败
	_, err := gm.InitGenesis("TestNetwork2", "2.0.0")
	if err != ErrGenesisAlreadyExists {
		t.Errorf("预期 ErrGenesisAlreadyExists, got %v", err)
	}
}

func TestLoadGenesis(t *testing.T) {
	tempDir := t.TempDir()

	// 创建创世信息
	gm1, _ := NewGenesisManager(tempDir)
	genesis, _ := gm1.InitGenesis("TestNetwork", "1.0.0")

	// 序列化创世信息
	genesisJSON, _ := json.Marshal(genesis)

	// 创建新的管理器并加载
	gm2, _ := NewGenesisManager(t.TempDir())
	err := gm2.LoadGenesis(genesisJSON)
	if err != nil {
		t.Fatalf("加载创世信息失败: %v", err)
	}

	// 验证加载的信息
	loaded := gm2.GetGenesis()
	if loaded.NetworkName != genesis.NetworkName {
		t.Errorf("网络名称不匹配: got %s, want %s", loaded.NetworkName, genesis.NetworkName)
	}
}

func TestLoadGenesisInvalidSignature(t *testing.T) {
	tempDir := t.TempDir()

	gm1, _ := NewGenesisManager(tempDir)
	genesis, _ := gm1.InitGenesis("TestNetwork", "1.0.0")

	// 篡改签名
	genesis.Signature = "invalid_signature"
	genesisJSON, _ := json.Marshal(genesis)

	gm2, _ := NewGenesisManager(t.TempDir())
	err := gm2.LoadGenesis(genesisJSON)
	if err == nil {
		t.Error("加载篡改的创世信息应该失败")
	}
}

func TestCreateInvitation(t *testing.T) {
	tempDir := t.TempDir()

	gm, _ := NewGenesisManager(tempDir)
	gm.InitGenesis("TestNetwork", "1.0.0")

	// 生成新节点密钥
	newPriv, _ := sm2.GenerateKey(rand.Reader)
	newPubKeyHex := hex.EncodeToString(sm2.Compress(&newPriv.PublicKey))

	// 创建邀请函
	invitation, err := gm.CreateInvitation(newPubKeyHex)
	if err != nil {
		t.Fatalf("创建邀请函失败: %v", err)
	}

	// 验证邀请函
	if invitation.InviterNodeID != gm.GetNodeID() {
		t.Errorf("邀请者ID不匹配")
	}

	if invitation.NewNodeKey != newPubKeyHex {
		t.Errorf("新节点公钥不匹配")
	}

	if invitation.Signature == "" {
		t.Error("邀请函签名为空")
	}

	// 验证邀请函
	err = gm.VerifyInvitation(invitation)
	if err != nil {
		t.Fatalf("验证邀请函失败: %v", err)
	}
}

func TestVerifyInvitationExpired(t *testing.T) {
	tempDir := t.TempDir()

	gm, _ := NewGenesisManager(tempDir)
	gm.InitGenesis("TestNetwork", "1.0.0")

	newPriv, _ := sm2.GenerateKey(rand.Reader)
	newPubKeyHex := hex.EncodeToString(sm2.Compress(&newPriv.PublicKey))

	invitation, _ := gm.CreateInvitation(newPubKeyHex)

	// 手动设置过期时间
	invitation.ExpiresAt = time.Now().Add(-1 * time.Hour).UnixMilli()

	err := gm.VerifyInvitation(invitation)
	if err != ErrInvitationExpired {
		t.Errorf("预期 ErrInvitationExpired, got %v", err)
	}
}

func TestProcessJoinRequest(t *testing.T) {
	tempDir := t.TempDir()

	gm, _ := NewGenesisManager(tempDir)
	gm.InitGenesis("TestNetwork", "1.0.0")

	// 生成新节点密钥
	newPriv, _ := sm2.GenerateKey(rand.Reader)
	newPubKey := &newPriv.PublicKey
	newPubKeyHex := hex.EncodeToString(sm2.Compress(newPubKey))
	newNodeID := generateNodeID(newPubKey)

	// 创建邀请函
	invitation, _ := gm.CreateInvitation(newPubKeyHex)

	// 创建加入请求
	joinReq := &JoinRequest{
		NewNodeID:  newNodeID,
		NewNodeKey: newPubKeyHex,
		Invitation: invitation,
		Timestamp:  time.Now().UnixMilli(),
	}

	// 处理加入请求
	resp, err := gm.ProcessJoinRequest(joinReq)
	if err != nil {
		t.Fatalf("处理加入请求失败: %v", err)
	}

	// 验证响应
	if !resp.Accepted {
		t.Errorf("加入请求应该被接受, reason: %s", resp.Reason)
	}

	if resp.NodeID != newNodeID {
		t.Errorf("节点ID不匹配: got %s, want %s", resp.NodeID, newNodeID)
	}

	if resp.InitReputation != 1 {
		t.Errorf("初始声誉错误: got %d, want 1", resp.InitReputation)
	}

	// 验证节点已加入
	if !gm.IsNodeJoined(newNodeID) {
		t.Error("新节点应该已加入网络")
	}

	// 验证邻居推荐
	if len(resp.Neighbors) == 0 {
		t.Log("警告: 没有推荐邻居（可能是因为只有创世节点）")
	}
}

func TestProcessJoinRequestNodeAlreadyJoined(t *testing.T) {
	tempDir := t.TempDir()

	gm, _ := NewGenesisManager(tempDir)
	gm.InitGenesis("TestNetwork", "1.0.0")

	newPriv, _ := sm2.GenerateKey(rand.Reader)
	newPubKey := &newPriv.PublicKey
	newPubKeyHex := hex.EncodeToString(sm2.Compress(newPubKey))
	newNodeID := generateNodeID(newPubKey)

	invitation, _ := gm.CreateInvitation(newPubKeyHex)

	joinReq := &JoinRequest{
		NewNodeID:  newNodeID,
		NewNodeKey: newPubKeyHex,
		Invitation: invitation,
		Timestamp:  time.Now().UnixMilli(),
	}

	// 第一次加入
	gm.ProcessJoinRequest(joinReq)

	// 第二次加入应该失败
	_, err := gm.ProcessJoinRequest(joinReq)
	if err != ErrNodeAlreadyJoined {
		t.Errorf("预期 ErrNodeAlreadyJoined, got %v", err)
	}
}

func TestUpdateNodeReputation(t *testing.T) {
	tempDir := t.TempDir()

	gm, _ := NewGenesisManager(tempDir)
	genesis, _ := gm.InitGenesis("TestNetwork", "1.0.0")

	// 增加声誉
	err := gm.UpdateNodeReputation(genesis.GenesisNodeID, 10)
	if err != nil {
		t.Fatalf("更新声誉失败: %v", err)
	}

	rep, _ := gm.GetNodeReputation(genesis.GenesisNodeID)
	if rep != 110 {
		t.Errorf("声誉更新错误: got %d, want 110", rep)
	}

	// 减少声誉
	err = gm.UpdateNodeReputation(genesis.GenesisNodeID, -50)
	if err != nil {
		t.Fatalf("更新声誉失败: %v", err)
	}

	rep, _ = gm.GetNodeReputation(genesis.GenesisNodeID)
	if rep != 60 {
		t.Errorf("声誉更新错误: got %d, want 60", rep)
	}

	// 声誉不能为负
	err = gm.UpdateNodeReputation(genesis.GenesisNodeID, -100)
	if err != nil {
		t.Fatalf("更新声誉失败: %v", err)
	}

	rep, _ = gm.GetNodeReputation(genesis.GenesisNodeID)
	if rep != 0 {
		t.Errorf("声誉应该为0: got %d", rep)
	}
}

func TestGetJoinedNodes(t *testing.T) {
	tempDir := t.TempDir()

	gm, _ := NewGenesisManager(tempDir)
	gm.InitGenesis("TestNetwork", "1.0.0")

	nodes := gm.GetJoinedNodes()
	if len(nodes) != 1 {
		t.Errorf("节点数量错误: got %d, want 1", len(nodes))
	}
}

func TestPersistence(t *testing.T) {
	tempDir := t.TempDir()

	// 创建并初始化
	gm1, _ := NewGenesisManager(tempDir)
	genesis, _ := gm1.InitGenesis("TestNetwork", "1.0.0")

	// 添加一个新节点
	newPriv, _ := sm2.GenerateKey(rand.Reader)
	newPubKey := &newPriv.PublicKey
	newPubKeyHex := hex.EncodeToString(sm2.Compress(newPubKey))
	newNodeID := generateNodeID(newPubKey)

	invitation, _ := gm1.CreateInvitation(newPubKeyHex)
	joinReq := &JoinRequest{
		NewNodeID:  newNodeID,
		NewNodeKey: newPubKeyHex,
		Invitation: invitation,
		Timestamp:  time.Now().UnixMilli(),
	}
	gm1.ProcessJoinRequest(joinReq)

	// 重新加载
	gm2, _ := NewGenesisManager(tempDir)

	// 验证创世信息
	loaded := gm2.GetGenesis()
	if loaded == nil {
		t.Fatal("创世信息未加载")
	}
	if loaded.NetworkName != genesis.NetworkName {
		t.Errorf("网络名称不匹配")
	}

	// 验证节点列表
	if !gm2.IsNodeJoined(genesis.GenesisNodeID) {
		t.Error("创世节点应该已加入")
	}
	if !gm2.IsNodeJoined(newNodeID) {
		t.Error("新节点应该已加入")
	}

	// 验证节点ID
	if gm2.GetNodeID() != genesis.GenesisNodeID {
		t.Errorf("节点ID不匹配")
	}
}

func TestNeighborRecommendation(t *testing.T) {
	tempDir := t.TempDir()

	gm, _ := NewGenesisManager(tempDir)
	gm.InitGenesis("TestNetwork", "1.0.0")

	// 添加多个节点
	for i := 0; i < 5; i++ {
		newPriv, _ := sm2.GenerateKey(rand.Reader)
		newPubKey := &newPriv.PublicKey
		newPubKeyHex := hex.EncodeToString(sm2.Compress(newPubKey))
		newNodeID := generateNodeID(newPubKey)

		invitation, _ := gm.CreateInvitation(newPubKeyHex)
		joinReq := &JoinRequest{
			NewNodeID:  newNodeID,
			NewNodeKey: newPubKeyHex,
			Invitation: invitation,
			Timestamp:  time.Now().UnixMilli(),
		}
		gm.ProcessJoinRequest(joinReq)
	}

	// 添加新节点并检查邻居推荐
	newPriv, _ := sm2.GenerateKey(rand.Reader)
	newPubKey := &newPriv.PublicKey
	newPubKeyHex := hex.EncodeToString(sm2.Compress(newPubKey))
	newNodeID := generateNodeID(newPubKey)

	invitation, _ := gm.CreateInvitation(newPubKeyHex)
	joinReq := &JoinRequest{
		NewNodeID:  newNodeID,
		NewNodeKey: newPubKeyHex,
		Invitation: invitation,
		Timestamp:  time.Now().UnixMilli(),
	}

	resp, err := gm.ProcessJoinRequest(joinReq)
	if err != nil {
		t.Fatalf("处理加入请求失败: %v", err)
	}

	// 应该有邻居推荐
	if len(resp.Neighbors) == 0 {
		t.Error("应该有邻居推荐")
	}

	// 邻居不应该包含新节点自己
	for _, neighbor := range resp.Neighbors {
		if neighbor.NodeID == newNodeID {
			t.Error("邻居列表不应该包含新节点自己")
		}
	}

	t.Logf("推荐了 %d 个邻居", len(resp.Neighbors))
}

func BenchmarkCreateInvitation(b *testing.B) {
	tempDir := b.TempDir()
	gm, _ := NewGenesisManager(tempDir)
	gm.InitGenesis("TestNetwork", "1.0.0")

	newPriv, _ := sm2.GenerateKey(rand.Reader)
	newPubKeyHex := hex.EncodeToString(sm2.Compress(&newPriv.PublicKey))

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		gm.CreateInvitation(newPubKeyHex)
	}
}

func BenchmarkVerifyInvitation(b *testing.B) {
	tempDir := b.TempDir()
	gm, _ := NewGenesisManager(tempDir)
	gm.InitGenesis("TestNetwork", "1.0.0")

	newPriv, _ := sm2.GenerateKey(rand.Reader)
	newPubKeyHex := hex.EncodeToString(sm2.Compress(&newPriv.PublicKey))
	invitation, _ := gm.CreateInvitation(newPubKeyHex)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		gm.VerifyInvitation(invitation)
	}
}
