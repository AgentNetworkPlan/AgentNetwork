package network

import (
	"context"
	"encoding/binary"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"sync"
	"sync/atomic"
	"time"

	"github.com/libp2p/go-libp2p/core/host"
	"github.com/libp2p/go-libp2p/core/network"
	"github.com/libp2p/go-libp2p/core/peer"
	"github.com/libp2p/go-libp2p/core/protocol"
	"github.com/multiformats/go-multiaddr"
)

const (
	// ProtocolMessage 消息协议
	ProtocolMessage = protocol.ID("/daan/message/1.0.0")
	// ProtocolRequest 请求-响应协议
	ProtocolRequest = protocol.ID("/daan/request/1.0.0")

	// 默认超时
	DefaultConnectTimeout = 30 * time.Second
	DefaultRequestTimeout = 30 * time.Second
	DefaultMessageTimeout = 10 * time.Second

	// 最大简单消息大小 (1MB)
	MaxSimpleMessageSize = 1024 * 1024
)

// MessageType 消息类型
type MsgType byte

const (
	MsgTypeOneWay   MsgType = 0x01 // 单向消息
	MsgTypeRequest  MsgType = 0x02 // 请求
	MsgTypeResponse MsgType = 0x03 // 响应
	MsgTypeError    MsgType = 0x04 // 错误响应
)

// Message 网络消息
type Message struct {
	Type      MsgType `json:"type"`
	RequestID uint64  `json:"request_id,omitempty"`
	Payload   []byte  `json:"payload"`
}

// MessageHandler 消息处理器
type MessageHandlerFunc func(ctx context.Context, peerID peer.ID, payload []byte) error

// RequestHandler 请求处理器
type RequestHandlerFunc func(ctx context.Context, peerID peer.ID, payload []byte) ([]byte, error)

// Messenger 消息通信器
type Messenger struct {
	host host.Host

	// 请求 ID 计数器
	requestCounter uint64

	// 消息处理器
	messageHandler MessageHandlerFunc
	requestHandler RequestHandlerFunc
	handlerMu      sync.RWMutex

	// 待处理的请求（等待响应）
	pendingRequests map[uint64]chan *Message
	pendingMu       sync.RWMutex

	ctx    context.Context
	cancel context.CancelFunc
}

// NewMessenger 创建消息通信器
func NewMessenger(h host.Host) *Messenger {
	ctx, cancel := context.WithCancel(context.Background())

	m := &Messenger{
		host:            h,
		pendingRequests: make(map[uint64]chan *Message),
		ctx:             ctx,
		cancel:          cancel,
	}

	// 注册流处理器
	h.SetStreamHandler(ProtocolMessage, m.handleMessageStream)
	h.SetStreamHandler(ProtocolRequest, m.handleRequestStream)

	return m
}

// SetMessageHandler 设置消息处理器
func (m *Messenger) SetMessageHandler(handler MessageHandlerFunc) {
	m.handlerMu.Lock()
	m.messageHandler = handler
	m.handlerMu.Unlock()
}

// SetRequestHandler 设置请求处理器
func (m *Messenger) SetRequestHandler(handler RequestHandlerFunc) {
	m.handlerMu.Lock()
	m.requestHandler = handler
	m.handlerMu.Unlock()
}

// ConnectToPeer 连接到指定节点
func (m *Messenger) ConnectToPeer(peerIDStr string) error {
	return m.ConnectToPeerWithTimeout(peerIDStr, DefaultConnectTimeout)
}

// ConnectToPeerWithTimeout 带超时连接到指定节点
func (m *Messenger) ConnectToPeerWithTimeout(peerIDStr string, timeout time.Duration) error {
	peerID, err := peer.Decode(peerIDStr)
	if err != nil {
		return fmt.Errorf("无效的 PeerID: %w", err)
	}

	ctx, cancel := context.WithTimeout(m.ctx, timeout)
	defer cancel()

	// 检查是否已连接
	if m.host.Network().Connectedness(peerID) == network.Connected {
		return nil
	}

	// 尝试从 peerstore 获取地址
	addrs := m.host.Peerstore().Addrs(peerID)
	if len(addrs) == 0 {
		return fmt.Errorf("未找到节点 %s 的地址", peerIDStr)
	}

	addrInfo := peer.AddrInfo{
		ID:    peerID,
		Addrs: addrs,
	}

	if err := m.host.Connect(ctx, addrInfo); err != nil {
		return fmt.Errorf("连接节点失败: %w", err)
	}

	return nil
}

