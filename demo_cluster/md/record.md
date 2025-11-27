<!--
 * @Author: t 921865806@qq.com
 * @Date: 2025-09-17 11:23:06
 * @LastEditors: t 921865806@qq.com
 * @LastEditTime: 2025-11-26 10:36:11
 * @FilePath: /examples/demo_cluster/md/record.md
 * @Description: 这是默认设置,请设置`customMade`, 打开koroFileHeader查看配置 进行设置: https://github.com/OBKoro1/koro1FileHeader/wiki/%E9%85%8D%E7%BD%AE
-->
### question
 1. grpc和nats-server怎么使用的

    gRPC 是一种点对点的 远程过程调用（RPC）框架，专注于高效通信，需要知道服务器的端点地址。而NATS Server 是一种 息代理，支持各种发布/订阅（pub/sub）和队列模式，通常用于解耦服务，不直接进行RPC，但可以与gRPC 集成，或作为服 发现和通信的基础设施

 2. 登陆和注册是不是真的actor模型。

    Web节点，是传统的http服务模式，因为一般web服务通常是无状态的，不需要actor的状态管理
    每个HTTP请求都是独立的，不需要维护长期状态
    Web层：使用传统的HTTP服务器 + Gin框架 + Controller模式
    通信层：通过RPC调用Cherry的Actor系统
    业务层：在Center节点的Actor中处理实际的业务逻辑

 3. gateway层怎么连接多个game层。

    通过user绑定到某个game层，然后通过nats发布，订阅通信
    
 4. 同节点和跨节点通信，节点之间是怎么通信的，怎么把信息传递给actor的，功能之间都是通过actor吗？

    同节点：直接注册到把接口注入到actor 邮箱的funcMap，调用的时候直接通过system actor push到mailbox队列，然后按顺序通过反射机制调用
    不同节点：远程直接注册到把接口注入到actor 邮箱的funcMap，调用的时候直接消息中间件（nats），远程收消息 再通过system actor push到mailbox队列，然后按顺序通过反射机制调用

 5. 如果一个game层崩了，那绑定到这个game层的user怎么处理呢。

 6. 框架中的remote和local 区别

    remote： 是指不同节点的actor，直接调用 rpc
    // 内部RPC路由  
    Gate.AgentActor ──► NATS ──► Game.PlayerActor.Remote
                PublishRemote   PostRemote
    local： 是指客户端消息的路由，
    // 客户端消息路由
    客户端 ──► Gate.AgentActor ──► NATS ──► Game.PlayerActor.Local
       TCP连接           PublishLocal    PostLocal

 | 特性 | 本地路由 | 远程路由 | RPC调用 | 
 |------|----------|----------|---------|
 | 通信范围 | 同进程内 | 跨节点 | 跨节点 |
 | 通信方式 | 内存队列 | NATS消息队列 | NATS请求-响应 | 
 | 调用模式 | 异步消息 | 异步消息 | 同步调用 |
 | 返回机制 | 通过Agent响应 | 通过Agent响应 | 直接返回 |
 | 会话保持 | 保持Session | 保持Session | 不保持Session |
 | 适用场景 | 网关内部处理 | 客户端消息转发 | 服务间调用 |
 | 性能 | 最高 | 中等 | 较低 |
 | 复杂度 | 最低 | 中等 | 最高 |

 7 框架的设计约定
| 注册类型 | 第一个参数 | 第二个参数 | 灵活性 |
|---------|-----------|-----------|--------| 
| Local | 必须是 *cproto.Session | 业务数据（protobuf） | ❌ 固定 | 
| Remote | 业务数据（protobuf） | - | ✅ 灵活 |

为什么这样设计？
Local 调用：来自客户端的请求，总是需要 Session 来识别用户
Remote 调用：Actor 之间的 RPC 调用，不需要 Session，只需要业务数据

#### 补救
 1. gorm
 2. 基础项目写