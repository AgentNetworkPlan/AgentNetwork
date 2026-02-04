package main

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"net/http"
	"os"
	"os/signal"
	"path/filepath"
	"strings"
	"syscall"
	"time"

	"github.com/AgentNetworkPlan/AgentNetwork/internal/api/server"
	"github.com/AgentNetworkPlan/AgentNetwork/internal/bulletin"
	"github.com/AgentNetworkPlan/AgentNetwork/internal/config"
	"github.com/AgentNetworkPlan/AgentNetwork/internal/daemon"
	"github.com/AgentNetworkPlan/AgentNetwork/internal/httpapi"
	"github.com/AgentNetworkPlan/AgentNetwork/internal/mailbox"
	"github.com/AgentNetworkPlan/AgentNetwork/internal/neighbor"
	"github.com/AgentNetworkPlan/AgentNetwork/internal/p2p/host"
	"github.com/AgentNetworkPlan/AgentNetwork/internal/p2p/identity"
	"github.com/AgentNetworkPlan/AgentNetwork/internal/p2p/node"
	"github.com/AgentNetworkPlan/AgentNetwork/internal/webadmin"
	"github.com/libp2p/go-libp2p/core/peer"
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
	case "token":
		cmdToken()
	case "config":
		cmdConfig()
	case "keygen":
		cmdKeygen()
	case "health":
		cmdHealth()
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
	fmt.Printf(`%s
DAAN P2P Node v%s

用法:
  agentnetwork <命令> [选项]

命令:
  start       启动节点（后台运行）
  stop        停止节点
  restart     重启节点
  status      查看节点状态
  logs        查看节点日志
  run         前台运行节点（调试用）
  
  token       管理访问令牌
  config      管理配置文件
  keygen      生成密钥对
  health      健康检查
  
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
  
  agentnetwork token show                      # 显示当前令牌
  agentnetwork token refresh                   # 刷新令牌
  agentnetwork config init                     # 初始化配置
  agentnetwork config show                     # 显示配置
  agentnetwork keygen                          # 生成新密钥
  agentnetwork health                          # 检查节点健康

运行 'agentnetwork <命令> -h' 查看命令的详细选项
`, getASCIILogo(), version)
}

