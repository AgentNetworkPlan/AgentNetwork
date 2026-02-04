// Package sync - 统一同步管理器
// 整合邮件路由、留言板同步、加密、回执和发现功能
package sync

import (
	"context"
	"encoding/json"
	"fmt"
	"sync"
)

// ManagerConfig 管理器配置
type ManagerConfig struct {
	NodeID            string
	DataDir           string
	EnableEncryption  bool
	EnableReceipts    bool
	EnableDiscovery   bool
	EnableMailSync    bool
	EnableBulletinSync bool
}

// DefaultManagerConfig 默认管理器配置
func DefaultManagerConfig(nodeID string) *ManagerConfig {
	return &ManagerConfig{
		NodeID:            nodeID,
		DataDir:           "./data/sync",
		EnableEncryption:  true,
		EnableReceipts:    true,
		EnableDiscovery:   true,
		EnableMailSync:    true,
		EnableBulletinSync: true,
	}
}

// SyncManager 同步管理器
type SyncManager struct {
	config *ManagerConfig
	
	// 子模块
	mailRouter      *MailRouter
	bulletinSyncer  *BulletinSyncer
	encryptor       *E2EEncryptor
	receiptManager  *ReceiptManager
	autoDiscovery   *AutoDiscovery
	
	// 外部依赖
	connector   PeerConnector
	signer      MessageSigner
	neighbors   NeighborProvider
	reputation  ReputationChecker
	
	// 回调
	onMailReceived     func(*SyncMessage, *MailPayload)
	onBulletinReceived func(*BulletinPayload)
	onPeerDiscovered   func(*DiscoveredPeer)
	onDeliveryReceipt  func(*DeliveryReceipt)
	onReadReceipt      func(*ReadReceipt)
	
	mu     sync.RWMutex
	ctx    context.Context
	cancel context.CancelFunc
	wg     sync.WaitGroup
}

// NewSyncManager 创建同步管理器
func NewSyncManager(config *ManagerConfig) (*SyncManager, error) {
	if config == nil {
		config = DefaultManagerConfig("")
	}
	
	ctx, cancel := context.WithCancel(context.Background())
	
	sm := &SyncManager{
		config: config,
		ctx:    ctx,
		cancel: cancel,
	}
	
	// 初始化子模块
	if config.EnableMailSync {
		sm.mailRouter = NewMailRouter(DefaultRouterConfig(config.NodeID))
	}
	
	if config.EnableBulletinSync {
		sm.bulletinSyncer = NewBulletinSyncer(DefaultSyncerConfig(config.NodeID))
	}
	
	if config.EnableEncryption {
		encryptor, err := NewE2EEncryptor(DefaultEncryptorConfig(config.NodeID))
		if err != nil {
			cancel()
			return nil, fmt.Errorf("create encryptor: %w", err)
		}
		sm.encryptor = encryptor
	}
	
	if config.EnableReceipts {
		sm.receiptManager = NewReceiptManager(DefaultReceiptManagerConfig(config.NodeID))
	}
	
	if config.EnableDiscovery {
		sm.autoDiscovery = NewAutoDiscovery(DefaultDiscoveryConfig(config.NodeID))
	}
	
	return sm, nil
}

// SetPeerConnector 设置节点连接器
func (sm *SyncManager) SetPeerConnector(c PeerConnector) {
	sm.connector = c
	
	if sm.mailRouter != nil {
		sm.mailRouter.SetPeerConnector(c)
	}
	if sm.bulletinSyncer != nil {
		sm.bulletinSyncer.SetPeerConnector(c)
	}
	if sm.autoDiscovery != nil {
		sm.autoDiscovery.SetPeerConnector(c)
	}
}

// SetSigner 设置签名器
func (sm *SyncManager) SetSigner(s MessageSigner) {
	sm.signer = s
	
	if sm.mailRouter != nil {
		sm.mailRouter.SetSigner(s)
	}
	if sm.bulletinSyncer != nil {
		sm.bulletinSyncer.SetSigner(s)
	}
	if sm.autoDiscovery != nil {
		sm.autoDiscovery.SetSigner(s)
	}
}

