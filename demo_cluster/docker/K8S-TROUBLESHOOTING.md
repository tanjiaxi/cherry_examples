# Kubernetes 部署故障排查指南

## 常见问题和解决方案

### 1. Minikube 启动失败

#### 问题：`minikube start` 失败

**症状：**
```
Error starting cluster: ...
```

**解决方案：**

```bash
# 删除旧的 Minikube 配置
minikube delete

# 重新启动
minikube start --cpus=4 --memory=8192 --disk-size=50g

# 如果仍然失败，检查 Docker 是否运行
docker ps

# 检查系统资源
# macOS: 打开 Activity Monitor 检查 CPU 和内存使用
```

### 2. kubectl 无法连接到集群

#### 问题：`kubectl cluster-info` 失败

**症状：**
```
The connection to the server was refused - did you specify the right host or port?
```

**解决方案：**

```bash
# 检查 Minikube 状态
minikube status

# 如果 Minikube 未运行，启动它
minikube start

# 检查 kubeconfig
kubectl config view

# 重置 kubeconfig
minikube update-context

# 验证连接
kubectl cluster-info
```

### 3. Docker 镜像构建失败

#### 问题：`docker build` 失败

**症状：**
```
ERROR [internal] load metadata for docker.io/library/golang:1.23: 403 Forbidden
```

**解决方案：**

```bash
# 切换到 Minikube 的 Docker 环境
eval $(minikube docker-env)

# 清理 Docker 缓存
docker system prune -a

# 重新构建
cd ..
docker build -f docker/Dockerfile -t demo-cluster:latest .
cd docker

# 如果仍然失败，检查网络连接
ping docker.io
```

#### 问题：镜像太大或构建超时

**解决方案：**

```bash
# 增加 Docker 构建超时
docker build --build-arg BUILDKIT_INLINE_CACHE=1 -f docker/Dockerfile -t demo-cluster:latest .

# 或使用多阶段构建优化镜像大小
# 编辑 Dockerfile，使用 alpine 基础镜像
```

### 4. Pod 无法启动

#### 问题：Pod 状态为 `Pending`

**症状：**
```
NAME                      READY   STATUS    RESTARTS   AGE
postgres-xxxxx            0/1     Pending   0          5m
```

**解决方案：**

```bash
# 查看 Pod 详情
kubectl describe pod <pod-name> -n demo-cluster

# 查看事件
kubectl get events -n demo-cluster

# 常见原因：
# 1. 资源不足
kubectl top nodes
kubectl top pods -n demo-cluster

# 2. 镜像无法拉取
kubectl logs <pod-name> -n demo-cluster

# 3. PVC 无法绑定
kubectl get pvc -n demo-cluster
kubectl describe pvc <pvc-name> -n demo-cluster
```

#### 问题：Pod 状态为 `CrashLoopBackOff`

**症状：**
```
NAME                      READY   STATUS             RESTARTS   AGE
center-xxxxx              0/1     CrashLoopBackOff   5          5m
```

**解决方案：**

```bash
# 查看 Pod 日志
kubectl logs <pod-name> -n demo-cluster

# 查看前一个容器的日志
kubectl logs <pod-name> -n demo-cluster --previous

# 进入容器调试
kubectl exec -it <pod-name> -n demo-cluster -- sh

# 检查依赖服务是否就绪
kubectl get pods -n demo-cluster
kubectl get svc -n demo-cluster
```

#### 问题：Pod 状态为 `ImagePullBackOff`

**症状：**
```
NAME                      READY   STATUS             RESTARTS   AGE
center-xxxxx              0/1     ImagePullBackOff   0          5m
```

**解决方案：**

```bash
# 检查镜像是否存在
docker images | grep demo-cluster

# 如果使用 Minikube，确保镜像已加载
minikube image load demo-cluster:latest

# 检查镜像拉取策略
# 编辑部署配置，确保 imagePullPolicy 为 Never（本地镜像）或 IfNotPresent

# 重新构建和加载镜像
eval $(minikube docker-env)
cd ..
docker build -f docker/Dockerfile -t demo-cluster:latest .
cd docker
minikube image load demo-cluster:latest

# 重启 Pod
kubectl delete pod <pod-name> -n demo-cluster
```

