# 恶意节点集群测试脚本
# 功能: 启动30个节点(包含10个恶意节点)，监控资源，执行攻击测试

$ErrorActionPreference = "Continue"
$ProjectRoot = "D:\WangXianQiang\github\hyfree\AgentNetwork"
$TestnetDir = "$ProjectRoot\testnet_malicious"
$DataDir = "$TestnetDir\data"
$LogDir = "$TestnetDir\logs"

# 配置
$MaxNodes = 30
$MaliciousNodeIndices = @(2, 5, 8, 12, 15, 18, 21, 24, 27, 29)  # 10个恶意节点

# 节点信息存储
$Global:Nodes = @{}
$Global:NodeProcesses = @{}
$Global:Report = @{
    start_time = (Get-Date).ToString("o")
    stages = @()
    attack_events = @()
    network_stats = @()
    security_analysis = ""
    performance_analysis = ""
}

# 日志函数
function Write-Log {
    param($Message, $Level = "INFO")
    $timestamp = Get-Date -Format "HH:mm:ss"
    $color = switch ($Level) {
        "INFO" { "Green" }
        "WARN" { "Yellow" }
        "ERROR" { "Red" }
        "STEP" { "Cyan" }
        "MALICIOUS" { "Magenta" }
        default { "White" }
    }
    Write-Host "[$timestamp] [$Level] $Message" -ForegroundColor $color
}

# 创建节点配置
function New-NodeConfig {
    param($Index, $IsGenesis = $false, $IsMalicious = $false)
    
    $basePort = 9000 + ($Index * 10)
    $node = @{
        id = "node{0:D2}" -f $Index
        index = $Index
        p2p_port = $basePort
        http_port = $basePort + 1
        admin_port = $basePort + 2
        grpc_port = $basePort + 3
        data_dir = "$DataDir\node{0:D2}" -f $Index
        is_genesis = $IsGenesis
        is_malicious = $IsMalicious
        status = "stopped"
        pid = 0
        api_token = ""
    }
    
    New-Item -ItemType Directory -Force -Path $node.data_dir | Out-Null
    $Global:Nodes[$node.id] = $node
    
    $nodeType = if ($IsGenesis) { "GENESIS" } elseif ($IsMalicious) { "MALICIOUS" } else { "NORMAL" }
    Write-Log "创建节点: $($node.id) [类型: $nodeType] [端口: P2P=$($node.p2p_port), HTTP=$($node.http_port)]"
    
    return $node
}

# 启动节点
function Start-NodeProcess {
    param($Node, $BootstrapAddr = "")
    
    $cmd = "go run ./cmd/node/main.go run " +
           "-data `"$($Node.data_dir)`" " +
           "-listen `"/ip4/0.0.0.0/tcp/$($Node.p2p_port)`" " +
           "-http `":$($Node.http_port)`" " +
           "-admin `":$($Node.admin_port)`" " +
           "-grpc `":$($Node.grpc_port)`""
    
    if ($BootstrapAddr -and -not $Node.is_genesis) {
        $cmd += " -bootstrap `"$BootstrapAddr`""
    }
    
    $logFile = "$LogDir\$($Node.id).log"
    $nodeType = if ($Node.is_malicious) { "[恶意节点]" } else { "" }
    Write-Log "启动节点 $($Node.id) $nodeType" "STEP"
    
    $process = Start-Process -FilePath "powershell" -ArgumentList "-Command", "cd '$ProjectRoot'; $cmd 2>&1 | Tee-Object -FilePath '$logFile'" -PassThru -WindowStyle Hidden
    
    $Global:NodeProcesses[$Node.id] = $process
    $Node.pid = $process.Id
    $Node.status = "starting"
    
    # 等待节点启动
    Start-Sleep -Seconds 3
    
    # 获取API Token
    if (Test-Path $logFile) {
        $content = Get-Content $logFile -Raw -ErrorAction SilentlyContinue
        if ($content -match "X-API-Token:\s*([a-f0-9]+)") {
            $Node.api_token = $Matches[1]
        }
    }
    
    return $Node
}

