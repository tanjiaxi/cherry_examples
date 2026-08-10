# Classic Slots 后端项目 — 面试准备文档

> 仓库：`/Users/t/ServerProjects/bravo/classic02/classic`  
> 用途：面试自述 + 给其他模型的精读路径（已排除 `.gitignore`；`spinCommon` 仅浅读）  
> 生成日期：2026-08-09

---

## 0. 给其他模型的阅读指引（优先顺序）

### 0.1 必读（框架与主链路）

| 优先级 | 绝对路径 | 读什么 |
|--------|----------|--------|
| P0 | `/Users/t/ServerProjects/bravo/classic02/classic/index.js` | Lambda 入口：`handler` / `sqs` / `schedule` |
| P0 | `/Users/t/ServerProjects/bravo/classic02/classic/package.json` | 依赖：DynamoDB/SQS/Firehose/SES/JWT 等 |
| P0 | `/Users/t/ServerProjects/bravo/classic02/classic/src/common/RequestMapping.js` | cmd → Handler 路由表（约 500+ 指令） |
| P0 | `/Users/t/ServerProjects/bravo/classic02/classic/src/handler/BaseHandler.js` | 鉴权、设备、幂等缓存、取用户、响应封装 |
| P0 | `/Users/t/ServerProjects/bravo/classic02/classic/src/handler/GatewayHandler.js` | 网关：cmd → API 路径映射 |
| P0 | `/Users/t/ServerProjects/bravo/classic02/classic/src/handler/spin/SpinHandler.js` | Spin 热路径：锁 → 开奖 → 回包 |
| P0 | `/Users/t/ServerProjects/bravo/classic02/classic/src/spin/GetSpinNew.js` | Spin 编排：扣币、RTP、落库、投 SQS |
| P0 | `/Users/t/ServerProjects/bravo/classic02/classic/src/service/SqsService.js` | afterSpin / 支付验单 / 推送等异步消费 |
| P0 | `/Users/t/ServerProjects/bravo/classic02/classic/src/service/ScheduleService.js` | 定时：Redis→Dynamo 同步、推送、扩缩容等 |
| P0 | `/Users/t/ServerProjects/bravo/classic02/classic/src/domain/DynamoRedis.js` | Redis 写穿/写回 + 定时刷 Dynamo |
| P0 | `/Users/t/ServerProjects/bravo/classic02/classic/config/ConfigProd.js` | 生产配置形态（多区域可对照同目录其他 Config） |

### 0.2 次读（难点证据）

| 绝对路径 | 读什么 |
|----------|--------|
| `/Users/t/ServerProjects/bravo/classic02/classic/src/common/RedisLockUtils.js` | Redis 分布式锁（SET NX + Lua 解锁） |
| `/Users/t/ServerProjects/bravo/classic02/classic/src/service/MsgCacheService.js` | 请求幂等阶段机 |
| `/Users/t/ServerProjects/bravo/classic02/classic/src/dao/redis/MsgCacheDao.js` | 幂等缓存实现 |
| `/Users/t/ServerProjects/bravo/classic02/classic/src/domain/AutoLock.js` | 锁 + Dynamo 条件失败重试 |
| `/Users/t/ServerProjects/bravo/classic02/classic/src/domain/Mall.js` | 订单/支付门面 |
| `/Users/t/ServerProjects/bravo/classic02/classic/src/handler/order/` | GenerateOrder / FinishOrder / Repair 等 |
| `/Users/t/ServerProjects/bravo/classic02/classic/src/dao/postgresql/UserSpinRecordPartitionedDao.js` | Spin 记录分区表 |
| `/Users/t/ServerProjects/bravo/classic02/classic/src/domain/N2CMemory.js` | 配置热更新（Redis 版本号） |
| `/Users/t/ServerProjects/bravo/classic02/classic/src/domain/RedisS3Log.js` | Redis 日志转 S3 |
| `/Users/t/ServerProjects/bravo/classic02/classic/src/consts/QueueConst.js` | SQS 队列名 |
| `/Users/t/ServerProjects/bravo/classic02/classic/tools/sam/prod/template/lambda-template.yaml` | 业务 Lambda / SQS / Schedule |
| `/Users/t/ServerProjects/bravo/classic02/classic/tools/sam/prod/template/spin1-template.yaml` | Spin 分片 Lambda |
| `/Users/t/ServerProjects/bravo/classic02/classic/tools/sam/prod/template/apigateway-template.yaml` | API Gateway 路径 |

