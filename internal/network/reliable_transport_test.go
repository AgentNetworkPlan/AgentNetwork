package network

import (
	"bytes"
	"context"
	"sync"
	"testing"
	"time"

	"github.com/libp2p/go-libp2p"
	"github.com/libp2p/go-libp2p/core/peer"
	"github.com/libp2p/go-libp2p/core/protocol"
)

const testProtocol = protocol.ID("/test/reliable/1.0.0")

func TestNewReliableTransport(t *testing.T) {
	h, err := libp2p.New()
	if err != nil {
		t.Fatalf("创建主机失败: %v", err)
	}
	defer h.Close()

	rt := NewReliableTransport(h, testProtocol, nil)
	if rt == nil {
		t.Fatal("ReliableTransport 不应为 nil")
	}

	rt.Stop()
}

func TestDefaultReliableTransportConfig(t *testing.T) {
	cfg := DefaultReliableTransportConfig()

	if cfg.MaxRetries != 3 {
		t.Errorf("默认重试次数错误: %d", cfg.MaxRetries)
	}
	if cfg.AckTimeout != 5*time.Second {
		t.Errorf("默认确认超时错误: %v", cfg.AckTimeout)
	}
	if cfg.ChunkSize != MaxChunkSize {
		t.Errorf("默认块大小错误: %d", cfg.ChunkSize)
	}
	if cfg.MaxMessageSize != MaxMessageSize {
		t.Errorf("默认最大消息大小错误: %d", cfg.MaxMessageSize)
	}
}

func TestSplitIntoChunks(t *testing.T) {
	h, _ := libp2p.New()
	defer h.Close()

	cfg := &ReliableTransportConfig{
		ChunkSize: 10,
	}
	rt := NewReliableTransport(h, testProtocol, cfg)
	defer rt.Stop()

	tests := []struct {
		name     string
		data     []byte
		expected int
	}{
		{"空数据", []byte{}, 1},
		{"小于块大小", []byte("hello"), 1},
		{"等于块大小", []byte("0123456789"), 1},
		{"两个块", []byte("0123456789abc"), 2},
		{"三个块", []byte("0123456789abcdefghijk"), 3},
	}

	for _, tc := range tests {
		chunks := rt.splitIntoChunks(tc.data)
		if len(chunks) != tc.expected {
			t.Errorf("%s: 块数错误 got %d, want %d", tc.name, len(chunks), tc.expected)
		}

		// 验证重组
		var reassembled []byte
		for _, chunk := range chunks {
			reassembled = append(reassembled, chunk...)
		}
		if !bytes.Equal(reassembled, tc.data) {
			t.Errorf("%s: 重组数据不匹配", tc.name)
		}
	}
}

func TestMessageTypeConstants(t *testing.T) {
	if MsgTypeData != 0x01 {
		t.Error("MsgTypeData 值错误")
	}
	if MsgTypeAck != 0x02 {
		t.Error("MsgTypeAck 值错误")
	}
	if MsgTypeNack != 0x03 {
		t.Error("MsgTypeNack 值错误")
	}
	if MsgTypeComplete != 0x04 {
		t.Error("MsgTypeComplete 值错误")
	}
}

func TestCreateMessage(t *testing.T) {
	h, _ := libp2p.New()
	defer h.Close()

	rt := NewReliableTransport(h, testProtocol, nil)
	defer rt.Stop()

	data := []byte("test message data")
	msg := rt.createMessage(MsgTypeData, 1, 5, 2, data)

	if msg.Header.Type != MsgTypeData {
		t.Errorf("消息类型错误: %d", msg.Header.Type)
	}
	if msg.Header.SequenceNo != 1 {
		t.Errorf("序号错误: %d", msg.Header.SequenceNo)
	}
	if msg.Header.TotalChunks != 5 {
		t.Errorf("总块数错误: %d", msg.Header.TotalChunks)
	}
	if msg.Header.ChunkIndex != 2 {
		t.Errorf("块索引错误: %d", msg.Header.ChunkIndex)
	}
	if msg.Header.DataLength != uint32(len(data)) {
		t.Errorf("数据长度错误: %d", msg.Header.DataLength)
	}
	if !bytes.Equal(msg.Payload, data) {
		t.Error("Payload 不匹配")
	}
}

