#!/bin/bash
###
 # @Author: t 921865806@qq.com
 # @Date: 2026-01-03 21:43:04
 # @LastEditors: t 921865806@qq.com
 # @LastEditTime: 2026-01-16 16:03:37
 # @FilePath: /examples/deploy/stop-game.sh
 # @Description: 这是默认设置,请设置`customMade`, 打开koroFileHeader查看配置 进行设置: https://github.com/OBKoro1/koro1FileHeader/wiki/%E9%85%8D%E7%BD%AE
### 
# Cherry 游戏服务器 - 停止脚本

echo "🛑 停止所有游戏服务节点..."

pkill -f "io_sql" 2>/dev/null || true

sleep 1

# 检查是否还有进程
if pgrep -f "io_sql" > /dev/null; then
    echo "⚠️  部分进程未停止，强制终止..."
    pkill -9 -f "io_sql" 2>/dev/null || true
fi

echo "✅ 所有节点已停止"
