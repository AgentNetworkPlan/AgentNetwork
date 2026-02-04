# DAAN 构建与发布指南

> **Version**: v0.1.0 | **Last Updated**: 2026-02-04

本文档描述如何构建、测试和发布 DAAN 项目。

---

## 构建脚本

项目提供 PowerShell 构建脚本 `scripts/build.ps1`，支持跨平台编译和 GitHub Release。

### 基本用法

```powershell
# 查看帮助
.\scripts\build.ps1 -Help

# 编译所有平台 (6个)
.\scripts\build.ps1 -All

# 仅编译特定平台
.\scripts\build.ps1 -Windows
.\scripts\build.ps1 -Linux
.\scripts\build.ps1 -MacOS

# 组合编译
.\scripts\build.ps1 -Linux -MacOS
```

### 清理与发布

```powershell
# 清理构建目录
.\scripts\build.ps1 -Clean

# 提交并推送到 GitHub
.\scripts\build.ps1 -Push -Message "feat: new feature"

# 创建 GitHub Release
.\scripts\build.ps1 -Release -Version v0.1.0

# 一键编译并发布
.\scripts\build.ps1 -All -Release -Version v0.1.0
```

---

## 支持的平台

| 平台 | 架构 | 输出文件 |
|:-----|:-----|:---------|
| Windows | amd64 | `agentnetwork-windows-amd64.exe` |
| Windows | arm64 | `agentnetwork-windows-arm64.exe` |
| Linux | amd64 | `agentnetwork-linux-amd64` |
| Linux | arm64 | `agentnetwork-linux-arm64` |
| macOS | amd64 | `agentnetwork-darwin-amd64` |
| macOS | arm64 | `agentnetwork-darwin-arm64` |

构建产物位于 `build/` 目录。

---

## Makefile 命令

```bash
# 编译当前平台
make build

# 编译所有平台
make build-all

# 安装到系统
make install

# 运行测试
make test

# 清理
make clean

# 格式化代码
make fmt

# 代码检查
make lint
```

---

## 版本号规范

版本号格式：`vX.Y.Z`

- **X** (Major): 不兼容的 API 变更
- **Y** (Minor): 向后兼容的功能新增
- **Z** (Patch): 向后兼容的问题修复

### 发布流程

1. 更新版本号
   - `scripts/build.ps1` 中的 `$VERSION`
   - `Makefile` 中的 `VERSION`
   - `SKILL.md` 中的版本信息

2. 编译所有平台
   ```powershell
   .\scripts\build.ps1 -All
   ```

3. 测试
   ```bash
   go test ./...
   python scripts/lifecycle_test.py
   ```

4. 提交并打标签
   ```bash
   git add -A
   git commit -m "release: v0.1.0"
   git tag v0.1.0
   git push origin master --tags
   ```

5. 创建 Release
   ```powershell
   .\scripts\build.ps1 -Release -Version v0.1.0
   ```

---

## CI/CD（规划中）

未来将支持 GitHub Actions 自动化：

```yaml
# .github/workflows/release.yml
name: Release

on:
  push:
    tags:
      - 'v*'

jobs:
  build:
    runs-on: ubuntu-latest
    steps:
      - uses: actions/checkout@v4
      - uses: actions/setup-go@v5
        with:
          go-version: '1.22'
      - run: make build-all
      - uses: softprops/action-gh-release@v1
        with:
          files: build/*
```

---

## 故障排除

### 编译错误

```bash
# 清理 Go 缓存
go clean -cache

# 重新下载依赖
go mod tidy
```

### Release 创建失败

```powershell
# 检查 gh 登录状态
gh auth status

# 重新登录
gh auth login
```

### 跨平台编译问题

确保设置 `CGO_ENABLED=0`：
```bash
CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build ...
```
