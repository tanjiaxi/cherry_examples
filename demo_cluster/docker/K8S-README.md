# Kubernetes 部署完整指南

## 快速开始

### 一键部署（推荐）

```bash
cd demo_cluster/docker

# 方式 1：使用快速设置脚本（自动安装工具和部署）
sh quick-k8s-setup.sh

# 方式 2：使用部署脚本（需要已安装 kubectl 和 Minikube）
sh k8s-deploy.sh deploy
```

### 手动部署

```bash
cd demo_cluster/docker

# 1. 检查集群
sh k8s-deploy.sh check

# 2. 构建镜像
sh k8s-deploy.sh build

# 3. 部署
sh k8s-deploy.sh deploy

# 4. 查看状态
sh k8s-deploy.sh status

# 5. 显示访问信息
sh k8s-deploy.sh access
```

## 部署架构

```
┌─────────────────────────────────────────────────────────┐
│                  Kubernetes Cluster                      │
├─────────────────────────────────────────────────────────┤
│                                                           │
│  ┌──────────────────────────────────────────────────┐   │
│  │         demo-cluster Namespace                   │   │
│  ├──────────────────────────────────────────────────┤   │
│  │                                                  │   │
│  │  ┌─────────────┐  ┌─────────────┐              │   │
│  │  │ PostgreSQL  │  │    ETCD     │              │   │
│  │  │  (1 Pod)    │  │  (1 Pod)    │              │   │
│  │  └─────────────┘  └─────────────┘              │   │
│  │                                                  │   │
│  │  ┌─────────────┐  ┌─────────────┐              │   │
│  │  │    NATS     │  │   Center    │              │   │
│  │  │  (1 Pod)    │  │  (1 Pod)    │              │   │
│  │  └─────────────┘  └─────────────┘              │   │
│  │                                                  │   │
│  │  ┌─────────────┐  ┌─────────────┐              │   │
│  │  │    Gate     │  │    Game     │              │   │
│  │  │  (2 Pods)   │  │  (2 Pods)   │              │   │
│  │  └─────────────┘  └─────────────┘              │   │
│  │                                                  │   │
│  │  ┌─────────────┐                               │   │
│  │  │     Web     │                               │   │
│  │  │  (2 Pods)   │                               │   │
│  │  └─────────────┘                               │   │
│  │                                                  │   │
│  └──────────────────────────────────────────────────┘   │
│                                                           │
└─────────────────────────────────────────────────────────┘
```

## 文件结构

```
demo_cluster/docker/
├── k8s-deploy.sh                 # 主部署脚本
├── quick-k8s-setup.sh            # 快速设置脚本
├── k8s-namespace.yaml            # 命名空间配置
├── k8s-postgres.yaml             # PostgreSQL 部署
├── k8s-etcd.yaml                 # ETCD 部署
├── k8s-nats.yaml                 # NATS 部署
├── k8s-game-nodes.yaml           # 游戏节点部署
├── Dockerfile                    # Docker 镜像构建
├── docker-compose-local.yml      # 本地开发配置
├── docker-compose.yml            # 完整 Docker Compose 配置
├── K8S-README.md                 # 本文件
├── K8S-DEPLOYMENT-GUIDE.md       # 详细部署指南
├── K8S-TROUBLESHOOTING.md        # 故障排查指南
└── K8S-MONITORING.md             # 监控管理指南
```

## 部署流程

### 1. 环境准备

```bash
# 安装必要工具
brew install kubectl minikube docker

# 启动 Minikube
minikube start --cpus=4 --memory=8192 --disk-size=50g

# 验证集群
kubectl cluster-info
```

### 2. 构建镜像

```bash
# 切换到 Minikube 的 Docker 环境
eval $(minikube docker-env)

# 构建镜像
cd demo_cluster
docker build -f docker/Dockerfile -t demo-cluster:latest .

# 加载到 Minikube
minikube image load demo-cluster:latest
```

### 3. 部署服务

```bash
cd demo_cluster/docker

# 创建命名空间
kubectl apply -f k8s-namespace.yaml

# 部署基础服务
kubectl apply -f k8s-postgres.yaml
kubectl apply -f k8s-etcd.yaml
kubectl apply -f k8s-nats.yaml

# 等待基础服务就绪
kubectl wait --for=condition=ready pod -l app=postgres -n demo-cluster --timeout=300s

# 部署游戏节点
kubectl apply -f k8s-game-nodes.yaml

# 等待游戏节点就绪
kubectl wait --for=condition=ready pod -l app=web -n demo-cluster --timeout=300s
```

### 4. 访问服务

```bash
# 端口转发
kubectl port-forward svc/web 3013:3013 -n demo-cluster

# 访问 Web 服务
# http://localhost:3013
```

## 常用命令

### 查看状态

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

### 查看日志

```bash
# 查看 Web 日志
kubectl logs -f -l app=web -n demo-cluster

# 查看 Center 日志
kubectl logs -f -l app=center -n demo-cluster

# 查看特定 Pod 日志
kubectl logs -f <pod-name> -n demo-cluster
```

### 管理部署

