package crypto

import (
	"encoding/json"
	"testing"
	"time"
)

func TestNewMessageSigner(t *testing.T) {
	signer, err := NewMessageSigner()
	if err != nil {
		t.Fatalf("创建签名器失败: %v", err)
	}

	if signer.NodeID() == "" {
		t.Error("NodeID 不应为空")
	}

	if signer.PublicKeyHex() == "" {
		t.Error("PublicKeyHex 不应为空")
	}

	t.Logf("NodeID: %s", signer.NodeID())
	t.Logf("PublicKey: %s", signer.PublicKeyHex())
}

func TestNewMessageSignerFromKey(t *testing.T) {
	// 先创建一个签名器获取私钥
	signer1, err := NewMessageSigner()
	if err != nil {
		t.Fatalf("创建签名器失败: %v", err)
	}

	privKeyHex := signer1.PrivateKeyHex()

	// 从私钥重建签名器
	signer2, err := NewMessageSignerFromKey(privKeyHex)
	if err != nil {
		t.Fatalf("从私钥创建签名器失败: %v", err)
	}

	// 验证两个签名器的NodeID相同
	if signer1.NodeID() != signer2.NodeID() {
		t.Errorf("NodeID 不匹配: %s != %s", signer1.NodeID(), signer2.NodeID())
	}

	if signer1.PublicKeyHex() != signer2.PublicKeyHex() {
		t.Errorf("PublicKey 不匹配")
	}
}

func TestSignMessage(t *testing.T) {
	signer, err := NewMessageSigner()
	if err != nil {
		t.Fatalf("创建签名器失败: %v", err)
	}

	content := map[string]interface{}{
		"task_id":     "task-001",
		"description": "测试任务",
		"difficulty":  5,
	}

	msg, err := signer.SignMessage(MsgTypeTaskSubmit, content)
	if err != nil {
		t.Fatalf("签名消息失败: %v", err)
	}

	// 验证消息结构
	if msg.MessageID == "" {
		t.Error("MessageID 不应为空")
	}
	if msg.MessageType != MsgTypeTaskSubmit {
		t.Errorf("MessageType 不匹配: %s", msg.MessageType)
	}
	if msg.Sender != signer.NodeID() {
		t.Errorf("Sender 不匹配: %s != %s", msg.Sender, signer.NodeID())
	}
	if msg.SenderKey != signer.PublicKeyHex() {
		t.Error("SenderKey 不匹配")
	}
	if msg.Signature == "" {
		t.Error("Signature 不应为空")
	}
	if msg.Timestamp == 0 {
		t.Error("Timestamp 不应为0")
	}

	t.Logf("MessageID: %s", msg.MessageID)
	t.Logf("Signature长度: %d bytes", len(msg.Signature)/2)
}

func TestSignRawMessage(t *testing.T) {
	signer, err := NewMessageSigner()
	if err != nil {
		t.Fatalf("创建签名器失败: %v", err)
	}

	content := []byte(`{"raw": "data", "value": 123}`)

	msg, err := signer.SignRawMessage(MsgTypeLogSubmit, content)
	if err != nil {
		t.Fatalf("签名原始消息失败: %v", err)
	}

	if msg.MessageID == "" {
		t.Error("MessageID 不应为空")
	}
	if string(msg.Content) != string(content) {
		t.Error("Content 不匹配")
	}
}

func TestVerifyMessage(t *testing.T) {
	signer, err := NewMessageSigner()
	if err != nil {
		t.Fatalf("创建签名器失败: %v", err)
	}

	content := map[string]string{
		"action": "test",
		"data":   "hello",
	}

	msg, err := signer.SignMessage(MsgTypeHeartbeat, content)
	if err != nil {
		t.Fatalf("签名消息失败: %v", err)
	}

	verifier := NewMessageVerifier(10 * time.Minute)
	result := verifier.VerifyMessage(msg)

	if !result.Valid {
		t.Errorf("验证失败: %s", result.Error)
	}
	if result.Sender != signer.NodeID() {
		t.Errorf("Sender 不匹配: %s", result.Sender)
	}

	t.Logf("验证结果: Valid=%v, Sender=%s", result.Valid, result.Sender)
}

func TestVerifyMessageStatic(t *testing.T) {
	signer, err := NewMessageSigner()
	if err != nil {
		t.Fatalf("创建签名器失败: %v", err)
	}

	msg, err := signer.SignMessage(MsgTypeTaskResult, map[string]int{"result": 42})
	if err != nil {
		t.Fatalf("签名消息失败: %v", err)
	}

	result := VerifyMessageStatic(msg)
	if !result.Valid {
		t.Errorf("静态验证失败: %s", result.Error)
	}
}

