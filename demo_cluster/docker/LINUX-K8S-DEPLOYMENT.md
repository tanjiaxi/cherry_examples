# Linux 上的 Kubernetes 部署指南

## 概述

本指南提供了在 Linux 系统上部署 demo_cluster 游戏服务器的完整步骤。

## 前置要求

### 系统要求

- **操作系统**：Ubuntu 20.04 LTS 或更高版本（推荐）、CentOS 8 或更高版本
- **CPU**：4 核或以上
- **内存**：8 GB 或以上
- **磁盘**：50 GB 或以上
- **网络**：正常的互联网连接

### 支持的 Linux 发行版

- ✅ Ubuntu 20.04 LTS / 22.04 LTS
- ✅ Debian 11 / 12
- ✅ CentOS 8 / 9
- ✅ RHEL 8 / 9
- ✅ Fedora 36+
- ✅ Rocky Linux 8 / 9

## 安装步骤

### 步骤 1：安装必要工具

#### 1.1 更新系统包

```bash
# Ubuntu/Debian
sudo apt-get update
sudo apt-get upgrade -y

# CentOS/RHEL
sudo yum update -y
```

#### 1.2 安装基础工具

```bash
# Ubuntu/Debian
sudo apt-get install -y \
  curl \
  wget \
  git \
  vim \
  net-tools \
  htop \
  apt-transport-https \
  ca-certificates \
  gnupg \
  lsb-release

# CentOS/RHEL
sudo yum install -y \
  curl \
  wget \
  git \
  vim \
  net-tools \
  htop \
  yum-utils
```

#### 1.3 安装 Docker

**Ubuntu/Debian：**

```bash
# 添加 Docker 官方 GPG 密钥
curl -fsSL https://download.docker.com/linux/ubuntu/gpg | sudo gpg --dearmor -o /usr/share/keyrings/docker-archive-keyring.gpg

# 添加 Docker 仓库
echo \
  "deb [arch=amd64 signed-by=/usr/share/keyrings/docker-archive-keyring.gpg] https://download.docker.com/linux/ubuntu \
  $(lsb_release -cs) stable" | sudo tee /etc/apt/sources.list.d/docker.list > /dev/null

# 安装 Docker
sudo apt-get update
sudo apt-get install -y docker-ce docker-ce-cli containerd.io docker-compose-plugin

# 启动 Docker
sudo systemctl start docker
sudo systemctl enable docker

# 添加当前用户到 docker 组（避免每次都用 sudo）
sudo usermod -aG docker $USER
newgrp docker
```

**CentOS/RHEL：**

```bash
# 添加 Docker 仓库
sudo yum-config-manager --add-repo https://download.docker.com/linux/centos/docker-ce.repo

# 安装 Docker
sudo yum install -y docker-ce docker-ce-cli containerd.io docker-compose-plugin

# 启动 Docker
sudo systemctl start docker
sudo systemctl enable docker

# 添加当前用户到 docker 组
sudo usermod -aG docker $USER
newgrp docker
```

#### 1.4 验证 Docker 安装

```bash
docker --version
docker run hello-world
```

#### 1.5 安装 kubectl

```bash
# 下载最新版本
curl -LO "https://dl.k8s.io/release/$(curl -L -s https://dl.k8s.io/release/stable.txt)/bin/linux/amd64/kubectl"

# 安装
sudo install -o root -g root -m 0755 kubectl /usr/local/bin/kubectl

# 验证
kubectl version --client
```

#### 1.6 安装 Minikube（用于本地开发）

```bash
# 下载 Minikube
curl -LO https://github.com/kubernetes/minikube/releases/latest/download/minikube-linux-amd64

# 安装
sudo install minikube-linux-amd64 /usr/local/bin/minikube

# 验证
minikube version
```

#### 1.7 安装 Helm（可选，用于包管理）

```bash
# 下载 Helm
curl https://raw.githubusercontent.com/helm/helm/main/scripts/get-helm-3 | bash

# 验证
helm version
```

### 步骤 2：启动 Kubernetes 集群

#### 2.1 使用 Minikube（本地开发）

