# Protobuf 浏览器兼容性修复方案

## 问题描述

`pb.js` 文件使用 CommonJS 模块格式（`require` 和 `exports`），在浏览器环境中无法直接使用，导致错误：
```
Uncaught TypeError: proto.pb.EnterMachine is not a constructor
```

## 解决方案

我们创建了三个辅助文件来解决这个问题：

### 1. pb-wrapper.js
在 `pb.js` **之前**加载，创建模拟的 CommonJS 环境。

### 2. pb-extract.js  
在 `pb.js` **之后**加载，从 `module.exports` 中提取 `proto` 对象到全局作用域。

### 3. test-proto.html
测试页面，用于验证 protobuf 是否正确加载。

## 使用方法

### 步骤 1: 生成 pb.js 文件

```bash
cd demo_cluster
./build_js_protocol.sh
```

或使用完整的设置脚本：
```bash
./setup_and_build_js_protocol.sh
```

### 步骤 2: 在 HTML 中按正确顺序加载脚本

```html
<!-- 1. Google Protobuf 运行时库 -->
<script src="https://cdn.jsdelivr.net/npm/google-protobuf@3.21.2/google-protobuf.js"></script>

<!-- 2. pb.js 包装器（在 pb.js 之前）-->
<script src="static/pb-wrapper.js"></script>

<!-- 3. 生成的 protobuf 文件 -->
<script src="static/pb.js"></script>

<!-- 4. 提取 proto 对象（在 pb.js 之后）-->
<script src="static/pb-extract.js"></script>
```

### 步骤 3: 测试 protobuf 加载

访问测试页面：
```
http://localhost:8080/static/test-proto.html
```

这个页面会显示：
- ✅ 所有依赖是否正确加载
- ✅ proto 对象是否可用
- ✅ 各个消息类型是否存在
- ✅ 序列化/反序列化是否正常工作

### 步骤 4: 在代码中使用

```javascript
// 创建消息
var enterRequest = new proto.pb.EnterMachine();
enterRequest.setId(123);
enterRequest.setSelectbet(100);

// 序列化
var bytes = enterRequest.serializeBinary();

// 发送到服务器
pomelo.request("game.slots.entermachine", bytes, function(data) {
    // 反序列化响应
    var response = proto.pb.EnterMachineResponse.deserializeBinary(data.body);
    var obj = response.toObject();
    console.log(obj);
});
```

## 故障排查

### 问题 1: proto is not defined

**原因**: 脚本加载顺序错误或 pb-extract.js 未加载

**解决**: 
1. 检查浏览器控制台，确认所有脚本都成功加载
2. 确保 pb-extract.js 在 pb.js 之后加载
3. 访问 test-proto.html 查看详细的加载状态

### 问题 2: proto.pb.EnterMachine is not a constructor

**原因**: proto 对象存在但消息类型未正确提取

**解决**:
1. 打开浏览器控制台，输入 `console.log(proto.pb)` 查看可用的消息类型
2. 检查 pb.js 文件是否包含 EnterMachine 定义
3. 重新生成 pb.js: `./build_js_protocol.sh`

### 问题 3: jspb is not defined

**原因**: google-protobuf 库未加载

**解决**:
1. 确保 google-protobuf CDN 链接可访问
2. 或下载到本地: `npm install google-protobuf` 然后复制到 static 目录

## 替代方案：使用 JSON 格式

如果 protobuf 问题持续存在，建议使用更简单的 JSON 格式方案：

### 优点
- ✅ 无需额外的库和工具
- ✅ 易于调试（数据可读）
- ✅ 浏览器原生支持
- ✅ 开发速度快

### 缺点
- ❌ 数据包稍大
- ❌ 无类型检查
- ❌ 无向后兼容性保证

JSON 方案的代码已经在服务器端和客户端都实现了，可以直接使用。

## 文件清单

```
demo_cluster/
├── build_js_protocol.sh              # macOS/Linux 生成脚本
├── setup_and_build_js_protocol.sh    # 完整设置脚本（含依赖检查）
├── nodes/web/static/
│   ├── pb-wrapper.js                 # CommonJS 环境模拟器
│   ├── pb-extract.js                 # proto 对象提取器
│   ├── test-proto.html               # 测试页面
│   └── pb.js                         # 生成的 protobuf 文件
└── nodes/web/view/
    └── index.html                    # 主页面（已更新脚本加载顺序）
```

## 推荐做法

1. **开发阶段**: 使用 JSON 格式，快速迭代
2. **生产环境**: 如果需要优化性能，再切换到 protobuf
3. **测试**: 始终先访问 test-proto.html 确认 protobuf 正常工作

## 技术细节

### CommonJS vs 浏览器全局变量

**CommonJS (Node.js)**:
```javascript
var proto = require('./pb.js');
module.exports = proto;
```

**浏览器全局变量**:
```javascript
window.proto = { pb: { ... } };
```

我们的包装器通过模拟 `module.exports` 和 `require`，将 CommonJS 格式的代码转换为浏览器可用的全局变量。

### 为什么需要 google-protobuf 库

生成的 pb.js 文件依赖 `jspb` 对象提供的基础功能：
- `jspb.Message` - 消息基类
- `jspb.BinaryReader` - 二进制读取器
- `jspb.BinaryWriter` - 二进制写入器

这些功能由 google-protobuf 库提供。