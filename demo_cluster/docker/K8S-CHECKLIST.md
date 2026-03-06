# Kubernetes 部署检查清单

## 部署前检查

### 环境检查

- [ ] 已安装 Docker Desktop 或 Docker Engine
- [ ] 已安装 kubectl
- [ ] 已安装 Minikube（用于本地开发）
- [ ] 系统有足够的资源（CPU ≥ 4 核，内存 ≥ 8 GB）
- [ ] 网络连接正常

### 工具验证

```bash
# 检查 Docker
docker --version

# 检查 kubectl
kubectl version --client

# 检查 Minikube
minikube version
```

### 文件检查

- [ ] `k8s-deploy.sh` 存在且可执行
- [ ] `quick-k8s-setup.sh` 存在且可执行
- [ ] `Dockerfile` 存在
- [ ] `k8s-namespace.yaml` 存在
- [ ] `k8s-postgres.yaml` 存在
- [ ] `k8s-etcd.yaml` 存在
- [ ] `k8s-nats.yaml` 存在
- [ ] `k8s-game-nodes.yaml` 存在

## 部署步骤检查

### 步骤 1：启动 Minikube

```bash
minikube start --cpus=4 --memory=8192 --disk-size=50g
```

检查清单：
- [ ] Minikube 启动成功
- [ ] 集群状态为 Running
- [ ] 可以连接到集群

验证命令：
```bash
kubectl cluster-info
kubectl get nodes
```

### 步骤 2：构建 Docker 镜像

```bash
eval $(minikube docker-env)
cd demo_cluster
docker build -f docker/Dockerfile -t demo-cluster:latest .
minikube image load demo-cluster:latest
```

检查清单：
- [ ] 镜像构建成功
- [ ] 镜像大小合理（< 500 MB）
- [ ] 镜像已加载到 Minikube

验证命令：
```bash
docker images | grep demo-cluster
minikube image ls | grep demo-cluster
```

### 步骤 3：创建命名空间

```bash
kubectl apply -f k8s-namespace.yaml
```

检查清单：
- [ ] 命名空间创建成功
- [ ] 命名空间名称为 demo-cluster

验证命令：
```bash
kubectl get namespace demo-cluster
```

### 步骤 4：部署基础服务

```bash
kubectl apply -f k8s-postgres.yaml
kubectl apply -f k8s-etcd.yaml
kubectl apply -f k8s-nats.yaml
```

检查清单：
- [ ] PostgreSQL Pod 创建成功
- [ ] ETCD Pod 创建成功
- [ ] NATS Pod 创建成功
- [ ] 所有 Pod 状态为 Running

验证命令：
```bash
kubectl get pods -n demo-cluster
kubectl get svc -n demo-cluster
```

### 步骤 5：等待基础服务就绪

```bash
kubectl wait --for=condition=ready pod -l app=postgres -n demo-cluster --timeout=300s
kubectl wait --for=condition=ready pod -l app=etcd -n demo-cluster --timeout=300s
kubectl wait --for=condition=ready pod -l app=nats -n demo-cluster --timeout=300s
```

检查清单：
- [ ] PostgreSQL 就绪
- [ ] ETCD 就绪
- [ ] NATS 就绪

验证命令：
```bash
kubectl get pods -n demo-cluster -o wide
```

### 步骤 6：部署游戏节点

```bash
kubectl apply -f k8s-game-nodes.yaml
```

检查清单：
- [ ] Center Pod 创建成功
- [ ] Gate Pod 创建成功（2 个副本）
- [ ] Game Pod 创建成功（2 个副本）
- [ ] Web Pod 创建成功（2 个副本）

验证命令：
```bash
kubectl get pods -n demo-cluster
```

### 步骤 7：等待游戏节点就绪

```bash
kubectl wait --for=condition=ready pod -l app=center -n demo-cluster --timeout=300s
kubectl wait --for=condition=ready pod -l app=gate -n demo-cluster --timeout=300s
kubectl wait --for=condition=ready pod -l app=game -n demo-cluster --timeout=300s
kubectl wait --for=condition=ready pod -l app=web -n demo-cluster --timeout=300s
```