# 等待节点就绪
function Wait-NodeReady {
    param($Node, $Timeout = 15)
    
    $startTime = Get-Date
    while ((Get-Date) - $startTime -lt [TimeSpan]::FromSeconds($Timeout)) {
        try {
            $response = Invoke-WebRequest -Uri "http://127.0.0.1:$($Node.http_port)/health" -TimeoutSec 2 -ErrorAction SilentlyContinue
            if ($response.StatusCode -eq 200) {
                $Node.status = "running"
                return $true
            }
        } catch {}
        Start-Sleep -Milliseconds 500
    }
    $Node.status = "failed"
    return $false
}

# API调用
function Invoke-NodeAPI {
    param($Node, $Endpoint, $Method = "GET", $Body = $null)
    
    try {
        $headers = @{}
        if ($Node.api_token) {
            $headers["X-API-Token"] = $Node.api_token
        }
        
        $url = "http://127.0.0.1:$($Node.http_port)/api/v1$Endpoint"
        
        if ($Method -eq "GET") {
            $response = Invoke-RestMethod -Uri $url -Method GET -Headers $headers -TimeoutSec 5 -ErrorAction SilentlyContinue
        } else {
            $jsonBody = $Body | ConvertTo-Json -Compress
            $response = Invoke-RestMethod -Uri $url -Method $Method -Headers $headers -Body $jsonBody -ContentType "application/json" -TimeoutSec 5 -ErrorAction SilentlyContinue
        }
        return $response
    } catch {
        return $null
    }
}

# 获取节点信息
function Get-NodeInfo {
    param($Node)
    return Invoke-NodeAPI -Node $Node -Endpoint "/node/info"
}

# 获取邻居列表
function Get-NodeNeighbors {
    param($Node)
    $result = Invoke-NodeAPI -Node $Node -Endpoint "/neighbor/list"
    if ($result -and $result.neighbors) {
        return $result.neighbors
    }
    return @()
}

# 执行恶意行为
function Invoke-MaliciousBehavior {
    param($Node)
    
    if (-not $Node.is_malicious) { return }
    
    $behaviors = @(
        @{ name = "垃圾消息攻击"; action = { Send-SpamMessages -Node $Node -Count (Get-Random -Minimum 5 -Maximum 20) } },
        @{ name = "虚假信息广播"; action = { Send-FalseInfo -Node $Node } },
        @{ name = "畸形请求攻击"; action = { Send-MalformedRequests -Node $Node } },
        @{ name = "重放攻击尝试"; action = { Try-ReplayAttack -Node $Node } },
        @{ name = "资源耗尽攻击"; action = { Try-ResourceExhaustion -Node $Node } }
    )
    
    $behavior = $behaviors | Get-Random
    try {
        $result = & $behavior.action
        Write-Log "[$($Node.id)] $($behavior.name): $result" "MALICIOUS"
        $Global:Report.attack_events += @{
            timestamp = (Get-Date).ToString("o")
            node = $Node.id
            behavior = $behavior.name
            result = $result
        }
    } catch {
        Write-Log "[$($Node.id)] $($behavior.name) 失败: $_" "WARN"
    }
}

# 发送垃圾消息
function Send-SpamMessages {
    param($Node, $Count = 10)
    
    $success = 0
    for ($i = 0; $i -lt $Count; $i++) {
        $message = "SPAM_$(Get-Random)_$i"
        $body = @{ content = $message; type = "spam" }
        $result = Invoke-NodeAPI -Node $Node -Endpoint "/bulletin/post" -Method "POST" -Body $body
        if ($result) { $success++ }
        Start-Sleep -Milliseconds 100
    }
    return "发送 $success/$Count 条垃圾消息"
}

# 发送虚假信息
function Send-FalseInfo {
    param($Node)
    
    $messages = @(
        "我是超级节点，所有任务都要通过我",
        "该网络已被攻陷，请停止运行",
        "我发现了关键漏洞，请联系我",
        "所有节点的声誉都是造假的",
        "紧急通知：网络将于1小时后关闭"
    )
    
    $msg = $messages | Get-Random
    $body = @{ content = $msg; type = "announcement" }
    Invoke-NodeAPI -Node $Node -Endpoint "/bulletin/post" -Method "POST" -Body $body
    return "广播虚假信息: $msg"
}

