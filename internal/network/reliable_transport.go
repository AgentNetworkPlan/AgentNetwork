package network

import (
	"context"
	"encoding/binary"
	"errors"
	"fmt"
	"io"
	"sync"
	"time"

	"github.com/libp2p/go-libp2p/core/host"
	"github.com/libp2p/go-libp2p/core/network"
	"github.com/libp2p/go-libp2p/core/peer"
	"github.com/libp2p/go-libp2p/core/protocol"
	"github.com/tjfoc/gmsm/sm3"
)

const (
	// 消息头大小：类型(1) + 序号(4) + 总块数(4) + 块序号(4) + 数据长度(4) + 校验和(32) = 49 bytes
	MessageHeaderSize = 49
	// 最大块大小 (64KB)
	MaxChunkSize = 64 * 1024
	// 最大消息大小 (16MB)
	MaxMessageSize = 16 * 1024 * 1024
	// 默认重传次数
	DefaultMaxRetries = 3
	// 默认确认超时
	DefaultAckTimeout = 5 * time.Second
)

// MessageType 消息类型
type MessageType byte

const (
	MsgTypeData     MessageType = 0x01 // 数据消息
	MsgTypeAck      MessageType = 0x02 // 确认消息
	MsgTypeNack     MessageType = 0x03 // 否认消息（请求重传）
	MsgTypeComplete MessageType = 0x04 // 完成消息
)

// MessageHeader 消息头
type MessageHeader struct {
	Type       MessageType
	SequenceNo uint32 // 消息序号
	TotalChunks uint32 // 总块数
	ChunkIndex uint32 // 当前块序号
	DataLength uint32 // 数据长度
	Checksum   [32]byte // SM3 校验和
}

// ReliableMessage 可靠消息
type ReliableMessage struct {
	Header  MessageHeader
	Payload []byte
}

// ReliableTransportConfig 可靠传输配置
type ReliableTransportConfig struct {
	MaxRetries    int
	AckTimeout    time.Duration
	ChunkSize     int
	MaxMessageSize int
}

// DefaultReliableTransportConfig 默认配置
func DefaultReliableTransportConfig() *ReliableTransportConfig {
	return &ReliableTransportConfig{
		MaxRetries:    DefaultMaxRetries,
		AckTimeout:    DefaultAckTimeout,
		ChunkSize:     MaxChunkSize,
		MaxMessageSize: MaxMessageSize,
	}
}

// ReliableTransport 可靠传输层
type ReliableTransport struct {
	host     host.Host
	config   *ReliableTransportConfig
	protocol protocol.ID

	// 发送序号计数器
	seqCounter uint32
	seqMu      sync.Mutex

	// 接收缓冲区（处理分块消息）
	receiveBuffers map[string]*receiveBuffer
	bufferMu       sync.RWMutex

	// 消息处理器
	handlers   map[protocol.ID]MessageHandler
	handlerMu  sync.RWMutex

	ctx    context.Context
	cancel context.CancelFunc
}

// receiveBuffer 接收缓冲区
type receiveBuffer struct {
	totalChunks uint32
	received    map[uint32][]byte
	createdAt   time.Time
	mu          sync.Mutex
}

// MessageHandler 消息处理器
type MessageHandler func(peerID peer.ID, data []byte) error

// NewReliableTransport 创建可靠传输层
func NewReliableTransport(h host.Host, protocolID protocol.ID, cfg *ReliableTransportConfig) *ReliableTransport {
	if cfg == nil {
		cfg = DefaultReliableTransportConfig()
	}

	ctx, cancel := context.WithCancel(context.Background())

	rt := &ReliableTransport{
		host:           h,
		config:         cfg,
		protocol:       protocolID,
		receiveBuffers: make(map[string]*receiveBuffer),
		handlers:       make(map[protocol.ID]MessageHandler),
		ctx:            ctx,
		cancel:         cancel,
	}

	// 注册流处理器
	h.SetStreamHandler(protocolID, rt.handleStream)

	// 启动清理过期缓冲区的协程
	go rt.cleanupLoop()

	return rt
}

