package network

import (
	"sync"
	"testing"
	"time"

	"github.com/libp2p/go-libp2p"
	"github.com/libp2p/go-libp2p/core/peer"
)

func TestNewBroadcaster(t *testing.T) {
	h, err := libp2p.New()
	if err != nil {
		t.Fatalf("创建主机失败: %v", err)
	}
	defer h.Close()

	b, err := NewBroadcaster(h)
	if err != nil {
		t.Fatalf("创建广播器失败: %v", err)
	}

	b.Stop()
}

func TestBroadcasterSubscribe(t *testing.T) {
	h, _ := libp2p.New()
	defer h.Close()

	b, err := NewBroadcaster(h)
	if err != nil {
		t.Fatalf("创建广播器失败: %v", err)
	}
	defer b.Stop()

	// 订阅
	err = b.Subscribe("test-topic", func(msg *BroadcastMessage) {
		t.Logf("收到消息: %s", msg.Payload)
	})
	if err != nil {
		t.Fatalf("订阅失败: %v", err)
	}

	// 检查已订阅的主题
	topics := b.GetSubscribedTopics()
	if len(topics) != 1 {
		t.Errorf("应该有 1 个订阅，实际有 %d", len(topics))
	}
	if topics[0] != "test-topic" {
		t.Errorf("主题名不匹配: %s", topics[0])
	}
}

func TestBroadcasterUnsubscribe(t *testing.T) {
	h, _ := libp2p.New()
	defer h.Close()

	b, err := NewBroadcaster(h)
	if err != nil {
		t.Fatalf("创建广播器失败: %v", err)
	}
	defer b.Stop()

	// 订阅
	b.Subscribe("test-topic", func(msg *BroadcastMessage) {})

	// 取消订阅
	err = b.Unsubscribe("test-topic")
	if err != nil {
		t.Fatalf("取消订阅失败: %v", err)
	}

	// 检查
	topics := b.GetSubscribedTopics()
	if len(topics) != 0 {
		t.Errorf("应该没有订阅，实际有 %d", len(topics))
	}
}

func TestBroadcasterDoubleSubscribe(t *testing.T) {
	h, _ := libp2p.New()
	defer h.Close()

	b, err := NewBroadcaster(h)
	if err != nil {
		t.Fatalf("创建广播器失败: %v", err)
	}
	defer b.Stop()

	// 第一次订阅
	err = b.Subscribe("test-topic", func(msg *BroadcastMessage) {})
	if err != nil {
		t.Fatalf("第一次订阅失败: %v", err)
	}

	// 第二次订阅应该失败
	err = b.Subscribe("test-topic", func(msg *BroadcastMessage) {})
	if err == nil {
		t.Error("重复订阅应该失败")
	}
}

func TestBroadcasterUnsubscribeNonExistent(t *testing.T) {
	h, _ := libp2p.New()
	defer h.Close()

	b, err := NewBroadcaster(h)
	if err != nil {
		t.Fatalf("创建广播器失败: %v", err)
	}
	defer b.Stop()

	// 取消不存在的订阅
	err = b.Unsubscribe("non-existent")
	if err == nil {
		t.Error("取消不存在的订阅应该失败")
	}
}

func TestBroadcasterBroadcast(t *testing.T) {
	h, _ := libp2p.New()
	defer h.Close()

	b, err := NewBroadcaster(h)
	if err != nil {
		t.Fatalf("创建广播器失败: %v", err)
	}
	defer b.Stop()

	// 广播消息（即使没有订阅者）
	err = b.Broadcast("test-topic", []byte("Hello"))
	if err != nil {
		t.Fatalf("广播失败: %v", err)
	}

	// 检查已加入的主题
	topics := b.GetJoinedTopics()
	if len(topics) != 1 {
		t.Errorf("应该有 1 个主题，实际有 %d", len(topics))
	}
}

func TestBroadcasterTwoNodes(t *testing.T) {
	// 创建两个主机
	h1, _ := libp2p.New()
	defer h1.Close()

	h2, _ := libp2p.New()
	defer h2.Close()

	b1, err := NewBroadcaster(h1)
	if err != nil {
		t.Fatalf("创建广播器1失败: %v", err)
	}
	defer b1.Stop()

	b2, err := NewBroadcaster(h2)
	if err != nil {
		t.Fatalf("创建广播器2失败: %v", err)
	}
	defer b2.Stop()

	// 设置接收处理器
	var receivedMsg *BroadcastMessage
	var wg sync.WaitGroup
	wg.Add(1)

	err = b2.Subscribe("test-topic", func(msg *BroadcastMessage) {
		receivedMsg = msg
		wg.Done()
	})
	if err != nil {
		t.Fatalf("订阅失败: %v", err)
	}

	// 连接两个主机
	h1.Peerstore().AddAddrs(h2.ID(), h2.Addrs(), time.Hour)
	peerInfo := peer.AddrInfo{ID: h2.ID(), Addrs: h2.Addrs()}
	if err := h1.Connect(b1.ctx, peerInfo); err != nil {
		t.Fatalf("连接失败: %v", err)
	}

	// 等待 PubSub 同步
	time.Sleep(500 * time.Millisecond)

	// 广播消息
	testPayload := []byte("Hello from h1!")
	err = b1.Broadcast("test-topic", testPayload)
	if err != nil {
		t.Fatalf("广播失败: %v", err)
	}

	// 等待接收
	done := make(chan struct{})
	go func() {
		wg.Wait()
		close(done)
	}()

	select {
	case <-done:
		if receivedMsg == nil {
			t.Fatal("没有收到消息")
		}
		if string(receivedMsg.Payload) != string(testPayload) {
			t.Errorf("消息不匹配: got %s, want %s", receivedMsg.Payload, testPayload)
		}
		if receivedMsg.Sender != h1.ID().String() {
			t.Errorf("发送者不匹配: got %s, want %s", receivedMsg.Sender, h1.ID())
		}
		if receivedMsg.Topic != "test-topic" {
			t.Errorf("主题不匹配: got %s", receivedMsg.Topic)
		}
	case <-time.After(5 * time.Second):
		t.Fatal("等待消息超时")
	}
}

