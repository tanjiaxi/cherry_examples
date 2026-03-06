# Kubernetes 部署完整方案

## 📋 项目概述

已为 demo_cluster 游戏服务器创建完整的 Kubernetes 部署方案，包括：

- ✅ 自动化部署脚本
- ✅ Kubernetes 配置文件
- ✅ Docker 镜像构建
- ✅ 详细文档和指南
- ✅ 故障排查方案
- ✅ 监控管理工具

## 📁 文件清单

### 核心部署文件

```
demo_cluster/docker/
├── k8s-deploy.sh                 # 主部署脚本（推荐）
├── quick-k8s-setup.sh            # 快速设置脚本（自动安装工具）
├── Dockerfile                    # Docker 镜像构建
├── k8s-namespace.yaml            # K8s 命名空间
├── k8s-postgres.yaml             # PostgreSQL 部署
├── k8s-etcd.yaml                 # ETCD 部署
├── k8s-nats.yaml                 # NATS 部署
└── k8s-game-nodes.yaml           # 游戏节点部署
```

### 文档文件

```
├── K8S-README.md                 # 快速开始指南 ⭐
├── K8S-DEPLOYMENT-GUIDE.md       # 详细部署指南
├── K8S-TROUBLESHOOTING.md        # 故障排查指南
├── K8S-MONITORING.md             # 监控管理指南
├── K8S-CHECKLIST.md              # 部署检查清单
├── K8S-DEPLOYMENT-SUMMARY.md     # 部署完成总结
└── K8S-COMPLETE.md               # 本文件
```

### 其他配置

```
├── docker-compose-local.yml      # 本地开发配置
├── docker-compose.yml            # 完整 Docker Compose
├── DEPLOYMENT-GUIDE.md           # 部署方式选择
├── README-LOCAL.md               # 本地开发指南
└── README.md                     # Docker 部署指南
```

## 🚀 快速开始

### 最简单的方式（推荐）

```bash
cd demo_cluster/docker
sh quick-k8s-setup.sh
```

这个脚本会自动：
1. 检查并安装必要工具
2. 启动 Minikube 集群
3. 构建 Docker 镜像
4. 部署所有服务
5. 显示访问信息

### 标准部署方式

```bash
cd demo_cluster/docker

# 检查集群
sh k8s-deploy.sh check

# 构建镜像
sh k8s-deploy.sh build

# 完整部署
sh k8s-deploy.sh deploy

# 查看状态
sh k8s-deploy.sh status

# 显示访问信息
sh k8s-deploy.sh access
```

## 🏗️ 部署架构

```
┌─────────────────────────────────────────────────────────┐
│                  Kubernetes Cluster                      │
├─────────────────────────────────────────────────────────┤
│                                                           │
│  ┌──────────────────────────────────────────────────┐   │
│  │         demo-cluster Namespace                   │   │
│  ├──────────────────────────────────────────────────┤   │
│  │                                                  │   │
│  │  基础服务：                                      │   │
│  │  ├─ PostgreSQL (1)  - 数据库                    │   │
│  │  ├─ ETCD (1)        - 服务发现                  │   │
│  │  └─ NATS (1)        - 消息队列                  │   │
│  │                                                  │   │
│  │  游戏节点：                                      │   │
│  │  ├─ Center (1)      - 中心服务                  │   │
│  │  ├─ Gate (2)        - 网关服务                  │   │
│  │  ├─ Game (2)        - 游戏服务                  │   │
│  │  └─ Web (2)         - Web 服务                  │   │
│  │                                                  │   │
│  └──────────────────────────────────────────────────┘   │
│                                                           │
└─────────────────────────────────────────────────────────┘
```

## 📊 部署时间表

| 步骤 | 时间 | 说明 |
|------|------|------|
| 安装工具 | 5-10 分钟 | 首次运行 |
| 启动 Minikube | 2-3 分钟 | 初始化集群 |
| 构建镜像 | 5-10 分钟 | 编译代码 |
| 部署服务 | 2-3 分钟 | 创建资源 |
| 等待就绪 | 2-3 分钟 | 启动 Pod |
| **总计** | **15-30 分钟** | 首次部署 |

## 🔌 服务端口

| 服务 | 端口 | 说明 |
|------|------|------|
| Web | 3013 | Web 服务 |
| Gate | 3011 | 网关服务 |
| Center | 3010 | 中心服务 |
| Game | 3012 | 游戏服务 |
| PostgreSQL | 5432 | 数据库 |
| ETCD | 2379 | 服务发现 |
| NATS | 4222 | 消息队列 |

## 📖 文档导航