// getASCIILogo returns the ASCII art logo for DAAN.
func getASCIILogo() string {
	return `
╔══════════════════════════════════════════════════════════╗
║     ____    _    _    _   _                              ║
║    |  _ \  / \  / \  | \ | |                             ║
║    | | | |/ _ \/ _ \ |  \| |                             ║
║    | |_| / ___ \ ___ \| |\  |                            ║
║    |____/_/   \_\   \_\_| \_|                            ║
║                                                          ║
║        Decentralized Agent Autonomous Network            ║
╚══════════════════════════════════════════════════════════╝
`
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
	adminAddr      string
	adminToken     string
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
	fs.StringVar(&cf.adminAddr, "admin", ":18080", "管理后台地址")
	fs.StringVar(&cf.adminToken, "admin-token", "", "管理后台访问令牌（可选，默认自动生成）")
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

	// 加载或生成 API Token（在创建 HTTP Server 之前）
	adminToken := cf.adminToken
	if adminToken == "" {
		// 从数据目录读取或生成新令牌
		adminToken = loadOrGenerateToken(cf.dataDir)
	}

	// 启动 HTTP API 服务
	httpConfig := httpapi.DefaultConfig(n.Host().ID().String())
	httpConfig.ListenAddr = cf.httpAddr
	httpConfig.APIToken = adminToken // 使用统一的 Token
	httpServer, err := httpapi.NewServer(httpConfig)
	if err != nil {
		fmt.Fprintf(os.Stderr, "创建 HTTP 服务失败: %v\n", err)
	} else {
		if err := httpServer.Start(); err != nil {
			fmt.Fprintf(os.Stderr, "启动 HTTP 服务失败: %v\n", err)
		} else {
			fmt.Printf("HTTP API 服务已启动: %s\n", cf.httpAddr)
		}
	}

	// 启动管理后台服务
	var adminServer *webadmin.Server

	nodeInfoProvider := webadmin.NewDefaultNodeInfoProvider()
	nodeInfoProvider.SetNodeInfo(n.Host().ID().String(), "", version)
	nodeInfoProvider.SetPorts(0, extractPort(cf.httpAddr), extractPort(cf.grpcAddr), extractPort(cf.adminAddr))
	nodeInfoProvider.SetRole(cf.role == "bootstrap", nodeRole == host.RoleRelay)
	nodeInfoProvider.SetPeersFunc(func() []string {
		peers := n.Host().Peers()
		peerList := make([]string, 0, len(peers))
		for _, p := range peers {
			peerList = append(peerList, p.String())
		}
		return peerList
	})

	adminConfig := &webadmin.Config{
		ListenAddr: cf.adminAddr,
		AdminToken: adminToken,
	}

	adminServer = webadmin.New(adminConfig, nodeInfoProvider)
	if err := adminServer.Start(); err != nil {
		fmt.Fprintf(os.Stderr, "启动管理后台失败: %v\n", err)
	} else {
		fmt.Printf("管理后台已启动: %s\n", adminServer.GetAdminURL())
	}

	// 初始化邻居管理器
	neighborConfig := neighbor.DefaultConfig()
	neighborManager := neighbor.NewNeighborManager(neighborConfig)
	neighborManager.SetPingFunc(func(nodeID string) error {
		peerID, err := peer.Decode(nodeID)
		if err != nil {
			return err
		}
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		_, err = n.Host().FindPeer(ctx, peerID)
		return err
	})
	neighborManager.Start()

	// 初始化邮箱
	nodeID := n.Host().ID().String()
	mailboxConfig := mailbox.DefaultConfig(nodeID)
	mailboxConfig.DataDir = filepath.Join(cf.dataDir, "mailbox")
	mb, err := mailbox.NewMailbox(mailboxConfig)
	if err != nil {
		fmt.Fprintf(os.Stderr, "创建邮箱失败: %v\n", err)
	} else {
		mb.Start()
	}

	// 初始化留言板
	bulletinConfig := bulletin.DefaultBulletinConfig(nodeID)
	bulletinConfig.DataDir = filepath.Join(cf.dataDir, "bulletin")
	bb, err := bulletin.NewBulletinBoard(bulletinConfig)
	if err != nil {
		fmt.Fprintf(os.Stderr, "创建留言板失败: %v\n", err)
	} else {
		bb.Start()
	}

	// 设置 OperationsProvider
	opsProvider := webadmin.NewRealOperationsProvider(nodeID)
	opsProvider.SetNeighborManager(neighborManager)
	if mb != nil {
		opsProvider.SetMailbox(mb)
	}
	if bb != nil {
		opsProvider.SetBulletinBoard(bb)
	}
	opsProvider.SetGetPeersFunc(func() []peer.ID {
		return n.Host().Peers()
	})
	opsProvider.SetConnectFunc(func(ctx context.Context, peerInfo peer.AddrInfo) error {
		return n.Host().Connect(ctx, peerInfo)
	})
	opsProvider.SetFindPeerFunc(func(ctx context.Context, id peer.ID) (peer.AddrInfo, error) {
		return n.Host().FindPeer(ctx, id)
	})
	adminServer.SetOperationsProvider(opsProvider)

	// 获取节点监听地址
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
	if adminServer != nil {
		adminServer.Stop()
	}
	if httpServer != nil {
		httpServer.Stop()
	}
	grpcServer.Stop()
	
	// 停止邻居、邮箱、留言板服务
	neighborManager.Stop()
	if mb != nil {
		mb.Stop()
	}
	if bb != nil {
		bb.Stop()
	}
	
	n.Stop()

	fmt.Println("节点已停止")
}

// ============ 新增命令实现 ============

func cmdToken() {
	if len(os.Args) < 3 {
		printTokenUsage()
		return
	}

	subCmd := os.Args[2]
	switch subCmd {
	case "show":
		fs := flag.NewFlagSet("token show", flag.ExitOnError)
		dataDir := fs.String("data", "./data", "数据目录")
		fs.Parse(os.Args[3:])

		token := loadOrGenerateToken(*dataDir)
		fmt.Println("======== 访问令牌 ========")
		fmt.Printf("令牌: %s\n", token)
		fmt.Printf("管理后台 URL: http://localhost:18080/?token=%s\n", token)
		fmt.Println("==========================")

	case "refresh":
		fs := flag.NewFlagSet("token refresh", flag.ExitOnError)
		dataDir := fs.String("data", "./data", "数据目录")
		fs.Parse(os.Args[3:])

		token := generateAndSaveToken(*dataDir)
		fmt.Println("======== 新令牌 ========")
		fmt.Printf("令牌: %s\n", token)
		fmt.Printf("管理后台 URL: http://localhost:18080/?token=%s\n", token)
		fmt.Println("========================")
		fmt.Println("⚠️  提示: 如果节点正在运行，请重启以应用新令牌")

	default:
		fmt.Fprintf(os.Stderr, "未知子命令: %s\n", subCmd)
		printTokenUsage()
		os.Exit(1)
	}
}

