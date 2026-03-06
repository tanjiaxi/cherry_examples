#!/bin/bash

# Docker 集群监控脚本
# 用法: ./monitor-docker-cluster.sh [--stats|--logs|--health]

set -e

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
cd "$SCRIPT_DIR"

# 颜色定义
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
BLUE='\033[0;34m'
CYAN='\033[0;36m'
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

# 显示容器状态
show_container_status() {
    echo ""
    echo -e "${CYAN}========== 容器状态 ==========${NC}"
    docker-compose -f docker-compose-local.yml ps
    echo ""
}

# 显示资源使用
show_resource_usage() {
    echo ""
    echo -e "${CYAN}========== 资源使用 ==========${NC}"
    docker stats --no-stream
    echo ""
}

# 显示网络信息
show_network_info() {
    echo ""
    echo -e "${CYAN}========== 网络信息 ==========${NC}"
    
    log_info "检查 PostgreSQL 连接..."
    if docker exec demo-postgres pg_isready -U postgres &> /dev/null; then
        log_success "PostgreSQL 连接正常"
    else
        log_error "PostgreSQL 连接失败"
    fi
    
    log_info "检查 ETCD 连接..."
    if curl -s http://localhost:2379/version &> /dev/null; then
        log_success "ETCD 连接正常"
    else
        log_error "ETCD 连接失败"
    fi
    
    log_info "检查 NATS 连接..."
    if timeout 2 bash -c "echo 'PING' | nc localhost 4222" &> /dev/null; then
        log_success "NATS 连接正常"
    else
        log_error "NATS 连接失败"
    fi
    
    echo ""
}

# 显示日志
show_logs() {
    local service=$1
    
    if [ -z "$service" ]; then
        log_info "显示所有服务日志..."
        docker-compose -f docker-compose-local.yml logs -f
    else
        log_info "显示 $service 服务日志..."
        docker-compose -f docker-compose-local.yml logs -f "$service"
    fi
}

# 显示健康检查
show_health_check() {
    echo ""
    echo -e "${CYAN}========== 健康检查 ==========${NC}"
    
    # PostgreSQL 健康检查
    echo -e "${BLUE}PostgreSQL:${NC}"
    if docker exec demo-postgres pg_isready -U postgres &> /dev/null; then
        log_success "数据库连接正常"
        docker exec demo-postgres psql -U postgres -c "SELECT version();" | head -1
    else
        log_error "数据库连接失败"
    fi
    
    # ETCD 健康检查
    echo ""
    echo -e "${BLUE}ETCD:${NC}"
    if curl -s http://localhost:2379/version &> /dev/null; then
        log_success "ETCD 连接正常"
        curl -s http://localhost:2379/version | jq .
    else
        log_error "ETCD 连接失败"
    fi
    
    # NATS 健康检查
    echo ""
    echo -e "${BLUE}NATS:${NC}"
    if curl -s http://localhost:8222/varz &> /dev/null; then
        log_success "NATS 连接正常"
        curl -s http://localhost:8222/varz | jq '.connections, .subscriptions, .total_connections'
    else
        log_error "NATS 连接失败"
    fi
    
    echo ""
}

# 显示详细信息
show_detailed_info() {
    echo ""
    echo -e "${CYAN}========== 详细信息 ==========${NC}"
    
    # 容器详情
    echo -e "${BLUE}容器详情:${NC}"
    docker-compose -f docker-compose-local.yml ps -a
    
    # 网络详情
    echo ""
    echo -e "${BLUE}网络详情:${NC}"
    docker network inspect demo-cluster-network | jq '.[] | {Name, Containers}'
    
    # 卷详情
    echo ""
    echo -e "${BLUE}卷详情:${NC}"
    docker volume ls | grep demo
    
    echo ""
}

# 显示帮助
show_help() {
    cat << EOF
Docker 集群监控脚本

用法: $0 [选项]

选项:
  --status      显示容器状态和资源使用
  --stats       显示实时资源使用（持续监控）
  --logs        显示所有服务日志（持续监控）
  --logs-pg     显示 PostgreSQL 日志
  --logs-etcd   显示 ETCD 日志
  --logs-nats   显示 NATS 日志
  --health      显示健康检查信息
  --detailed    显示详细信息
  --help        显示此帮助信息

示例:
  $0 --status           # 显示状态
  $0 --stats            # 实时监控资源
  $0 --logs             # 查看所有日志
  $0 --logs-pg          # 查看 PostgreSQL 日志
  $0 --health           # 健康检查
  $0 --detailed         # 详细信息

EOF
}

# 主函数
main() {
    # 检查是否有运行中的容器
    if ! docker-compose -f docker-compose-local.yml ps | grep -q "Up"; then
        log_error "没有运行中的容器"
        log_info "请先运行: ./start-docker-cluster.sh"
        exit 1
    fi
    
    # 解析参数
    case "${1:-}" in
        --status)
            show_container_status
            show_resource_usage
            show_network_info
            ;;
        --stats)
            docker stats
            ;;
        --logs)
            show_logs
            ;;
        --logs-pg)
            show_logs postgres
            ;;
        --logs-etcd)
            show_logs etcd
            ;;
        --logs-nats)
            show_logs nats
            ;;
        --health)
            show_health_check
            ;;
        --detailed)
            show_container_status
            show_resource_usage
            show_network_info
            show_detailed_info
            ;;
        --help|-h)
            show_help
            ;;
        *)
            # 默认显示状态
            show_container_status
            show_resource_usage
            show_network_info
            show_health_check
            ;;
    esac
}

# 错误处理
trap 'log_error "脚本执行失败"; exit 1' ERR

# 执行主函数
main "$@"
