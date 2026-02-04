// Package sync 实现P2P网络中的消息同步协议
// 支持邮件路由、留言板同步、端到端加密、消息确认和邻居自动发现
package sync

import (
	"encoding/json"
	"time"
)

// ProtocolID 协议标识符
const (
	MailSyncProtocol     = "/agentnetwork/mail/1.0.0"
	BulletinSyncProtocol = "/agentnetwork/bulletin/1.0.0"
	DiscoveryProtocol    = "/agentnetwork/discovery/1.0.0"
)

// MessageType 消息类型
type MessageType string

const (
	// 邮件相关
	TypeMailSend      MessageType = "mail_send"       // 发送邮件
	TypeMailDelivered MessageType = "mail_delivered"  // 送达确认
	TypeMailRead      MessageType = "mail_read"       // 已读回执
	TypeMailRelay     MessageType = "mail_relay"      // 中继请求

	// 留言板相关
	TypeBulletinPublish MessageType = "bulletin_publish"  // 发布留言
	TypeBulletinSync    MessageType = "bulletin_sync"     // 同步请求
	TypeBulletinQuery   MessageType = "bulletin_query"    // 查询消息
	TypeBulletinResp    MessageType = "bulletin_response" // 查询响应

	// 发现相关
	TypePeerAnnounce MessageType = "peer_announce" // 节点广播
	TypePeerQuery    MessageType = "peer_query"    // 查询节点
	TypePeerResponse MessageType = "peer_response" // 节点响应
)

// SyncMessage 同步消息结构
type SyncMessage struct {
	ID        string          `json:"id"`         // 消息唯一ID
	Type      MessageType     `json:"type"`       // 消息类型
	Sender    string          `json:"sender"`     // 发送者节点ID
	Receiver  string          `json:"receiver"`   // 接收者节点ID（可选）
	Timestamp time.Time       `json:"timestamp"`  // 时间戳
	TTL       int             `json:"ttl"`        // 生存时间/跳数
	Nonce     string          `json:"nonce"`      // 随机数防重放
	Payload   json.RawMessage `json:"payload"`    // 实际载荷
	Signature string          `json:"signature"`  // 签名
	Encrypted bool            `json:"encrypted"`  // 是否加密
}

// MailPayload 邮件载荷
type MailPayload struct {
	MessageID string `json:"message_id"` // 邮件ID
	Subject   string `json:"subject"`    // 主题
	Content   []byte `json:"content"`    // 内容（可能加密）
	ReplyTo   string `json:"reply_to"`   // 回复的消息ID
}

// DeliveryReceipt 送达回执
type DeliveryReceipt struct {
	MessageID   string    `json:"message_id"`   // 原消息ID
	DeliveredAt time.Time `json:"delivered_at"` // 送达时间
	ReceiverID  string    `json:"receiver_id"`  // 接收者ID
}

// ReadReceipt 已读回执
type ReadReceipt struct {
	MessageID string    `json:"message_id"` // 原消息ID
	ReadAt    time.Time `json:"read_at"`    // 阅读时间
	ReaderID  string    `json:"reader_id"`  // 阅读者ID
}

// BulletinPayload 留言板载荷
type BulletinPayload struct {
	MessageID       string    `json:"message_id"`
	Author          string    `json:"author"`
	Topic           string    `json:"topic"`
	Content         string    `json:"content"`
	Timestamp       time.Time `json:"timestamp"`
	ExpiresAt       time.Time `json:"expires_at"`
	ReputationScore float64   `json:"reputation_score"`
	Tags            []string  `json:"tags"`
	ReplyTo         string    `json:"reply_to"`
}

// BulletinSyncRequest 留言板同步请求
type BulletinSyncRequest struct {
	Topics    []string  `json:"topics"`     // 需要同步的话题
	SinceTime time.Time `json:"since_time"` // 从什么时间开始
	Limit     int       `json:"limit"`      // 最大消息数
}

// BulletinSyncResponse 留言板同步响应
type BulletinSyncResponse struct {
	Messages []BulletinPayload `json:"messages"` // 消息列表
	HasMore  bool              `json:"has_more"` // 是否还有更多
}

// PeerInfo 节点信息
type PeerInfo struct {
	NodeID     string    `json:"node_id"`     // 节点ID
	PublicKey  string    `json:"public_key"`  // 公钥
	Addresses  []string  `json:"addresses"`   // 地址列表
	Reputation float64   `json:"reputation"`  // 声誉值
	Roles      []string  `json:"roles"`       // 角色列表
	LastSeen   time.Time `json:"last_seen"`   // 最后在线时间
	Version    string    `json:"version"`     // 协议版本
}

// PeerAnnouncement 节点广播
type PeerAnnouncement struct {
	Info       PeerInfo `json:"info"`       // 节点信息
	Neighbors  []string `json:"neighbors"`  // 邻居列表
	Services   []string `json:"services"`   // 提供的服务
	Capacity   int      `json:"capacity"`   // 可用容量
}

// PeerQuery 节点查询
type PeerQuery struct {
	QueryID   string   `json:"query_id"`   // 查询ID
	TargetID  string   `json:"target_id"`  // 目标节点ID（可选）
	MaxHops   int      `json:"max_hops"`   // 最大跳数
	Filters   []string `json:"filters"`    // 过滤条件
	RequestBy string   `json:"request_by"` // 请求者ID
}

// PeerQueryResponse 节点查询响应
type PeerQueryResponse struct {
	QueryID string     `json:"query_id"` // 原查询ID
	Peers   []PeerInfo `json:"peers"`    // 发现的节点
	HopPath []string   `json:"hop_path"` // 跳转路径
}
