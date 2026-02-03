package mailbox

import (
	"os"
	"path/filepath"
	"testing"
	"time"
)

// 测试辅助函数 - 创建测试配置
func createTestConfig(t *testing.T) *MailboxConfig {
	tempDir := t.TempDir()
	return &MailboxConfig{
		NodeID:          "test-node-001",
		DataDir:         tempDir,
		MaxInboxSize:    100,
		MaxOutboxSize:   50,
		DefaultTTL:      1 * time.Hour,
		CleanupInterval: 1 * time.Minute,
		EnableEncrypt:   true,
	}
}

// 测试辅助函数 - 创建邮箱实例
func createTestMailbox(t *testing.T) *Mailbox {
	config := createTestConfig(t)
	mb, err := NewMailbox(config)
	if err != nil {
		t.Fatalf("Failed to create mailbox: %v", err)
	}
	return mb
}

// 测试辅助函数 - 模拟签名函数
func mockSignFunc(data []byte) ([]byte, error) {
	// 简单返回数据的前32字节作为"签名"
	result := make([]byte, 32)
	copy(result, data)
	return result, nil
}

// 测试辅助函数 - 模拟验签函数
func mockVerifyFunc(pubKey string, data, signature []byte) (bool, error) {
	return true, nil
}

// 测试辅助函数 - 模拟加密函数
func mockEncryptFunc(pubKey string, data []byte) ([]byte, error) {
	// 简单反转数据作为"加密"
	encrypted := make([]byte, len(data))
	for i, b := range data {
		encrypted[len(data)-1-i] = b
	}
	return encrypted, nil
}

// 测试辅助函数 - 模拟解密函数
func mockDecryptFunc(data []byte) ([]byte, error) {
	// 再次反转恢复原数据
	decrypted := make([]byte, len(data))
	for i, b := range data {
		decrypted[len(data)-1-i] = b
	}
	return decrypted, nil
}

// === 测试用例 ===

func TestNewMailbox(t *testing.T) {
	tests := []struct {
		name    string
		config  *MailboxConfig
		wantErr bool
	}{
		{
			name:    "nil config",
			config:  nil,
			wantErr: true,
		},
		{
			name: "empty node ID",
			config: &MailboxConfig{
				NodeID: "",
			},
			wantErr: true,
		},
		{
			name: "valid config",
			config: &MailboxConfig{
				NodeID:  "test-node",
				DataDir: t.TempDir(),
			},
			wantErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mb, err := NewMailbox(tt.config)
			if (err != nil) != tt.wantErr {
				t.Errorf("NewMailbox() error = %v, wantErr %v", err, tt.wantErr)
			}
			if !tt.wantErr && mb == nil {
				t.Error("NewMailbox() returned nil mailbox")
			}
		})
	}
}

func TestDefaultConfig(t *testing.T) {
	nodeID := "test-node-123"
	config := DefaultConfig(nodeID)

	if config.NodeID != nodeID {
		t.Errorf("NodeID = %v, want %v", config.NodeID, nodeID)
	}
	if config.MaxInboxSize <= 0 {
		t.Error("MaxInboxSize should be positive")
	}
	if config.MaxOutboxSize <= 0 {
		t.Error("MaxOutboxSize should be positive")
	}
	if config.DefaultTTL <= 0 {
		t.Error("DefaultTTL should be positive")
	}
}