// SetNeighborProvider 设置邻居提供者
func (sm *SyncManager) SetNeighborProvider(n NeighborProvider) {
	sm.neighbors = n
	
	if sm.mailRouter != nil {
		sm.mailRouter.SetNeighborProvider(n)
	}
	if sm.bulletinSyncer != nil {
		sm.bulletinSyncer.SetNeighborProvider(n)
	}
	if sm.autoDiscovery != nil {
		sm.autoDiscovery.SetNeighborProvider(n)
	}
}

// SetReputationChecker 设置声誉检查器
func (sm *SyncManager) SetReputationChecker(rc ReputationChecker) {
	sm.reputation = rc
	
	if sm.mailRouter != nil {
		sm.mailRouter.SetReputationChecker(rc)
	}
	if sm.bulletinSyncer != nil {
		sm.bulletinSyncer.SetReputationChecker(rc)
	}
	if sm.autoDiscovery != nil {
		sm.autoDiscovery.SetReputationChecker(rc)
	}
}

// SetBulletinStore 设置留言板存储
func (sm *SyncManager) SetBulletinStore(store BulletinStore) {
	if sm.bulletinSyncer != nil {
		sm.bulletinSyncer.SetStore(store)
	}
}

// SetOnMailReceived 设置邮件接收回调
func (sm *SyncManager) SetOnMailReceived(fn func(*SyncMessage, *MailPayload)) {
	sm.onMailReceived = fn
}

// SetOnBulletinReceived 设置留言板消息接收回调
func (sm *SyncManager) SetOnBulletinReceived(fn func(*BulletinPayload)) {
	sm.onBulletinReceived = fn
}

// SetOnPeerDiscovered 设置节点发现回调
func (sm *SyncManager) SetOnPeerDiscovered(fn func(*DiscoveredPeer)) {
	sm.onPeerDiscovered = fn
}

// SetOnDeliveryReceipt 设置送达回执回调
func (sm *SyncManager) SetOnDeliveryReceipt(fn func(*DeliveryReceipt)) {
	sm.onDeliveryReceipt = fn
}

// SetOnReadReceipt 设置已读回执回调
func (sm *SyncManager) SetOnReadReceipt(fn func(*ReadReceipt)) {
	sm.onReadReceipt = fn
}

// Start 启动同步管理器
func (sm *SyncManager) Start() error {
	// 设置内部回调
	sm.setupCallbacks()
	
	// 启动子模块
	if sm.mailRouter != nil {
		sm.mailRouter.Start()
	}
	
	if sm.bulletinSyncer != nil {
		sm.bulletinSyncer.Start()
	}
	
	if sm.receiptManager != nil {
		sm.receiptManager.Start()
	}
	
	if sm.autoDiscovery != nil {
		if err := sm.autoDiscovery.Start(); err != nil {
			return err
		}
	}
	
	return nil
}

// Stop 停止同步管理器
func (sm *SyncManager) Stop() {
	sm.cancel()
	
	if sm.mailRouter != nil {
		sm.mailRouter.Stop()
	}
	
	if sm.bulletinSyncer != nil {
		sm.bulletinSyncer.Stop()
	}
	
	if sm.receiptManager != nil {
		sm.receiptManager.Stop()
	}
	
	if sm.autoDiscovery != nil {
		sm.autoDiscovery.Stop()
	}
	
	sm.wg.Wait()
}