### 🌟 必读文档

1. **[K8S-README.md](K8S-README.md)** - 快速开始指南
   - 快速部署步骤
   - 常用命令
   - 基本故障排查

2. **[K8S-DEPLOYMENT-GUIDE.md](K8S-DEPLOYMENT-GUIDE.md)** - 详细部署指南
   - 完整安装步骤
   - 环境配置
   - 访问方式
   - 扩展优化

### 🔧 参考文档

3. **[K8S-TROUBLESHOOTING.md](K8S-TROUBLESHOOTING.md)** - 故障排查指南
   - 常见问题解决
   - 调试技巧
   - 性能优化

4. **[K8S-MONITORING.md](K8S-MONITORING.md)** - 监控管理指南
   - 资源监控
   - 日志管理
   - 告警配置
   - 备份恢复

### ✅ 检查清单

5. **[K8S-CHECKLIST.md](K8S-CHECKLIST.md)** - 部署检查清单
   - 部署前检查
   - 部署步骤检查
   - 部署后验证
   - 功能测试

### 📝 总结文档

6. **[K8S-DEPLOYMENT-SUMMARY.md](K8S-DEPLOYMENT-SUMMARY.md)** - 部署完成总结
   - 完成情况总结
   - 快速开始
   - 下一步指南

## 💻 常用命令

### 部署命令

```bash
# 一键部署
sh k8s-deploy.sh deploy

# 检查集群
sh k8s-deploy.sh check

# 构建镜像
sh k8s-deploy.sh build

# 查看状态
sh k8s-deploy.sh status

# 显示访问信息
sh k8s-deploy.sh access

# 清理部署
sh k8s-deploy.sh clean

# 显示帮助
sh k8s-deploy.sh help
```

### kubectl 命令

```bash
# 查看 Pod
kubectl get pods -n demo-cluster

# 查看服务
kubectl get svc -n demo-cluster

# 查看日志
kubectl logs -f <pod-name> -n demo-cluster

# 进入容器
kubectl exec -it <pod-name> -n demo-cluster -- sh

# 查看资源使用
kubectl top pods -n demo-cluster

# 查看事件
kubectl get events -n demo-cluster

# 端口转发
kubectl port-forward svc/<service> <local>:<remote> -n demo-cluster
```

## 🔍 验证部署

### 检查 Pod 状态

```bash
kubectl get pods -n demo-cluster
```

所有 Pod 应该显示 `Running` 状态。

### 检查服务

```bash
kubectl get svc -n demo-cluster
```

所有服务应该显示正确的端口。

### 查看日志

```bash
kubectl logs -f -l app=web -n demo-cluster
```

日志应该显示服务正常启动。

### 测试连接

```bash
# 端口转发
kubectl port-forward svc/web 3013:3013 -n demo-cluster

# 访问 Web 服务
curl http://localhost:3013
```

## 🛠️ 故障排查

### 常见问题

1. **Pod 无法启动**
   - 查看日志：`kubectl logs <pod-name> -n demo-cluster`
   - 查看事件：`kubectl describe pod <pod-name> -n demo-cluster`
   - 参考：[K8S-TROUBLESHOOTING.md](K8S-TROUBLESHOOTING.md)

2. **服务无法连接**
   - 检查 Pod 状态：`kubectl get pods -n demo-cluster`
   - 检查服务：`kubectl get svc -n demo-cluster`
   - 参考：[K8S-TROUBLESHOOTING.md](K8S-TROUBLESHOOTING.md)

3. **资源不足**
   - 查看资源使用：`kubectl top nodes`
   - 增加 Minikube 资源：`minikube delete && minikube start --cpus=8 --memory=16384`
   - 参考：[K8S-TROUBLESHOOTING.md](K8S-TROUBLESHOOTING.md)

## 📈 监控和管理

### 启动 Dashboard

```bash
# Minikube
minikube dashboard

# 其他集群
kubectl proxy
# 访问: http://localhost:8001/api/v1/namespaces/kubernetes-dashboard/services/https:kubernetes-dashboard:/proxy/
```

### 查看资源使用

```bash
kubectl top pods -n demo-cluster
kubectl top nodes
```

### 查看日志

```bash
# 实时查看日志
kubectl logs -f -l app=web -n demo-cluster

# 查看历史日志
kubectl logs --tail=100 <pod-name> -n demo-cluster
```

详见：[K8S-MONITORING.md](K8S-MONITORING.md)

## 🔄 扩展和优化

### 扩展副本

