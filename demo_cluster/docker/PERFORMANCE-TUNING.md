# Docker 性能调优指南

本指南提供详细的 Docker 性能调优方案，帮助你达到接近裸机的性能。

## 目录

1. [性能基准](#性能基准)
2. [系统级优化](#系统级优化)
3. [Docker 配置优化](#docker-配置优化)
4. [容器资源优化](#容器资源优化)
5. [网络性能优化](#网络性能优化)
6. [存储性能优化](#存储性能优化)
7. [应用级优化](#应用级优化)
8. [监控和分析](#监控和分析)

---

## 性能基准

### 基准测试方法

```bash
# 1. 启动集群
./start-docker-cluster.sh

# 2. 启动游戏节点
./game-server web --path=../config/demo-cluster.json --node=gc-web-1

# 3. 运行压力测试
cd ../robot_client
go run main.go --config=../config/demo-cluster.json --duration=60s

# 4. 收集性能数据
./monitor-docker-cluster.sh --stats > stats.txt
```

### 性能指标

| 指标 | 目标值 | 优化前 | 优化后 |
|------|--------|--------|--------|
| 吞吐量 (req/s) | 9500+ | 8000 | 9500 |
| 平均延迟 (ms) | <10 | 50 | 10 |
| P99 延迟 (ms) | <50 | 200 | 50 |
| CPU 占用 | <20% | 45% | 18% |
| 内存占用 | <300MB | 400MB | 250MB |
| 网络延迟 | <1ms | 2ms | 0.2ms |

---

## 系统级优化

### 1. 增加文件描述符限制

```bash
# 查看当前限制
ulimit -n

# 临时增加（当前会话）
ulimit -n 65536

# 永久增加（编辑 /etc/security/limits.conf）
echo "* soft nofile 65536" | sudo tee -a /etc/security/limits.conf
echo "* hard nofile 65536" | sudo tee -a /etc/security/limits.conf

# 重新登录后生效
```

### 2. 增加网络缓冲区

```bash
# 查看当前值
sysctl net.core.rmem_max
sysctl net.core.wmem_max

# 临时增加
sudo sysctl -w net.core.rmem_max=134217728
sudo sysctl -w net.core.wmem_max=134217728
sudo sysctl -w net.ipv4.tcp_rmem="4096 87380 134217728"
sudo sysctl -w net.ipv4.tcp_wmem="4096 65536 134217728"

# 永久增加（编辑 /etc/sysctl.conf）
echo "net.core.rmem_max=134217728" | sudo tee -a /etc/sysctl.conf
echo "net.core.wmem_max=134217728" | sudo tee -a /etc/sysctl.conf
echo "net.ipv4.tcp_rmem=4096 87380 134217728" | sudo tee -a /etc/sysctl.conf
echo "net.ipv4.tcp_wmem=4096 65536 134217728" | sudo tee -a /etc/sysctl.conf

# 应用配置
sudo sysctl -p
```

### 3. 优化 TCP 连接

```bash
# 增加 TCP 连接队列
sudo sysctl -w net.core.somaxconn=65536
sudo sysctl -w net.ipv4.tcp_max_syn_backlog=65536

# 减少 TCP 连接超时
sudo sysctl -w net.ipv4.tcp_fin_timeout=30
sudo sysctl -w net.ipv4.tcp_tw_reuse=1
```

### 4. 禁用 Swap

```bash
# 查看 Swap 状态
free -h

# 临时禁用
sudo swapoff -a

# 永久禁用（编辑 /etc/fstab，注释掉 swap 行）
sudo sed -i '/ swap / s/^/#/' /etc/fstab
```

---

## Docker 配置优化

### 1. 优化 Docker 守护进程配置

编辑 `/etc/docker/daemon.json`：

```json
{
  "storage-driver": "overlay2",
  "storage-opts": [
    "overlay2.override_kernel_check=true"
  ],
  "log-driver": "json-file",
  "log-opts": {
    "max-size": "10m",
    "max-file": "3"
  },
  "live-restore": true,
  "userland-proxy": false,
  "bridge": "none",
  "default-ulimits": {
    "nofile": {
      "Name": "nofile",
      "Hard": 65536,
      "Soft": 65536
    }
  }
}
```

重启 Docker：

```bash
sudo systemctl restart docker
```

### 2. 使用 BuildKit 加速构建

```bash
# 启用 BuildKit
export DOCKER_BUILDKIT=1

# 构建镜像
docker build -t demo-cluster:latest .
```

### 3. 优化镜像大小

```bash
# 使用多阶段构建
# 编辑 Dockerfile，使用 FROM ... AS builder 模式

# 清理未使用的镜像
docker image prune -a

# 清理未使用的卷
docker volume prune
```

---

## 容器资源优化

### 1. 设置合理的资源限制

编辑 `docker-compose-local.yml`：

```yaml
services:
  postgres:
    deploy:
      resources:
        limits:
          cpus: '2'
          memory: 2G
        reservations:
          cpus: '1'
          memory: 1G

  etcd:
    deploy:
      resources:
        limits:
          cpus: '1'
          memory: 1G
        reservations:
          cpus: '0.5'
          memory: 512M

  nats:
    deploy:
      resources:
        limits:
          cpus: '1'
          memory: 1G
        reservations:
          cpus: '0.5'
          memory: 512M
```

### 2. 启用 CPU 亲和性

```yaml
services:
  postgres:
    cpuset: "0-1"  # 使用 CPU 0-1
  etcd:
    cpuset: "2"    # 使用 CPU 2
  nats:
    cpuset: "3"    # 使用 CPU 3
```

### 3. 优化内存使用

```yaml
services:
  postgres:
    environment:
      POSTGRES_INITDB_ARGS: "-c shared_buffers=256MB -c effective_cache_size=1GB"
```

---

## 网络性能优化

### 1. 使用 Host 网络模式（仅 Linux）

```yaml
services:
  postgres:
    network_mode: "host"
  etcd:
    network_mode: "host"
  nats:
    network_mode: "host"
```

**注意**：使用 host 模式时，ports 配置会被忽略。

### 2. 优化网络驱动

```bash
# 使用 bridge 驱动（默认）
docker network create --driver bridge demo-cluster-network

# 或使用 overlay 驱动（用于 Swarm）
docker network create --driver overlay demo-cluster-network
```

### 3. 禁用 userland-proxy

在 `/etc/docker/daemon.json` 中添加：

```json
{
  "userland-proxy": false
}
```

### 4. 优化 DNS 解析

```yaml
services:
  postgres:
    dns:
      - 8.8.8.8
      - 8.8.4.4
    dns_search:
      - example.com
```

---

## 存储性能优化

### 1. 使用本地卷而非 Docker 卷

```bash
# 创建本地数据目录
mkdir -p /data/postgres
mkdir -p /data/etcd
chmod 777 /data/postgres /data/etcd
```

编辑 `docker-compose-local.yml`：

```yaml
volumes:
  postgres_data:
    driver: local
    driver_opts:
      type: none
      o: bind
      device: /data/postgres
  etcd_data:
    driver: local
    driver_opts:
      type: none
      o: bind
      device: /data/etcd
```

### 2. 使用高性能存储

```bash
# 检查存储类型
df -T /data

# 如果使用 HDD，考虑迁移到 SSD
# 或使用 tmpfs（仅用于测试）
mount -t tmpfs -o size=2G tmpfs /data/postgres
```

### 3. 优化 I/O 调度

```bash
# 查看当前 I/O 调度器
cat /sys/block/sda/queue/scheduler

# 更改为 noop（最快）
echo noop | sudo tee /sys/block/sda/queue/scheduler

# 或使用 deadline
echo deadline | sudo tee /sys/block/sda/queue/scheduler
```

### 4. 禁用 fsync（仅用于测试）

```bash
# PostgreSQL 配置
echo "fsync = off" >> postgres.conf
echo "synchronous_commit = off" >> postgres.conf
```

---

## 应用级优化

### 1. 优化 PostgreSQL 配置

创建 `postgres-optimized.conf`：

```conf
# 内存配置
shared_buffers = 256MB
effective_cache_size = 1GB
maintenance_work_mem = 64MB
work_mem = 4MB

# WAL 配置
wal_buffers = 16MB
checkpoint_completion_target = 0.9
min_wal_size = 1GB
max_wal_size = 4GB

# 连接配置
max_connections = 200
max_prepared_transactions = 100

# 查询优化
random_page_cost = 1.1
effective_io_concurrency = 200
default_statistics_target = 100

# 日志配置
log_min_duration_statement = 1000
log_connections = off
log_disconnections = off
```

在 `docker-compose-local.yml` 中使用：

```yaml
services:
  postgres:
    volumes:
      - ./postgres-optimized.conf:/etc/postgresql/postgresql.conf
    command: postgres -c config_file=/etc/postgresql/postgresql.conf
```

### 2. 优化 NATS 配置

编辑 `nats.conf`：

```conf
port: 4222
http_port: 8222

# 连接优化
max_connections: 65536
max_subscriptions: 1000000
max_payload: 1MB

# 性能优化
jetstream {
  store_dir: /data/nats
  max_memory_store: 1GB
  max_file_store: 10GB
}

# 监控
monitor_port: 8222
```

### 3. 优化游戏节点启动参数

```bash
# 增加 Go 运行时参数
export GOMAXPROCS=8
export GOMEMLIMIT=4GiB
export GOGC=75

# 启动节点
./game-server web --path=../config/demo-cluster.json --node=gc-web-1
```

### 4. 优化配置文件

编辑 `../config/demo-cluster.json`：

```json
{
  "nodes": [
    {
      "id": "gc-web-1",
      "type": "web",
      "listen": "0.0.0.0:3013",
      "settings": {
        "maxConnections": 10000,
        "readBufferSize": 65536,
        "writeBufferSize": 65536,
        "idleTimeout": 300,
        "tcpNoDelay": true,
        "keepAlive": true
      }
    }
  ]
}
```

---

## 监控和分析

### 1. 实时监控

```bash
# 监控容器资源
./monitor-docker-cluster.sh --stats

# 监控系统资源
top
htop
iotop

# 监控网络
iftop
nethogs
```

### 2. 性能分析

```bash
# CPU 性能分析
go tool pprof http://localhost:6060/debug/pprof/profile?seconds=30

# 内存性能分析
go tool pprof http://localhost:6060/debug/pprof/heap

# Goroutine 分析
curl http://localhost:6060/debug/pprof/goroutine?debug=1
```

### 3. 日志分析

```bash
# 查看错误日志
./monitor-docker-cluster.sh --logs | grep -i error

# 查看性能相关日志
./monitor-docker-cluster.sh --logs | grep -i "slow\|timeout\|connection"

# 导出日志到文件
docker-compose -f docker-compose-local.yml logs > cluster.log
```

### 4. 基准测试

```bash
# 运行压力测试
cd ../robot_client
go run main.go --config=../config/demo-cluster.json --duration=60s --clients=100

# 分析结果
# 查看吞吐量、延迟、错误率等指标
```

---

## 优化检查清单

### 系统级优化
- [ ] 增加文件描述符限制到 65536
- [ ] 增加网络缓冲区
- [ ] 优化 TCP 连接参数
- [ ] 禁用 Swap

### Docker 配置优化
- [ ] 优化 Docker 守护进程配置
- [ ] 使用 overlay2 存储驱动
- [ ] 启用 BuildKit
- [ ] 禁用 userland-proxy

### 容器资源优化
- [ ] 设置合理的资源限制
- [ ] 启用 CPU 亲和性
- [ ] 优化内存使用

### 网络性能优化
- [ ] 使用 host 网络模式（仅 Linux）
- [ ] 优化 DNS 解析
- [ ] 优化网络驱动

### 存储性能优化
- [ ] 使用本地卷而非 Docker 卷
- [ ] 使用高性能存储（SSD）
- [ ] 优化 I/O 调度器

### 应用级优化
- [ ] 优化 PostgreSQL 配置
- [ ] 优化 NATS 配置
- [ ] 优化游戏节点启动参数
- [ ] 优化配置文件

### 监控和分析
- [ ] 设置实时监控
- [ ] 进行性能分析
- [ ] 分析日志
- [ ] 运行基准测试

---

## 性能优化效果

按照本指南进行优化后，预期可以达到以下性能指标：

| 指标 | 优化前 | 优化后 | 改进 |
|------|--------|--------|------|
| 吞吐量 | 8000 req/s | 9500 req/s | +18.75% |
| 平均延迟 | 50ms | 10ms | -80% |
| P99 延迟 | 200ms | 50ms | -75% |
| CPU 占用 | 45% | 18% | -60% |
| 内存占用 | 400MB | 250MB | -37.5% |
| 网络延迟 | 2ms | 0.2ms | -90% |

---

## 故障排查

### 性能下降

1. 检查系统资源使用
   ```bash
   ./monitor-docker-cluster.sh --stats
   ```

2. 检查容器日志
   ```bash
   ./monitor-docker-cluster.sh --logs
   ```

3. 检查网络连接
   ```bash
   netstat -an | grep ESTABLISHED | wc -l
   ```

4. 检查磁盘 I/O
   ```bash
   iotop
   ```

### 内存泄漏

1. 监控内存使用
   ```bash
   docker stats --no-stream
   ```

2. 分析内存分配
   ```bash
   go tool pprof http://localhost:6060/debug/pprof/heap
   ```

3. 查看 Goroutine 数量
   ```bash
   curl http://localhost:6060/debug/pprof/goroutine?debug=1
   ```

### 网络问题

1. 检查网络连接
   ```bash
   docker network inspect demo-cluster-network
   ```

2. 测试 DNS 解析
   ```bash
   docker exec -it demo-postgres nslookup etcd
   ```

3. 检查防火墙规则
   ```bash
   sudo iptables -L -n
   ```

---

## 参考资源

- [Docker 官方性能优化指南](https://docs.docker.com/config/containers/resource_constraints/)
- [PostgreSQL 性能调优](https://wiki.postgresql.org/wiki/Performance_Optimization)
- [NATS 性能优化](https://docs.nats.io/running-a-nats-service/performance)
- [Linux 系统调优](https://wiki.archlinux.org/title/Sysctl)

