package bulletin

import (
	"os"
	"path/filepath"
	"sync"
	"testing"
	"time"
)

func createTestBoard(t *testing.T) *BulletinBoard {
	t.Helper()
	
	tmpDir := t.TempDir()
	config := &BulletinConfig{
		NodeID:             "test-node-001",
		DataDir:            tmpDir,
		MaxContentSize:     65536,
		DefaultTTL:         10,
		DefaultExpiry:      24 * time.Hour,
		MaxMessagesPerTopic: 100,
		PreviewLength:      50,
		CleanupInterval:    time.Minute,
		GossipEnabled:      true,
		DHTEnabled:         true,
	}
	
	bb, err := NewBulletinBoard(config)
	if err != nil {
		t.Fatalf("Failed to create bulletin board: %v", err)
	}
	
	return bb
}

func TestNewBulletinBoard(t *testing.T) {
	t.Run("nil config", func(t *testing.T) {
		_, err := NewBulletinBoard(nil)
		if err != ErrNilConfig {
			t.Errorf("expected ErrNilConfig, got %v", err)
		}
	})
	
	t.Run("empty node ID", func(t *testing.T) {
		config := &BulletinConfig{}
		_, err := NewBulletinBoard(config)
		if err != ErrEmptyNodeID {
			t.Errorf("expected ErrEmptyNodeID, got %v", err)
		}
	})
	
	t.Run("valid config", func(t *testing.T) {
		bb := createTestBoard(t)
		if bb == nil {
			t.Fatal("expected non-nil board")
		}
	})
}

func TestDefaultConfig(t *testing.T) {
	config := DefaultBulletinConfig("test-node")
	
	if config.NodeID != "test-node" {
		t.Errorf("NodeID = %s, want test-node", config.NodeID)
	}
	if config.MaxContentSize != 65536 {
		t.Errorf("MaxContentSize = %d, want 65536", config.MaxContentSize)
	}
	if config.DefaultTTL != 10 {
		t.Errorf("DefaultTTL = %d, want 10", config.DefaultTTL)
	}
}

func TestPublishMessage(t *testing.T) {
	bb := createTestBoard(t)
	
	msg, err := bb.PublishMessage("Hello World!", "general")
	if err != nil {
		t.Fatalf("PublishMessage failed: %v", err)
	}
	
	if msg.MessageID == "" {
		t.Error("expected non-empty message ID")
	}
	if msg.Author != "test-node-001" {
		t.Errorf("Author = %s, want test-node-001", msg.Author)
	}
	if msg.Content != "Hello World!" {
		t.Errorf("Content = %s, want Hello World!", msg.Content)
	}
	if msg.Topic != "general" {
		t.Errorf("Topic = %s, want general", msg.Topic)
	}
	if msg.Status != StatusActive {
		t.Errorf("Status = %s, want active", msg.Status)
	}
	if msg.TTL != 10 {
		t.Errorf("TTL = %d, want 10", msg.TTL)
	}
}

func TestPublishMessageWithOptions(t *testing.T) {
	bb := createTestBoard(t)
	
	tags := []string{"tag1", "tag2"}
	attachments := []string{"hash1", "hash2"}
	
	msg, err := bb.PublishMessageWithOptions("Content with options", "test-topic", tags, attachments, "")
	if err != nil {
		t.Fatalf("PublishMessageWithOptions failed: %v", err)
	}
	
	if len(msg.Tags) != 2 {
		t.Errorf("Tags length = %d, want 2", len(msg.Tags))
	}
	if len(msg.Attachments) != 2 {
		t.Errorf("Attachments length = %d, want 2", len(msg.Attachments))
	}
}

func TestPublishMessageErrors(t *testing.T) {
	bb := createTestBoard(t)
	
	t.Run("empty content", func(t *testing.T) {
		_, err := bb.PublishMessage("", "topic")
		if err != ErrEmptyContent {
			t.Errorf("expected ErrEmptyContent, got %v", err)
		}
	})
	
	t.Run("empty topic", func(t *testing.T) {
		_, err := bb.PublishMessage("content", "")
		if err != ErrEmptyTopic {
			t.Errorf("expected ErrEmptyTopic, got %v", err)
		}
	})
	
	t.Run("content too large", func(t *testing.T) {
		largeContent := make([]byte, 100000)
		_, err := bb.PublishMessage(string(largeContent), "topic")
		if err != ErrMessageTooLarge {
			t.Errorf("expected ErrMessageTooLarge, got %v", err)
		}
	})
}

