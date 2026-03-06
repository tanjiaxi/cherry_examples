# Kubernetes (K8s) 部署完整指南

## 概述

本指南提供了在 Kubernetes 集群上部署 demo_cluster 游戏服务器的完整步骤。

## 前置要求

### 1. 安装必要工具

#### macOS 上安装

```bash
# 安装 Homebrew（如果未安装）
/bin/bash -c "$(curl -fsSL https://raw.githubusercontent.com/Homebrew/install/HEAD/install.sh)"

# 安装 kubectl
brew install kubectl

# 安装 Minikube（本地 K8s 集群）
brew install minikube

# 安装 Docker Desktop（包含 Docker 和 Kubernetes 支持）
brew install --cask docker
```

#### 验证安装

```bash
kubectl version --client
minikube version
docker --version
```

### 2. 启动 Kubernetes 集群

#### 选项 A：使用 Minikube（推荐用于本地开发）

```bash
# 启动 Minikube
minikube start --cpus=4 --memory=8192 --disk-size=50g

# 验证集群状态
kubectl cluster-info
kubectl get nodes

# 启用 Minikube 的 Docker 环境（用于构建镜像）
eval $(minikube docker-env)
```

#### 选项 B：使用 Docker Desktop 内置 K8s

1. 打开 Docker Desktop
2. 进入 Preferences → Kubernetes
3. 勾选 "Enable Kubernetes"
4. 等待集群启动完成

#### 选项 C：连接到远程 K8s 集群

```bash
# 配置 kubeconfig
kubectl config use-context <your-cluster-context>

# 验证连接
kubectl cluster-info
```

## 部署步骤

### 步骤 1：准备工作目录

```bash
cd demo_cluster/docker
```

### 步骤 2：检查集群状态

```bash
# 使用部署脚本检查
sh k8s-deploy.sh check

# 或手动检查
kubectl cluster-info
kubectl get nodes
```

### 步骤 3：构建 Docker 镜像

#### 方式 A：使用部署脚本

```bash
sh k8s-deploy.sh build
```

#### 方式 B：手动构建

```bash
# 如果使用 Minikube，先切换到 Minikube 的 Docker 环境
eval $(minikube docker-env)

# 构建镜像
cd ..
docker build -f docker/Dockerfile -t demo-cluster:latest .
cd docker

# 如果使用 Minikube，加载镜像
minikube image load demo-cluster:latest
```

### 步骤 4：一键部署

```bash
# 完整部署（包括检查、构建、部署）
sh k8s-deploy.sh deploy
```

这个命令会：
1. 检查 K8s 集群
2. 构建 Docker 镜像
3. 创建命名空间
4. 部署 PostgreSQL
5. 部署 ETCD
6. 部署 NATS
7. 部署游戏节点（Center、Gate、Game、Web）
8. 显示部署状态和访问信息

### 步骤 5：验证部署

```bash
# 查看部署状态
sh k8s-deploy.sh status

# 或手动查看
kubectl get pods -n demo-cluster
kubectl get svc -n demo-cluster
kubectl get pvc -n demo-cluster
```

预期输出：
```
NAME                      READY   STATUS    RESTARTS   AGE
postgres-xxxxx            1/1     Running   0          2m
etcd-xxxxx                1/1     Running   0          2m
nats-xxxxx                1/1     Running   0          2m
center-xxxxx              1/1     Running   0          1m
gate-xxxxx                1/1     Running   0          1m
gate-xxxxx                1/1     Running   0          1m
game-xxxxx                1/1     Running   0          1m
game-xxxxx                1/1     Running   0          1m
web-xxxxx                 1/1     Running   0          1m
web-xxxxx                 1/1     Running   0          1m
```

## 访问服务

### 方式 1：使用端口转发（推荐用于本地开发）

```bash
# 显示所有端口转发命令
sh k8s-deploy.sh access

# 或手动设置端口转发

# Web 服务（在新终端运行）
kubectl port-forward svc/web 3013:3013 -n demo-cluster

# Gate 服务（在新终端运行）
kubectl port-forward svc/gate 3011:3011 -n demo-cluster

# PostgreSQL（在新终端运行）
kubectl port-forward svc/postgres 5432:5432 -n demo-cluster

# ETCD（在新终端运行）
kubectl port-forward svc/etcd 2379:2379 -n demo-cluster

# NATS（在新终端运行）
kubectl port-forward svc/nats 4222:4222 -n demo-cluster
```

然后访问：
- Web 服务: http://localhost:3013
- Gate 服务: localhost:3011
- PostgreSQL: localhost:5432
- ETCD: http://localhost:2379
- NATS: nats://localhost:4222

### 方式 2：使用 LoadBalancer（如果集群支持）

```bash
# 获取外部 IP
kubectl get svc -n demo-cluster

# 访问 Web 服务
# 使用 EXTERNAL-IP:3013
```

## 查看日志

```bash
# 查看所有 Pod 日志
kubectl logs -f <pod-name> -n demo-cluster

# 查看特定应用的日志
kubectl logs -f -l app=web -n demo-cluster
kubectl logs -f -l app=center -n demo-cluster
kubectl logs -f -l app=gate -n demo-cluster
kubectl logs -f -l app=game -n demo-cluster

# 查看最后 100 行日志
kubectl logs --tail=100 <pod-name> -n demo-cluster

# 实时查看日志
kubectl logs -f <pod-name> -n demo-cluster
```

