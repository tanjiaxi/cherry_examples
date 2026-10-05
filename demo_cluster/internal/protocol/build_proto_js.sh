#!/bin/bash
# 已废弃：pbjs + sed 无法生成页面使用的 jspb API。
# 请改用同目录 build_proto.sh --js（protoc + browserify）。
echo "提示: build_proto_js.sh 已改为调用 build_proto.sh --js"
exec "$(cd "$(dirname "$0")" && pwd)/build_proto.sh" --js "$@"