func TestVerifyMessage_Expired(t *testing.T) {
	signer, err := NewMessageSigner()
	if err != nil {
		t.Fatalf("创建签名器失败: %v", err)
	}

	msg, err := signer.SignMessage(MsgTypeHeartbeat, "test")
	if err != nil {
		t.Fatalf("签名消息失败: %v", err)
	}

	// 修改时间戳为过去
	msg.Timestamp = time.Now().Add(-20 * time.Minute).UnixMilli()

	verifier := NewMessageVerifier(10 * time.Minute)
	result := verifier.VerifyMessage(msg)

	if result.Valid {
		t.Error("过期消息不应该验证通过")
	}
	t.Logf("过期消息验证结果: %s", result.Error)
}

func TestVerifyMessage_Replay(t *testing.T) {
	signer, err := NewMessageSigner()
	if err != nil {
		t.Fatalf("创建签名器失败: %v", err)
	}

	msg, err := signer.SignMessage(MsgTypeTaskSubmit, "test")
	if err != nil {
		t.Fatalf("签名消息失败: %v", err)
	}

	verifier := NewMessageVerifier(10 * time.Minute)

	// 第一次验证
	result1 := verifier.VerifyMessage(msg)
	if !result1.Valid {
		t.Errorf("第一次验证失败: %s", result1.Error)
	}

	// 重放攻击 - 第二次验证同一消息
	result2 := verifier.VerifyMessage(msg)
	if result2.Valid {
		t.Error("重放消息不应该验证通过")
	}
	t.Logf("重放检测结果: %s", result2.Error)
}

func TestVerifyMessage_TamperedContent(t *testing.T) {
	signer, err := NewMessageSigner()
	if err != nil {
		t.Fatalf("创建签名器失败: %v", err)
	}

	msg, err := signer.SignMessage(MsgTypeTaskSubmit, map[string]int{"amount": 100})
	if err != nil {
		t.Fatalf("签名消息失败: %v", err)
	}

	// 篡改内容
	msg.Content = json.RawMessage(`{"amount": 999999}`)

	verifier := NewMessageVerifier(10 * time.Minute)
	result := verifier.VerifyMessage(msg)

	if result.Valid {
		t.Error("篡改后的消息不应该验证通过")
	}
	t.Logf("篡改检测结果: %s", result.Error)
}

func TestVerifyMessage_InvalidSender(t *testing.T) {
	signer, err := NewMessageSigner()
	if err != nil {
		t.Fatalf("创建签名器失败: %v", err)
	}

	msg, err := signer.SignMessage(MsgTypeHeartbeat, "test")
	if err != nil {
		t.Fatalf("签名消息失败: %v", err)
	}

	// 伪造发送者ID
	msg.Sender = "fake_sender_id_12345"

	verifier := NewMessageVerifier(10 * time.Minute)
	result := verifier.VerifyMessage(msg)

	if result.Valid {
		t.Error("伪造发送者的消息不应该验证通过")
	}
	t.Logf("发送者验证结果: %s", result.Error)
}

func TestCleanupExpired(t *testing.T) {
	verifier := NewMessageVerifier(100 * time.Millisecond)

	signer, _ := NewMessageSigner()

	// 创建几条消息
	for i := 0; i < 5; i++ {
		msg, _ := signer.SignMessage(MsgTypeHeartbeat, i)
		verifier.VerifyMessage(msg)
	}

	if verifier.SeenCount() != 5 {
		t.Errorf("应该有5条记录，实际: %d", verifier.SeenCount())
	}

	// 等待过期
	time.Sleep(150 * time.Millisecond)

	cleaned := verifier.CleanupExpired()
	if cleaned != 5 {
		t.Errorf("应该清理5条记录，实际: %d", cleaned)
	}

	if verifier.SeenCount() != 0 {
		t.Errorf("清理后应该为0，实际: %d", verifier.SeenCount())
	}
}

func TestBatchVerify(t *testing.T) {
	signer, err := NewMessageSigner()
	if err != nil {
		t.Fatalf("创建签名器失败: %v", err)
	}

	// 创建10条消息
	messages := make([]*SignedMessage, 10)
	for i := 0; i < 10; i++ {
		msg, err := signer.SignMessage(MsgTypeTaskSubmit, map[string]int{"index": i})
		if err != nil {
			t.Fatalf("签名消息失败: %v", err)
		}
		messages[i] = msg
	}

	verifier := NewMessageVerifier(10 * time.Minute)
	result := verifier.BatchVerify(messages)

	if result.Total != 10 {
		t.Errorf("Total 不匹配: %d", result.Total)
	}
	if result.Valid != 10 {
		t.Errorf("Valid 不匹配: %d", result.Valid)
	}
	if result.Invalid != 0 {
		t.Errorf("Invalid 不匹配: %d", result.Invalid)
	}

	t.Logf("批量验证结果: Total=%d, Valid=%d, Duration=%v", result.Total, result.Valid, result.Duration)
}

