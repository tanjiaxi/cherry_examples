#!/bin/bash

# Cherry框架Master节点启动脚本（修复protobuf冲突版本）

set -e

echo "🚀 启动Cherry框架Master节点（修复版）"
echo "====================================="

# 颜色定义
GREEN='\033[0;32m'
BLUE='\033[0;34m'
YELLOW='\033[1;33m'
NC='\033[0m'

# 检查NATS服务器
echo -e "${BLUE}📡 检查NATS服务器...${NC}"
if ! nc -z 127.0.0.1 4222 2>/dev/null; then
    echo -e "${YELLOW}⚠️  NATS服务器未运行，正在启动...${NC}"
    
    # 检查是否安装了NATS
    if command -v nats-server &> /dev/null; then
        nats-server --port 4222 --http_port 8222 &
        NATS_PID=$!
        echo "NATS服务器已启动 (PID: $NATS_PID)"
        sleep 2
    else
        echo "请先安装并启动NATS服务器："
        echo "  brew install nats-server"
        echo "  nats-server --port 4222"
        exit 1
    fi
else
    echo -e "${GREEN}✅ NATS服务器运行正常${NC}"
fi

# 切换到项目目录
cd "$(dirname "$0")/.."

# 使用修复后的配置文件启动Master节点
echo -e "${BLUE}🌸 启动Master节点...${NC}"
echo "配置文件: config/demo-cluster-nats.json"
echo "节点ID: gc-master"
echo ""

# 启动Master节点
go run ./demo_cluster/nodes/ \
    --profile=config/demo-cluster-nats.json \
    --node=gc-master

echo -e "${GREEN}✅ Master节点启动完成${NC}"