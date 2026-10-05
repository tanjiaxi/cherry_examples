#!/bin/bash
# 已合并到 build_proto.sh。本脚本只生成浏览器 pb.js。
exec "$(cd "$(dirname "$0")" && pwd)/build_proto.sh" --js "$@"
