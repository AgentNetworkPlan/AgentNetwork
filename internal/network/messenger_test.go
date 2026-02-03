package network

import (
	"context"
	"encoding/json"
	"sync"
	"testing"
	"time"

	"github.com/libp2p/go-libp2p"
	"github.com/libp2p/go-libp2p/core/peer"
)

func TestNewMessenger(t *testing.T) {
	h, err := libp2p.New()
	if err != nil {
		t.Fatalf("创建主机失败: %v", err)
	}
	defer h.Close()

	m := NewMessenger(h)
	if m == nil {
		t.Fatal("Messenger 不应为 nil")
	}

	m.Stop()
}

func TestMessengerConnectToPeer(t *testing.T) {
	// 创建两个主机
	h1, _ := libp2p.New()
	defer h1.Close()

	h2, _ := libp2p.New()
	defer h2.Close()

	m1 := NewMessenger(h1)
	defer m1.Stop()

	// 将 h2 的地址添加到 h1 的 peerstore
	h1.Peerstore().AddAddrs(h2.ID(), h2.Addrs(), time.Hour)

	// 连接
	err := m1.ConnectToPeer(h2.ID().String())
	if err != nil {
		t.Fatalf("连接失败: %v", err)
	}

	// 检查连接状态
	if !m1.IsConnected(h2.ID().String()) {
		t.Error("应该已连接")
	}
}

func TestMessengerConnectToPeerAddr(t *testing.T) {
	h1, _ := libp2p.New()
	defer h1.Close()

	h2, _ := libp2p.New()
	defer h2.Close()

	m1 := NewMessenger(h1)
	defer m1.Stop()

	// 构建完整地址
	if len(h2.Addrs()) == 0 {
		t.Fatal("h2 没有地址")
	}
	fullAddr := h2.Addrs()[0].String() + "/p2p/" + h2.ID().String()

	err := m1.ConnectToPeerAddr(fullAddr)
	if err != nil {
		t.Fatalf("连接失败: %v", err)
	}

	if !m1.IsConnected(h2.ID().String()) {
		t.Error("应该已连接")
	}
}

func TestMessengerSendMessage(t *testing.T) {
	h1, _ := libp2p.New()
	defer h1.Close()

	h2, _ := libp2p.New()
	defer h2.Close()

	m1 := NewMessenger(h1)
	defer m1.Stop()

	m2 := NewMessenger(h2)
	defer m2.Stop()

	// 设置消息处理器
	var receivedPayload []byte
	var receivedFrom peer.ID
	var wg sync.WaitGroup
	wg.Add(1)

	m2.SetMessageHandler(func(ctx context.Context, peerID peer.ID, payload []byte) error {
		receivedPayload = payload
		receivedFrom = peerID
		wg.Done()
		return nil
	})

	// 连接
	h1.Peerstore().AddAddrs(h2.ID(), h2.Addrs(), time.Hour)
	m1.ConnectToPeer(h2.ID().String())

	// 发送消息
	testPayload := []byte("Hello, World!")
	err := m1.SendMessage(h2.ID().String(), testPayload)
	if err != nil {
		t.Fatalf("发送消息失败: %v", err)
	}

	// 等待接收
	done := make(chan struct{})
	go func() {
		wg.Wait()
		close(done)
	}()

	select {
	case <-done:
		if string(receivedPayload) != string(testPayload) {
			t.Errorf("接收到的消息不匹配: got %s, want %s", receivedPayload, testPayload)
		}
		if receivedFrom != h1.ID() {
			t.Errorf("发送者不匹配: got %s, want %s", receivedFrom, h1.ID())
		}
	case <-time.After(5 * time.Second):
		t.Fatal("等待消息超时")
	}
}

func TestMessengerRequest(t *testing.T) {
	h1, _ := libp2p.New()
	defer h1.Close()

	h2, _ := libp2p.New()
	defer h2.Close()

	m1 := NewMessenger(h1)
	defer m1.Stop()

	m2 := NewMessenger(h2)
	defer m2.Stop()

	// 设置请求处理器
	m2.SetRequestHandler(func(ctx context.Context, peerID peer.ID, payload []byte) ([]byte, error) {
		// 回显请求 + 前缀
		return append([]byte("Response: "), payload...), nil
	})

	// 连接
	h1.Peerstore().AddAddrs(h2.ID(), h2.Addrs(), time.Hour)
	m1.ConnectToPeer(h2.ID().String())

	// 发送请求
	testPayload := []byte("Hello")
	resp, err := m1.Request(h2.ID().String(), testPayload)
	if err != nil {
		t.Fatalf("请求失败: %v", err)
	}

	expected := "Response: Hello"
	if string(resp) != expected {
		t.Errorf("响应不匹配: got %s, want %s", resp, expected)
	}
}

