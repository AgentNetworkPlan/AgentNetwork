package sync

import (
	"context"
	"encoding/json"
	"testing"
	"time"
)

// MockPeerConnector 模拟节点连接器
type MockPeerConnector struct {
	connected map[string]bool
	messages  map[string][][]byte
}

func NewMockPeerConnector() *MockPeerConnector {
	return &MockPeerConnector{
		connected: make(map[string]bool),
		messages:  make(map[string][][]byte),
	}
}

func (m *MockPeerConnector) IsConnected(nodeID string) bool {
	return m.connected[nodeID]
}

func (m *MockPeerConnector) Connect(ctx context.Context, nodeID string) error {
	m.connected[nodeID] = true
	return nil
}

func (m *MockPeerConnector) Send(ctx context.Context, nodeID string, data []byte) error {
	m.messages[nodeID] = append(m.messages[nodeID], data)
	return nil
}

func (m *MockPeerConnector) GetConnectedPeers() []string {
	var peers []string
	for id := range m.connected {
		peers = append(peers, id)
	}
	return peers
}

func (m *MockPeerConnector) GetPeerInfo(nodeID string) (*PeerInfo, error) {
	return &PeerInfo{
		NodeID:     nodeID,
		Reputation: 50.0,
	}, nil
}

// MockMessageSigner 模拟签名器
type MockMessageSigner struct {
	nodeID    string
	publicKey string
}

func NewMockMessageSigner(nodeID string) *MockMessageSigner {
	return &MockMessageSigner{
		nodeID:    nodeID,
		publicKey: "mock_pubkey_" + nodeID,
	}
}

func (m *MockMessageSigner) Sign(data []byte) ([]byte, error) {
	return []byte("mock_signature"), nil
}

func (m *MockMessageSigner) Verify(publicKey string, data, signature []byte) (bool, error) {
	return true, nil
}

func (m *MockMessageSigner) GetNodeID() string {
	return m.nodeID
}

func (m *MockMessageSigner) GetPublicKey() string {
	return m.publicKey
}

// MockNeighborProvider 模拟邻居提供者
type MockNeighborProvider struct {
	neighbors []string
	relays    []string
}

func NewMockNeighborProvider() *MockNeighborProvider {
	return &MockNeighborProvider{
		neighbors: []string{"node2", "node3"},
		relays:    []string{"relay1"},
	}
}

func (m *MockNeighborProvider) GetNeighbors() []string {
	return m.neighbors
}

func (m *MockNeighborProvider) GetRelayNodes() []string {
	return m.relays
}

// MockReputationChecker 模拟声誉检查器
type MockReputationChecker struct {
	blacklist map[string]bool
}

func NewMockReputationChecker() *MockReputationChecker {
	return &MockReputationChecker{
		blacklist: make(map[string]bool),
	}
}

func (m *MockReputationChecker) GetReputation(nodeID string) float64 {
	return 50.0
}

func (m *MockReputationChecker) IsBlacklisted(nodeID string) bool {
	return m.blacklist[nodeID]
}

// MockBulletinStore 模拟留言板存储
type MockBulletinStore struct {
	messages map[string]*BulletinPayload
}

func NewMockBulletinStore() *MockBulletinStore {
	return &MockBulletinStore{
		messages: make(map[string]*BulletinPayload),
	}
}

func (m *MockBulletinStore) GetMessage(messageID string) (*BulletinPayload, error) {
	if msg, ok := m.messages[messageID]; ok {
		return msg, nil
	}
	return nil, ErrTopicNotFound
}

func (m *MockBulletinStore) GetMessagesByTopic(topic string, since time.Time, limit int) ([]*BulletinPayload, error) {
	var result []*BulletinPayload
	for _, msg := range m.messages {
		if msg.Topic == topic && msg.Timestamp.After(since) {
			result = append(result, msg)
			if len(result) >= limit {
				break
			}
		}
	}
	return result, nil
}

func (m *MockBulletinStore) GetAllTopics() []string {
	topics := make(map[string]bool)
	for _, msg := range m.messages {
		topics[msg.Topic] = true
	}
	var result []string
	for t := range topics {
		result = append(result, t)
	}
	return result
}

func (m *MockBulletinStore) StoreMessage(msg *BulletinPayload) error {
	m.messages[msg.MessageID] = msg
	return nil
}

func (m *MockBulletinStore) HasMessage(messageID string) bool {
	_, ok := m.messages[messageID]
	return ok
}

func (m *MockBulletinStore) GetLatestTimestamp(topic string) time.Time {
	var latest time.Time
	for _, msg := range m.messages {
		if msg.Topic == topic && msg.Timestamp.After(latest) {
			latest = msg.Timestamp
		}
	}
	return latest
}

