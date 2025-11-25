#!/bin/bash

# Cherry框架 Protobuf版本统一脚本

set -e

echo "🔧 统一Cherry框架Protobuf版本"
echo "=============================="

# 颜色定义
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
BLUE='\033[0;34m'
NC='\033[0m'

echo -e "${BLUE}📋 检查当前protobuf版本...${NC}"
echo "修复前的版本："
go list -m all | grep protobuf || true

echo ""
echo -e "${YELLOW}🔄 应用版本统一策略...${NC}"

# 检查go.mod是否已有replace指令
if grep -q "replace (" go.mod; then
    echo "✅ go.mod已包含replace指令"
else
    echo "❌ 需要手动添加replace指令到go.mod"
    echo ""
    echo "请在go.mod中添加以下内容："
    echo ""
    echo "// 统一protobuf版本，解决冲突"
    echo "replace ("
    echo "    google.golang.org/protobuf => google.golang.org/protobuf v1.31.0"
    echo "    github.com/golang/protobuf => github.com/golang/protobuf v1.5.4"
    echo ")"
    exit 1
fi

echo -e "${YELLOW}🧹 清理模块缓存...${NC}"
go clean -modcache

echo -e "${YELLOW}📦 重新下载依赖...${NC}"
go mod download

echo -e "${YELLOW}🔧 整理依赖...${NC}"
go mod tidy

echo -e "${YELLOW}✅ 验证依赖...${NC}"
go mod verify

echo ""
echo -e "${BLUE}📋 修复后的protobuf版本：${NC}"
go list -m all | grep protobuf

echo ""
echo -e "${YELLOW}🧪 测试编译...${NC}"
if go build -o /tmp/test-cherry ./demo_cluster/nodes/; then
    echo -e "${GREEN}✅ 编译成功！protobuf冲突已解决${NC}"
    rm -f /tmp/test-cherry
else
    echo -e "${RED}❌ 编译失败，需要进一步调试${NC}"
    exit 1
fi

echo ""
echo -e "${GREEN}🎉 Protobuf版本统一完成！${NC}"
echo -e "${BLUE}💡 现在可以正常使用etcd组件了${NC}"