<!--
 * @Author: t 921865806@qq.com
 * @Date: 2025-12-19 17:33:28
 * @LastEditors: t 921865806@qq.com
 * @LastEditTime: 2025-12-21 18:38:31
 * @FilePath: /examples/demo_cluster/md/game_type.md
 * @Description: 这是默认设置,请设置`customMade`, 打开koroFileHeader查看配置 进行设置: https://github.com/OBKoro1/koro1FileHeader/wiki/%E9%85%8D%E7%BD%AE
-->
### 分区分服的选择
| 游戏类型 | 区服关系 | 说明 | |----------|----------|------| 
| MMORPG | 1区 = 1服 = 1Game | 数据隔离，不能跨服 | 
| SLG/卡牌 | 1区 = N服，可合服 | 服之间可以合并 | 
| 休闲/棋牌 | 全区全服 | 所有玩家在一起 |
| Slots（你的项目） | 全区全服 | 玩家数据全局共享 |

### slg游戏架构分析
一、典型节点类型和职责
| 节点类型 | 数量 | 有状态/无状态 | 职责 | 
|---------|------|-------------|------|
| Gate | 多个 | 无状态 | 连接管理、消息转发、协议加解密 | 
| Game/Scene | 多个 | 有状态 | 地图数据、玩家城池、资源采集点 | 
| Player | 多个 | 有状态 | 玩家数据、背包、英雄、科技 | | Battle | 多个 | 无状态 | 战斗计算（纯计算，不存状态） | 
| Alliance | 1-2个 | 有状态 | 联盟数据、联盟战 | | Chat | 1-2个 | 无状态 | 聊天消息转发 | 
| Center | 1个 | 有状态 | 玩家位置、节点协调、全局数据 | | Rank | 1个 | 有状态 | 排行榜 |
二、核心设计：地图分片
SLG最核心的问题是大地图如何分布到多个节点：
┌─────────────────────────────────────────────────────────┐
│                    世界地图 (1000x1000)                  │
├─────────────┬─────────────┬─────────────┬──────────────┤
│  Scene-1    │  Scene-2    │  Scene-3    │  Scene-4     │
│ (0,0)-      │ (250,0)-    │ (500,0)-    │ (750,0)-     │
│ (250,250)   │ (500,250)   │ (750,250)   │ (1000,250)   │
├─────────────┼─────────────┼─────────────┼──────────────┤
│  Scene-5    │  Scene-6    │  Scene-7    │  Scene-8     │
│ (0,250)-    │ (250,250)-  │ (500,250)-  │ (750,250)-   │
│ (250,500)   │ (500,500)   │ (750,500)   │ (1000,500)   │
├─────────────┼─────────────┼─────────────┼──────────────┤
│    ...      │    ...      │    ...      │    ...       │
└─────────────┴─────────────┴─────────────┴──────────────┘
// 根据坐标计算所属Scene节点
func GetSceneNodeId(x, y int32) string {
    // 假设每个Scene负责250x250的区域
    sceneX := x / 250
    sceneY := y / 250
    return fmt.Sprintf("scene-%d-%d", sceneX, sceneY)
}

三、跨节点战斗实现
这是SLG最复杂的部分。假设玩家A（在Scene-1）攻击玩家B（在Scene-2）：

方案1：Battle节点协调（推荐）

1. 玩家A发起攻击 → Scene-1
2. Scene-1 → Center: 查询玩家B位置
3. Center → Scene-1: 玩家B在Scene-2
4. Scene-1 → Battle-X: 创建战斗任务 {attackerId, defenderId, ...}
5. Battle-X → Scene-1: 获取攻击方数据（军队、英雄、科技加成）
6. Battle-X → Scene-2: 获取防守方数据（城防、驻军、陷阱）
7. Battle-X: 执行战斗计算
8. Battle-X → Scene-1: 扣除攻击方损失
9. Battle-X → Scene-2: 扣除防守方损失、更新城池状态
10. Battle-X → Gate: 推送战斗结果给双方玩家
Battle节点为什么是无状态的？

