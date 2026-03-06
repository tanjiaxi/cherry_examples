# Demo Cluster Docker & Kubernetes 部署指南

## 目录结构

```
docker/
├── Dockerfile              # 游戏服务器镜像
├── docker-compose.yml      # Docker Compose 编排
├── nats.conf              # NATS 配置
├── k8s-namespace.yaml     # K8s 命名空间
├── k8s-postgres.yaml      # K8s PostgreSQL 部署
├── k8s-etcd.yaml          # K8s ETCD 部署
├── k8s-nats.yaml          # K8s NATS 部署
├── k8s-game-nodes.yaml    # K8s 游戏节点部署
└── README.md              # 本文件
```

## Docker Compose 部署

### 快速启动

```bash
cd demo_cluster/docker
docker-compose up -d
```

### 查看日志

```bash
# 查看所有服务日志
docker-compose logs -f

# 查看特定服务日志
docker-compose logs -f center
docker-compose logs -f game
docker-compose logs -f web
```

### 停止服务

```bash
docker-compose down
```

### 清理数据

```bash
docker-compose down -v  # 删除所有卷
```

## Kubernetes 部署

### 前置条件

- 已安装 kubectl
- 已连接到 K8s 集群
- 已安装 Docker（用于构建镜像）

### 构建镜像

```bash
cd demo_cluster
docker build -f docker/Dockerfile -t demo-cluster:latest .
```

如果使用 Minikube，需要加载镜像到 Minikube：

```bash
minikube image load demo-cluster:latest
```

### 部署步骤

1. **创建命名空间**

```bash
kubectl apply -f demo_cluster/docker/k8s-namespace.yaml
```

2. **部署基础服务（PostgreSQL、ETCD、NATS）**

```bash
kubectl apply -f demo_cluster/docker/k8s-postgres.yaml
kubectl apply -f demo_cluster/docker/k8s-etcd.yaml
kubectl apply -f demo_cluster/docker/k8s-nats.yaml
```

3. **等待基础服务就绪**

```bash
kubectl wait --for=condition=ready pod -l app=postgres -n demo-cluster --timeout=300s
kubectl wait --for=condition=ready pod -l app=etcd -n demo-cluster --timeout=300s
kubectl wait --for=condition=ready pod -l app=nats -n demo-cluster --timeout=300s
```

4. **部署游戏节点**

```bash
kubectl apply -f demo_cluster/docker/k8s-game-nodes.yaml
```

### 查看部署状态

```bash
# 查看所有 Pod
kubectl get pods -n demo-cluster

# 查看 Pod 详情
kubectl describe pod <pod-name> -n demo-cluster

# 查看日志
kubectl logs -f <pod-name> -n demo-cluster

# 查看服务
kubectl get svc -n demo-cluster
```

### 端口转发（本地访问）

```bash
# 转发 Web 服务
kubectl port-forward svc/web 3013:3013 -n demo-cluster

# 转发 Gate 服务
kubectl port-forward svc/gate 3011:3011 -n demo-cluster

# 转发 PostgreSQL
kubectl port-forward svc/postgres 5432:5432 -n demo-cluster

# 转发 ETCD
kubectl port-forward svc/etcd 2379:2379 -n demo-cluster

# 转发 NATS
kubectl port-forward svc/nats 4222:4222 -n demo-cluster
```

### 扩展副本数

```bash
# 扩展 Gate 节点到 3 个副本
kubectl scale deployment gate --replicas=3 -n demo-cluster

# 扩展 Game 节点到 4 个副本
kubectl scale deployment game --replicas=4 -n demo-cluster
```

### 删除部署

```bash
# 删除所有资源
kubectl delete namespace demo-cluster

# 或逐个删除
kubectl delete -f demo_cluster/docker/k8s-game-nodes.yaml
kubectl delete -f demo_cluster/docker/k8s-nats.yaml
kubectl delete -f demo_cluster/docker/k8s-etcd.yaml
kubectl delete -f demo_cluster/docker/k8s-postgres.yaml
kubectl delete -f demo_cluster/docker/k8s-namespace.yaml
```

## 环境变量配置

### Docker Compose

在 `docker-compose.yml` 中修改环境变量：

```yaml
environment:
  DB_HOST: postgres
  DB_PORT: 5432
  ETCD_ENDPOINTS: http://etcd:2379
  NATS_URL: nats://nats:4222
```

### Kubernetes

在 `k8s-game-nodes.yaml` 中修改环境变量：

```yaml
env:
- name: DB_HOST
  value: postgres
- name: DB_PORT
  value: "5432"
```

## 常见问题

### 1. Pod 无法连接到数据库

检查 PostgreSQL 是否已就绪：

```bash
kubectl get pods -n demo-cluster | grep postgres
kubectl logs -f postgres-xxx -n demo-cluster
```

### 2. 服务无法通信

检查网络策略和 DNS：

```bash
# 进入 Pod 测试 DNS
kubectl exec -it <pod-name> -n demo-cluster -- sh
nslookup postgres
nslookup etcd
nslookup nats
```

### 3. 镜像拉取失败

确保镜像已构建并加载到 K8s：

```bash
docker images | grep demo-cluster
kubectl get images -n demo-cluster
```

## 性能优化建议

1. **资源限制**：在 K8s 部署中添加 resources 限制
2. **副本数**：根据负载调整 Gate 和 Game 节点副本数
3. **持久化**：使用 PVC 确保数据持久化
4. **监控**：部署 Prometheus + Grafana 监控集群

## 生产环境建议

1. 使用私有镜像仓库存储镜像
2. 配置 RBAC 和网络策略
3. 使用 Ingress 管理外部访问
4. 配置自动扩展（HPA）
5. 使用 ConfigMap 和 Secret 管理配置
6. 定期备份数据库
