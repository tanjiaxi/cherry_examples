#!/bin/bash

# 配置 Docker 镜像源（macOS）

echo "配置 Docker 镜像源..."

# 获取 Docker Desktop 配置文件路径
DOCKER_CONFIG="$HOME/.docker/daemon.json"

# 备份原配置
if [ -f "$DOCKER_CONFIG" ]; then
    cp "$DOCKER_CONFIG" "$DOCKER_CONFIG.bak"
    echo "已备份原配置到 $DOCKER_CONFIG.bak"
fi

# 创建新配置
cat > "$DOCKER_CONFIG" << 'EOF'
{
  "registry-mirrors": [
    "https://docker.mirrors.ustc.edu.cn",
    "https://registry.docker-cn.com",
    "https://mirror.ccs.tencentyun.com",
    "https://dockerhub.azk8s.cn"
  ],
  "insecure-registries": [],
  "debug": false,
  "experimental": false
}
EOF

echo "已更新 Docker 配置文件"
echo "请重启 Docker Desktop 以应用更改"
echo ""
echo "macOS 重启 Docker 步骤："
echo "1. 点击菜单栏 Docker 图标"
echo "2. 选择 'Quit Docker Desktop'"
echo "3. 重新打开 Docker Desktop"
echo ""
echo "或者使用命令："
echo "  osascript -e 'quit app \"Docker\"'"
echo "  sleep 5"
echo "  open -a Docker"
