package agent

import (
	"context"
	"fmt"
	"os"
	"os/signal"
	"syscall"

	"github.com/AgentNetworkPlan/AgentNetwork/internal/config"
	"github.com/AgentNetworkPlan/AgentNetwork/internal/crypto"
	"github.com/AgentNetworkPlan/AgentNetwork/internal/heartbeat"
	"github.com/AgentNetworkPlan/AgentNetwork/internal/protocol"
)

// Agent DAAN 网络 Agent
type Agent struct {
	config    *config.Config
	signer    crypto.Signer
	heartbeat *heartbeat.Service
	protocol  *protocol.Handler
}

// New 创建新的 Agent 实例
func New(cfg *config.Config) (*Agent, error) {
	// 初始化签名器
	signer, err := crypto.NewSigner(cfg.KeyAlgorithm, cfg.PrivateKeyPath)
	if err != nil {
		return nil, fmt.Errorf("初始化签名器失败: %w", err)
	}

	// 计算 Agent ID (公钥哈希)
	agentID, err := signer.GetAgentID()
	if err != nil {
		return nil, fmt.Errorf("获取 Agent ID 失败: %w", err)
	}
	cfg.AgentID = agentID

	// 初始化心跳服务
	hbService := heartbeat.NewService(cfg, signer)

	// 初始化协议处理器
	protoHandler := protocol.NewHandler(cfg, signer)

	return &Agent{
		config:    cfg,
		signer:    signer,
		heartbeat: hbService,
		protocol:  protoHandler,
	}, nil
}

// Run 启动 Agent
func (a *Agent) Run() error {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// 捕获系统信号
	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)

	fmt.Printf("Agent ID: %s\n", a.config.AgentID)
	fmt.Printf("版本: %s\n", a.config.Version)
	fmt.Printf("监听地址: %s\n", a.config.Network.ListenAddr)
	fmt.Println("Agent 已启动，按 Ctrl+C 退出...")

	// 启动心跳服务
	go a.heartbeat.Start(ctx)

	// 启动协议处理
	go a.protocol.Start(ctx)

	// 等待退出信号
	<-sigCh
	fmt.Println("\n正在关闭 Agent...")
	cancel()

	return nil
}

// SendHeartbeat 发送心跳包
func (a *Agent) SendHeartbeat() error {
	return a.heartbeat.Send()
}

// GetStatus 获取 Agent 状态
func (a *Agent) GetStatus() string {
	return a.heartbeat.GetStatus()
}
