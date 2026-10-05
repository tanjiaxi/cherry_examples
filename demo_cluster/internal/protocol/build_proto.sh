#!/bin/bash
###
 # @Author: t 921865806@qq.com
 # @Date: 2026-01-03
 # @LastEditors: t 921865806@qq.com
 # @LastEditTime: 2026-09-20
 # @FilePath: /examples/demo_cluster/internal/protocol/build_proto.sh
 # @Description: 从 .proto 生成 Go 代码，以及浏览器可用的 pb.js（browserify 打包）
###
# 用法（可在任意目录执行）:
#   ./build_proto.sh           # 同时生成 Go + 网页 pb.js
#   ./build_proto.sh --go      # 只生成 internal/pb/*.go
#   ./build_proto.sh --js      # 只生成 nodes/web/static/pb.js
#
# 不要把 protoc 的 commonjs 产物 cat 成 pb.js，浏览器没有 require。

set -e

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
PROTO_DIR="$SCRIPT_DIR"
CLUSTER_DIR="$(cd "$SCRIPT_DIR/../.." && pwd)"
GO_OUT="$SCRIPT_DIR/../pb"
WEB_STATIC="$CLUSTER_DIR/nodes/web/static"
TMP_JS="$PROTO_DIR/.outjs"

DO_GO=1
DO_JS=1

usage() {
    cat <<EOF
用法: $0 [--go|--js]

  （默认）  生成 Go protobuf 和浏览器 pb.js
  --go      只生成 $GO_OUT/*.go
  --js      只生成 $WEB_STATIC/pb.js（browserify 浏览器包）
  -h        帮助

网页请用: <script src="static/pb.js"></script>
EOF
}

while [ $# -gt 0 ]; do
    case "$1" in
        -h|--help)
            usage
            exit 0
            ;;
        --go)
            DO_GO=1
            DO_JS=0
            shift
            ;;
        --js)
            DO_GO=0
            DO_JS=1
            shift
            ;;
        *)
            echo "未知选项: $1"
            usage
            exit 1
            ;;
    esac
done

if ! command -v protoc >/dev/null 2>&1; then
    echo "错误: 未找到 protoc"
    echo "  macOS: brew install protobuf"
    exit 1
fi

mkdir -p "$GO_OUT" "$WEB_STATIC"

build_go() {
    echo "生成 Go protobuf -> $GO_OUT"
    protoc \
        --proto_path="$PROTO_DIR" \
        --go_out="$GO_OUT" \
        --go_opt=paths=source_relative \
        "$PROTO_DIR"/*.proto
    echo "Go 完成"
}

build_js() {
    if ! command -v browserify >/dev/null 2>&1; then
        echo "错误: 未找到 browserify（把 CommonJS 打成浏览器包）"
        echo "  npm install -g browserify"
        exit 1
    fi
    if [ ! -d "$CLUSTER_DIR/node_modules/google-protobuf" ]; then
        echo "错误: 未找到 $CLUSTER_DIR/node_modules/google-protobuf"
        echo "  cd $CLUSTER_DIR && npm install google-protobuf"
        exit 1
    fi

    echo "生成 JS CommonJS 中间文件 -> $TMP_JS"
    rm -rf "$TMP_JS"
    mkdir -p "$TMP_JS"
    protoc \
        --proto_path="$PROTO_DIR" \
        --js_out=import_style=commonjs,binary:"$TMP_JS" \
        "$PROTO_DIR"/*.proto

    entry="$TMP_JS/browser_entry.js"
    : > "$entry"
    for f in "$TMP_JS"/*_pb.js; do
        [ -f "$f" ] || continue
        echo "require('./$(basename "$f")');" >> "$entry"
    done

    echo "browserify 打包浏览器 pb.js -> $WEB_STATIC/pb.js"
    (
        cd "$TMP_JS"
        NODE_PATH="$CLUSTER_DIR/node_modules${NODE_PATH:+:$NODE_PATH}" \
            browserify browser_entry.js --outfile "$WEB_STATIC/pb.js"
    )

    rm -rf "$TMP_JS"

    if ! grep -q 'function r(e,n,t)' "$WEB_STATIC/pb.js"; then
        echo "警告: pb.js 看起来不是 browserify 包，请勿用 cat *_pb.js 覆盖它"
    fi
    size=$(wc -c < "$WEB_STATIC/pb.js" | tr -d ' ')
    echo "JS 完成: $WEB_STATIC/pb.js ($size 字节)"
}

if [ "$DO_GO" -eq 1 ]; then
    build_go
fi
if [ "$DO_JS" -eq 1 ]; then
    build_js
fi

echo "完成"
