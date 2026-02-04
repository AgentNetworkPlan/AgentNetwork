.PHONY: build build-all run test clean fmt lint deps install

# 项目信息
PROJECT_NAME := agentnetwork
VERSION := 0.1.0
BUILD_DIR := build
MAIN_PATH := ./cmd/node

# Go 参数
GOCMD := go
GOBUILD := $(GOCMD) build
GORUN := $(GOCMD) run
GOTEST := $(GOCMD) test
GOCLEAN := $(GOCMD) clean
GOGET := $(GOCMD) get
GOMOD := $(GOCMD) mod
GOFMT := $(GOCMD) fmt
GOVET := $(GOCMD) vet

# 构建参数
LDFLAGS := -ldflags "-s -w -X main.Version=$(VERSION)"

# 默认目标
all: deps build

# 安装依赖
deps:
	$(GOMOD) download
	$(GOMOD) tidy

# 构建当前平台
build:
	@mkdir -p $(BUILD_DIR)
ifeq ($(OS),Windows_NT)
	$(GOBUILD) $(LDFLAGS) -o $(BUILD_DIR)/$(PROJECT_NAME).exe $(MAIN_PATH)
else
	$(GOBUILD) $(LDFLAGS) -o $(BUILD_DIR)/$(PROJECT_NAME) $(MAIN_PATH)
endif

# 跨平台构建 (所有平台)
build-all:
	@mkdir -p $(BUILD_DIR)
	@echo "Building for all platforms..."
	GOOS=windows GOARCH=amd64 CGO_ENABLED=0 $(GOBUILD) $(LDFLAGS) -o $(BUILD_DIR)/$(PROJECT_NAME)-windows-amd64.exe $(MAIN_PATH)
	GOOS=windows GOARCH=arm64 CGO_ENABLED=0 $(GOBUILD) $(LDFLAGS) -o $(BUILD_DIR)/$(PROJECT_NAME)-windows-arm64.exe $(MAIN_PATH)
	GOOS=linux GOARCH=amd64 CGO_ENABLED=0 $(GOBUILD) $(LDFLAGS) -o $(BUILD_DIR)/$(PROJECT_NAME)-linux-amd64 $(MAIN_PATH)
	GOOS=linux GOARCH=arm64 CGO_ENABLED=0 $(GOBUILD) $(LDFLAGS) -o $(BUILD_DIR)/$(PROJECT_NAME)-linux-arm64 $(MAIN_PATH)
	GOOS=darwin GOARCH=amd64 CGO_ENABLED=0 $(GOBUILD) $(LDFLAGS) -o $(BUILD_DIR)/$(PROJECT_NAME)-darwin-amd64 $(MAIN_PATH)
	GOOS=darwin GOARCH=arm64 CGO_ENABLED=0 $(GOBUILD) $(LDFLAGS) -o $(BUILD_DIR)/$(PROJECT_NAME)-darwin-arm64 $(MAIN_PATH)
	@echo "Done! Check $(BUILD_DIR)/"

# 安装到系统路径
install: build
ifeq ($(OS),Windows_NT)
	@echo "Please manually copy $(BUILD_DIR)/$(PROJECT_NAME).exe to your PATH"
else
	sudo cp $(BUILD_DIR)/$(PROJECT_NAME) /usr/local/bin/
endif

# 运行
run:
	$(GORUN) $(MAIN_PATH)

# 测试
test:
	$(GOTEST) -v ./...

# 测试覆盖率
test-coverage:
	$(GOTEST) -v -coverprofile=coverage.out ./...
	$(GOCMD) tool cover -html=coverage.out -o coverage.html

# 格式化代码
fmt:
	$(GOFMT) ./...

# 代码检查
lint:
	$(GOVET) ./...

# 清理
clean:
	$(GOCLEAN)
	rm -rf $(BUILD_DIR)
	rm -f coverage.out coverage.html

# 生成密钥
keygen:
	@mkdir -p keys
	@echo "请运行 Python 脚本生成密钥："
	@echo "python scripts/generate_keypair.py --algorithm sm2"

# 帮助
help:
	@echo "可用命令："
	@echo "  make deps          - 安装依赖"
	@echo "  make build         - 构建项目"
	@echo "  make build-admin   - 构建管理界面"
	@echo "  make run           - 运行项目"
	@echo "  make test          - 运行测试"
	@echo "  make test-coverage - 运行测试并生成覆盖率报告"
	@echo "  make fmt           - 格式化代码"
	@echo "  make lint          - 代码检查"
	@echo "  make clean         - 清理构建产物"
	@echo "  make keygen        - 生成密钥对"
	@echo "  make dev-admin     - 启动前端开发服务器"

# 构建管理界面 (Vue.js)
build-admin:
	@echo "构建 DAAN Admin 前端..."
	cd web/admin && pnpm install && pnpm run build
	@echo "前端已构建到 internal/webadmin/static/"

# 启动前端开发服务器
dev-admin:
	@echo "启动前端开发服务器..."
	cd web/admin && pnpm run dev
