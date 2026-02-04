package webadmin

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/AgentNetworkPlan/AgentNetwork/internal/bulletin"
	"github.com/AgentNetworkPlan/AgentNetwork/internal/mailbox"
	"github.com/AgentNetworkPlan/AgentNetwork/internal/neighbor"
	"github.com/AgentNetworkPlan/AgentNetwork/internal/security"
	"github.com/libp2p/go-libp2p/core/peer"
	"github.com/multiformats/go-multiaddr"
)

// RealOperationsProvider 真实的操作提供者，集成邻居、邮箱、留言板功能
type RealOperationsProvider struct {
	nodeID          string
	neighborManager *neighbor.NeighborManager
	mailbox         *mailbox.Mailbox
	bulletinBoard   *bulletin.BulletinBoard
	
	// 安全管理器
	securityManager *security.SecurityManager
	
	// P2P 连接功能
	connectFunc     func(ctx context.Context, peerInfo peer.AddrInfo) error
	findPeerFunc    func(ctx context.Context, id peer.ID) (peer.AddrInfo, error)
	getPeersFunc    func() []peer.ID
	
	// 消息发送功能
	sendMessageFunc      func(to string, msgType string, content []byte) error
	broadcastMessageFunc func(content []byte) (int, error)
}

// NewRealOperationsProvider 创建真实操作提供者
func NewRealOperationsProvider(nodeID string) *RealOperationsProvider {
	return &RealOperationsProvider{
		nodeID:          nodeID,
		securityManager: security.NewSecurityManager(),
	}
}

// SetSecurityManager 设置安全管理器
func (p *RealOperationsProvider) SetSecurityManager(sm *security.SecurityManager) {
	p.securityManager = sm
}

// GetSecurityManager 获取安全管理器
func (p *RealOperationsProvider) GetSecurityManager() *security.SecurityManager {
	return p.securityManager
}

// SetNeighborManager 设置邻居管理器
func (p *RealOperationsProvider) SetNeighborManager(nm *neighbor.NeighborManager) {
	p.neighborManager = nm
}

// SetMailbox 设置邮箱
func (p *RealOperationsProvider) SetMailbox(mb *mailbox.Mailbox) {
	p.mailbox = mb
}

// SetBulletinBoard 设置留言板
func (p *RealOperationsProvider) SetBulletinBoard(bb *bulletin.BulletinBoard) {
	p.bulletinBoard = bb
}

// SetConnectFunc 设置连接函数
func (p *RealOperationsProvider) SetConnectFunc(fn func(ctx context.Context, peerInfo peer.AddrInfo) error) {
	p.connectFunc = fn
}

// SetFindPeerFunc 设置查找节点函数
func (p *RealOperationsProvider) SetFindPeerFunc(fn func(ctx context.Context, id peer.ID) (peer.AddrInfo, error)) {
	p.findPeerFunc = fn
}

// SetGetPeersFunc 设置获取节点列表函数
func (p *RealOperationsProvider) SetGetPeersFunc(fn func() []peer.ID) {
	p.getPeersFunc = fn
}

// SetSendMessageFunc 设置发送消息函数
func (p *RealOperationsProvider) SetSendMessageFunc(fn func(to string, msgType string, content []byte) error) {
	p.sendMessageFunc = fn
}

// SetBroadcastMessageFunc 设置广播消息函数
func (p *RealOperationsProvider) SetBroadcastMessageFunc(fn func(content []byte) (int, error)) {
	p.broadcastMessageFunc = fn
}

// ============ 邻居管理 ============

// GetNeighbors 获取邻居列表
func (p *RealOperationsProvider) GetNeighbors() ([]*NeighborInfo, error) {
	if p.neighborManager == nil {
		// 如果没有邻居管理器，从 P2P 层获取
		if p.getPeersFunc == nil {
			return nil, errors.New("neighbor manager not configured")
		}
		peers := p.getPeersFunc()
		result := make([]*NeighborInfo, 0, len(peers))
		for _, peerID := range peers {
			result = append(result, &NeighborInfo{
				NodeID:     peerID.String(),
				Type:       "peer",
				Status:     "online",
				TrustScore: 1.0,
				LastSeen:   time.Now().Format(time.RFC3339),
			})
		}
		return result, nil
	}
	
	neighbors := p.neighborManager.GetAllNeighbors()
	result := make([]*NeighborInfo, 0, len(neighbors))
	for _, n := range neighbors {
		result = append(result, &NeighborInfo{
			NodeID:       n.NodeID,
			PublicKey:    n.PublicKey,
			Type:         string(n.Type),
			Reputation:   n.Reputation,
			TrustScore:   n.TrustScore,
			Status:       string(n.PingStatus),
			LastSeen:     n.LastSeen.Format(time.RFC3339),
			Addresses:    n.Addresses,
			SuccessPings: n.SuccessfulPings,
			FailedPings:  n.FailedPings,
		})
	}
	return result, nil
}