func TestQueryMessage(t *testing.T) {
	bb := createTestBoard(t)
	
	msg, _ := bb.PublishMessage("Test query", "topic")
	
	t.Run("found", func(t *testing.T) {
		found, err := bb.QueryMessage(msg.MessageID)
		if err != nil {
			t.Fatalf("QueryMessage failed: %v", err)
		}
		if found.Content != "Test query" {
			t.Errorf("Content = %s, want Test query", found.Content)
		}
	})
	
	t.Run("not found", func(t *testing.T) {
		_, err := bb.QueryMessage("nonexistent")
		if err != ErrMessageNotFound {
			t.Errorf("expected ErrMessageNotFound, got %v", err)
		}
	})
}

func TestQueryByTopic(t *testing.T) {
	bb := createTestBoard(t)
	
	// 发布多条消息
	bb.PublishMessage("Message 1", "topic-a")
	bb.PublishMessage("Message 2", "topic-a")
	bb.PublishMessage("Message 3", "topic-b")
	
	t.Run("query topic-a", func(t *testing.T) {
		messages, err := bb.QueryByTopic("topic-a", 10, 0)
		if err != nil {
			t.Fatalf("QueryByTopic failed: %v", err)
		}
		if len(messages) != 2 {
			t.Errorf("messages count = %d, want 2", len(messages))
		}
	})
	
	t.Run("query topic-b", func(t *testing.T) {
		messages, err := bb.QueryByTopic("topic-b", 10, 0)
		if err != nil {
			t.Fatalf("QueryByTopic failed: %v", err)
		}
		if len(messages) != 1 {
			t.Errorf("messages count = %d, want 1", len(messages))
		}
	})
	
	t.Run("empty topic", func(t *testing.T) {
		_, err := bb.QueryByTopic("", 10, 0)
		if err != ErrEmptyTopic {
			t.Errorf("expected ErrEmptyTopic, got %v", err)
		}
	})
	
	t.Run("pagination", func(t *testing.T) {
		messages, _ := bb.QueryByTopic("topic-a", 1, 0)
		if len(messages) != 1 {
			t.Errorf("messages count = %d, want 1", len(messages))
		}
		
		messages, _ = bb.QueryByTopic("topic-a", 10, 10)
		if len(messages) != 0 {
			t.Errorf("messages count = %d, want 0", len(messages))
		}
	})
}

func TestQueryByAuthor(t *testing.T) {
	bb := createTestBoard(t)
	
	bb.PublishMessage("Message 1", "topic")
	bb.PublishMessage("Message 2", "topic")
	
	messages, err := bb.QueryByAuthor("test-node-001", 10, 0)
	if err != nil {
		t.Fatalf("QueryByAuthor failed: %v", err)
	}
	if len(messages) != 2 {
		t.Errorf("messages count = %d, want 2", len(messages))
	}
	
	// 查询不存在的作者
	messages, err = bb.QueryByAuthor("nonexistent", 10, 0)
	if err != nil {
		t.Fatalf("QueryByAuthor failed: %v", err)
	}
	if len(messages) != 0 {
		t.Errorf("messages count = %d, want 0", len(messages))
	}
}

func TestReceiveMessage(t *testing.T) {
	bb := createTestBoard(t)
	
	msg := &Message{
		MessageID: "external-msg-001",
		Author:    "external-node",
		Topic:     "external",
		Content:   "External message",
		Timestamp: time.Now(),
		ExpiresAt: time.Now().Add(24 * time.Hour),
		Status:    StatusActive,
		TTL:       5,
	}
	
	err := bb.ReceiveMessage(msg, "from-node")
	if err != nil {
		t.Fatalf("ReceiveMessage failed: %v", err)
	}
	
	// 验证消息已存储
	found, _ := bb.QueryMessage("external-msg-001")
	if found.Content != "External message" {
		t.Errorf("Content = %s, want External message", found.Content)
	}
	
	// TTL 应该减少
	if found.TTL != 4 {
		t.Errorf("TTL = %d, want 4", found.TTL)
	}
}

