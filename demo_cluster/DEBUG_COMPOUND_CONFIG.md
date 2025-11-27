# VSCode 复合调试配置

## 如何添加一键启动所有节点的调试配置

由于 `.vscode/launch.json` 文件被保护，请手动添加以下配置。

### 步骤 1: 打开 launch.json

在 VSCode 中打开 `.vscode/launch.json` 文件。

### 步骤 2: 添加 compounds 配置

在 `configurations` 数组**之后**，添加 `compounds` 配置：

```json
{
    "version": "0.2.0",
    "configurations": [
        // ... 现有的配置保持不变 ...
    ],
    "compounds": [
        {
            "name": "🚀 启动所有集群节点 (Debug)",
            "configurations": [
                "gc-center",
                "gc-web-1",
                "gc-gate-1",
                "gc-game-10001"
            ],
            "stopAll": true,
            "presentation": {
                "hidden": false,
                "group": "cluster",
                "order": 1
            }
        },
        {
            "name": "🎮 启动完整集群 (含Master)",
            "configurations": [
                "gc-master",
                "gc-center",
                "gc-web-1",
                "gc-gate-1",
                "gc-game-10001"
            ],
            "stopAll": true,
            "presentation": {
                "hidden": false,
                "group": "cluster",
                "order": 2
            }
        },
        {
            "name": "🌐 启动双网关集群",
            "configurations": [
                "gc-center",
                "gc-web-1",
                "gc-gate-1",
                "gc-gate-2",
                "gc-game-10001"
            ],
            "stopAll": true,
            "presentation": {
                "hidden": false,
                "group": "cluster",
                "order": 3
            }
        }
    ]
}
```

### 完整的 launch.json 示例

```json
{
    "version": "0.2.0",
    "configurations": [
        {
            "name":"demo-chat",
            "type":"go",
            "request":"launch",
            "mode":"debug",
            "program":"${workspaceFolder}/demo_chat/room",
            "console" :"integratedTerminal"
        },
        {
            "name":"---------------",
            "type":"go",
            "request":"launch"
        },
        {
            "name": "gc-master",
            "type": "go",
            "request": "launch",
            "mode": "debug",
            "program": "${workspaceFolder}/demo_cluster/nodes/main.go",
            "console" :"integratedTerminal",
            "args": [
                "master",
                "--path=../../config/demo-cluster.json",
                "--node=gc-master"
            ]
        },
        {
            "name": "gc-center",
            "type": "go",
            "request": "launch",
            "mode": "debug",
            "program": "${workspaceFolder}/demo_cluster/nodes/main.go",
            "console" :"integratedTerminal",
            "args": [
                "center",
                "--path=../../config/demo-cluster.json",
                "--node=gc-center"
            ]
        },
        {
            "name": "gc-web-1",
            "type": "go",
            "request": "launch",
            "mode": "debug",
            "program": "${workspaceFolder}/demo_cluster/nodes/main.go",
            "console" :"integratedTerminal",
            "args": [
                "web",
                "--path=../../config/demo-cluster.json",
                "--node=gc-web-1"
            ]
        },
        {
            "name": "gc-gate-1",
            "type": "go",
            "request": "launch",
            "mode": "debug",
            "program": "${workspaceFolder}/demo_cluster/nodes/main.go",
            "console" :"integratedTerminal",
            "args": [
                "gate",
                "--path=../../config/demo-cluster.json",
                "--node=gc-gate-1"
            ]
        },
        {
            "name": "gc-gate-2",
            "type": "go",
            "request": "launch",
            "mode": "debug",
            "program": "${workspaceFolder}/demo_cluster/nodes/main.go",
            "console" :"integratedTerminal",
            "args": [
                "gate",
                "--path=../../config/demo-cluster.json",
                "--node=gc-gate-2"
            ]
        },
        {
            "name": "gc-game-10001",
            "type": "go",
            "request": "launch",
            "mode": "debug",
            "program": "${workspaceFolder}/demo_cluster/nodes/main.go",
            "console" :"integratedTerminal",
            "args": [
                "game",
                "--path=../../config/demo-cluster.json",
                "--node=10001"
            ]
        },
        {
            "name": "robot_client",
            "type": "go",
            "request": "launch",
            "mode": "debug",
            "program": "${workspaceFolder}/demo_cluster/robot_client/main.go",
            "console" :"integratedTerminal",
            "args": []
        },
        {
            "name": "actor_demo",
            "type": "go",
            "request": "launch",
            "mode": "debug",
            "program": "${workspaceFolder}/test_nats/replystrem/main.go",
            "console" :"integratedTerminal",
            "args": []
        }
    ],
    "compounds": [
        {
            "name": "🚀 启动所有集群节点 (Debug)",
            "configurations": [
                "gc-center",
                "gc-web-1",
                "gc-gate-1",
                "gc-game-10001"
            ],
            "stopAll": true,
            "presentation": {
                "hidden": false,
                "group": "cluster",
                "order": 1
            }
        },
        {
            "name": "🎮 启动完整集群 (含Master)",
            "configurations": [
                "gc-master",
                "gc-center",
                "gc-web-1",
                "gc-gate-1",
                "gc-game-10001"
            ],
            "stopAll": true,
            "presentation": {
                "hidden": false,
                "group": "cluster",
                "order": 2
            }
        },
        {
            "name": "🌐 启动双网关集群",
            "configurations": [
                "gc-center",
                "gc-web-1",
                "gc-gate-1",
                "gc-gate-2",
                "gc-game-10001"
            ],
            "stopAll": true,
            "presentation": {
                "hidden": false,
                "group": "cluster",
                "order": 3
            }
        }
    ]
}
```

