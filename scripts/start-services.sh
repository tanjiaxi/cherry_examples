#!/bin/bash
###
 # @Author: t 921865806@qq.com
 # @Date: 2026-01-03 21:33:18
 # @LastEditors: t 921865806@qq.com
 # @LastEditTime: 2026-01-03 21:36:15
 # @FilePath: /examples/scripts/start-services.sh
 # @Description: 这是默认设置,请设置`customMade`, 打开koroFileHeader查看配置 进行设置: https://github.com/OBKoro1/koro1FileHeader/wiki/%E9%85%8D%E7%BD%AE
### 
# Cherry 游戏服务器 - 启动基础服务 (ETCD + NATS)

set -e

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
PROJECT_ROOT="$(dirname "$SCRIPT_DIR")"
DOCKER_DIR="$PROJECT_ROOT/docker"

echo "=========================================="
echo "  Cherry 游戏服务器 - 基础服务启动脚本"
echo "=========================================="

# 检查 Docker 是否运行
if ! docker info > /dev/null 2>&1; then
    echo "❌ Docker 未运行，请先启动 Docker"
    exit 1
fi

# 进入 docker 目录
cd "$DOCKER_DIR"

# 启动服务
echo ""
echo "🚀 启动 ETCD 和 NATS 服务..."
docker-compose up -d

# 等待服务就绪
echo ""
echo "⏳ 等待服务就绪..."
sleep 3

# 检查服务状态
echo ""
echo "📊 服务状态:"
docker-compose ps

# 检查 ETCD
echo ""
echo "🔍 检查 ETCD 连接..."
if docker exec cherry-etcd etcdctl endpoint health 2>/dev/null; then
    echo "✅ ETCD 服务正常"
else
    echo "⚠️  ETCD 服务可能还在启动中..."
fi

# 检查 NATS
echo ""
echo "🔍 检查 NATS 连接..."
if curl -s http://127.0.0.1:8222/healthz > /dev/null 2>&1; then
    echo "✅ NATS 服务正常"
else
    echo "⚠️  NATS 服务可能还在启动中..."
fi

echo ""
echo "=========================================="
echo "  服务端口信息:"
echo "  - ETCD:  127.0.0.1:2379"
echo "  - NATS:  127.0.0.1:4222"
echo "  - NATS Monitor: http://127.0.0.1:8222"
echo "=========================================="
echo ""
echo "✅ 基础服务启动完成！"
echo ""
echo "💡 提示: 现在可以在 VSCode 中运行 '🚀 启动所有单节点 (Debug)'"