func TestSendMessage(t *testing.T) {
	mb := createTestMailbox(t)
	mb.SetSignFunc(mockSignFunc)

	tests := []struct {
		name     string
		receiver string
		subject  string
		content  []byte
		encrypt  bool
		wantErr  bool
	}{
		{
			name:     "empty receiver",
			receiver: "",
			subject:  "Test",
			content:  []byte("Hello"),
			wantErr:  true,
		},
		{
			name:     "empty content",
			receiver: "receiver-001",
			subject:  "Test",
			content:  []byte{},
			wantErr:  true,
		},
		{
			name:     "valid message",
			receiver: "receiver-001",
			subject:  "Hello",
			content:  []byte("Hello World!"),
			encrypt:  false,
			wantErr:  false,
		},
		{
			name:     "valid encrypted message",
			receiver: "receiver-002",
			subject:  "Secret",
			content:  []byte("Secret Message"),
			encrypt:  true,
			wantErr:  false, // 没有加密函数时不会加密
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			msg, err := mb.SendMessage(tt.receiver, tt.subject, tt.content, tt.encrypt)
			if (err != nil) != tt.wantErr {
				t.Errorf("SendMessage() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if !tt.wantErr {
				if msg == nil {
					t.Error("SendMessage() returned nil message")
					return
				}
				if msg.ID == "" {
					t.Error("Message ID should not be empty")
				}
				if msg.Sender != mb.config.NodeID {
					t.Errorf("Sender = %v, want %v", msg.Sender, mb.config.NodeID)
				}
				if msg.Receiver != tt.receiver {
					t.Errorf("Receiver = %v, want %v", msg.Receiver, tt.receiver)
				}
			}
		})
	}
}

func TestSendMessageWithEncryption(t *testing.T) {
	mb := createTestMailbox(t)
	mb.SetSignFunc(mockSignFunc)
	mb.SetEncryptFunc(mockEncryptFunc)

	content := []byte("Secret Message")
	msg, err := mb.SendMessage("receiver-001", "Secret", content, true)
	if err != nil {
		t.Fatalf("SendMessage() error = %v", err)
	}

	if !msg.Encrypted {
		t.Error("Message should be encrypted")
	}
	// 加密后内容应该不同（除非是空或特殊情况）
	if string(msg.Content) == string(content) {
		t.Error("Encrypted content should differ from original")
	}
}

func TestReceiveMessage(t *testing.T) {
	mb := createTestMailbox(t)
	mb.SetVerifyFunc(mockVerifyFunc)

	// 创建一条消息
	msg := &Message{
		ID:        "test-msg-001",
		Sender:    "sender-001",
		Receiver:  mb.config.NodeID,
		Subject:   "Test",
		Content:   []byte("Hello"),
		Timestamp: time.Now(),
		ExpiresAt: time.Now().Add(1 * time.Hour),
		Status:    StatusPending,
		Signature: []byte("signature"),
	}

	// 接收消息
	err := mb.ReceiveMessage(msg)
	if err != nil {
		t.Fatalf("ReceiveMessage() error = %v", err)
	}

	// 验证消息已存入收件箱
	if mb.GetInboxCount() != 1 {
		t.Errorf("InboxCount = %d, want 1", mb.GetInboxCount())
	}

	// 状态应该更新为已投递
	received, _ := mb.GetMessage("test-msg-001")
	if received.Status != StatusDelivered {
		t.Errorf("Status = %v, want %v", received.Status, StatusDelivered)
	}
}

