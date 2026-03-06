# Docker 部署快速开始指南

## 5 分钟快速启动

### 前置条件

- Docker 已安装
- Docker Compose 已安装
- 游戏服务器二进制已编译

### 编译二进制（如果还没有）

```bash
cd demo_cluster
go build -o docker/game-server ./nodes/main.go
```

### 启动集群

```bash
cd demo_cluster/docker

# 一键启动所有基础服务
./start-docker-cluster.sh

# 等待脚本完成，然后在不同的终端启动游戏节点
```

### 启动游戏节点

在 4 个不同的终端中分别运行：

**终端 1 - Center 节点**
```bash
cd demo_cluster/docker
./game-server center --path=../config/demo-cluster.json --node=gc-center-1
```

**终端 2 - Gate 节点**
```bash
cd demo_cluster/docker
./game-server gate --path=../config/demo-cluster.json --node=gc-gate-1
```

**终端 3 - Game 节点**
```bash
cd demo_cluster/docker
./game-server game --path=../config/demo-cluster.json --node=gc-game-1
```

**终端 4 - Web 节点**
```bash
cd demo_cluster/docker
./game-server web --path=../config/demo-cluster.json --node=gc-web-1
```

### 验证部署

```bash
# 查看容器状态
./monitor-docker-cluster.sh --status

# 查看健康检查
./monitor-docker-cluster.sh --health

# 实时监控资源
./monitor-docker-cluster.sh --stats

# 查看日志
./monitor-docker-cluster.sh --logs
```

### 停止集群

```bash
# 停止但保留数据
./stop-docker-cluster.sh

# 停止并清理数据
./stop-docker-cluster.sh --clean
```

---

## 常用命令速查

### 启动和停止

```bash
# 启动集群
./start-docker-cluster.sh

# 停止集群
./stop-docker-cluster.sh

# 停止并清理数据
./stop-docker-cluster.sh --clean
```

### 监控和调试

```bash
# 显示容器状态
./monitor-docker-cluster.sh --status

# 实时监控资源使用
./monitor-docker-cluster.sh --stats

# 查看所有日志
./monitor-docker-cluster.sh --logs

# 查看特定服务日志
./monitor-docker-cluster.sh --logs-pg      # PostgreSQL
./monitor-docker-cluster.sh --logs-etcd    # ETCD
./monitor-docker-cluster.sh --logs-nats    # NATS

# 健康检查
./monitor-docker-cluster.sh --health

# 详细信息
./monitor-docker-cluster.sh --detailed
```

### 手动 Docker 命令

```bash
# 查看容器状态
docker-compose -f docker-compose-local.yml ps

# 查看日志
docker-compose -f docker-compose-local.yml logs -f

# 进入容器
docker exec -it demo-postgres psql -U postgres
docker exec -it demo-etcd sh
docker exec -it demo-nats sh

# 查看资源使用
docker stats

# 停止容器
docker-compose -f docker-compose-local.yml stop

# 启动容器
docker-compose -f docker-compose-local.yml start

# 重启容器
docker-compose -f docker-compose-local.yml restart
```

---

## 服务地址

| 服务 | 地址 | 端口 |
|------|------|------|
| Web | http://localhost:3013 | 3013 |
| Gate | localhost | 3011 |
| Game | localhost | 3012 |
| Center | localhost | 3010 |
| PostgreSQL | localhost | 5432 |
| ETCD | http://localhost:2379 | 2379 |
| NATS | nats://localhost:4222 | 4222 |
| NATS Monitor | http://localhost:8222 | 8222 |

---

## 常见问题

### Q: 如何查看游戏节点的日志？

A: 游戏节点在前台运行，日志直接输出到终端。如果要保存日志，可以重定向：

```bash
./game-server web --path=../config/demo-cluster.json --node=gc-web-1 > web.log 2>&1 &
```

### Q: 如何修改配置？

A: 编辑 `../config/demo-cluster.json` 文件，然后重启相应的节点。

### Q: 如何扩展游戏节点？

A: 启动多个游戏节点实例，使用不同的节点名称：

```bash
./game-server game --path=../config/demo-cluster.json --node=gc-game-1
./game-server game --path=../config/demo-cluster.json --node=gc-game-2
./game-server game --path=../config/demo-cluster.json --node=gc-game-3
```

### Q: 如何重置数据库？

A: 停止集群并清理数据：

```bash
./stop-docker-cluster.sh --clean
./start-docker-cluster.sh
```

### Q: 性能不如预期怎么办？

A: 参考 `DOCKER-DEPLOYMENT-GUIDE.md` 中的性能优化部分。

---

## 下一步

- 阅读 `DOCKER-DEPLOYMENT-GUIDE.md` 了解详细的部署和优化方案
- 阅读 `DEPLOYMENT-GUIDE.md` 了解不同的部署方式对比
- 查看 `K8S-DEPLOYMENT-GUIDE.md` 了解 Kubernetes 部署

