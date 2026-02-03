package message

import (
	"encoding/json"
	"time"
)

// Type 消息类型
type Type string

const (
	TypeHeartbeat   Type = "heartbeat"
	TypePing        Type = "ping"
	TypePong        Type = "pong"
	TypeFindNode    Type = "find_node"
	TypeFoundNode   Type = "found_node"
	TypeAnnounce    Type = "announce"
	TypeProposal    Type = "proposal"
	TypeVote        Type = "vote"
)

// Message 通用消息结构
type Message struct {
	Version   string          `json:"version"`
	Type      Type            `json:"type"`
	ID        string          `json:"id"`
	From      string          `json:"from"`
	To        string          `json:"to,omitempty"`
	Timestamp string          `json:"timestamp"`
	Payload   json.RawMessage `json:"payload"`
	Signature string          `json:"signature"`
}

// NewMessage 创建新消息
func NewMessage(msgType Type, from string, payload interface{}) (*Message, error) {
	payloadData, err := json.Marshal(payload)
	if err != nil {
		return nil, err
	}

	return &Message{
		Version:   "0.1.0",
		Type:      msgType,
		ID:        generateID(),
		From:      from,
		Timestamp: time.Now().UTC().Format(time.RFC3339),
		Payload:   payloadData,
	}, nil
}

// GetPayload 解析 payload
func (m *Message) GetPayload(v interface{}) error {
	return json.Unmarshal(m.Payload, v)
}

// Marshal 序列化消息
func (m *Message) Marshal() ([]byte, error) {
	return json.Marshal(m)
}

// Unmarshal 反序列化消息
func Unmarshal(data []byte) (*Message, error) {
	var msg Message
	if err := json.Unmarshal(data, &msg); err != nil {
		return nil, err
	}
	return &msg, nil
}

// generateID 生成消息 ID
func generateID() string {
	return time.Now().Format("20060102150405.000000000")
}

// PingPayload Ping 消息 payload
type PingPayload struct {
	Nonce int64 `json:"nonce"`
}

// PongPayload Pong 消息 payload
type PongPayload struct {
	Nonce int64 `json:"nonce"`
}

// FindNodePayload FindNode 消息 payload
type FindNodePayload struct {
	TargetID string `json:"target_id"`
}

// FoundNodePayload FoundNode 消息 payload
type FoundNodePayload struct {
	Nodes []NodeInfo `json:"nodes"`
}

// NodeInfo 节点信息
type NodeInfo struct {
	ID      string `json:"id"`
	Address string `json:"address"`
	Port    int    `json:"port"`
}

// ProposalPayload 提案消息 payload
type ProposalPayload struct {
	ProposalID  string `json:"proposal_id"`
	Title       string `json:"title"`
	Description string `json:"description"`
	Author      string `json:"author"`
	Category    string `json:"category"` // RFC, Accusation, etc.
}

// VotePayload 投票消息 payload
type VotePayload struct {
	ProposalID string `json:"proposal_id"`
	Vote       string `json:"vote"` // approve, reject, abstain
	Reason     string `json:"reason,omitempty"`
}