### 0.3 spinCommon（浅读即可）

子模块：`.gitmodules` → `src/spinCommon`（`common_slots_classic.git`）

建议只读：

- `/Users/t/ServerProjects/bravo/classic02/classic/src/spinCommon/SpinProgress.js`（开奖进度、seed 重roll、连续状态）
- 任意一个 `machine/MachineInfo*.js` 了解机台结构即可

**不必**深入 `bonus/`、`result/`、`collect/` 全量机台实现。

### 0.4 不要读（.gitignore / 噪声）

`node_modules`（除需要时可看 `@private/lambda-commons`）、`pkg/`、`.idea`、`.vscode`、`package-lock.json`、`tools/cfg-excel/datas|sources|local-tools`、`tools/spin/*seed*`、`tools/sam/test/yaml`、`tools/sam/local-sam-tools`、资源包版本目录等。

框架内核（可选）：

- `/Users/t/ServerProjects/bravo/classic02/classic/node_modules/@private/lambda-commons/index.js`（`commons.exec` 分发）

---

## 1. 最近项目框架介绍（60 秒 / 3 分钟版）

### 1.1 一句话

海外 slots（经典老虎机）在线游戏后端：**Node.js + AWS Serverless**（API Gateway + 多 Lambda + SQS + EventBridge 定时），存储为 **PostgreSQL（权威配置/金币/订单）+ DynamoDB（用户局内状态）+ Redis（热数据/锁/排行榜/幂等）**，玩法引擎在 git 子模块 `spinCommon`。

### 1.2 架构分层

```
Client (Cocos/Unity)
    │  HTTPS { cmd, data, uuid, md5 }
    ▼
API Gateway（路径混淆：resu=user, redro=order, ytivitca=activity…）
    │
    ├─ Gateway Lambda  → 返回 cmd→path 路由表
    ├─ 业务域 Lambda   → user / order / activity / store / …（同代码包 index.handler）
    ├─ Spin 分片 Lambda → spin/1 … spin/N（同代码，独立并发与扩缩）
    │
    ├─ SQS Lambda (index.sqs)
    │     afterSpin / mall verify / friend mail / push …
    │
    └─ Schedule Lambda (index.schedule)
          Redis→Dynamo 同步 / Firebase 推送 / 清缓存 / 超时告警 / 日志上 S3 …
```

### 1.3 代码分层（本仓库）

| 层 | 目录 | 职责 |
|----|------|------|
| 入口 | `index.js` | handler / sqs / schedule |
| 路由 | `src/common/RequestMapping.js` | cmd → Handler 类路径 |
| Handler | `src/handler/**` | 协议入参、调用 Domain、组包返回 |
| Domain | `src/domain/**`（约 228 个） | 业务规则 |
| Spin 编排 | `src/spin/**` | 开奖前后编排（非机台细节） |
| 玩法 | `src/spinCommon/**`（子模块） | 机台/bonus/结果生成 |
| DAO | `src/dao/{postgresql,dynamodb,redis}` | 存储访问 |
| Service | `src/service/**` | SQS/Schedule/Push/MsgCache 等横切 |
| IaC | `tools/sam/{prod,beta}/template/` | CloudFormation/SAM |
| 配置 | `config/Config*.js` | 本地/Beta/Prod/东西区 |

### 1.4 与简历表述的对齐说明（面试诚实口径）