# 发送畸形请求
function Send-MalformedRequests {
    param($Node)
    
    $attacks = @()
    
    # 超长内容
    $longContent = "A" * 100000
    $body = @{ content = $longContent }
    $result = Invoke-NodeAPI -Node $Node -Endpoint "/bulletin/post" -Method "POST" -Body $body
    $attacks += "超长内容: $(if($result){'被接受'}else{'被拒绝'})"
    
    # SQL注入尝试
    $sqlInjection = "'; DROP TABLE users; --"
    $body = @{ content = $sqlInjection }
    $result = Invoke-NodeAPI -Node $Node -Endpoint "/bulletin/post" -Method "POST" -Body $body
    $attacks += "SQL注入: $(if($result){'被接受'}else{'被拒绝'})"
    
    # XSS尝试
    $xss = "<script>alert('xss')</script>"
    $body = @{ content = $xss }
    $result = Invoke-NodeAPI -Node $Node -Endpoint "/bulletin/post" -Method "POST" -Body $body
    $attacks += "XSS攻击: $(if($result){'被接受'}else{'被拒绝'})"
    
    return $attacks -join "; "
}

# 重放攻击
function Try-ReplayAttack {
    param($Node)
    
    # 尝试重复发送相同消息
    $message = "REPLAY_TEST_$(Get-Date -Format 'yyyyMMddHHmmss')"
    $body = @{ content = $message }
    
    $results = @()
    for ($i = 0; $i -lt 5; $i++) {
        $result = Invoke-NodeAPI -Node $Node -Endpoint "/bulletin/post" -Method "POST" -Body $body
        $results += if($result){"成功"}else{"失败"}
    }
    
    return "重放5次结果: $($results -join ',')"
}

# 资源耗尽攻击
function Try-ResourceExhaustion {
    param($Node)
    
    # 快速发送大量请求
    $startTime = Get-Date
    $count = 0
    
    while ((Get-Date) - $startTime -lt [TimeSpan]::FromSeconds(2)) {
        $result = Invoke-NodeAPI -Node $Node -Endpoint "/node/info"
        if ($result) { $count++ }
    }
    
    return "2秒内发送 $count 个请求"
}

# 监控网络状态
function Get-NetworkStats {
    $stats = @{
        timestamp = (Get-Date).ToString("o")
        total_nodes = $Global:Nodes.Count
        running_nodes = 0
        malicious_nodes = 0
        nodes_info = @{}
    }
    
    foreach ($node in $Global:Nodes.Values) {
        if ($node.status -eq "running") {
            $stats.running_nodes++
            if ($node.is_malicious) {
                $stats.malicious_nodes++
            }
        }
        
        $neighbors = @()
        if ($node.status -eq "running") {
            $neighbors = Get-NodeNeighbors -Node $node
        }
        
        $stats.nodes_info[$node.id] = @{
            status = $node.status
            is_malicious = $node.is_malicious
            neighbors_count = $neighbors.Count
        }
    }
    
    return $stats
}

# 获取系统资源
function Get-SystemResources {
    $cpuCounter = Get-Counter '\Processor(_Total)\% Processor Time' -ErrorAction SilentlyContinue
    $cpuUsage = if ($cpuCounter) { [math]::Round($cpuCounter.CounterSamples[0].CookedValue, 2) } else { 0 }
    
    $memory = Get-Process | Measure-Object WorkingSet64 -Sum
    $totalMemory = (Get-CimInstance Win32_ComputerSystem).TotalPhysicalMemory
    $memoryUsage = [math]::Round(($memory.Sum / $totalMemory) * 100, 2)
    
    $goProcesses = Get-Process go -ErrorAction SilentlyContinue
    $goMemory = if ($goProcesses) { [math]::Round(($goProcesses | Measure-Object WorkingSet64 -Sum).Sum / 1MB, 2) } else { 0 }
    
    return @{
        cpu_percent = $cpuUsage
        memory_percent = $memoryUsage
        go_memory_mb = $goMemory
        go_process_count = ($goProcesses | Measure-Object).Count
    }
}