// 测试邮件路由器
func TestMailRouter(t *testing.T) {
	config := DefaultRouterConfig("node1")
	router := NewMailRouter(config)
	
	connector := NewMockPeerConnector()
	connector.connected["node2"] = true
	
	router.SetPeerConnector(connector)
	router.SetSigner(NewMockMessageSigner("node1"))
	router.SetNeighborProvider(NewMockNeighborProvider())
	router.SetReputationChecker(NewMockReputationChecker())
	
	router.Start()
	defer router.Stop()
	
	// 测试发送邮件
	err := router.SendMail("node2", "Test Subject", []byte("Test Content"), false)
	if err != nil {
		t.Errorf("SendMail failed: %v", err)
	}
	
	// 检查消息是否发送
	if len(connector.messages["node2"]) == 0 {
		t.Error("Expected message to be sent to node2")
	}
}

// 测试邮件接收
func TestMailRouterReceive(t *testing.T) {
	config := DefaultRouterConfig("node2")
	router := NewMailRouter(config)
	
	received := false
	router.SetOnReceive(func(msg *SyncMessage, payload *MailPayload) {
		received = true
		if payload.Subject != "Test Subject" {
			t.Errorf("Expected subject 'Test Subject', got '%s'", payload.Subject)
		}
	})
	
	router.Start()
	defer router.Stop()
	
	// 创建测试消息
	payload := &MailPayload{
		MessageID: "test123",
		Subject:   "Test Subject",
		Content:   []byte("Test Content"),
	}
	payloadBytes, _ := json.Marshal(payload)
	
	msg := &SyncMessage{
		ID:        "msg123",
		Type:      TypeMailSend,
		Sender:    "node1",
		Receiver:  "node2",
		Timestamp: time.Now(),
		TTL:       5,
		Nonce:     "nonce123",
		Payload:   payloadBytes,
	}
	
	data, _ := json.Marshal(msg)
	err := router.HandleMessage(data)
	if err != nil {
		t.Errorf("HandleMessage failed: %v", err)
	}
	
	if !received {
		t.Error("Expected message to be received")
	}
}

// 测试端到端加密
func TestE2EEncryption(t *testing.T) {
	// 创建两个加密器
	encryptor1, err := NewE2EEncryptor(DefaultEncryptorConfig("node1"))
	if err != nil {
		t.Fatalf("Failed to create encryptor1: %v", err)
	}
	
	encryptor2, err := NewE2EEncryptor(DefaultEncryptorConfig("node2"))
	if err != nil {
		t.Fatalf("Failed to create encryptor2: %v", err)
	}
	
	// 交换公钥
	pubKey1 := encryptor1.GetPublicKey()
	pubKey2 := encryptor2.GetPublicKey()
	
	if err := encryptor1.SetPeerPublicKey("node2", pubKey2); err != nil {
		t.Fatalf("Failed to set peer public key: %v", err)
	}
	if err := encryptor2.SetPeerPublicKey("node1", pubKey1); err != nil {
		t.Fatalf("Failed to set peer public key: %v", err)
	}
	
	// 测试加密/解密
	plaintext := []byte("Hello, this is a secret message!")
	
	// node1 加密
	ciphertext, err := encryptor1.Encrypt("node2", plaintext)
	if err != nil {
		t.Fatalf("Encrypt failed: %v", err)
	}
	
	// node2 解密
	decrypted, err := encryptor2.Decrypt("node1", ciphertext)
	if err != nil {
		t.Fatalf("Decrypt failed: %v", err)
	}
	
	if string(decrypted) != string(plaintext) {
		t.Errorf("Decrypted message doesn't match: got %s, want %s", string(decrypted), string(plaintext))
	}
}

// 测试前向保密密钥交换
func TestPFSKeyExchange(t *testing.T) {
	encryptor1, _ := NewE2EEncryptor(DefaultEncryptorConfig("node1"))
	encryptor2, _ := NewE2EEncryptor(DefaultEncryptorConfig("node2"))
	
	// node1 创建密钥交换请求
	request, err := encryptor1.CreateKeyExchangeRequest()
	if err != nil {
		t.Fatalf("CreateKeyExchangeRequest failed: %v", err)
	}
	
	// node2 响应
	response, err := encryptor2.CreateKeyExchangeResponse(request.SessionID)
	if err != nil {
		t.Fatalf("CreateKeyExchangeResponse failed: %v", err)
	}
	
	// 双方派生会话密钥
	key1, err := encryptor1.DeriveSessionKey(request.SessionID, response.PublicKey)
	if err != nil {
		t.Fatalf("DeriveSessionKey failed for node1: %v", err)
	}
	
	key2, err := encryptor2.DeriveSessionKey(request.SessionID, request.PublicKey)
	if err != nil {
		t.Fatalf("DeriveSessionKey failed for node2: %v", err)
	}
	
	// 验证密钥相同
	if string(key1) != string(key2) {
		t.Error("Session keys don't match")
	}
	
	// 使用会话密钥加密/解密
	plaintext := []byte("Forward secret message")
	ciphertext, err := encryptor1.EncryptWithSessionKey(key1, plaintext)
	if err != nil {
		t.Fatalf("EncryptWithSessionKey failed: %v", err)
	}
	
	decrypted, err := encryptor2.DecryptWithSessionKey(key2, ciphertext)
	if err != nil {
		t.Fatalf("DecryptWithSessionKey failed: %v", err)
	}
	
	if string(decrypted) != string(plaintext) {
		t.Errorf("Decrypted message doesn't match: got %s, want %s", string(decrypted), string(plaintext))
	}
}

