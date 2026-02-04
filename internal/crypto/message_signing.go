package crypto

import (
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"math/big"
	"sync"
	"time"

	"github.com/tjfoc/gmsm/sm2"
	"github.com/tjfoc/gmsm/sm3"
)

// MessageType 消息类型
type MessageType string

const (
	// 任务相关
	MsgTypeTaskSubmit   MessageType = "TaskSubmit"
	MsgTypeTaskAssign   MessageType = "TaskAssign"
	MsgTypeTaskResult   MessageType = "TaskResult"

	// 声誉相关
	MsgTypeReputationChange    MessageType = "ReputationChange"
	MsgTypeReputationPropagate MessageType = "ReputationPropagate"

	// 指责相关
	MsgTypeAccuse          MessageType = "Accuse"
	MsgTypeAccusePropagate MessageType = "AccusePropagate"

	// 邮箱相关
	MsgTypeMailSend  MessageType = "MailSend"
	MsgTypeMailFetch MessageType = "MailFetch"

	// 日志相关
	MsgTypeLogSubmit MessageType = "LogSubmit"

	// 网络管理
	MsgTypeNodeJoin MessageType = "NodeJoin"
	MsgTypeNodeVote MessageType = "NodeVote"

	// 心跳
	MsgTypeHeartbeat MessageType = "Heartbeat"
)

// SignedMessage 签名消息结构
type SignedMessage struct {
	// 消息头
	MessageID   string      `json:"message_id"`   // 消息唯一ID
	MessageType MessageType `json:"message_type"` // 消息类型
	Sender      string      `json:"sender"`       // 发送者节点ID
	SenderKey   string      `json:"sender_key"`   // 发送者公钥(hex)
	Timestamp   int64       `json:"timestamp"`    // 时间戳(毫秒)
	Nonce       string      `json:"nonce"`        // 随机数防重放

	// 消息体
	Content json.RawMessage `json:"content"` // 消息内容

	// 签名
	Signature string `json:"signature"` // SM2签名(hex)
}

// NonceLength Nonce 长度（16字节 = 128位）
const NonceLength = 16

// generateNonce 生成随机 Nonce
func generateNonce() (string, error) {
	bytes := make([]byte, NonceLength)
	if _, err := rand.Read(bytes); err != nil {
		return "", fmt.Errorf("生成 Nonce 失败: %w", err)
	}
	return hex.EncodeToString(bytes), nil
}

// MessageSigner 消息签名器
type MessageSigner struct {
	privateKey *sm2.PrivateKey
	publicKey  *sm2.PublicKey
	nodeID     string
	mu         sync.RWMutex
}

// NewMessageSigner 创建消息签名器
func NewMessageSigner() (*MessageSigner, error) {
	priv, err := sm2.GenerateKey(rand.Reader)
	if err != nil {
		return nil, fmt.Errorf("生成 SM2 密钥失败: %w", err)
	}

	pubBytes := sm2.Compress(&priv.PublicKey)
	hash := sm3.Sm3Sum(pubBytes)
	nodeID := hex.EncodeToString(hash[:16])

	return &MessageSigner{
		privateKey: priv,
		publicKey:  &priv.PublicKey,
		nodeID:     nodeID,
	}, nil
}

// NewMessageSignerFromKey 从私钥字节创建签名器
func NewMessageSignerFromKey(privKeyHex string) (*MessageSigner, error) {
	privBytes, err := hex.DecodeString(privKeyHex)
	if err != nil {
		return nil, fmt.Errorf("解析私钥失败: %w", err)
	}

	priv := new(sm2.PrivateKey)
	priv.Curve = sm2.P256Sm2()
	priv.D = new(big.Int).SetBytes(privBytes)
	priv.PublicKey.X, priv.PublicKey.Y = priv.Curve.ScalarBaseMult(privBytes)

	pubBytes := sm2.Compress(&priv.PublicKey)
	hash := sm3.Sm3Sum(pubBytes)
	nodeID := hex.EncodeToString(hash[:16])

	return &MessageSigner{
		privateKey: priv,
		publicKey:  &priv.PublicKey,
		nodeID:     nodeID,
	}, nil
}

// NodeID 返回节点ID
func (ms *MessageSigner) NodeID() string {
	ms.mu.RLock()
	defer ms.mu.RUnlock()
	return ms.nodeID
}

// PublicKeyHex 返回公钥的hex编码
func (ms *MessageSigner) PublicKeyHex() string {
	ms.mu.RLock()
	defer ms.mu.RUnlock()
	return hex.EncodeToString(sm2.Compress(ms.publicKey))
}

