# Docker 部署文档索引

## 📖 文档列表

### 🚀 快速开始（推荐从这里开始）
1. **[QUICK-START.md](QUICK-START.md)** ⭐⭐⭐
   - 5 分钟快速启动
   - 常用命令速查
   - 常见问题解答
   - **适合**: 第一次使用

### 📚 详细指南
2. **[DOCKER-DEPLOYMENT-GUIDE.md](DOCKER-DEPLOYMENT-GUIDE.md)** ⭐⭐⭐
   - 完整的部署步骤
   - 环境准备
   - 性能优化建议
   - 监控和调试
   - **适合**: 深入了解部署过程

3. **[PERFORMANCE-TUNING.md](PERFORMANCE-TUNING.md)** ⭐⭐
   - 系统级优化
   - Docker 配置优化
   - 容器资源优化
   - 网络性能优化
   - 存储性能优化
   - **适合**: 性能优化

4. **[ARCHITECTURE.md](ARCHITECTURE.md)** ⭐⭐
   - 系统架构图
   - 容器部署架构
   - 网络通信流程
   - 数据流向
   - **适合**: 理解系统设计

### 🔄 部署方式对比
5. **[DEPLOYMENT-GUIDE.md](DEPLOYMENT-GUIDE.md)** ⭐
   - 三种部署方式对比
   - 本地开发
   - Docker Compose
   - Kubernetes
   - **适合**: 选择合适的部署方式

### 🐳 Kubernetes 部署（进阶）
6. **[K8S-DEPLOYMENT-GUIDE.md](K8S-DEPLOYMENT-GUIDE.md)**
   - Kubernetes 部署指南
   - **适合**: 生产环境部署

---

## 🛠️ 脚本工具

### 启动脚本
```bash
./start-docker-cluster.sh
```
- 启动所有基础服务
- 检查依赖
- 验证连接
- 显示启动信息

### 停止脚本
```bash
./stop-docker-cluster.sh              # 停止但保留数据
./stop-docker-cluster.sh --clean      # 停止并清理数据
```
- 停止容器
- 移除容器
- 可选清理数据

### 监控脚本
```bash
./monitor-docker-cluster.sh --status      # 显示状态
./monitor-docker-cluster.sh --stats       # 实时监控
./monitor-docker-cluster.sh --logs        # 查看日志
./monitor-docker-cluster.sh --health      # 健康检查
./monitor-docker-cluster.sh --detailed    # 详细信息
```
- 监控容器状态
- 查看资源使用
- 查看日志
- 健康检查

---

## 📋 配置文件

| 文件 | 用途 | 说明 |
|------|------|------|
| `docker-compose-local.yml` | 本地开发 | 仅包含基础服务 |
| `docker-compose.yml` | 完整部署 | 包含所有服务 |
| `Dockerfile` | 镜像定义 | 游戏服务器镜像 |
| `nats.conf` | NATS 配置 | 消息队列配置 |

---

## 🎯 使用场景

### 场景 1: 本地开发
```bash
# 1. 启动基础服务
./start-docker-cluster.sh

# 2. 启动游戏节点（本地运行）
./game-server web --path=../config/demo-cluster.json --node=gc-web-1
```
**文档**: [QUICK-START.md](QUICK-START.md)

### 场景 2: 功能测试
```bash
# 1. 启动所有服务
docker-compose up -d

# 2. 运行测试
cd ../robot_client
go run main.go
```
**文档**: [DOCKER-DEPLOYMENT-GUIDE.md](DOCKER-DEPLOYMENT-GUIDE.md)

### 场景 3: 性能测试
```bash
# 1. 应用性能优化
# 参考 PERFORMANCE-TUNING.md

# 2. 运行压力测试
cd ../robot_client
go run main.go --clients=100 --duration=60s
```
**文档**: [PERFORMANCE-TUNING.md](PERFORMANCE-TUNING.md)