func TestReceiveMessageErrors(t *testing.T) {
	bb := createTestBoard(t)
	
	t.Run("nil message", func(t *testing.T) {
		err := bb.ReceiveMessage(nil, "node")
		if err == nil {
			t.Error("expected error for nil message")
		}
	})
	
	t.Run("empty message ID", func(t *testing.T) {
		msg := &Message{Content: "test"}
		err := bb.ReceiveMessage(msg, "node")
		if err != ErrInvalidMessageID {
			t.Errorf("expected ErrInvalidMessageID, got %v", err)
		}
	})
	
	t.Run("expired message", func(t *testing.T) {
		msg := &Message{
			MessageID: "expired-msg",
			ExpiresAt: time.Now().Add(-1 * time.Hour),
		}
		err := bb.ReceiveMessage(msg, "node")
		if err != ErrMessageExpired {
			t.Errorf("expected ErrMessageExpired, got %v", err)
		}
	})
	
	t.Run("duplicate message", func(t *testing.T) {
		msg := &Message{
			MessageID: "dup-msg",
			ExpiresAt: time.Now().Add(time.Hour),
			Topic:     "test",
		}
		_ = bb.ReceiveMessage(msg, "node")
		err := bb.ReceiveMessage(msg, "node")
		if err != ErrDuplicateMessage {
			t.Errorf("expected ErrDuplicateMessage, got %v", err)
		}
	})
}

func TestReceiveMessageWithSignatureVerification(t *testing.T) {
	tmpDir := t.TempDir()
	config := &BulletinConfig{
		NodeID:          "test-node",
		DataDir:         tmpDir,
		MaxContentSize:  65536,
		DefaultTTL:      10,
		DefaultExpiry:   24 * time.Hour,
		CleanupInterval: time.Minute,
		VerifyFunc: func(publicKey string, data []byte, signature string) bool {
			return signature == "valid-sig"
		},
	}
	
	bb, _ := NewBulletinBoard(config)
	
	t.Run("valid signature", func(t *testing.T) {
		msg := &Message{
			MessageID: "signed-msg-1",
			Author:    "author",
			Topic:     "test",
			Content:   "content",
			Timestamp: time.Now(),
			ExpiresAt: time.Now().Add(time.Hour),
			Signature: "valid-sig",
			TTL:       5,
		}
		err := bb.ReceiveMessage(msg, "node")
		if err != nil {
			t.Errorf("expected no error, got %v", err)
		}
	})
	
	t.Run("invalid signature", func(t *testing.T) {
		msg := &Message{
			MessageID: "signed-msg-2",
			Author:    "author",
			Topic:     "test",
			Content:   "content",
			Timestamp: time.Now(),
			ExpiresAt: time.Now().Add(time.Hour),
			Signature: "invalid-sig",
			TTL:       5,
		}
		err := bb.ReceiveMessage(msg, "node")
		if err != ErrInvalidSignature {
			t.Errorf("expected ErrInvalidSignature, got %v", err)
		}
	})
}

func TestSubscribeTopic(t *testing.T) {
	bb := createTestBoard(t)
	
	var received []*Message
	var mu sync.Mutex
	
	callback := func(msg *Message) {
		mu.Lock()
		received = append(received, msg)
		mu.Unlock()
	}
	
	err := bb.SubscribeTopic("news", callback)
	if err != nil {
		t.Fatalf("SubscribeTopic failed: %v", err)
	}
	
	// 验证订阅
	subs := bb.GetSubscriptions()
	if len(subs) != 1 {
		t.Errorf("subscriptions count = %d, want 1", len(subs))
	}
	
	// 发布消息，验证回调
	bb.PublishMessage("Breaking news!", "news")
	
	time.Sleep(50 * time.Millisecond)
	
	mu.Lock()
	count := len(received)
	mu.Unlock()
	
	if count != 1 {
		t.Errorf("received count = %d, want 1", count)
	}
}

func TestUnsubscribeTopic(t *testing.T) {
	bb := createTestBoard(t)
	
	bb.SubscribeTopic("topic1", nil)
	bb.SubscribeTopic("topic2", nil)
	
	err := bb.UnsubscribeTopic("topic1")
	if err != nil {
		t.Fatalf("UnsubscribeTopic failed: %v", err)
	}
	
	subs := bb.GetSubscriptions()
	if len(subs) != 1 {
		t.Errorf("subscriptions count = %d, want 1", len(subs))
	}
	
	// 取消不存在的订阅
	err = bb.UnsubscribeTopic("nonexistent")
	if err != ErrNotSubscribed {
		t.Errorf("expected ErrNotSubscribed, got %v", err)
	}
}