// PrivateKeyHex 返回私钥的hex编码
func (ms *MessageSigner) PrivateKeyHex() string {
	ms.mu.RLock()
	defer ms.mu.RUnlock()
	return hex.EncodeToString(ms.privateKey.D.Bytes())
}

// SignMessage 对消息进行签名
func (ms *MessageSigner) SignMessage(msgType MessageType, content interface{}) (*SignedMessage, error) {
	ms.mu.RLock()
	defer ms.mu.RUnlock()

	// 序列化内容
	contentBytes, err := json.Marshal(content)
	if err != nil {
		return nil, fmt.Errorf("序列化消息内容失败: %w", err)
	}

	timestamp := time.Now().UnixMilli()
	senderKey := hex.EncodeToString(sm2.Compress(ms.publicKey))

	// 生成 Nonce 防重放
	nonce, err := generateNonce()
	if err != nil {
		return nil, err
	}

	// 生成消息ID = SM3(Sender + Timestamp + Nonce + Content)
	idData := fmt.Sprintf("%s%d%s%s", ms.nodeID, timestamp, nonce, string(contentBytes))
	idHash := sm3.Sm3Sum([]byte(idData))
	messageID := hex.EncodeToString(idHash[:16])

	// 计算摘要 = SM3(Content + MessageType + Timestamp + Nonce + Sender)
	digest := computeDigestWithNonce(contentBytes, msgType, timestamp, nonce, ms.nodeID)

	// SM2 签名
	sig, err := ms.privateKey.Sign(rand.Reader, digest, nil)
	if err != nil {
		return nil, fmt.Errorf("签名失败: %w", err)
	}

	return &SignedMessage{
		MessageID:   messageID,
		MessageType: msgType,
		Sender:      ms.nodeID,
		SenderKey:   senderKey,
		Nonce:       nonce,
		Timestamp:   timestamp,
		Content:     contentBytes,
		Signature:   hex.EncodeToString(sig),
	}, nil
}

// SignRawMessage 对原始字节消息签名
func (ms *MessageSigner) SignRawMessage(msgType MessageType, content []byte) (*SignedMessage, error) {
	ms.mu.RLock()
	defer ms.mu.RUnlock()

	timestamp := time.Now().UnixMilli()
	senderKey := hex.EncodeToString(sm2.Compress(ms.publicKey))

	// 生成 Nonce 防重放
	nonce, err := generateNonce()
	if err != nil {
		return nil, err
	}

	// 生成消息ID
	idData := fmt.Sprintf("%s%d%s%s", ms.nodeID, timestamp, nonce, string(content))
	idHash := sm3.Sm3Sum([]byte(idData))
	messageID := hex.EncodeToString(idHash[:16])

	// 计算摘要（带 Nonce）
	digest := computeDigestWithNonce(content, msgType, timestamp, nonce, ms.nodeID)

	// SM2 签名
	sig, err := ms.privateKey.Sign(rand.Reader, digest, nil)
	if err != nil {
		return nil, fmt.Errorf("签名失败: %w", err)
	}

	return &SignedMessage{
		MessageID:   messageID,
		MessageType: msgType,
		Sender:      ms.nodeID,
		SenderKey:   senderKey,
		Nonce:       nonce,
		Timestamp:   timestamp,
		Content:     content,
		Signature:   hex.EncodeToString(sig),
	}, nil
}

// computeDigest 计算消息摘要 (兼容旧消息)
// Digest = SM3(Content + MessageType + Timestamp + Sender)
func computeDigest(content []byte, msgType MessageType, timestamp int64, sender string) []byte {
	data := fmt.Sprintf("%s%s%d%s", string(content), msgType, timestamp, sender)
	hash := sm3.Sm3Sum([]byte(data))
	return hash[:]
}

// computeDigestWithNonce 计算带 Nonce 的消息摘要
// Digest = SM3(Content + MessageType + Timestamp + Nonce + Sender)
func computeDigestWithNonce(content []byte, msgType MessageType, timestamp int64, nonce, sender string) []byte {
	data := fmt.Sprintf("%s%s%d%s%s", string(content), msgType, timestamp, nonce, sender)
	hash := sm3.Sm3Sum([]byte(data))
	return hash[:]
}

// ========== 消息验证器 ==========

// MessageVerifier 消息验证器
type MessageVerifier struct {
	// 防重放：消息ID缓存
	seenMessages map[string]int64 // messageID -> timestamp
	maxAge       time.Duration    // 消息最大有效期
	mu           sync.RWMutex
}