战斗是一次性计算，不需要持久化
战斗数据从Scene节点获取，结果写回Scene节点
无状态便于水平扩展和负载均衡
方案2：Scene直接通信（简单场景）


1. Scene-1 → Scene-2: RPC调用，传递攻击数据
2. Scene-2: 执行战斗计算
3. Scene-2 → Scene-1: 返回战斗结果
4. 双方各自更新状态

四、有状态 vs 无状态详解
有状态节点
| 节点 | 状态内容 | 为什么有状态 | 
|------|---------|-------------| 
| Scene | 地图格子、城池、资源点、NPC | 频繁读写，DB扛不住 | 
| Player | 玩家数据、背包、英雄 | 玩家操作频繁 | 
| Alliance | 联盟成员、联盟建筑、联盟战 | 多人共享数据 |

有状态节点的特点：

数据缓存在内存
定时持久化到DB
玩家绑定到特定节点
需要故障迁移机制
无状态节点
| 节点 | 为什么无状态 | 
|------|-------------| 
| Gate | 只做转发，不存数据 | 
| Battle | 纯计算，数据从其他节点获取 | 
| Chat | 消息转发，不存历史 |

无状态节点的特点：

可以随意扩缩容
任意节点都能处理请求
故障后直接重启，无需迁移

五、数据流示例

玩家登录流程
Client → Gate → Center: 分配Player节点
                Center → Client: 返回Player节点ID
Client → Gate → Player-X: 加载玩家数据
                Player-X → Client: 返回玩家信息
Client → Gate → Scene-Y: 加载玩家城池所在地图
                Scene-Y → Client: 返回地图数据
采集资源流程
Client → Gate → Scene-Y: 派兵采集(x, y)
                Scene-Y: 检查格子状态、创建采集任务
                Scene-Y → Client: 采集开始
                Scene-Y: 定时器完成后 → Player-X: 增加资源
跨服战斗（联盟战）
Alliance-1 → Center: 发起跨服战
Center → 目标服Center: 转发战斗请求
目标服Center → Battle集群: 创建大规模战斗
Battle集群: 分布式计算战斗结果
Battle集群 → 双方Alliance: 同步结果
六、Cherry框架下的实现建议
基于你现有的Cherry框架，SLG架构可以这样组织：

// 节点类型定义
const (
    NodeTypeGate     = "gate"
    NodeTypePlayer   = "player"   // 新增
    NodeTypeScene    = "scene"    // 新增（替代game）
    NodeTypeBattle   = "battle"   // 新增
    NodeTypeAlliance = "alliance" // 新增
    NodeTypeCenter   = "center"
)

// Scene节点 - 管理地图分片
type SceneNode struct {
    sceneId   string           // 如 "scene-0-0"
    minX, minY int32           // 负责区域
    maxX, maxY int32
    cells     map[int64]*Cell  // 格子数据 key=x*10000+y
    cities    map[int64]*City  // 城池数据
}

// Player节点 - 管理玩家数据
type PlayerNode struct {
    players map[int64]*PlayerData // 在线玩家
}

// Battle节点 - 无状态战斗计算
type BattleNode struct {
    // 无状态，每次战斗都是独立的
}
七、关键问题解答
Q: Battle节点需要多个吗？ A: 是的，根据并发战斗数量。一般 Battle节点数 = 预期并发战斗数 / 单节点处理能力

Q: 玩家数据放Player节点还是Scene节点？ A: 分开放。Player节点管玩家私有数据（背包、英雄），Scene节点管地图相关数据（城池位置、采集状态）

Q: 如何处理边界问题？ A: 玩家在Scene边界移动时，需要跨节点同步。通常用"AOI（Area of Interest）"算法，只同步视野范围内的数据。

需要我针对某个具体方面深入讲解吗？比如：

地图分片的具体实现
跨节点战斗的详细流程
联盟系统的设计
基于Cherry框架的代码示例