// GetBestNeighbors 获取最佳邻居
func (p *RealOperationsProvider) GetBestNeighbors(count int) ([]*NeighborInfo, error) {
	if p.neighborManager == nil {
		return p.GetNeighbors()
	}
	
	neighbors := p.neighborManager.GetBestNeighbors(count)
	result := make([]*NeighborInfo, 0, len(neighbors))
	for _, n := range neighbors {
		result = append(result, &NeighborInfo{
			NodeID:       n.NodeID,
			PublicKey:    n.PublicKey,
			Type:         string(n.Type),
			Reputation:   n.Reputation,
			TrustScore:   n.TrustScore,
			Status:       string(n.PingStatus),
			LastSeen:     n.LastSeen.Format(time.RFC3339),
			Addresses:    n.Addresses,
			SuccessPings: n.SuccessfulPings,
			FailedPings:  n.FailedPings,
		})
	}
	return result, nil
}

// AddNeighbor 添加邻居
func (p *RealOperationsProvider) AddNeighbor(nodeID string, addresses []string) error {
	// 尝试连接
	if p.connectFunc != nil && len(addresses) > 0 {
		peerID, err := peer.Decode(nodeID)
		if err != nil {
			return fmt.Errorf("invalid peer ID: %w", err)
		}
		
		var addrs []multiaddr.Multiaddr
		for _, addrStr := range addresses {
			addr, err := multiaddr.NewMultiaddr(addrStr)
			if err != nil {
				continue
			}
			addrs = append(addrs, addr)
		}
		
		if len(addrs) > 0 {
			ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
			defer cancel()
			
			if err := p.connectFunc(ctx, peer.AddrInfo{
				ID:    peerID,
				Addrs: addrs,
			}); err != nil {
				return fmt.Errorf("failed to connect: %w", err)
			}
		}
	}
	
	// 添加到邻居管理器
	if p.neighborManager != nil {
		return p.neighborManager.AddNeighbor(&neighbor.Neighbor{
			NodeID:     nodeID,
			Type:       neighbor.TypeNormal,
			Addresses:  addresses,
			AddedAt:    time.Now(),
			LastSeen:   time.Now(),
			PingStatus: neighbor.StatusOnline,
			TrustScore: 0.5,
		})
	}
	
	return nil
}

// RemoveNeighbor 移除邻居
func (p *RealOperationsProvider) RemoveNeighbor(nodeID string) error {
	if p.neighborManager == nil {
		return errors.New("neighbor manager not configured")
	}
	return p.neighborManager.RemoveNeighbor(nodeID, "user request")
}

// PingNeighbor Ping邻居
func (p *RealOperationsProvider) PingNeighbor(nodeID string) (*PingResult, error) {
	result := &PingResult{
		NodeID: nodeID,
		Online: false,
	}
	
	if p.neighborManager != nil {
		startTime := time.Now()
		err := p.neighborManager.Ping(nodeID)
		latency := time.Since(startTime).Milliseconds()
		
		if err != nil {
			result.Error = err.Error()
			return result, nil
		}
		
		result.Online = true
		result.LatencyMs = latency
		return result, nil
	}
	
	// 如果没有邻居管理器，尝试通过 findPeer 检查
	if p.findPeerFunc != nil {
		peerID, err := peer.Decode(nodeID)
		if err != nil {
			result.Error = "invalid peer ID"
			return result, nil
		}
		
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		
		startTime := time.Now()
		_, err = p.findPeerFunc(ctx, peerID)
		latency := time.Since(startTime).Milliseconds()
		
		if err != nil {
			result.Error = err.Error()
			return result, nil
		}
		
		result.Online = true
		result.LatencyMs = latency
	}
	
	return result, nil
}

