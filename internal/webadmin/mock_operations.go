package webadmin

import (
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"sort"
	"sync"
	"time"
)

// MockOperationsProvider 模拟操作提供者，用于测试和演示
type MockOperationsProvider struct {
	mu          sync.RWMutex
	neighbors   []*NeighborInfo
	inbox       []*MailMessage
	outbox      []*MailMessage
	bulletins   []*BulletinMessage
	subscriptions []string
	reputations map[string]*ReputationInfo
}

// NewMockOperationsProvider 创建模拟操作提供者
func NewMockOperationsProvider() *MockOperationsProvider {
	m := &MockOperationsProvider{
		neighbors:     make([]*NeighborInfo, 0),
		inbox:         make([]*MailMessage, 0),
		outbox:        make([]*MailMessage, 0),
		bulletins:     make([]*BulletinMessage, 0),
		subscriptions: []string{"general", "announcements"},
		reputations:   make(map[string]*ReputationInfo),
	}
	m.initMockData()
	return m
}

func (m *MockOperationsProvider) initMockData() {
	// 添加一些模拟邻居
	m.neighbors = []*NeighborInfo{
		{
			NodeID:       "16Uiu2HAm7abc123def456ghi789jkl012mno345pqr678stu901vwx234yz",
			Type:         "normal",
			Reputation:   85,
			TrustScore:   0.85,
			Status:       "online",
			LastSeen:     time.Now().Format(time.RFC3339),
			SuccessPings: 42,
			FailedPings:  3,
		},
		{
			NodeID:       "16Uiu2HAm8xyz987wvu654tsr321qpo098nml765kji432hgf210edc",
			Type:         "super",
			Reputation:   95,
			TrustScore:   0.95,
			Status:       "online",
			LastSeen:     time.Now().Format(time.RFC3339),
			SuccessPings: 100,
			FailedPings:  1,
		},
		{
			NodeID:       "16Uiu2HAm9qwerty123456789abcdefghijklmnopqrstuvwxyz0123456",
			Type:         "normal",
			Reputation:   65,
			TrustScore:   0.65,
			Status:       "offline",
			LastSeen:     time.Now().Add(-30 * time.Minute).Format(time.RFC3339),
			SuccessPings: 20,
			FailedPings:  10,
		},
	}

	// 添加模拟邮件
	m.inbox = []*MailMessage{
		{
			ID:        "mail001",
			From:      "16Uiu2HAm7abc123def456ghi789jkl012mno345pqr678stu901vwx234yz",
			To:        "self",
			Subject:   "欢迎加入网络",
			Content:   "欢迎加入 DAAN 网络！这是一条测试消息。",
			Timestamp: time.Now().Add(-2 * time.Hour).Format(time.RFC3339),
			Status:    "delivered",
			Encrypted: false,
		},
		{
			ID:        "mail002",
			From:      "16Uiu2HAm8xyz987wvu654tsr321qpo098nml765kji432hgf210edc",
			To:        "self",
			Subject:   "任务通知",
			Content:   "您有一个新的任务等待处理。",
			Timestamp: time.Now().Add(-1 * time.Hour).Format(time.RFC3339),
			Status:    "delivered",
			Encrypted: true,
		},
	}

	// 添加模拟留言
	m.bulletins = []*BulletinMessage{
		{
			MessageID:  "bulletin001",
			Author:     "16Uiu2HAm7abc123def456ghi789jkl012mno345pqr678stu901vwx234yz",
			Topic:      "general",
			Content:    "大家好！这是 DAAN 网络的留言板测试消息。",
			Timestamp:  time.Now().Add(-3 * time.Hour).Format(time.RFC3339),
			ExpiresAt:  time.Now().Add(24 * time.Hour).Format(time.RFC3339),
			Status:     "active",
			Reputation: 85.0,
		},
		{
			MessageID:  "bulletin002",
			Author:     "16Uiu2HAm8xyz987wvu654tsr321qpo098nml765kji432hgf210edc",
			Topic:      "announcements",
			Content:    "【公告】系统将于明天进行升级维护。",
			Timestamp:  time.Now().Add(-1 * time.Hour).Format(time.RFC3339),
			ExpiresAt:  time.Now().Add(48 * time.Hour).Format(time.RFC3339),
			Status:     "pinned",
			Tags:       []string{"重要", "系统"},
			Reputation: 95.0,
		},
		{
			MessageID:  "bulletin003",
			Author:     "self",
			Topic:      "general",
			Content:    "测试发布的留言内容。",
			Timestamp:  time.Now().Add(-30 * time.Minute).Format(time.RFC3339),
			ExpiresAt:  time.Now().Add(6 * time.Hour).Format(time.RFC3339),
			Status:     "active",
			Reputation: 80.0,
		},
	}

	// 添加模拟声誉数据
	m.reputations = map[string]*ReputationInfo{
		"16Uiu2HAm7abc123def456ghi789jkl012mno345pqr678stu901vwx234yz": {
			NodeID:     "16Uiu2HAm7abc123def456ghi789jkl012mno345pqr678stu901vwx234yz",
			Reputation: 85.0,
			Rank:       3,
		},
		"16Uiu2HAm8xyz987wvu654tsr321qpo098nml765kji432hgf210edc": {
			NodeID:     "16Uiu2HAm8xyz987wvu654tsr321qpo098nml765kji432hgf210edc",
			Reputation: 95.0,
			Rank:       1,
		},
		"16Uiu2HAm9qwerty123456789abcdefghijklmnopqrstuvwxyz0123456": {
			NodeID:     "16Uiu2HAm9qwerty123456789abcdefghijklmnopqrstuvwxyz0123456",
			Reputation: 65.0,
			Rank:       5,
		},
	}
}

