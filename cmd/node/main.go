package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"time"

	"github.com/AgentNetworkPlan/AgentNetwork/internal/api/server"
	"github.com/AgentNetworkPlan/AgentNetwork/internal/daemon"
	"github.com/AgentNetworkPlan/AgentNetwork/internal/p2p/host"
	"github.com/AgentNetworkPlan/AgentNetwork/internal/p2p/node"
)

var (
	version   = "0.1.0"
	buildTime = "unknown"
)

func main() {
	// 如果没有参数，显示帮助
	if len(os.Args) < 2 {
		printUsage()
		os.Exit(0)
	}

	// 解析命令
	cmd := os.Args[1]
	switch cmd {
	case "start", "daemon":
		cmdStart()
	case "stop":
		cmdStop()
	case "restart":
		cmdRestart()
	case "status":
		cmdStatus()
	case "logs":
		cmdLogs()
	case "run":
		cmdRun()
	case "version", "-v", "--version":
		cmdVersion()
	case "help", "-h", "--help":
		printUsage()
	default:
		// 兼容旧的直接运行方式
		if strings.HasPrefix(cmd, "-") {
			// 旧参数方式，直接运行
			os.Args = append([]string{os.Args[0], "run"}, os.Args[1:]...)
			cmdRun()
		} else {
			fmt.Fprintf(os.Stderr, "未知命令: %s\n", cmd)
			printUsage()
			os.Exit(1)
		}
	}
}

func printUsage() {
	fmt.Printf(`DAAN P2P Node v%s

用法:
  agentnetwork <命令> [选项]

命令:
  start       启动节点（后台运行）
  stop        停止节点
  restart     重启节点
  status      查看节点状态
  logs        查看节点日志
  run         前台运行节点（调试用）
  version     显示版本信息
  help        显示帮助信息

示例:
  agentnetwork start                           # 启动节点
  agentnetwork start -data ./mydata            # 指定数据目录启动
  agentnetwork start -listen /ip4/0.0.0.0/tcp/9000  # 指定监听地址
  agentnetwork stop                            # 停止节点
  agentnetwork status                          # 查看状态
  agentnetwork logs -n 100                     # 查看最后100行日志
  agentnetwork logs -f                         # 实时查看日志
  agentnetwork run                             # 前台运行（调试）

运行 'agentnetwork <命令> -h' 查看命令的详细选项
`, version)
}

// 公共参数
type commonFlags struct {
	dataDir        string
	keyPath        string
	listenAddrs    string
	bootstrapPeers string
	role           string
	grpcAddr       string
	httpAddr       string
}

func parseCommonFlags(fs *flag.FlagSet) *commonFlags {
	cf := &commonFlags{}
	fs.StringVar(&cf.dataDir, "data", "./data", "数据目录")
	fs.StringVar(&cf.keyPath, "key", "", "密钥文件路径（默认: <数据目录>/keys/node.key）")
	fs.StringVar(&cf.listenAddrs, "listen", "/ip4/0.0.0.0/tcp/0,/ip4/0.0.0.0/udp/0/quic-v1", "P2P监听地址（逗号分隔）")
	fs.StringVar(&cf.bootstrapPeers, "bootstrap", "", "引导节点地址（逗号分隔）")
	fs.StringVar(&cf.role, "role", "normal", "节点角色: bootstrap, relay, normal")
	fs.StringVar(&cf.grpcAddr, "grpc", ":50051", "gRPC服务地址")
	fs.StringVar(&cf.httpAddr, "http", ":18345", "HTTP服务地址")
	return cf
}

// ============ 命令实现 ============

func cmdStart() {
	fs := flag.NewFlagSet("start", flag.ExitOnError)
	cf := parseCommonFlags(fs)
	fs.Parse(os.Args[2:])

	// 创建守护进程管理器
	d := daemon.New(&daemon.Config{
		DataDir: cf.dataDir,
	})

	// 启动守护进程
	isDaemon, err := d.Start()
	if err != nil {
		fmt.Fprintf(os.Stderr, "错误: %v\n", err)
		os.Exit(1)
	}

	// 父进程在Start()中已经fork并退出
	// 子进程继续执行
	if isDaemon {
		runNode(cf, d)
	}
}

func cmdStop() {
	fs := flag.NewFlagSet("stop", flag.ExitOnError)
	dataDir := fs.String("data", "./data", "数据目录")
	fs.Parse(os.Args[2:])

	d := daemon.New(&daemon.Config{
		DataDir: *dataDir,
	})

	if err := d.Stop(); err != nil {
		fmt.Fprintf(os.Stderr, "错误: %v\n", err)
		os.Exit(1)
	}
}

func cmdRestart() {
	fs := flag.NewFlagSet("restart", flag.ExitOnError)
	cf := parseCommonFlags(fs)
	fs.Parse(os.Args[2:])

	d := daemon.New(&daemon.Config{
		DataDir: cf.dataDir,
	})

	if err := d.Restart(); err != nil {
		fmt.Fprintf(os.Stderr, "错误: %v\n", err)
		os.Exit(1)
	}
}

