#!/bin/bash

# Cherry框架 Protobuf冲突修复脚本

set -e

echo "🔧 修复Cherry框架Protobuf冲突"
echo "================================"

# 颜色定义
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
BLUE='\033[0;34m'
NC='\033[0m'

echo -e "${BLUE}📋 检查当前依赖...${NC}"
echo "当前protobuf相关依赖："
go list -m all | grep -E "(protobuf|etcd)" || true

echo ""
echo -e "${YELLOW}🔄 清理依赖缓存...${NC}"
go clean -modcache

echo -e "${YELLOW}📦 重新下载依赖...${NC}"
go mod download

echo -e "${YELLOW}🧹 整理依赖...${NC}"
go mod tidy

echo -e "${YELLOW}🔍 验证依赖...${NC}"
go mod verify

echo ""
echo -e "${BLUE}📋 修复后的依赖：${NC}"
go list -m all | grep -E "(protobuf|etcd)" || true

echo ""
echo -e "${GREEN}✅ 依赖修复完成！${NC}"
echo -e "${BLUE}💡 建议：${NC}"
echo "1. 如果仍有冲突，考虑使用replace指令固定版本"
echo "2. 或者暂时不使用etcd组件，使用NATS discovery"
echo "3. 等待Cherry框架更新解决版本兼容性"

echo ""
echo -e "${YELLOW}🚀 尝试编译测试...${NC}"
if go build -o /tmp/test-build ./demo_cluster/nodes/; then
    echo -e "${GREEN}✅ 编译成功！${NC}"
    rm -f /tmp/test-build
else
    echo -e "${RED}❌ 编译仍有问题，需要进一步调试${NC}"
fi