## 常用命令

### Pod 管理

```bash
# 查看 Pod 详情
kubectl describe pod <pod-name> -n demo-cluster

# 进入 Pod 容器
kubectl exec -it <pod-name> -n demo-cluster -- sh

# 重启 Pod
kubectl delete pod <pod-name> -n demo-cluster

# 查看 Pod 资源使用
kubectl top pods -n demo-cluster
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

### 存储管理

```bash
# 查看 PVC
kubectl get pvc -n demo-cluster

# 查看 PV
kubectl get pv

# 查看存储类
kubectl get storageclass
```

### 事件和监控

```bash
# 查看事件
kubectl get events -n demo-cluster

# 查看资源使用
kubectl top nodes
kubectl top pods -n demo-cluster

# 查看节点信息
kubectl get nodes -o wide
```

## 故障排查

### 问题 1：Pod 无法启动

```bash
# 查看 Pod 状态
kubectl describe pod <pod-name> -n demo-cluster

# 查看 Pod 日志
kubectl logs <pod-name> -n demo-cluster

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
kubectl exec -it <pod-name> -n demo-cluster -- sh
# 在容器内测试：
# nc -zv postgres 5432
# nc -zv etcd 2379
# nc -zv nats 4222
```

### 问题 3：数据库连接失败

```bash
# 检查 PostgreSQL Pod
kubectl get pods -l app=postgres -n demo-cluster

# 查看 PostgreSQL 日志
kubectl logs -f -l app=postgres -n demo-cluster

# 进入 PostgreSQL 容器测试
kubectl exec -it <postgres-pod> -n demo-cluster -- psql -U postgres -d demo_cluster
```

### 问题 4：ETCD 连接失败

```bash
# 检查 ETCD Pod
kubectl get pods -l app=etcd -n demo-cluster

# 查看 ETCD 日志
kubectl logs -f -l app=etcd -n demo-cluster

# 进入 ETCD 容器测试
kubectl exec -it <etcd-pod> -n demo-cluster -- etcdctl --endpoints=http://localhost:2379 endpoint health
```

### 问题 5：NATS 连接失败

```bash
# 检查 NATS Pod
kubectl get pods -l app=nats -n demo-cluster

# 查看 NATS 日志
kubectl logs -f -l app=nats -n demo-cluster

# 进入 NATS 容器测试
kubectl exec -it <nats-pod> -n demo-cluster -- sh
# 在容器内测试：
# nc -zv localhost 4222
```

## 清理部署

### 删除所有资源

```bash
# 使用脚本清理
sh k8s-deploy.sh clean

# 或手动删除
kubectl delete namespace demo-cluster
```

### 停止 Minikube

```bash
minikube stop

# 删除 Minikube 集群
minikube delete
```

## 扩展和优化

### 扩展副本数

```bash
# 扩展 Gate 节点
kubectl scale deployment gate --replicas=3 -n demo-cluster

# 扩展 Game 节点
kubectl scale deployment game --replicas=3 -n demo-cluster

# 扩展 Web 节点
kubectl scale deployment web --replicas=3 -n demo-cluster
```

### 更新镜像

```bash
# 重新构建镜像
docker build -f docker/Dockerfile -t demo-cluster:latest .

# 更新部署
kubectl set image deployment/center center=demo-cluster:latest -n demo-cluster
kubectl set image deployment/gate gate=demo-cluster:latest -n demo-cluster
kubectl set image deployment/game game=demo-cluster:latest -n demo-cluster
kubectl set image deployment/web web=demo-cluster:latest -n demo-cluster
```

### 资源限制

编辑 `k8s-game-nodes.yaml`，在容器规范中添加：

```yaml
resources:
  requests:
    memory: "256Mi"
    cpu: "250m"
  limits:
    memory: "512Mi"
    cpu: "500m"
```

## 性能监控

### 使用 Kubernetes Dashboard

```bash
# 启动 Dashboard（Minikube）
minikube dashboard

# 或手动部署 Dashboard
kubectl apply -f https://raw.githubusercontent.com/kubernetes/dashboard/v2.7.0/aio/deploy/recommended.yaml

# 访问 Dashboard
kubectl proxy
# 访问: http://localhost:8001/api/v1/namespaces/kubernetes-dashboard/services/https:kubernetes-dashboard:/proxy/
```

### 使用 Prometheus 和 Grafana

参考 Kubernetes 官方文档进行部署。

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

```bash
# 一键部署
sh k8s-deploy.sh deploy

# 查看状态
sh k8s-deploy.sh status

# 显示访问信息
sh k8s-deploy.sh access

# 清理部署
sh k8s-deploy.sh clean

# 查看帮助
sh k8s-deploy.sh help
```

## 相关文件

- `k8s-deploy.sh` - 部署脚本
- `k8s-namespace.yaml` - 命名空间配置
- `k8s-postgres.yaml` - PostgreSQL 部署
- `k8s-etcd.yaml` - ETCD 部署
- `k8s-nats.yaml` - NATS 部署
- `k8s-game-nodes.yaml` - 游戏节点部署
- `Dockerfile` - Docker 镜像构建文件
- `docker-compose-local.yml` - 本地开发配置
- `docker-compose.yml` - 完整 Docker Compose 配置

## 支持和反馈

如有问题，请查看日志或联系技术支持。