// SetHandler 设置消息处理器
func (rt *ReliableTransport) SetHandler(protocolID protocol.ID, handler MessageHandler) {
	rt.handlerMu.Lock()
	rt.handlers[protocolID] = handler
	rt.handlerMu.Unlock()
}

// Send 发送消息（可靠传输）
func (rt *ReliableTransport) Send(ctx context.Context, peerID peer.ID, data []byte) error {
	if len(data) > rt.config.MaxMessageSize {
		return fmt.Errorf("消息太大: %d > %d", len(data), rt.config.MaxMessageSize)
	}

	// 分块
	chunks := rt.splitIntoChunks(data)
	seqNo := rt.nextSequenceNo()

	// 打开流
	stream, err := rt.host.NewStream(ctx, peerID, rt.protocol)
	if err != nil {
		return fmt.Errorf("打开流失败: %w", err)
	}
	defer stream.Close()

	// 发送所有块
	for i, chunk := range chunks {
		msg := rt.createMessage(MsgTypeData, seqNo, uint32(len(chunks)), uint32(i), chunk)

		for retry := 0; retry <= rt.config.MaxRetries; retry++ {
			if err := rt.writeMessage(stream, msg); err != nil {
				if retry == rt.config.MaxRetries {
					return fmt.Errorf("发送块 %d 失败: %w", i, err)
				}
				continue
			}

			// 等待确认
			ack, err := rt.readAck(stream)
			if err != nil {
				if retry == rt.config.MaxRetries {
					return fmt.Errorf("接收确认失败: %w", err)
				}
				continue
			}

			if ack.Header.Type == MsgTypeNack {
				// 收到 NACK，重传
				continue
			}

			if ack.Header.Type == MsgTypeAck {
				// 确认成功，发送下一块
				break
			}
		}
	}

	// 发送完成消息
	completeMsg := rt.createMessage(MsgTypeComplete, seqNo, uint32(len(chunks)), 0, nil)
	return rt.writeMessage(stream, completeMsg)
}

// handleStream 处理入站流
func (rt *ReliableTransport) handleStream(stream network.Stream) {
	defer stream.Close()

	peerID := stream.Conn().RemotePeer()
	bufferKey := fmt.Sprintf("%s-%d", peerID.String(), time.Now().UnixNano())

	for {
		msg, err := rt.readMessage(stream)
		if err != nil {
			if err != io.EOF {
				fmt.Printf("读取消息失败: %v\n", err)
			}
			return
		}

		switch msg.Header.Type {
		case MsgTypeData:
			// 验证校验和
			if !rt.verifyChecksum(msg) {
				// 发送 NACK
				nack := rt.createMessage(MsgTypeNack, msg.Header.SequenceNo, 0, msg.Header.ChunkIndex, nil)
				rt.writeMessage(stream, nack)
				continue
			}

			// 存储块
			rt.storeChunk(bufferKey, msg)

			// 发送 ACK
			ack := rt.createMessage(MsgTypeAck, msg.Header.SequenceNo, 0, msg.Header.ChunkIndex, nil)
			rt.writeMessage(stream, ack)

		case MsgTypeComplete:
			// 组装完整消息
			data, err := rt.assembleMessage(bufferKey, msg.Header.TotalChunks)
			if err != nil {
				fmt.Printf("组装消息失败: %v\n", err)
				return
			}

			// 调用处理器
			rt.handlerMu.RLock()
			handler := rt.handlers[rt.protocol]
			rt.handlerMu.RUnlock()

			if handler != nil {
				if err := handler(peerID, data); err != nil {
					fmt.Printf("处理消息失败: %v\n", err)
				}
			}

			// 清理缓冲区
			rt.bufferMu.Lock()
			delete(rt.receiveBuffers, bufferKey)
			rt.bufferMu.Unlock()

			return
		}
	}
}

// splitIntoChunks 将数据分块
func (rt *ReliableTransport) splitIntoChunks(data []byte) [][]byte {
	chunks := make([][]byte, 0)
	for i := 0; i < len(data); i += rt.config.ChunkSize {
		end := i + rt.config.ChunkSize
		if end > len(data) {
			end = len(data)
		}
		chunks = append(chunks, data[i:end])
	}
	if len(chunks) == 0 {
		chunks = append(chunks, []byte{})
	}
	return chunks
}

