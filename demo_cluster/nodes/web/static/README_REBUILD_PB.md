# 重新生成浏览器可用的 pb.js

页面加载的是 `static/pb.js`，必须是 **browserify 打好的浏览器包**。

## 正确做法

在 `internal/protocol` 或任意目录：

```bash
./build_proto.sh           # Go 代码 + 网页 pb.js
./build_proto.sh --go      # 只生成 internal/pb/*.go
./build_proto.sh --js      # 只生成 nodes/web/static/pb.js
```

旧路径仍可用：`demo_cluster/build_js_protocol.sh` 会转到 `build_proto.sh --js`。

依赖：`protoc`、`browserify`（`npm install -g browserify`）、`demo_cluster/node_modules/google-protobuf`。

## 不要这样

```bash
protoc --js_out=import_style=commonjs,binary:... *.proto
cat *_pb.js > pb.js
```

那会得到带 `require` 的 Node 模块，浏览器报 `require is not defined`。

验证：`pb.js` 开头应是 `(function(){function r(e,n,t){`。

控制台：`typeof proto.pb.Spin.prototype.setRequestid === "function"`。
