# Protobuf 浏览器加载调试指南

## 当前状态

✅ 已完成的工作：
1. 创建了 `pb-wrapper.js` - CommonJS 环境模拟器
2. 创建了 `pb-extract.js` - proto 对象提取器（增强版，带详细日志）
3. 更新了 `index.html` - 正确的脚本加载顺序
4. 服务器端代码已恢复为 protobuf 格式
5. 客户端代码已使用 protobuf 格式

## 调试步骤

### 步骤 1: 访问调试页面

打开浏览器，访问：
```
http://localhost:8080/static/debug-pb.html
```

这个页面会显示每一步的加载过程和详细信息。

### 步骤 2: 查看控制台输出

打开浏览器开发者工具（F12），查看控制台输出。你应该看到：

```
Step 1: 开始加载...
Step 2: google-protobuf 加载完成
  typeof jspb: object
  jspb.Message: function
  jspb.BinaryReader: function
  jspb.BinaryWriter: function
Step 3: pb-wrapper 加载完成
  typeof module: object
  typeof exports: object
  typeof require: function
Step 4: pb.js 加载完成
  typeof module.exports: object
  module.exports 是否为空对象: false
  module.exports keys: ErrorResponse, None, Bool, Int32, Int64
Step 5: pb-extract 加载完成
  typeof proto: object
  typeof proto.pb: object
  proto.pb keys 数量: 50+
  proto.pb keys (前10个): ErrorResponse, None, Bool, ...
Step 6: 测试创建消息
  ✅ 成功创建 EnterMachine 实例
  ✅ 序列化成功，字节数: 4
```

### 步骤 3: 诊断问题

#### 问题 A: jspb 未定义
**症状**: `typeof jspb: undefined`

**解决方案**:
1. 检查网络连接，确保 CDN 可访问
2. 或下载 google-protobuf 到本地：
   ```bash
   npm install google-protobuf
   cp node_modules/google-protobuf/google-protobuf.js demo_cluster/nodes/web/static/
   ```
3. 修改 HTML，使用本地文件：
   ```html
   <script src="static/google-protobuf.js"></script>
   ```

#### 问题 B: module.exports 是空对象
**症状**: `module.exports 是否为空对象: true`

**原因**: pb.js 文件没有正确执行或生成有问题

**解决方案**:
1. 重新生成 pb.js：
   ```bash
   cd demo_cluster
   ./build_js_protocol.sh
   ```
2. 检查 pb.js 文件大小，应该有几万行
3. 检查 pb.js 文件末尾是否有 `goog.object.extend(exports, proto.pb);`

#### 问题 C: proto.pb 是空对象
**症状**: `proto.pb keys 数量: 0`

**原因**: pb-extract.js 没有正确提取对象

**解决方案**:
1. 在浏览器控制台手动检查：
   ```javascript
   console.log(module.exports);
   console.log(Object.keys(module.exports));
   ```
2. 如果 module.exports 有内容但 proto.pb 是空的，手动设置：
   ```javascript
   window.proto = { pb: module.exports };
   ```

#### 问题 D: proto.pb.EnterMachine 不存在
**症状**: 其他消息存在，但 EnterMachine 不存在

**原因**: slots.proto 没有被编译到 pb.js

**解决方案**:
1. 检查 proto 文件是否存在：
   ```bash
   ls -la demo_cluster/internal/protocol/slots.proto
   ```
2. 重新生成，确保包含所有 proto 文件：
   ```bash
   cd demo_cluster
   ./build_js_protocol.sh
   ```
3. 检查生成的 pb.js 中是否包含 EnterMachine：
   ```bash
   grep -n "EnterMachine" demo_cluster/nodes/web/static/pb.js
   ```

### 步骤 4: 测试完整页面

如果 debug-pb.html 显示一切正常，访问主页面：
```
http://localhost:8080
```

测试流程：
1. 登录
2. 选择玩家
3. 输入机器 ID 和下注金额
4. 点击"进入机器"
5. 点击"Spin"

### 步骤 5: 查看网络请求

在浏览器开发者工具的 Network 标签中：
1. 找到 WebSocket 连接
2. 查看发送的消息（应该是二进制数据）
3. 查看接收的消息

## 常见错误和解决方案

### 错误 1: proto is not defined
```
Uncaught ReferenceError: proto is not defined
```

**检查清单**:
- [ ] pb-extract.js 是否在 pb.js 之后加载
- [ ] 浏览器控制台是否有其他错误
- [ ] 访问 debug-pb.html 查看详细信息

### 错误 2: proto.pb.EnterMachine is not a constructor
```
Uncaught TypeError: proto.pb.EnterMachine is not a constructor
```

**检查清单**:
- [ ] 在控制台输入 `typeof proto.pb.EnterMachine`，应该是 `function`
- [ ] 在控制台输入 `console.log(proto.pb.EnterMachine)`，查看是否是构造函数
- [ ] 检查 pb.js 是否包含 EnterMachine 定义

### 错误 3: jspb is not defined
```
Uncaught ReferenceError: jspb is not defined
```

**解决方案**: google-protobuf 库未加载，参见"问题 A"

## 手动测试命令

在浏览器控制台中运行以下命令进行手动测试：

```javascript
// 1. 检查依赖
console.log('jspb:', typeof jspb);
console.log('module:', typeof module);
console.log('proto:', typeof proto);

// 2. 检查 proto.pb 内容
console.log('proto.pb keys:', Object.keys(proto.pb));

// 3. 测试创建消息
var msg = new proto.pb.EnterMachine();
msg.setId(123);
msg.setSelectbet(100);
console.log('Message created:', msg);

// 4. 测试序列化
var bytes = msg.serializeBinary();
console.log('Serialized bytes:', bytes);
console.log('Bytes length:', bytes.length);

// 5. 测试反序列化
var decoded = proto.pb.EnterMachine.deserializeBinary(bytes);
console.log('Decoded:', decoded.toObject());
```

## 文件清单

确保以下文件存在且正确：

```
demo_cluster/nodes/web/static/
├── pb.js                    # 生成的 protobuf 文件（应该很大，几万行）
├── pb-wrapper.js            # CommonJS 环境模拟器
├── pb-extract.js            # proto 对象提取器
├── debug-pb.html            # 调试页面
└── test-proto.html          # 完整测试页面
```

## 下一步

如果所有调试步骤都通过了，但主页面仍然有问题：

1. 清除浏览器缓存（Ctrl+Shift+Delete）
2. 硬刷新页面（Ctrl+Shift+R 或 Cmd+Shift+R）
3. 检查服务器端日志，确认收到了 protobuf 格式的数据
4. 使用浏览器的 Network 标签查看 WebSocket 消息

## 获取帮助

如果问题仍然存在，请提供以下信息：

1. debug-pb.html 页面的完整输出（截图）
2. 浏览器控制台的错误信息
3. `grep -c "EnterMachine" demo_cluster/nodes/web/static/pb.js` 的输出
4. pb.js 文件的大小：`wc -l demo_cluster/nodes/web/static/pb.js`