// 测试回执管理器
func TestReceiptManager(t *testing.T) {
	config := DefaultReceiptManagerConfig("node1")
	rm := NewReceiptManager(config)
	
	deliveredCalled := false
	rm.SetOnDelivered(func(r *MessageReceipt) {
		deliveredCalled = true
	})
	
	rm.Start()
	defer rm.Stop()
	
	// 跟踪消息
	rm.TrackMessage("msg1", "node1", "node2")
	
	// 检查状态
	receipt := rm.GetReceipt("msg1")
	if receipt == nil {
		t.Fatal("Expected receipt to exist")
	}
	if receipt.Status != ReceiptPending {
		t.Errorf("Expected status pending, got %s", receipt.Status)
	}
	
	// 标记送达
	rm.MarkDelivered("msg1", time.Now())
	
	// 等待回调
	time.Sleep(100 * time.Millisecond)
	
	if !deliveredCalled {
		t.Error("Expected onDelivered callback to be called")
	}
	
	receipt = rm.GetReceipt("msg1")
	if receipt.Status != ReceiptDelivered {
		t.Errorf("Expected status delivered, got %s", receipt.Status)
	}
	
	// 标记已读
	rm.MarkRead("msg1", time.Now())
	
	receipt = rm.GetReceipt("msg1")
	if receipt.Status != ReceiptRead {
		t.Errorf("Expected status read, got %s", receipt.Status)
	}
}

// 测试回执统计
func TestReceiptStats(t *testing.T) {
	rm := NewReceiptManager(DefaultReceiptManagerConfig("node1"))
	
	// 添加多个消息
	rm.TrackMessage("msg1", "node1", "node2")
	rm.TrackMessage("msg2", "node1", "node3")
	rm.TrackMessage("msg3", "node1", "node4")
	
	rm.MarkDelivered("msg1", time.Now())
	rm.MarkRead("msg2", time.Now())
	
	stats := rm.GetStats()
	if stats["pending"] != 1 {
		t.Errorf("Expected 1 pending, got %d", stats["pending"])
	}
	if stats["delivered"] != 1 {
		t.Errorf("Expected 1 delivered, got %d", stats["delivered"])
	}
	if stats["read"] != 1 {
		t.Errorf("Expected 1 read, got %d", stats["read"])
	}
}

// 测试留言板同步器
func TestBulletinSyncer(t *testing.T) {
	config := DefaultSyncerConfig("node1")
	syncer := NewBulletinSyncer(config)
	
	store := NewMockBulletinStore()
	connector := NewMockPeerConnector()
	// 标记邻居节点为已连接（MockNeighborProvider返回node2, node3）
	connector.connected["node2"] = true
	connector.connected["node3"] = true
	
	syncer.SetStore(store)
	syncer.SetPeerConnector(connector)
	syncer.SetNeighborProvider(NewMockNeighborProvider())
	
	syncer.Start()
	defer syncer.Stop()
	
	// 发布消息
	msg := &BulletinPayload{
		MessageID: "bulletin1",
		Author:    "node1",
		Topic:     "general",
		Content:   "Hello World",
		Timestamp: time.Now(),
		ExpiresAt: time.Now().Add(24 * time.Hour),
	}
	
	err := syncer.PublishMessage(msg)
	if err != nil {
		t.Errorf("PublishMessage failed: %v", err)
	}
	
	// 验证存储
	if !store.HasMessage("bulletin1") {
		t.Error("Expected message to be stored")
	}
	
	// 等待异步广播完成
	time.Sleep(100 * time.Millisecond)
	
	// 验证广播
	if len(connector.messages) == 0 {
		t.Error("Expected message to be broadcast to neighbors")
	}
}