func TestBroadcasterJSON(t *testing.T) {
	h, _ := libp2p.New()
	defer h.Close()

	b, err := NewBroadcaster(h)
	if err != nil {
		t.Fatalf("创建广播器失败: %v", err)
	}
	defer b.Stop()

	type TestData struct {
		Name  string `json:"name"`
		Value int    `json:"value"`
	}

	data := TestData{Name: "test", Value: 42}
	err = b.BroadcastJSON("test-topic", data)
	if err != nil {
		t.Fatalf("广播 JSON 失败: %v", err)
	}
}

func TestPredefinedTopics(t *testing.T) {
	if TopicTask != "/daan/task" {
		t.Error("TopicTask 值错误")
	}
	if TopicReputation != "/daan/reputation" {
		t.Error("TopicReputation 值错误")
	}
	if TopicAnnounce != "/daan/announce" {
		t.Error("TopicAnnounce 值错误")
	}
	if TopicHeartbeat != "/daan/heartbeat" {
		t.Error("TopicHeartbeat 值错误")
	}
}

func TestBroadcasterPredefinedMethods(t *testing.T) {
	h, _ := libp2p.New()
	defer h.Close()

	b, err := NewBroadcaster(h)
	if err != nil {
		t.Fatalf("创建广播器失败: %v", err)
	}
	defer b.Stop()

	// 测试预定义广播方法
	err = b.BroadcastTask([]byte("task"))
	if err != nil {
		t.Errorf("BroadcastTask 失败: %v", err)
	}

	err = b.BroadcastReputation([]byte("reputation"))
	if err != nil {
		t.Errorf("BroadcastReputation 失败: %v", err)
	}

	err = b.BroadcastAnnounce([]byte("announce"))
	if err != nil {
		t.Errorf("BroadcastAnnounce 失败: %v", err)
	}

	err = b.BroadcastHeartbeat([]byte("heartbeat"))
	if err != nil {
		t.Errorf("BroadcastHeartbeat 失败: %v", err)
	}

	// 检查已加入的主题
	topics := b.GetJoinedTopics()
	if len(topics) != 4 {
		t.Errorf("应该有 4 个主题，实际有 %d", len(topics))
	}
}

func TestBroadcasterPredefinedSubscribe(t *testing.T) {
	h, _ := libp2p.New()
	defer h.Close()

	b, err := NewBroadcaster(h)
	if err != nil {
		t.Fatalf("创建广播器失败: %v", err)
	}
	defer b.Stop()

	handler := func(msg *BroadcastMessage) {}

	// 测试预定义订阅方法
	if err := b.SubscribeTask(handler); err != nil {
		t.Errorf("SubscribeTask 失败: %v", err)
	}
	if err := b.SubscribeReputation(handler); err != nil {
		t.Errorf("SubscribeReputation 失败: %v", err)
	}
	if err := b.SubscribeAnnounce(handler); err != nil {
		t.Errorf("SubscribeAnnounce 失败: %v", err)
	}
	if err := b.SubscribeHeartbeat(handler); err != nil {
		t.Errorf("SubscribeHeartbeat 失败: %v", err)
	}

	// 检查已订阅的主题
	topics := b.GetSubscribedTopics()
	if len(topics) != 4 {
		t.Errorf("应该有 4 个订阅，实际有 %d", len(topics))
	}
}

func TestGetTopicPeers(t *testing.T) {
	h, _ := libp2p.New()
	defer h.Close()

	b, err := NewBroadcaster(h)
	if err != nil {
		t.Fatalf("创建广播器失败: %v", err)
	}
	defer b.Stop()

	// 未加入主题
	peers := b.GetTopicPeers("non-existent")
	if peers != nil {
		t.Error("未加入的主题应该返回 nil")
	}

	// 加入主题
	b.Subscribe("test-topic", func(msg *BroadcastMessage) {})

	peers = b.GetTopicPeers("test-topic")
	// 单节点，没有其他 peer
	if len(peers) != 0 {
		t.Errorf("单节点应该没有 peer，实际有 %d", len(peers))
	}
}