// ============ 邮箱操作 ============

// SendMail 发送邮件
func (p *RealOperationsProvider) SendMail(to, subject, content string) (*SendMailResult, error) {
	if p.mailbox == nil {
		return nil, errors.New("mailbox not configured")
	}
	
	// 安全检查：限流和声誉阈值
	if p.securityManager != nil {
		if err := p.securityManager.CheckMailboxSend(p.nodeID); err != nil {
			return nil, fmt.Errorf("security check failed: %w", err)
		}
	}
	
	msg, err := p.mailbox.SendMessage(to, subject, []byte(content), false)
	if err != nil {
		return nil, err
	}
	
	// 消费配额并记录行为
	if p.securityManager != nil {
		p.securityManager.ConsumeMailboxQuota(p.nodeID, to)
	}
	
	return &SendMailResult{
		MessageID: msg.ID,
		Status:    string(msg.Status),
	}, nil
}

// GetInbox 获取收件箱
func (p *RealOperationsProvider) GetInbox(limit, offset int) (*MailboxResponse, error) {
	if p.mailbox == nil {
		return &MailboxResponse{
			Messages: []*MailSummary{},
			Total:    0,
			Offset:   offset,
			Limit:    limit,
		}, nil
	}
	
	messages := p.mailbox.ListInbox(limit, offset)
	result := &MailboxResponse{
		Messages: make([]*MailSummary, 0, len(messages)),
		Total:    p.mailbox.GetInboxCount(),
		Offset:   offset,
		Limit:    limit,
	}
	
	for _, msg := range messages {
		result.Messages = append(result.Messages, &MailSummary{
			ID:        msg.ID,
			From:      msg.Sender,
			To:        "",
			Subject:   msg.Subject,
			Timestamp: msg.Timestamp.Format(time.RFC3339),
			Status:    string(msg.Status),
			Encrypted: msg.Encrypted,
		})
	}
	
	return result, nil
}

// GetOutbox 获取发件箱
func (p *RealOperationsProvider) GetOutbox(limit, offset int) (*MailboxResponse, error) {
	if p.mailbox == nil {
		return &MailboxResponse{
			Messages: []*MailSummary{},
			Total:    0,
			Offset:   offset,
			Limit:    limit,
		}, nil
	}
	
	messages := p.mailbox.ListOutbox(limit, offset)
	result := &MailboxResponse{
		Messages: make([]*MailSummary, 0, len(messages)),
		Total:    p.mailbox.GetOutboxCount(),
		Offset:   offset,
		Limit:    limit,
	}
	
	for _, msg := range messages {
		result.Messages = append(result.Messages, &MailSummary{
			ID:        msg.ID,
			From:      msg.Sender,
			To:        "",
			Subject:   msg.Subject,
			Timestamp: msg.Timestamp.Format(time.RFC3339),
			Status:    string(msg.Status),
			Encrypted: msg.Encrypted,
		})
	}
	
	return result, nil
}

// ReadMail 读取邮件
func (p *RealOperationsProvider) ReadMail(messageID string) (*MailMessage, error) {
	if p.mailbox == nil {
		return nil, errors.New("mailbox not configured")
	}
	
	msg, err := p.mailbox.GetMessage(messageID)
	if err != nil {
		return nil, err
	}
	
	result := &MailMessage{
		ID:        msg.ID,
		From:      msg.Sender,
		To:        msg.Receiver,
		Subject:   msg.Subject,
		Content:   string(msg.Content),
		Timestamp: msg.Timestamp.Format(time.RFC3339),
		Status:    string(msg.Status),
		Encrypted: msg.Encrypted,
	}
	
	if msg.ReadAt != nil {
		result.ReadAt = msg.ReadAt.Format(time.RFC3339)
	}
	
	return result, nil
}

// MarkMailRead 标记已读
func (p *RealOperationsProvider) MarkMailRead(messageID string) error {
	if p.mailbox == nil {
		return errors.New("mailbox not configured")
	}
	return p.mailbox.MarkAsRead(messageID)
}

