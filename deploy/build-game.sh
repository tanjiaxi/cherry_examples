#!/bin/bash
###
 # @Author: t 921865806@qq.com
 # @Date: 2026-01-03 21:43:05
 # @LastEditors: t 921865806@qq.com
 # @LastEditTime: 2026-09-18 09:53:00
 # @FilePath: /examples/deploy/build-game.sh
 # @Description: 本地交叉编译 Linux amd64 二进制（默认发布版，--debug 保留调试符号）
###
# Cherry 游戏服务器 - 构建脚本
# 在本机（macOS/Linux）交叉编译，产物拷到 Linux 运行。
#
#   ./build-game.sh          # 发布版（去掉符号，体积小）
#   ./build-game.sh --debug  # 调试版（给 Linux 上 dlv 用，Linux 不需要源码）

set -e

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
PROJECT_ROOT="$(dirname "$SCRIPT_DIR")"
OUTPUT="$PROJECT_ROOT/demo_cluster/nodes/io_sql"
DEBUG_BUILD=0

usage() {
    cat <<EOF
用法: $0 [--debug]

在本机交叉编译 linux/amd64，不需要在 Linux 上放源码。

  （默认）   发布版: -ldflags="-s -w"，不能用来 dlv 断点
  --debug    调试版: -gcflags="all=-N -l"，保留符号，配合 Linux 上的 debug-game.sh
EOF
}

while [[ $# -gt 0 ]]; do
    case "$1" in
        -h|--help)
            usage
            exit 0
            ;;
        --debug|-d)
            DEBUG_BUILD=1
            shift
            ;;
        *)
            echo "未知选项: $1"
            usage
            exit 1
            ;;
    esac
done

echo "=========================================="
if [[ "$DEBUG_BUILD" == "1" ]]; then
    echo "  Cherry 游戏服务器 - Debug 构建 (linux/amd64)"
else
    echo "  Cherry 游戏服务器 - 构建 (linux/amd64)"
fi
echo "=========================================="

cd "$PROJECT_ROOT"

echo "编译中...  GOOS=linux GOARCH=amd64"

if [[ "$DEBUG_BUILD" == "1" ]]; then
    # 关闭优化、保留 DWARF，禁止 -s -w，否则 Linux 上 dlv 无法下断点
    CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -gcflags="all=-N -l" -o "$OUTPUT" ./demo_cluster/nodes/main.go
else
    CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -ldflags="-s -w" -o "$OUTPUT" ./demo_cluster/nodes/main.go
fi

echo ""
echo "构建完成: $OUTPUT"
file "$OUTPUT" 2>/dev/null || true
echo ""
if [[ "$DEBUG_BUILD" == "1" ]]; then
    echo "下一步: 把 io_sql 拷到 Linux 后执行 ./debug-game.sh"
    echo "  Linux 上只需要: 二进制 + 配置 + dlv，不需要 Go 源码"
else
    echo "下一步: ./start-game.sh"
    echo "若要远程调试，请改用: ./build-game.sh --debug"
fi