// NewMessageVerifier 创建消息验证器
func NewMessageVerifier(maxAge time.Duration) *MessageVerifier {
	if maxAge <= 0 {
		maxAge = 10 * time.Minute // 默认10分钟
	}
	return &MessageVerifier{
		seenMessages: make(map[string]int64),
		maxAge:       maxAge,
	}
}

// VerifyResult 验证结果
type VerifyResult struct {
	Valid       bool   `json:"valid"`
	Error       string `json:"error,omitempty"`
	Sender      string `json:"sender"`
	MessageType string `json:"message_type"`
}

// VerifyMessage 验证签名消息
func (mv *MessageVerifier) VerifyMessage(msg *SignedMessage) *VerifyResult {
	result := &VerifyResult{
		Sender:      msg.Sender,
		MessageType: string(msg.MessageType),
	}

	// 1. 检查时间戳（防重放）
	now := time.Now().UnixMilli()
	age := time.Duration(now-msg.Timestamp) * time.Millisecond
	if age > mv.maxAge {
		result.Error = fmt.Sprintf("消息已过期: %v", age)
		return result
	}
	if msg.Timestamp > now+60000 { // 允许1分钟时钟偏差
		result.Error = "消息时间戳在未来"
		return result
	}

	// 2. 检查 Nonce 是否存在（新消息必须有 Nonce）
	hasNonce := msg.Nonce != ""

	// 3. 检查重复消息（使用 MessageID 或 Nonce）
	mv.mu.Lock()
	if _, seen := mv.seenMessages[msg.MessageID]; seen {
		mv.mu.Unlock()
		result.Error = "重复消息 (MessageID 已存在)"
		return result
	}
	// 如果有 Nonce，也检查 Nonce 是否重复
	if hasNonce {
		nonceKey := msg.Sender + ":" + msg.Nonce
		if _, seen := mv.seenMessages[nonceKey]; seen {
			mv.mu.Unlock()
			result.Error = "重复消息 (Nonce 已使用)"
			return result
		}
		mv.seenMessages[nonceKey] = msg.Timestamp
	}
	mv.seenMessages[msg.MessageID] = msg.Timestamp
	mv.mu.Unlock()

	// 4. 验证发送者公钥与NodeID匹配
	pubKeyBytes, err := hex.DecodeString(msg.SenderKey)
	if err != nil {
		result.Error = "无效的发送者公钥"
		return result
	}
	pubKey := sm2.Decompress(pubKeyBytes)
	if pubKey == nil {
		result.Error = "解压公钥失败"
		return result
	}

	// 验证NodeID
	hash := sm3.Sm3Sum(pubKeyBytes)
	expectedNodeID := hex.EncodeToString(hash[:16])
	if msg.Sender != expectedNodeID {
		result.Error = "发送者ID与公钥不匹配"
		return result
	}

	// 5. 计算摘要并验证签名（根据是否有 Nonce 选择不同的摘要算法）
	var digest []byte
	if hasNonce {
		digest = computeDigestWithNonce(msg.Content, msg.MessageType, msg.Timestamp, msg.Nonce, msg.Sender)
	} else {
		// 兼容旧消息（无 Nonce）
		digest = computeDigest(msg.Content, msg.MessageType, msg.Timestamp, msg.Sender)
	}

	sigBytes, err := hex.DecodeString(msg.Signature)
	if err != nil {
		result.Error = "无效的签名格式"
		return result
	}

	if !pubKey.Verify(digest, sigBytes) {
		result.Error = "签名验证失败"
		return result
	}

	result.Valid = true
	return result
}

// VerifyMessageStatic 静态验证消息（不检查重放）
func VerifyMessageStatic(msg *SignedMessage) *VerifyResult {
	result := &VerifyResult{
		Sender:      msg.Sender,
		MessageType: string(msg.MessageType),
	}

	// 1. 验证发送者公钥与NodeID匹配
	pubKeyBytes, err := hex.DecodeString(msg.SenderKey)
	if err != nil {
		result.Error = "无效的发送者公钥"
		return result
	}
	pubKey := sm2.Decompress(pubKeyBytes)
	if pubKey == nil {
		result.Error = "解压公钥失败"
		return result
	}

	// 验证NodeID
	hash := sm3.Sm3Sum(pubKeyBytes)
	expectedNodeID := hex.EncodeToString(hash[:16])
	if msg.Sender != expectedNodeID {
		result.Error = "发送者ID与公钥不匹配"
		return result
	}

	// 2. 计算摘要并验证签名（根据是否有 Nonce 选择不同的摘要算法）
	var digest []byte
	if msg.Nonce != "" {
		digest = computeDigestWithNonce(msg.Content, msg.MessageType, msg.Timestamp, msg.Nonce, msg.Sender)
	} else {
		// 兼容旧消息（无 Nonce）
		digest = computeDigest(msg.Content, msg.MessageType, msg.Timestamp, msg.Sender)
	}

	sigBytes, err := hex.DecodeString(msg.Signature)
	if err != nil {
		result.Error = "无效的签名格式"
		return result
	}

	if !pubKey.Verify(digest, sigBytes) {
		result.Error = "签名验证失败"
		return result
	}

	result.Valid = true
	return result
}

