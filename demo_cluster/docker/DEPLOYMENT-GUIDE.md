# 部署方式选择指南

## 三种部署方式对比

| 特性 | 方式 1: 本地开发 | 方式 2: Docker Compose | 方式 3: Kubernetes |
|------|-----------------|----------------------|-------------------|
| **适用场景** | 本地开发调试 | 单机测试/演示 | 生产环境/多机部署 |
| **复杂度** | 低 | 中 | 高 |
| **性能** | 最高 | 中等 | 中等 |
| **可扩展性** | 不支持 | 有限 | 完全支持 |
| **自动恢复** | 否 | 否 | 是 |
| **负载均衡** | 否 | 否 | 是 |
| **监控告警** | 否 | 否 | 支持集成 |
| **网络隔离** | 否 | 是 | 是 |
| **持久化存储** | 本地 | Docker 卷 | PVC |
| **配置管理** | 文件 | 环境变量 | ConfigMap/Secret |

---

## 方式 1: 本地开发（推荐用于开发阶段）

### 使用场景
- ✅ 本地开发和调试
- ✅ 快速迭代测试
- ✅ 学习和理解系统
- ✅ 性能基准测试

### 不适用场景
- ❌ 生产环境
- ❌ 多机部署
- ❌ 需要自动恢复
- ❌ 需要负载均衡

### 启动步骤

```bash
# 1. 启动 Docker 基础服务
cd demo_cluster/docker
docker-compose -f docker-compose-local.yml up

# 2. 在新终端启动游戏节点
cd demo_cluster/docker

# 启动 Center
./game-server center --path=../config/demo-cluster.json --node=gc-center-1

# 启动 Gate（新终端）
./game-server gate --path=../config/demo-cluster.json --node=gc-gate-1

# 启动 Game（新终端）
./game-server game --path=../config/demo-cluster.json --node=gc-game-1

# 启动 Web（新终端）
./game-server web --path=../config/demo-cluster.json --node=gc-web-1
```

### 优点
- 最快的启动速度
- 直接调试代码
- 完整的日志输出
- 易于修改配置

### 缺点
- 需要手动管理进程
- 无自动恢复
- 不支持多机
- 需要本地编译

---

## 方式 2: Docker Compose（推荐用于测试/演示）

### 使用场景
- ✅ 单机完整部署
- ✅ 功能测试
- ✅ 演示和展示
- ✅ CI/CD 集成测试
- ✅ 压力测试

### 不适用场景
- ❌ 生产环境（单点故障）
- ❌ 多机部署
- ❌ 需要自动扩展
- ❌ 需要高可用

### 启动步骤

```bash
cd demo_cluster/docker

# 启动所有服务
docker-compose up -d

# 查看状态
docker-compose ps

# 查看日志
docker-compose logs -f

# 停止服务
docker-compose down

# 清理数据
docker-compose down -v
```

### 优点
- 一键启动所有服务
- 自动管理依赖关系
- 容器隔离
- 易于清理

### 缺点
- 需要网络访问 Docker Hub（可能失败）
- 单点故障
- 无自动恢复
- 不支持多机

### 常见问题

**问题：镜像拉取失败**
```bash
# 解决方案：使用本地编译的二进制
# 或配置 Docker 镜像源
bash fix-docker-mirror.sh
```

---

## 方式 3: Kubernetes（推荐用于生产环境）

### 使用场景
- ✅ 生产环境部署
- ✅ 多机集群
- ✅ 需要高可用
- ✅ 需要自动扩展
- ✅ 需要监控告警
- ✅ 需要灰度发布

### 不适用场景
- ❌ 本地开发（过于复杂）
- ❌ 快速原型（学习成本高）
- ❌ 没有 K8s 集群

### 启动步骤