func TestBatchVerifyParallel(t *testing.T) {
	signer, err := NewMessageSigner()
	if err != nil {
		t.Fatalf("创建签名器失败: %v", err)
	}

	// 创建100条消息
	messages := make([]*SignedMessage, 100)
	for i := 0; i < 100; i++ {
		msg, err := signer.SignMessage(MsgTypeTaskResult, map[string]int{"value": i * 10})
		if err != nil {
			t.Fatalf("签名消息失败: %v", err)
		}
		messages[i] = msg
	}

	verifier := NewMessageVerifier(10 * time.Minute)
	result := verifier.BatchVerifyParallel(messages, 4)

	if result.Total != 100 {
		t.Errorf("Total 不匹配: %d", result.Total)
	}
	if result.Valid != 100 {
		t.Errorf("Valid 不匹配: %d", result.Valid)
	}

	t.Logf("并行批量验证: Total=%d, Valid=%d, Duration=%v", result.Total, result.Valid, result.Duration)
}

func TestMessageMarshalUnmarshal(t *testing.T) {
	signer, err := NewMessageSigner()
	if err != nil {
		t.Fatalf("创建签名器失败: %v", err)
	}

	original, err := signer.SignMessage(MsgTypeAccuse, map[string]string{
		"target":  "bad_node",
		"reason":  "作弊",
		"evidence": "proof_hash",
	})
	if err != nil {
		t.Fatalf("签名消息失败: %v", err)
	}

	// 序列化
	data, err := original.Marshal()
	if err != nil {
		t.Fatalf("序列化失败: %v", err)
	}

	// 反序列化
	restored, err := UnmarshalSignedMessage(data)
	if err != nil {
		t.Fatalf("反序列化失败: %v", err)
	}

	// 验证字段
	if original.MessageID != restored.MessageID {
		t.Error("MessageID 不匹配")
	}
	if original.MessageType != restored.MessageType {
		t.Error("MessageType 不匹配")
	}
	if original.Sender != restored.Sender {
		t.Error("Sender 不匹配")
	}
	if original.Signature != restored.Signature {
		t.Error("Signature 不匹配")
	}

	// 验证反序列化后的消息签名
	result := VerifyMessageStatic(restored)
	if !result.Valid {
		t.Errorf("反序列化后验证失败: %s", result.Error)
	}
}

func TestGetContent(t *testing.T) {
	signer, err := NewMessageSigner()
	if err != nil {
		t.Fatalf("创建签名器失败: %v", err)
	}

	type TaskPayload struct {
		TaskID      string `json:"task_id"`
		Description string `json:"description"`
		Priority    int    `json:"priority"`
	}

	original := TaskPayload{
		TaskID:      "task-123",
		Description: "处理数据",
		Priority:    1,
	}

	msg, err := signer.SignMessage(MsgTypeTaskAssign, original)
	if err != nil {
		t.Fatalf("签名消息失败: %v", err)
	}

	// 解析内容
	var parsed TaskPayload
	if err := msg.GetContent(&parsed); err != nil {
		t.Fatalf("解析内容失败: %v", err)
	}

	if parsed.TaskID != original.TaskID {
		t.Errorf("TaskID 不匹配: %s", parsed.TaskID)
	}
	if parsed.Description != original.Description {
		t.Errorf("Description 不匹配: %s", parsed.Description)
	}
	if parsed.Priority != original.Priority {
		t.Errorf("Priority 不匹配: %d", parsed.Priority)
	}
}