# 打印网络状态
function Show-NetworkStatus {
    $stats = Get-NetworkStats
    $resources = Get-SystemResources
    
    Write-Host ""
    Write-Host "═══════════════════════════════════════════════════════════" -ForegroundColor Cyan
    Write-Host " 网络状态 - 总节点: $($stats.total_nodes) | 运行中: $($stats.running_nodes) | 恶意节点: $($stats.malicious_nodes)" -ForegroundColor Cyan
    Write-Host " CPU: $($resources.cpu_percent)% | 内存: $($resources.memory_percent)% | Go进程内存: $($resources.go_memory_mb)MB" -ForegroundColor Cyan
    Write-Host "═══════════════════════════════════════════════════════════" -ForegroundColor Cyan
    
    foreach ($node in ($Global:Nodes.Values | Sort-Object index)) {
        $symbol = if ($node.status -eq "running") { "✓" } else { "✗" }
        $type = if ($node.is_malicious) { "[恶意]" } else { "[正常]" }
        $color = if ($node.is_malicious) { "Magenta" } elseif ($node.status -eq "running") { "Green" } else { "Red" }
        $neighbors = 0
        if ($stats.nodes_info[$node.id]) {
            $neighbors = $stats.nodes_info[$node.id].neighbors_count
        }
        Write-Host "  $symbol $($node.id) $type 邻居数: $neighbors" -ForegroundColor $color
    }
    Write-Host "═══════════════════════════════════════════════════════════" -ForegroundColor Cyan
    
    return @{
        stats = $stats
        resources = $resources
    }
}

# 停止所有节点
function Stop-AllNodes {
    Write-Log "正在停止所有节点..." "STEP"
    
    foreach ($node in $Global:Nodes.Values) {
        if ($Global:NodeProcesses[$node.id]) {
            try {
                $proc = $Global:NodeProcesses[$node.id]
                if (-not $proc.HasExited) {
                    $proc | Stop-Process -Force -ErrorAction SilentlyContinue
                }
            } catch {}
        }
        $node.status = "stopped"
        Write-Log "停止节点 $($node.id)"
    }
    
    # 清理所有go进程
    Get-Process go -ErrorAction SilentlyContinue | Stop-Process -Force -ErrorAction SilentlyContinue
    
    Write-Log "所有节点已停止" "INFO"
}