### 5. 服务无法连接

#### 问题：无法连接到 PostgreSQL

**症状：**
```
connection refused
```

**解决方案：**

```bash
# 检查 PostgreSQL Pod
kubectl get pods -l app=postgres -n demo-cluster

# 查看 PostgreSQL 日志
kubectl logs -f -l app=postgres -n demo-cluster

# 进入 PostgreSQL 容器测试
kubectl exec -it <postgres-pod> -n demo-cluster -- psql -U postgres -d demo_cluster

# 检查服务
kubectl get svc postgres -n demo-cluster

# 测试连接
kubectl exec -it <any-pod> -n demo-cluster -- nc -zv postgres 5432

# 检查 PVC 是否绑定
kubectl get pvc -n demo-cluster
kubectl describe pvc postgres-pvc -n demo-cluster
```

#### 问题：无法连接到 ETCD

**症状：**
```
connection refused
```

**解决方案：**

```bash
# 检查 ETCD Pod
kubectl get pods -l app=etcd -n demo-cluster

# 查看 ETCD 日志
kubectl logs -f -l app=etcd -n demo-cluster

# 进入 ETCD 容器测试
kubectl exec -it <etcd-pod> -n demo-cluster -- etcdctl --endpoints=http://localhost:2379 endpoint health

# 检查服务
kubectl get svc etcd -n demo-cluster

# 测试连接
kubectl exec -it <any-pod> -n demo-cluster -- nc -zv etcd 2379
```

#### 问题：无法连接到 NATS

**症状：**
```
connection refused
```

**解决方案：**

```bash
# 检查 NATS Pod
kubectl get pods -l app=nats -n demo-cluster

# 查看 NATS 日志
kubectl logs -f -l app=nats -n demo-cluster

# 进入 NATS 容器测试
kubectl exec -it <nats-pod> -n demo-cluster -- sh
# 在容器内：nc -zv localhost 4222

# 检查服务
kubectl get svc nats -n demo-cluster

# 测试连接
kubectl exec -it <any-pod> -n demo-cluster -- nc -zv nats 4222
```

### 6. 端口转发失败

#### 问题：`kubectl port-forward` 失败

**症状：**
```
error: unable to forward port because pod is not running
```

**解决方案：**

```bash
# 检查 Pod 是否运行
kubectl get pods -n demo-cluster

# 检查 Pod 状态
kubectl describe pod <pod-name> -n demo-cluster

# 等待 Pod 就绪
kubectl wait --for=condition=ready pod -l app=web -n demo-cluster --timeout=300s

# 重新尝试端口转发
kubectl port-forward svc/web 3013:3013 -n demo-cluster
```

### 7. 存储问题

#### 问题：PVC 无法绑定

**症状：**
```
NAME              STATUS    VOLUME   CAPACITY   ACCESS MODES   STORAGECLASS   AGE
postgres-pvc      Pending                                                      5m
```

**解决方案：**

```bash
# 检查 PVC 详情
kubectl describe pvc postgres-pvc -n demo-cluster

# 检查存储类
kubectl get storageclass

# 检查 PV
kubectl get pv

# 如果使用 Minikube，启用存储驱动
minikube addons enable storage-provisioner

# 重新创建 PVC
kubectl delete pvc postgres-pvc -n demo-cluster
kubectl apply -f k8s-postgres.yaml
```

#### 问题：数据库数据丢失

**症状：**
```
数据库为空或数据不一致
```

**解决方案：**

```bash
# 检查 PVC 是否绑定
kubectl get pvc -n demo-cluster

# 检查 Pod 是否重启
kubectl get pods -n demo-cluster

# 查看 Pod 事件
kubectl describe pod <postgres-pod> -n demo-cluster

# 备份数据
kubectl exec -it <postgres-pod> -n demo-cluster -- pg_dump -U postgres demo_cluster > backup.sql

# 恢复数据
kubectl exec -it <postgres-pod> -n demo-cluster -- psql -U postgres demo_cluster < backup.sql
```