```bash
# 扩展 Web 节点
kubectl scale deployment web --replicas=3 -n demo-cluster

# 扩展 Gate 节点
kubectl scale deployment gate --replicas=3 -n demo-cluster

# 扩展 Game 节点
kubectl scale deployment game --replicas=3 -n demo-cluster
```

### 更新镜像

```bash
# 重新构建镜像
docker build -f docker/Dockerfile -t demo-cluster:v2 .

# 更新部署
kubectl set image deployment/web web=demo-cluster:v2 -n demo-cluster
```

### 配置自动扩展

参考：[K8S-MONITORING.md](K8S-MONITORING.md) 中的 HPA 配置

## 🌐 部署选项对比

| 选项 | 优点 | 缺点 | 适用场景 |
|------|------|------|---------|
| Docker Compose | 简单快速 | 不支持分布式 | 本地开发 |
| Minikube | 接近生产 | 资源占用多 | 本地测试 |
| 云端 K8s | 生产级别 | 成本高 | 生产环境 |

详见：[DEPLOYMENT-GUIDE.md](DEPLOYMENT-GUIDE.md)

## 📋 系统要求

### 最低配置

- CPU：4 核
- 内存：8 GB
- 磁盘：50 GB

### 推荐配置

- CPU：8 核
- 内存：16 GB
- 磁盘：100 GB

### 支持的平台

- ✅ macOS（Intel 和 Apple Silicon）
- ✅ Linux（Ubuntu、CentOS 等）
- ✅ Windows（WSL2）
- ✅ 云端 K8s（EKS、GKE、AKS）

## 🎯 下一步

### 1. 立即部署

```bash
cd demo_cluster/docker
sh quick-k8s-setup.sh
```

### 2. 验证部署

```bash
kubectl get pods -n demo-cluster
kubectl logs -f -l app=web -n demo-cluster
```

### 3. 访问服务

```bash
kubectl port-forward svc/web 3013:3013 -n demo-cluster
# 访问: http://localhost:3013
```

### 4. 阅读文档

- [K8S-README.md](K8S-README.md) - 快速开始
- [K8S-DEPLOYMENT-GUIDE.md](K8S-DEPLOYMENT-GUIDE.md) - 详细指南
- [K8S-TROUBLESHOOTING.md](K8S-TROUBLESHOOTING.md) - 故障排查
- [K8S-MONITORING.md](K8S-MONITORING.md) - 监控管理

### 5. 配置监控

- 启动 Dashboard：`minikube dashboard`
- 配置告警：参考 [K8S-MONITORING.md](K8S-MONITORING.md)
- 配置备份：参考 [K8S-MONITORING.md](K8S-MONITORING.md)

## 📞 获取帮助

### 查看脚本帮助

```bash
sh k8s-deploy.sh help
```

### 查看 kubectl 帮助

```bash
kubectl --help
kubectl describe --help
kubectl logs --help
```

### 查看文档

- 快速问题：[K8S-README.md](K8S-README.md)
- 详细问题：[K8S-DEPLOYMENT-GUIDE.md](K8S-DEPLOYMENT-GUIDE.md)
- 故障排查：[K8S-TROUBLESHOOTING.md](K8S-TROUBLESHOOTING.md)
- 监控管理：[K8S-MONITORING.md](K8S-MONITORING.md)

## ✨ 特性

- ✅ 自动化部署脚本
- ✅ 完整的 Kubernetes 配置
- ✅ Docker 多阶段构建
- ✅ 详细的文档和指南
- ✅ 故障排查方案
- ✅ 监控和管理工具
- ✅ 性能优化建议
- ✅ 生产环境支持

## 📚 相关资源

- [Kubernetes 官方文档](https://kubernetes.io/docs/)
- [kubectl 命令参考](https://kubernetes.io/docs/reference/kubectl/)
- [Minikube 文档](https://minikube.sigs.k8s.io/)
- [Docker 文档](https://docs.docker.com/)
- [Cherry 框架文档](https://github.com/cherry-game/cherry)

## 📝 更新日志

### v1.0（2024-01-21）

- ✅ 创建 K8s 部署脚本
- ✅ 创建 K8s 配置文件
- ✅ 创建详细文档
- ✅ 创建故障排查指南
- ✅ 创建监控管理指南
- ✅ 创建部署检查清单
- ✅ 创建完整方案文档

## 🎉 准备好了吗？

```bash
cd demo_cluster/docker
sh quick-k8s-setup.sh
```

祝部署顺利！🚀

---

**Kubernetes 部署完整方案已准备就绪！**

所有文件和文档都已创建，可以立即开始部署。

如有任何问题，请参考相关文档或联系技术支持。
