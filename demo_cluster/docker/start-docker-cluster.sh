#!/bin/bash

# 启动 Docker 集群脚本
# 用法: ./start-docker-cluster.sh

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

# 检查依赖
check_dependencies() {
    log_info "检查依赖..."
    
    if ! command -v docker &> /dev/null; then
        log_error "Docker 未安装"
        exit 1
    fi
    
    if ! command -v docker-compose &> /dev/null; then
        log_error "Docker Compose 未安装"
        exit 1
    fi
    
    if [ ! -f "game-server" ]; then
        log_error "game-server 二进制文件不存在"
        log_info "请先编译: cd .. && go build -o docker/game-server ./nodes/main.go"
        exit 1
    fi
    
    if [ ! -f "docker-compose-local.yml" ]; then
        log_error "docker-compose-local.yml 文件不存在"
        exit 1
    fi
    
    log_success "依赖检查完成"
}

# 启动基础服务
start_base_services() {
    log_info "启动基础服务 (PostgreSQL, ETCD, NATS)..."
    
    docker-compose -f docker-compose-local.yml up -d
    
    log_info "等待服务就绪..."
    sleep 5
    
    # 检查服务状态
    local max_retries=30
    local retry=0
    
    while [ $retry -lt $max_retries ]; do
        if docker-compose -f docker-compose-local.yml ps | grep -q "Up"; then
            log_success "基础服务已启动"
            return 0
        fi
        retry=$((retry + 1))
        sleep 1
    done
    
    log_error "基础服务启动超时"
    docker-compose -f docker-compose-local.yml logs
    exit 1
}

# 验证基础服务
verify_base_services() {
    log_info "验证基础服务连接..."
    
    # 检查 PostgreSQL
    if ! docker exec demo-postgres pg_isready -U postgres &> /dev/null; then
        log_warn "PostgreSQL 未就绪，继续等待..."
        sleep 5
    else
        log_success "PostgreSQL 连接正常"
    fi
    
    # 检查 ETCD
    if ! curl -s http://localhost:2379/version &> /dev/null; then
        log_warn "ETCD 未就绪，继续等待..."
        sleep 5
    else
        log_success "ETCD 连接正常"
    fi
    
    # 检查 NATS
    if ! timeout 2 bash -c "echo 'PING' | nc localhost 4222" &> /dev/null; then
        log_warn "NATS 未就绪，继续等待..."
        sleep 5
    else
        log_success "NATS 连接正常"
    fi
}

# 显示启动信息
show_startup_info() {
    echo ""
    echo -e "${GREEN}========================================${NC}"
    echo -e "${GREEN}Docker 集群启动完成${NC}"
    echo -e "${GREEN}========================================${NC}"
    echo ""
    echo "基础服务状态:"
    docker-compose -f docker-compose-local.yml ps
    echo ""
    echo "服务地址:"
    echo -e "  ${BLUE}Web 服务${NC}:      http://localhost:3013"
    echo -e "  ${BLUE}ETCD${NC}:          http://localhost:2379"
    echo -e "  ${BLUE}NATS${NC}:          nats://localhost:4222"
    echo -e "  ${BLUE}PostgreSQL${NC}:    localhost:5432"
    echo ""
    echo "下一步: 在不同的终端启动游戏节点"
    echo ""
    echo "  终端 1 - Center 节点:"
    echo "    cd $SCRIPT_DIR"
    echo "    ./game-server center --path=../config/demo-cluster.json --node=gc-center-1"
    echo ""
    echo "  终端 2 - Gate 节点:"
    echo "    cd $SCRIPT_DIR"
    echo "    ./game-server gate --path=../config/demo-cluster.json --node=gc-gate-1"
    echo ""
    echo "  终端 3 - Game 节点:"
    echo "    cd $SCRIPT_DIR"
    echo "    ./game-server game --path=../config/demo-cluster.json --node=gc-game-1"
    echo ""
    echo "  终端 4 - Web 节点:"
    echo "    cd $SCRIPT_DIR"
    echo "    ./game-server web --path=../config/demo-cluster.json --node=gc-web-1"
    echo ""
    echo "停止集群: ./stop-docker-cluster.sh"
    echo "查看日志: docker-compose -f docker-compose-local.yml logs -f"
    echo "监控资源: docker stats"
    echo ""
}

# 主函数
main() {
    echo -e "${BLUE}========================================${NC}"
    echo -e "${BLUE}Docker 集群启动脚本${NC}"
    echo -e "${BLUE}========================================${NC}"
    echo ""
    
    check_dependencies
    start_base_services
    verify_base_services
    show_startup_info
}

# 错误处理
trap 'log_error "脚本执行失败"; exit 1' ERR

# 执行主函数
main "$@"
