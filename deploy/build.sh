#!/bin/bash
###
 # @Author: t 921865806@qq.com
 # @Date: 2026-01-03 21:39:39
 # @LastEditors: t 921865806@qq.com
 # @LastEditTime: 2026-01-03 21:42:59
 # @FilePath: /examples/deploy/build.sh
 # @Description: 这是默认设置,请设置`customMade`, 打开koroFileHeader查看配置 进行设置: https://github.com/OBKoro1/koro1FileHeader/wiki/%E9%85%8D%E7%BD%AE
### 
# Cherry 游戏服务器 - 构建脚本

set -e

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
PROJECT_ROOT="$(dirname "$SCRIPT_DIR")"

echo "=========================================="
echo "  Cherry 游戏服务器 - Docker 镜像构建"
echo "=========================================="

cd "$PROJECT_ROOT"

# 构建 Docker 镜像
echo "📦 构建 Docker 镜像..."
docker build -f deploy/Dockerfile -t cherry-game:latest .

echo ""
echo "✅ 构建完成！"
echo "   镜像名称: cherry-game:latest"
echo ""
echo "下一步："
echo "  1. 推送到镜像仓库: docker push your-registry/cherry-game:latest"
echo "  2. 或者导出镜像:   docker save cherry-game:latest | gzip > cherry-game.tar.gz"
