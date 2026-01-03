#!/bin/bash
# Cherry 游戏服务器 - 停止脚本

echo "🛑 停止所有游戏服务节点..."

pkill -f "game-server" 2>/dev/null || true

sleep 1

# 检查是否还有进程
if pgrep -f "game-server" > /dev/null; then
    echo "⚠️  部分进程未停止，强制终止..."
    pkill -9 -f "game-server" 2>/dev/null || true
fi

echo "✅ 所有节点已停止"
