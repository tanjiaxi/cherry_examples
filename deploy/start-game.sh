#!/bin/bash
###
 # @Author: t 921865806@qq.com
 # @Date: 2026-01-03 21:42:58
 # @LastEditors: t 921865806@qq.com
 # @LastEditTime: 2026-01-03 21:43:08
 # @FilePath: /examples/deploy/start-game.sh
 # @Description: 这是默认设置,请设置`customMade`, 打开koroFileHeader查看配置 进行设置: https://github.com/OBKoro1/koro1FileHeader/wiki/%E9%85%8D%E7%BD%AE
### 
# Cherry 游戏服务器 - 启动脚本
# 启动单节点集群: center, web, gate, game

set -e

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
PROJECT_ROOT="$(dirname "$SCRIPT_DIR")"
CONFIG_PATH="$PROJECT_ROOT/config/demo-cluster.json"
BINARY="$PROJECT_ROOT/demo_cluster/nodes/game-server"
LOG_DIR="$PROJECT_ROOT/logs"

# 创建日志目录
mkdir -p "$LOG_DIR"

# 检查二进制文件
if [ ! -f "$BINARY" ]; then
    echo "❌ 二进制文件不存在: $BINARY"
    echo "   请先运行: ./build-game.sh"
    exit 1
fi

# 检查配置文件
if [ ! -f "$CONFIG_PATH" ]; then
    echo "❌ 配置文件不存在: $CONFIG_PATH"
    exit 1
fi

echo "=========================================="
echo "  Cherry 游戏服务器 - 启动单节点集群"
echo "=========================================="
echo "配置文件: $CONFIG_PATH"
echo "日志目录: $LOG_DIR"
echo ""

# 启动函数
start_node() {
    local node_type=$1
    local node_id=$2
    local log_file="$LOG_DIR/${node_type}.log"
    
    echo "🚀 启动 $node_type ($node_id)..."
    nohup "$BINARY" "$node_type" --path="$CONFIG_PATH" --node="$node_id" > "$log_file" 2>&1 &
    echo "   PID: $!, 日志: $log_file"
}

# 按顺序启动节点
start_node "center" "gc-center"
sleep 2

start_node "web" "gc-web-1"
sleep 1

start_node "gate" "gc-gate-1"
sleep 1

start_node "game" "10001"

echo ""
echo "✅ 所有节点已启动！"
echo ""
echo "查看进程: ps aux | grep game-server"
echo "查看日志: tail -f $LOG_DIR/*.log"
echo "停止服务: ./stop-game.sh"