```bash
# 启动 Minikube
minikube start \
  --cpus=4 \
  --memory=8192 \
  --disk-size=50g \
  --driver=docker

# 验证集群
kubectl cluster-info
kubectl get nodes
```

#### 2.2 使用现有 K8s 集群

如果已有 K8s 集群，配置 kubeconfig：

```bash
# 复制 kubeconfig 文件
mkdir -p ~/.kube
cp /path/to/kubeconfig ~/.kube/config

# 验证连接
kubectl cluster-info
kubectl get nodes
```

### 步骤 3：构建 Docker 镜像

```bash
# 进入项目目录
cd demo_cluster

# 构建镜像
docker build -f docker/Dockerfile -t demo-cluster:latest .

# 验证镜像
docker images | grep demo-cluster

# 如果使用 Minikube，加载镜像
minikube image load demo-cluster:latest
```

### 步骤 4：部署到 Kubernetes

```bash
cd docker

# 创建命名空间
kubectl apply -f k8s-namespace.yaml

# 部署基础服务
kubectl apply -f k8s-postgres.yaml
kubectl apply -f k8s-etcd.yaml
kubectl apply -f k8s-nats.yaml

# 等待基础服务就绪
kubectl wait --for=condition=ready pod -l app=postgres -n demo-cluster --timeout=300s
kubectl wait --for=condition=ready pod -l app=etcd -n demo-cluster --timeout=300s
kubectl wait --for=condition=ready pod -l app=nats -n demo-cluster --timeout=300s

# 部署游戏节点
kubectl apply -f k8s-game-nodes.yaml

# 等待游戏节点就绪
kubectl wait --for=condition=ready pod -l app=web -n demo-cluster --timeout=300s
```

### 步骤 5：验证部署

```bash
# 查看 Pod 状态
kubectl get pods -n demo-cluster

# 查看服务
kubectl get svc -n demo-cluster

# 查看存储
kubectl get pvc -n demo-cluster

# 查看资源使用
kubectl top pods -n demo-cluster
```

## 访问服务

### 方式 1：端口转发（推荐用于本地开发）

```bash
# Web 服务
kubectl port-forward svc/web 3013:3013 -n demo-cluster

# Gate 服务（新终端）
kubectl port-forward svc/gate 3011:3011 -n demo-cluster

# PostgreSQL（新终端）
kubectl port-forward svc/postgres 5432:5432 -n demo-cluster

# ETCD（新终端）
kubectl port-forward svc/etcd 2379:2379 -n demo-cluster

# NATS（新终端）
kubectl port-forward svc/nats 4222:4222 -n demo-cluster
```

然后访问：
- Web 服务: http://localhost:3013
- Gate 服务: localhost:3011
- PostgreSQL: localhost:5432
- ETCD: http://localhost:2379
- NATS: nats://localhost:4222

### 方式 2：使用 NodePort（如果集群支持）

```bash
# 获取 NodePort
kubectl get svc -n demo-cluster

# 获取节点 IP
kubectl get nodes -o wide

# 访问服务
# http://<node-ip>:<node-port>
```

### 方式 3：使用 Ingress（生产环境）

创建 `ingress.yaml`：

```yaml
apiVersion: networking.k8s.io/v1
kind: Ingress
metadata:
  name: demo-cluster-ingress
  namespace: demo-cluster
spec:
  rules:
  - host: demo-cluster.example.com
    http:
      paths:
      - path: /
        pathType: Prefix
        backend:
          service:
            name: web
            port:
              number: 3013
```

部署：

```bash
kubectl apply -f ingress.yaml
```

## 常用命令

### Pod 管理

```bash
# 查看 Pod
kubectl get pods -n demo-cluster

# 查看 Pod 详情
kubectl describe pod <pod-name> -n demo-cluster

# 查看 Pod 日志
kubectl logs -f <pod-name> -n demo-cluster

# 进入 Pod
kubectl exec -it <pod-name> -n demo-cluster -- bash

# 删除 Pod
kubectl delete pod <pod-name> -n demo-cluster
```

### 服务管理