检查清单：
- [ ] Center 就绪
- [ ] Gate 就绪
- [ ] Game 就绪
- [ ] Web 就绪

验证命令：
```bash
kubectl get pods -n demo-cluster
```

## 部署后检查

### Pod 状态检查

```bash
kubectl get pods -n demo-cluster
```

预期输出：
```
NAME                      READY   STATUS    RESTARTS   AGE
postgres-xxxxx            1/1     Running   0          5m
etcd-xxxxx                1/1     Running   0          5m
nats-xxxxx                1/1     Running   0          5m
center-xxxxx              1/1     Running   0          3m
gate-xxxxx                1/1     Running   0          3m
gate-xxxxx                1/1     Running   0          3m
game-xxxxx                1/1     Running   0          3m
game-xxxxx                1/1     Running   0          3m
web-xxxxx                 1/1     Running   0          3m
web-xxxxx                 1/1     Running   0          3m
```

检查清单：
- [ ] 所有 Pod 状态为 Running
- [ ] 所有 Pod Ready 状态为 1/1
- [ ] 没有 Pod 处于 Pending 或 CrashLoopBackOff 状态

### 服务检查

```bash
kubectl get svc -n demo-cluster
```

预期输出：
```
NAME        TYPE           CLUSTER-IP      EXTERNAL-IP   PORT(S)
postgres    ClusterIP      10.x.x.x        <none>        5432/TCP
etcd        ClusterIP      10.x.x.x        <none>        2379/TCP,2380/TCP
nats        ClusterIP      10.x.x.x        <none>        4222/TCP,6222/TCP,8222/TCP
center      ClusterIP      10.x.x.x        <none>        3010/TCP
gate        LoadBalancer   10.x.x.x        <pending>     3011:xxxxx/TCP
game        ClusterIP      10.x.x.x        <none>        3012/TCP
web         LoadBalancer   10.x.x.x        <pending>     3013:xxxxx/TCP
```

检查清单：
- [ ] 所有服务都已创建
- [ ] 服务类型正确（ClusterIP 或 LoadBalancer）
- [ ] 端口映射正确

### 存储检查

```bash
kubectl get pvc -n demo-cluster
```

预期输出：
```
NAME            STATUS   VOLUME                                     CAPACITY   ACCESS MODES   STORAGECLASS
postgres-pvc    Bound    pvc-xxxxx                                  10Gi       RWO            standard
etcd-pvc        Bound    pvc-xxxxx                                  5Gi        RWO            standard
```

检查清单：
- [ ] PostgreSQL PVC 状态为 Bound
- [ ] ETCD PVC 状态为 Bound
- [ ] 存储容量正确

### 网络连接检查

```bash
# 测试 PostgreSQL 连接
kubectl exec -it <postgres-pod> -n demo-cluster -- psql -U postgres -d demo_cluster -c "SELECT 1"

# 测试 ETCD 连接
kubectl exec -it <etcd-pod> -n demo-cluster -- etcdctl --endpoints=http://localhost:2379 endpoint health

# 测试 NATS 连接
kubectl exec -it <nats-pod> -n demo-cluster -- nc -zv localhost 4222
```

检查清单：
- [ ] PostgreSQL 可以连接
- [ ] ETCD 可以连接
- [ ] NATS 可以连接

### 日志检查

```bash
# 查看 Center 日志
kubectl logs -f -l app=center -n demo-cluster

# 查看 Gate 日志
kubectl logs -f -l app=gate -n demo-cluster

# 查看 Game 日志
kubectl logs -f -l app=game -n demo-cluster

# 查看 Web 日志
kubectl logs -f -l app=web -n demo-cluster
```

检查清单：
- [ ] 没有错误日志
- [ ] 服务正常启动
- [ ] 节点间通信正常

### 资源使用检查

```bash
kubectl top pods -n demo-cluster
kubectl top nodes
```

检查清单：
- [ ] CPU 使用率合理（< 80%）
- [ ] 内存使用率合理（< 80%）
- [ ] 没有 Pod 被 OOMKilled

## 访问服务检查

### 端口转发设置

