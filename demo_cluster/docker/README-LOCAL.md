# 本地开发部署指南

## 快速启动

### 1. 启动 Docker 基础服务

```bash
cd demo_cluster/docker
docker-compose -f docker-compose-local.yml up
```

这会启动：
- PostgreSQL (端口 5432)
- ETCD (端口 2379)
- NATS (端口 4222)

### 2. 在另一个终端启动游戏节点

```bash
cd demo_cluster/docker

# 启动 Center 节点
./game-server center --path=../config/demo-cluster.json --node=gc-center-1

# 启动 Gate 节点（新终端）
./game-server gate --path=../config/demo-cluster.json --node=gc-gate-1

# 启动 Game 节点（新终端）
./game-server game --path=../config/demo-cluster.json --node=gc-game-1

# 启动 Web 节点（新终端）
./game-server web --path=../config/demo-cluster.json --node=gc-web-1
```

### 3. 访问服务

- Web 服务: http://localhost:3013
- PostgreSQL: localhost:5432
- ETCD: localhost:2379
- NATS: localhost:4222

## 停止服务

```bash
# 停止 Docker 服务
docker-compose -f docker-compose-local.yml down

# 停止游戏节点（在各自的终端按 Ctrl+C）
```

## 清理数据

```bash
# 删除所有卷（数据库数据）
docker-compose -f docker-compose-local.yml down -v
```

## 查看日志

```bash
# 查看 Docker 服务日志
docker-compose -f docker-compose-local.yml logs -f

# 查看特定服务日志
docker-compose -f docker-compose-local.yml logs -f postgres
docker-compose -f docker-compose-local.yml logs -f etcd
docker-compose -f docker-compose-local.yml logs -f nats
```

## 故障排查

### PostgreSQL 连接失败

检查 PostgreSQL 是否运行：
```bash
docker ps | grep postgres
```

### ETCD 连接失败

检查 ETCD 是否运行：
```bash
docker ps | grep etcd
```

### NATS 连接失败

检查 NATS 是否运行：
```bash
docker ps | grep nats
```

## 环境变量

游戏节点会自动连接到：
- PostgreSQL: localhost:5432
- ETCD: http://localhost:2379
- NATS: nats://localhost:4222

如需修改，编辑 `../config/demo-cluster.json`
