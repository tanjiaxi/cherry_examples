#!/bin/bash
# 已移到 internal/protocol/build_proto.sh
# 在 demo_cluster 或任意目录执行均可。
exec "$(cd "$(dirname "$0")" && pwd)/internal/protocol/build_proto.sh" --js "$@"
