这是游戏后端面试、架构设计中最重要的一块内容之一。很多公司(腾讯、网易、米哈游、莉莉丝、字节游戏等)问"数据如何落地(Data Persistence)"，其实就是考察：

> **性能、可靠性、一致性、容灾、安全、扩展性**之间如何做Trade-off。

我按照大型游戏公司的实际架构来总结。

---

# 一、游戏服务器的数据落地(Persistence)方案总结（重点）

## 一张图先理解

```
                玩家请求
                    │
                    ▼
             Game Server(内存)
                    │
       ┌────────────┼─────────────┐
       │            │             │
       ▼            ▼             ▼
   Redis缓存     MQ(Kafka)     直接DB
       │            │             │
       ▼            ▼             ▼
    MySQL      Async Worker    MySQL
```

现代游戏基本不会只有一种方案，而是**多种方案混合使用**。大型游戏通常把实时逻辑放在内存，把持久化异步化，并根据数据的重要程度选择不同写策略。([Birdor Blog][1])

---

# 方案一：同步写数据库（Synchronous DB）

流程

```
玩家点击升级

↓

GameServer

↓

UPDATE player SET level=20

↓

数据库返回OK

↓

告诉客户端成功
```

## 特点

优点

✔ 数据最安全

✔ 强一致

✔ 崩服不会丢

缺点

❌ 慢

数据库RT：

```
2~10ms

高峰：

20ms+

甚至100ms
```

如果：

```
10000玩家

每秒20操作

就是

20万TPS

数据库直接炸。
```

所以：

> **实时游戏几乎不会所有操作同步写DB。** ([Birdor Blog][1])

---

## 适用场景

例如：

充值

扣钻石

购买礼包

邮件领取

交易

拍卖行

原因：

不能丢。

例如：

```
100元充值

必须：

收到钱

DB成功

才返回成功
```

否则：

钱没了。

---

# 方案二：内存运行 + 定时落地（最经典）

也是MMORPG最经典方案。

流程：

```
登录

↓

读DB

↓

玩家对象进入内存

↓

整个游戏过程

全部操作内存

↓

每30秒

写一次DB
```

例如：

```
HP

MP

经验

位置

Buff

技能CD

```

全部：

```
Memory
```

只有：

```
30秒

或者

退出

或者

切地图

写数据库
```

---

## 优点

快。

一次攻击：

```
Memory++

```

几乎：

几十ns

数据库：

几ms

差距：

100000倍。

---

## 缺点

如果：

服务器突然挂：

```
30秒数据

没了。
```

所以：

MMO：

通常：

```
30秒自动保存

退出保存

重要事件保存
```

结合使用。([Birdor Blog][1])

---

## 适用游戏

MMORPG

开放世界

沙盒

SLG

几乎都是。

---

# 方案三：事件触发保存(Event Save)

只有：

发生重要事件：

才写数据库。

例如：

```
升级

获得装备

完成任务

充值

获得金币

```

不是：

每秒保存。

而是：

```
事件发生

↓

立即保存
```

优点：

DB压力小。

现代游戏大量采用"关键事件保存"来减少数据库压力。([Birdor Blog][1])

---

# 方案四：异步保存（Async Write）★★★★★

这是大型游戏最常见。

流程：

```
玩家升级

↓

修改Memory

↓

Push Queue

↓

立即返回客户端

↓

后台线程

↓

写DB
```

例如：

```
Game Logic Thread

↓

Save Queue

↓

DB Thread

↓

MySQL
```

游戏不卡。

数据库慢：

不影响玩家。

---

## 优点

TPS非常高。

例如：

```
10000次修改

↓

Queue

↓

后台

Batch

↓

一次Insert 100条
```

数据库压力：

降低10倍以上。

---

## 缺点

如果：

Queue没持久化：

服务器炸：

数据丢。

所以：

通常：

```
Memory

+

MQ

+

RedoLog

```

一起使用。

---

# 方案五：消息队列（Kafka / RabbitMQ / Pulsar）

大型游戏：

不会：

GameServer直接写数据库。

而是：

```
Game

↓

Kafka

↓

Persistence Service

↓

MySQL
```

优点：

削峰

解耦

失败重试

批量写

水平扩容

事件还能用于审计、反作弊、数据分析等。([Birdor Blog][1])

---

例如：

```
打怪

↓

KillMonster Event

↓

Kafka

↓

多个消费者

```

消费者：

```
经验

金币

日志

统计

排行榜

```

全部：

独立消费。

---

# 方案六：Redis + DB

最常见。

```
读：

Redis

↓

没有

↓

DB

↓

回填Redis
```

写：

```
Memory

↓

Redis

↓

Async DB
```

优点：

数据库压力：

下降90%。

---

适合：

排行榜

玩家信息

在线状态

聊天

好友

队伍

---

# 方案七：Write Through

```
Memory

↓

Redis

↓

DB
```

每次：

同步：

Redis

DB

一致。

优点：

安全。

缺点：

慢。

适合：

商城。

---

# 方案八：Write Behind（最流行）

```
Memory

↓

Redis

↓

立即返回

↓

后台

↓

DB
```

Redis：

变缓存。

后台：

Flush。

大型游戏广泛采用Write-Behind（异步写回）来提高吞吐。([Birdor Blog][1])

---

# 方案九：Snapshot（快照）

例如：

Minecraft

Factorio

沙盒

MMO

整个：

```
Player Object

↓

Serialize

↓

Blob

↓

DB
```

不是：

更新：

100字段。

而是：

整个对象。

优点：

简单。

缺点：

大对象。

---

# 方案十：Event Sourcing（越来越流行）

传统：

```
金币=1000
```

数据库：

只有：

```
1000
```

Event Sourcing：

保存：

```
+100

-20

+50

+500

```

