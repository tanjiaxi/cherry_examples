#!/bin/bash

# 快速测试 Protobuf 加载的脚本

echo "🧪 Protobuf 浏览器加载测试"
echo "================================"
echo ""

# 检查必要的文件是否存在
echo "📋 检查文件..."

files=(
    "nodes/web/static/pb.js"
    "nodes/web/static/pb-wrapper.js"
    "nodes/web/static/pb-extract.js"
    "nodes/web/static/test-proto.html"
)

all_exist=true
for file in "${files[@]}"; do
    if [ -f "$file" ]; then
        echo "  ✅ $file"
    else
        echo "  ❌ $file (缺失)"
        all_exist=false
    fi
done

echo ""

if [ "$all_exist" = false ]; then
    echo "⚠️  部分文件缺失"
    echo ""
    echo "请运行以下命令生成 pb.js:"
    echo "  ./build_js_protocol.sh"
    echo ""
    exit 1
fi

# 检查 pb.js 中是否包含关键消息类型
echo "🔍 检查 pb.js 内容..."

if grep -q "proto.pb.EnterMachine" nodes/web/static/pb.js; then
    echo "  ✅ EnterMachine 定义存在"
else
    echo "  ❌ EnterMachine 定义不存在"
fi

if grep -q "proto.pb.Spin" nodes/web/static/pb.js; then
    echo "  ✅ Spin 定义存在"
else
    echo "  ❌ Spin 定义不存在"
fi

echo ""

# 检查 HTML 文件中的脚本加载顺序
echo "📄 检查 HTML 脚本加载顺序..."

if grep -q "pb-wrapper.js" nodes/web/view/index.html; then
    echo "  ✅ pb-wrapper.js 已引用"
else
    echo "  ⚠️  pb-wrapper.js 未引用"
fi

if grep -q "pb-extract.js" nodes/web/view/index.html; then
    echo "  ✅ pb-extract.js 已引用"
else
    echo "  ⚠️  pb-extract.js 未引用"
fi

echo ""
echo "✅ 文件检查完成"
echo ""
echo "📝 下一步操作:"
echo "1. 启动 web 服务器"
echo "2. 访问测试页面: http://localhost:8080/static/test-proto.html"
echo "3. 检查浏览器控制台的输出"
echo "4. 如果测试通过，访问主页面: http://localhost:8080"
echo ""
echo "💡 提示:"
echo "- 如果测试失败，查看 PROTOBUF_BROWSER_FIX.md 文档"
echo "- 或者使用 JSON 格式方案（已实现，更简单）"