func TestVerifyChecksum(t *testing.T) {
	h, _ := libp2p.New()
	defer h.Close()

	rt := NewReliableTransport(h, testProtocol, nil)
	defer rt.Stop()

	// 正确的消息
	data := []byte("test data for checksum")
	msg := rt.createMessage(MsgTypeData, 1, 1, 0, data)

	if !rt.verifyChecksum(msg) {
		t.Error("校验和验证应该通过")
	}

	// 篡改数据
	msg.Payload[0] = 'X'
	if rt.verifyChecksum(msg) {
		t.Error("篡改后校验和验证应该失败")
	}

	// 空数据
	emptyMsg := rt.createMessage(MsgTypeAck, 1, 0, 0, nil)
	if !rt.verifyChecksum(emptyMsg) {
		t.Error("空数据校验和验证应该通过")
	}
}

func TestSequenceNo(t *testing.T) {
	h, _ := libp2p.New()
	defer h.Close()

	rt := NewReliableTransport(h, testProtocol, nil)
	defer rt.Stop()

	seq1 := rt.nextSequenceNo()
	seq2 := rt.nextSequenceNo()
	seq3 := rt.nextSequenceNo()

	if seq1 != 1 {
		t.Errorf("第一个序号应该是 1, got %d", seq1)
	}
	if seq2 != 2 {
		t.Errorf("第二个序号应该是 2, got %d", seq2)
	}
	if seq3 != 3 {
		t.Errorf("第三个序号应该是 3, got %d", seq3)
	}
}

func TestTransportGetStats(t *testing.T) {
	h, _ := libp2p.New()
	defer h.Close()

	rt := NewReliableTransport(h, testProtocol, nil)
	defer rt.Stop()

	stats := rt.GetStats()
	if stats == nil {
		t.Fatal("Stats 不应为 nil")
	}
	if stats.PendingBuffers != 0 {
		t.Errorf("初始 PendingBuffers 应该是 0, got %d", stats.PendingBuffers)
	}

	// 递增序号
	rt.nextSequenceNo()
	rt.nextSequenceNo()

	stats = rt.GetStats()
	if stats.SequenceNo != 2 {
		t.Errorf("SequenceNo 应该是 2, got %d", stats.SequenceNo)
	}
}

func TestTwoNodesReliableTransport(t *testing.T) {
	// 创建两个主机
	h1, err := libp2p.New()
	if err != nil {
		t.Fatalf("创建主机1失败: %v", err)
	}
	defer h1.Close()

	h2, err := libp2p.New()
	if err != nil {
		t.Fatalf("创建主机2失败: %v", err)
	}
	defer h2.Close()

	// 创建传输层
	rt1 := NewReliableTransport(h1, testProtocol, nil)
	defer rt1.Stop()

	rt2 := NewReliableTransport(h2, testProtocol, nil)
	defer rt2.Stop()

	// 设置消息处理器
	var receivedData []byte
	var wg sync.WaitGroup
	wg.Add(1)

	rt2.SetHandler(testProtocol, func(peerID peer.ID, data []byte) error {
		receivedData = data
		wg.Done()
		return nil
	})

	// 连接两个主机
	h2Info := peer.AddrInfo{
		ID:    h2.ID(),
		Addrs: h2.Addrs(),
	}

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	if err := h1.Connect(ctx, h2Info); err != nil {
		t.Fatalf("连接失败: %v", err)
	}

	// 发送消息
	testData := []byte("Hello, reliable transport!")
	if err := rt1.Send(ctx, h2.ID(), testData); err != nil {
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
		if !bytes.Equal(receivedData, testData) {
			t.Errorf("接收数据不匹配: got %s, want %s", receivedData, testData)
		}
	case <-time.After(5 * time.Second):
		t.Fatal("等待接收超时")
	}
}

