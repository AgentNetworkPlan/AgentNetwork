package message

import (
	"encoding/json"
	"testing"
	"time"
)

func TestNewMessage(t *testing.T) {
	payload := map[string]string{"key": "value"}

	msg, err := NewMessage(TypeHeartbeat, "agent-123", payload)
	if err != nil {
		t.Fatalf("创建消息失败: %v", err)
	}

	if msg.Type != TypeHeartbeat {
		t.Errorf("消息类型错误: %s", msg.Type)
	}

	if msg.From != "agent-123" {
		t.Errorf("发送者错误: %s", msg.From)
	}

	if msg.Version != "0.1.0" {
		t.Errorf("版本错误: %s", msg.Version)
	}

	if msg.ID == "" {
		t.Error("消息 ID 为空")
	}

	if msg.Timestamp == "" {
		t.Error("时间戳为空")
	}
}

func TestMessage_GetPayload(t *testing.T) {
	type TestPayload struct {
		Name  string `json:"name"`
		Value int    `json:"value"`
	}

	original := TestPayload{Name: "test", Value: 42}
	msg, _ := NewMessage(TypePing, "sender", original)

	var decoded TestPayload
	if err := msg.GetPayload(&decoded); err != nil {
		t.Fatalf("解析 payload 失败: %v", err)
	}

	if decoded.Name != "test" {
		t.Errorf("Name 错误: %s", decoded.Name)
	}

	if decoded.Value != 42 {
		t.Errorf("Value 错误: %d", decoded.Value)
	}
}

func TestMessage_Marshal(t *testing.T) {
	msg, _ := NewMessage(TypePong, "sender", map[string]int{"nonce": 123})

	data, err := msg.Marshal()
	if err != nil {
		t.Fatalf("序列化失败: %v", err)
	}

	if len(data) == 0 {
		t.Error("序列化结果为空")
	}

	// 验证是有效的 JSON
	var parsed map[string]interface{}
	if err := json.Unmarshal(data, &parsed); err != nil {
		t.Errorf("序列化结果不是有效 JSON: %v", err)
	}
}

func TestUnmarshal(t *testing.T) {
	original, _ := NewMessage(TypeFindNode, "sender", FindNodePayload{TargetID: "target-123"})
	data, _ := original.Marshal()

	msg, err := Unmarshal(data)
	if err != nil {
		t.Fatalf("反序列化失败: %v", err)
	}

	if msg.Type != TypeFindNode {
		t.Errorf("类型错误: %s", msg.Type)
	}

	if msg.From != "sender" {
		t.Errorf("发送者错误: %s", msg.From)
	}

	var payload FindNodePayload
	if err := msg.GetPayload(&payload); err != nil {
		t.Fatalf("解析 payload 失败: %v", err)
	}

	if payload.TargetID != "target-123" {
		t.Errorf("TargetID 错误: %s", payload.TargetID)
	}
}

func TestUnmarshal_Invalid(t *testing.T) {
	_, err := Unmarshal([]byte("invalid json"))
	if err == nil {
		t.Error("无效 JSON 应该返回错误")
	}
}

func TestMessageTypes(t *testing.T) {
	types := []Type{
		TypeHeartbeat,
		TypePing,
		TypePong,
		TypeFindNode,
		TypeFoundNode,
		TypeAnnounce,
		TypeProposal,
		TypeVote,
	}

	for _, typ := range types {
		if typ == "" {
			t.Error("消息类型不应该为空")
		}
	}
}

func TestPingPayload(t *testing.T) {
	payload := PingPayload{Nonce: 12345}

	msg, err := NewMessage(TypePing, "sender", payload)
	if err != nil {
		t.Fatalf("创建消息失败: %v", err)
	}

	var decoded PingPayload
	if err := msg.GetPayload(&decoded); err != nil {
		t.Fatalf("解析 payload 失败: %v", err)
	}

	if decoded.Nonce != 12345 {
		t.Errorf("Nonce 错误: %d", decoded.Nonce)
	}
}

func TestPongPayload(t *testing.T) {
	payload := PongPayload{Nonce: 67890}

	msg, _ := NewMessage(TypePong, "sender", payload)

	var decoded PongPayload
	msg.GetPayload(&decoded)

	if decoded.Nonce != 67890 {
		t.Errorf("Nonce 错误: %d", decoded.Nonce)
	}
}

func TestFindNodePayload(t *testing.T) {
	payload := FindNodePayload{TargetID: "target-node-id"}

	msg, _ := NewMessage(TypeFindNode, "sender", payload)

	var decoded FindNodePayload
	msg.GetPayload(&decoded)

	if decoded.TargetID != "target-node-id" {
		t.Errorf("TargetID 错误: %s", decoded.TargetID)
	}
}

func TestFoundNodePayload(t *testing.T) {
	payload := FoundNodePayload{
		Nodes: []NodeInfo{
			{ID: "node-1", Address: "192.168.1.1", Port: 8080},
			{ID: "node-2", Address: "192.168.1.2", Port: 8081},
		},
	}

	msg, _ := NewMessage(TypeFoundNode, "sender", payload)

	var decoded FoundNodePayload
	msg.GetPayload(&decoded)

	if len(decoded.Nodes) != 2 {
		t.Errorf("节点数量错误: %d", len(decoded.Nodes))
	}

	if decoded.Nodes[0].ID != "node-1" {
		t.Errorf("第一个节点 ID 错误: %s", decoded.Nodes[0].ID)
	}
}

func TestProposalPayload(t *testing.T) {
	payload := ProposalPayload{
		ProposalID:  "RFC-001",
		Title:       "改进心跳协议",
		Description: "详细描述...",
		Author:      "agent-123",
		Category:    "RFC",
	}

	msg, _ := NewMessage(TypeProposal, "sender", payload)

	var decoded ProposalPayload
	msg.GetPayload(&decoded)

	if decoded.ProposalID != "RFC-001" {
		t.Errorf("ProposalID 错误: %s", decoded.ProposalID)
	}

	if decoded.Category != "RFC" {
		t.Errorf("Category 错误: %s", decoded.Category)
	}
}

func TestVotePayload(t *testing.T) {
	payload := VotePayload{
		ProposalID: "RFC-001",
		Vote:       "approve",
		Reason:     "好提案",
	}

	msg, _ := NewMessage(TypeVote, "sender", payload)

	var decoded VotePayload
	msg.GetPayload(&decoded)

	if decoded.Vote != "approve" {
		t.Errorf("Vote 错误: %s", decoded.Vote)
	}

	if decoded.Reason != "好提案" {
		t.Errorf("Reason 错误: %s", decoded.Reason)
	}
}

func TestGenerateID(t *testing.T) {
	id1 := generateID()
	time.Sleep(time.Nanosecond)
	id2 := generateID()

	if id1 == "" {
		t.Error("ID 为空")
	}

	if id1 == id2 {
		t.Error("ID 应该是唯一的")
	}
}

func TestMessage_WithTo(t *testing.T) {
	msg, _ := NewMessage(TypePing, "sender", nil)
	msg.To = "receiver"

	if msg.To != "receiver" {
		t.Error("To 字段设置失败")
	}

	data, _ := msg.Marshal()
	var parsed map[string]interface{}
	json.Unmarshal(data, &parsed)

	if parsed["to"] != "receiver" {
		t.Error("To 字段未正确序列化")
	}
}