| 简历说法 | 本仓库对应证据 | 建议说法 |
|----------|----------------|----------|
| AWS Serverless（Lambda/Dynamo/PG/Redis） | `index.js` + SAM + 三套 DAO | 直接讲本项目 |
| CloudFormation/IaC | `tools/sam/**/template/*.yaml` | SAM + CFN 模板多环境部署 |
| MQ 削峰解耦 | SQS `afterSpin`、支付延迟验单 | Spin 同步返回，非关键副作用异步 |
| Outbox / 一致性 | **无经典 Outbox 表**；等价方案是「同步关键路径 + SQS 最终一致 + DynamoRedis 写回」 | 讲「类 Outbox / 可靠异步」时说明是 SQS + 写回，勿硬套表名 |
| 二级缓存 | DynamoRedis（Redis + Dynamo）+ 进程内 `N2CMemory` | 热路径 Redis，落盘 Dynamo；配置本地内存 + Redis 版本失效 |
| Firehose + S3 日志 | `LogFirehose`、`RedisS3Log`、`SendLogToS3` schedule | 日志链路存在；Athena 若简历有写，属运营侧/平台能力，本仓无 Athena SQL 文件 |
| Golang 分布式 → Serverless | **本仓主体是 Node.js** | 若简历有 Go 经历，分开讲「历史/其他项目」与「本项目 Node Serverless」 |
| WebSocket/NATS | 本仓无 WS 服务；`cpPrefix: 'cp-ws-'` 暗示外部长连接；推送主路径是 Firebase | 实时推送用 FCM；长连接若做过需讲外部服务，勿说本仓实现了 NATS |

### 1.5 3 分钟口述稿（可背）

我们最近项目是海外 slots 游戏后端。整体是 **一套 Node 代码包部署成多个 AWS Lambda**：API Gateway 按业务域拆函数（用户、订单、活动、Spin 分片等），同一入口 `index.handler` 用 `cmd` 路由到几百个 Handler。

请求先进 Gateway 拿路由表，再打到对应域。统一走 `BaseHandler`：设备登录校验、可选消息幂等缓存、加载用户和 RTP schema，再进业务。

最核心是 **Spin**：对 `userId+roomId` 加 Redis 锁，跑开奖编排（扣币、控奖、写房间状态），关键结果同步返回；排行榜、任务、活动进度等丢到 **SQS afterSpin** 异步做，避免拖垮热路径。

数据上 PG 管用户金币/订单/配置与分区流水，Dynamo 管局内状态，Redis 管锁、排行、幂等和热缓存；还有一套 **Redis→Dynamo 定时同步** 降低 Dynamo 写入压力。支付走 mall 子服务 + 延迟验单队列。推送用 Firebase + 定时/SQS。基础设施用 SAM/CloudFormation 管多环境、版本别名 `online` 发布。

---

## 2. 你在项目中的角色与主要工作（建议自述）

> 结合简历「主导海外线上游戏后端」口径；按代码职责域组织，面试时可按真实分工微调比例。

### 2.1 角色定位

**海外 slots 游戏后端核心开发 / 技术骨干**：负责 Serverless 业务后端从需求到上线的完整链路，覆盖高并发 Spin、活动系统、支付发货、异步解耦与运维配套。

### 2.2 主要事情（按面试官关心度排序）

1. **核心玩法链路（Spin）**  
   - 热路径锁、开奖编排、金币一致性、房间状态持久化  
   - 与 `spinCommon` 子模块协作（机台逻辑独立演进）  
   - 证据：`SpinHandler.js`、`GetSpinNew.js`、`domain/Spin*.js`、`domain/Remedy*.js`

2. **高并发与稳定性治理**  
   - Spin 水平分片多 Lambda、Redis 锁防重入  
   - 请求幂等（MsgCache）、Dynamo 条件写重试（AutoLock）  
   - 超时监控、钉钉告警（Schedule `RequestTimeout`）