```bash
cd demo_cluster/docker

# 1. 构建镜像
docker build -f Dockerfile -t demo-cluster:latest ..

# 2. 加载到 Minikube（如果使用 Minikube）
minikube image load demo-cluster:latest

# 3. 创建命名空间
kubectl apply -f k8s-namespace.yaml

# 4. 部署基础服务
kubectl apply -f k8s-postgres.yaml
kubectl apply -f k8s-etcd.yaml
kubectl apply -f k8s-nats.yaml

# 5. 等待基础服务就绪
kubectl wait --for=condition=ready pod -l app=postgres -n demo-cluster --timeout=300s

# 6. 部署游戏节点
kubectl apply -f k8s-game-nodes.yaml

# 7. 查看状态
kubectl get pods -n demo-cluster
kubectl get svc -n demo-cluster

# 8. 查看日志
kubectl logs -f <pod-name> -n demo-cluster

# 9. 端口转发（本地访问）
kubectl port-forward svc/web 3013:3013 -n demo-cluster

# 10. 删除部署
kubectl delete namespace demo-cluster
```

### 优点
- 自动恢复故障 Pod
- 自动扩展副本数
- 负载均衡
- 灰度发布
- 完整的监控体系
- 支持多机集群

### 缺点
- 学习成本高
- 配置复杂
- 需要 K8s 集群
- 调试困难

### 高级功能

**自动扩展（HPA）**
```bash
kubectl autoscale deployment gate --min=2 --max=10 -n demo-cluster
```

**灰度发布**
```bash
# 更新镜像
kubectl set image deployment/web web=demo-cluster:v2 -n demo-cluster

# 查看发布进度
kubectl rollout status deployment/web -n demo-cluster

# 回滚
kubectl rollout undo deployment/web -n demo-cluster
```

**监控和日志**
```bash
# 查看资源使用
kubectl top pods -n demo-cluster

# 查看事件
kubectl get events -n demo-cluster

# 进入 Pod 调试
kubectl exec -it <pod-name> -n demo-cluster -- sh
```

---

## 选择决策树

```
开始
  ↓
是否本地开发？
  ├─ 是 → 使用方式 1（本地开发）
  └─ 否
      ↓
    是否需要多机部署？
      ├─ 是 → 使用方式 3（Kubernetes）
      └─ 否
          ↓
        是否需要自动恢复/扩展？
          ├─ 是 → 使用方式 3（Kubernetes）
          └─ 否 → 使用方式 2（Docker Compose）
```

---

## 快速参考

### 方式 1 命令
```bash
# 启动
docker-compose -f docker-compose-local.yml up
./game-server web --path=../config/demo-cluster.json --node=gc-web-1

# 停止
docker-compose -f docker-compose-local.yml down
# 按 Ctrl+C 停止游戏节点
```

### 方式 2 命令
```bash
# 启动
docker-compose up -d

# 查看
docker-compose ps
docker-compose logs -f

# 停止
docker-compose down
docker-compose down -v  # 清理数据
```

### 方式 3 命令
```bash
# 启动
kubectl apply -f k8s-namespace.yaml
kubectl apply -f k8s-postgres.yaml
kubectl apply -f k8s-etcd.yaml
kubectl apply -f k8s-nats.yaml
kubectl apply -f k8s-game-nodes.yaml

# 查看
kubectl get pods -n demo-cluster
kubectl logs -f <pod-name> -n demo-cluster

# 停止
kubectl delete namespace demo-cluster
```

---

## 性能对比

| 操作 | 方式 1 | 方式 2 | 方式 3 |
|------|--------|--------|--------|
| 启动时间 | ~5s | ~30s | ~60s |
| 内存占用 | 最低 | 中等 | 较高 |
| CPU 占用 | 最低 | 中等 | 中等 |
| 网络延迟 | 最低 | 低 | 低 |
| 吞吐量 | 最高 | 中等 | 中等 |

---

## 总结

- **开发阶段**：使用方式 1（本地开发）
- **测试/演示**：使用方式 2（Docker Compose）
- **生产环境**：使用方式 3（Kubernetes）
