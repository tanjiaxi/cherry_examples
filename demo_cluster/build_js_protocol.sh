#!/bin/bash
###
 # @Author: t 921865806@qq.com
 # @Date: 2025-11-25 00:19:43
 # @LastEditors: t 921865806@qq.com
 # @LastEditTime: 2025-11-25 00:20:20
 # @FilePath: /examples/demo_cluster/build_js_protocol.sh
 # @Description: 这是默认设置,请设置`customMade`, 打开koroFileHeader查看配置 进行设置: https://github.com/OBKoro1/koro1FileHeader/wiki/%E9%85%8D%E7%BD%AE
### 

# macOS/Linux 版本的 JavaScript Protocol Buffer 生成脚本

set -e

echo "开始生成 JavaScript Protocol Buffer 文件..."

# 清理并创建输出目录
if [ -d "outjs" ]; then
    rm -rf outjs
fi

mkdir -p outjs

echo "使用 protoc 生成 JavaScript 文件..."

# 使用 protoc 生成 JavaScript 文件
protoc --proto_path=internal/protocol/ \
       --js_out=import_style=commonjs,binary:outjs/ \
       internal/protocol/*.proto

if [ $? -ne 0 ]; then
    echo "错误: protoc 生成失败"
    exit 1
fi

# 收集所有生成的 JavaScript 文件
outjs_dir="$(pwd)/outjs"
all_js_files=""

# 使用 find 命令收集所有 .js 文件
for js_file in $(find "$outjs_dir" -name "*.js" -type f); do
    all_js_files="$all_js_files $js_file"
done

echo "找到的 JavaScript 文件: $all_js_files"

# 检查是否安装了 browserify
if ! command -v browserify &> /dev/null; then
    echo "错误: 未找到 browserify 命令"
    echo "请安装 browserify: npm install -g browserify"
    exit 1
fi

echo "使用 browserify 打包文件..."

# 使用 browserify 打包所有文件
browserify $all_js_files --outfile nodes/web/static/pb.js

if [ $? -eq 0 ]; then
    echo "✅ JavaScript Protocol Buffer 文件生成成功!"
    echo "输出文件: nodes/web/static/pb.js"
    
    # 显示文件大小
    if [ -f "nodes/web/static/pb.js" ]; then
        file_size=$(wc -c < "nodes/web/static/pb.js")
        echo "文件大小: $file_size 字节"
    fi
else
    echo "❌ browserify 打包失败"
    exit 1
fi

# 清理临时文件
echo "清理临时文件..."
rm -rf outjs

echo "完成!"