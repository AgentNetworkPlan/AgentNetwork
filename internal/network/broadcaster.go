package network

import (
	"context"
	"encoding/json"
	"fmt"
	"sync"
	"time"

	pubsub "github.com/libp2p/go-libp2p-pubsub"
	"github.com/libp2p/go-libp2p/core/host"
	"github.com/libp2p/go-libp2p/core/peer"
)

// BroadcastMessage 广播消息
type BroadcastMessage struct {
	Topic     string    `json:"topic"`
	Sender    string    `json:"sender"`
	Timestamp time.Time `json:"timestamp"`
	Payload   []byte    `json:"payload"`
}

// TopicHandler 主题消息处理器
type TopicHandler func(msg *BroadcastMessage)

// Subscription 订阅
type Subscription struct {
	topic   *pubsub.Topic
	sub     *pubsub.Subscription
	handler TopicHandler
	cancel  context.CancelFunc
}

// Broadcaster 广播器
type Broadcaster struct {
	host   host.Host
	pubsub *pubsub.PubSub

	topics map[string]*pubsub.Topic
	subs   map[string]*Subscription
	mu     sync.RWMutex

	ctx    context.Context
	cancel context.CancelFunc
}

// NewBroadcaster 创建广播器
func NewBroadcaster(h host.Host) (*Broadcaster, error) {
	ctx, cancel := context.WithCancel(context.Background())

	// 创建 GossipSub
	ps, err := pubsub.NewGossipSub(ctx, h)
	if err != nil {
		cancel()
		return nil, fmt.Errorf("创建 PubSub 失败: %w", err)
	}

	return &Broadcaster{
		host:   h,
		pubsub: ps,
		topics: make(map[string]*pubsub.Topic),
		subs:   make(map[string]*Subscription),
		ctx:    ctx,
		cancel: cancel,
	}, nil
}

// Broadcast 广播消息到指定主题
func (b *Broadcaster) Broadcast(topicName string, payload []byte) error {
	topic, err := b.getOrJoinTopic(topicName)
	if err != nil {
		return err
	}

	msg := &BroadcastMessage{
		Topic:     topicName,
		Sender:    b.host.ID().String(),
		Timestamp: time.Now(),
		Payload:   payload,
	}

	data, err := json.Marshal(msg)
	if err != nil {
		return fmt.Errorf("序列化消息失败: %w", err)
	}

	return topic.Publish(b.ctx, data)
}

// BroadcastJSON 广播 JSON 消息
func (b *Broadcaster) BroadcastJSON(topicName string, v interface{}) error {
	payload, err := json.Marshal(v)
	if err != nil {
		return fmt.Errorf("序列化失败: %w", err)
	}
	return b.Broadcast(topicName, payload)
}

// Subscribe 订阅主题
func (b *Broadcaster) Subscribe(topicName string, handler TopicHandler) error {
	b.mu.Lock()
	defer b.mu.Unlock()

	// 检查是否已订阅
	if _, exists := b.subs[topicName]; exists {
		return fmt.Errorf("已订阅主题 %s", topicName)
	}

	topic, err := b.getOrJoinTopicLocked(topicName)
	if err != nil {
		return err
	}

	sub, err := topic.Subscribe()
	if err != nil {
		return fmt.Errorf("订阅主题失败: %w", err)
	}

	ctx, cancel := context.WithCancel(b.ctx)

	subscription := &Subscription{
		topic:   topic,
		sub:     sub,
		handler: handler,
		cancel:  cancel,
	}

	b.subs[topicName] = subscription

	// 启动消息接收协程
	go b.receiveMessages(ctx, topicName, sub, handler)

	return nil
}

// Unsubscribe 取消订阅
func (b *Broadcaster) Unsubscribe(topicName string) error {
	b.mu.Lock()
	defer b.mu.Unlock()

	sub, exists := b.subs[topicName]
	if !exists {
		return fmt.Errorf("未订阅主题 %s", topicName)
	}

	sub.cancel()
	sub.sub.Cancel()
	delete(b.subs, topicName)

	return nil
}