func TestSendLargeMessage(t *testing.T) {
	// 创建两个主机
	h1, _ := libp2p.New()
	defer h1.Close()

	h2, _ := libp2p.New()
	defer h2.Close()

	// 使用小块大小以测试分块
	cfg := &ReliableTransportConfig{
		MaxRetries:    3,
		AckTimeout:    5 * time.Second,
		ChunkSize:     100, // 100 bytes per chunk
		MaxMessageSize: 10000,
	}

	rt1 := NewReliableTransport(h1, testProtocol, cfg)
	defer rt1.Stop()

	rt2 := NewReliableTransport(h2, testProtocol, cfg)
	defer rt2.Stop()

	var receivedData []byte
	var wg sync.WaitGroup
	wg.Add(1)

	rt2.SetHandler(testProtocol, func(peerID peer.ID, data []byte) error {
		receivedData = data
		wg.Done()
		return nil
	})

	// 连接
	h2Info := peer.AddrInfo{ID: h2.ID(), Addrs: h2.Addrs()}
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	h1.Connect(ctx, h2Info)

	// 发送大消息（500 bytes，分成5块）
	testData := make([]byte, 500)
	for i := range testData {
		testData[i] = byte(i % 256)
	}

	if err := rt1.Send(ctx, h2.ID(), testData); err != nil {
		t.Fatalf("发送大消息失败: %v", err)
	}

	done := make(chan struct{})
	go func() {
		wg.Wait()
		close(done)
	}()

	select {
	case <-done:
		if !bytes.Equal(receivedData, testData) {
			t.Errorf("大消息数据不匹配: len(received)=%d, len(expected)=%d", len(receivedData), len(testData))
		}
		t.Logf("成功接收 %d 字节消息", len(receivedData))
	case <-time.After(10 * time.Second):
		t.Fatal("等待大消息接收超时")
	}
}

func TestSendSimple(t *testing.T) {
	h1, _ := libp2p.New()
	defer h1.Close()

	h2, _ := libp2p.New()
	defer h2.Close()

	rt1 := NewReliableTransport(h1, testProtocol, nil)
	defer rt1.Stop()

	// h2 也需要创建传输层来注册流处理器
	rt2 := NewReliableTransport(h2, testProtocol, nil)
	defer rt2.Stop()

	// 连接
	h2Info := peer.AddrInfo{ID: h2.ID(), Addrs: h2.Addrs()}
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	if err := h1.Connect(ctx, h2Info); err != nil {
		t.Fatalf("连接失败: %v", err)
	}

	// 发送简单消息
	testData := []byte("simple message")
	if err := rt1.SendSimple(ctx, h2.ID(), testData); err != nil {
		t.Fatalf("发送简单消息失败: %v", err)
	}
}

func TestMessageTooLarge(t *testing.T) {
	h, _ := libp2p.New()
	defer h.Close()

	cfg := &ReliableTransportConfig{
		MaxMessageSize: 100,
	}
	rt := NewReliableTransport(h, testProtocol, cfg)
	defer rt.Stop()

	largeData := make([]byte, 200)
	peerID, _ := peer.Decode("QmYyQSo1c1Ym7orWxLYvCrM2EmxFTANf8wXmmE7DWjhx5N")

	ctx := context.Background()
	err := rt.Send(ctx, peerID, largeData)

	if err == nil {
		t.Error("应该返回消息太大的错误")
	}
}

func TestSetHandler(t *testing.T) {
	h, _ := libp2p.New()
	defer h.Close()

	rt := NewReliableTransport(h, testProtocol, nil)
	defer rt.Stop()

	handler := func(peerID peer.ID, data []byte) error {
		return nil
	}

	rt.SetHandler(testProtocol, handler)

	rt.handlerMu.RLock()
	_, exists := rt.handlers[testProtocol]
	rt.handlerMu.RUnlock()

	if !exists {
		t.Error("处理器应该被注册")
	}
}

func BenchmarkSplitIntoChunks(b *testing.B) {
	h, _ := libp2p.New()
	defer h.Close()

	rt := NewReliableTransport(h, testProtocol, nil)
	defer rt.Stop()

	data := make([]byte, 1024*1024) // 1MB

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		rt.splitIntoChunks(data)
	}
}

func BenchmarkCreateMessage(b *testing.B) {
	h, _ := libp2p.New()
	defer h.Close()

	rt := NewReliableTransport(h, testProtocol, nil)
	defer rt.Stop()

	data := make([]byte, 1024)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		rt.createMessage(MsgTypeData, 1, 1, 0, data)
	}
}

func BenchmarkVerifyChecksum(b *testing.B) {
	h, _ := libp2p.New()
	defer h.Close()

	rt := NewReliableTransport(h, testProtocol, nil)
	defer rt.Stop()

	data := make([]byte, 1024)
	msg := rt.createMessage(MsgTypeData, 1, 1, 0, data)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		rt.verifyChecksum(msg)
	}
}