```bash
# 查看服务
kubectl get svc -n demo-cluster

# 查看服务详情
kubectl describe svc <service-name> -n demo-cluster

# 查看端点
kubectl get endpoints -n demo-cluster
```

### 部署管理

```bash
# 查看部署
kubectl get deployments -n demo-cluster

# 扩展副本
kubectl scale deployment <deployment-name> --replicas=3 -n demo-cluster

# 更新镜像
kubectl set image deployment/<deployment-name> <container-name>=<image>:<tag> -n demo-cluster

# 查看部署历史
kubectl rollout history deployment/<deployment-name> -n demo-cluster

# 回滚部署
kubectl rollout undo deployment/<deployment-name> -n demo-cluster
```

### 存储管理

```bash
# 查看 PVC
kubectl get pvc -n demo-cluster

# 查看 PV
kubectl get pv

# 查看存储类
kubectl get storageclass
```

### 监控和调试

```bash
# 查看资源使用
kubectl top pods -n demo-cluster
kubectl top nodes

# 查看事件
kubectl get events -n demo-cluster

# 查看节点信息
kubectl get nodes -o wide

# 查看集群信息
kubectl cluster-info
```

## 故障排查

### 问题 1：Pod 无法启动

```bash
# 查看 Pod 状态
kubectl describe pod <pod-name> -n demo-cluster

# 查看 Pod 日志
kubectl logs <pod-name> -n demo-cluster

# 查看前一个容器的日志
kubectl logs <pod-name> -n demo-cluster --previous

# 常见原因：
# - 镜像不存在或无法拉取
# - 资源不足
# - 依赖服务未就绪
```

### 问题 2：服务无法连接

```bash
# 检查服务是否存在
kubectl get svc -n demo-cluster

# 检查端点
kubectl get endpoints -n demo-cluster

# 检查 Pod 是否运行
kubectl get pods -n demo-cluster

# 测试连接
kubectl exec -it <pod-name> -n demo-cluster -- bash
# 在容器内测试：
# nc -zv postgres 5432
# nc -zv etcd 2379
# nc -zv nats 4222
```

### 问题 3：资源不足

```bash
# 查看节点资源
kubectl describe nodes

# 查看 Pod 资源使用
kubectl top pods -n demo-cluster

# 增加节点资源或添加新节点
```

### 问题 4：镜像构建失败

```bash
# 检查网络连接
ping docker.io

# 清理 Docker 缓存
docker system prune -a

# 重新构建
docker build -f docker/Dockerfile -t demo-cluster:latest .
```

## 监控和管理

### 启动 Kubernetes Dashboard

```bash
# Minikube
minikube dashboard

# 其他集群
kubectl proxy
# 访问: http://localhost:8001/api/v1/namespaces/kubernetes-dashboard/services/https:kubernetes-dashboard:/proxy/
```

### 查看日志

```bash
# 查看 Pod 日志
kubectl logs -f <pod-name> -n demo-cluster

# 查看所有 Pod 日志
kubectl logs -f -l app=web -n demo-cluster

# 查看最后 100 行
kubectl logs --tail=100 <pod-name> -n demo-cluster

# 查看最后 1 小时的日志
kubectl logs --since=1h <pod-name> -n demo-cluster
```

### 查看资源使用

```bash
# 查看 Pod 资源使用
kubectl top pods -n demo-cluster

# 查看节点资源使用
kubectl top nodes

# 查看详细资源使用
kubectl describe node <node-name>
```

## 清理部署

### 删除所有资源

```bash
# 删除命名空间（会删除所有资源）
kubectl delete namespace demo-cluster

# 或逐个删除
kubectl delete deployment --all -n demo-cluster
kubectl delete service --all -n demo-cluster
kubectl delete pvc --all -n demo-cluster
```

### 停止 Minikube

```bash
minikube stop

# 删除 Minikube
minikube delete
```

## 性能优化

### 1. 资源限制

编辑部署配置，添加资源限制：

```yaml
resources:
  requests:
    memory: "256Mi"
    cpu: "250m"
  limits:
    memory: "512Mi"
    cpu: "500m"
```

### 2. 自动扩展

创建 HPA（Horizontal Pod Autoscaler）：