func printTokenUsage() {
	fmt.Print(`用法: agentnetwork token <子命令> [选项]

子命令:
  show      显示当前访问令牌
  refresh   刷新（重新生成）访问令牌

选项:
  -data     数据目录 (默认: ./data)

示例:
  agentnetwork token show
  agentnetwork token refresh -data ./mydata
`)
}

func cmdConfig() {
	if len(os.Args) < 3 {
		printConfigUsage()
		return
	}

	subCmd := os.Args[2]
	switch subCmd {
	case "init":
		fs := flag.NewFlagSet("config init", flag.ExitOnError)
		dataDir := fs.String("data", "./data", "数据目录")
		force := fs.Bool("force", false, "强制覆盖现有配置")
		fs.Parse(os.Args[3:])

		configPath := *dataDir + "/config.json"
		if _, err := os.Stat(configPath); err == nil && !*force {
			fmt.Fprintf(os.Stderr, "配置文件已存在: %s\n", configPath)
			fmt.Fprintln(os.Stderr, "使用 -force 强制覆盖")
			os.Exit(1)
		}

		cfg := config.DefaultConfig()
		if err := config.SaveConfig(cfg, configPath); err != nil {
			fmt.Fprintf(os.Stderr, "保存配置失败: %v\n", err)
			os.Exit(1)
		}

		fmt.Printf("配置文件已创建: %s\n", configPath)

	case "show":
		fs := flag.NewFlagSet("config show", flag.ExitOnError)
		dataDir := fs.String("data", "./data", "数据目录")
		fs.Parse(os.Args[3:])

		configPath := *dataDir + "/config.json"
		cfg, err := config.LoadConfig(configPath)
		if err != nil {
			fmt.Fprintf(os.Stderr, "加载配置失败: %v\n", err)
			os.Exit(1)
		}

		data, _ := json.MarshalIndent(cfg, "", "  ")
		fmt.Println(string(data))

	case "validate":
		fs := flag.NewFlagSet("config validate", flag.ExitOnError)
		dataDir := fs.String("data", "./data", "数据目录")
		fs.Parse(os.Args[3:])

		configPath := *dataDir + "/config.json"
		_, err := config.LoadConfig(configPath)
		if err != nil {
			fmt.Printf("❌ 配置无效: %v\n", err)
			os.Exit(1)
		}

		fmt.Println("✅ 配置有效")

	default:
		fmt.Fprintf(os.Stderr, "未知子命令: %s\n", subCmd)
		printConfigUsage()
		os.Exit(1)
	}
}

func printConfigUsage() {
	fmt.Print(`用法: agentnetwork config <子命令> [选项]

子命令:
  init      初始化配置文件
  show      显示当前配置
  validate  验证配置文件

选项:
  -data     数据目录 (默认: ./data)
  -force    强制覆盖现有配置 (仅用于 init)

示例:
  agentnetwork config init
  agentnetwork config init -force
  agentnetwork config show
  agentnetwork config validate
`)
}

func cmdKeygen() {
	fs := flag.NewFlagSet("keygen", flag.ExitOnError)
	dataDir := fs.String("data", "./data", "数据目录")
	force := fs.Bool("force", false, "强制覆盖现有密钥")
	fs.Parse(os.Args[2:])

	keyPath := *dataDir + "/keys/node.key"

	if _, err := os.Stat(keyPath); err == nil && !*force {
		fmt.Fprintf(os.Stderr, "密钥文件已存在: %s\n", keyPath)
		fmt.Fprintln(os.Stderr, "使用 -force 强制覆盖")
		os.Exit(1)
	}

	// 创建密钥目录
	keysDir := *dataDir + "/keys"
	if err := os.MkdirAll(keysDir, 0700); err != nil {
		fmt.Fprintf(os.Stderr, "创建目录失败: %v\n", err)
		os.Exit(1)
	}

	// 使用 identity 模块生成 libp2p 兼容的密钥
	id, err := identity.NewIdentity()
	if err != nil {
		fmt.Fprintf(os.Stderr, "生成密钥失败: %v\n", err)
		os.Exit(1)
	}

	// 保存私钥
	if err := id.Save(keyPath); err != nil {
		fmt.Fprintf(os.Stderr, "保存密钥失败: %v\n", err)
		os.Exit(1)
	}

	pubKeyHex, _ := id.PublicKeyHex()
	fmt.Println("======== 密钥生成成功 ========")
	fmt.Printf("私钥路径: %s\n", keyPath)
	fmt.Printf("节点ID:   %s\n", id.PeerID.String())
	fmt.Printf("公钥(hex): %s\n", pubKeyHex)
	fmt.Println("==============================")
	fmt.Println("⚠️  警告: 请妥善保管私钥文件!")
}

