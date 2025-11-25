#!/bin/bash

# Protocol Buffer 编译脚本
# 用于编译 demo_cluster/internal/protocol 目录下的 .proto 文件

set -e

# 获取脚本所在目录
SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
PROTO_DIR="$SCRIPT_DIR"
OUTPUT_DIR="$SCRIPT_DIR/../pb"
WEB_OUTPUT_DIR="$SCRIPT_DIR/../nodes/web/static"

echo "Protocol directory: $PROTO_DIR"
echo "Go output directory: $OUTPUT_DIR"
echo "Web output directory: $WEB_OUTPUT_DIR"

# 创建输出目录
mkdir -p "$OUTPUT_DIR"
mkdir -p "$WEB_OUTPUT_DIR"

# 编译 Go 版本的 .proto 文件
echo "Compiling Go protocol buffers..."

protoc \
  --proto_path="$PROTO_DIR" \
  --go_out="$OUTPUT_DIR" \
  --go_opt=paths=source_relative \
  "$PROTO_DIR"/*.proto

echo "Go protocol buffers compiled successfully!"

# 编译 JavaScript 版本的 .proto 文件
echo "Compiling JavaScript protocol buffers..."

protoc \
  --proto_path="$PROTO_DIR" \
  --js_out=import_style=commonjs,binary:"$WEB_OUTPUT_DIR" \
  "$PROTO_DIR"/*.proto

echo "JavaScript protocol buffers compiled successfully!"
echo "All protocol buffers compiled successfully!"