func cmdStatus() {
	fs := flag.NewFlagSet("status", flag.ExitOnError)
	dataDir := fs.String("data", "./data", "数据目录")
	jsonOutput := fs.Bool("json", false, "JSON格式输出")
	fs.Parse(os.Args[2:])

	d := daemon.New(&daemon.Config{
		DataDir: *dataDir,
	})

	status := d.Status()

	if *jsonOutput {
		data, _ := json.MarshalIndent(status, "", "  ")
		fmt.Println(string(data))
		return
	}

	// 格式化输出
	fmt.Println("======== 节点状态 ========")
	if status.Running {
		fmt.Printf("状态:     \033[32m运行中\033[0m\n")
		fmt.Printf("PID:      %d\n", status.PID)
	} else {
		fmt.Printf("状态:     \033[31m已停止\033[0m\n")
	}

	if status.NodeID != "" {
		fmt.Printf("节点ID:   %s\n", status.NodeID)
	}
	if status.Version != "" {
		fmt.Printf("版本:     %s\n", status.Version)
	}
	if status.Uptime != "" {
		fmt.Printf("运行时间: %s\n", status.Uptime)
	}
	if len(status.ListenAddrs) > 0 {
		fmt.Printf("监听地址:\n")
		for _, addr := range status.ListenAddrs {
			fmt.Printf("  - %s\n", addr)
		}
	}
	if status.PeerCount > 0 {
		fmt.Printf("连接节点: %d\n", status.PeerCount)
	}
	fmt.Printf("数据目录: %s\n", status.DataDir)
	fmt.Printf("日志文件: %s\n", status.LogFile)
	fmt.Println("==========================")
}

func cmdLogs() {
	fs := flag.NewFlagSet("logs", flag.ExitOnError)
	dataDir := fs.String("data", "./data", "数据目录")
	lines := fs.Int("n", 50, "显示行数")
	follow := fs.Bool("f", false, "实时跟踪")
	fs.Parse(os.Args[2:])

	d := daemon.New(&daemon.Config{
		DataDir: *dataDir,
	})

	if err := d.Logs(*lines, *follow); err != nil {
		fmt.Fprintf(os.Stderr, "错误: %v\n", err)
		os.Exit(1)
	}
}

func cmdRun() {
	fs := flag.NewFlagSet("run", flag.ExitOnError)
	cf := parseCommonFlags(fs)
	fs.Parse(os.Args[2:])

	d := daemon.New(&daemon.Config{
		DataDir: cf.dataDir,
	})

	// 检查是否是守护进程子进程
	if daemon.IsDaemonProcess() {
		// 写入PID
		d.Start()
	}

	runNode(cf, d)
}

func cmdVersion() {
	fmt.Printf("DAAN P2P Node\n")
	fmt.Printf("  版本:     %s\n", version)
	fmt.Printf("  构建时间: %s\n", buildTime)
}

// ============ 节点运行 ============

func runNode(cf *commonFlags, d *daemon.Daemon) {
	startTime := time.Now()

	// 设置默认密钥路径
	keyPath := cf.keyPath
	if keyPath == "" {
		keyPath = cf.dataDir + "/keys/node.key"
	}

	// 解析监听地址
	var addrs []string
	if cf.listenAddrs != "" {
		addrs = strings.Split(cf.listenAddrs, ",")
	}

	// 解析引导节点
	var peers []string
	if cf.bootstrapPeers != "" {
		peers = strings.Split(cf.bootstrapPeers, ",")
	}

	// 解析角色
	var nodeRole host.NodeRole
	switch cf.role {
	case "bootstrap":
		nodeRole = host.RoleBootstrap
	case "relay":
		nodeRole = host.RoleRelay
	default:
		nodeRole = host.RoleNormal
	}

	// 创建节点配置
	cfg := &node.Config{
		KeyPath:        keyPath,
		ListenAddrs:    addrs,
		BootstrapPeers: peers,
		Role:           nodeRole,
		EnableRelay:    true,
		EnableDHT:      true,
	}

	// 创建节点
	fmt.Println("正在创建节点...")
	n, err := node.New(cfg)
	if err != nil {
		fmt.Fprintf(os.Stderr, "创建节点失败: %v\n", err)
		os.Exit(1)
	}

	// 启动节点
	fmt.Println("正在启动节点...")
	if err := n.Start(); err != nil {
		fmt.Fprintf(os.Stderr, "启动节点失败: %v\n", err)
		os.Exit(1)
	}

	// 启动 gRPC 服务
	grpcServer := server.NewServer(n, cf.grpcAddr)
	if err := grpcServer.Start(); err != nil {
		fmt.Fprintf(os.Stderr, "启动 gRPC 服务失败: %v\n", err)
	}

	// 获取节点信息
	nodeID := n.Host().ID().String()
	listenAddrs := make([]string, 0)
	for _, addr := range n.Host().Addrs() {
		listenAddrs = append(listenAddrs, addr.String()+"/p2p/"+nodeID)
	}

	// 写入状态
	status := &daemon.NodeStatus{
		Running:     true,
		PID:         os.Getpid(),
		StartTime:   startTime,
		NodeID:      nodeID,
		Version:     version,
		ListenAddrs: listenAddrs,
	}
	d.WriteStatus(status)

	fmt.Printf("节点已启动\n")
	fmt.Printf("  节点ID: %s\n", nodeID)
	fmt.Printf("  监听地址:\n")
	for _, addr := range listenAddrs {
		fmt.Printf("    - %s\n", addr)
	}

	// 定期更新状态
	go func() {
		ticker := time.NewTicker(10 * time.Second)
		defer ticker.Stop()
		for {
			select {
			case <-ticker.C:
				status.PeerCount = n.Host().ConnectedPeers()
				status.Uptime = time.Since(startTime).Round(time.Second).String()
				d.WriteStatus(status)

				// 轮转日志
				d.RotateLogs()
			}
		}
	}()

	// 等待停止信号
	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)

	if !daemon.IsDaemonProcess() {
		fmt.Println("\n按 Ctrl+C 停止节点...")
	}

	<-sigCh

	fmt.Println("\n正在停止节点...")

	// 清理
	d.Cleanup()

	// 停止服务
	grpcServer.Stop()
	n.Stop()

	fmt.Println("节点已停止")
}