// ConnectToPeerAddr 通过完整地址连接节点
func (m *Messenger) ConnectToPeerAddr(addrStr string) error {
	ma, err := multiaddr.NewMultiaddr(addrStr)
	if err != nil {
		return fmt.Errorf("无效的地址: %w", err)
	}

	addrInfo, err := peer.AddrInfoFromP2pAddr(ma)
	if err != nil {
		return fmt.Errorf("解析地址失败: %w", err)
	}

	ctx, cancel := context.WithTimeout(m.ctx, DefaultConnectTimeout)
	defer cancel()

	if err := m.host.Connect(ctx, *addrInfo); err != nil {
		return fmt.Errorf("连接节点失败: %w", err)
	}

	return nil
}

// SendMessage 发送单向消息
func (m *Messenger) SendMessage(peerIDStr string, payload []byte) error {
	return m.SendMessageWithTimeout(peerIDStr, payload, DefaultMessageTimeout)
}

// SendMessageWithTimeout 带超时发送消息
func (m *Messenger) SendMessageWithTimeout(peerIDStr string, payload []byte, timeout time.Duration) error {
	peerID, err := peer.Decode(peerIDStr)
	if err != nil {
		return fmt.Errorf("无效的 PeerID: %w", err)
	}

	ctx, cancel := context.WithTimeout(m.ctx, timeout)
	defer cancel()

	stream, err := m.host.NewStream(ctx, peerID, ProtocolMessage)
	if err != nil {
		return fmt.Errorf("打开流失败: %w", err)
	}
	defer stream.Close()

	msg := &Message{
		Type:    MsgTypeOneWay,
		Payload: payload,
	}

	return m.writeMessage(stream, msg)
}

// Request 发送请求并等待响应
func (m *Messenger) Request(peerIDStr string, payload []byte) ([]byte, error) {
	return m.RequestWithTimeout(peerIDStr, payload, DefaultRequestTimeout)
}

// RequestWithTimeout 带超时发送请求
func (m *Messenger) RequestWithTimeout(peerIDStr string, payload []byte, timeout time.Duration) ([]byte, error) {
	peerID, err := peer.Decode(peerIDStr)
	if err != nil {
		return nil, fmt.Errorf("无效的 PeerID: %w", err)
	}

	ctx, cancel := context.WithTimeout(m.ctx, timeout)
	defer cancel()

	stream, err := m.host.NewStream(ctx, peerID, ProtocolRequest)
	if err != nil {
		return nil, fmt.Errorf("打开流失败: %w", err)
	}
	defer stream.Close()

	// 生成请求 ID
	requestID := atomic.AddUint64(&m.requestCounter, 1)

	// 发送请求
	msg := &Message{
		Type:      MsgTypeRequest,
		RequestID: requestID,
		Payload:   payload,
	}

	if err := m.writeMessage(stream, msg); err != nil {
		return nil, fmt.Errorf("发送请求失败: %w", err)
	}

	// 等待响应
	stream.SetReadDeadline(time.Now().Add(timeout))
	resp, err := m.readMessage(stream)
	if err != nil {
		return nil, fmt.Errorf("读取响应失败: %w", err)
	}

	if resp.Type == MsgTypeError {
		return nil, fmt.Errorf("远程错误: %s", string(resp.Payload))
	}

	return resp.Payload, nil
}

// handleMessageStream 处理消息流
func (m *Messenger) handleMessageStream(stream network.Stream) {
	defer stream.Close()

	peerID := stream.Conn().RemotePeer()

	msg, err := m.readMessage(stream)
	if err != nil {
		return
	}

	m.handlerMu.RLock()
	handler := m.messageHandler
	m.handlerMu.RUnlock()

	if handler != nil {
		ctx, cancel := context.WithTimeout(m.ctx, 30*time.Second)
		defer cancel()
		handler(ctx, peerID, msg.Payload)
	}
}

