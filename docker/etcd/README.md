# Cherry框架 etcd Docker环境

基于`demo-cluster.json`配置的etcd Docker环境，用于Cherry框架的服务发现。

## 配置说明

根据`config/demo-cluster.json`中的etcd配置：

```json
"etcd": {
    "end_points": "dev.com:2379",
    "prefix": "cherry",
    "ttl": 5,
    "dial_timeout": 3,
    "dial_keep_alive_time": 1,
    "dial_keep_alive_timeout": 1,
    "user": "",
    "password": ""
}
```

## 快速启动

### 方式1：使用docker-compose（推荐）

```bash
# 启动etcd服务
cd docker/etcd
docker-compose up -d

# 查看服务状态
docker-compose ps

# 查看日志
docker-compose logs -f etcd

# 停止服务
docker-compose down
```

### 方式2：使用Docker直接运行

```bash
# 构建镜像
docker build -t cherry-etcd:latest .

# 运行容器
docker run -d \
  --name cherry-etcd \
  --hostname dev.com \
  -p 2379:2379 \
  -p 2380:2380 \
  -v etcd_data:/etcd-data \
  cherry-etcd:latest
```

## 服务访问

- **etcd客户端端口**: `http://dev.com:2379` 或 `http://localhost:2379`
- **etcd Web管理界面**: `http://localhost:8080`

## 主机名配置

由于配置中使用了`dev.com`作为主机名，需要在本地hosts文件中添加映射：

### macOS/Linux
```bash
echo "127.0.0.1 dev.com" | sudo tee -a /etc/hosts
```

### Windows
在`C:\Windows\System32\drivers\etc\hosts`文件中添加：
```
127.0.0.1 dev.com
```

## etcd客户端测试

### 安装etcdctl
```bash
# macOS
brew install etcd

# Ubuntu/Debian
apt-get install etcd-client

# 或使用Docker
alias etcdctl='docker exec cherry-etcd etcdctl'
```

### 基本操作测试
```bash
# 设置环境变量
export ETCDCTL_API=3
export ETCDCTL_ENDPOINTS=http://dev.com:2379

# 测试连接
etcdctl endpoint health

# 写入数据（模拟Cherry框架节点注册）
etcdctl put /cherry/nodes/gc-master-1 '{"nodeId":"gc-master-1","nodeType":"master","address":"127.0.0.1:8080"}'

# 读取数据
etcdctl get /cherry/nodes/gc-master-1

# 列出所有Cherry节点
etcdctl get /cherry/nodes/ --prefix

# 监听节点变化
etcdctl watch /cherry/nodes/ --prefix
```

## Cherry框架集成

在Cherry框架中使用etcd服务发现，需要修改配置：

```json
{
  "cluster": {
    "discovery": {
      "mode": "etcd"
    },
    "etcd": {
      "end_points": "dev.com:2379",
      "prefix": "cherry",
      "ttl": 5,
      "dial_timeout": 3,
      "dial_keep_alive_time": 1,
      "dial_keep_alive_timeout": 1
    }
  }
}
```

## 监控和维护

### 查看集群状态
```bash
# 集群成员信息
etcdctl member list

# 集群健康状态
etcdctl endpoint health

# 性能统计
etcdctl endpoint status --write-out=table
```

### 数据备份
```bash
# 创建快照
etcdctl snapshot save /backup/etcd-snapshot.db

# 恢复快照
etcdctl snapshot restore /backup/etcd-snapshot.db --data-dir=/etcd-data-restore
```

### 日志查看
```bash
# 查看etcd日志
docker-compose logs -f etcd

# 查看特定时间段的日志
docker-compose logs --since="2024-01-01T00:00:00" etcd
```

## 故障排除

### 常见问题

1. **连接失败**
   - 检查hosts文件是否配置了`dev.com`
   - 确认端口2379没有被占用
   - 检查防火墙设置

2. **数据丢失**
   - 检查数据卷是否正确挂载
   - 确认TTL设置是否合理

3. **性能问题**
   - 调整`quota-backend-bytes`参数
   - 启用自动压缩功能

### 重置环境
```bash
# 停止并删除容器
docker-compose down -v

# 删除数据卷
docker volume rm etcd_etcd_data

# 重新启动
docker-compose up -d
```

## 生产环境建议

1. **集群部署**: 生产环境建议部署3节点或5节点etcd集群
2. **数据备份**: 定期备份etcd数据
3. **监控告警**: 配置etcd监控和告警
4. **安全配置**: 启用TLS和身份验证
5. **资源限制**: 设置合适的内存和CPU限制

## 相关链接

- [etcd官方文档](https://etcd.io/docs/)
- [Cherry框架文档](https://github.com/cherry-game/cherry)
- [etcdkeeper Web UI](https://github.com/evildecay/etcdkeeper)