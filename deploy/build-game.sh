#!/bin/bash
###
 # @Author: t 921865806@qq.com
 # @Date: 2026-01-03 21:43:05
 # @LastEditors: t 921865806@qq.com
 # @LastEditTime: 2026-01-16 16:02:13
 # @FilePath: /examples/deploy/build-game.sh
 # @Description: 这是默认设置,请设置`customMade`, 打开koroFileHeader查看配置 进行设置: https://github.com/OBKoro1/koro1FileHeader/wiki/%E9%85%8D%E7%BD%AE
### 
# Cherry 游戏服务器 - 构建脚本

set -e

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
PROJECT_ROOT="$(dirname "$SCRIPT_DIR")"
OUTPUT="$PROJECT_ROOT/demo_cluster/nodes/io_sql"

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
