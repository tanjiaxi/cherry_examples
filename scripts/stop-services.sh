#!/bin/bash
# Cherry 游戏服务器 - 停止基础服务

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
PROJECT_ROOT="$(dirname "$SCRIPT_DIR")"
DOCKER_DIR="$PROJECT_ROOT/docker"

echo "🛑 停止 ETCD 和 NATS 服务..."

cd "$DOCKER_DIR"
docker-compose down

echo "✅ 服务已停止"