// receiveMessages 接收消息
func (b *Broadcaster) receiveMessages(ctx context.Context, topicName string, sub *pubsub.Subscription, handler TopicHandler) {
	for {
		msg, err := sub.Next(ctx)
		if err != nil {
			if ctx.Err() != nil {
				return // 上下文取消
			}
			continue
		}

		// 忽略自己的消息
		if msg.ReceivedFrom == b.host.ID() {
			continue
		}

		var broadcastMsg BroadcastMessage
		if err := json.Unmarshal(msg.Data, &broadcastMsg); err != nil {
			continue
		}

		// 调用处理器
		if handler != nil {
			handler(&broadcastMsg)
		}
	}
}

// getOrJoinTopic 获取或加入主题
func (b *Broadcaster) getOrJoinTopic(topicName string) (*pubsub.Topic, error) {
	b.mu.Lock()
	defer b.mu.Unlock()
	return b.getOrJoinTopicLocked(topicName)
}

// getOrJoinTopicLocked 获取或加入主题（已持有锁）
func (b *Broadcaster) getOrJoinTopicLocked(topicName string) (*pubsub.Topic, error) {
	if topic, exists := b.topics[topicName]; exists {
		return topic, nil
	}

	topic, err := b.pubsub.Join(topicName)
	if err != nil {
		return nil, fmt.Errorf("加入主题失败: %w", err)
	}

	b.topics[topicName] = topic
	return topic, nil
}

// GetTopicPeers 获取主题的订阅者
func (b *Broadcaster) GetTopicPeers(topicName string) []peer.ID {
	b.mu.RLock()
	topic, exists := b.topics[topicName]
	b.mu.RUnlock()

	if !exists {
		return nil
	}

	return topic.ListPeers()
}

// GetSubscribedTopics 获取已订阅的主题列表
func (b *Broadcaster) GetSubscribedTopics() []string {
	b.mu.RLock()
	defer b.mu.RUnlock()

	topics := make([]string, 0, len(b.subs))
	for topic := range b.subs {
		topics = append(topics, topic)
	}
	return topics
}

// GetJoinedTopics 获取已加入的主题列表
func (b *Broadcaster) GetJoinedTopics() []string {
	b.mu.RLock()
	defer b.mu.RUnlock()

	topics := make([]string, 0, len(b.topics))
	for topic := range b.topics {
		topics = append(topics, topic)
	}
	return topics
}

// Stop 停止广播器
func (b *Broadcaster) Stop() {
	b.mu.Lock()
	defer b.mu.Unlock()

	// 取消所有订阅
	for _, sub := range b.subs {
		sub.cancel()
		sub.sub.Cancel()
	}

	// 关闭所有主题
	for _, topic := range b.topics {
		topic.Close()
	}

	b.cancel()
}

// ========== 预定义主题 ==========

const (
	// TopicTask 任务相关消息
	TopicTask = "/daan/task"
	// TopicReputation 声誉相关消息
	TopicReputation = "/daan/reputation"
	// TopicAnnounce 节点公告
	TopicAnnounce = "/daan/announce"
	// TopicHeartbeat 心跳消息
	TopicHeartbeat = "/daan/heartbeat"
)

// BroadcastTask 广播任务消息
func (b *Broadcaster) BroadcastTask(payload []byte) error {
	return b.Broadcast(TopicTask, payload)
}

// BroadcastReputation 广播声誉消息
func (b *Broadcaster) BroadcastReputation(payload []byte) error {
	return b.Broadcast(TopicReputation, payload)
}

// BroadcastAnnounce 广播节点公告
func (b *Broadcaster) BroadcastAnnounce(payload []byte) error {
	return b.Broadcast(TopicAnnounce, payload)
}

// BroadcastHeartbeat 广播心跳
func (b *Broadcaster) BroadcastHeartbeat(payload []byte) error {
	return b.Broadcast(TopicHeartbeat, payload)
}

// SubscribeTask 订阅任务消息
func (b *Broadcaster) SubscribeTask(handler TopicHandler) error {
	return b.Subscribe(TopicTask, handler)
}

// SubscribeReputation 订阅声誉消息
func (b *Broadcaster) SubscribeReputation(handler TopicHandler) error {
	return b.Subscribe(TopicReputation, handler)
}

// SubscribeAnnounce 订阅节点公告
func (b *Broadcaster) SubscribeAnnounce(handler TopicHandler) error {
	return b.Subscribe(TopicAnnounce, handler)
}

// SubscribeHeartbeat 订阅心跳消息
func (b *Broadcaster) SubscribeHeartbeat(handler TopicHandler) error {
	return b.Subscribe(TopicHeartbeat, handler)
}
