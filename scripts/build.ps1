#!/usr/bin/env pwsh
# =============================================================================
# DAAN 跨平台构建脚本
# 支持 Windows, Linux, macOS 编译打包
# =============================================================================

param(
    [switch]$All,           # 编译所有平台
    [switch]$Windows,       # 仅编译 Windows
    [switch]$Linux,         # 仅编译 Linux
    [switch]$MacOS,         # 仅编译 macOS
    [switch]$Clean,         # 清理构建目录
    [switch]$Push,          # 提交并推送到 GitHub
    [switch]$Release,       # 创建 GitHub Release
    [string]$Version = "",  # 版本号 (用于 Release)
    [string]$Message = "",  # 提交信息
    [switch]$Help           # 显示帮助
)

$ErrorActionPreference = "Stop"

# 项目信息
$PROJECT_NAME = "agentnetwork"
$VERSION = "0.1.0"
$BUILD_DIR = "build"
$MAIN_PATH = "./cmd/node/"

# 构建参数
$LDFLAGS = "-s -w -X main.Version=$VERSION"

# 平台配置
$PLATFORMS = @(
    @{ GOOS = "windows"; GOARCH = "amd64"; EXT = ".exe"; NAME = "windows-amd64" },
    @{ GOOS = "windows"; GOARCH = "arm64"; EXT = ".exe"; NAME = "windows-arm64" },
    @{ GOOS = "linux";   GOARCH = "amd64"; EXT = "";     NAME = "linux-amd64" },
    @{ GOOS = "linux";   GOARCH = "arm64"; EXT = "";     NAME = "linux-arm64" },
    @{ GOOS = "darwin";  GOARCH = "amd64"; EXT = "";     NAME = "darwin-amd64" },
    @{ GOOS = "darwin";  GOARCH = "arm64"; EXT = "";     NAME = "darwin-arm64" }
)

function Show-Help {
    Write-Host @"
DAAN 构建脚本

用法: .\scripts\build.ps1 [选项]

选项:
  -All          编译所有平台 (Windows/Linux/macOS, amd64/arm64)
  -Windows      仅编译 Windows (amd64 + arm64)
  -Linux        仅编译 Linux (amd64 + arm64)
  -MacOS        仅编译 macOS (amd64 + arm64)
  -Clean        清理构建目录
  -Push         提交并推送到 GitHub
  -Release      创建 GitHub Release (需要先编译)
  -Version      指定版本号 (如 v0.1.0)
  -Message      Git 提交信息 (与 -Push 一起使用)
  -Help         显示帮助

示例:
  .\scripts\build.ps1 -All                    # 编译所有平台
  .\scripts\build.ps1 -Windows                # 仅编译 Windows
  .\scripts\build.ps1 -Linux -MacOS           # 编译 Linux 和 macOS
  .\scripts\build.ps1 -Clean                  # 清理构建目录
  .\scripts\build.ps1 -Push -Message "feat: xxx"  # 提交并推送
  .\scripts\build.ps1 -All -Release -Version v0.1.0  # 编译并发布

输出目录: $BUILD_DIR/
"@
}

function Clean-Build {
    Write-Host "🧹 清理构建目录..." -ForegroundColor Yellow
    if (Test-Path $BUILD_DIR) {
        Remove-Item -Recurse -Force $BUILD_DIR
    }
    Write-Host "✅ 清理完成" -ForegroundColor Green
}

function Build-Platform {
    param(
        [string]$GOOS,
        [string]$GOARCH,
        [string]$EXT,
        [string]$NAME
    )
    
    $outputName = "$PROJECT_NAME-$NAME$EXT"
    $outputPath = "$BUILD_DIR/$outputName"
    
    Write-Host "🔨 编译 $NAME..." -ForegroundColor Cyan
    
    $env:GOOS = $GOOS
    $env:GOARCH = $GOARCH
    $env:CGO_ENABLED = "0"
    
    go build -ldflags $LDFLAGS -o $outputPath $MAIN_PATH
    
    if ($LASTEXITCODE -eq 0) {
        $size = [math]::Round((Get-Item $outputPath).Length / 1MB, 2)
        Write-Host "   ✅ $outputName ($size MB)" -ForegroundColor Green
    } else {
        Write-Host "   ❌ 编译失败: $NAME" -ForegroundColor Red
        exit 1
    }
}

function Build-All {
    Write-Host "`n📦 开始编译所有平台..." -ForegroundColor Magenta
    Write-Host "=" * 50
    
    if (-not (Test-Path $BUILD_DIR)) {
        New-Item -ItemType Directory -Path $BUILD_DIR | Out-Null
    }
    
    foreach ($p in $PLATFORMS) {
        Build-Platform -GOOS $p.GOOS -GOARCH $p.GOARCH -EXT $p.EXT -NAME $p.NAME
    }
    
    Write-Host "`n✅ 所有平台编译完成!" -ForegroundColor Green
    Show-BuildSummary
}

function Build-Selected {
    param([string[]]$OSList)
    
    Write-Host "`n📦 开始编译..." -ForegroundColor Magenta
    Write-Host "=" * 50
    
    if (-not (Test-Path $BUILD_DIR)) {
        New-Item -ItemType Directory -Path $BUILD_DIR | Out-Null
    }
    
    foreach ($p in $PLATFORMS) {
        if ($OSList -contains $p.GOOS) {
            Build-Platform -GOOS $p.GOOS -GOARCH $p.GOARCH -EXT $p.EXT -NAME $p.NAME
        }
    }
    
    Write-Host "`n✅ 编译完成!" -ForegroundColor Green
    Show-BuildSummary
}