func TestVerificationLogger(t *testing.T) {
	logger := NewVerificationLogger(100)
	signer, _ := NewMessageSigner()
	verifier := NewMessageVerifier(10 * time.Minute)

	// 创建并验证一些消息
	for i := 0; i < 5; i++ {
		msg, _ := signer.SignMessage(MsgTypeHeartbeat, i)
		result := verifier.VerifyMessage(msg)
		logger.Log(msg, result)
	}

	// 创建一个无效消息
	invalidMsg, _ := signer.SignMessage(MsgTypeTaskSubmit, "test")
	invalidMsg.Content = json.RawMessage(`"tampered"`)
	invalidResult := verifier.VerifyMessage(invalidMsg)
	logger.Log(invalidMsg, invalidResult)

	// 检查统计
	stats := logger.GetStats()
	if stats.TotalVerified != 6 {
		t.Errorf("TotalVerified 不匹配: %d", stats.TotalVerified)
	}
	if stats.ValidCount != 5 {
		t.Errorf("ValidCount 不匹配: %d", stats.ValidCount)
	}
	if stats.InvalidCount != 1 {
		t.Errorf("InvalidCount 不匹配: %d", stats.InvalidCount)
	}

	// 检查失败日志
	failed := logger.GetFailedLogs(10)
	if len(failed) != 1 {
		t.Errorf("失败日志数量不匹配: %d", len(failed))
	}

	// 检查发送者日志
	senderLogs := logger.GetLogsBySender(signer.NodeID(), 10)
	if len(senderLogs) != 6 {
		t.Errorf("发送者日志数量不匹配: %d", len(senderLogs))
	}

	t.Logf("统计: Total=%d, Valid=%d, Invalid=%d, Rate=%.2f",
		stats.TotalVerified, stats.ValidCount, stats.InvalidCount, stats.ValidRate)
}

func TestVerificationLoggerMaxLogs(t *testing.T) {
	logger := NewVerificationLogger(10)
	signer, _ := NewMessageSigner()
	verifier := NewMessageVerifier(10 * time.Minute)

	// 创建20条消息
	for i := 0; i < 20; i++ {
		msg, _ := signer.SignMessage(MsgTypeHeartbeat, i)
		result := verifier.VerifyMessage(msg)
		logger.Log(msg, result)
	}

	logs := logger.GetLogs(100)
	if len(logs) != 10 {
		t.Errorf("日志数量应该被限制为10，实际: %d", len(logs))
	}
}

func TestAllMessageTypes(t *testing.T) {
	signer, err := NewMessageSigner()
	if err != nil {
		t.Fatalf("创建签名器失败: %v", err)
	}

	verifier := NewMessageVerifier(10 * time.Minute)

	types := []MessageType{
		MsgTypeTaskSubmit,
		MsgTypeTaskAssign,
		MsgTypeTaskResult,
		MsgTypeReputationChange,
		MsgTypeReputationPropagate,
		MsgTypeAccuse,
		MsgTypeAccusePropagate,
		MsgTypeMailSend,
		MsgTypeMailFetch,
		MsgTypeLogSubmit,
		MsgTypeNodeJoin,
		MsgTypeNodeVote,
		MsgTypeHeartbeat,
	}

	for _, msgType := range types {
		msg, err := signer.SignMessage(msgType, map[string]string{"type": string(msgType)})
		if err != nil {
			t.Errorf("签名 %s 消息失败: %v", msgType, err)
			continue
		}

		result := verifier.VerifyMessage(msg)
		if !result.Valid {
			t.Errorf("验证 %s 消息失败: %s", msgType, result.Error)
		}
	}

	t.Logf("测试了 %d 种消息类型", len(types))
}

func TestComputeMessageID(t *testing.T) {
	sender := "test_sender_123"
	timestamp := int64(1700000000000)
	content := []byte(`{"key": "value"}`)

	id1 := ComputeMessageID(sender, timestamp, content)
	id2 := ComputeMessageID(sender, timestamp, content)

	if id1 != id2 {
		t.Error("相同输入应该产生相同的消息ID")
	}

	// 不同输入应该产生不同的ID
	id3 := ComputeMessageID(sender, timestamp+1, content)
	if id1 == id3 {
		t.Error("不同输入应该产生不同的消息ID")
	}

	t.Logf("消息ID: %s", id1)
}

func BenchmarkSignMessage(b *testing.B) {
	signer, _ := NewMessageSigner()
	content := map[string]interface{}{
		"task_id": "benchmark-task",
		"data":    "benchmark data",
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, _ = signer.SignMessage(MsgTypeTaskSubmit, content)
	}
}

func BenchmarkVerifyMessage(b *testing.B) {
	signer, _ := NewMessageSigner()
	msg, _ := signer.SignMessage(MsgTypeTaskSubmit, map[string]string{"data": "benchmark"})

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = VerifyMessageStatic(msg)
	}
}

func BenchmarkBatchVerifyParallel(b *testing.B) {
	signer, _ := NewMessageSigner()
	messages := make([]*SignedMessage, 100)
	for i := 0; i < 100; i++ {
		msg, _ := signer.SignMessage(MsgTypeTaskSubmit, i)
		messages[i] = msg
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		verifier := NewMessageVerifier(10 * time.Minute)
		_ = verifier.BatchVerifyParallel(messages, 4)
	}
}