func TestRevokeMessage(t *testing.T) {
	bb := createTestBoard(t)
	
	msg, _ := bb.PublishMessage("To be revoked", "topic")
	
	err := bb.RevokeMessage(msg.MessageID)
	if err != nil {
		t.Fatalf("RevokeMessage failed: %v", err)
	}
	
	found, _ := bb.QueryMessage(msg.MessageID)
	if found.Status != StatusRevoked {
		t.Errorf("Status = %s, want revoked", found.Status)
	}
}

func TestRevokeMessageErrors(t *testing.T) {
	bb := createTestBoard(t)
	
	t.Run("not found", func(t *testing.T) {
		err := bb.RevokeMessage("nonexistent")
		if err != ErrMessageNotFound {
			t.Errorf("expected ErrMessageNotFound, got %v", err)
		}
	})
	
	t.Run("other author", func(t *testing.T) {
		// 接收外部消息
		msg := &Message{
			MessageID: "other-msg",
			Author:    "other-node",
			Topic:     "topic",
			Content:   "content",
			Timestamp: time.Now(),
			ExpiresAt: time.Now().Add(time.Hour),
			TTL:       5,
		}
		bb.ReceiveMessage(msg, "node")
		
		err := bb.RevokeMessage("other-msg")
		if err == nil {
			t.Error("expected error for revoking other's message")
		}
	})
}

func TestPinUnpinMessage(t *testing.T) {
	bb := createTestBoard(t)
	
	msg, _ := bb.PublishMessage("Pin me", "topic")
	
	// 置顶
	err := bb.PinMessage(msg.MessageID)
	if err != nil {
		t.Fatalf("PinMessage failed: %v", err)
	}
	
	found, _ := bb.QueryMessage(msg.MessageID)
	if found.Status != StatusPinned {
		t.Errorf("Status = %s, want pinned", found.Status)
	}
	
	pinned := bb.GetPinnedMessages()
	if len(pinned) != 1 {
		t.Errorf("pinned count = %d, want 1", len(pinned))
	}
	
	// 取消置顶
	err = bb.UnpinMessage(msg.MessageID)
	if err != nil {
		t.Fatalf("UnpinMessage failed: %v", err)
	}
	
	found, _ = bb.QueryMessage(msg.MessageID)
	if found.Status != StatusActive {
		t.Errorf("Status = %s, want active", found.Status)
	}
	
	pinned = bb.GetPinnedMessages()
	if len(pinned) != 0 {
		t.Errorf("pinned count = %d, want 0", len(pinned))
	}
}

func TestSearchMessages(t *testing.T) {
	bb := createTestBoard(t)
	
	bb.PublishMessage("Hello world", "topic1")
	bb.PublishMessage("Hello there", "topic2")
	bb.PublishMessage("Goodbye world", "topic1")
	
	results := bb.SearchMessages("hello", 10)
	if len(results) != 2 {
		t.Errorf("results count = %d, want 2", len(results))
	}
	
	results = bb.SearchMessages("world", 10)
	if len(results) != 2 {
		t.Errorf("results count = %d, want 2", len(results))
	}
	
	results = bb.SearchMessages("goodbye", 10)
	if len(results) != 1 {
		t.Errorf("results count = %d, want 1", len(results))
	}
}

func TestGetTopics(t *testing.T) {
	bb := createTestBoard(t)
	
	bb.PublishMessage("msg1", "topic-a")
	bb.PublishMessage("msg2", "topic-b")
	bb.PublishMessage("msg3", "topic-c")
	
	topics := bb.GetTopics()
	if len(topics) != 3 {
		t.Errorf("topics count = %d, want 3", len(topics))
	}
}

func TestGetTopicStats(t *testing.T) {
	bb := createTestBoard(t)
	
	bb.PublishMessage("msg1", "stats-topic")
	bb.PublishMessage("msg2", "stats-topic")
	msg3, _ := bb.PublishMessage("msg3", "stats-topic")
	
	// 撤回一条
	bb.RevokeMessage(msg3.MessageID)
	
	total, active := bb.GetTopicStats("stats-topic")
	if total != 3 {
		t.Errorf("total = %d, want 3", total)
	}
	if active != 2 {
		t.Errorf("active = %d, want 2", active)
	}
}