// createMessage 创建消息
func (rt *ReliableTransport) createMessage(msgType MessageType, seqNo, totalChunks, chunkIndex uint32, data []byte) *ReliableMessage {
	msg := &ReliableMessage{
		Header: MessageHeader{
			Type:        msgType,
			SequenceNo:  seqNo,
			TotalChunks: totalChunks,
			ChunkIndex:  chunkIndex,
			DataLength:  uint32(len(data)),
		},
		Payload: data,
	}

	// 计算校验和
	if len(data) > 0 {
		hash := sm3.Sm3Sum(data)
		copy(msg.Header.Checksum[:], hash)
	}

	return msg
}

// writeMessage 写入消息
func (rt *ReliableTransport) writeMessage(stream network.Stream, msg *ReliableMessage) error {
	// 写入头部
	header := make([]byte, MessageHeaderSize)
	header[0] = byte(msg.Header.Type)
	binary.BigEndian.PutUint32(header[1:5], msg.Header.SequenceNo)
	binary.BigEndian.PutUint32(header[5:9], msg.Header.TotalChunks)
	binary.BigEndian.PutUint32(header[9:13], msg.Header.ChunkIndex)
	binary.BigEndian.PutUint32(header[13:17], msg.Header.DataLength)
	copy(header[17:49], msg.Header.Checksum[:])

	if _, err := stream.Write(header); err != nil {
		return err
	}

	// 写入数据
	if len(msg.Payload) > 0 {
		if _, err := stream.Write(msg.Payload); err != nil {
			return err
		}
	}

	return nil
}

// readMessage 读取消息
func (rt *ReliableTransport) readMessage(stream network.Stream) (*ReliableMessage, error) {
	// 读取头部
	header := make([]byte, MessageHeaderSize)
	if _, err := io.ReadFull(stream, header); err != nil {
		return nil, err
	}

	msg := &ReliableMessage{
		Header: MessageHeader{
			Type:        MessageType(header[0]),
			SequenceNo:  binary.BigEndian.Uint32(header[1:5]),
			TotalChunks: binary.BigEndian.Uint32(header[5:9]),
			ChunkIndex:  binary.BigEndian.Uint32(header[9:13]),
			DataLength:  binary.BigEndian.Uint32(header[13:17]),
		},
	}
	copy(msg.Header.Checksum[:], header[17:49])

	// 读取数据
	if msg.Header.DataLength > 0 {
		msg.Payload = make([]byte, msg.Header.DataLength)
		if _, err := io.ReadFull(stream, msg.Payload); err != nil {
			return nil, err
		}
	}

	return msg, nil
}

// readAck 读取确认消息
func (rt *ReliableTransport) readAck(stream network.Stream) (*ReliableMessage, error) {
	stream.SetReadDeadline(time.Now().Add(rt.config.AckTimeout))
	defer stream.SetReadDeadline(time.Time{})

	return rt.readMessage(stream)
}

// verifyChecksum 验证校验和
func (rt *ReliableTransport) verifyChecksum(msg *ReliableMessage) bool {
	if len(msg.Payload) == 0 {
		return true
	}
	expected := sm3.Sm3Sum(msg.Payload)
	for i := 0; i < 32; i++ {
		if expected[i] != msg.Header.Checksum[i] {
			return false
		}
	}
	return true
}

// storeChunk 存储块
func (rt *ReliableTransport) storeChunk(bufferKey string, msg *ReliableMessage) {
	rt.bufferMu.Lock()
	defer rt.bufferMu.Unlock()

	buf, exists := rt.receiveBuffers[bufferKey]
	if !exists {
		buf = &receiveBuffer{
			totalChunks: msg.Header.TotalChunks,
			received:    make(map[uint32][]byte),
			createdAt:   time.Now(),
		}
		rt.receiveBuffers[bufferKey] = buf
	}

	buf.mu.Lock()
	buf.received[msg.Header.ChunkIndex] = msg.Payload
	buf.mu.Unlock()
}

