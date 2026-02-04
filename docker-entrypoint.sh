#!/bin/sh
set -e

# Docker 容器启动脚本
echo "========================================"
echo "  AgentNetwork Node Starting..."
echo "========================================"
echo "Data Dir: ${DATA_DIR:-/data}"
echo "P2P Port: ${P2P_PORT:-9000}"
echo "HTTP Port: ${HTTP_PORT:-18345}"
echo "Admin Port: ${ADMIN_PORT:-18080}"
echo "GRPC Port: ${GRPC_PORT:-50051}"
echo "Bootstrap: ${BOOTSTRAP_ADDR:-none}"
echo "Node Role: ${NODE_ROLE:-normal}"
echo "========================================"

# 构建启动参数
ARGS="run"
ARGS="$ARGS -data ${DATA_DIR:-/data}"
ARGS="$ARGS -listen /ip4/0.0.0.0/tcp/${P2P_PORT:-9000}"
ARGS="$ARGS -http :${HTTP_PORT:-18345}"
ARGS="$ARGS -admin :${ADMIN_PORT:-18080}"
ARGS="$ARGS -grpc :${GRPC_PORT:-50051}"

# 如果有引导节点地址
if [ -n "$BOOTSTRAP_ADDR" ]; then
    ARGS="$ARGS -bootstrap $BOOTSTRAP_ADDR"
fi

# 如果有角色设置
if [ -n "$NODE_ROLE" ]; then
    ARGS="$ARGS -role $NODE_ROLE"
fi

echo "Starting with args: $ARGS"

exec /app/agentnetwork $ARGS
