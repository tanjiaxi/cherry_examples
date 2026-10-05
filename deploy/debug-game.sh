#!/bin/sh
###
 # @Author: t 921865806@qq.com
 # @Date: 2026-09-18 09:45:00
 # @LastEditors: t 921865806@qq.com
 # @LastEditTime: 2026-09-18 10:16:00
 # @FilePath: /examples/deploy/debug-game.sh
 # @Description: 把 dlv 挂到 start-game.sh 已启动的进程上（默认 attach，不重复拉起节点）
###
# Cherry 游戏服务器 - Linux Delve 远程调试脚本
#
# 推荐流程（全集群由 start-game.sh 启动，这里只挂调试器）：
#   ./start-game.sh
#   ./debug-game.sh                 # 挂到已有的 game/10001
#   ./debug-game.sh gate gc-gate-1  # 挂到已有的 gate
#
# 本机 VS Code: launch.json → "Connect to Remote Go App"（port 2345）

set -e

SCRIPT_DIR=$(cd "$(dirname "$0")" && pwd)
PROJECT_ROOT=$(dirname "$SCRIPT_DIR")
CONFIG_PATH="$PROJECT_ROOT/config/demo-cluster.json"
BINARY="$PROJECT_ROOT/demo_cluster/nodes/io_sql"
LOG_DIR="$PROJECT_ROOT/logs"

if [ -z "$LISTEN" ]; then
    LISTEN=":2345"
fi

# attach: 挂到 start-game.sh 已有进程（默认）
# exec:   由 dlv 单独拉起一个节点（会再起一份，容易和已有服务冲突）
MODE="attach"
CONTINUE_SET=0
CONTINUE_FLAG=1
NODE_TYPE="game"
NODE_ID="10001"
TARGET_PID=""

usage() {
    cat <<EOF
用法: $0 [选项] [节点类型] [节点ID]

默认：attach 到 start-game.sh 已经启动的进程，不会再起一份服务。
Linux 不需要源码；请先在本机 ./build-game.sh --debug 并拷贝 io_sql。

选项:
  --attach           挂到已有进程（默认）
  --exec             由 dlv 单独启动该节点（不要和 start-game.sh 的同一节点一起用）
  --pid PID          直接按 PID attach
  --continue, -c     attach 后进程继续跑（attach 默认开启，服务不会卡住）
  --stop             attach/exec 后先停住，等 VS Code 连上再跑
  --listen ADDR      调试监听地址，默认 :2345
  -h, --help         显示帮助

位置参数:
  节点类型           默认 game（center / web / gate / game / activity）
  节点ID             默认 10001

推荐:
  ./start-game.sh
  $0                      # 调试已有 game/10001
  $0 gate gc-gate-1       # 调试已有 gate
  $0 --pid 12345

VS Code: "Connect to Remote Go App"（host = Linux IP，port 2345）
Ctrl+C 只退出 dlv，attach 模式下一般不会杀掉游戏进程
EOF
}

list_io_sql() {
    echo "当前 io_sql 进程:"
    ps -eo pid,args | grep '[i]o_sql' | grep -v '[d]lv' || echo "  （无）"
}

find_node_pid() {
    _type=$1
    _id=$2
    _found=""
    for _pid in $(pgrep -f "io_sql ${_type} .*--node=${_id}" 2>/dev/null || true); do
        _comm=$(ps -p "$_pid" -o comm= 2>/dev/null || true)
        if [ "$_comm" = "io_sql" ]; then
            _found=$_pid
            break
        fi
    done
    echo "$_found"
}

POS_COUNT=0
while [ $# -gt 0 ]; do
    case "$1" in
        -h|--help)
            usage
            exit 0
            ;;
        --attach)
            MODE="attach"
            shift
            ;;
        --exec)
            MODE="exec"
            if [ "$CONTINUE_SET" -eq 0 ]; then
                CONTINUE_FLAG=0
            fi
            shift
            ;;
        --pid)
            if [ -z "$2" ]; then
                echo "错误: --pid 需要进程号"
                exit 1
            fi
            MODE="attach"
            TARGET_PID="$2"
            shift 2
            ;;
        --pid=*)
            MODE="attach"
            TARGET_PID="${1#--pid=}"
            shift
            ;;
        --continue|-c)
            CONTINUE_FLAG=1
            CONTINUE_SET=1
            shift
            ;;
        --stop)
            CONTINUE_FLAG=0
            CONTINUE_SET=1
            shift
            ;;
        --listen)
            if [ -z "$2" ]; then
                echo "错误: --listen 需要地址，例如 :2345"
                exit 1
            fi
            LISTEN="$2"
            shift 2
            ;;
        --listen=*)
            LISTEN="${1#--listen=}"
            shift
            ;;
        -*)
            echo "未知选项: $1"
            usage
            exit 1
            ;;
        *)
            POS_COUNT=$((POS_COUNT + 1))
            if [ "$POS_COUNT" -eq 1 ]; then
                NODE_TYPE="$1"
            elif [ "$POS_COUNT" -eq 2 ]; then
                NODE_ID="$1"
            elif [ "$POS_COUNT" -eq 3 ]; then
                LISTEN="$1"
            fi
            shift
            ;;
    esac
done

UNAME_S=$(uname -s)
if [ "$UNAME_S" != "Linux" ]; then
    echo "警告: 当前系统是 $UNAME_S，dlv 应在 Linux 上启动。"
fi

mkdir -p "$LOG_DIR"

if [ -n "$GOPATH" ]; then
    PATH="$GOPATH/bin:$PATH"
    export PATH