// assembleMessage 组装消息
func (rt *ReliableTransport) assembleMessage(bufferKey string, totalChunks uint32) ([]byte, error) {
	rt.bufferMu.RLock()
	buf, exists := rt.receiveBuffers[bufferKey]
	rt.bufferMu.RUnlock()

	if !exists {
		return nil, errors.New("缓冲区不存在")
	}

	buf.mu.Lock()
	defer buf.mu.Unlock()

	if uint32(len(buf.received)) != totalChunks {
		return nil, fmt.Errorf("块数不完整: %d/%d", len(buf.received), totalChunks)
	}

	// 按顺序组装
	var data []byte
	for i := uint32(0); i < totalChunks; i++ {
		chunk, ok := buf.received[i]
		if !ok {
			return nil, fmt.Errorf("缺少块 %d", i)
		}
		data = append(data, chunk...)
	}

	return data, nil
}

// nextSequenceNo 获取下一个序号
func (rt *ReliableTransport) nextSequenceNo() uint32 {
	rt.seqMu.Lock()
	defer rt.seqMu.Unlock()
	rt.seqCounter++
	return rt.seqCounter
}

// cleanupLoop 清理过期缓冲区
func (rt *ReliableTransport) cleanupLoop() {
	ticker := time.NewTicker(time.Minute)
	defer ticker.Stop()

	for {
		select {
		case <-rt.ctx.Done():
			return
		case <-ticker.C:
			rt.cleanupExpiredBuffers()
		}
	}
}

// cleanupExpiredBuffers 清理过期的接收缓冲区
func (rt *ReliableTransport) cleanupExpiredBuffers() {
	rt.bufferMu.Lock()
	defer rt.bufferMu.Unlock()

	cutoff := time.Now().Add(-5 * time.Minute)
	for key, buf := range rt.receiveBuffers {
		if buf.createdAt.Before(cutoff) {
			delete(rt.receiveBuffers, key)
		}
	}
}

// Stop 停止传输层
func (rt *ReliableTransport) Stop() {
	rt.cancel()
	rt.host.RemoveStreamHandler(rt.protocol)
}

// Stats 传输统计
type TransportStats struct {
	PendingBuffers int `json:"pending_buffers"`
	SequenceNo     uint32 `json:"sequence_no"`
}

// GetStats 获取统计信息
func (rt *ReliableTransport) GetStats() *TransportStats {
	rt.bufferMu.RLock()
	pendingBuffers := len(rt.receiveBuffers)
	rt.bufferMu.RUnlock()

	rt.seqMu.Lock()
	seqNo := rt.seqCounter
	rt.seqMu.Unlock()

	return &TransportStats{
		PendingBuffers: pendingBuffers,
		SequenceNo:     seqNo,
	}
}

// ========== 简单消息发送（不分块）==========

// SendSimple 发送简单消息（不分块，适用于小消息）
func (rt *ReliableTransport) SendSimple(ctx context.Context, peerID peer.ID, data []byte) error {
	if len(data) > rt.config.ChunkSize {
		return rt.Send(ctx, peerID, data)
	}

	stream, err := rt.host.NewStream(ctx, peerID, rt.protocol)
	if err != nil {
		return fmt.Errorf("打开流失败: %w", err)
	}
	defer stream.Close()

	// 直接写入数据长度和数据
	lenBuf := make([]byte, 4)
	binary.BigEndian.PutUint32(lenBuf, uint32(len(data)))

	if _, err := stream.Write(lenBuf); err != nil {
		return err
	}
	if _, err := stream.Write(data); err != nil {
		return err
	}

	return nil
}

// ReceiveSimple 接收简单消息
func (rt *ReliableTransport) ReceiveSimple(stream network.Stream) ([]byte, error) {
	lenBuf := make([]byte, 4)
	if _, err := io.ReadFull(stream, lenBuf); err != nil {
		return nil, err
	}

	dataLen := binary.BigEndian.Uint32(lenBuf)
	if dataLen > uint32(rt.config.MaxMessageSize) {
		return nil, fmt.Errorf("消息太大: %d", dataLen)
	}

	data := make([]byte, dataLen)
	if _, err := io.ReadFull(stream, data); err != nil {
		return nil, err
	}

	return data, nil
}
