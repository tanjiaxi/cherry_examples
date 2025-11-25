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
echo "🔧 开始生成 JavaScript Protocol Buffer 文件..."

# 清理并创建输出目录
if [ -d "outjs" ]; then
    rm -rf outjs
fi
mkdir -p outjs

echo "📝 使用 protoc 生成 JavaScript 文件..."

# 生成 JavaScript 文件
protoc --proto_path=internal/protocol/ \
       --js_out=import_style=commonjs,binary:outjs/ \
       internal/protocol/*.proto

if [ $? -ne 0 ]; then
    echo "❌ protoc 生成失败"
    exit 1
fi

# 检查生成的文件
js_count=$(find outjs -name "*.js" -type f | wc -l)
echo "✅ 生成了 $js_count 个 JavaScript 文件"

# 收集所有 JavaScript 文件
outjs_dir="$(pwd)/outjs"
all_js_files=""

for js_file in $(find "$outjs_dir" -name "*.js" -type f); do
    all_js_files="$all_js_files $js_file"
done

echo "📦 使用 browserify 打包文件..."

# 创建输出目录
mkdir -p nodes/web/static

# 打包文件
browserify $all_js_files --outfile nodes/web/static/pb.js

if [ $? -eq 0 ]; then
    echo "✅ JavaScript Protocol Buffer 文件生成成功!"
    echo "📄 输出文件: nodes/web/static/pb.js"
    
    # 显示文件信息
    if [ -f "nodes/web/static/pb.js" ]; then
        file_size=$(wc -c < "nodes/web/static/pb.js")
        lines=$(wc -l < "nodes/web/static/pb.js")
        echo "📊 文件大小: $file_size 字节 ($lines 行)"
    fi
else
    echo "❌ browserify 打包失败"
    exit 1
fi

# 清理临时文件
echo "🧹 清理临时文件..."
rm -rf outjs

echo ""
echo "🎉 完成! 现在可以在浏览器中使用 protobuf 协议了"
echo ""
echo "💡 使用提示:"
echo "1. 确保在 HTML 中引入: <script src=\"static/pb.js\"></script>"
echo "2. 使用方式: var msg = new proto.pb.YourMessage();"
echo ""
echo "🔧 如果遇到浏览器兼容性问题，可以:"
echo "1. 添加 protobuf 运行时库"
echo "2. 或者使用我们之前创建的 JSON 格式方案"