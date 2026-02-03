package main

import (
	"flag"
	"fmt"
	"os"
	"os/signal"
	"strings"
	"syscall"

	"github.com/AgentNetworkPlan/AgentNetwork/internal/api/server"
	"github.com/AgentNetworkPlan/AgentNetwork/internal/p2p/host"
	"github.com/AgentNetworkPlan/AgentNetwork/internal/p2p/node"
)

var (
	version = "0.1.0"
)

func main() {
	// 命令行参数
	keyPath := flag.String("key", "keys/node.key", "密钥文件路径")
	listenAddrs := flag.String("listen", "/ip4/0.0.0.0/tcp/0,/ip4/0.0.0.0/udp/0/quic-v1", "监听地址（逗号分隔）")
	bootstrapPeers := flag.String("bootstrap", "", "引导节点地址（逗号分隔）")
	role := flag.String("role", "normal", "节点角色: bootstrap, relay, normal")
	grpcAddr := flag.String("grpc", ":50051", "gRPC 服务地址")
	showVersion := flag.Bool("version", false, "显示版本")

	flag.Parse()

	if *showVersion {
		fmt.Printf("DAAN P2P Node v%s\n", version)
		os.Exit(0)
	}

	// 解析监听地址
	var addrs []string
	if *listenAddrs != "" {
		addrs = strings.Split(*listenAddrs, ",")
	}

	// 解析引导节点
	var peers []string
	if *bootstrapPeers != "" {
		peers = strings.Split(*bootstrapPeers, ",")
	}

	// 解析角色
	var nodeRole host.NodeRole
	switch *role {
	case "bootstrap":
		nodeRole = host.RoleBootstrap
	case "relay":
		nodeRole = host.RoleRelay
	default:
		nodeRole = host.RoleNormal
	}

	// 创建节点配置
	cfg := &node.Config{
		KeyPath:        *keyPath,
		ListenAddrs:    addrs,
		BootstrapPeers: peers,
		Role:           nodeRole,
		EnableRelay:    true,
		EnableDHT:      true,
	}

	// 创建节点
	n, err := node.New(cfg)
	if err != nil {
		fmt.Fprintf(os.Stderr, "创建节点失败: %v\n", err)
		os.Exit(1)
	}

	// 启动节点
	if err := n.Start(); err != nil {
		fmt.Fprintf(os.Stderr, "启动节点失败: %v\n", err)
		os.Exit(1)
	}

	// 启动 gRPC 服务
	grpcServer := server.NewServer(n, *grpcAddr)
	if err := grpcServer.Start(); err != nil {
		fmt.Fprintf(os.Stderr, "启动 gRPC 服务失败: %v\n", err)
	}

	// 等待停止信号
	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)

	fmt.Println("按 Ctrl+C 停止节点...")
	<-sigCh

	fmt.Println("\n正在停止节点...")
	grpcServer.Stop()
	n.Stop()
}