### 场景 4: 生产部署
```bash
# 迁移到 Kubernetes
kubectl apply -f k8s-namespace.yaml
kubectl apply -f k8s-postgres.yaml
# ...
```
**文档**: [K8S-DEPLOYMENT-GUIDE.md](K8S-DEPLOYMENT-GUIDE.md)

---

## 📊 性能指标

| 指标 | 目标值 | 优化前 | 优化后 |
|------|--------|--------|--------|
| 吞吐量 | 9500+ req/s | 8000 | 9500 |
| 延迟 | <10ms | 50ms | 10ms |
| CPU | <20% | 45% | 18% |
| 内存 | <300MB | 400MB | 250MB |

**详见**: [PERFORMANCE-TUNING.md](PERFORMANCE-TUNING.md)

---

## 🔍 常见问题

### Q: 如何快速启动？
A: 参考 [QUICK-START.md](QUICK-START.md)

### Q: 如何优化性能？
A: 参考 [PERFORMANCE-TUNING.md](PERFORMANCE-TUNING.md)

### Q: 如何理解架构？
A: 参考 [ARCHITECTURE.md](ARCHITECTURE.md)

### Q: 如何选择部署方式？
A: 参考 [DEPLOYMENT-GUIDE.md](DEPLOYMENT-GUIDE.md)

### Q: 如何部署到 Kubernetes？
A: 参考 [K8S-DEPLOYMENT-GUIDE.md](K8S-DEPLOYMENT-GUIDE.md)

---

## 📞 获取帮助

### 查看脚本帮助
```bash
./start-docker-cluster.sh --help
./stop-docker-cluster.sh --help
./monitor-docker-cluster.sh --help
```

### 查看日志
```bash
./monitor-docker-cluster.sh --logs
./monitor-docker-cluster.sh --logs-pg
./monitor-docker-cluster.sh --logs-etcd
./monitor-docker-cluster.sh --logs-nats
```

### 查看状态
```bash
./monitor-docker-cluster.sh --status
./monitor-docker-cluster.sh --health
./monitor-docker-cluster.sh --detailed
```

---

## 🗺️ 学习路径

### 初级（第一次使用）
1. 阅读 [QUICK-START.md](QUICK-START.md)
2. 运行 `./start-docker-cluster.sh`
3. 启动游戏节点
4. 验证部署

### 中级（深入了解）
1. 阅读 [DOCKER-DEPLOYMENT-GUIDE.md](DOCKER-DEPLOYMENT-GUIDE.md)
2. 理解部署步骤
3. 学习监控和调试
4. 解决常见问题

### 高级（性能优化）
1. 阅读 [PERFORMANCE-TUNING.md](PERFORMANCE-TUNING.md)
2. 应用优化建议
3. 运行性能测试
4. 分析性能指标

### 专家（架构设计）
1. 阅读 [ARCHITECTURE.md](ARCHITECTURE.md)
2. 理解系统设计
3. 学习扩展方案
4. 迁移到 Kubernetes

---

## 📈 下一步

- ✅ 完成快速启动
- ✅ 理解部署过程
- ✅ 优化性能
- ⬜ 迁移到 Kubernetes
- ⬜ 配置监控告警
- ⬜ 实施灰度发布

---

## 📚 相关资源

- [Docker 官方文档](https://docs.docker.com/)
- [Docker Compose 文档](https://docs.docker.com/compose/)
- [PostgreSQL 文档](https://www.postgresql.org/docs/)
- [ETCD 文档](https://etcd.io/docs/)
- [NATS 文档](https://docs.nats.io/)
- [Kubernetes 文档](https://kubernetes.io/docs/)

---

## 📝 版本历史

### v1.0.0 (2024-01-22)
- 初始版本
- 包含快速启动脚本
- 包含性能优化指南
- 包含监控脚本
- 包含架构文档

---

## 🤝 贡献

欢迎提交问题和改进建议。

---

**最后更新**: 2024-01-22
**维护者**: DevOps Team