// ========== 邻居管理实现 ==========

func (m *MockOperationsProvider) GetNeighbors() ([]*NeighborInfo, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	result := make([]*NeighborInfo, len(m.neighbors))
	copy(result, m.neighbors)
	return result, nil
}

func (m *MockOperationsProvider) GetBestNeighbors(count int) ([]*NeighborInfo, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	
	// 按信任分排序
	sorted := make([]*NeighborInfo, len(m.neighbors))
	copy(sorted, m.neighbors)
	sort.Slice(sorted, func(i, j int) bool {
		return sorted[i].TrustScore > sorted[j].TrustScore
	})
	
	if count > len(sorted) {
		count = len(sorted)
	}
	return sorted[:count], nil
}

func (m *MockOperationsProvider) AddNeighbor(nodeID string, addresses []string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	
	// 检查是否已存在
	for _, n := range m.neighbors {
		if n.NodeID == nodeID {
			return errors.New("邻居已存在")
		}
	}
	
	m.neighbors = append(m.neighbors, &NeighborInfo{
		NodeID:       nodeID,
		Type:         "normal",
		Reputation:   50,
		TrustScore:   0.5,
		Status:       "unknown",
		Addresses:    addresses,
		SuccessPings: 0,
		FailedPings:  0,
	})
	return nil
}

func (m *MockOperationsProvider) RemoveNeighbor(nodeID string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	
	for i, n := range m.neighbors {
		if n.NodeID == nodeID {
			m.neighbors = append(m.neighbors[:i], m.neighbors[i+1:]...)
			return nil
		}
	}
	return errors.New("邻居不存在")
}

func (m *MockOperationsProvider) PingNeighbor(nodeID string) (*PingResult, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	
	for _, n := range m.neighbors {
		if n.NodeID == nodeID {
			return &PingResult{
				NodeID:    nodeID,
				Online:    n.Status == "online",
				LatencyMs: 50 + int64(len(nodeID)%100),
			}, nil
		}
	}
	return &PingResult{
		NodeID: nodeID,
		Online: false,
		Error:  "邻居不存在",
	}, nil
}

// ========== 邮箱实现 ==========

func (m *MockOperationsProvider) SendMail(to, subject, content string) (*SendMailResult, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	
	msgID := generateMessageID(to + subject + content)
	msg := &MailMessage{
		ID:        msgID,
		From:      "self",
		To:        to,
		Subject:   subject,
		Content:   content,
		Timestamp: time.Now().Format(time.RFC3339),
		Status:    "pending",
		Encrypted: false,
	}
	m.outbox = append([]*MailMessage{msg}, m.outbox...)
	
	return &SendMailResult{
		MessageID: msgID,
		Status:    "pending",
	}, nil
}