3. **异步解耦与最终一致**  
   - afterSpin 队列消费：排行、小猪、任务、活动、联赛等  
   - DynamoRedis 写回、定时 SyncTable  
   - 证据：`SqsService.js`、`DynamoRedis.js`、`ScheduleService.js`

4. **商业化 / 支付发货**  
   - 下单、完成订单、补单、发货扇出到各活动 Domain  
   - 延迟验单（SQS + mall-server）  
   - 证据：`handler/order/*`、`domain/Mall.js`

5. **活动与运营玩法**  
   - `handler/activity/` 大量活动 Handler（通行证、爬塔、拼图、节日活动等）  
   - 活动与 Spin 数据打通（`SpinActivity`）

6. **基础设施与多环境**  
   - SAM 模板：Gateway / 业务 Lambda / Spin 分片 / SQS / Cron  
   - Config 多环境（Prod West、Beta East、只读库等）  
   - 配置热更：`N2CMemory`

7. **工程效率**  
   - 协议工具、配置 Excel、本地 SAM、Spin 相关工具链（`tools/`）  
   - Jest 测试目录 `__tests__/`

### 2.3 「我负责什么」一句话版

我主要负责 **Serverless 游戏后端的 Spin 热路径与活动/支付周边**：保证高并发下金币与局内状态正确，用锁、幂等、异步队列和缓存写回把延迟和成本压住，并用 SAM 把多 Lambda 多队列的发布运维起来。

---

## 3. 项目难点与解决方案（STAR 友好）

### 难点 A：Spin 高并发下状态正确且延迟可控

**问题**：同一用户同一机台连点/重试会导致重复扣币、状态错乱；Spin 同步路径若塞进排行榜/任务会超时。

**方案**：

1. `spin_lock:{userId}:{roomId}` Redis 分布式锁，拿不到返回 `RequestWaiting`  
2. 同步路径只做开奖与关键写；`afterSpin` 丢 SQS  
3. Spin 独立多 Lambda 分片，与活动/订单函数隔离并发

**证据路径**：

- `/Users/t/ServerProjects/bravo/classic02/classic/src/handler/spin/SpinHandler.js`
- `/Users/t/ServerProjects/bravo/classic02/classic/src/spin/GetSpinNew.js`
- `/Users/t/ServerProjects/bravo/classic02/classic/src/service/SqsService.js`（`case 'afterSpin'`）
- `/Users/t/ServerProjects/bravo/classic02/classic/tools/sam/prod/template/spin1-template.yaml`

**可量化口述（与简历对齐，需你用真实线上数字确认）**：峰值万级 QPS、可用性、P99；代码侧可讲「热路径加锁 + 副作用异步 + 函数分片」。

---

### 难点 B：客户端重试导致重复请求 / 重复发货风险

**问题**：弱网重试同一 `md5` 请求，可能重复执行业务。

**方案**：`MsgCacheService` 阶段机：

- `next`：首次处理并缓存成功结果  
- `waiting`：处理中，让客户端稍候  
- `fetchContent`：直接回放缓存成功响应  

支付完成单可对 md5 做特殊处理（避免与缓存策略冲突，代码里有 `re` 前缀一类改写，见 order 相关 Handler）。

**证据路径**：

- `/Users/t/ServerProjects/bravo/classic02/classic/src/handler/BaseHandler.js`（`cacheStage`）
- `/Users/t/ServerProjects/bravo/classic02/classic/src/service/MsgCacheService.js`
- `/Users/t/ServerProjects/bravo/classic02/classic/src/dao/redis/MsgCacheDao.js`

**面试关键词**：幂等、请求去重、结果回放、与「业务幂等键（订单号）」区分开讲。

---

### 难点 C：Dynamo 成本 / 热点 vs 一致性

**问题**：局内状态高频读写，直写 Dynamo 贵且易打热点；纯 Redis 又怕丢。

**方案**：`DynamoRedis`：