### 8. 资源不足

#### 问题：Pod 无法调度

**症状：**
```
Insufficient memory
Insufficient cpu
```

**解决方案：**

```bash
# 检查节点资源
kubectl top nodes
kubectl describe nodes

# 检查 Pod 资源使用
kubectl top pods -n demo-cluster

# 增加 Minikube 资源
minikube delete
minikube start --cpus=8 --memory=16384 --disk-size=100g

# 或减少 Pod 资源需求
# 编辑部署配置，减少 resources.requests 和 resources.limits
```

### 9. 网络问题

#### 问题：Pod 之间无法通信

**症状：**
```
connection refused
timeout
```

**解决方案：**

```bash
# 检查网络策略
kubectl get networkpolicies -n demo-cluster

# 检查 DNS
kubectl exec -it <pod-name> -n demo-cluster -- nslookup postgres

# 测试连接
kubectl exec -it <pod-name> -n demo-cluster -- nc -zv postgres 5432

# 检查服务端点
kubectl get endpoints -n demo-cluster

# 查看 iptables 规则（如果有权限）
kubectl exec -it <pod-name> -n demo-cluster -- iptables -L
```

### 10. 日志和调试

#### 查看详细日志

```bash
# 查看 Pod 日志
kubectl logs <pod-name> -n demo-cluster

# 查看前一个容器的日志
kubectl logs <pod-name> -n demo-cluster --previous

# 实时查看日志
kubectl logs -f <pod-name> -n demo-cluster

# 查看最后 N 行日志
kubectl logs --tail=100 <pod-name> -n demo-cluster

# 查看特定时间范围的日志
kubectl logs --since=1h <pod-name> -n demo-cluster
```

#### 进入容器调试

```bash
# 进入容器
kubectl exec -it <pod-name> -n demo-cluster -- sh

# 在容器内执行命令
kubectl exec <pod-name> -n demo-cluster -- cat /etc/hosts

# 运行调试容器
kubectl run -it --rm debug --image=alpine --restart=Never -n demo-cluster -- sh
```

#### 查看事件

```bash
# 查看所有事件
kubectl get events -n demo-cluster

# 查看特定 Pod 的事件
kubectl describe pod <pod-name> -n demo-cluster

# 实时查看事件
kubectl get events -n demo-cluster --watch
```

## 快速诊断脚本

```bash
#!/bin/bash

echo "=== Kubernetes 诊断 ==="
echo ""

echo "1. 集群信息"
kubectl cluster-info
echo ""

echo "2. 节点状态"
kubectl get nodes
kubectl top nodes
echo ""

echo "3. Pod 状态"
kubectl get pods -n demo-cluster
kubectl top pods -n demo-cluster
echo ""

echo "4. 服务状态"
kubectl get svc -n demo-cluster
echo ""

echo "5. 存储状态"
kubectl get pvc -n demo-cluster
echo ""

echo "6. 事件"
kubectl get events -n demo-cluster
echo ""

echo "7. 资源使用"
kubectl describe nodes
echo ""
```

## 性能优化建议

1. **增加资源**：增加 CPU 和内存
2. **优化镜像**：使用 alpine 基础镜像，减少镜像大小
3. **配置 HPA**：使用 Horizontal Pod Autoscaler 自动扩展
4. **使用 PDB**：配置 Pod Disruption Budget 保证可用性
5. **监控和告警**：使用 Prometheus + Grafana

## 获取帮助

```bash
# 查看 kubectl 帮助
kubectl --help

# 查看特定命令帮助
kubectl describe --help
kubectl logs --help
kubectl exec --help

# 查看 API 资源
kubectl api-resources

# 查看 API 版本
kubectl api-versions
```

## 相关资源

- [Kubernetes 官方文档](https://kubernetes.io/docs/)
- [kubectl 命令参考](https://kubernetes.io/docs/reference/kubectl/)
- [Minikube 文档](https://minikube.sigs.k8s.io/)
- [Docker 文档](https://docs.docker.com/)