func TestGetMessageSummaries(t *testing.T) {
	bb := createTestBoard(t)
	
	bb.PublishMessage("This is a long message content that should be truncated in the preview", "summary-topic")
	bb.PublishMessage("Short message", "summary-topic")
	
	summaries := bb.GetMessageSummaries("summary-topic", 10, 0)
	if len(summaries) != 2 {
		t.Errorf("summaries count = %d, want 2", len(summaries))
	}
	
	// 验证预览长度
	for _, s := range summaries {
		if len(s.Preview) > 50+3 { // 50 chars + "..."
			t.Errorf("preview too long: %d", len(s.Preview))
		}
	}
}

func TestGetMessagesForGossip(t *testing.T) {
	bb := createTestBoard(t)
	
	bb.PublishMessage("Gossip 1", "gossip")
	bb.PublishMessage("Gossip 2", "gossip")
	
	messages := bb.GetMessagesForGossip(10)
	if len(messages) != 2 {
		t.Errorf("messages count = %d, want 2", len(messages))
	}
	
	// 所有消息应该有 TTL > 0
	for _, msg := range messages {
		if msg.TTL <= 0 {
			t.Error("expected TTL > 0")
		}
	}
}

func TestGetStats(t *testing.T) {
	bb := createTestBoard(t)
	
	bb.PublishMessage("msg1", "topic-a")
	bb.PublishMessage("msg2", "topic-b")
	msg3, _ := bb.PublishMessage("msg3", "topic-a")
	
	bb.SubscribeTopic("topic-a", nil)
	bb.PinMessage(msg3.MessageID)
	
	stats := bb.GetStats()
	
	if stats.TotalMessages != 3 {
		t.Errorf("TotalMessages = %d, want 3", stats.TotalMessages)
	}
	if stats.TotalTopics != 2 {
		t.Errorf("TotalTopics = %d, want 2", stats.TotalTopics)
	}
	if stats.TotalAuthors != 1 {
		t.Errorf("TotalAuthors = %d, want 1", stats.TotalAuthors)
	}
	if stats.Subscriptions != 1 {
		t.Errorf("Subscriptions = %d, want 1", stats.Subscriptions)
	}
	if stats.PinnedMessages != 1 {
		t.Errorf("PinnedMessages = %d, want 1", stats.PinnedMessages)
	}
}

func TestStartStop(t *testing.T) {
	bb := createTestBoard(t)
	
	bb.Start()
	
	// 重复启动应该无效
	bb.Start()
	
	bb.Stop()
	
	// 重复停止应该无效
	bb.Stop()
}

func TestPersistence(t *testing.T) {
	tmpDir := t.TempDir()
	
	// 创建并发布消息
	config := &BulletinConfig{
		NodeID:          "persist-node",
		DataDir:         tmpDir,
		MaxContentSize:  65536,
		DefaultTTL:      10,
		DefaultExpiry:   24 * time.Hour,
		CleanupInterval: time.Minute,
	}
	
	bb1, _ := NewBulletinBoard(config)
	bb1.PublishMessage("Persistent message", "persist-topic")
	bb1.SubscribeTopic("persist-topic", nil)
	bb1.save() // 显式保存
	
	// 重新加载
	bb2, _ := NewBulletinBoard(config)
	
	messages, _ := bb2.QueryByTopic("persist-topic", 10, 0)
	if len(messages) != 1 {
		t.Errorf("messages count = %d, want 1", len(messages))
	}
	
	subs := bb2.GetSubscriptions()
	if len(subs) != 1 {
		t.Errorf("subscriptions count = %d, want 1", len(subs))
	}
}

