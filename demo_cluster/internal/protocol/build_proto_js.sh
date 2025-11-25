#!/bin/bash
###
 # @Author: t 921865806@qq.com
 # @Date: 2025-11-24 22:24:11
 # @LastEditors: t 921865806@qq.com
 # @LastEditTime: 2025-11-24 23:59:35
 # @FilePath: /examples/demo_cluster/internal/protocol/build_proto_js.sh
 # @Description: 这是默认设置,请设置`customMade`, 打开koroFileHeader查看配置 进行设置: https://github.com/OBKoro1/koro1FileHeader/wiki/%E9%85%8D%E7%BD%AE
### 

# 生成 JavaScript Protocol Buffer 文件
# 需要安装: npm install -g protobufjs

set -e

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
PROTO_DIR="$SCRIPT_DIR"
WEB_OUTPUT_DIR="$SCRIPT_DIR/../../nodes/web/static"

echo "Protocol directory: $PROTO_DIR"
echo "Web output directory: $WEB_OUTPUT_DIR"

# 创建输出目录
mkdir -p "$WEB_OUTPUT_DIR"

# 使用 pbjs 生成 JavaScript 静态模块（浏览器全局变量格式）
echo "Generating JavaScript protocol buffers..."

pbjs -t static-module -w es6 -o "$WEB_OUTPUT_DIR/pb_slots_temp.js" \
  "$PROTO_DIR/slots.proto" \
  "$PROTO_DIR/slots_define.proto" \
  "$PROTO_DIR/slots_feature.proto" \
  "$PROTO_DIR/common_define.proto"

# 转换为浏览器兼容格式
echo "Converting to browser-compatible format..."
cat > "$WEB_OUTPUT_DIR/pb_slots.js" << 'EOF'
// Browser-compatible Protocol Buffer definitions
(function(global) {
  'use strict';
  
  // 模拟 require 函数
  function require(name) {
    if (name === 'google-protobuf') {
      return jspb;
    }
    return {};
  }
  
  // 模拟 exports 对象
  var exports = {};
  
EOF

# 添加生成的代码（去掉 ES6 import/export）
sed 's/import.*from.*;//g; s/export.*{.*}.*;//g' "$WEB_OUTPUT_DIR/pb_slots_temp.js" >> "$WEB_OUTPUT_DIR/pb_slots.js"

cat >> "$WEB_OUTPUT_DIR/pb_slots.js" << 'EOF'

  // 将 proto 对象暴露到全局
  global.proto = exports.proto || {};
  
})(this);
EOF

# 清理临时文件
rm -f "$WEB_OUTPUT_DIR/pb_slots_temp.js"

echo "JavaScript protocol buffers generated successfully!"
echo "File: $WEB_OUTPUT_DIR/pb_slots.js"