```bash
# 扩展副本
kubectl scale deployment web --replicas=3 -n demo-cluster

# 更新镜像
kubectl set image deployment/web web=demo-cluster:v2 -n demo-cluster

# 回滚部署
kubectl rollout undo deployment/web -n demo-cluster

# 删除部署
kubectl delete deployment web -n demo-cluster
```

### 进入容器

```bash
# 进入 Pod
kubectl exec -it <pod-name> -n demo-cluster -- sh

# 执行命令
kubectl exec <pod-name> -n demo-cluster -- cat /etc/hosts
```

## 端口映射

| 服务 | 内部端口 | 外部端口 | 说明 |
|------|---------|---------|------|
| Web | 3013 | 3013 | Web 服务 |
| Gate | 3011 | 3011 | 网关服务 |
| Center | 3010 | 3010 | 中心服务 |
| Game | 3012 | 3012 | 游戏服务 |
| PostgreSQL | 5432 | 5432 | 数据库 |
| ETCD | 2379 | 2379 | 服务发现 |
| NATS | 4222 | 4222 | 消息队列 |

## 部署选项

### 选项 1：本地开发（Docker Compose）

```bash
cd demo_cluster/docker
docker-compose -f docker-compose-local.yml up -d
```

**优点：**
- 简单快速
- 资源占用少
- 适合本地开发

**缺点：**
- 不支持分布式
- 扩展性差

### 选项 2：Kubernetes 本地（Minikube）

```bash
sh k8s-deploy.sh deploy
```

**优点：**
- 接近生产环境
- 支持分布式
- 易于扩展

**缺点：**
- 资源占用多
- 启动较慢

### 选项 3：Kubernetes 云端（EKS/GKE/AKS）

```bash
# 配置 kubeconfig
kubectl config use-context <cloud-cluster>

# 部署
sh k8s-deploy.sh deploy
```

**优点：**
- 生产级别
- 高可用性
- 自动扩展

**缺点：**
- 成本较高
- 配置复杂

## 故障排查

### 常见问题

1. **Pod 无法启动**
   - 查看日志：`kubectl logs <pod-name> -n demo-cluster`
   - 查看事件：`kubectl describe pod <pod-name> -n demo-cluster`

2. **服务无法连接**
   - 检查 Pod 状态：`kubectl get pods -n demo-cluster`
   - 检查服务：`kubectl get svc -n demo-cluster`
   - 测试连接：`kubectl exec -it <pod> -n demo-cluster -- nc -zv <service> <port>`

3. **资源不足**
   - 查看资源使用：`kubectl top nodes`
   - 增加 Minikube 资源：`minikube delete && minikube start --cpus=8 --memory=16384`

详见 [K8S-TROUBLESHOOTING.md](K8S-TROUBLESHOOTING.md)

## 监控和管理

### 查看监控数据

```bash
# 查看资源使用
kubectl top pods -n demo-cluster

# 查看事件
kubectl get events -n demo-cluster

# 查看日志
kubectl logs -f <pod-name> -n demo-cluster
```

### 启动 Dashboard

```bash
# Minikube
minikube dashboard

# 其他集群
kubectl proxy
# 访问: http://localhost:8001/api/v1/namespaces/kubernetes-dashboard/services/https:kubernetes-dashboard:/proxy/
```

详见 [K8S-MONITORING.md](K8S-MONITORING.md)

## 清理部署

```bash
# 删除所有资源
sh k8s-deploy.sh clean

# 或手动删除
kubectl delete namespace demo-cluster

# 停止 Minikube
minikube stop

# 删除 Minikube
minikube delete
```

## 性能优化

1. **增加资源**：增加 CPU 和内存
2. **优化镜像**：使用 alpine 基础镜像
3. **配置 HPA**：自动扩展 Pod
4. **使用 PDB**：保证可用性
5. **监控告警**：使用 Prometheus + Grafana

## 生产环境建议

1. 使用专业 K8s 集群（EKS/GKE/AKS）
2. 配置持久化存储
3. 设置监控告警
4. 配置日志收集
5. 实施备份策略
6. 使用 Ingress 进行流量管理
7. 配置 HPA 自动扩展
8. 安全加固（RBAC、网络策略等）

## 相关文档

- [K8S-DEPLOYMENT-GUIDE.md](K8S-DEPLOYMENT-GUIDE.md) - 详细部署指南
- [K8S-TROUBLESHOOTING.md](K8S-TROUBLESHOOTING.md) - 故障排查指南
- [K8S-MONITORING.md](K8S-MONITORING.md) - 监控管理指南
- [DEPLOYMENT-GUIDE.md](DEPLOYMENT-GUIDE.md) - 部署方式选择指南
- [README-LOCAL.md](README-LOCAL.md) - 本地开发指南
- [README.md](README.md) - Docker 部署指南

## 快速命令

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

## 获取帮助

```bash
# 查看脚本帮助
sh k8s-deploy.sh help

# 查看 kubectl 帮助
kubectl --help

# 查看特定命令帮助
kubectl describe --help
kubectl logs --help
kubectl exec --help
```

## 支持和反馈

如有问题，请：
1. 查看相关文档
2. 查看日志和事件
3. 参考故障排查指南
4. 联系技术支持

---

**最后更新：** 2024-01-21
**版本：** 1.0