func TestCallbacks(t *testing.T) {
	bb := createTestBoard(t)
	
	var publishedMsg *Message
	var receivedMsg *Message
	var revokedID string
	var subscribedTopic string
	
	bb.OnMessagePublished = func(msg *Message) {
		publishedMsg = msg
	}
	bb.OnMessageReceived = func(msg *Message) {
		receivedMsg = msg
	}
	bb.OnMessageRevoked = func(id string) {
		revokedID = id
	}
	bb.OnTopicSubscribed = func(topic string) {
		subscribedTopic = topic
	}
	
	// 发布
	msg, _ := bb.PublishMessage("Callback test", "callback-topic")
	if publishedMsg == nil {
		t.Error("OnMessagePublished not called")
	}
	
	// 订阅
	bb.SubscribeTopic("callback-topic", nil)
	time.Sleep(50 * time.Millisecond)
	if subscribedTopic != "callback-topic" {
		t.Errorf("OnTopicSubscribed: topic = %s, want callback-topic", subscribedTopic)
	}
	
	// 接收外部消息
	extMsg := &Message{
		MessageID: "ext-callback-msg",
		Author:    "ext-node",
		Topic:     "callback-topic",
		Content:   "External",
		Timestamp: time.Now(),
		ExpiresAt: time.Now().Add(time.Hour),
		TTL:       5,
	}
	bb.ReceiveMessage(extMsg, "node")
	if receivedMsg == nil {
		t.Error("OnMessageReceived not called")
	}
	
	// 撤回
	bb.RevokeMessage(msg.MessageID)
	time.Sleep(50 * time.Millisecond)
	if revokedID != msg.MessageID {
		t.Errorf("OnMessageRevoked: id = %s, want %s", revokedID, msg.MessageID)
	}
}

func TestCleanup(t *testing.T) {
	tmpDir := t.TempDir()
	config := &BulletinConfig{
		NodeID:             "cleanup-node",
		DataDir:            tmpDir,
		MaxContentSize:     65536,
		DefaultTTL:         10,
		DefaultExpiry:      100 * time.Millisecond, // 很快过期
		MaxMessagesPerTopic: 2,                     // 限制数量
		CleanupInterval:    50 * time.Millisecond,
	}
	
	bb, _ := NewBulletinBoard(config)
	
	// 发布消息
	bb.PublishMessage("msg1", "cleanup-topic")
	bb.PublishMessage("msg2", "cleanup-topic")
	bb.PublishMessage("msg3", "cleanup-topic") // 超出限制
	
	// 触发清理
	bb.cleanup()
	
	// 因为限制是2，应该只保留2条
	messages, _ := bb.QueryByTopic("cleanup-topic", 10, 0)
	if len(messages) > 2 {
		t.Errorf("messages count = %d, want <= 2", len(messages))
	}
}

func TestReputationOrdering(t *testing.T) {
	tmpDir := t.TempDir()
	
	// 创建带声誉函数的配置
	reputationMap := map[string]float64{
		"node-high":   90.0,
		"node-medium": 50.0,
		"node-low":    10.0,
	}
	
	config := &BulletinConfig{
		NodeID:          "test-node",
		DataDir:         tmpDir,
		MaxContentSize:  65536,
		DefaultTTL:      10,
		DefaultExpiry:   24 * time.Hour,
		CleanupInterval: time.Minute,
		GetReputationFunc: func(nodeID string) float64 {
			if r, ok := reputationMap[nodeID]; ok {
				return r
			}
			return 50.0
		},
	}
	
	bb, _ := NewBulletinBoard(config)
	
	// 模拟接收不同声誉节点的消息
	msgs := []struct {
		id     string
		author string
		rep    float64
	}{
		{"msg-low", "node-low", 10.0},
		{"msg-high", "node-high", 90.0},
		{"msg-medium", "node-medium", 50.0},
	}
	
	for _, m := range msgs {
		msg := &Message{
			MessageID:       m.id,
			Author:          m.author,
			Topic:           "rep-topic",
			Content:         "Content from " + m.author,
			Timestamp:       time.Now(),
			ExpiresAt:       time.Now().Add(time.Hour),
			ReputationScore: m.rep,
			Status:          StatusActive,
			TTL:             5,
		}
		bb.ReceiveMessage(msg, "node")
	}
	
	// 查询应该按声誉排序
	messages, _ := bb.QueryByTopic("rep-topic", 10, 0)
	if len(messages) != 3 {
		t.Fatalf("messages count = %d, want 3", len(messages))
	}
	
	// 第一条应该是高声誉的
	if messages[0].ReputationScore != 90.0 {
		t.Errorf("first message reputation = %f, want 90.0", messages[0].ReputationScore)
	}
	// 最后一条应该是低声誉的
	if messages[2].ReputationScore != 10.0 {
		t.Errorf("last message reputation = %f, want 10.0", messages[2].ReputationScore)
	}
}

