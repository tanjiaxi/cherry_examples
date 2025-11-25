<!--
 * @Author: t 921865806@qq.com
 * @Date: 2025-11-24 22:25:28
 * @LastEditors: t 921865806@qq.com
 * @LastEditTime: 2025-11-25 00:19:52
 * @FilePath: /examples/demo_cluster/nodes/web/static/README_REBUILD_PB.md
 * @Description: 这是默认设置,请设置`customMade`, 打开koroFileHeader查看配置 进行设置: https://github.com/OBKoro1/koro1FileHeader/wiki/%E9%85%8D%E7%BD%AE
-->
# 重新生成 pb.js 文件（包含 Slots 协议）

## 问题
当前的 `pb.js` 文件不包含 slots 相关的 protobuf 消息定义（EnterMachine, Spin 等），导致网页端无法调用 slots 接口。

## 解决方案

### 方法 1：使用 protoc 编译器（推荐）

```bash
cd demo_cluster/internal/protocol

# 编译生成 JavaScript protobuf 文件
protoc \
  --js_out=import_style=commonjs,binary:../nodes/web/static \
  --proto_path=. \
  *.proto

# 合并所有生成的 _pb.js 文件到 pb.js
cat ../nodes/web/static/*_pb.js > ../nodes/web/static/pb_new.js
mv ../nodes/web/static/pb_new.js ../nodes/web/static/pb.js
```

### 方法 2：使用 pbjs 工具（需要 protobufjs）

```bash
# 安装 protobufjs
npm install -g protobufjs

cd demo_cluster/internal/protocol

# 生成静态 JavaScript 模块
pbjs -t static-module -w commonjs \
  -o ../nodes/web/static/pb.js \
  login.proto \
  player.proto \
  rpc.proto \
  slots.proto \
  slots_define.proto \
  slots_feature.proto \
  common_define.proto \
  base_type.proto \
  base_error.proto
```

### 方法 3：使用提供的脚本

```bash
cd demo_cluster/internal/protocol
chmod +x build_proto_js.sh
./build_proto_js.sh
```

## 验证

重新生成后，在浏览器控制台中检查：

```javascript
console.log(proto.pb.EnterMachine);
console.log(proto.pb.Spin);
console.log(proto.pb.MachineInfo);
```

如果这些对象存在，说明生成成功。

## 临时解决方案

如果无法重新编译，可以修改 HTML 页面使用简化的消息格式（不使用 protobuf）。
