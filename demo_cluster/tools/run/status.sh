#!/bin/bash

# 查看所有游戏集群节点的状态

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
cd "$SCRIPT_DIR"

# 颜色定义
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
BLUE='\033[0;34m'
NC='\033[0m' # No Color

echo -e "${BLUE}========================================${NC}"
echo -e "${BLUE}📊 游戏集群节点状态${NC}"
echo -e "${BLUE}========================================${NC}"
echo ""

# 检查节点状态
check_node() {
    local node_name=$1
    local pid_file="logs/${node_name}.pid"
    
    printf "%-20s" "$node_name:"
    
    if [ -f "$pid_file" ]; then
        local pid=$(cat "$pid_file")
        if ps -p "$pid" > /dev/null 2>&1; then
            echo -e "${GREEN}✅ 运行中${NC} (PID: $pid)"
            # 显示内存和CPU使用情况
            ps -p "$pid" -o %cpu,%mem,etime | tail -n 1 | awk '{printf "   CPU: %s%%, MEM: %s%%, 运行时间: %s\n", $1, $2, $3}'
        else
            echo -e "${RED}❌ 已停止${NC} (PID 文件存在但进程不存在)"
        fi
    else
        echo -e "${YELLOW}⚪ 未启动${NC} (无 PID 文件)"
    fi
}

# 检查所有节点
check_node "gc-center"
check_node "gc-web-1"
check_node "gc-gate-1"
check_node "10001"

echo ""
echo -e "${BLUE}📝 日志文件:${NC}"
if [ -d "logs" ]; then
    ls -lh logs/*.log 2>/dev/null | awk '{printf "  %s  %s\n", $9, $5}' || echo "  (无日志文件)"
else
    echo "  (logs 目录不存在)"
fi

echo ""
echo -e "${BLUE}🔍 所有相关进程:${NC}"
ps aux | grep "go run.*demo_cluster/nodes/main.go" | grep -v grep | awk '{printf "  PID: %-8s CPU: %-6s MEM: %-6s CMD: %s\n", $2, $3"%", $4"%", substr($0, index($0,$11))}'

if [ $? -ne 0 ]; then
    echo "  (无运行中的进程)"
fi

echo ""
echo -e "${BLUE}💡 常用命令:${NC}"
echo "  启动所有: ./start_all.sh"
echo "  停止所有: ./stop_all.sh"
echo "  查看日志: tail -f logs/gc-center.log"
echo "  重启所有: ./stop_all.sh && ./start_all.sh"
echo ""