最后：

Replay：

得到：

```
1000
```

优点：

可追溯

可恢复

方便反作弊

适合金融、经济系统。([Birdor Blog][1])

---

# 各方案对比

| 方案             | 性能    | 数据安全  | 一致性       | 实现复杂度 | 适合场景     |
| -------------- | ----- | ----- | --------- | ----- | -------- |
| 同步DB           | ⭐     | ⭐⭐⭐⭐⭐ | 强一致       | ⭐     | 充值、支付、交易 |
| 内存+定时保存        | ⭐⭐⭐⭐⭐ | ⭐⭐⭐   | 最终一致      | ⭐⭐    | MMO、SLG  |
| 事件保存           | ⭐⭐⭐⭐  | ⭐⭐⭐⭐  | 强一致(关键事件) | ⭐⭐    | 升级、任务奖励  |
| 异步写队列          | ⭐⭐⭐⭐⭐ | ⭐⭐⭐⭐  | 最终一致      | ⭐⭐⭐   | 大型在线游戏   |
| Redis+DB       | ⭐⭐⭐⭐⭐ | ⭐⭐⭐   | 最终一致      | ⭐⭐⭐   | 热数据、排行榜  |
| Write Through  | ⭐⭐⭐   | ⭐⭐⭐⭐⭐ | 强一致       | ⭐⭐⭐   | 商城、库存    |
| Write Behind   | ⭐⭐⭐⭐⭐ | ⭐⭐⭐   | 最终一致      | ⭐⭐⭐⭐  | 高频写入     |
| Event Sourcing | ⭐⭐⭐⭐  | ⭐⭐⭐⭐⭐ | 最终一致      | ⭐⭐⭐⭐⭐ | 经济系统、审计  |

---

# 不同架构的数据落地

| 架构  | 数据落地特点                                                       |
| --- | ------------------------------------------------------------ |
| 单体  | 游戏服直接访问数据库，简单但扩展有限                                           |
| 分布式 | 每个游戏服维护本地内存状态，通过MQ/缓存统一持久化                                   |
| 微服务 | 每个服务拥有自己的数据库，跨服务通常采用事件驱动、Saga、Outbox等模式，而不是跨库事务。([arXiv][2]) |

---

# 二、日志落地(Log Persistence)方案总结

日志通常分成**运行日志**和**业务日志**两大类。

```
                    日志
                      │
      ┌───────────────┼────────────────┐
      │               │                │
   系统日志        业务日志         数据分析日志
```

---

## 1. 文件日志（File Log）

```
GameServer

↓

log.txt
```

用途：

* Debug
* Crash定位
* Error排查

常见：

```
log4cpp

spdlog

zap

logrus
```

---

## 2. 数据库日志

例如：

```
充值记录

交易记录

封号记录
```

直接：

```
Insert
```

特点：

可查询。

但：

不能大量写。

---

## 3. MQ日志（推荐）

```
Game

↓

Kafka

↓

Log Service

↓

ES
```

这是互联网、大型游戏最常见方案。

---

## 4. ELK（Elasticsearch + Logstash + Kibana）

```
Game

↓

File

↓

Filebeat

↓

Logstash

↓

ES

↓

Kibana
```

优点：

搜索：

```
玩家ID

服务器ID

错误码

```

秒查。

---

## 5. ClickHouse

越来越多游戏：

统计：

```
登录

充值

在线

留存

```

全部：

```
ClickHouse
```

因为：

TB级：

秒查。

---

## 6. 对象存储

例如：

```
S3

OSS

COS
```

保存：

Replay

录像

聊天记录归档

超长日志

成本低。

---

## 7. 审计日志（Audit Log）

例如：

```
管理员：

发金币

删号

封号

GM命令
```

必须：

永久保存。

不可修改。

通常：

DB+对象存储。

---

# 日志分类总结

| 日志类型    | 落地位置               | 是否实时 | 典型用途        |
| ------- | ------------------ | ---- | ----------- |
| 运行日志    | 本地文件               | 是    | Debug、异常定位  |
| 错误日志    | 文件/ELK             | 是    | Crash、Error |
| 玩家行为日志  | Kafka → ClickHouse | 准实时  | 埋点、运营分析     |
| 充值/交易日志 | MySQL              | 是    | 财务、审计       |
| GM操作日志  | MySQL + 对象存储       | 是    | 审计、追责       |
| 聊天日志    | Kafka → ES/对象存储    | 准实时  | 举报、风控       |
| 对战事件日志  | Kafka → 数据仓库       | 准实时  | 回放、反作弊、数据分析 |

---

## 大型游戏公司的推荐实践（★★★★★）

**数据落地：**

```
GameServer(内存)
      │
      ├── Redis（热数据）
      ├── Kafka（事件队列）
      ├── Async Worker（异步持久化）
      └── MySQL（最终持久化）
```

**日志落地：**

```
GameServer
      │
      ├── 本地日志(spdlog/zap)
      ├── Kafka
      ├── Elasticsearch（检索）
      ├── ClickHouse（运营分析）
      └── 对象存储（归档）
```

这种架构兼顾了**低延迟、高吞吐、数据安全、可扩展性**，也是目前大型实时游戏后端最常见的整体方案。([Birdor Blog][3])

[1]: https://blog.birdor.com/game-server-development-06-persistence/?utm_source=chatgpt.com "Game Server Development Series — Part 6: Databases & Persistence | Birdor Blog"
[2]: https://arxiv.org/abs/2103.00170?utm_source=chatgpt.com "Data Management in Microservices: State of the Practice, Challenges, and Research Directions"
[3]: https://blog.birdor.com/game-server-development-03-architecture/?utm_source=chatgpt.com "Game Server Development Series — Part 3: Core Architecture | Birdor Blog"