// 测试自动发现
func TestAutoDiscovery(t *testing.T) {
	config := DefaultDiscoveryConfig("node1")
	discovery := NewAutoDiscovery(config)
	
	connector := NewMockPeerConnector()
	
	discovery.SetPeerConnector(connector)
	discovery.SetNeighborProvider(NewMockNeighborProvider())
	discovery.SetReputationChecker(NewMockReputationChecker())
	discovery.SetSelfInfo(&PeerInfo{
		NodeID:     "node1",
		Reputation: 50.0,
		Addresses:  []string{"/ip4/127.0.0.1/tcp/8080"},
	})
	
	discovered := false
	discovery.SetOnPeerDiscovered(func(peer *DiscoveredPeer) {
		discovered = true
	})
	
	if err := discovery.Start(); err != nil {
		t.Fatalf("Start failed: %v", err)
	}
	defer discovery.Stop()
	
	// 模拟收到节点广播
	announcement := &PeerAnnouncement{
		Info: PeerInfo{
			NodeID:     "node5",
			Reputation: 60.0,
			Addresses:  []string{"/ip4/192.168.1.5/tcp/8080"},
		},
		Neighbors: []string{"node6"},
	}
	
	payloadBytes, _ := json.Marshal(announcement)
	msg := &SyncMessage{
		ID:        "announce1",
		Type:      TypePeerAnnounce,
		Sender:    "node5",
		Timestamp: time.Now(),
		TTL:       3,
		Payload:   payloadBytes,
	}
	
	data, _ := json.Marshal(msg)
	discovery.HandleMessage(data)
	
	// 等待处理
	time.Sleep(100 * time.Millisecond)
	
	if !discovered {
		t.Error("Expected peer to be discovered")
	}
	
	peers := discovery.GetDiscoveredPeers()
	found := false
	for _, p := range peers {
		if p.Info.NodeID == "node5" {
			found = true
			break
		}
	}
	if !found {
		t.Error("Expected node5 to be in discovered peers")
	}
}

// 测试同步管理器
func TestSyncManager(t *testing.T) {
	config := DefaultManagerConfig("node1")
	manager, err := NewSyncManager(config)
	if err != nil {
		t.Fatalf("NewSyncManager failed: %v", err)
	}
	
	connector := NewMockPeerConnector()
	connector.connected["node2"] = true
	
	manager.SetPeerConnector(connector)
	manager.SetSigner(NewMockMessageSigner("node1"))
	manager.SetNeighborProvider(NewMockNeighborProvider())
	manager.SetReputationChecker(NewMockReputationChecker())
	
	if err := manager.Start(); err != nil {
		t.Fatalf("Start failed: %v", err)
	}
	defer manager.Stop()
	
	// 测试发送邮件
	err = manager.SendMail("node2", "Test", []byte("Hello"), false)
	if err != nil {
		t.Errorf("SendMail failed: %v", err)
	}
	
	// 测试获取公钥
	pubKey := manager.GetPublicKey()
	if pubKey == "" {
		t.Error("Expected public key to be non-empty")
	}
	
	// 测试统计
	stats := manager.GetStats()
	if stats == nil {
		t.Error("Expected stats to be non-nil")
	}
}

// 测试消息缓存防重放
func TestMessageCacheAntiReplay(t *testing.T) {
	config := DefaultRouterConfig("node1")
	router := NewMailRouter(config)
	router.Start()
	defer router.Stop()
	
	// 创建测试消息
	payload := &MailPayload{
		MessageID: "test123",
		Subject:   "Test",
		Content:   []byte("Content"),
	}
	payloadBytes, _ := json.Marshal(payload)
	
	msg := &SyncMessage{
		ID:        "unique_msg_id",
		Type:      TypeMailSend,
		Sender:    "node2",
		Receiver:  "node1",
		Timestamp: time.Now(),
		TTL:       5,
		Nonce:     "nonce123",
		Payload:   payloadBytes,
	}
	
	data, _ := json.Marshal(msg)
	
	// 第一次处理应该成功
	receiveCount := 0
	router.SetOnReceive(func(m *SyncMessage, p *MailPayload) {
		receiveCount++
	})
	
	router.HandleMessage(data)
	
	// 第二次处理同一消息应该被忽略（防重放）
	router.HandleMessage(data)
	
	// 应该只收到一次
	if receiveCount != 1 {
		t.Errorf("Expected message to be received once, got %d times", receiveCount)
	}
}

// 测试生成ID唯一性
func TestGenerateIDUniqueness(t *testing.T) {
	ids := make(map[string]bool)
	for i := 0; i < 1000; i++ {
		id := generateID()
		if ids[id] {
			t.Errorf("Generated duplicate ID: %s", id)
		}
		ids[id] = true
	}
}
