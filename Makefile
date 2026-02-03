.PHONY: build run test clean fmt lint deps

# 项目信息
PROJECT_NAME := agent-network
VERSION := 0.1.0
BUILD_DIR := build
MAIN_PATH := ./cmd/agent

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
LDFLAGS := -ldflags "-X main.Version=$(VERSION)"

# 默认目标
all: deps build

# 安装依赖
deps:
	$(GOMOD) download
	$(GOMOD) tidy

# 构建
build:
	@mkdir -p $(BUILD_DIR)
	$(GOBUILD) $(LDFLAGS) -o $(BUILD_DIR)/$(PROJECT_NAME).exe $(MAIN_PATH)

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
	@echo "  make run           - 运行项目"
	@echo "  make test          - 运行测试"
	@echo "  make test-coverage - 运行测试并生成覆盖率报告"
	@echo "  make fmt           - 格式化代码"
	@echo "  make lint          - 代码检查"
	@echo "  make clean         - 清理构建产物"
	@echo "  make keygen        - 生成密钥对"