func cmdHealth() {
	fs := flag.NewFlagSet("health", flag.ExitOnError)
	dataDir := fs.String("data", "./data", "数据目录")
	httpAddr := fs.String("http", ":18345", "HTTP服务地址")
	timeout := fs.Int("timeout", 5, "超时时间（秒）")
	jsonOutput := fs.Bool("json", false, "JSON格式输出")
	fs.Parse(os.Args[2:])

	// 首先检查守护进程状态
	d := daemon.New(&daemon.Config{
		DataDir: *dataDir,
	})

	status := d.Status()

	healthResult := struct {
		Healthy     bool     `json:"healthy"`
		Process     bool     `json:"process"`
		HTTPService bool     `json:"http_service"`
		PID         int      `json:"pid,omitempty"`
		NodeID      string   `json:"node_id,omitempty"`
		Uptime      string   `json:"uptime,omitempty"`
		Errors      []string `json:"errors,omitempty"`
	}{
		Healthy:     true,
		Process:     status.Running,
		HTTPService: false,
		PID:         status.PID,
		NodeID:      status.NodeID,
		Uptime:      status.Uptime,
		Errors:      []string{},
	}

	if !status.Running {
		healthResult.Healthy = false
		healthResult.Errors = append(healthResult.Errors, "节点进程未运行")
	}

	// 检查 HTTP 服务
	if status.Running {
		httpURL := fmt.Sprintf("http://localhost%s/v1/health", *httpAddr)
		client := &httpClient{timeout: time.Duration(*timeout) * time.Second}
		if err := client.checkHealth(httpURL); err != nil {
			healthResult.Errors = append(healthResult.Errors, fmt.Sprintf("HTTP服务检查失败: %v", err))
		} else {
			healthResult.HTTPService = true
		}
	}

	healthResult.Healthy = healthResult.Process && healthResult.HTTPService

	if *jsonOutput {
		data, _ := json.MarshalIndent(healthResult, "", "  ")
		fmt.Println(string(data))
		if !healthResult.Healthy {
			os.Exit(1)
		}
		return
	}

	// 格式化输出
	fmt.Println("======== 健康检查 ========")
	if healthResult.Healthy {
		fmt.Println("状态: ✅ 健康")
	} else {
		fmt.Println("状态: ❌ 不健康")
	}

	fmt.Printf("进程状态: %s\n", boolToStatus(healthResult.Process))
	fmt.Printf("HTTP服务: %s\n", boolToStatus(healthResult.HTTPService))

	if healthResult.NodeID != "" {
		fmt.Printf("节点ID: %s\n", healthResult.NodeID)
	}
	if healthResult.Uptime != "" {
		fmt.Printf("运行时间: %s\n", healthResult.Uptime)
	}

	if len(healthResult.Errors) > 0 {
		fmt.Println("错误:")
		for _, err := range healthResult.Errors {
			fmt.Printf("  - %s\n", err)
		}
	}

	fmt.Println("==========================")

	if !healthResult.Healthy {
		os.Exit(1)
	}
}

// ============ 辅助函数 ============

func loadOrGenerateToken(dataDir string) string {
	tokenPath := dataDir + "/admin_token"
	data, err := os.ReadFile(tokenPath)
	if err == nil && len(data) > 0 {
		return strings.TrimSpace(string(data))
	}
	return generateAndSaveToken(dataDir)
}

func generateAndSaveToken(dataDir string) string {
	token := webadmin.GenerateToken()

	// 确保目录存在
	if err := os.MkdirAll(dataDir, 0755); err != nil {
		return token
	}

	tokenPath := dataDir + "/admin_token"
	_ = os.WriteFile(tokenPath, []byte(token), 0600)
	return token
}

func extractPort(addr string) int {
	if addr == "" {
		return 0
	}
	// 处理 :port 或 host:port 格式
	if strings.HasPrefix(addr, ":") {
		var port int
		fmt.Sscanf(addr, ":%d", &port)
		return port
	}
	parts := strings.Split(addr, ":")
	if len(parts) >= 2 {
		var port int
		fmt.Sscanf(parts[len(parts)-1], "%d", &port)
		return port
	}
	return 0
}

func boolToStatus(b bool) string {
	if b {
		return "✅"
	}
	return "❌"
}

// httpClient is a simple HTTP client for health checks.
type httpClient struct {
	timeout time.Duration
}

func (c *httpClient) checkHealth(url string) error {
	// 使用标准库进行健康检查
	client := &http.Client{Timeout: c.timeout}
	resp, err := client.Get(url)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("HTTP状态码: %d", resp.StatusCode)
	}
	return nil
}