// CleanupExpired 清理过期的消息记录
func (mv *MessageVerifier) CleanupExpired() int {
	mv.mu.Lock()
	defer mv.mu.Unlock()

	now := time.Now().UnixMilli()
	maxAgeMs := mv.maxAge.Milliseconds()
	cleaned := 0

	for id, ts := range mv.seenMessages {
		if now-ts > maxAgeMs {
			delete(mv.seenMessages, id)
			cleaned++
		}
	}

	return cleaned
}

// SeenCount 返回已见消息数量
func (mv *MessageVerifier) SeenCount() int {
	mv.mu.RLock()
	defer mv.mu.RUnlock()
	return len(mv.seenMessages)
}

// ========== 批量验证 ==========

// BatchVerifyResult 批量验证结果
type BatchVerifyResult struct {
	Total    int             `json:"total"`
	Valid    int             `json:"valid"`
	Invalid  int             `json:"invalid"`
	Results  []*VerifyResult `json:"results"`
	Duration time.Duration   `json:"duration"`
}

// BatchVerify 批量验证消息
func (mv *MessageVerifier) BatchVerify(messages []*SignedMessage) *BatchVerifyResult {
	start := time.Now()
	result := &BatchVerifyResult{
		Total:   len(messages),
		Results: make([]*VerifyResult, len(messages)),
	}

	for i, msg := range messages {
		vr := mv.VerifyMessage(msg)
		result.Results[i] = vr
		if vr.Valid {
			result.Valid++
		} else {
			result.Invalid++
		}
	}

	result.Duration = time.Since(start)
	return result
}

// BatchVerifyParallel 并行批量验证消息
func (mv *MessageVerifier) BatchVerifyParallel(messages []*SignedMessage, workers int) *BatchVerifyResult {
	start := time.Now()
	result := &BatchVerifyResult{
		Total:   len(messages),
		Results: make([]*VerifyResult, len(messages)),
	}

	if workers <= 0 {
		workers = 4
	}

	// 使用 channel 进行并行处理
	type job struct {
		index int
		msg   *SignedMessage
	}
	jobs := make(chan job, len(messages))
	results := make(chan struct {
		index  int
		result *VerifyResult
	}, len(messages))

	// 启动 worker
	var wg sync.WaitGroup
	for w := 0; w < workers; w++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for j := range jobs {
				vr := mv.VerifyMessage(j.msg)
				results <- struct {
					index  int
					result *VerifyResult
				}{j.index, vr}
			}
		}()
	}

	// 发送任务
	go func() {
		for i, msg := range messages {
			jobs <- job{i, msg}
		}
		close(jobs)
	}()

	// 等待完成并关闭结果 channel
	go func() {
		wg.Wait()
		close(results)
	}()

	// 收集结果
	for r := range results {
		result.Results[r.index] = r.result
		if r.result.Valid {
			result.Valid++
		} else {
			result.Invalid++
		}
	}

	result.Duration = time.Since(start)
	return result
}

// ========== 消息序列化 ==========

// Marshal 序列化签名消息
func (msg *SignedMessage) Marshal() ([]byte, error) {
	return json.Marshal(msg)
}

// UnmarshalSignedMessage 反序列化签名消息
func UnmarshalSignedMessage(data []byte) (*SignedMessage, error) {
	var msg SignedMessage
	if err := json.Unmarshal(data, &msg); err != nil {
		return nil, fmt.Errorf("反序列化消息失败: %w", err)
	}
	return &msg, nil
}

// GetContent 解析消息内容
func (msg *SignedMessage) GetContent(v interface{}) error {
	return json.Unmarshal(msg.Content, v)
}

// ========== 验证日志 ==========

// VerificationLog 验证日志记录
type VerificationLog struct {
	MessageID   string      `json:"message_id"`
	MessageType MessageType `json:"message_type"`
	Sender      string      `json:"sender"`
	Timestamp   time.Time   `json:"timestamp"`
	Valid       bool        `json:"valid"`
	Error       string      `json:"error,omitempty"`
	VerifiedAt  time.Time   `json:"verified_at"`
}