// DeleteMail 删除邮件
func (p *RealOperationsProvider) DeleteMail(messageID string) error {
	if p.mailbox == nil {
		return errors.New("mailbox not configured")
	}
	return p.mailbox.DeleteMessage(messageID)
}

// ============ 留言板操作 ============

// PublishBulletin 发布留言
func (p *RealOperationsProvider) PublishBulletin(topic, content string, ttl int) (*PublishResult, error) {
	if p.bulletinBoard == nil {
		return nil, errors.New("bulletin board not configured")
	}
	
	// 安全检查：限流和声誉阈值
	if p.securityManager != nil {
		if err := p.securityManager.CheckBulletinPublish(p.nodeID); err != nil {
			return nil, fmt.Errorf("security check failed: %w", err)
		}
	}
	
	msg, err := p.bulletinBoard.PublishMessage(content, topic)
	if err != nil {
		return nil, err
	}
	
	// 消费配额并记录行为
	if p.securityManager != nil {
		p.securityManager.ConsumeBulletinQuota(p.nodeID, topic)
	}
	
	return &PublishResult{
		MessageID: msg.MessageID,
		Topic:     msg.Topic,
		Status:    string(msg.Status),
	}, nil
}

// GetBulletinByTopic 按话题获取留言
func (p *RealOperationsProvider) GetBulletinByTopic(topic string, limit int) ([]*BulletinMessage, error) {
	if p.bulletinBoard == nil {
		return []*BulletinMessage{}, nil
	}
	
	messages, err := p.bulletinBoard.QueryByTopic(topic, limit, 0)
	if err != nil {
		return nil, err
	}
	return convertBulletinMessages(messages), nil
}

// GetBulletinByAuthor 按作者获取留言
func (p *RealOperationsProvider) GetBulletinByAuthor(author string, limit int) ([]*BulletinMessage, error) {
	if p.bulletinBoard == nil {
		return []*BulletinMessage{}, nil
	}
	
	messages, err := p.bulletinBoard.QueryByAuthor(author, limit, 0)
	if err != nil {
		return nil, err
	}
	return convertBulletinMessages(messages), nil
}

// SearchBulletin 搜索留言
func (p *RealOperationsProvider) SearchBulletin(keyword string, limit int) ([]*BulletinMessage, error) {
	if p.bulletinBoard == nil {
		return []*BulletinMessage{}, nil
	}
	
	messages := p.bulletinBoard.SearchMessages(keyword, limit)
	return convertBulletinMessages(messages), nil
}

// SubscribeTopic 订阅话题
func (p *RealOperationsProvider) SubscribeTopic(topic string) error {
	if p.bulletinBoard == nil {
		return errors.New("bulletin board not configured")
	}
	return p.bulletinBoard.SubscribeTopic(topic, nil)
}

// UnsubscribeTopic 取消订阅
func (p *RealOperationsProvider) UnsubscribeTopic(topic string) error {
	if p.bulletinBoard == nil {
		return errors.New("bulletin board not configured")
	}
	return p.bulletinBoard.UnsubscribeTopic(topic)
}

// RevokeBulletin 撤回留言
func (p *RealOperationsProvider) RevokeBulletin(messageID string) error {
	if p.bulletinBoard == nil {
		return errors.New("bulletin board not configured")
	}
	return p.bulletinBoard.RevokeMessage(messageID)
}

// GetSubscriptions 获取订阅列表
func (p *RealOperationsProvider) GetSubscriptions() ([]string, error) {
	if p.bulletinBoard == nil {
		return []string{}, nil
	}
	subs := p.bulletinBoard.GetSubscriptions()
	topics := make([]string, 0, len(subs))
	for _, sub := range subs {
		topics = append(topics, sub.Topic)
	}
	return topics, nil
}

// ============ 声誉查询 ============

// GetReputation 获取声誉
func (p *RealOperationsProvider) GetReputation(nodeID string) (*ReputationInfo, error) {
	// 简单实现：从邻居管理器获取
	if p.neighborManager != nil {
		n, err := p.neighborManager.GetNeighbor(nodeID)
		if err == nil && n != nil {
			return &ReputationInfo{
				NodeID:     nodeID,
				Reputation: float64(n.Reputation),
			}, nil
		}
	}
	
	return &ReputationInfo{
		NodeID:     nodeID,
		Reputation: 0,
	}, nil
}

