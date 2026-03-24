# 消息日志文档索引

## 📋 文档列表

### 1. 最终方案（必读）
**文件**: `消息日志最终方案.md`  
**内容**: 已采用的底层框架方案的完整说明  
**状态**: ✅ 已实施

### 2. 底层框架方案详解
**文件**: `底层框架消息日志方案.md`  
**内容**: 底层框架方案的详细实现和使用说明  
**状态**: ✅ 已实施

### 3. 方案对比分析
**文件**: `方案对比分析.md`  
**内容**: 中间件方案 vs 底层框架方案的详细对比  
**推荐**: 阅读此文档了解为什么选择底层方案

### 4. 中间件方案（已废弃）
**文件**: `底层统一消息打印方案.md`  
**内容**: 中间件方案的实现（已废弃，仅供参考）  
**状态**: ❌ 已废弃

### 5. 简要方案
**文件**: `消息打印方案.md`  
**内容**: 早期的简要方案说明  
**状态**: 📚 参考文档

## 🎯 快速导航

### 我想了解...

#### 当前使用的方案
👉 阅读 `消息日志最终方案.md`

#### 如何使用消息日志
👉 阅读 `底层框架消息日志方案.md`

#### 为什么选择底层方案
👉 阅读 `方案对比分析.md`

#### 日志输出格式
👉 阅读 `底层框架消息日志方案.md` 的"日志格式"章节

#### 性能影响
👉 阅读 `底层框架消息日志方案.md` 的"性能优化"章节

#### 如何配置日志级别
👉 阅读 `消息日志最终方案.md` 的"配置示例"章节

## 📊 实施状态

### ✅ 已完成
- [x] 修改 `vendor/github.com/cherry-game/cherry/net/parser/pomelo/agent.go`
- [x] 恢复 `demo_cluster/nodes/gate/actor_agent.go` 为原始代码
- [x] 编译验证通过
- [x] 文档完善

### 📝 修改的文件

#### 框架层（底层）
- `vendor/github.com/cherry-game/cherry/net/parser/pomelo/agent.go`
  - `Response()` 方法：添加日志打印
  - `Push()` 方法：添加日志打印
  - `Kick()` 方法：添加日志打印

#### 业务层（上层）
- `demo_cluster/nodes/gate/actor_agent.go`
  - 恢复为原始代码，无需使用 middleware

## 🔍 日志格式速查

### Info 级别（生产环境）
```
[GATE-IN] route=gate.user.login, uid=0, sid=abc123, mid=1, size=256 bytes
[GATE-OUT] uid=10001, sid=abc123, mid=1
[GATE-PUSH] uid=10001, sid=abc123, route=game.player.levelUp
[GATE-KICK] uid=10001, sid=abc123, reason={"code":1001}, closed=true
```

### Debug 级别（开发环境）
```
[GATE-IN] route=gate.user.login, uid=0, sid=abc123, mid=1, size=256 bytes
[GATE-OUT] uid=10001, sid=abc123, mid=1
[GATE-OUT-DETAIL] uid=10001, sid=abc123, mid=1, resp={"userId":10001,"pid":2126001}
[GATE-PUSH] uid=10001, sid=abc123, route=game.player.levelUp
[GATE-PUSH-DETAIL] uid=10001, sid=abc123, route=game.player.levelUp, data={"newLevel":2}
```

## ⚙️ 配置速查

### 生产环境配置
```json
{
  "logger": {
    "gate_log": {
      "level": "info",
      "enable_console": true,
      "enable_write_file": true
    }
  }
}
```

### 开发环境配置
```json
{
  "logger": {
    "gate_log": {
      "level": "debug",
      "enable_console": true,
      "enable_write_file": true
    }
  }
}
```

## 💡 使用示例

### 业务代码（完全不需要修改）
```go
func (p *ActorAgent) login(session *cproto.Session, req *pb.LoginRequest) {
    // ... 业务逻辑 ...
    
    response := &pb.LoginResponse{
        UserId: userId,
        Pid:    userToken.PID,
        OpenId: userToken.OpenID,
    }
    
    // 直接调用，底层自动打印日志
    agent.Response(session, response)
}
```

**所有 `agent.Response()`、`agent.Push()`、`agent.Kick()` 调用都会自动打印日志！**

## 🎓 学习路径

### 新手入门
1. 阅读 `消息日志最终方案.md` 了解整体方案
2. 查看日志格式速查，了解输出格式
3. 启动服务测试，观察日志输出

### 深入理解
1. 阅读 `底层框架消息日志方案.md` 了解实现细节
2. 阅读 `方案对比分析.md` 了解方案选择原因
3. 查看 `agent.go` 源码，理解底层实现

### 维护和扩展
1. 了解如何管理 vendor 代码修改
2. 了解框架升级策略
3. 了解性能优化原理

## 📞 常见问题

### Q: 为什么修改 vendor 代码？
A: Cherry 是自己的框架，可以自由定制。这是实现"底层统一"的最优方案。

### Q: 框架升级怎么办？
A: 在修改处添加 `// [CUSTOM]` 注释，文档记录修改点，升级时重新应用。

### Q: 性能影响大吗？
A: 极小。Info 级别不序列化，Debug 级别才序列化。生产环境使用 Info 级别。

### Q: 需要修改业务代码吗？
A: 完全不需要！这就是底层方案的优势。

### Q: 如何关闭日志？
A: 将日志级别设置为 `warn` 或更高级别。

## 🔗 相关资源

- Cherry 框架文档: [待补充]
- 日志配置说明: `config/demo-cluster.json`
- Agent 源码: `vendor/github.com/cherry-game/cherry/net/parser/pomelo/agent.go`

## 📅 更新历史

- 2024/03/24: 采用底层框架方案，废弃中间件方案
- 2024/03/24: 创建完整的文档体系
- 2024/03/24: 编译验证通过

---

**推荐阅读顺序**: 
1. `消息日志最终方案.md` 
2. `方案对比分析.md` 
3. `底层框架消息日志方案.md`
