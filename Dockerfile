# 多阶段构建：编译阶段
FROM golang:1.23-alpine AS builder

# 设置允许使用更新的Go版本
ENV GOTOOLCHAIN=auto

# 安装必要的构建工具
RUN apk add --no-cache git make

WORKDIR /app

# 复制 go mod 文件并下载依赖
COPY go.mod go.sum ./
RUN go mod download

# 复制源代码
COPY . .

# 编译
RUN CGO_ENABLED=0 GOOS=linux go build -ldflags="-s -w" -o /app/agentnetwork ./cmd/node/main.go

# 运行阶段 - 使用同一镜像避免拉取问题
FROM golang:1.23-alpine

# 安装运行时依赖
RUN apk add --no-cache ca-certificates tzdata curl

WORKDIR /app

# 从构建阶段复制可执行文件
COPY --from=builder /app/agentnetwork /app/agentnetwork

# 创建数据目录
RUN mkdir -p /data

# 环境变量
ENV DATA_DIR=/data
ENV P2P_PORT=9000
ENV HTTP_PORT=18345
ENV ADMIN_PORT=18080
ENV GRPC_PORT=50051

# 暴露端口
EXPOSE 9000 18345 18080 50051

# 启动脚本
COPY docker-entrypoint.sh /app/docker-entrypoint.sh
RUN chmod +x /app/docker-entrypoint.sh

ENTRYPOINT ["/app/docker-entrypoint.sh"]
