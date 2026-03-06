#!/bin/bash

# 快速 K8s 部署设置脚本
# 用于 macOS 环境

set -e

# 颜色定义
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
BLUE='\033[0;34m'
NC='\033[0m'

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

# 安装工具
install_tools() {
    print_step "检查并安装必要工具..."
    
    # 检查 Homebrew
    if ! command_exists brew; then
        print_warn "Homebrew 未安装，正在安装..."
        /bin/bash -c "$(curl -fsSL https://raw.githubusercontent.com/Homebrew/install/HEAD/install.sh)"
    fi
    
    # 检查 kubectl
    if ! command_exists kubectl; then
        print_warn "kubectl 未安装，正在安装..."
        brew install kubectl
    else
        print_info "kubectl 已安装: $(kubectl version --client --short)"
    fi
    
    # 检查 Minikube
    if ! command_exists minikube; then
        print_warn "Minikube 未安装，正在安装..."
        brew install minikube
    else
        print_info "Minikube 已安装: $(minikube version)"
    fi
    
    # 检查 Docker
    if ! command_exists docker; then
        print_error "Docker 未安装"
        print_info "请访问 https://www.docker.com/products/docker-desktop 安装 Docker Desktop"
        exit 1
    else
        print_info "Docker 已安装: $(docker --version)"
    fi
}

# 启动 Minikube
start_minikube() {
    print_step "启动 Minikube..."
    
    if minikube status | grep -q "Running"; then
        print_info "Minikube 已在运行"
    else
        print_warn "启动 Minikube（这可能需要几分钟）..."
        minikube start --cpus=4 --memory=8192 --disk-size=50g
    fi
    
    # 验证集群
    print_info "验证集群..."
    kubectl cluster-info
    kubectl get nodes
}

# 配置 Docker 环境
setup_docker_env() {
    print_step "配置 Docker 环境..."
    
    # 切换到 Minikube 的 Docker 环境
    eval $(minikube docker-env)
    print_info "已切换到 Minikube 的 Docker 环境"
}

# 构建镜像
build_image() {
    print_step "构建 Docker 镜像..."
    
    # 切换到 Minikube 的 Docker 环境
    eval $(minikube docker-env)
    
    # 构建镜像
    cd ..
    print_info "构建镜像（这可能需要几分钟）..."
    docker build -f docker/Dockerfile -t demo-cluster:latest .
    cd docker
    
    # 加载镜像到 Minikube
    print_info "加载镜像到 Minikube..."
    minikube image load demo-cluster:latest
    
    print_info "镜像构建完成"
}

# 部署
deploy() {
    print_step "部署到 Kubernetes..."
    
    sh k8s-deploy.sh deploy
}

# 显示访问信息
show_info() {
    print_step "部署完成！"
    
    echo ""
    echo "=== 快速访问 ==="
    echo ""
    echo "1. 在新终端运行以下命令进行端口转发："
    echo ""
    echo "   # Web 服务"
    echo "   kubectl port-forward svc/web 3013:3013 -n demo-cluster"
    echo ""
    echo "   # Gate 服务"
    echo "   kubectl port-forward svc/gate 3011:3011 -n demo-cluster"
    echo ""
    echo "2. 访问服务："
    echo "   - Web: http://localhost:3013"
    echo "   - Gate: localhost:3011"
    echo ""
    echo "3. 查看日志："
    echo "   kubectl logs -f -l app=web -n demo-cluster"
    echo ""
    echo "4. 查看 Pod 状态："
    echo "   kubectl get pods -n demo-cluster"
    echo ""
}

# 主程序
main() {
    print_info "开始 K8s 快速部署设置..."
    echo ""
    
    install_tools
    echo ""
    
    start_minikube
    echo ""
    
    setup_docker_env
    echo ""
    
    build_image
    echo ""
    
    deploy
    echo ""
    
    show_info
}

# 执行主程序
main