## 使用方法

### 1. 在 VSCode 中启动调试

1. 按 `F5` 或点击左侧调试图标
2. 在调试下拉菜单中选择：
   - **🚀 启动所有集群节点 (Debug)** - 启动 4 个核心节点
   - **🎮 启动完整集群 (含Master)** - 启动 5 个节点（含 Master）
   - **🌐 启动双网关集群** - 启动 5 个节点（含双网关）

3. 点击绿色播放按钮或按 `F5`

### 2. 调试功能

启动后，你可以：

- ✅ **设置断点**：在任何节点的代码中设置断点
- ✅ **查看变量**：查看所有节点的变量值
- ✅ **单步调试**：逐行执行代码
- ✅ **查看调用栈**：查看函数调用链
- ✅ **多终端**：每个节点有独立的终端窗口
- ✅ **一键停止**：点击停止按钮，所有节点同时停止

### 3. 调试面板

VSCode 会显示：

```
调试控制台
├── gc-center (调试中)
├── gc-web-1 (调试中)
├── gc-gate-1 (调试中)
└── gc-game-10001 (调试中)
```

每个节点都有独立的：
- 调试控制台
- 终端输出
- 变量查看器
- 调用栈

## 配置说明

### compounds 配置项

```json
{
    "name": "🚀 启动所有集群节点 (Debug)",  // 显示名称
    "configurations": [                      // 要启动的配置列表
        "gc-center",
        "gc-web-1",
        "gc-gate-1",
        "gc-game-10001"
    ],
    "stopAll": true,                        // 停止时关闭所有节点
    "presentation": {
        "hidden": false,                    // 在下拉菜单中显示
        "group": "cluster",                 // 分组名称
        "order": 1                          // 排序顺序
    }
}
```

### 启动顺序

VSCode 会**并行启动**所有节点，但由于节点之间有依赖关系，建议：

1. 如果遇到启动问题，可以先单独启动 `gc-center`
2. 等待 2-3 秒后，再启动其他节点
3. 或者使用脚本 `./start_all.sh` 按顺序启动

## 优势对比

### 使用 Compound Debug 配置

✅ **优点**：
- 可以设置断点调试
- 可以查看变量和调用栈
- 可以单步执行
- 集成在 VSCode 中
- 一键启动和停止

❌ **缺点**：
- 并行启动，可能有依赖问题
- 占用更多资源（调试模式）
- 需要手动配置

### 使用 Shell 脚本

✅ **优点**：
- 按顺序启动，避免依赖问题
- 后台运行，不占用 VSCode
- 日志保存到文件
- 启动更快（非调试模式）

❌ **缺点**：
- 无法设置断点
- 无法单步调试
- 需要查看日志文件

## 建议

- **开发调试**：使用 Compound Debug 配置
- **功能测试**：使用 Shell 脚本 `./start_all.sh`
- **生产环境**：使用独立的部署脚本

## 故障排查

### 问题 1: 节点启动失败

**原因**：依赖节点未就绪

**解决**：
1. 先单独启动 `gc-center`
2. 等待 2-3 秒
3. 再启动其他节点

### 问题 2: 端口被占用

**解决**：
```bash
# 停止所有节点
./stop_all.sh

# 或手动杀死进程
lsof -i :3250 | grep LISTEN | awk '{print $2}' | xargs kill -9
```

### 问题 3: 调试器连接失败

**解决**：
1. 重启 VSCode
2. 清理 Go 缓存：`go clean -cache`
3. 重新安装 Go 扩展

## 相关文档

- [VSCode 调试文档](https://code.visualstudio.com/docs/editor/debugging)
- [Go 调试指南](https://github.com/golang/vscode-go/wiki/debugging)
- [Compound Launch 配置](https://code.visualstudio.com/docs/editor/debugging#_compound-launch-configurations)