// setupCallbacks 设置内部回调
func (sm *SyncManager) setupCallbacks() {
	if sm.mailRouter != nil {
		sm.mailRouter.SetOnReceive(func(msg *SyncMessage, payload *MailPayload) {
			// 解密内容（如果需要）
			if msg.Encrypted && sm.encryptor != nil {
				decrypted, err := sm.encryptor.Decrypt(msg.Sender, payload.Content)
				if err == nil {
					payload.Content = decrypted
				}
			}
			
			if sm.onMailReceived != nil {
				sm.onMailReceived(msg, payload)
			}
		})
		
		sm.mailRouter.SetOnDelivered(func(receipt *DeliveryReceipt) {
			if sm.receiptManager != nil {
				sm.receiptManager.MarkDelivered(receipt.MessageID, receipt.DeliveredAt)
			}
			if sm.onDeliveryReceipt != nil {
				sm.onDeliveryReceipt(receipt)
			}
		})
		
		sm.mailRouter.SetOnRead(func(receipt *ReadReceipt) {
			if sm.receiptManager != nil {
				sm.receiptManager.MarkRead(receipt.MessageID, receipt.ReadAt)
			}
			if sm.onReadReceipt != nil {
				sm.onReadReceipt(receipt)
			}
		})
	}
	
	if sm.bulletinSyncer != nil {
		sm.bulletinSyncer.SetOnMessageReceived(func(payload *BulletinPayload) {
			if sm.onBulletinReceived != nil {
				sm.onBulletinReceived(payload)
			}
		})
	}
	
	if sm.autoDiscovery != nil {
		sm.autoDiscovery.SetOnPeerDiscovered(func(peer *DiscoveredPeer) {
			if sm.onPeerDiscovered != nil {
				sm.onPeerDiscovered(peer)
			}
		})
	}
}

// SendMail 发送邮件
func (sm *SyncManager) SendMail(receiver, subject string, content []byte, encrypt bool) error {
	if sm.mailRouter == nil {
		return fmt.Errorf("mail sync not enabled")
	}
	
	// 加密内容（如果需要）
	if encrypt && sm.encryptor != nil {
		encrypted, err := sm.encryptor.Encrypt(receiver, content)
		if err != nil {
			return fmt.Errorf("encrypt content: %w", err)
		}
		content = encrypted
	}
	
	return sm.mailRouter.SendMail(receiver, subject, content, encrypt)
}

// SendMailWithReceipt 发送邮件并跟踪回执
func (sm *SyncManager) SendMailWithReceipt(receiver, subject string, content []byte, encrypt bool) (string, error) {
	if sm.mailRouter == nil {
		return "", fmt.Errorf("mail sync not enabled")
	}
	
	messageID := generateID()
	
	// 跟踪消息
	if sm.receiptManager != nil {
		sm.receiptManager.TrackMessage(messageID, sm.config.NodeID, receiver)
	}
	
	// 加密内容
	if encrypt && sm.encryptor != nil {
		encrypted, err := sm.encryptor.Encrypt(receiver, content)
		if err != nil {
			return "", fmt.Errorf("encrypt content: %w", err)
		}
		content = encrypted
	}
	
	if err := sm.mailRouter.SendMail(receiver, subject, content, encrypt); err != nil {
		if sm.receiptManager != nil {
			sm.receiptManager.MarkFailed(messageID, err.Error())
		}
		return "", err
	}
	
	return messageID, nil
}

// PublishBulletin 发布留言板消息
func (sm *SyncManager) PublishBulletin(msg *BulletinPayload) error {
	if sm.bulletinSyncer == nil {
		return fmt.Errorf("bulletin sync not enabled")
	}
	
	return sm.bulletinSyncer.PublishMessage(msg)
}

// SubscribeTopic 订阅话题
func (sm *SyncManager) SubscribeTopic(topic string) {
	if sm.bulletinSyncer != nil {
		sm.bulletinSyncer.Subscribe(topic)
	}
}

// UnsubscribeTopic 取消订阅话题
func (sm *SyncManager) UnsubscribeTopic(topic string) {
	if sm.bulletinSyncer != nil {
		sm.bulletinSyncer.Unsubscribe(topic)
	}
}

// SyncTopic 同步话题
func (sm *SyncManager) SyncTopic(topic string) error {
	if sm.bulletinSyncer == nil {
		return fmt.Errorf("bulletin sync not enabled")
	}
	
	return sm.bulletinSyncer.SyncTopic(topic)
}

// SendReadReceipt 发送已读回执
func (sm *SyncManager) SendReadReceipt(sender, messageID string) error {
	if sm.mailRouter == nil {
		return fmt.Errorf("mail sync not enabled")
	}
	
	return sm.mailRouter.SendReadReceipt(sender, messageID)
}

// GetMessageReceipt 获取消息回执状态
func (sm *SyncManager) GetMessageReceipt(messageID string) *MessageReceipt {
	if sm.receiptManager == nil {
		return nil
	}
	
	return sm.receiptManager.GetReceipt(messageID)
}

