<!--
 * @Author: t 921865806@qq.com
 * @Date: 2025-12-25 10:41:20
 * @LastEditors: t 921865806@qq.com
 * @LastEditTime: 2025-12-25 10:51:41
 * @FilePath: /examples/demo_cluster/md/quest.md
 * @Description: 这是默认设置,请设置`customMade`, 打开koroFileHeader查看配置 进行设置: https://github.com/OBKoro1/koro1FileHeader/wiki/%E9%85%8D%E7%BD%AE
-->
## 问题分析
当前架构问题：
- center 节点使用单一 Actor 模型导致的并发瓶颈问题，为无状态的公共服务（如登录验证、账号查询）提供高并发处理力
- center 节点的 `ActorAccount` 是单一 Actor，所有 RPC 请求进入同一个 mailbox 队列
- 500 并发请求 × 30ms/请求 = 15 秒串行处理时间
- NATS cluster 的 `remoteProcess()` 将消息投递到 `ActorSystem().PostRemote()`，进入 Actor 队列
解决思路：
- 创建并发服务组件，直接订阅 NATS 主题
- 收到消息后直接在 goroutine 中处理，不经过 Actor mailbox
- 保持与现有 RPC 调用方式兼容
