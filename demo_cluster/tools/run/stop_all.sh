#!/bin/bash
###
 # @Author: t 921865806@qq.com
 # @Date: 2025-11-26 10:43:55
 # @LastEditors: t 921865806@qq.com
 # @LastEditTime: 2025-11-26 11:09:04
 # @FilePath: /examples/demo_cluster/tools/run/stop_all.sh
 # @Description: 这是默认设置,请设置`customMade`, 打开koroFileHeader查看配置 进行设置: https://github.com/OBKoro1/koro1FileHeader/wiki/%E9%85%8D%E7%BD%AE
### 

# 停止所有游戏集群节点

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
cd "$SCRIPT_DIR"

# 颜色定义
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
BLUE='\033[0;34m'
NC='\033[0m' # No Color

echo -e "${YELLOW}========================================${NC}"
echo -e "${YELLOW}🛑 停止所有游戏集群节点${NC}"
echo -e "${YELLOW}========================================${NC}"
echo ""

# 停止函数
stop_node() {
    local node_name=$1
    local pid_file="logs/${node_name}.pid"
    
    if [ -f "$pid_file" ]; then
        local pid=$(cat "$pid_file")
        if ps -p "$pid" > /dev/null 2>&1; then
            echo -e "${YELLOW}⏹️  停止 ${node_name} (PID: $pid)...${NC}"
            kill "$pid"
            sleep 1
            
            # 如果进程还在，强制杀死
            if ps -p "$pid" > /dev/null 2>&1; then
                echo -e "${RED}   强制停止 ${node_name}...${NC}"
                kill -9 "$pid"
            fi
            
            echo -e "${GREEN}✅ ${node_name} 已停止${NC}"
        else
            echo -e "${BLUE}ℹ️  ${node_name} 未运行 (PID: $pid 不存在)${NC}"
        fi
        rm -f "$pid_file"
    else
        echo -e "${BLUE}ℹ️  ${node_name} PID 文件不存在${NC}"
    fi
}

# 按相反顺序停止节点（先停游戏节点，最后停中心节点）
stop_node "10001"
stop_node "gc-gate-1"
stop_node "gc-web-1"
stop_node "gc-center"

echo ""
echo -e "${YELLOW}🔍 检查残留进程...${NC}"

# 查找并杀死所有相关的 go run 进程
pkill -f "go run.*demo_cluster/nodes/main.go" && echo -e "${GREEN}✅ 清理了残留进程${NC}" || echo -e "${BLUE}ℹ️  没有残留进程${NC}"

echo ""
echo -e "${GREEN}========================================${NC}"
echo -e "${GREEN}✅ 所有节点已停止${NC}"
echo -e "${GREEN}========================================${NC}"
echo ""