// GetReceiptStats 获取回执统计
func (sm *SyncManager) GetReceiptStats() map[string]int {
	if sm.receiptManager == nil {
		return nil
	}
	
	return sm.receiptManager.GetStats()
}

// GetDiscoveredPeers 获取发现的节点
func (sm *SyncManager) GetDiscoveredPeers() []*DiscoveredPeer {
	if sm.autoDiscovery == nil {
		return nil
	}
	
	return sm.autoDiscovery.GetDiscoveredPeers()
}

// GetDiscoveryStats 获取发现统计
func (sm *SyncManager) GetDiscoveryStats() map[string]int {
	if sm.autoDiscovery == nil {
		return nil
	}
	
	return sm.autoDiscovery.GetStats()
}

// SetPeerPublicKey 设置对方公钥（用于加密）
func (sm *SyncManager) SetPeerPublicKey(peerID, publicKey string) error {
	if sm.encryptor == nil {
		return fmt.Errorf("encryption not enabled")
	}
	
	return sm.encryptor.SetPeerPublicKey(peerID, publicKey)
}

// GetPublicKey 获取本节点公钥
func (sm *SyncManager) GetPublicKey() string {
	if sm.encryptor == nil {
		return ""
	}
	
	return sm.encryptor.GetPublicKey()
}

// HandleMessage 处理收到的P2P消息
func (sm *SyncManager) HandleMessage(data []byte) error {
	// 先尝试解析消息类型
	var msg SyncMessage
	if err := json.Unmarshal(data, &msg); err != nil {
		return err
	}
	
	// 根据消息类型分发到对应处理器
	switch msg.Type {
	case TypeMailSend, TypeMailRelay, TypeMailDelivered, TypeMailRead:
		if sm.mailRouter != nil {
			return sm.mailRouter.HandleMessage(data)
		}
	case TypeBulletinPublish, TypeBulletinSync, TypeBulletinQuery, TypeBulletinResp:
		if sm.bulletinSyncer != nil {
			return sm.bulletinSyncer.HandleMessage(data)
		}
	case TypePeerAnnounce, TypePeerQuery, TypePeerResponse:
		if sm.autoDiscovery != nil {
			return sm.autoDiscovery.HandleMessage(data)
		}
	}
	
	return nil
}

// GetStats 获取综合统计
func (sm *SyncManager) GetStats() map[string]interface{} {
	stats := make(map[string]interface{})
	
	if sm.receiptManager != nil {
		stats["receipts"] = sm.receiptManager.GetStats()
	}
	
	if sm.autoDiscovery != nil {
		stats["discovery"] = sm.autoDiscovery.GetStats()
	}
	
	return stats
}

// ExportState 导出状态（用于持久化）
func (sm *SyncManager) ExportState() ([]byte, error) {
	state := make(map[string]interface{})
	
	if sm.receiptManager != nil {
		data, err := sm.receiptManager.ExportReceipts()
		if err == nil {
			state["receipts"] = string(data)
		}
	}
	
	return json.Marshal(state)
}

// ImportState 导入状态（从持久化恢复）
func (sm *SyncManager) ImportState(data []byte) error {
	var state map[string]interface{}
	if err := json.Unmarshal(data, &state); err != nil {
		return err
	}
	
	if receiptsData, ok := state["receipts"].(string); ok && sm.receiptManager != nil {
		sm.receiptManager.ImportReceipts([]byte(receiptsData))
	}
	
	return nil
}

// Encryptor 获取加密器（用于高级操作）
func (sm *SyncManager) Encryptor() *E2EEncryptor {
	return sm.encryptor
}

// MailRouter 获取邮件路由器
func (sm *SyncManager) MailRouter() *MailRouter {
	return sm.mailRouter
}

// BulletinSyncer 获取留言板同步器
func (sm *SyncManager) BulletinSyncer() *BulletinSyncer {
	return sm.bulletinSyncer
}

// AutoDiscovery 获取自动发现服务
func (sm *SyncManager) AutoDiscovery() *AutoDiscovery {
	return sm.autoDiscovery
}

// ReceiptManager 获取回执管理器
func (sm *SyncManager) ReceiptManager() *ReceiptManager {
	return sm.receiptManager
}