- 读：先 Redis，未命中读 Dynamo 并回填  
- 写：加锁写 Redis；缺失时落 Dynamo  
- 定时 `SyncTableData2Dynamo` / `SyncAllTableData2Dynamo` 批量刷回  
- 条件写失败用 `AutoLock.tryAgain` 重试

**证据路径**：

- `/Users/t/ServerProjects/bravo/classic02/classic/src/domain/DynamoRedis.js`
- `/Users/t/ServerProjects/bravo/classic02/classic/src/dao/tool/DynamoRedisTables.js`
- `/Users/t/ServerProjects/bravo/classic02/classic/src/service/ScheduleService.js`
- `/Users/t/ServerProjects/bravo/classic02/classic/src/domain/AutoLock.js`

**诚实边界**：这是 **写回（write-behind）缓存**，崩溃窗口存在最终一致风险；面试可讲补偿：定时同步、锁、条件表达式、失败重试与监控。

---

### 难点 D：RTP / 控奖 / 多 schema 与「公平娱乐」平衡

**问题**：既要控制长期 RTP，又要做新手保护、破产保护、付费救济、吸引局等，且多配置 schema 分流。

**方案**：

- 用户绑定 `schema`（`Classic777RemedyDao` / `RemedyDomain.getUserSchema`）  
- `SpinProgress` 内 seed 重roll、限赢、连续 free/respin 状态  
- 多层 Remedy Domain：`Remedy` / `PayRemedy` / `RWDRemedy` / `LoseRemedy` / `AttractionNew` 等

**证据路径**：

- `/Users/t/ServerProjects/bravo/classic02/classic/src/spin/GetSpinNew.js`（引入大量 Remedy）
- `/Users/t/ServerProjects/bravo/classic02/classic/src/spinCommon/SpinProgress.js`（浅读）
- `/Users/t/ServerProjects/bravo/classic02/classic/src/domain/Remedy.js` 及同目录 `*Remedy*.js`

**面试口径**：强调「配置化控奖 + 审计流水（分区 spin record）+ 测试用户 seed」，避免讲成「改随机骗玩家」。

---

### 难点 E：支付验单与发货一致性

**问题**：IAP/第三方支付回调不可靠；发货要扇出到多活动；验单耗时长。

**方案**：

- 同步：`FinishOrder` → mall `GoodsService` 发货  
- 异步：SQS `DelayVerify` / mall verify 队列补验  
- 补单 / 失败单 Handler 兜底

**证据路径**：

- `/Users/t/ServerProjects/bravo/classic02/classic/src/domain/Mall.js`
- `/Users/t/ServerProjects/bravo/classic02/classic/src/handler/order/`
- `/Users/t/ServerProjects/bravo/classic02/classic/src/service/SqsService.js`（`DelayVerify`）
- `node_modules/@private/mall-server/`（支付子包）

---

### 难点 F：大规模流水与广告数据膨胀

**问题**：Spin/广告记录量大，单表难撑。

**方案**：

- `UserSpinRecordPartitionedDao`：按房间+日期建分区，缺分区自动创建并重试  
- `UserAdRecordDao`：按日分区查询

**证据路径**：

- `/Users/t/ServerProjects/bravo/classic02/classic/src/dao/postgresql/UserSpinRecordPartitionedDao.js`
- `/Users/t/ServerProjects/bravo/classic02/classic/src/dao/postgresql/UserAdRecordDao.js`

---

### 难点 G：配置热更新与多环境

**问题**：运营改表不能每次发版；Beta/Prod、东西区、只读库并存。

**方案**：

- `config/Config*.js` 多文件  
- `N2CMemory` 轮询 Redis 配置版本，失效进程内缓存  
- SAM 打包时替换 Config；Lambda Alias `online` 做版本切换

**证据路径**：

- `/Users/t/ServerProjects/bravo/classic02/classic/config/`
- `/Users/t/ServerProjects/bravo/classic02/classic/src/domain/N2CMemory.js`
- `/Users/t/ServerProjects/bravo/classic02/classic/tools/sam/prod/template/`

