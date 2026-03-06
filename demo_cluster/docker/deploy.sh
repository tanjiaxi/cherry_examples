#!/bin/bash

set -e

# 颜色定义
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
NC='\033[0m' # No Color

# 打印带颜色的消息
print_info() {
    echo -e "${GREEN}[INFO]${NC} $1"
}

print_warn() {
    echo -e "${YELLOW}[WARN]${NC} $1"
}

print_error() {
    echo -e "${RED}[ERROR]${NC} $1"
}

# 检查命令是否存在
command_exists() {
    command -v "$1" >/dev/null 2>&1
}

# 部署到 Docker Compose
deploy_docker_compose() {
    print_info "开始 Docker Compose 部署..."
    
    if ! command_exists docker-compose; then
        print_error "docker-compose 未安装"
        exit 1
    fi
    
    cd "$(dirname "$0")"
    
    print_info "启动所有服务..."
    docker-compose up -d
    
    print_info "等待服务就绪..."
    sleep 10
    
    print_info "检查服务状态..."
    docker-compose ps
    
    print_info "Docker Compose 部署完成！"
    print_info "访问 Web 服务: http://localhost:3013"
}

# 部署到 Kubernetes
deploy_kubernetes() {
    print_info "开始 Kubernetes 部署..."
    
    if ! command_exists kubectl; then
        print_error "kubectl 未安装"
        exit 1
    fi
    
    if ! command_exists docker; then
        print_error "docker 未安装"
        exit 1
    fi
    
    cd "$(dirname "$0")/.."
    
    # 构建镜像
    print_info "构建 Docker 镜像..."
    docker build -f docker/Dockerfile -t demo-cluster:latest .
    
    # 如果使用 Minikube，加载镜像
    if command_exists minikube; then
        print_info "加载镜像到 Minikube..."
        minikube image load demo-cluster:latest
    fi
    
    cd docker
    
    # 创建命名空间
    print_info "创建命名空间..."
    kubectl apply -f k8s-namespace.yaml
    
    # 部署基础服务
    print_info "部署 PostgreSQL..."
    kubectl apply -f k8s-postgres.yaml
    
    print_info "部署 ETCD..."
    kubectl apply -f k8s-etcd.yaml
    
    print_info "部署 NATS..."
    kubectl apply -f k8s-nats.yaml
    
    # 等待基础服务就绪
    print_info "等待基础服务就绪..."
    kubectl wait --for=condition=ready pod -l app=postgres -n demo-cluster --timeout=300s || true
    kubectl wait --for=condition=ready pod -l app=etcd -n demo-cluster --timeout=300s || true
    kubectl wait --for=condition=ready pod -l app=nats -n demo-cluster --timeout=300s || true
    
    sleep 5
    
    # 部署游戏节点
    print_info "部署游戏节点..."
    kubectl apply -f k8s-game-nodes.yaml
    
    print_info "等待游戏节点就绪..."
    kubectl wait --for=condition=ready pod -l app=center -n demo-cluster --timeout=300s || true
    kubectl wait --for=condition=ready pod -l app=gate -n demo-cluster --timeout=300s || true
    kubectl wait --for=condition=ready pod -l app=game -n demo-cluster --timeout=300s || true
    kubectl wait --for=condition=ready pod -l app=web -n demo-cluster --timeout=300s || true
    
    print_info "Kubernetes 部署完成！"
    print_info "查看 Pod 状态: kubectl get pods -n demo-cluster"
    print_info "查看服务: kubectl get svc -n demo-cluster"
}

# 清理部署
cleanup() {
    print_warn "清理部署..."
    
    if [ "$1" == "docker" ]; then
        cd "$(dirname "$0")"
        docker-compose down -v
        print_info "Docker Compose 清理完成"
    elif [ "$1" == "k8s" ]; then
        kubectl delete namespace demo-cluster --ignore-not-found=true
        print_info "Kubernetes 清理完成"
    fi
}

# 显示帮助
show_help() {
    cat << EOF
用法: $0 [命令]

命令:
  docker      部署到 Docker Compose
  k8s         部署到 Kubernetes
  clean-docker  清理 Docker Compose 部署
  clean-k8s     清理 Kubernetes 部署
  help        显示此帮助信息

示例:
  $0 docker       # 部署到 Docker Compose
  $0 k8s          # 部署到 Kubernetes
  $0 clean-docker # 清理 Docker Compose
EOF
}

# 主程序
case "${1:-help}" in
    docker)
        deploy_docker_compose
        ;;
    k8s)
        deploy_kubernetes
        ;;
    clean-docker)
        cleanup docker
        ;;
    clean-k8s)
        cleanup k8s
        ;;
    help)
        show_help
        ;;
    *)
        print_error "未知命令: $1"
        show_help
        exit 1
        ;;
esac