```bash
# Web 服务
kubectl port-forward svc/web 3013:3013 -n demo-cluster

# Gate 服务
kubectl port-forward svc/gate 3011:3011 -n demo-cluster

# PostgreSQL
kubectl port-forward svc/postgres 5432:5432 -n demo-cluster

# ETCD
kubectl port-forward svc/etcd 2379:2379 -n demo-cluster

# NATS
kubectl port-forward svc/nats 4222:4222 -n demo-cluster
```

检查清单：
- [ ] Web 服务可以访问（http://localhost:3013）
- [ ] Gate 服务可以连接（localhost:3011）
- [ ] PostgreSQL 可以连接（localhost:5432）
- [ ] ETCD 可以访问（http://localhost:2379）
- [ ] NATS 可以连接（nats://localhost:4222）

## 功能测试

### 基础功能测试

```bash
# 测试 Web 服务
curl http://localhost:3013

# 测试数据库连接
psql -h localhost -U postgres -d demo_cluster -c "SELECT 1"

# 测试 ETCD
curl http://localhost:2379/v3/version
```

检查清单：
- [ ] Web 服务响应正常
- [ ] 数据库查询成功
- [ ] ETCD 可以访问

### 游戏功能测试

```bash
# 查看游戏节点日志
kubectl logs -f -l app=center -n demo-cluster

# 检查节点注册
kubectl exec -it <etcd-pod> -n demo-cluster -- etcdctl --endpoints=http://localhost:2379 get --prefix /cherry/nodes
```

检查清单：
- [ ] 游戏节点正常启动
- [ ] 节点已注册到 ETCD
- [ ] 节点间通信正常

## 故障排查检查

如果部署失败，请检查：

### Pod 无法启动

```bash
# 查看 Pod 详情
kubectl describe pod <pod-name> -n demo-cluster

# 查看 Pod 日志
kubectl logs <pod-name> -n demo-cluster

# 查看事件
kubectl get events -n demo-cluster
```

检查清单：
- [ ] 镜像是否存在
- [ ] 资源是否充足
- [ ] 依赖服务是否就绪
- [ ] 配置是否正确

### 服务无法连接

```bash
# 检查服务端点
kubectl get endpoints -n demo-cluster

# 测试 Pod 间通信
kubectl exec -it <pod1> -n demo-cluster -- nc -zv <service> <port>
```

检查清单：
- [ ] 服务端点是否正确
- [ ] Pod 是否就绪
- [ ] 网络策略是否允许

### 资源不足

```bash
# 查看节点资源
kubectl describe nodes

# 查看 Pod 资源使用
kubectl top pods -n demo-cluster
```

检查清单：
- [ ] 节点资源是否充足
- [ ] Pod 资源限制是否合理
- [ ] 是否需要扩展集群

## 清理检查

部署完成后，如需清理：

```bash
# 删除所有资源
kubectl delete namespace demo-cluster

# 停止 Minikube
minikube stop

# 删除 Minikube
minikube delete
```

检查清单：
- [ ] 所有 Pod 已删除
- [ ] 所有服务已删除
- [ ] 所有 PVC 已删除
- [ ] 命名空间已删除

## 部署完成确认

部署完成后，请确认以下事项：

- [ ] 所有 Pod 状态为 Running
- [ ] 所有服务可以访问
- [ ] 所有日志正常
- [ ] 资源使用合理
- [ ] 功能测试通过
- [ ] 文档已阅读
- [ ] 故障排查指南已保存

## 下一步

- [ ] 阅读 [K8S-README.md](K8S-README.md)
- [ ] 阅读 [K8S-DEPLOYMENT-GUIDE.md](K8S-DEPLOYMENT-GUIDE.md)
- [ ] 阅读 [K8S-TROUBLESHOOTING.md](K8S-TROUBLESHOOTING.md)
- [ ] 阅读 [K8S-MONITORING.md](K8S-MONITORING.md)
- [ ] 配置监控告警
- [ ] 配置备份策略
- [ ] 配置自动扩展

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
```

---

**部署检查清单完成！** ✅

如有任何问题，请参考相关文档或联系技术支持。
