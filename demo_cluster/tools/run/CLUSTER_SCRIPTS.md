# 游戏集群管理脚本

一键启动、停止、重启和查看游戏集群所有节点的便捷脚本。

## 脚本列表

### 1. start_all.sh - 启动所有节点
一次性启动所有游戏集群节点（按正确顺序）。

```bash
cd demo_cluster
./start_all.sh
```

**启动顺序**：
1. gc-center (中心节点)
2. gc-web-1 (Web节点)
3. gc-gate-1 (网关节点)
4. gc-game-10001 (游戏节点)

**特性**：
- ✅ 后台运行，不阻塞终端
- ✅ 自动创建日志文件（logs/*.log）
- ✅ 保存进程 PID（logs/*.pid）
- ✅ 彩色输出，清晰易读

### 2. stop_all.sh - 停止所有节点
停止所有运行中的节点。

```bash
./stop_all.sh
```

**特性**：
- ✅ 优雅停止（先 SIGTERM，再 SIGKILL）
- ✅ 按相反顺序停止（先游戏节点，后中心节点）
- ✅ 清理残留进程
- ✅ 删除 PID 文件

### 3. status.sh - 查看节点状态
查看所有节点的运行状态。

```bash
./status.sh
```

**显示信息**：
- ✅ 节点运行状态（运行中/已停止/未启动）
- ✅ 进程 PID
- ✅ CPU 和内存使用率
- ✅ 运行时间
- ✅ 日志文件大小

### 4. restart_all.sh - 重启所有节点
停止并重新启动所有节点。

```bash
./restart_all.sh
```

**流程**：
1. 停止所有节点
2. 等待 3 秒
3. 启动所有节点

## 使用示例

### 首次启动
```bash
cd demo_cluster
./start_all.sh
```

### 查看状态
```bash
./status.sh
```

### 查看日志
```bash
# 实时查看某个节点的日志
tail -f logs/gc-center.log
tail -f logs/gc-web-1.log
tail -f logs/gc-gate-1.log
tail -f logs/10001.log

# 查看所有日志
tail -f logs/*.log
```

### 停止服务
```bash
./stop_all.sh
```

### 重启服务
```bash
./restart_all.sh
```

## 日志文件

所有日志文件保存在 `logs/` 目录：

```
logs/
├── gc-center.log      # 中心节点日志
├── gc-center.pid      # 中心节点 PID
├── gc-web-1.log       # Web节点日志
├── gc-web-1.pid       # Web节点 PID
├── gc-gate-1.log      # 网关节点日志
├── gc-gate-1.pid      # 网关节点 PID
├── 10001.log          # 游戏节点日志
└── 10001.pid          # 游戏节点 PID
```

## 故障排查

### 节点启动失败
1. 查看日志文件：`cat logs/gc-center.log`
2. 检查端口占用：`lsof -i :8080`
3. 检查配置文件：`cat ../config/demo-cluster.json`

### 端口被占用
```bash
# 查找占用端口的进程
lsof -i :8080
lsof -i :3250

# 杀死进程
kill -9 <PID>
```

### 清理所有进程
```bash
# 强制杀死所有相关进程
pkill -9 -f "go run.*demo_cluster/nodes/main.go"

# 清理 PID 文件
rm -f logs/*.pid
```

### 清理日志文件
```bash
# 删除所有日志
rm -f logs/*.log

# 或者只清空日志内容
> logs/gc-center.log
> logs/gc-web-1.log
> logs/gc-gate-1.log
> logs/10001.log
```

## 访问地址

启动成功后，可以访问：

- **Web 界面**: http://localhost:8080
- **API 接口**: http://localhost:8080/api/*

## 注意事项

1. **启动顺序很重要**：必须先启动 center，再启动其他节点
2. **端口冲突**：确保所需端口未被占用
3. **配置文件**：确保 `config/demo-cluster.json` 存在且正确
4. **日志监控**：建议启动后查看日志确认无错误
5. **资源占用**：4个节点会占用一定的 CPU 和内存

## 高级用法

### 只启动特定节点
```bash
# 手动启动单个节点
cd demo_cluster
go run nodes/main.go center --path=../config/demo-cluster.json --node=gc-center &
```

### 自定义日志位置
修改 `start_all.sh` 中的 `log_file` 变量。

### 开机自启动
可以将 `start_all.sh` 添加到系统启动项（需要使用绝对路径）。

## 脚本维护

如果需要添加或删除节点，修改以下文件：
- `start_all.sh` - 添加/删除 `start_node` 调用
- `stop_all.sh` - 添加/删除 `stop_node` 调用
- `status.sh` - 添加/删除 `check_node` 调用

## 相关文档

- [Cherry 框架文档](https://github.com/cherry-game/cherry)
- [项目 README](./README.md)
- [配置文件说明](../config/README.md)