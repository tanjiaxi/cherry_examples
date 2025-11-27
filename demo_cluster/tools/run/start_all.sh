#!/bin/bash
###
 # @Author: t 921865806@qq.com
 # @Date: 2025-11-26 10:43:14
 # @LastEditors: t 921865806@qq.com
 # @LastEditTime: 2025-11-26 10:48:48
 # @FilePath: /examples/demo_cluster/start_all.sh
 # @Description: 这是默认设置,请设置`customMade`, 打开koroFileHeader查看配置 进行设置: https://github.com/OBKoro1/koro1FileHeader/wiki/%E9%85%8D%E7%BD%AE
### 

# 一键启动所有游戏集群节点
# 启动顺序：center -> web -> gate -> game

set -e

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
cd "$SCRIPT_DIR"

CONFIG_PATH="../../../config/demo-cluster.json"
MAIN_GO="../../nodes/main.go"

# 颜色定义
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
BLUE='\033[0;34m'
NC='\033[0m' # No Color

echo -e "${GREEN}========================================${NC}"
echo -e "${GREEN}🚀 启动游戏集群所有节点${NC}"
echo -e "${GREEN}========================================${NC}"
echo ""

# 检查配置文件
if [ ! -f "$CONFIG_PATH" ]; then
    echo -e "${RED}❌ 配置文件不存在: $CONFIG_PATH${NC}"
    exit 1
fi

# 检查 main.go
if [ ! -f "$MAIN_GO" ]; then
    echo -e "${RED}❌ 主程序不存在: $MAIN_GO${NC}"
    exit 1
fi

# 创建日志目录
mkdir -p logs

# 清理旧的日志文件（可选）
# rm -f logs/*.log

echo -e "${BLUE}📋 启动节点列表:${NC}"
echo "  1. gc-center  (中心节点)"
echo "  2. gc-web-1   (Web节点)"
echo "  3. gc-gate-1  (网关节点)"
echo "  4. gc-game-10001 (游戏节点)"
echo ""

# 启动函数
start_node() {
    local node_type=$1
    local node_name=$2
    local log_file="logs/${node_name}.log"
    
    echo -e "${YELLOW}▶️  启动 ${node_name}...${NC}"
    
    # 后台启动并重定向输出到日志文件
    nohup go run "$MAIN_GO" "$node_type" --path="$CONFIG_PATH" --node="$node_name" > "$log_file" 2>&1 &
    
    local pid=$!
    echo "$pid" > "logs/${node_name}.pid"
    
    echo -e "${GREEN}✅ ${node_name} 已启动 (PID: $pid)${NC}"
    echo "   日志文件: $log_file"
    echo ""
    
    # 等待一下，确保节点启动
    sleep 2
}

# 按顺序启动节点
start_node "center" "gc-center"
start_node "web" "gc-web-1"
start_node "gate" "gc-gate-1"
start_node "game" "10001"

echo -e "${GREEN}========================================${NC}"
echo -e "${GREEN}✅ 所有节点启动完成！${NC}"
echo -e "${GREEN}========================================${NC}"
echo ""
echo -e "${BLUE}📊 节点状态:${NC}"
ps aux | grep "go run.*main.go" | grep -v grep || echo "  (使用 ps 命令查看进程)"
echo ""
echo -e "${BLUE}💡 常用命令:${NC}"
echo "  查看日志: tail -f logs/gc-center.log"
echo "  停止所有: ./stop_all.sh"
echo "  查看进程: ps aux | grep 'go run.*main.go'"
echo ""
echo -e "${BLUE}🌐 访问地址:${NC}"
echo "  Web界面: http://localhost:8080"
echo ""
