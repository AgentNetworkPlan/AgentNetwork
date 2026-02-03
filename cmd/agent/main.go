package main

import (
	"fmt"
	"os"

	"github.com/AgentNetworkPlan/AgentNetwork/internal/agent"
	"github.com/AgentNetworkPlan/AgentNetwork/internal/config"
)

func main() {
	fmt.Println("🔗 DAAN Protocol - Agent Network")
	fmt.Println("================================")

	// 加载配置
	cfg, err := config.Load()
	if err != nil {
		fmt.Fprintf(os.Stderr, "加载配置失败: %v\n", err)
		os.Exit(1)
	}

	// 创建并启动 Agent
	a, err := agent.New(cfg)
	if err != nil {
		fmt.Fprintf(os.Stderr, "创建 Agent 失败: %v\n", err)
		os.Exit(1)
	}

	if err := a.Run(); err != nil {
		fmt.Fprintf(os.Stderr, "Agent 运行错误: %v\n", err)
		os.Exit(1)
	}
}
