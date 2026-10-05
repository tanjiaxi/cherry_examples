#!/bin/bash

# 完整的 JavaScript Protocol Buffer 设置和生成脚本
# 包含依赖检查和安装指导

set -e

echo "🚀 JavaScript Protocol Buffer 生成工具 (macOS/Linux)"
echo "=================================================="

# 检查是否在正确的目录
if [ ! -f "nodes/main.go" ]; then
    echo "❌ 错误: 请在 demo_cluster 目录下运行此脚本"
    exit 1
fi

# 检查 protoc 是否安装
echo "🔍 检查依赖..."
if ! command -v protoc &> /dev/null; then
    echo "❌ 未找到 protoc 命令"
    echo ""
    echo "请安装 Protocol Buffers 编译器:"
    echo "  macOS: brew install protobuf"
    echo "  Ubuntu/Debian: sudo apt-get install protobuf-compiler"
    echo "  CentOS/RHEL: sudo yum install protobuf-compiler"
    echo ""
    exit 1
else
    protoc_version=$(protoc --version)
    echo "✅ protoc 已安装: $protoc_version"
fi

# 检查 Node.js 和 npm
if ! command -v node &> /dev/null; then
    echo "❌ 未找到 node 命令"
    echo ""
    echo "请安装 Node.js:"
    echo "  访问 https://nodejs.org/ 下载安装"
    echo "  或使用包管理器: brew install node"
    echo ""
    exit 1
else
    node_version=$(node --version)
    echo "✅ Node.js 已安装: $node_version"
fi

if ! command -v npm &> /dev/null; then
    echo "❌ 未找到 npm 命令"
    exit 1
else
    npm_version=$(npm --version)
    echo "✅ npm 已安装: $npm_version"
fi

# 检查 browserify
if ! command -v browserify &> /dev/null; then
    echo "⚠️  未找到 browserify，正在安装..."
    npm install -g browserify
    
    if [ $? -ne 0 ]; then
        echo "❌ browserify 安装失败"
        echo "请手动安装: npm install -g browserify"
        exit 1
    fi
    echo "✅ browserify 安装成功"
else
    browserify_version=$(browserify --version)
    echo "✅ browserify 已安装: $browserify_version"
fi

echo ""
echo "开始生成浏览器 pb.js..."
exec "$(cd "$(dirname "$0")" && pwd)/internal/protocol/build_proto.sh" --js