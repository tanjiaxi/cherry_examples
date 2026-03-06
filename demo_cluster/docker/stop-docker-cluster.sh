#!/bin/bash

# 停止 Docker 集群脚本
# 用法: ./stop-docker-cluster.sh [--clean]

set -e

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
cd "$SCRIPT_DIR"

# 颜色定义
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
BLUE='\033[0;34m'
NC='\033[0m' # No Color

# 日志函数
log_info() {
    echo -e "${BLUE}[INFO]${NC} $1"
}

log_success() {
    echo -e "${GREEN}[SUCCESS]${NC} $1"
}

log_warn() {
    echo -e "${YELLOW}[WARN]${NC} $1"
}

log_error() {
    echo -e "${RED}[ERROR]${NC} $1"
}

# 停止容器
stop_containers() {
    log_info "停止容器..."
    
    if docker-compose -f docker-compose-local.yml ps | grep -q "Up"; then
        docker-compose -f docker-compose-local.yml stop
        log_success "容器已停止"
    else
        log_warn "没有运行中的容器"
    fi
}

# 移除容器
remove_containers() {
    log_info "移除容器..."
    docker-compose -f docker-compose-local.yml rm -f
    log_success "容器已移除"
}

# 清理数据卷
clean_volumes() {
    log_info "清理数据卷..."
    docker-compose -f docker-compose-local.yml down -v
    log_success "数据卷已清理"
}

# 显示停止信息
show_stop_info() {
    echo ""
    echo -e "${GREEN}========================================${NC}"
    echo -e "${GREEN}Docker 集群已停止${NC}"
    echo -e "${GREEN}========================================${NC}"
    echo ""
    
    if [ "$CLEAN_DATA" = "true" ]; then
        echo "数据已清理"
    else
        echo "数据已保留"
        echo "下次启动时将恢复之前的数据"
        echo ""
        echo "要清理数据，请运行: ./stop-docker-cluster.sh --clean"
    fi
    echo ""
}

# 主函数
main() {
    echo -e "${BLUE}========================================${NC}"
    echo -e "${BLUE}Docker 集群停止脚本${NC}"
    echo -e "${BLUE}========================================${NC}"
    echo ""
    
    # 解析参数
    CLEAN_DATA="false"
    if [ "$1" = "--clean" ] || [ "$1" = "-c" ]; then
        CLEAN_DATA="true"
    fi
    
    # 检查是否有运行中的容器
    if ! docker-compose -f docker-compose-local.yml ps | grep -q "Up"; then
        log_warn "没有运行中的容器"
        show_stop_info
        exit 0
    fi
    
    # 停止容器
    stop_containers
    
    # 移除容器
    remove_containers
    
    # 清理数据（如果指定了 --clean）
    if [ "$CLEAN_DATA" = "true" ]; then
        log_info "清理数据卷..."
        docker volume prune -f
        log_success "数据卷已清理"
    fi
    
    show_stop_info
}

# 错误处理
trap 'log_error "脚本执行失败"; exit 1' ERR

# 执行主函数
main "$@"
