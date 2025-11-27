#!/bin/bash

# 重启所有游戏集群节点

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
cd "$SCRIPT_DIR"

# 颜色定义
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
BLUE='\033[0;34m'
NC='\033[0m' # No Color

echo -e "${BLUE}========================================${NC}"
echo -e "${BLUE}🔄 重启所有游戏集群节点${NC}"
echo -e "${BLUE}========================================${NC}"
echo ""

# 停止所有节点
echo -e "${YELLOW}步骤 1/2: 停止所有节点${NC}"
./stop_all.sh

echo ""
echo -e "${YELLOW}等待 3 秒...${NC}"
sleep 3

echo ""
echo -e "${YELLOW}步骤 2/2: 启动所有节点${NC}"
./start_all.sh

echo ""
echo -e "${GREEN}========================================${NC}"
echo -e "${GREEN}✅ 重启完成！${NC}"
echo -e "${GREEN}========================================${NC}"
echo ""