// handleRequestStream 处理请求流
func (m *Messenger) handleRequestStream(stream network.Stream) {
	defer stream.Close()

	peerID := stream.Conn().RemotePeer()

	msg, err := m.readMessage(stream)
	if err != nil {
		return
	}

	m.handlerMu.RLock()
	handler := m.requestHandler
	m.handlerMu.RUnlock()

	var resp *Message
	if handler != nil {
		ctx, cancel := context.WithTimeout(m.ctx, 30*time.Second)
		result, err := handler(ctx, peerID, msg.Payload)
		cancel()

		if err != nil {
			resp = &Message{
				Type:      MsgTypeError,
				RequestID: msg.RequestID,
				Payload:   []byte(err.Error()),
			}
		} else {
			resp = &Message{
				Type:      MsgTypeResponse,
				RequestID: msg.RequestID,
				Payload:   result,
			}
		}
	} else {
		resp = &Message{
			Type:      MsgTypeError,
			RequestID: msg.RequestID,
			Payload:   []byte("no handler registered"),
		}
	}

	m.writeMessage(stream, resp)
}

// writeMessage 写入消息
func (m *Messenger) writeMessage(stream network.Stream, msg *Message) error {
	data, err := json.Marshal(msg)
	if err != nil {
		return err
	}

	if len(data) > MaxMessageSize {
		return errors.New("消息太大")
	}

	// 写入长度前缀
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

// readMessage 读取消息
func (m *Messenger) readMessage(stream network.Stream) (*Message, error) {
	// 读取长度前缀
	lenBuf := make([]byte, 4)
	if _, err := io.ReadFull(stream, lenBuf); err != nil {
		return nil, err
	}

	length := binary.BigEndian.Uint32(lenBuf)
	if length > MaxMessageSize {
		return nil, errors.New("消息太大")
	}

	// 读取消息体
	data := make([]byte, length)
	if _, err := io.ReadFull(stream, data); err != nil {
		return nil, err
	}

	var msg Message
	if err := json.Unmarshal(data, &msg); err != nil {
		return nil, err
	}

	return &msg, nil
}

// IsConnected 检查是否已连接到指定节点
func (m *Messenger) IsConnected(peerIDStr string) bool {
	peerID, err := peer.Decode(peerIDStr)
	if err != nil {
		return false
	}
	return m.host.Network().Connectedness(peerID) == network.Connected
}

// GetConnectedPeers 获取已连接的节点列表
func (m *Messenger) GetConnectedPeers() []string {
	peers := m.host.Network().Peers()
	result := make([]string, len(peers))
	for i, p := range peers {
		result[i] = p.String()
	}
	return result
}

// Disconnect 断开与指定节点的连接
func (m *Messenger) Disconnect(peerIDStr string) error {
	peerID, err := peer.Decode(peerIDStr)
	if err != nil {
		return fmt.Errorf("无效的 PeerID: %w", err)
	}

	return m.host.Network().ClosePeer(peerID)
}

// Stop 停止消息通信器
func (m *Messenger) Stop() {
	m.cancel()
	m.host.RemoveStreamHandler(ProtocolMessage)
	m.host.RemoveStreamHandler(ProtocolRequest)
}

// ========== 便捷方法 ==========

// SendJSON 发送 JSON 消息
func (m *Messenger) SendJSON(peerIDStr string, v interface{}) error {
	data, err := json.Marshal(v)
	if err != nil {
		return fmt.Errorf("序列化失败: %w", err)
	}
	return m.SendMessage(peerIDStr, data)
}

// RequestJSON 发送 JSON 请求并解析响应
func (m *Messenger) RequestJSON(peerIDStr string, req interface{}, resp interface{}) error {
	data, err := json.Marshal(req)
	if err != nil {
		return fmt.Errorf("序列化请求失败: %w", err)
	}

	respData, err := m.Request(peerIDStr, data)
	if err != nil {
		return err
	}

	if err := json.Unmarshal(respData, resp); err != nil {
		return fmt.Errorf("解析响应失败: %w", err)
	}

	return nil
}
