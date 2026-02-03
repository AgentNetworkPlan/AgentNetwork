package heartbeat

import (
	"context"
	"encoding/json"
	"fmt"
	"sync"
	"time"

	"github.com/AgentNetworkPlan/AgentNetwork/internal/config"
	"github.com/AgentNetworkPlan/AgentNetwork/internal/crypto"
)

// Status Agent 状态
type Status string

const (
	StatusIdle    Status = "idle"
	StatusWorking Status = "working"
	StatusBlocked Status = "blocked"
)

// Packet 心跳包
type Packet struct {
	Version       string        `json:"version"`
	Type          string        `json:"type"`
	AgentID       string        `json:"agent_id"`
	Timestamp     string        `json:"timestamp"`
	Status        Status        `json:"status"`
	CurrentTask   *string       `json:"current_task"`
	Contributions Contributions `json:"contributions"`
	ProtocolHash  string        `json:"protocol_hash"`
	Signature     string        `json:"signature"`
}

// Contributions 贡献数据
type Contributions struct {
	PRsMerged    int `json:"prs_merged"`
	PRsReviewed  int `json:"prs_reviewed"`
	IssuesClosed int `json:"issues_closed"`
	Discussions  int `json:"discussions"`
}

// Service 心跳服务
type Service struct {
	config *config.Config
	signer crypto.Signer
	status Status
	task   *string
	mu     sync.RWMutex
}

// NewService 创建心跳服务
func NewService(cfg *config.Config, signer crypto.Signer) *Service {
	return &Service{
		config: cfg,
		signer: signer,
		status: StatusIdle,
	}
}

// Start 启动心跳服务
func (s *Service) Start(ctx context.Context) {
	// 立即发送一次心跳
	if err := s.Send(); err != nil {
		fmt.Printf("发送心跳失败: %v\n", err)
	}

	// 设置定时器，每天发送一次心跳
	ticker := time.NewTicker(24 * time.Hour)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			if err := s.Send(); err != nil {
				fmt.Printf("发送心跳失败: %v\n", err)
			}
		}
	}
}

// Send 发送心跳包
func (s *Service) Send() error {
	packet := s.createPacket()

	// 序列化
	data, err := json.Marshal(packet)
	if err != nil {
		return fmt.Errorf("序列化心跳包失败: %w", err)
	}

	// 签名
	signature, err := s.signer.Sign(data)
	if err != nil {
		return fmt.Errorf("签名失败: %w", err)
	}
	packet.Signature = fmt.Sprintf("%x", signature)

	// 再次序列化（包含签名）
	data, err = json.MarshalIndent(packet, "", "  ")
	if err != nil {
		return fmt.Errorf("序列化失败: %w", err)
	}

	fmt.Printf("心跳包:\n%s\n", string(data))

	// TODO: 广播心跳包到网络
	// TODO: 保存到 heartbeats/ 目录

	return nil
}

// createPacket 创建心跳包
func (s *Service) createPacket() *Packet {
	s.mu.RLock()
	defer s.mu.RUnlock()

	return &Packet{
		Version:     s.config.Version,
		Type:        "heartbeat",
		AgentID:     s.config.AgentID,
		Timestamp:   time.Now().UTC().Format(time.RFC3339),
		Status:      s.status,
		CurrentTask: s.task,
		Contributions: Contributions{
			PRsMerged:    0,
			PRsReviewed:  0,
			IssuesClosed: 0,
			Discussions:  0,
		},
		ProtocolHash: "", // TODO: 计算 SKILL.md 的 SHA256
	}
}

// SetStatus 设置状态
func (s *Service) SetStatus(status Status) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.status = status
}

// SetTask 设置当前任务
func (s *Service) SetTask(task *string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.task = task
}

// GetStatus 获取状态
func (s *Service) GetStatus() string {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return string(s.status)
}
