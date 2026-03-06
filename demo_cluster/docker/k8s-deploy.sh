#!/bin/bash

set -e

# 颜色定义
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
BLUE='\033[0;34m'
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

print_step() {
    echo -e "${BLUE}[STEP]${NC} $1"
}

# 检查命令是否存在
command_exists() {
    command -v "$1" >/dev/null 2>&1
}

# 检查 K8s 集群
check_cluster() {
    print_step "检查 Kubernetes 集群..."
    
    if ! command_exists kubectl; then
        print_error "kubectl 未安装"
        exit 1
    fi
    
    if ! kubectl cluster-info &>/dev/null; then
        print_warn "Kubernetes 集群未运行"
        
        if command_exists minikube; then
            print_info "启动 Minikube..."
            minikube start --cpus=4 --memory=8192 --disk-size=50g
        else
            print_error "请安装 Minikube 或连接到 K8s 集群"
            exit 1
        fi
    fi
    
    print_info "Kubernetes 集群已就绪"
    kubectl cluster-info
}

# 构建镜像
build_image() {
    print_step "构建 Docker 镜像..."
    
    if ! command_exists docker; then
        print_error "docker 未安装"
        exit 1
    fi
    
    cd ..
    docker build -f docker/Dockerfile -t demo-cluster:latest .
    cd docker
    
    # 如果使用 Minikube，加载镜像
    if command_exists minikube; then
        print_info "加载镜像到 Minikube..."
        minikube image load demo-cluster:latest
    fi
    
    print_info "镜像构建完成"
}

# 部署
deploy() {
    print_step "开始 Kubernetes 部署..."
    
    # 1. 创建命名空间
    print_info "创建命名空间..."
    kubectl apply -f k8s-namespace.yaml
    
    # 2. 部署 PostgreSQL
    print_info "部署 PostgreSQL..."
    kubectl apply -f k8s-postgres.yaml
    
    # 3. 部署 ETCD
    print_info "部署 ETCD..."
    kubectl apply -f k8s-etcd.yaml
    
    # 4. 部署 NATS
    print_info "部署 NATS..."
    kubectl apply -f k8s-nats.yaml
    
    # 5. 等待基础服务就绪
    print_info "等待基础服务就绪..."
    print_warn "这可能需要 1-2 分钟..."
    
    kubectl wait --for=condition=ready pod -l app=postgres -n demo-cluster --timeout=300s || true
    kubectl wait --for=condition=ready pod -l app=etcd -n demo-cluster --timeout=300s || true
    kubectl wait --for=condition=ready pod -l app=nats -n demo-cluster --timeout=300s || true
    
    sleep 5
    
    # 6. 部署游戏节点
    print_info "部署游戏节点..."
    kubectl apply -f k8s-game-nodes.yaml
    
    # 7. 等待游戏节点就绪
    print_info "等待游戏节点就绪..."
    kubectl wait --for=condition=ready pod -l app=center -n demo-cluster --timeout=300s || true
    kubectl wait --for=condition=ready pod -l app=gate -n demo-cluster --timeout=300s || true
    kubectl wait --for=condition=ready pod -l app=game -n demo-cluster --timeout=300s || true
    kubectl wait --for=condition=ready pod -l app=web -n demo-cluster --timeout=300s || true
    
    print_info "部署完成！"
}

# 显示状态
show_status() {
    print_step "显示部署状态..."
    
    echo ""
    print_info "Pod 状态："
    kubectl get pods -n demo-cluster
    
    echo ""
    print_info "服务状态："
    kubectl get svc -n demo-cluster
    
    echo ""
    print_info "PVC 状态："
    kubectl get pvc -n demo-cluster
}

# 显示访问信息
show_access_info() {
    print_step "访问信息..."
    
    echo ""
    echo "=== 端口转发命令 ==="
    echo "# Web 服务"
    echo "kubectl port-forward svc/web 3013:3013 -n demo-cluster"
    echo ""
    echo "# Gate 服务"
    echo "kubectl port-forward svc/gate 3011:3011 -n demo-cluster"
    echo ""
    echo "# PostgreSQL"
    echo "kubectl port-forward svc/postgres 5432:5432 -n demo-cluster"
    echo ""
    echo "# ETCD"
    echo "kubectl port-forward svc/etcd 2379:2379 -n demo-cluster"
    echo ""
    echo "# NATS"
    echo "kubectl port-forward svc/nats 4222:4222 -n demo-cluster"
    echo ""
    
    echo "=== 查看日志 ==="
    echo "# 查看所有日志"
    echo "kubectl logs -f <pod-name> -n demo-cluster"
    echo ""
    echo "# 查看特定服务日志"
    echo "kubectl logs -f -l app=web -n demo-cluster"
    echo ""
    
    echo "=== 常用命令 ==="
    echo "# 查看 Pod 详情"
    echo "kubectl describe pod <pod-name> -n demo-cluster"
    echo ""
    echo "# 进入 Pod"
    echo "kubectl exec -it <pod-name> -n demo-cluster -- sh"
    echo ""
    echo "# 查看资源使用"
    echo "kubectl top pods -n demo-cluster"
    echo ""
    echo "# 查看事件"
    echo "kubectl get events -n demo-cluster"
    echo ""
}

# 清理部署
cleanup() {
    print_warn "清理 Kubernetes 部署..."
    kubectl delete namespace demo-cluster --ignore-not-found=true
    print_info "清理完成"
}

# 显示帮助
show_help() {
    cat << EOF
用法: $0 [命令]

命令:
  check       检查 K8s 集群
  build       构建 Docker 镜像
  deploy      完整部署（包括检查、构建、部署）
  status      显示部署状态
  access      显示访问信息
  clean       清理部署
  help        显示此帮助信息

示例:
  $0 check        # 检查集群
  $0 build        # 构建镜像
  $0 deploy       # 完整部署
  $0 status       # 查看状态
  $0 access       # 显示访问信息
  $0 clean        # 清理部署

快速开始:
  $0 deploy       # 一键部署
  kubectl port-forward svc/web 3013:3013 -n demo-cluster  # 访问 Web
EOF
}

# 主程序
case "${1:-help}" in
    check)
        check_cluster
        ;;
    build)
        build_image
        ;;
    deploy)
        check_cluster
        build_image
        deploy
        show_status
        show_access_info
        ;;
    status)
        show_status
        ;;
    access)
        show_access_info
        ;;
    clean)
        cleanup
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