---

### 难点 H：可观测与成本

**问题**：Serverless 排障难；日志存储贵。

**方案**：

- 业务日志 Firehose（`LogFirehose`）  
- Redis 临时日志定时 `SendLogToS3`（`RedisS3Log`）  
- CloudWatch + 自建超时聚合钉钉告警  
- Dynamo 容量相关定时调整（`DynamodbCfgModify`）

**证据路径**：

- `/Users/t/ServerProjects/bravo/classic02/classic/src/domain/RedisS3Log.js`
- `/Users/t/ServerProjects/bravo/classic02/classic/src/service/ScheduleService.js`（`SendLogToS3` / `RequestTimeout`）
- `/Users/t/ServerProjects/bravo/classic02/classic/src/domain/DingTalk.js`
- `/Users/t/ServerProjects/bravo/classic02/classic/src/domain/DynamodbCfgModify.js`

---

## 4. 面试官可能问的问题（含参考答纲）

### 4.1 项目与架构

**Q1：介绍最近项目？**  
→ 用 §1.5 口述稿。

**Q2：为什么用 Serverless 而不是常驻 Go 服务？**  
→ 流量波峰波谷明显（活动/节假日）；按域拆函数隔离故障与并发；运维用 SAM；冷启动用 alias/预热/合并热点（Spin 独立扩）。诚实说：长连接/强状态房间更适合常驻服务，本业务以短请求 + 推送为主。

**Q3：一次 Spin 请求经过哪些步骤？**  
→ Gateway 路由 → Spin Lambda → BaseHandler 鉴权/幂等 → SpinHandler 顶号校验 → Redis 锁 → GetSpinNew（beforeSpin/开奖/扣币/写状态）→ 组包 → Promise.all 附加字段 → unlock → afterSpin SQS。

**Q4：为什么 API 路径是倒写英文？**  
→ 网关路径混淆/防简单扫描；真实业务靠 body.cmd；见 `GatewayHandler` + `apigateway-template.yaml`。

**Q5：代码如何拆 Lambda 却共用一个包？**  
→ 同一 artifact，不同 CloudFormation Function 指向 `index.handler`/`index.sqs`/`index.schedule`，用 API 路径与事件源分流；cmd 决定 Handler。

---

### 4.2 并发、一致性、幂等（目标岗位高频）

**Q6：如何保证 Spin 不重复扣币？**  
→ 用户房间级锁 + 幂等缓存 + Dynamo 条件写；异步 afterSpin 失败不影响已返回的扣币结果，靠重试/补偿。

**Q7：你们有 Outbox 吗？**  
→ 没有独立 Outbox 表。模式是：事务内/同步路径写完关键状态后发 SQS；消费端可再查用户状态；支付用延迟验单。可类比为「应用层 Outbox」。

**Q8：消息消费失败怎么办？**  
→ `index.sqs` 记录消费失败；依赖 SQS 重试/DLQ（面试前确认线上是否配 DLQ）；业务侧尽量幂等（排行榜 incr、任务进度等要可重入）。

**Q9：Redis 与 Dynamo 不一致怎么办？**  
→ 写回延迟窗口；定时全量/按表同步；读穿透回填；锁降低并发写乱序；关键货币仍以 PG 为准。

**Q10：分布式锁怎么实现？误删怎么办？**  
→ SET key token EX NX；解锁 Lua 校验 token；见 `RedisLockUtils.js`。

**Q11：如何做接口幂等？**  
→ 设备 uuid + 请求 md5；阶段机 waiting/回放；业务层订单号/发货单另做幂等。

---

### 4.3 存储与性能

**Q12：为什么 PG + Dynamo + Redis 三套？**  
→ PG：强一致账户、配置、流水；Dynamo：海量用户局内 KV、易扩；Redis：微秒级热数据与锁。按访问模式选型。