function Show-BuildSummary {
    Write-Host "`n📋 构建产物:" -ForegroundColor Yellow
    Get-ChildItem $BUILD_DIR | ForEach-Object {
        $size = [math]::Round($_.Length / 1MB, 2)
        Write-Host "   $($_.Name) - $size MB"
    }
}

function Git-Push {
    param([string]$CommitMessage)
    
    Write-Host "`n🔄 检查 Git 状态..." -ForegroundColor Cyan
    
    $status = git status --porcelain
    if (-not $status) {
        Write-Host "⚠️  没有需要提交的更改" -ForegroundColor Yellow
        return
    }
    
    Write-Host "📝 未提交的文件:" -ForegroundColor Yellow
    git status --short
    
    if (-not $CommitMessage) {
        $CommitMessage = Read-Host "`n请输入提交信息"
    }
    
    if (-not $CommitMessage) {
        Write-Host "❌ 提交信息不能为空" -ForegroundColor Red
        return
    }
    
    Write-Host "`n📤 提交并推送..." -ForegroundColor Cyan
    git add -A
    git commit -m $CommitMessage
    git push origin master
    
    if ($LASTEXITCODE -eq 0) {
        Write-Host "✅ 推送成功!" -ForegroundColor Green
    } else {
        Write-Host "❌ 推送失败" -ForegroundColor Red
    }
}

function Create-Release {
    param([string]$ReleaseVersion)
    
    Write-Host "`n🚀 创建 GitHub Release..." -ForegroundColor Magenta
    
    # 检查 gh 命令
    if (-not (Get-Command gh -ErrorAction SilentlyContinue)) {
        Write-Host "❌ 未找到 gh 命令，请先安装 GitHub CLI" -ForegroundColor Red
        Write-Host "   安装: https://cli.github.com/" -ForegroundColor Yellow
        return
    }
    
    # 检查构建目录
    if (-not (Test-Path $BUILD_DIR) -or (Get-ChildItem $BUILD_DIR).Count -eq 0) {
        Write-Host "❌ 构建目录为空，请先运行 -All 编译" -ForegroundColor Red
        return
    }
    
    # 确定版本号
    if (-not $ReleaseVersion) {
        $ReleaseVersion = "v$VERSION"
    }
    if (-not $ReleaseVersion.StartsWith("v")) {
        $ReleaseVersion = "v$ReleaseVersion"
    }
    
    Write-Host "📦 版本: $ReleaseVersion" -ForegroundColor Cyan
    
    # 获取构建产物
    $assets = Get-ChildItem $BUILD_DIR | ForEach-Object { $_.FullName }
    $assetCount = $assets.Count
    
    Write-Host "📁 上传 $assetCount 个文件..." -ForegroundColor Cyan
    
    # 生成 Release Notes
    $releaseNotes = @"
## DAAN $ReleaseVersion

### 下载

| 平台 | 架构 | 文件 |
|:-----|:-----|:-----|
| Windows | amd64 | agentnetwork-windows-amd64.exe |
| Windows | arm64 | agentnetwork-windows-arm64.exe |
| Linux | amd64 | agentnetwork-linux-amd64 |
| Linux | arm64 | agentnetwork-linux-arm64 |
| macOS | amd64 | agentnetwork-darwin-amd64 |
| macOS | arm64 | agentnetwork-darwin-arm64 |

### 使用方法

``````bash
# 下载后添加执行权限 (Linux/macOS)
chmod +x agentnetwork-*

# 初始化并启动
./agentnetwork config init
./agentnetwork keygen
./agentnetwork start

# 查看帮助
./agentnetwork -h
``````
"@
    
    # 创建 Release
    Write-Host "`n🔄 创建 Release $ReleaseVersion ..." -ForegroundColor Cyan
    
    $releaseArgs = @(
        "release", "create", $ReleaseVersion,
        "--title", "DAAN $ReleaseVersion",
        "--notes", $releaseNotes
    )
    
    # 添加所有资产文件
    foreach ($asset in $assets) {
        $releaseArgs += $asset
    }
    
    & gh @releaseArgs
    
    if ($LASTEXITCODE -eq 0) {
        Write-Host "`n✅ Release $ReleaseVersion 发布成功!" -ForegroundColor Green
        Write-Host "🔗 https://github.com/AgentNetworkPlan/AgentNetwork/releases/tag/$ReleaseVersion" -ForegroundColor Cyan
    } else {
        Write-Host "❌ Release 创建失败" -ForegroundColor Red
    }
}

# =============================================================================
# 主逻辑
# =============================================================================

if ($Help) {
    Show-Help
    exit 0
}

if ($Clean) {
    Clean-Build
    exit 0
}

if ($Push) {
    Git-Push -CommitMessage $Message
    exit 0
}

if ($Release) {
    Create-Release -ReleaseVersion $Version
    exit 0
}

# 确定要编译的平台
$selectedOS = @()
if ($Windows) { $selectedOS += "windows" }
if ($Linux)   { $selectedOS += "linux" }
if ($MacOS)   { $selectedOS += "darwin" }

if ($All -or $selectedOS.Count -eq 0) {
    # 默认编译所有平台
    Build-All
} else {
    Build-Selected -OSList $selectedOS
}