// GetReputationRanking 获取声誉排行
func (p *RealOperationsProvider) GetReputationRanking(limit int) ([]*ReputationInfo, error) {
	if p.neighborManager == nil {
		return []*ReputationInfo{}, nil
	}
	
	neighbors := p.neighborManager.GetBestNeighbors(limit)
	result := make([]*ReputationInfo, 0, len(neighbors))
	for i, n := range neighbors {
		result = append(result, &ReputationInfo{
			NodeID:     n.NodeID,
			Reputation: float64(n.Reputation),
			Rank:       i + 1,
		})
	}
	return result, nil
}

// ============ 消息发送 ============

// SendDirectMessage 发送直接消息
func (p *RealOperationsProvider) SendDirectMessage(to, msgType, content string) (*SendMessageResult, error) {
	if p.sendMessageFunc != nil {
		err := p.sendMessageFunc(to, msgType, []byte(content))
		if err != nil {
			return nil, err
		}
		return &SendMessageResult{
			MessageID: fmt.Sprintf("msg-%d", time.Now().UnixNano()),
			Status:    "sent",
		}, nil
	}
	
	// 使用邮箱发送
	if p.mailbox != nil {
		msg, err := p.mailbox.SendMessage(to, msgType, []byte(content), false)
		if err != nil {
			return nil, err
		}
		return &SendMessageResult{
			MessageID: msg.ID,
			Status:    string(msg.Status),
		}, nil
	}
	
	return nil, errors.New("message sending not configured")
}

// BroadcastMessage 广播消息
func (p *RealOperationsProvider) BroadcastMessage(content string) (*BroadcastResult, error) {
	if p.broadcastMessageFunc != nil {
		count, err := p.broadcastMessageFunc([]byte(content))
		if err != nil {
			return nil, err
		}
		return &BroadcastResult{
			MessageID:    fmt.Sprintf("broadcast-%d", time.Now().UnixNano()),
			ReachedCount: count,
		}, nil
	}
	
	// 使用留言板广播
	if p.bulletinBoard != nil {
		msg, err := p.bulletinBoard.PublishMessage(content, "broadcast")
		if err != nil {
			return nil, err
		}
		return &BroadcastResult{
			MessageID:    msg.MessageID,
			ReachedCount: 1, // 至少本地存储
		}, nil
	}
	
	return nil, errors.New("broadcast not configured")
}

// convertBulletinMessages 转换留言板消息
func convertBulletinMessages(messages []*bulletin.Message) []*BulletinMessage {
	result := make([]*BulletinMessage, 0, len(messages))
	for _, msg := range messages {
		result = append(result, &BulletinMessage{
			MessageID:  msg.MessageID,
			Author:     msg.Author,
			Topic:      msg.Topic,
			Content:    msg.Content,
			Timestamp:  msg.Timestamp.Format(time.RFC3339),
			ExpiresAt:  msg.ExpiresAt.Format(time.RFC3339),
			Status:     string(msg.Status),
			Tags:       msg.Tags,
			ReplyTo:    msg.ReplyTo,
			Reputation: msg.ReputationScore,
		})
	}
	return result
}

// ============ 安全相关 API ============

// GetRateLimitStatus 获取限流状态
func (p *RealOperationsProvider) GetRateLimitStatus() map[string]interface{} {
	if p.securityManager == nil {
		return map[string]interface{}{
			"enabled": false,
		}
	}
	
	bulletinStatus := p.securityManager.GetBulletinStatus(p.nodeID)
	mailboxStatus := p.securityManager.GetMailboxStatus(p.nodeID)
	
	return map[string]interface{}{
		"enabled":  true,
		"bulletin": bulletinStatus,
		"mailbox":  mailboxStatus,
	}
}

// GetSecurityReport 获取安全报告
func (p *RealOperationsProvider) GetSecurityReport() *security.SecurityReport {
	if p.securityManager == nil {
		return nil
	}
	return p.securityManager.GenerateSecurityReport()
}

// IsBlacklisted 检查节点是否被黑名单
func (p *RealOperationsProvider) IsBlacklisted(nodeID string) bool {
	if p.securityManager == nil {
		return false
	}
	return p.securityManager.IsBlacklisted(nodeID)
}