func (m *MockOperationsProvider) GetInbox(limit, offset int) (*MailboxResponse, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	
	total := len(m.inbox)
	if offset >= total {
		return &MailboxResponse{
			Messages: []*MailSummary{},
			Total:    total,
			Offset:   offset,
			Limit:    limit,
		}, nil
	}
	
	end := offset + limit
	if end > total {
		end = total
	}
	
	msgs := make([]*MailSummary, 0, end-offset)
	for _, msg := range m.inbox[offset:end] {
		msgs = append(msgs, &MailSummary{
			ID:        msg.ID,
			From:      msg.From,
			To:        msg.To,
			Subject:   msg.Subject,
			Timestamp: msg.Timestamp,
			Status:    msg.Status,
			Encrypted: msg.Encrypted,
		})
	}
	
	return &MailboxResponse{
		Messages: msgs,
		Total:    total,
		Offset:   offset,
		Limit:    limit,
	}, nil
}

func (m *MockOperationsProvider) GetOutbox(limit, offset int) (*MailboxResponse, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	
	total := len(m.outbox)
	if offset >= total {
		return &MailboxResponse{
			Messages: []*MailSummary{},
			Total:    total,
			Offset:   offset,
			Limit:    limit,
		}, nil
	}
	
	end := offset + limit
	if end > total {
		end = total
	}
	
	msgs := make([]*MailSummary, 0, end-offset)
	for _, msg := range m.outbox[offset:end] {
		msgs = append(msgs, &MailSummary{
			ID:        msg.ID,
			From:      msg.From,
			To:        msg.To,
			Subject:   msg.Subject,
			Timestamp: msg.Timestamp,
			Status:    msg.Status,
			Encrypted: msg.Encrypted,
		})
	}
	
	return &MailboxResponse{
		Messages: msgs,
		Total:    total,
		Offset:   offset,
		Limit:    limit,
	}, nil
}

func (m *MockOperationsProvider) ReadMail(messageID string) (*MailMessage, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	
	// 先查收件箱
	for _, msg := range m.inbox {
		if msg.ID == messageID {
			return msg, nil
		}
	}
	// 再查发件箱
	for _, msg := range m.outbox {
		if msg.ID == messageID {
			return msg, nil
		}
	}
	return nil, errors.New("邮件不存在")
}

func (m *MockOperationsProvider) MarkMailRead(messageID string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	
	for _, msg := range m.inbox {
		if msg.ID == messageID {
			msg.Status = "read"
			readAt := time.Now().Format(time.RFC3339)
			msg.ReadAt = readAt
			return nil
		}
	}
	return errors.New("邮件不存在")
}

func (m *MockOperationsProvider) DeleteMail(messageID string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	
	// 从收件箱删除
	for i, msg := range m.inbox {
		if msg.ID == messageID {
			m.inbox = append(m.inbox[:i], m.inbox[i+1:]...)
			return nil
		}
	}
	// 从发件箱删除
	for i, msg := range m.outbox {
		if msg.ID == messageID {
			m.outbox = append(m.outbox[:i], m.outbox[i+1:]...)
			return nil
		}
	}
	return errors.New("邮件不存在")
}

// ========== 留言板实现 ==========

func (m *MockOperationsProvider) PublishBulletin(topic, content string, ttl int) (*PublishResult, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	
	msgID := generateMessageID(topic + content)
	msg := &BulletinMessage{
		MessageID:  msgID,
		Author:     "self",
		Topic:      topic,
		Content:    content,
		Timestamp:  time.Now().Format(time.RFC3339),
		ExpiresAt:  time.Now().Add(time.Duration(ttl) * time.Second).Format(time.RFC3339),
		Status:     "active",
		Reputation: 80.0,
	}
	m.bulletins = append([]*BulletinMessage{msg}, m.bulletins...)
	
	return &PublishResult{
		MessageID: msgID,
		Topic:     topic,
		Status:    "published",
	}, nil
}