// VerificationLogger 验证日志器
type VerificationLogger struct {
	logs     []*VerificationLog
	maxLogs  int
	mu       sync.RWMutex
}

// NewVerificationLogger 创建验证日志器
func NewVerificationLogger(maxLogs int) *VerificationLogger {
	if maxLogs <= 0 {
		maxLogs = 10000
	}
	return &VerificationLogger{
		logs:    make([]*VerificationLog, 0),
		maxLogs: maxLogs,
	}
}

// Log 记录验证结果
func (vl *VerificationLogger) Log(msg *SignedMessage, result *VerifyResult) {
	vl.mu.Lock()
	defer vl.mu.Unlock()

	log := &VerificationLog{
		MessageID:   msg.MessageID,
		MessageType: msg.MessageType,
		Sender:      msg.Sender,
		Timestamp:   time.UnixMilli(msg.Timestamp),
		Valid:       result.Valid,
		Error:       result.Error,
		VerifiedAt:  time.Now(),
	}

	vl.logs = append(vl.logs, log)

	// 限制日志数量
	if len(vl.logs) > vl.maxLogs {
		vl.logs = vl.logs[len(vl.logs)-vl.maxLogs:]
	}
}

// GetLogs 获取日志
func (vl *VerificationLogger) GetLogs(limit int) []*VerificationLog {
	vl.mu.RLock()
	defer vl.mu.RUnlock()

	if limit <= 0 || limit > len(vl.logs) {
		limit = len(vl.logs)
	}

	result := make([]*VerificationLog, limit)
	copy(result, vl.logs[len(vl.logs)-limit:])
	return result
}

// GetFailedLogs 获取失败的验证日志
func (vl *VerificationLogger) GetFailedLogs(limit int) []*VerificationLog {
	vl.mu.RLock()
	defer vl.mu.RUnlock()

	failed := make([]*VerificationLog, 0)
	for _, log := range vl.logs {
		if !log.Valid {
			failed = append(failed, log)
		}
	}

	if limit > 0 && limit < len(failed) {
		return failed[len(failed)-limit:]
	}
	return failed
}

// GetLogsBySender 获取指定发送者的日志
func (vl *VerificationLogger) GetLogsBySender(sender string, limit int) []*VerificationLog {
	vl.mu.RLock()
	defer vl.mu.RUnlock()

	result := make([]*VerificationLog, 0)
	for _, log := range vl.logs {
		if log.Sender == sender {
			result = append(result, log)
		}
	}

	if limit > 0 && limit < len(result) {
		return result[len(result)-limit:]
	}
	return result
}

// Stats 统计信息
type VerificationStats struct {
	TotalVerified int     `json:"total_verified"`
	ValidCount    int     `json:"valid_count"`
	InvalidCount  int     `json:"invalid_count"`
	ValidRate     float64 `json:"valid_rate"`
}

// GetStats 获取统计信息
func (vl *VerificationLogger) GetStats() *VerificationStats {
	vl.mu.RLock()
	defer vl.mu.RUnlock()

	stats := &VerificationStats{
		TotalVerified: len(vl.logs),
	}

	for _, log := range vl.logs {
		if log.Valid {
			stats.ValidCount++
		} else {
			stats.InvalidCount++
		}
	}

	if stats.TotalVerified > 0 {
		stats.ValidRate = float64(stats.ValidCount) / float64(stats.TotalVerified)
	}

	return stats
}

// Clear 清空日志
func (vl *VerificationLogger) Clear() {
	vl.mu.Lock()
	defer vl.mu.Unlock()
	vl.logs = make([]*VerificationLog, 0)
}

// ========== 辅助函数 ==========

// VerifyWithPublicKeyHex 使用公钥hex验证签名
func VerifyWithPublicKeyHex(pubKeyHex string, data, signature []byte) (bool, error) {
	pubKeyBytes, err := hex.DecodeString(pubKeyHex)
	if err != nil {
		return false, fmt.Errorf("解析公钥失败: %w", err)
	}

	pubKey := sm2.Decompress(pubKeyBytes)
	if pubKey == nil {
		return false, errors.New("解压公钥失败")
	}

	return pubKey.Verify(data, signature), nil
}

// ComputeMessageID 计算消息ID
func ComputeMessageID(sender string, timestamp int64, content []byte) string {
	data := fmt.Sprintf("%s%d%s", sender, timestamp, string(content))
	hash := sm3.Sm3Sum([]byte(data))
	return hex.EncodeToString(hash[:16])
}