# 主测试流程
function Start-MaliciousNodeTest {
    Write-Host ""
    Write-Host "╔══════════════════════════════════════════════════════════╗" -ForegroundColor Yellow
    Write-Host "║          恶意节点集群测试 - AgentNetwork                 ║" -ForegroundColor Yellow
    Write-Host "╚══════════════════════════════════════════════════════════╝" -ForegroundColor Yellow
    Write-Host ""
    
    try {
        # 阶段1: 启动创世节点
        Write-Host ""
        Write-Log "═══ 阶段 1: 启动创世节点 ═══" "STEP"
        
        $genesisNode = New-NodeConfig -Index 0 -IsGenesis $true
        Start-NodeProcess -Node $genesisNode | Out-Null
        
        if (Wait-NodeReady -Node $genesisNode -Timeout 20) {
            Write-Log "创世节点已就绪" "INFO"
            $Global:Report.stages += @{ stage = "genesis"; status = "success"; nodes = 1 }
        } else {
            Write-Log "创世节点启动失败" "ERROR"
            return
        }
        
        Start-Sleep -Seconds 3
        Show-NetworkStatus | Out-Null
        
        # 阶段2: 扩展到10个节点
        Write-Host ""
        Write-Log "═══ 阶段 2: 扩展到10个节点（含2个恶意节点）═══" "STEP"
        
        $bootstrapAddr = "/ip4/127.0.0.1/tcp/$($genesisNode.p2p_port)/p2p/$(Get-NodePeerId -Node $genesisNode)"
        if (-not $bootstrapAddr -or $bootstrapAddr -like "*//p2p/") {
            $bootstrapAddr = "127.0.0.1:$($genesisNode.p2p_port)"
        }
        
        for ($i = 1; $i -lt 10; $i++) {
            $isMalicious = $i -in $MaliciousNodeIndices
            $node = New-NodeConfig -Index $i -IsMalicious $isMalicious
            Start-NodeProcess -Node $node -BootstrapAddr $bootstrapAddr | Out-Null
            
            if (Wait-NodeReady -Node $node -Timeout 15) {
                Write-Log "节点 $($node.id) 已加入网络" "INFO"
                
                # 恶意节点立即开始攻击
                if ($isMalicious) {
                    Invoke-MaliciousBehavior -Node $node
                }
            }
            
            Start-Sleep -Seconds 2
        }
        
        Show-NetworkStatus | Out-Null
        $Global:Report.stages += @{ stage = "expand_to_10"; nodes = 10; status = "success" }
        
        # 阶段3: 继续扩展并监控
        Write-Host ""
        Write-Log "═══ 阶段 3: 扩展到30个节点（含10个恶意节点）═══" "STEP"
        
        for ($i = 10; $i -lt $MaxNodes; $i++) {
            $isMalicious = $i -in $MaliciousNodeIndices
            $node = New-NodeConfig -Index $i -IsMalicious $isMalicious
            Start-NodeProcess -Node $node -BootstrapAddr $bootstrapAddr | Out-Null
            
            if (Wait-NodeReady -Node $node -Timeout 15) {
                Write-Log "节点 $($node.id) 已加入网络 (总数: $($Global:Nodes.Count))" "INFO"
                
                if ($isMalicious) {
                    Invoke-MaliciousBehavior -Node $node
                }
            }
            
            # 每5个节点检查一次资源
            if ($i % 5 -eq 0) {
                $status = Show-NetworkStatus
                $Global:Report.network_stats += $status
                
                # 检查资源是否过高
                if ($status.resources.cpu_percent -gt 90 -or $status.resources.memory_percent -gt 85) {
                    Write-Log "资源使用率过高，停止扩展" "WARN"
                    break
                }
            }
            
            Start-Sleep -Seconds 2
        }
        
        Show-NetworkStatus | Out-Null
        $Global:Report.stages += @{ stage = "expand_to_30"; nodes = $Global:Nodes.Count; status = "success" }
        
        # 阶段4: 持续观察和攻击
        Write-Host ""
        Write-Log "═══ 阶段 4: 持续观察网络（恶意节点持续攻击）═══" "STEP"
        
        for ($round = 1; $round -le 5; $round++) {
            Write-Log "观察轮次 $round/5" "INFO"
            
            # 所有恶意节点执行攻击
            foreach ($node in ($Global:Nodes.Values | Where-Object { $_.is_malicious -and $_.status -eq "running" })) {
                Invoke-MaliciousBehavior -Node $node
            }
            
            # 正常节点执行正常操作
            foreach ($node in ($Global:Nodes.Values | Where-Object { -not $_.is_malicious -and $_.status -eq "running" } | Get-Random -Count 3)) {
                $body = @{ content = "正常消息 from $($node.id) at $(Get-Date -Format 'HH:mm:ss')" }
                Invoke-NodeAPI -Node $node -Endpoint "/bulletin/post" -Method "POST" -Body $body | Out-Null
            }
            
            $status = Show-NetworkStatus
            $Global:Report.network_stats += $status
            
            Start-Sleep -Seconds 5
        }
        
        $Global:Report.stages += @{ stage = "observation"; rounds = 5; status = "success" }
        
    } finally {
        # 阶段5: 关闭所有节点
        Write-Host ""
        Write-Log "═══ 阶段 5: 关闭所有节点 ═══" "STEP"
        Stop-AllNodes
        
        $Global:Report.end_time = (Get-Date).ToString("o")
        $Global:Report.stages += @{ stage = "shutdown"; status = "success" }
        
        # 保存报告
        $reportPath = "$LogDir\malicious_test_report.json"
        $Global:Report | ConvertTo-Json -Depth 10 | Out-File $reportPath -Encoding UTF8
        Write-Log "测试报告已保存: $reportPath" "INFO"
    }
}

function Get-NodePeerId {
    param($Node)
    
    $logFile = "$LogDir\$($Node.id).log"
    if (Test-Path $logFile) {
        $content = Get-Content $logFile -Raw -ErrorAction SilentlyContinue
        if ($content -match "PeerID:\s*(\S+)") {
            return $Matches[1]
        }
    }
    return ""
}

# 运行测试
Start-MaliciousNodeTest