func (m *MockOperationsProvider) GetBulletinByTopic(topic string, limit int) ([]*BulletinMessage, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	
	result := make([]*BulletinMessage, 0)
	for _, msg := range m.bulletins {
		if msg.Topic == topic && msg.Status != "revoked" {
			result = append(result, msg)
			if len(result) >= limit {
				break
			}
		}
	}
	return result, nil
}

func (m *MockOperationsProvider) GetBulletinByAuthor(author string, limit int) ([]*BulletinMessage, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	
	result := make([]*BulletinMessage, 0)
	for _, msg := range m.bulletins {
		if msg.Author == author && msg.Status != "revoked" {
			result = append(result, msg)
			if len(result) >= limit {
				break
			}
		}
	}
	return result, nil
}

func (m *MockOperationsProvider) SearchBulletin(keyword string, limit int) ([]*BulletinMessage, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	
	result := make([]*BulletinMessage, 0)
	for _, msg := range m.bulletins {
		if msg.Status != "revoked" && 
		   (containsString(msg.Content, keyword) || containsString(msg.Topic, keyword)) {
			result = append(result, msg)
			if len(result) >= limit {
				break
			}
		}
	}
	return result, nil
}

func (m *MockOperationsProvider) SubscribeTopic(topic string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	
	for _, t := range m.subscriptions {
		if t == topic {
			return errors.New("已订阅该话题")
		}
	}
	m.subscriptions = append(m.subscriptions, topic)
	return nil
}

func (m *MockOperationsProvider) UnsubscribeTopic(topic string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	
	for i, t := range m.subscriptions {
		if t == topic {
			m.subscriptions = append(m.subscriptions[:i], m.subscriptions[i+1:]...)
			return nil
		}
	}
	return errors.New("未订阅该话题")
}

func (m *MockOperationsProvider) RevokeBulletin(messageID string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	
	for _, msg := range m.bulletins {
		if msg.MessageID == messageID {
			if msg.Author != "self" {
				return errors.New("只能撤回自己的留言")
			}
			msg.Status = "revoked"
			return nil
		}
	}
	return errors.New("留言不存在")
}

func (m *MockOperationsProvider) GetSubscriptions() ([]string, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	
	result := make([]string, len(m.subscriptions))
	copy(result, m.subscriptions)
	return result, nil
}

// ========== 声誉实现 ==========

func (m *MockOperationsProvider) GetReputation(nodeID string) (*ReputationInfo, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	
	if rep, ok := m.reputations[nodeID]; ok {
		return rep, nil
	}
	return &ReputationInfo{
		NodeID:     nodeID,
		Reputation: 50.0,
	}, nil
}

func (m *MockOperationsProvider) GetReputationRanking(limit int) ([]*ReputationInfo, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	
	result := make([]*ReputationInfo, 0, len(m.reputations))
	for _, rep := range m.reputations {
		result = append(result, rep)
	}
	
	// 按声誉排序
	sort.Slice(result, func(i, j int) bool {
		return result[i].Reputation > result[j].Reputation
	})
	
	// 更新排名
	for i := range result {
		result[i].Rank = i + 1
	}
	
	if limit > len(result) {
		limit = len(result)
	}
	return result[:limit], nil
}

// ========== 消息实现 ==========

func (m *MockOperationsProvider) SendDirectMessage(to, msgType, content string) (*SendMessageResult, error) {
	msgID := generateMessageID(to + msgType + content)
	return &SendMessageResult{
		MessageID: msgID,
		Status:    "sent",
	}, nil
}

func (m *MockOperationsProvider) BroadcastMessage(content string) (*BroadcastResult, error) {
	msgID := generateMessageID("broadcast" + content)
	return &BroadcastResult{
		MessageID:    msgID,
		ReachedCount: len(m.neighbors),
	}, nil
}

// ========== 辅助函数 ==========

func generateMessageID(data string) string {
	hash := sha256.Sum256([]byte(data + fmt.Sprintf("%d", time.Now().UnixNano())))
	return hex.EncodeToString(hash[:])[:16]
}

func containsString(s, substr string) bool {
	return len(s) >= len(substr) && (s == substr || len(substr) == 0 ||
		(len(s) > 0 && len(substr) > 0 && findSubstring(s, substr)))
}

func findSubstring(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}