func TestReceiveMessageErrors(t *testing.T) {
	mb := createTestMailbox(t)

	tests := []struct {
		name    string
		msg     *Message
		wantErr bool
	}{
		{
			name:    "nil message",
			msg:     nil,
			wantErr: true,
		},
		{
			name: "wrong receiver",
			msg: &Message{
				ID:        "test-001",
				Receiver:  "other-node",
				Timestamp: time.Now(),
				ExpiresAt: time.Now().Add(1 * time.Hour),
			},
			wantErr: true,
		},
		{
			name: "expired message",
			msg: &Message{
				ID:        "test-002",
				Receiver:  mb.config.NodeID,
				Timestamp: time.Now().Add(-2 * time.Hour),
				ExpiresAt: time.Now().Add(-1 * time.Hour),
			},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := mb.ReceiveMessage(tt.msg)
			if (err != nil) != tt.wantErr {
				t.Errorf("ReceiveMessage() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func TestMarkAsRead(t *testing.T) {
	mb := createTestMailbox(t)

	// 添加消息到收件箱
	msg := &Message{
		ID:        "test-msg-001",
		Sender:    "sender-001",
		Receiver:  mb.config.NodeID,
		Content:   []byte("Hello"),
		Timestamp: time.Now(),
		ExpiresAt: time.Now().Add(1 * time.Hour),
		Status:    StatusDelivered,
	}
	mb.ReceiveMessage(msg)

	// 标记为已读
	err := mb.MarkAsRead("test-msg-001")
	if err != nil {
		t.Fatalf("MarkAsRead() error = %v", err)
	}

	// 验证状态
	received, _ := mb.GetMessage("test-msg-001")
	if received.Status != StatusRead {
		t.Errorf("Status = %v, want %v", received.Status, StatusRead)
	}
	if received.ReadAt == nil {
		t.Error("ReadAt should be set")
	}
}

func TestMarkAsReadNotFound(t *testing.T) {
	mb := createTestMailbox(t)

	err := mb.MarkAsRead("non-existent")
	if err == nil {
		t.Error("MarkAsRead() should return error for non-existent message")
	}
}

func TestListInbox(t *testing.T) {
	mb := createTestMailbox(t)

	// 添加多条消息
	for i := 0; i < 5; i++ {
		msg := &Message{
			ID:        "msg-" + string(rune('a'+i)),
			Sender:    "sender-001",
			Receiver:  mb.config.NodeID,
			Subject:   "Test " + string(rune('a'+i)),
			Content:   []byte("Content"),
			Timestamp: time.Now().Add(time.Duration(i) * time.Minute),
			ExpiresAt: time.Now().Add(1 * time.Hour),
			Status:    StatusPending,
		}
		mb.ReceiveMessage(msg)
	}

	// 获取列表
	summaries := mb.ListInbox(10, 0)
	if len(summaries) != 5 {
		t.Errorf("ListInbox() returned %d messages, want 5", len(summaries))
	}

	// 验证排序（按时间倒序）
	for i := 1; i < len(summaries); i++ {
		if summaries[i].Timestamp.After(summaries[i-1].Timestamp) {
			t.Error("Messages should be sorted by time descending")
		}
	}

	// 测试分页
	page1 := mb.ListInbox(2, 0)
	if len(page1) != 2 {
		t.Errorf("Page 1 should have 2 messages, got %d", len(page1))
	}

	page2 := mb.ListInbox(2, 2)
	if len(page2) != 2 {
		t.Errorf("Page 2 should have 2 messages, got %d", len(page2))
	}

	page3 := mb.ListInbox(2, 4)
	if len(page3) != 1 {
		t.Errorf("Page 3 should have 1 message, got %d", len(page3))
	}
}

func TestListOutbox(t *testing.T) {
	mb := createTestMailbox(t)
	mb.SetSignFunc(mockSignFunc)

	// 发送多条消息，使用不同的接收者和内容确保ID不同
	receivers := []string{"receiver-001", "receiver-002", "receiver-003"}
	for i, receiver := range receivers {
		content := []byte("Content " + string(rune('a'+i)))
		_, err := mb.SendMessage(receiver, "Subject "+string(rune('a'+i)), content, false)
		if err != nil {
			t.Fatalf("SendMessage() error = %v", err)
		}
	}

	// 获取发件箱列表
	summaries := mb.ListOutbox(10, 0)
	if len(summaries) != 3 {
		t.Errorf("ListOutbox() returned %d messages, want 3", len(summaries))
	}
}

func TestDeleteMessage(t *testing.T) {
	mb := createTestMailbox(t)

	// 添加收件箱消息
	msg := &Message{
		ID:        "inbox-msg-001",
		Sender:    "sender-001",
		Receiver:  mb.config.NodeID,
		Content:   []byte("Hello"),
		Timestamp: time.Now(),
		ExpiresAt: time.Now().Add(1 * time.Hour),
	}
	mb.ReceiveMessage(msg)

	// 删除消息
	err := mb.DeleteMessage("inbox-msg-001")
	if err != nil {
		t.Fatalf("DeleteMessage() error = %v", err)
	}

	// 验证删除
	if mb.GetInboxCount() != 0 {
		t.Error("Inbox should be empty after deletion")
	}

	// 删除不存在的消息
	err = mb.DeleteMessage("non-existent")
	if err == nil {
		t.Error("DeleteMessage() should return error for non-existent message")
	}
}

func TestGetUnreadCount(t *testing.T) {
	mb := createTestMailbox(t)

	// 添加消息
	for i := 0; i < 5; i++ {
		msg := &Message{
			ID:        "msg-" + string(rune('a'+i)),
			Sender:    "sender-001",
			Receiver:  mb.config.NodeID,
			Content:   []byte("Content"),
			Timestamp: time.Now(),
			ExpiresAt: time.Now().Add(1 * time.Hour),
		}
		mb.ReceiveMessage(msg)
	}

	// 初始未读数
	if mb.GetUnreadCount() != 5 {
		t.Errorf("UnreadCount = %d, want 5", mb.GetUnreadCount())
	}

	// 标记两条为已读
	mb.MarkAsRead("msg-a")
	mb.MarkAsRead("msg-b")

	if mb.GetUnreadCount() != 3 {
		t.Errorf("UnreadCount = %d, want 3", mb.GetUnreadCount())
	}
}

func TestStoreForRelay(t *testing.T) {
	mb := createTestMailbox(t)
	mb.SetVerifyFunc(mockVerifyFunc)

	// 存储中继消息
	msg := &Message{
		ID:        "relay-msg-001",
		Sender:    "sender-001",
		Receiver:  "other-node-001",
		Content:   []byte("Hello"),
		Timestamp: time.Now(),
		ExpiresAt: time.Now().Add(1 * time.Hour),
		Signature: []byte("sig"),
	}

	err := mb.StoreForRelay(msg)
	if err != nil {
		t.Fatalf("StoreForRelay() error = %v", err)
	}

	// 验证存储
	count := mb.GetPendingCount("other-node-001")
	if count != 1 {
		t.Errorf("PendingCount = %d, want 1", count)
	}
}

func TestFetchPendingMessages(t *testing.T) {
	mb := createTestMailbox(t)

	// 存储多条中继消息
	receiver := "other-node-001"
	for i := 0; i < 5; i++ {
		msg := &Message{
			ID:        "relay-msg-" + string(rune('a'+i)),
			Sender:    "sender-001",
			Receiver:  receiver,
			Content:   []byte("Hello"),
			Timestamp: time.Now(),
			ExpiresAt: time.Now().Add(1 * time.Hour),
		}
		mb.StoreForRelay(msg)
	}

	// 获取部分消息
	messages := mb.FetchPendingMessages(receiver, 3)
	if len(messages) != 3 {
		t.Errorf("FetchPendingMessages() returned %d, want 3", len(messages))
	}

	// 验证剩余数量
	remaining := mb.GetPendingCount(receiver)
	if remaining != 2 {
		t.Errorf("Remaining count = %d, want 2", remaining)
	}

	// 获取剩余所有
	messages = mb.FetchPendingMessages(receiver, 0)
	if len(messages) != 2 {
		t.Errorf("FetchPendingMessages() returned %d, want 2", len(messages))
	}

	// 验证已清空
	if mb.GetPendingCount(receiver) != 0 {
		t.Error("Pending messages should be empty")
	}
}

func TestPersistence(t *testing.T) {
	tempDir := t.TempDir()

	// 创建邮箱并添加数据
	config := &MailboxConfig{
		NodeID:          "test-node",
		DataDir:         tempDir,
		MaxInboxSize:    100,
		MaxOutboxSize:   50,
		DefaultTTL:      1 * time.Hour,
		CleanupInterval: 1 * time.Hour,
	}

	mb1, _ := NewMailbox(config)
	mb1.SetSignFunc(mockSignFunc)

	// 发送消息
	msg, _ := mb1.SendMessage("receiver-001", "Test", []byte("Hello"), false)

	// 接收消息
	inboxMsg := &Message{
		ID:        "inbox-001",
		Sender:    "sender-001",
		Receiver:  "test-node",
		Content:   []byte("Received"),
		Timestamp: time.Now(),
		ExpiresAt: time.Now().Add(1 * time.Hour),
	}
	mb1.ReceiveMessage(inboxMsg)

	// 保存到磁盘
	err := mb1.saveToDisk()
	if err != nil {
		t.Fatalf("saveToDisk() error = %v", err)
	}

	// 验证文件存在
	filePath := filepath.Join(tempDir, "mailbox.json")
	if _, err := os.Stat(filePath); os.IsNotExist(err) {
		t.Fatal("Mailbox file should exist")
	}

	// 创建新邮箱实例并加载
	mb2, _ := NewMailbox(config)
	err = mb2.loadFromDisk()
	if err != nil {
		t.Fatalf("loadFromDisk() error = %v", err)
	}

	// 验证数据恢复
	if mb2.GetOutboxCount() != 1 {
		t.Errorf("OutboxCount = %d, want 1", mb2.GetOutboxCount())
	}
	if mb2.GetInboxCount() != 1 {
		t.Errorf("InboxCount = %d, want 1", mb2.GetInboxCount())
	}

	// 验证消息内容
	loaded, err := mb2.GetMessage(msg.ID)
	if err != nil {
		t.Fatalf("GetMessage() error = %v", err)
	}
	if loaded.Subject != "Test" {
		t.Errorf("Subject = %v, want Test", loaded.Subject)
	}
}

func TestStartStop(t *testing.T) {
	mb := createTestMailbox(t)

	// 启动
	err := mb.Start()
	if err != nil {
		t.Fatalf("Start() error = %v", err)
	}

	// 发送一条消息
	mb.SetSignFunc(mockSignFunc)
	_, err = mb.SendMessage("receiver-001", "Test", []byte("Hello"), false)
	if err != nil {
		t.Fatalf("SendMessage() error = %v", err)
	}

	// 停止
	err = mb.Stop()
	if err != nil {
		t.Fatalf("Stop() error = %v", err)
	}
}

func TestGetStats(t *testing.T) {
	mb := createTestMailbox(t)
	mb.SetSignFunc(mockSignFunc)

	// 发送消息
	mb.SendMessage("receiver-001", "Test", []byte("Hello"), false)
	mb.SendMessage("receiver-002", "Test", []byte("Hello"), false)

	// 接收消息
	for i := 0; i < 3; i++ {
		msg := &Message{
			ID:        "inbox-" + string(rune('a'+i)),
			Sender:    "sender-001",
			Receiver:  mb.config.NodeID,
			Content:   []byte("Hello"),
			Timestamp: time.Now(),
			ExpiresAt: time.Now().Add(1 * time.Hour),
		}
		mb.ReceiveMessage(msg)
	}

	// 标记一条已读
	mb.MarkAsRead("inbox-a")

	// 存储中继消息
	mb.StoreForRelay(&Message{
		ID:        "relay-001",
		Receiver:  "other-node",
		Content:   []byte("Relay"),
		Timestamp: time.Now(),
		ExpiresAt: time.Now().Add(1 * time.Hour),
	})

	// 获取统计
	stats := mb.GetStats()

	if stats.OutboxCount != 2 {
		t.Errorf("OutboxCount = %d, want 2", stats.OutboxCount)
	}
	if stats.InboxCount != 3 {
		t.Errorf("InboxCount = %d, want 3", stats.InboxCount)
	}
	if stats.UnreadCount != 2 {
		t.Errorf("UnreadCount = %d, want 2", stats.UnreadCount)
	}
	if stats.PendingCount != 1 {
		t.Errorf("PendingCount = %d, want 1", stats.PendingCount)
	}
}

func TestGetMessageContent(t *testing.T) {
	mb := createTestMailbox(t)
	mb.SetEncryptFunc(mockEncryptFunc)
	mb.SetDecryptFunc(mockDecryptFunc)

	originalContent := []byte("Hello World")

	// 加密消息
	encryptedContent, _ := mockEncryptFunc("", originalContent)
	msg := &Message{
		ID:        "encrypted-msg",
		Sender:    "sender-001",
		Receiver:  mb.config.NodeID,
		Content:   encryptedContent,
		Encrypted: true,
		Timestamp: time.Now(),
		ExpiresAt: time.Now().Add(1 * time.Hour),
	}
	mb.ReceiveMessage(msg)

	// 获取内容（应该自动解密）
	content, err := mb.GetMessageContent("encrypted-msg")
	if err != nil {
		t.Fatalf("GetMessageContent() error = %v", err)
	}

	if string(content) != string(originalContent) {
		t.Errorf("Content = %s, want %s", content, originalContent)
	}
}

func TestGetMessageContentNotFound(t *testing.T) {
	mb := createTestMailbox(t)

	_, err := mb.GetMessageContent("non-existent")
	if err == nil {
		t.Error("GetMessageContent() should return error for non-existent message")
	}
}

func TestOutboxFull(t *testing.T) {
	config := &MailboxConfig{
		NodeID:        "test-node",
		DataDir:       t.TempDir(),
		MaxInboxSize:  100,
		MaxOutboxSize: 2, // 只允许2条
		DefaultTTL:    1 * time.Hour,
	}

	mb, _ := NewMailbox(config)
	mb.SetSignFunc(mockSignFunc)

	// 发送2条消息
	mb.SendMessage("receiver-001", "Test 1", []byte("Hello 1"), false)
	mb.SendMessage("receiver-002", "Test 2", []byte("Hello 2"), false)

	// 第3条应该失败
	_, err := mb.SendMessage("receiver-003", "Test 3", []byte("Hello 3"), false)
	if err == nil {
		t.Error("SendMessage() should fail when outbox is full")
	}
}

func TestInboxFull(t *testing.T) {
	config := &MailboxConfig{
		NodeID:        "test-node",
		DataDir:       t.TempDir(),
		MaxInboxSize:  2, // 只允许2条
		MaxOutboxSize: 50,
		DefaultTTL:    1 * time.Hour,
	}

	mb, _ := NewMailbox(config)

	// 接收2条消息
	for i := 0; i < 2; i++ {
		msg := &Message{
			ID:        "msg-" + string(rune('a'+i)),
			Receiver:  "test-node",
			Content:   []byte("Hello"),
			Timestamp: time.Now().Add(time.Duration(i) * time.Second),
			ExpiresAt: time.Now().Add(1 * time.Hour),
		}
		mb.ReceiveMessage(msg)
	}

	// 第3条应该成功，但会删除最旧的
	msg3 := &Message{
		ID:        "msg-c",
		Receiver:  "test-node",
		Content:   []byte("Hello"),
		Timestamp: time.Now().Add(2 * time.Second),
		ExpiresAt: time.Now().Add(1 * time.Hour),
	}
	err := mb.ReceiveMessage(msg3)
	if err != nil {
		t.Fatalf("ReceiveMessage() error = %v", err)
	}

	// 应该还是2条，最旧的被删除
	if mb.GetInboxCount() != 2 {
		t.Errorf("InboxCount = %d, want 2", mb.GetInboxCount())
	}
}

func TestDuplicateMessage(t *testing.T) {
	mb := createTestMailbox(t)

	msg := &Message{
		ID:        "dup-msg-001",
		Receiver:  mb.config.NodeID,
		Content:   []byte("Hello"),
		Timestamp: time.Now(),
		ExpiresAt: time.Now().Add(1 * time.Hour),
	}

	// 第一次接收成功
	err := mb.ReceiveMessage(msg)
	if err != nil {
		t.Fatalf("First ReceiveMessage() error = %v", err)
	}

	// 第二次应该失败（重复）
	err = mb.ReceiveMessage(msg)
	if err == nil {
		t.Error("Second ReceiveMessage() should fail for duplicate message")
	}
}

func TestCallbacks(t *testing.T) {
	mb := createTestMailbox(t)
	mb.SetSignFunc(mockSignFunc)

	var receivedMsg *Message
	var sentMsg *Message
	var readMsg *Message

	mb.SetOnMessageReceived(func(msg *Message) {
		receivedMsg = msg
	})
	mb.SetOnMessageSent(func(msg *Message) {
		sentMsg = msg
	})
	mb.SetOnMessageRead(func(msg *Message) {
		readMsg = msg
	})

	// 发送消息
	_, err := mb.SendMessage("receiver-001", "Test", []byte("Hello"), false)
	if err != nil {
		t.Fatalf("SendMessage() error = %v", err)
	}
	// 等待回调
	time.Sleep(50 * time.Millisecond)
	if sentMsg == nil {
		t.Error("OnMessageSent callback not triggered")
	}

	// 接收消息
	msg := &Message{
		ID:        "callback-test",
		Receiver:  mb.config.NodeID,
		Content:   []byte("Hello"),
		Timestamp: time.Now(),
		ExpiresAt: time.Now().Add(1 * time.Hour),
	}
	mb.ReceiveMessage(msg)
	time.Sleep(50 * time.Millisecond)
	if receivedMsg == nil {
		t.Error("OnMessageReceived callback not triggered")
	}

	// 标记已读
	mb.MarkAsRead("callback-test")
	time.Sleep(50 * time.Millisecond)
	if readMsg == nil {
		t.Error("OnMessageRead callback not triggered")
	}
}

func TestSetFunctions(t *testing.T) {
	mb := createTestMailbox(t)

	// 测试设置各种函数不会panic
	mb.SetSignFunc(nil)
	mb.SetVerifyFunc(nil)
	mb.SetEncryptFunc(nil)
	mb.SetDecryptFunc(nil)
	mb.SetDeliverFunc(nil)

	mb.SetSignFunc(mockSignFunc)
	mb.SetVerifyFunc(mockVerifyFunc)
	mb.SetEncryptFunc(mockEncryptFunc)
	mb.SetDecryptFunc(mockDecryptFunc)
	mb.SetDeliverFunc(func(receiver string, msg *Message) error {
		return nil
	})

	// 验证设置成功后可以正常使用
	_, err := mb.SendMessage("receiver-001", "Test", []byte("Hello"), true)
	if err != nil {
		t.Errorf("SendMessage() after SetFunctions error = %v", err)
	}
}

func TestCleanup(t *testing.T) {
	mb := createTestMailbox(t)

	// 添加即将过期的消息
	expiredMsg := &Message{
		ID:        "expired-msg",
		Receiver:  mb.config.NodeID,
		Content:   []byte("Hello"),
		Timestamp: time.Now().Add(-2 * time.Hour),
		ExpiresAt: time.Now().Add(-1 * time.Hour), // 已过期
	}

	// 直接写入（跳过验证）
	mb.mu.Lock()
	mb.inbox["expired-msg"] = expiredMsg
	mb.mu.Unlock()

	// 添加未过期消息
	validMsg := &Message{
		ID:        "valid-msg",
		Receiver:  mb.config.NodeID,
		Content:   []byte("Hello"),
		Timestamp: time.Now(),
		ExpiresAt: time.Now().Add(1 * time.Hour),
	}
	mb.ReceiveMessage(validMsg)

	// 执行清理
	mb.cleanup()

	// 验证过期消息被删除
	if mb.GetInboxCount() != 1 {
		t.Errorf("InboxCount = %d, want 1 (expired should be removed)", mb.GetInboxCount())
	}

	_, err := mb.GetMessage("expired-msg")
	if err == nil {
		t.Error("Expired message should be removed")
	}

	_, err = mb.GetMessage("valid-msg")
	if err != nil {
		t.Error("Valid message should not be removed")
	}
}