**Q13：接口如何从 300ms 优化到 100ms 内？（简历数字）**  
→ 结合代码讲：并行 `Promise.all` 预取、DynamoRedis 热读、afterSpin 下沉、Spin 分片减排队、配置内存缓存。用你真实压测/监控数字，不要死背。

**Q14：分库分表怎么做的？**  
→ Spin/广告流水按日（及房间）分区；用户侧更多是多 schema（RTP 组）而非传统用户哈希分库——讲清楚「分区表 + 多 schema」，避免夸大。

---

### 4.4 支付 / 活动 / 推送

**Q15：支付流程？**  
→ GenerateOrder → 客户端支付 → FinishOrder 验单发货 → 失败 Repair；异步 DelayVerify 补验。

**Q16：活动很多如何扩展？**  
→ Handler 按活动拆文件；Spin 后 `SpinActivity` 统一分发；配置表驱动；注意锁（拼图扣道具等）。

**Q17：实时消息怎么做？**  
→ 本仓主推 Firebase；长连接若体系有独立 WS 服务可补充，但别说本仓库实现了 NATS。

---

### 4.5 运维 / AWS

**Q18：怎么发布？如何回滚？**  
→ SAM 打版本 + Alias `online` 切换；模板里可见 Version/Alias 资源。

**Q19：如何监控告警？**  
→ CloudWatch + 自研超时统计钉钉；日志 Firehose/S3；简历若写 Grafana/Prometheus，说明是平台侧或其他组件，本仓主要是 CW + 业务告警。

**Q20：冷启动怎么处理？**  
→ 业务拆分降低单函数包体影响、provisioned concurrency（若有）、合并热点请求、初始化放 `exports.init` 复用执行环境。

---

### 4.6 目标岗位迁移题（电商客服 SaaS：订单/消息/WS/Kafka/Outbox）

目标 JD 关键词：Go、Gin/GORM/go-zero、MySQL/Mongo/Redis、Kafka、WebSocket/gRPC、Outbox、幂等、AI 回复。

**可迁移叙事**：

| SaaS 场景 | 你可映射的经验 |
|-----------|----------------|
| 订单状态机 + 幂等 | Mall 下单发货、MsgCache、补单 |
| 消息可靠投递 | SQS afterSpin、延迟验单、重试 |
| 在线状态 / 推送 | Firebase 推送调度、登录设备/token |
| 多系统对接 | 支付渠道、Adjust、Facebook Instant 工具 |
| 异步链路 | Schedule + SQS 编排 |
| AI 辅助研发 | 简历已写 Cursor/Codex；强调审查 AI 代码、单测与安全 |

**需补强的诚实差距**（面试别装熟）：

- 本项目主力语言是 **Node.js**，目标岗偏 **Go** → 准备 Go 并发、Gin 小项目或把「GMP/channel」用在讲设计而非本仓代码  
- **Kafka** vs SQS：讲清差异（消费模型、顺序、重试）并说明你熟悉「队列解耦」思想  
- **WebSocket 客服长连接**：本仓弱，准备通用方案（心跳、重连、ACK、多端互踢——你设备顶号逻辑可类比）  
- **经典 Outbox 表**：能画「本地消息表 + 定时投递 + 消费幂等」标准图，并对比你们 SQS 方案权衡

---

### 4.7 AI 相关（JD 强调）

**Q21：你怎么用 AI 写代码？如何保证质量？**  
→ 生成脚手架/单测/重构草案 → 人审边界条件、幂等、锁、金额字段 → 跑 Jest / 关键路径手工回归 → 不把 AI 输出直接合入支付与扣币路径。

**Q22：如何让 AI 读这个项目？**  
→ 把本文 §0 路径清单丢给模型；先读入口与 BaseHandler/Spin，再下钻难点文件；忽略 gitignore 与 spinCommon 细节。

---

## 5. 关键目录速查（按业务域）