func TestMessengerRequestJSON(t *testing.T) {
	h1, _ := libp2p.New()
	defer h1.Close()

	h2, _ := libp2p.New()
	defer h2.Close()

	m1 := NewMessenger(h1)
	defer m1.Stop()

	m2 := NewMessenger(h2)
	defer m2.Stop()

	type Request struct {
		Name string `json:"name"`
		Age  int    `json:"age"`
	}

	type Response struct {
		Message string `json:"message"`
	}

	// 设置请求处理器
	m2.SetRequestHandler(func(ctx context.Context, peerID peer.ID, payload []byte) ([]byte, error) {
		var req Request
		if err := json.Unmarshal(payload, &req); err != nil {
			return nil, err
		}
		resp := Response{Message: "Hello, " + req.Name}
		return json.Marshal(resp)
	})

	// 连接
	h1.Peerstore().AddAddrs(h2.ID(), h2.Addrs(), time.Hour)
	m1.ConnectToPeer(h2.ID().String())

	// 发送 JSON 请求
	req := Request{Name: "Alice", Age: 30}
	var resp Response
	err := m1.RequestJSON(h2.ID().String(), req, &resp)
	if err != nil {
		t.Fatalf("JSON 请求失败: %v", err)
	}

	if resp.Message != "Hello, Alice" {
		t.Errorf("响应不匹配: got %s", resp.Message)
	}
}

func TestMessengerGetConnectedPeers(t *testing.T) {
	h1, _ := libp2p.New()
	defer h1.Close()

	h2, _ := libp2p.New()
	defer h2.Close()

	h3, _ := libp2p.New()
	defer h3.Close()

	m1 := NewMessenger(h1)
	defer m1.Stop()

	// 初始应该没有连接
	peers := m1.GetConnectedPeers()
	if len(peers) != 0 {
		t.Errorf("初始应该没有连接，实际有 %d", len(peers))
	}

	// 连接 h2 和 h3
	h1.Peerstore().AddAddrs(h2.ID(), h2.Addrs(), time.Hour)
	h1.Peerstore().AddAddrs(h3.ID(), h3.Addrs(), time.Hour)

	m1.ConnectToPeer(h2.ID().String())
	m1.ConnectToPeer(h3.ID().String())

	peers = m1.GetConnectedPeers()
	if len(peers) != 2 {
		t.Errorf("应该有 2 个连接，实际有 %d", len(peers))
	}
}

func TestMessengerDisconnect(t *testing.T) {
	h1, _ := libp2p.New()
	defer h1.Close()

	h2, _ := libp2p.New()
	defer h2.Close()

	m1 := NewMessenger(h1)
	defer m1.Stop()

	// 连接
	h1.Peerstore().AddAddrs(h2.ID(), h2.Addrs(), time.Hour)
	m1.ConnectToPeer(h2.ID().String())

	if !m1.IsConnected(h2.ID().String()) {
		t.Fatal("应该已连接")
	}

	// 断开
	err := m1.Disconnect(h2.ID().String())
	if err != nil {
		t.Fatalf("断开连接失败: %v", err)
	}

	// 等待断开
	time.Sleep(100 * time.Millisecond)

	if m1.IsConnected(h2.ID().String()) {
		t.Error("应该已断开")
	}
}

func TestMessengerInvalidPeerID(t *testing.T) {
	h, _ := libp2p.New()
	defer h.Close()

	m := NewMessenger(h)
	defer m.Stop()

	// 无效的 PeerID
	err := m.ConnectToPeer("invalid-peer-id")
	if err == nil {
		t.Error("应该返回错误")
	}

	err = m.SendMessage("invalid-peer-id", []byte("test"))
	if err == nil {
		t.Error("应该返回错误")
	}

	_, err = m.Request("invalid-peer-id", []byte("test"))
	if err == nil {
		t.Error("应该返回错误")
	}
}

func TestMessageTypes(t *testing.T) {
	if MsgTypeOneWay != 0x01 {
		t.Error("MsgTypeOneWay 值错误")
	}
	if MsgTypeRequest != 0x02 {
		t.Error("MsgTypeRequest 值错误")
	}
	if MsgTypeResponse != 0x03 {
		t.Error("MsgTypeResponse 值错误")
	}
	if MsgTypeError != 0x04 {
		t.Error("MsgTypeError 值错误")
	}
}

func TestProtocolConstants(t *testing.T) {
	if ProtocolMessage != "/daan/message/1.0.0" {
		t.Error("ProtocolMessage 值错误")
	}
	if ProtocolRequest != "/daan/request/1.0.0" {
		t.Error("ProtocolRequest 值错误")
	}
}

func BenchmarkSendMessage(b *testing.B) {
	h1, _ := libp2p.New()
	defer h1.Close()

	h2, _ := libp2p.New()
	defer h2.Close()

	m1 := NewMessenger(h1)
	defer m1.Stop()

	m2 := NewMessenger(h2)
	defer m2.Stop()

	m2.SetMessageHandler(func(ctx context.Context, peerID peer.ID, payload []byte) error {
		return nil
	})

	h1.Peerstore().AddAddrs(h2.ID(), h2.Addrs(), time.Hour)
	m1.ConnectToPeer(h2.ID().String())

	payload := make([]byte, 1024)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		m1.SendMessage(h2.ID().String(), payload)
	}
}

func BenchmarkRequest(b *testing.B) {
	h1, _ := libp2p.New()
	defer h1.Close()

	h2, _ := libp2p.New()
	defer h2.Close()

	m1 := NewMessenger(h1)
	defer m1.Stop()

	m2 := NewMessenger(h2)
	defer m2.Stop()

	m2.SetRequestHandler(func(ctx context.Context, peerID peer.ID, payload []byte) ([]byte, error) {
		return payload, nil
	})

	h1.Peerstore().AddAddrs(h2.ID(), h2.Addrs(), time.Hour)
	m1.ConnectToPeer(h2.ID().String())

	payload := make([]byte, 1024)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		m1.Request(h2.ID().String(), payload)
	}
}