```yaml
apiVersion: autoscaling/v2
kind: HorizontalPodAutoscaler
metadata:
  name: web-hpa
  namespace: demo-cluster
spec:
  scaleTargetRef:
    apiVersion: apps/v1
    kind: Deployment
    name: web
  minReplicas: 2
  maxReplicas: 10
  metrics:
  - type: Resource
    resource:
      name: cpu
      target:
        type: Utilization
        averageUtilization: 70
```

### 3. 存储优化

使用高性能存储类：

```bash
# 查看可用的存储类
kubectl get storageclass

# 创建自定义存储类
kubectl apply -f - <<EOF
apiVersion: storage.k8s.io/v1
kind: StorageClass
metadata:
  name: fast-ssd
provisioner: kubernetes.io/no-provisioner
volumeBindingMode: WaitForFirstConsumer
EOF
```

## 生产环境建议

1. **使用专业 K8s 集群**：AWS EKS、Google GKE、Azure AKS 等
2. **配置持久化存储**：使用云提供商的存储服务
3. **设置监控告警**：使用 Prometheus + Grafana
4. **配置日志收集**：使用 ELK Stack 或云提供商的日志服务
5. **实施备份策略**：定期备份数据库和配置
6. **使用 Ingress**：配置 Ingress 进行流量管理
7. **配置 HPA**：使用 Horizontal Pod Autoscaler 自动扩展
8. **安全加固**：配置 RBAC、网络策略等

## 快速参考

### 一键部署脚本

创建 `linux-k8s-deploy.sh`：

```bash
#!/bin/bash

set -e

# 颜色定义
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
NC='\033[0m'

echo -e "${GREEN}[INFO]${NC} 开始 Linux K8s 部署..."

# 检查 Docker
if ! command -v docker &> /dev/null; then
    echo -e "${YELLOW}[WARN]${NC} Docker 未安装，请先安装 Docker"
    exit 1
fi

# 检查 kubectl
if ! command -v kubectl &> /dev/null; then
    echo -e "${YELLOW}[WARN]${NC} kubectl 未安装，请先安装 kubectl"
    exit 1
fi

# 构建镜像
echo -e "${GREEN}[INFO]${NC} 构建 Docker 镜像..."
cd ..
docker build -f docker/Dockerfile -t demo-cluster:latest .
cd docker

# 部署
echo -e "${GREEN}[INFO]${NC} 部署到 Kubernetes..."
kubectl apply -f k8s-namespace.yaml
kubectl apply -f k8s-postgres.yaml
kubectl apply -f k8s-etcd.yaml
kubectl apply -f k8s-nats.yaml
kubectl apply -f k8s-game-nodes.yaml

# 等待就绪
echo -e "${GREEN}[INFO]${NC} 等待服务就绪..."
kubectl wait --for=condition=ready pod -l app=postgres -n demo-cluster --timeout=300s
kubectl wait --for=condition=ready pod -l app=web -n demo-cluster --timeout=300s

echo -e "${GREEN}[INFO]${NC} 部署完成！"
kubectl get pods -n demo-cluster
```

### 常用命令速查

```bash
# 查看 Pod 状态
kubectl get pods -n demo-cluster

# 查看日志
kubectl logs -f -l app=web -n demo-cluster

# 进入容器
kubectl exec -it <pod-name> -n demo-cluster -- bash

# 查看资源使用
kubectl top pods -n demo-cluster

# 端口转发
kubectl port-forward svc/web 3013:3013 -n demo-cluster

# 扩展副本
kubectl scale deployment web --replicas=3 -n demo-cluster

# 清理部署
kubectl delete namespace demo-cluster
```

## 相关资源

- [Kubernetes 官方文档](https://kubernetes.io/docs/)
- [kubectl 命令参考](https://kubernetes.io/docs/reference/kubectl/)
- [Docker 文档](https://docs.docker.com/)
- [Minikube 文档](https://minikube.sigs.k8s.io/)

## 支持和反馈

如有问题，请：
1. 查看相关文档
2. 查看日志和事件
3. 参考故障排查指南
4. 联系技术支持

---

**最后更新：** 2024-01-21  
**版本：** 1.0