```
classic/
├── index.js                          # Lambda 三入口
├── config/                           # 多环境配置
├── src/
│   ├── common/                       # 路由、锁、协议、工具
│   ├── consts/                       # 状态码、队列名、枚举
│   ├── handler/                      # 对外接口
│   │   ├── BaseHandler.js
│   │   ├── GatewayHandler.js
│   │   ├── spin/                     # Spin API
│   │   ├── order/                    # 支付
│   │   ├── activity/                 # 大量活动
│   │   ├── user|mail|store|…         # 其他域
│   ├── spin/                         # Spin 编排（读这个，少读 spinCommon）
│   ├── spinCommon/                   # 子模块玩法（浅读）
│   ├── domain/                       # 业务核心（约 228）
│   ├── dao/{postgresql,dynamodb,redis}
│   ├── service/                      # SQS/Schedule/Push/Cache
│   └── push/                         # FB 推送脚本等
├── tools/sam/                        # IaC 与发布
├── tools/{spin,cfg-excel,http,…}     # 内部工具
├── scripts/                          # 运维脚本
└── __tests__/                        # Jest
```

---

## 6. 建议你面试前自己确认的数字 / 事实（代码无法代替）

1. 线上峰值 QPS、Spin P99、可用性是否与简历 1 万 QPS / 99.98% 一致  
2. 你是否主导 SAM 迁移，还是维护既有 Serverless  
3. Athena / Grafana / NATS / Go 服务是否在**其他仓库或平台**，避免本仓对不上被追问穿帮  
4. 你个人贡献最大的 2～3 个模块（建议选：Spin 锁与异步、DynamoRedis、某个复杂活动或支付补单）  
5. 一次真实故障：现象 → 排查（日志/Redis/Dynamo/SQS）→ 修复 → 防再发

---

## 7. 精简「简历 → 代码」对照表

| 简历 bullet | 代码锚点 |
|-------------|----------|
| Serverless Lambda | `index.js` + `tools/sam/prod/template/` |
| API 鉴权限流层 | `BaseHandler` + API Gateway（限流在 AWS 侧/commons） |
| MQ 解耦 Spin/活动 | `SqsService.afterSpin` + `QueueConst` |
| 数据一致低延迟 | 锁 + MsgCache + DynamoRedis + PG 金币 |
| Firehose/S3 日志 | `LogFirehose` / `RedisS3Log` / `SendLogToS3` |
| 二级缓存降延迟 | DynamoRedis + N2CMemory |
| 分表/大数据量 | `UserSpinRecordPartitionedDao` / Ad 分区 |
| Docker 本地 | 本地 Dynamo/SQS/Redis endpoint（Config.js）+ tools |
| S3/CloudFront 资源 | `tools/package-resources`、热更 `scripts/hot-update` |
| DynamoDB 模型 | `src/dao/dynamodb/*`、`UserRoomInfoDao` |
| API Gateway | `apigateway-template.yaml` |
| 监控告警 | CloudWatch + `DingTalk` + RequestTimeout |

---

## 8. 一页纸速记（开场 30 秒）

**项目**：海外 slots Serverless 后端（Node + AWS）。  
**架构**：API Gateway → 多 Lambda（业务域 + Spin 分片）→ PG/Dynamo/Redis；SQS/Schedule 做异步与运维。  
**我负责**：Spin 热路径正确性与性能、异步解耦、缓存写回、支付发货与活动扩展、SAM 多环境。  
**最难**：高并发 Spin 下「不重扣、不超卖状态、又要快」——锁 + 幂等 + 关键同步/非关键异步 + Redis 写回 Dynamo。  
**迁移到贵司**：同一套「订单/消息可靠、幂等、异步、可观测」经验，可快速上手客服 SaaS；Go/Kafka/WS 按贵司栈补齐，设计模式可复用。

---

*文档结束。其他模型请从 §0 路径清单开始精读，并结合 §3 难点与 §4 问答组织回答。*