func TestSetExpiry(t *testing.T) {
	bb := createTestBoard(t)
	
	msg, _ := bb.PublishMessage("Test expiry", "topic")
	
	newExpiry := time.Now().Add(48 * time.Hour)
	err := bb.SetExpiry(msg.MessageID, newExpiry)
	if err != nil {
		t.Fatalf("SetExpiry failed: %v", err)
	}
	
	found, _ := bb.QueryMessage(msg.MessageID)
	if !found.ExpiresAt.Equal(newExpiry) {
		t.Errorf("ExpiresAt = %v, want %v", found.ExpiresAt, newExpiry)
	}
	
	// 不存在的消息
	err = bb.SetExpiry("nonexistent", newExpiry)
	if err != ErrMessageNotFound {
		t.Errorf("expected ErrMessageNotFound, got %v", err)
	}
}

func TestVerifyMessage(t *testing.T) {
	tmpDir := t.TempDir()
	config := &BulletinConfig{
		NodeID:          "verify-node",
		DataDir:         tmpDir,
		MaxContentSize:  65536,
		DefaultTTL:      10,
		DefaultExpiry:   24 * time.Hour,
		CleanupInterval: time.Minute,
		SignFunc: func(data []byte) (string, error) {
			return "test-signature", nil
		},
		VerifyFunc: func(publicKey string, data []byte, signature string) bool {
			return signature == "test-signature"
		},
	}
	
	bb, _ := NewBulletinBoard(config)
	
	msg, _ := bb.PublishMessage("Verify me", "verify-topic")
	
	if !bb.VerifyMessage(msg) {
		t.Error("expected message to be verified")
	}
	
	// 修改签名
	msg.Signature = "invalid"
	if bb.VerifyMessage(msg) {
		t.Error("expected verification to fail")
	}
}

func TestClear(t *testing.T) {
	bb := createTestBoard(t)
	
	bb.PublishMessage("msg1", "topic1")
	bb.PublishMessage("msg2", "topic2")
	
	bb.Clear()
	
	stats := bb.GetStats()
	if stats.TotalMessages != 0 {
		t.Errorf("TotalMessages = %d, want 0", stats.TotalMessages)
	}
	if stats.TotalTopics != 0 {
		t.Errorf("TotalTopics = %d, want 0", stats.TotalTopics)
	}
}

func TestConcurrentAccess(t *testing.T) {
	bb := createTestBoard(t)
	
	var wg sync.WaitGroup
	
	// 并发发布
	for i := 0; i < 10; i++ {
		wg.Add(1)
		go func(n int) {
			defer wg.Done()
			bb.PublishMessage("Concurrent message", "concurrent-topic")
		}(i)
	}
	
	// 并发查询
	for i := 0; i < 10; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			bb.QueryByTopic("concurrent-topic", 10, 0)
			bb.GetStats()
			bb.GetTopics()
		}()
	}
	
	wg.Wait()
	
	messages, _ := bb.QueryByTopic("concurrent-topic", 100, 0)
	if len(messages) != 10 {
		t.Errorf("messages count = %d, want 10", len(messages))
	}
}

func TestReplyTo(t *testing.T) {
	bb := createTestBoard(t)
	
	// 发布原始消息
	original, _ := bb.PublishMessage("Original message", "reply-topic")
	
	// 发布回复
	reply, _ := bb.PublishMessageWithOptions("This is a reply", "reply-topic", nil, nil, original.MessageID)
	
	if reply.ReplyTo != original.MessageID {
		t.Errorf("ReplyTo = %s, want %s", reply.ReplyTo, original.MessageID)
	}
}

func TestPersistenceWithDataDir(t *testing.T) {
	tmpDir := t.TempDir()
	
	config := &BulletinConfig{
		NodeID:          "persist-test",
		DataDir:         tmpDir,
		MaxContentSize:  65536,
		DefaultTTL:      10,
		DefaultExpiry:   24 * time.Hour,
		CleanupInterval: time.Minute,
	}
	
	bb, _ := NewBulletinBoard(config)
	
	// 发布并保存
	bb.PublishMessage("Test persistence", "persist-topic")
	bb.save()
	
	// 验证文件存在
	filePath := filepath.Join(tmpDir, "bulletin.json")
	if _, err := os.Stat(filePath); os.IsNotExist(err) {
		t.Error("expected bulletin.json to exist")
	}
}
