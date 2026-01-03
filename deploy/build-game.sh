#!/bin/bash
# Cherry 游戏服务器 - 构建脚本

set -e

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
PROJECT_ROOT="$(dirname "$SCRIPT_DIR")"
OUTPUT="$PROJECT_ROOT/demo_cluster/nodes/game-server"

echo "=========================================="
echo "  Cherry 游戏服务器 - 构建"
echo "=========================================="

cd "$PROJECT_ROOT"

# 构建
echo "📦 编译中..."
go build -mod=vendor -ldflags="-s -w" -o "$OUTPUT" ./demo_cluster/nodes/main.go

echo ""
echo "✅ 构建完成: $OUTPUT"
echo ""
echo "下一步: ./start-game.sh"
