#!/bin/bash
# Cherry 游戏服务器 - 部署脚本

set -e

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"

echo "=========================================="
echo "  Cherry 游戏服务器 - Docker 部署"
echo "=========================================="

cd "$SCRIPT_DIR"

# 检查配置文件
if [ ! -f "config/config.json" ]; then
    echo "❌ 错误: config/config.json 不存在"
    exit 1
fi

# 复制数据配置文件
if [ ! -d "config/data" ]; then
    echo "📁 创建 config/data 目录..."
    mkdir -p config/data
fi

# 检查是否需要复制数据文件
if [ -d "../config/data" ] && [ -z "$(ls -A config/data 2>/dev/null)" ]; then
    echo "📋 复制数据配置文件..."
    cp -r ../config/data/* config/data/ 2>/dev/null || true
fi

# 创建日志目录
mkdir -p logs

case "${1:-up}" in
    up)
        echo "🚀 启动服务..."
        docker-compose up -d
        echo ""
        echo "✅ 服务已启动！"
        echo ""
        echo "查看状态: docker-compose ps"
        echo "查看日志: docker-compose logs -f"
        ;;
    down)
        echo "🛑 停止服务..."
        docker-compose down
        echo "✅ 服务已停止"
        ;;
    restart)
        echo "🔄 重启服务..."
        docker-compose restart
        echo "✅ 服务已重启"
        ;;
    logs)
        docker-compose logs -f ${2:-}
        ;;
    ps)
        docker-compose ps
        ;;
    build)
        echo "📦 构建镜像..."
        cd ..
        docker build -f deploy/Dockerfile -t cherry-game:latest .
        echo "✅ 镜像构建完成"
        ;;
    *)
        echo "用法: $0 {up|down|restart|logs|ps|build}"
        echo ""
        echo "  up      - 启动所有服务"
        echo "  down    - 停止所有服务"
        echo "  restart - 重启所有服务"
        echo "  logs    - 查看日志 (可选: logs center/gate/game/web)"
        echo "  ps      - 查看服务状态"
        echo "  build   - 构建 Docker 镜像"
        exit 1
        ;;
esac