fi
if command -v go >/dev/null 2>&1; then
    PATH="$(go env GOPATH)/bin:$PATH"
    export PATH
fi

if ! command -v dlv >/dev/null 2>&1; then
    echo "错误: Linux 上未找到 dlv。"
    echo "有 Go 时: go install github.com/go-delve/delve/cmd/dlv@latest"
    echo "无 Go 时: 从 https://github.com/go-delve/delve/releases 下载 linux/amd64 的 dlv"
    exit 1
fi

if [ "$MODE" = "attach" ] && [ -z "$TARGET_PID" ]; then
    TARGET_PID=$(find_node_pid "$NODE_TYPE" "$NODE_ID")
    if [ -z "$TARGET_PID" ]; then
        echo "错误: 没有找到已运行的 $NODE_TYPE / $NODE_ID。"
        echo "请先启动全集群，再把调试器挂上去:"
        echo "  ./start-game.sh"
        echo "  $0 $NODE_TYPE $NODE_ID"
        echo ""
        list_io_sql
        exit 1
    fi
fi

if [ "$MODE" = "exec" ]; then
    if [ ! -f "$BINARY" ]; then
        echo "错误: 二进制不存在: $BINARY"
        echo "请先在本机编译并拷贝: ./build-game.sh --debug"
        exit 1
    fi
    chmod +x "$BINARY" 2>/dev/null || true
    EXISTING=$(find_node_pid "$NODE_TYPE" "$NODE_ID")
    if [ -n "$EXISTING" ]; then
        echo "错误: $NODE_TYPE / $NODE_ID 已经在跑 (PID $EXISTING)。"
        echo "不要再用 --exec 起第二份。请改用 attach:"
        echo "  $0 $NODE_TYPE $NODE_ID"
        exit 1
    fi
    if [ ! -f "$CONFIG_PATH" ]; then
        echo "错误: 配置文件不存在: $CONFIG_PATH"
        exit 1
    fi
fi

if [ "$MODE" = "attach" ] && [ -n "$TARGET_PID" ]; then
    _args=$(ps -p "$TARGET_PID" -o args= 2>/dev/null || true)
    _parsed_type=$(echo "$_args" | awk '{
        for (i = 1; i <= NF; i++) {
            if ($i ~ /\/io_sql$/ || $i == "io_sql") { print $(i + 1); exit }
        }
    }')
    _parsed_id=$(echo "$_args" | awk '{
        for (i = 1; i <= NF; i++) {
            if ($i ~ /^--node=/) { sub(/^--node=/, "", $i); print $i; exit }
        }
    }')
    if [ -n "$_parsed_type" ]; then
        NODE_TYPE=$_parsed_type
    fi
    if [ -n "$_parsed_id" ]; then
        NODE_ID=$_parsed_id
    fi
fi

LISTEN_PORT="${LISTEN##*:}"
if command -v fuser >/dev/null 2>&1 && [ -n "$LISTEN_PORT" ]; then
    fuser -k "${LISTEN_PORT}/tcp" 2>/dev/null || true
fi

echo "=========================================="
echo "  Cherry 游戏服务器 - dlv 远程调试"
echo "=========================================="
if [ "$MODE" = "attach" ]; then
    echo "方式:     attach 已有进程（不新启动服务）"
    echo "PID:      $TARGET_PID"
    ps -p "$TARGET_PID" -o args= 2>/dev/null | sed 's/^/命令:     /'
else
    echo "方式:     exec 由 dlv 单独拉起该节点"
    echo "二进制:   $BINARY"
    echo "配置:     $CONFIG_PATH"
fi
echo "节点:     $NODE_TYPE / $NODE_ID"
echo "监听:     $LISTEN"
echo "dlv:      $(command -v dlv)"
if [ "$CONTINUE_FLAG" -eq 1 ]; then
    echo "运行:     --continue（服务继续处理请求，VS Code 随时连）"
else
    echo "运行:     先停住，等 VS Code 连上再继续"
fi
echo ""
echo "VS Code 附加:  Connect to Remote Go App"
echo "  host = Linux IP，port = $LISTEN_PORT"
echo "=========================================="
echo ""

cd "$PROJECT_ROOT"

# --continue 必须跟在 attach/exec 子命令后面，不能放在子命令前，
# 否则旧版 cobra 会把下一个词当成 --continue 的参数，PID 就会被当成 dlv 子命令。
if [ "$MODE" = "attach" ]; then
    if [ "$CONTINUE_FLAG" -eq 1 ]; then
        exec dlv attach \
            --listen="$LISTEN" \
            --headless=true \
            --api-version=2 \
            --accept-multiclient \
            --log \
            --log-output=debugger,rpc \
            --continue \
            "$TARGET_PID"
    fi
    exec dlv attach \
        --listen="$LISTEN" \
        --headless=true \
        --api-version=2 \
        --accept-multiclient \
        --log \
        --log-output=debugger,rpc \
        "$TARGET_PID"
fi

if [ "$CONTINUE_FLAG" -eq 1 ]; then
    exec dlv exec \
        --listen="$LISTEN" \
        --headless=true \
        --api-version=2 \
        --accept-multiclient \
        --log \
        --log-output=debugger,rpc \
        --continue \
        "$BINARY" -- \
        "$NODE_TYPE" --path="$CONFIG_PATH" --node="$NODE_ID"
fi

exec dlv exec \
    --listen="$LISTEN" \
    --headless=true \
    --api-version=2 \
    --accept-multiclient \
    --log \
    --log-output=debugger,rpc \
    "$BINARY" -- \
    "$NODE_TYPE" --path="$CONFIG_PATH" --node="$NODE_ID"
