# Actor 模型下的 Slots 资产结算设计与 Go 实现导读

本文基于本项目的实际结构，而不是泛用的微服务模板。目标是让 Slots Spin 在保持低延迟的同时，做到：

- 同一请求重试不会重复扣钱、不会产生两次不同结果；
- 金币可以严格对账；
- Spin 房间状态与活动状态可以异步更新；
- Game 节点崩溃、NATS 临时不可用、消费者重复投递后，数据可以恢复；
- 新活动、新代币可以不断增加，而不污染 Slots 核心代码。

本文先讲边界，再给出可以逐步手动放入项目的代码。代码是设计参考；接入时应按当前 protobuf、GORM 版本和 AWS SDK 版本调整。

## 1. 先从当前项目的 Actor 所有权出发

当前 Spin 的客户端协议路由到：

```text
Gate
  -> game.player.<uid>
       -> player Actor 的 Local mailbox
            -> ActorRoom.Spin()
```

`ActorRoom` 虽然名字像 Actor，但当前它由 `actorPlayer` 持有：

```go
type actorPlayer struct {
    pomelo.ActorBase
    playerData *PlayerData
    slotsRoom  *slotsRoom.ActorRoom
}

func NewActorPlayer(app cfacade.IApplication) *actorPlayer {
    return &actorPlayer{
        slotsRoom: slotsRoom.NewActorRoom(app),
    }
}
```

并且 Spin 被注册到 `player.<uid>` 的 Local mailbox：

```go
p.Local().Register("spin", p.slotsRoom.Spin)
```

所以在目前实际调用路径中，`RoomDataManager` 是 **一个玩家 Actor 私有的房间状态容器**。`roomId` 作为 map key 是正确的：玩家 ID 已由 Actor 实例隔离，不必在这张私有 map 内再拼接 `userId:roomId`。

### 1.1 Actor 解决了什么，没解决什么

Actor 解决的是**在线、同一玩家、同一 Game 节点内**的串行问题：

```text
同一 player.<uid> 的 spin / bonus / collect
  -> 同一个 mailbox
  -> 按队列顺序一个一个执行
  -> 可以安全修改 Actor 私有内存，不需要 mutex
```

Actor 不解决：

- 进程崩溃后内存丢失；
- 客户端超时重发；
- 支付回调与 Spin 并发；
- 节点切换后短时间双写；
- PostgreSQL 与 DynamoDB 的跨库事务；
- NATS 普通发布时消费者离线导致的消息丢失。

因此正确的分工是：

```text
Actor: 命令排序、会话内存、快速计算
PG:    金币权威余额、账本、幂等操作、可靠事件记录
DDB:   不同关卡状态、活动 JSON、可最终一致投影
JetStream: 可靠把已经提交的事实扇出给异步消费者
```

## 2. 同步边界：Spin 返回前究竟必须完成什么

只有不可回退的核心事实才应在同步链路完成：

```text
1. 校验 request_id；重复请求直接回放旧结果
2. 用 RoomDataInfo 的副本计算本次结果
3. 一个 PG 事务内完成：
   - 条件扣除下注 / 发放中奖；
   - 写不可变资产账本；
   - 写本次 Spin 的幂等结果；
   - 写 Outbox 事件。
4. PG commit 成功后替换 Actor 内存 RoomDataInfo
5. 返回 SpinResponse
```

以下事情不等待：DDB 房间快照、活动任务、活动代币、排行榜、分析、Push。

这不是降低可靠性。可靠性来自 Outbox：事件和金币在同一个 PG 事务里提交。即使 Game 进程在返回响应后立刻崩溃，后台 Relay 仍会继续把待发布事件送入 JetStream。

## 3. 三种数据不要混用一种写法

| 数据 | 例子 | 正确写法 | 不要使用 |
|---|---|---|---|
| 金融核心资产 | 金币、钻石、付费货币 | PG 同一事务：余额 + 不可变账本 + operation | 内存改完以后异步刷 PG |
| Slots 状态 | FreeSpin、Respin、种子计数、关卡独有字段 | Actor 内存 + 可靠事件 + DDB 快照 | 只依赖普通异步队列 |
| 活动状态 | 进度、活动币、骰子、Buff | JetStream 异步消费 + DDB 条件写 + eventId 幂等 | Spin 内同步远程调用活动节点 |

当前 `write_behind_queue` 很适合第三方投影的最后快照：同一玩家同一房间连续 100 次变更，最终只写版本 100。

但它不能用于金币账本或每次 Spin 事件：当前组件队列与批处理 map 都在进程内存，且同 key 会合并覆盖；进程异常时未刷任务无法恢复。

## 4. 首先修正 RoomDataManager 的 Actor 私有实现

这里不加锁；它必须只由所属 `player.<uid>` Actor handler 调用。真正需要修的是 map 初始化、快照深拷贝和入队失败处理。

```go
// nodes/game/db/dynamodb/room_data_manager.go
func NewRoomDataManager(app cfacade.IApplication) *RoomDataManager {
    comp := app.Find("db_write_queue")

    var queue *dbQueue.DBWriteQueueComponent
    if comp != nil {
        queue = comp.(*dbQueue.DBWriteQueueComponent)
    }

    return &RoomDataManager{
        dbQueueComp:  queue,
        roomDataInfo: make(map[int32]*slotsModel.RoomDataInfo),
    }
}

// ReplaceData 在 PG 结算成功后调用。
// 该函数的调用者必须已经处于拥有它的 player Actor 中。
func (s *RoomDataManager) ReplaceData(roomID int32, next *slotsModel.RoomDataInfo) {
    s.roomDataInfo[roomID] = next
}

// SaveSnapshot 只表达"已提交的内存状态希望尽快写成快照"。
// 它失败不能回滚已完成的金币结算。真正恢复来源是可靠 Spin 事件。
func (s *RoomDataManager) SaveSnapshot(ctx context.Context, roomID int32) bool {
    if s.dbQueueComp == nil {
        return false
    }

    data := s.roomDataInfo[roomID]
    if data == nil {
        return false
    }

    // 不能把可能继续被 Actor 修改的对象直接交给异步 goroutine。
    // 先构造 JSON 快照，后续异步队列持有的是不可变 []byte。
    payload, err := json.Marshal(data)
    if err != nil {
        return false
    }

    return s.dbQueueComp.SubmitTask(&dbQueue.DbWriteTask{
        Table:      tableName,
        ExtraKeyId: strconv.FormatInt(int64(data.RoomId), 10),
        // DbWriteTask.PlayerID 是现有写后队列组件固定的分片字段名；
        // 传入的业务主键仍然是 SlotsUser.UserID。
        PlayerID:   data.UserId,
        OpType:     dbQueue.OpUpdate,
        Data:       payload,
    })
}
```

### Go 注意事项：异步快照与指针逃逸

下列写法常见但危险：

```go
// 错误：后台 worker 取得的是指针。主 Actor 继续改 state 后，
// worker 最终序列化的很可能不是提交时的状态。
queue.SubmitTask(&Task{Data: state})
```

正确方式是交付不可变值：`json.Marshal(state)`、明确的深拷贝，或 immutable event struct。

## 5. 金币采用现有 SlotsUser.Money int64

当前项目的 `SlotsUser.Money` 已经是 `int64`，对应 PostgreSQL 的整数列，因此本项目不需要再增加 `money_units`。后文统一使用现有 `money` 列作为 Gold 权威余额。

需要保持一条不变量：所有入口都必须以整数最小单位修改 `Money`，不能在结算链路中临时转成 `float64`。如果策划倍率需要小数，应先用定点数/有理数计算出最终整数 Gold，再进入资产事务。

### 5.1 最小账本表

```sql
CREATE TABLE IF NOT EXISTS asset_operation (
    operation_id UUID PRIMARY KEY,
    user_id BIGINT NOT NULL,
    operation_type TEXT NOT NULL,
    request_id TEXT NOT NULL,
    status TEXT NOT NULL,
    response_payload BYTEA,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    completed_at TIMESTAMPTZ,
    UNIQUE (user_id, operation_type, request_id)
);

CREATE TABLE IF NOT EXISTS asset_ledger (
    ledger_id BIGSERIAL PRIMARY KEY,
    operation_id UUID NOT NULL REFERENCES asset_operation(operation_id),
    user_id BIGINT NOT NULL,
    asset_kind TEXT NOT NULL,
    delta BIGINT NOT NULL CHECK (delta <> 0),
    balance_after BIGINT NOT NULL,
    reason TEXT NOT NULL,
    source_type TEXT NOT NULL,
    source_id TEXT NOT NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    UNIQUE (operation_id, asset_kind, reason)
);

CREATE INDEX IF NOT EXISTS idx_asset_ledger_player_time
    ON asset_ledger (user_id, created_at DESC);

CREATE TABLE IF NOT EXISTS domain_outbox (
    event_id UUID PRIMARY KEY,
    aggregate_type TEXT NOT NULL,
    aggregate_id TEXT NOT NULL,
    event_type TEXT NOT NULL,
    payload JSONB NOT NULL,
    status TEXT NOT NULL DEFAULT 'PENDING',
    retry_count INT NOT NULL DEFAULT 0,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    published_at TIMESTAMPTZ
);
```

`asset_operation` 的唯一约束是本次命令的业务幂等线；`asset_ledger` 是对账事实；`domain_outbox` 是跨系统可靠发送的桥。

## 6. 不要让 Spin 引擎直接修改真实 RoomDataInfo

计算随机结果前复制状态，结算成功后再替换：

```go
current := r.roomDataManager.GetData(ctx, userID, roomID)
if current == nil {
    return nil, errNoRoomData
}

// RoomDataInfo 当前都是值字段；Clone 是安全浅拷贝。
// 将来加入 map/slice/pointer 字段时，Clone 必须做深拷贝。
next := current.Clone()
next.Version++
next.MarkDirty()

result, err := spinManager.ReadySPin(
    ctx, roomID, ruleID, false, int(bet), config, next, r.roomDataManager,
)
if err != nil {
    return nil, err
}

// 这里只是计算；不能提前 r.roomDataManager.ReplaceData(roomID, next)。
plan := SpinPlan{
    RoomID: roomID,
    Bet: bet,
    Win: totalWin(result),
    NextState: next,
    Result: result,
}
```

原因：若 PG 事务因为余额不足、超时、死锁回滚，真实内存状态绝对不能已经消费掉一次 FreeSpin 或推进 Seed。

### 6.1 建议把引擎输出收敛为 SpinPlan

```go
type SpinPlan struct {
    RoomID    int32
    Bet       int64
    Win       int64
    NextState *slotsModel.RoomDataInfo
    Result    *pb.SpinResult

    // 给恢复与审计使用。不要只保留给客户端展示的最终画面。
    ConfigVersion int32
    RNGVersion    string
    Seed          uint64
}
```

优势是职责清晰：SpinEngine 只负责计算；Settlement 只负责提交事实；Actor 只负责排序和内存提交。

## 7. 幂等：先查已完成操作，才允许跑随机数

在 `slots.proto` 给 `Spin` 增加：

```proto
string request_id = 10; // 客户端生成 UUID；重试必须原样复用
```

这是关键语义：

```text
request_id 相同
  -> 不再计算随机数
  -> 不再扣金币
  -> 不再投递活动奖励
  -> 原样返回第一次持久化的 SpinResponse
```

下面是一个职责较小、方便单测的 PG Repository。项目已经由 `internal/component/pg_gorm` 初始化 GORM，并通过 `internal/component/db.GetDB()` 暴露 `*gorm.DB`，所以 Repository 继续使用 GORM，不再引入另一套 `database/sql` 访问风格。

```go
package settlement

import (
    "context"
    "errors"
    "fmt"
    "strconv"
    "time"

    "github.com/jackc/pgconn"
    "github.com/google/uuid"
    gameModel "github.com/cherry-game/examples/demo_cluster/internal/model"
    "gorm.io/gorm"
)

// Repository 是资产结算对 PostgreSQL 的访问边界。
//
// 它只依赖根 *gorm.DB，而不保存某一次事务 tx：
// - 根 *gorm.DB 包装了并发安全的连接池，可作为 Game 组件/Service 的长生命周期依赖；
// - Transaction 回调里的 tx 只能属于一次命令；
// - 把 tx 存入 Repository 字段会让多个玩家 Actor 错误共享事务。
type Repository struct {
    db *gorm.DB
}

func NewRepository(db *gorm.DB) (*Repository, error) {
    if db == nil {
        return nil, errors.New("settlement repository: nil db")
    }
    return &Repository{db: db}, nil
}

// 以下错误由上层 Actor 映射为业务错误码；
// 不要用字符串比较 PostgreSQL 错误信息。
var (
    ErrInvalidCommand  = errors.New("invalid settlement command")
    ErrInsufficientGold = errors.New("insufficient gold")
    ErrInProgress      = errors.New("operation is processing")
)

// isUniqueViolation 只负责识别 PG 唯一索引冲突（SQLSTATE 23505）。
// 例如 (user_id, operation_type, request_id) 冲突，通常代表客户端重试。
func isUniqueViolation(err error) bool {
    var pgErr *pgconn.PgError
    return errors.As(err, &pgErr) && pgErr.Code == "23505"
}

// loadExistingSpin 是唯一冲突后的统一处理：
// 已完成则回放；处理中则交给客户端短暂重试或轮询。
func (r *Repository) loadExistingSpin(
    ctx context.Context, userID int64, requestID string,
) (SettleSpinResult, error) {
    op, err := r.FindSpinOperation(ctx, userID, requestID)
    if err != nil {
        return SettleSpinResult{}, err
    }
    if op == nil {
        // 唯一冲突后理论上必定能查到；查不到说明事务可见性或数据异常，不能继续扣款。
        return SettleSpinResult{}, fmt.Errorf("duplicate spin operation not found: user=%d request=%s", userID, requestID)
    }
    if op.Status != "COMPLETED" {
        return SettleSpinResult{}, ErrInProgress
    }
    return SettleSpinResult{
        Replay:   true,
        Response: op.Response,
    }, nil
}

type SpinOperation struct {
    OperationID string
    Status string
    Response []byte
}

// 以下三个 Model 对应手动维护的资产表，不应放入 gorm/gen 自动生成的 SlotsUser 文件。
// 资产表的字段与索引属于结算领域，需要由 migration 严格控制。
type AssetOperation struct {
    OperationID     string     `gorm:"column:operation_id;primaryKey"`
    UserID          int64      `gorm:"column:user_id"`
    OperationType   string     `gorm:"column:operation_type"`
    RequestID       string     `gorm:"column:request_id"`
    Status          string     `gorm:"column:status"`
    ResponsePayload []byte     `gorm:"column:response_payload"`
    CompletedAt     *time.Time `gorm:"column:completed_at"`
}
func (AssetOperation) TableName() string { return "asset_operation" }

type AssetLedger struct {
    LedgerID     int64  `gorm:"column:ledger_id;primaryKey"`
    OperationID  string `gorm:"column:operation_id"`
    UserID       int64  `gorm:"column:user_id"`
    AssetKind    string `gorm:"column:asset_kind"`
    Delta        int64  `gorm:"column:delta"`
    BalanceAfter int64  `gorm:"column:balance_after"`
    Reason       string `gorm:"column:reason"`
    SourceType   string `gorm:"column:source_type"`
    SourceID     string `gorm:"column:source_id"`
}
func (AssetLedger) TableName() string { return "asset_ledger" }

type OutboxEvent struct {
    EventID       string `gorm:"column:event_id;primaryKey"`
    AggregateType string `gorm:"column:aggregate_type"`
    AggregateID   string `gorm:"column:aggregate_id"`
    EventType     string `gorm:"column:event_type"`
    // Payload 是已经 json.Marshal 完成的领域事件；列类型为 jsonb。
    Payload       []byte `gorm:"column:payload;type:jsonb"`
    Status        string `gorm:"column:status"`
}
func (OutboxEvent) TableName() string { return "domain_outbox" }

func (r *Repository) FindSpinOperation(
    ctx context.Context, userID int64, requestID string,
) (*SpinOperation, error) {
    var op SpinOperation
    err := r.db.WithContext(ctx).
        Table("asset_operation").
        Select("operation_id, status, response_payload").
        Where("user_id = ? AND operation_type = ? AND request_id = ?",
            userID, "spin", requestID).
        Take(&op).Error
    if errors.Is(err, gorm.ErrRecordNotFound) {
        return nil, nil
    }
    if err != nil {
        return nil, err
    }
    return &op, nil
}
```

Actor 中的使用：

```go
op, err := repo.FindSpinOperation(ctx, userID, req.GetRequestId())
if err != nil {
    return internalError(err)
}
if op != nil && op.Status == "COMPLETED" {
    response := new(pb.SpinResponse)
    if err := proto.Unmarshal(op.Response, response); err != nil {
        return internalError(err) // 数据损坏必须告警
    }
    r.Response(session, response)
    return nil
}
```

不要仅靠 Actor 邮箱来做幂等。Actor 对在线状态有效；断线重连、节点重启、支付回调都可能绕过同一个内存实例。数据库唯一约束才是最终保护。

## 8. 单个 PG 事务：余额、账本、操作结果、Outbox 一起提交

下面给出事务的核心代码。为了让账本的 `balance_after` 精确表现中间状态，先扣款，再加奖，而不是只做 `win - bet` 的净额更新。

```go
type SettleSpinCommand struct {
    OperationID string
    UserID      int64
    RequestID   string
    Bet         int64
    Win         int64
    Response    []byte
    OutboxJSON  []byte
}

type SettleSpinResult struct {
    Balance int64
    Replay  bool
    Response []byte
}

func (r *Repository) SettleSpin(
    ctx context.Context, cmd SettleSpinCommand,
) (SettleSpinResult, error) {
    if cmd.UserID <= 0 || cmd.RequestID == "" || cmd.Bet <= 0 || cmd.Win < 0 {
        return SettleSpinResult{}, ErrInvalidCommand
    }

    var result SettleSpinResult
    err := r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
        // 1) 尽早占据业务唯一键。并发重试时只有一个请求可插入。
        //
        // 注意：这一行与后面的余额、账本、Outbox 在同一个 PG 事务中。
        // 所以别的连接在 commit 前完全看不到这条记录；commit 后它和所有
        // 资产事实同时可见。这里直接写 COMPLETED 是正确的，不能被理解为
        // "金币尚未提交就已完成"。
        //
        // 把唯一键插入放在事务开头的目的，是让并发重复请求尽早因唯一约束失败，
        // 避免它继续执行扣款、写账本等无效工作。
        completedAt := time.Now()
        op := AssetOperation{
            OperationID: cmd.OperationID, UserID: cmd.UserID,
            OperationType: "spin", RequestID: cmd.RequestID, Status: "COMPLETED",
            ResponsePayload: cmd.Response, CompletedAt: &completedAt,
        }
        if err := tx.Create(&op).Error; err != nil {
            return err
        }

        // 2) 原子条件扣款。不能先 SELECT 余额再 UPDATE，后者会产生竞态。
        debit := tx.Model(&gameModel.SlotsUser{}).
            Where("user_id = ? AND money >= ?", cmd.UserID, cmd.Bet).
            UpdateColumn("money", gorm.Expr("money - ?", cmd.Bet))
        if debit.Error != nil {
            return debit.Error
        }
        if debit.RowsAffected != 1 {
            return ErrInsufficientGold
        }

        var user gameModel.SlotsUser
        if err := tx.Select("user_id, money").
            Where("user_id = ?", cmd.UserID).Take(&user).Error; err != nil {
            return err
        }
        afterDebit := user.Money

    // 3) 下注账本。账本是 append-only，不允许 UPDATE/DELETE。
        if err := tx.Create(&AssetLedger{
            OperationID: cmd.OperationID, UserID: cmd.UserID, AssetKind: "core.gold",
            Delta: -cmd.Bet, BalanceAfter: afterDebit, Reason: "spin_bet",
            SourceType: "spin", SourceID: cmd.OperationID,
        }).Error; err != nil {
            return err
        }

        finalBalance := afterDebit
        if cmd.Win > 0 {
        // 4) 中奖入账。这里无需余额条件，仍必须在同一事务中。
            credit := tx.Model(&gameModel.SlotsUser{}).
                Where("user_id = ?", cmd.UserID).
                UpdateColumn("money", gorm.Expr("money + ?", cmd.Win))
            if credit.Error != nil {
                return credit.Error
            }
            if credit.RowsAffected != 1 {
                return gorm.ErrRecordNotFound
            }
            finalBalance += cmd.Win

            if err := tx.Create(&AssetLedger{
                OperationID: cmd.OperationID, UserID: cmd.UserID, AssetKind: "core.gold",
                Delta: cmd.Win, BalanceAfter: finalBalance, Reason: "spin_win",
                SourceType: "spin", SourceID: cmd.OperationID,
            }).Error; err != nil {
                return err
            }
        }

    // 5) Outbox 先于 Commit 写入。不能先 Commit 再 Publish NATS。
        eventID := uuid.NewString()
        if err := tx.Create(&OutboxEvent{
            EventID: eventID, AggregateType: "user",
            AggregateID: strconv.FormatInt(cmd.UserID, 10),
            EventType: "game.spin.completed.v1", Payload: cmd.OutboxJSON, Status: "PENDING",
        }).Error; err != nil {
            return err
        }

        result = SettleSpinResult{Balance: finalBalance, Response: cmd.Response}
        return nil
    })
    if isUniqueViolation(err) {
        // GORM Transaction 回调返回后已经回滚，再用根 DB 查询旧结果。
        return r.loadExistingSpin(ctx, cmd.UserID, cmd.RequestID)
    }
    if err != nil {
        return SettleSpinResult{}, err
    }
    return result, nil
}
```

### 8.1 关于死锁、唯一冲突与 context

- `context.Context` 必须从 Actor handler 一路传到 GORM 的 `WithContext(ctx)`；不要在 Repository 内替换成 `context.Background()`。否则连接断开或服务超时后 SQL 还会继续跑。
- PostgreSQL 在热点玩家上可能报 `40P01 deadlock_detected` 或 `40001 serialization_failure`。对**同一个 operation_id**做有限次数退避重试是安全的；不能重试时生成新 operation_id。
- 唯一冲突是正常并发场景，不应只记录 error。应查询已有操作：完成则回放，处理中则返回 `IN_PROGRESS` 或短暂等待。
- 余额行的写入不能依赖 Actor 串行，因为充值回调、GM、跨节点命令可能不经过该 Actor。SQL 的条件更新才是最终余额保护。

## 9. Actor handler 的正确提交顺序

下面是简化后的 `ActorRoom.Spin` 结构。它展示顺序，省略现有的协议错误细节。

```go
func (r *ActorRoom) Spin(ctx context.Context, session *cproto.Session, req *pb.Spin) {
    if req.GetRequestId() == "" || req.GetCurCost() <= 0 {
        r.ResponseCode(session, code.InvalidRequest)
        return
    }

    userID := session.Uid

    // A. 首先回放，不能在这里之后重新跑 RNG。
    old, err := r.settlement.FindSpinOperation(ctx, userID, req.GetRequestId())
    if err != nil {
        r.ResponseCode(session, code.AssetSettleFailed)
        return
    }
    if old != nil && old.Status == "COMPLETED" {
        response := new(pb.SpinResponse)
        if err := proto.Unmarshal(old.Response, response); err != nil {
            r.ResponseCode(session, code.AssetSettleFailed)
            return
        }
        r.Response(session, response)
        return
    }

    // B. 读取 Actor 私有状态，构造副本。
    current := r.roomDataManager.GetData(ctx, int32(userID), req.GetId())
    if current == nil {
        r.ResponseCode(session, code.NoRoomPlayerData)
        return
    }
    next := current.Clone()

    // C. 计算结果，计算阶段不提交任何真实状态。
    result, err := r.spinEngine.Execute(SpinInput{
        RequestID: req.GetRequestId(),
        RoomID: req.GetId(),
        Bet: req.GetCurCost(),
        State: next,
    })
    if err != nil {
        r.ResponseCode(session, code.GetRulstInfoError)
        return
    }

    response := buildSpinResponse(result)
    responseBytes, err := proto.Marshal(response)
    if err != nil {
        r.ResponseCode(session, code.AssetSettleFailed)
        return
    }

    eventBytes, err := json.Marshal(buildSpinCompletedEvent(userID, result, next))
    if err != nil {
        r.ResponseCode(session, code.AssetSettleFailed)
        return
    }

    // D. PG 成功是唯一可提交点。
    settled, err := r.settlement.SettleSpin(ctx, SettleSpinCommand{
        OperationID: uuid.NewString(),
        UserID: userID,
        RequestID: req.GetRequestId(),
        Bet: result.Bet,
        Win: result.Win,
        Response: responseBytes,
        OutboxJSON: eventBytes,
    })
    if errors.Is(err, ErrInsufficientGold) {
        r.ResponseCode(session, code.NotEnoughMoney)
        return
    }
    if err != nil {
        r.ResponseCode(session, code.AssetSettleFailed)
        return
    }

    // E. PG 已完成，才能改变会话内真实状态。
    r.roomDataManager.ReplaceData(req.GetId(), next)

    // F. 快照入队失败只告警；Outbox 事件仍可恢复 DDB。
    if ok := r.roomDataManager.SaveSnapshot(ctx, req.GetId()); !ok {
        clog.Warnf("room snapshot enqueue failed user=%d room=%d", userID, req.GetId())
    }

    // G. 用事务返回的余额填充响应，不能继续相信旧 Actor 缓存。
    response.SpinUserInfo = buildSpinUserInfo(settled.Balance)
    r.Response(session, response)
}
```

这里的关键设计模式是 **Plan then Commit（先计划，后提交）**。计算会产生 `next`，只有持久化提交成功，才把它变为真实 Actor 状态。

## 10. Outbox：为什么不能在事务里直接 Publish NATS

错误时序一：

```text
PG Commit 成功 -> Game 崩溃 -> 尚未 Publish NATS
结果：金币变了，活动和 DDB 永远不知道。
```

错误时序二：

```text
Publish NATS 成功 -> PG Commit 失败
结果：活动拿到一个从未真正发生的 Spin。
```

正确时序：

```text
同一个 PG Transaction:
  金币余额 + 账本 + asset_operation + domain_outbox
  -> COMMIT

独立 Relay 持续扫描 domain_outbox
  -> 发布 JetStream
  -> 标记 PUBLISHED
```

Relay 发布时以 `event_id` 作为 JetStream 去重 ID：

```go
func (r *Relay) publishOne(ctx context.Context, e OutboxEvent) error {
    _, err := r.js.Publish(
        e.EventType,
        e.Payload,
        nats.MsgId(e.EventID), // Relay 崩溃后重发，同 id 不新增一条流消息
    )
    if err != nil {
        return err
    }

    return r.db.WithContext(ctx).Model(&OutboxEvent{}).
        Where("event_id = ? AND status = ?", e.EventID, "PENDING").
        Updates(map[string]any{
            "status": "PUBLISHED",
            "published_at": time.Now(),
        }).Error
}
```

注意：JetStream 的 `MsgId` 是窗口去重，消费者仍必须按 `event_id` 做业务幂等。分布式系统的正确目标是 at-least-once transport + idempotent processing，而不是幻想网络层真正 exactly-once。

### 10.1 当前缺失的四块实现，以及它们如何组成闭环

目前文档中的 `SettleSpin` 只覆盖了 **Slots 本次下注与本次中奖 Gold**：

```text
Spin bet / win
  -> PG 同步事务
  -> 余额、账本、Spin 操作、Outbox 一起提交
```

它还没有自动拥有以下能力，必须分别补上：

1. 通用的活动资产变更命令；
2. 持续扫描 PG Outbox 并发布 JetStream 的 Relay；
3. 订阅 `game.spin.completed.v1` 的活动消费者；
4. 活动成功更新 DDB 后对 PG 事件处理状态的确认。

第四点尤其重要。**不应要求 Spin 的 PG 事务等待 DDB 成功**；PG 与 DDB 没有共同事务，强行做同步双写会出现部分成功且拖慢 Spin。正确模型是“PG 先记录可恢复的待办事实，活动消费者最终把待办推进到完成”。

完整状态机：

```text
                 同一 PG 事务
Spin ------> asset_operation = COMPLETED
              asset_ledger   = 已记账
              domain_outbox  = PENDING
                         |
                         | Relay 发布 JetStream
                         v
              domain_outbox  = PUBLISHED
                         |
                         | Activity Consumer 收到 eventId
                         v
              DDB 活动状态 + 活动 inbox 幂等记录 原子提交
                         |
                         | Consumer ACK，写 PG consumer checkpoint
                         v
              activity_inbox = APPLIED
```

发生故障时：

```text
PG 已提交、Relay 未发布
  -> domain_outbox 仍为 PENDING；下次扫描继续发布。

JetStream 已发布、Relay 未标 PUBLISHED
  -> Relay 重发同 eventId；消费者幂等，因此无副作用。

活动 Worker 已收到、DDB 未成功
  -> 不 ACK；JetStream 重投。

DDB 已成功、Worker 在 ACK 前崩溃
  -> JetStream 重投；DDB inbox eventId 已存在，识别为重复后 ACK。

DDB 已成功、PG checkpoint 未成功
  -> 重投时 DDB 幂等命中；补写 checkpoint 后 ACK。
```

所以“PG 成功后如何保证活动 DDB 成功”的准确回答是：

> 不做跨库同步原子提交；通过永久保存的 Outbox、持久消息流、消费者幂等 Inbox、无限/受控重试和可观测的未完成状态，保证活动 DDB **最终必达且不会重复应用**。

若活动尚未完成，Spin 依然已经完成。客户端查询活动时允许看到稍旧的进度；这就是此处明确选择的最终一致性。

### 10.2 Outbox Relay：真正的扫描与发布代码

Relay 不属于某个玩家 Actor。它是 Game/独立 worker 的后台组件；它只处理 PG 中已经提交的不可变 Outbox 记录，不修改任何玩家 Actor 内存。

先为 Outbox 增加租约字段，避免多个 Relay 实例同时无限重复扫描：

```sql
ALTER TABLE domain_outbox
    ADD COLUMN IF NOT EXISTS locked_until TIMESTAMPTZ,
    ADD COLUMN IF NOT EXISTS locked_by TEXT,
    ADD COLUMN IF NOT EXISTS last_error TEXT;

CREATE INDEX IF NOT EXISTS idx_domain_outbox_dispatch
    ON domain_outbox(status, locked_until, created_at);
```

下面的 PostgreSQL 查询使用 `FOR UPDATE SKIP LOCKED`。多台 Relay 可安全并行；每台只领到自己的一批事件。

```go
package outbox

type Event struct {
    EventID   string
    EventType string
    Payload   []byte
}

type Relay struct {
    db       *gorm.DB
    js       nats.JetStreamContext
    workerID string
}

func (r *Relay) Run(ctx context.Context) {
    ticker := time.NewTicker(100 * time.Millisecond)
    defer ticker.Stop()

    for {
        select {
        case <-ctx.Done():
            return
        case <-ticker.C:
            if err := r.dispatchBatch(ctx, 100); err != nil {
                slog.Error("outbox dispatch failed", "err", err)
            }
        }
    }
}

func (r *Relay) dispatchBatch(ctx context.Context, batchSize int) error {
    events, err := r.claim(ctx, batchSize)
    if err != nil {
        return err
    }

    for _, e := range events {
        // Nats-Msg-Id 让 Relay 的重复 Publish 在 JetStream 去重窗口内不新增消息。
        _, err := r.js.Publish(e.EventType, e.Payload, nats.MsgId(e.EventID))
        if err != nil {
            _ = r.releaseWithError(ctx, e.EventID, err)
            continue
        }
        if err := r.markPublished(ctx, e.EventID); err != nil {
            // 此处失败也不能重新执行业务；eventId 会保护后续重发。
            slog.Error("outbox publish mark failed", "event_id", e.EventID, "err", err)
        }
    }
    return nil
}

func (r *Relay) claim(ctx context.Context, limit int) ([]Event, error) {
    events := make([]Event, 0, limit)
    err := r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
        // GORM 仍可执行 PostgreSQL 特有的领取 SQL；Scan 会把 RETURNING 结果映射到 Event。
        return tx.Raw(`
            WITH claimed AS (
                SELECT event_id
                FROM domain_outbox
                WHERE status = 'PENDING'
                  AND (locked_until IS NULL OR locked_until < now())
                ORDER BY created_at
                FOR UPDATE SKIP LOCKED
                LIMIT ?
            )
            UPDATE domain_outbox o
            SET locked_by = ?, locked_until = now() + interval '30 seconds'
            FROM claimed
            WHERE o.event_id = claimed.event_id
            RETURNING o.event_id, o.event_type, o.payload`, limit, r.workerID).
            Scan(&events).Error
    })
    return events, err
}

func (r *Relay) markPublished(ctx context.Context, eventID string) error {
    return r.db.WithContext(ctx).Model(&OutboxEvent{}).
        Where("event_id = ? AND status = ?", eventID, "PENDING").
        Updates(map[string]any{
            "status": "PUBLISHED", "published_at": time.Now(),
            "locked_by": nil, "locked_until": nil,
        }).Error
}

func (r *Relay) releaseWithError(ctx context.Context, eventID string, cause error) error {
    return r.db.WithContext(ctx).Model(&OutboxEvent{}).
        Where("event_id = ?", eventID).
        Updates(map[string]any{
            "retry_count": gorm.Expr("retry_count + 1"),
            "last_error": cause.Error(), "locked_by": nil, "locked_until": nil,
        }).Error
}
```

Go 注意事项：不要在领取 Outbox 的 SQL transaction 中调用 `js.Publish`。领任务的事务应很短，只负责锁定/租约；网络发布在事务外完成，否则 NATS 延迟会长期占用 PG 行锁。

### 10.3 通用活动资产变动：先定义命令，不让每个活动直接改 JSON

活动资产不应复用 `SettleSpin`。`SettleSpin` 的职责是 Slots 核心 Gold 结算；活动层需要一个通用、可扩展的命令。

```go
package activityasset

type AssetKind string

type Change struct {
    Kind  AssetKind `json:"kind"`
    Delta int64     `json:"delta"` // 正加负减；禁止零值
}

type Command struct {
    // OperationID 是本次业务操作的全局幂等键。
    // Spin 触发奖励时使用：activity:<activityID>:<spinEventID>:<rewardCode>。
    OperationID string `json:"operationId"`
    EventID     string `json:"eventId"`
    UserID      int64  `json:"userId"`
    ActivityID  string `json:"activityId"`
    Reason      string `json:"reason"`
    Changes     []Change `json:"changes"`
}

func (c Command) Validate() error {
    if c.OperationID == "" || c.EventID == "" || c.UserID <= 0 || c.ActivityID == "" {
        return errors.New("missing activity asset identity")
    }
    if len(c.Changes) == 0 {
        return errors.New("empty activity asset changes")
    }
    for _, one := range c.Changes {
        if one.Kind == "" || one.Delta == 0 {
            return errors.New("invalid activity asset change")
        }
        if !strings.HasPrefix(string(one.Kind), "event."+c.ActivityID+".") {
            return errors.New("asset does not belong to activity")
        }
    }
    return nil
}
```

活动 handler 只根据 Spin 事件计算命令，不直接写 DDB：

```go
func (h *SummerHandler) OnSpin(e SpinCompletedEvent) ([]activityasset.Command, error) {
    if !h.isEligible(e) {
        return nil, nil
    }

    return []activityasset.Command{{
        OperationID: fmt.Sprintf("activity:summer_2026:%s:spin_coin", e.EventID),
        EventID: e.EventID,
        UserID: e.UserID,
        ActivityID: "summer_2026",
        Reason: "spin_completed",
        Changes: []activityasset.Change{{
            Kind: "event.summer_2026.coin",
            Delta: 2,
        }},
    }}, nil
}
```

这样 GiftType `EventCoin=6` 只是策划输入；解析后变成明确的 `event.summer_2026.coin`。新增活动只新增 handler 与 schema，不改 Spin。

### 10.4 活动需要额外奖励 Gold 时

活动 DDB Worker **不能直接写 `SlotsUser.Money`**。

```text
活动发现奖励 core.gold
  -> 生成 GrantCoreAsset Command
  -> 资产 PG 模块事务：operation 去重 + asset_ledger + slots_user.money + outbox
  -> 返回/投递 player.asset.changed.v1
```

这里与 `SettleSpin` 共用的是底层的 `asset_operation + asset_ledger` 模式，不是让活动调用 `SettleSpin`。可抽取通用 `GrantCoreAsset`：

```go
type GrantCoreAssetCommand struct {
    OperationID string
    UserID      int64
    Kind        string // 仅允许 core.gold / core.diamond 等 PG 权威资产
    Amount      int64  // 必须大于 0
    Reason      string
    SourceID    string // 例如 activity:<id>:<eventId>
}

// 实现结构与 SettleSpin 相同：
// 1. asset_operation 的 operation_id 唯一；
// 2. GORM 原子更新 slots_user.money = money + amount；
// 3. INSERT asset_ledger；
// 4. INSERT domain_outbox；
// 5. COMMIT。
```

这笔 Gold 允许异步到达，意味着 SpinResponse 可能先显示 Slots 自身中奖后的余额，随后通过推送或下一次拉取余额看到活动赠送金币。这是低延迟与跨库最终一致性的正常语义。

### 10.5 活动 Consumer：DDB Inbox、状态更新与 ACK

每个活动可有独立 Durable Consumer，例如：

```text
stream: GAME_EVENTS
subject: game.spin.completed.v1
durable: activity-summer-2026-v1
```

消费者的正确顺序是：解析 -> 计算奖励命令 -> DDB 条件/事务写 -> ACK。不要先 ACK 再写 DDB。

```go
func (c *Consumer) Handle(ctx context.Context, msg *nats.Msg) {
    var event SpinCompletedEvent
    if err := json.Unmarshal(msg.Data, &event); err != nil {
        // 不可解析的毒消息：记录到 DLQ 后 ACK，避免无限阻塞整条消费流。
        c.toDLQ(ctx, msg, err)
        _ = msg.Ack()
        return
    }

    commands, err := c.handler.OnSpin(event)
    if err != nil {
        // 规则错误/依赖故障，暂不 ACK，等待重投；需配置 MaxDeliver + DLQ。
        c.log.Error("activity rule failed", "event_id", event.EventID, "err", err)
        return
    }

    for _, cmd := range commands {
        if err := c.store.Apply(ctx, cmd); err != nil {
            // 网络故障或 DDB 节流：不 ACK。
            return
        }
    }

    // 只有所有命令都已幂等成功才确认 JetStream。
    _ = msg.Ack()
}
```

`Apply` 的责任是一次 DynamoDB `TransactWriteItems` 中同时完成：

1. 检查并创建 `OP#<operationId>` Inbox 项；
2. 更新 `ACTIVITY#<activityId>` 的 JSON 状态与 revision；
3. 可选写活动资产日志。

```go
func (s *Store) Apply(ctx context.Context, cmd activityasset.Command) error {
    if err := cmd.Validate(); err != nil {
        return err
    }

    // 实际项目建议将 document 读出、按 schema 迁移、计算 next JSON，
    // 再带 expected revision 写回。这里省略 AttributeValue 转换细节。
    _, err := s.ddb.TransactWriteItems(ctx, &dynamodb.TransactWriteItemsInput{
        TransactItems: []types.TransactWriteItem{
            {Put: &types.Put{
                TableName: aws.String(s.table),
                Item: map[string]types.AttributeValue{
                    "pk":  &types.AttributeValueMemberS{Value: fmt.Sprintf("USER#%d", cmd.UserID)},
                    "sk":  &types.AttributeValueMemberS{Value: "OP#" + cmd.OperationID},
                    "ttl": &types.AttributeValueMemberN{Value: strconv.FormatInt(time.Now().Add(90*24*time.Hour).Unix(), 10)},
                },
                ConditionExpression: aws.String("attribute_not_exists(pk)"),
            }},
            {Update: &types.Update{
                TableName: aws.String(s.table),
                Key: map[string]types.AttributeValue{
                    "pk": &types.AttributeValueMemberS{Value: fmt.Sprintf("USER#%d", cmd.UserID)},
                    "sk": &types.AttributeValueMemberS{Value: "ACTIVITY#" + cmd.ActivityID},
                },
                // 真实代码不要把多个活动字段铺成通用列；
                // 使用 data JSON/document，handler 负责理解自己的 schema。
                UpdateExpression: aws.String("SET #data = :next, #revision = if_not_exists(#revision, :zero) + :one, #updatedAt = :now"),
                ExpressionAttributeNames: map[string]string{
                    "#data": "data", "#revision": "revision", "#updatedAt": "updatedAt",
                },
                ExpressionAttributeValues: map[string]types.AttributeValue{
                    ":next": &types.AttributeValueMemberS{Value: string(nextJSON)},
                    ":zero": &types.AttributeValueMemberN{Value: "0"},
                    ":one": &types.AttributeValueMemberN{Value: "1"},
                    ":now": &types.AttributeValueMemberN{Value: strconv.FormatInt(time.Now().Unix(), 10)},
                },
            }},
        },
    })

    if isTransactionCanceledBecauseDuplicateOperation(err) {
        // OP 已存在表示先前成功；按成功处理，让 Consumer ACK。
        return nil
    }
    return err
}
```

上例的 `nextJSON` 必须来自当前活动 schema 的纯函数计算。若多个事件可能并发改同一活动文档，则要将 `revision` 加到 `ConditionExpression` 并发生冲突时重新读取、重算、重试；不能盲目 Last Write Wins。

## 11. DynamoDB：房间快照与活动状态都需要版本和幂等键

建议不要一活动一张 DynamoDB 物理表。更适合的是一个逻辑活动状态表：

```text
PK                 SK
USER#10001         ROOM#86001
USER#10001         ACTIVITY#summer_2026
USER#10001         ACTIVITY#witch_2026
USER#10001         EVENT#<eventId>
```

每个活动仍可拥有独立 Go 模块、JSON schema、奖励逻辑和迁移函数；物理表不必随着活动数量无限膨胀。

### 11.1 状态文档建议字段

```json
{
  "schemaVersion": 3,
  "revision": 108,
  "lastEventId": "...",
  "data": {
    "coin": 120,
    "dice": 4,
    "buffs": { "speedUntil": 1780000000 }
  }
}
```

对于 Room 快照，使用 Spin 顺序号或 `RoomDataInfo.Version`；DDB 更新要求新版本大于旧版本。这样重复或乱序事件都不会覆盖新状态。

```go
// 伪代码：写快照时增加条件。
UpdateExpression: "SET #state = :state, #version = :next, #last = :eventID"
ConditionExpression: "attribute_not_exists(#version) OR #version < :next"
```

条件失败不必都当故障：

- 已处理同 `eventId`：重复消息，ACK；
- 存储版本已更大：旧消息乱序，ACK；
- 条件以外的网络故障：不 ACK，等待 JetStream 重投。

### 11.2 活动代币的两种语义

**Spin 触发的奖励活动币**：可以异步。活动 Worker 消费 `game.spin.completed.v1` 后增加活动币。

**玩家立刻消费的活动币**：消费接口不能只依赖异步。它必须在活动 Actor/Worker 中以 DDB 条件更新原子扣除：

```text
SET coin = coin - :cost, revision = revision + :one
WHERE coin >= :cost AND revision = :expected
```

并持久化 `operation_id`。否则双击抽奖或请求重试会重复消费。

## 12. GiftType：保留协议枚举，内部换成可扩展 AssetKind

现有 `GiftType` 数字适合策划表和客户端协议，但不要让业务代码出现大量：

```go
if giftType == 1 {
    user.Money += reward
}
```

把它集中映射为领域资产：

```go
type AssetKind string

const (
    AssetGold        AssetKind = "core.gold"
    AssetDiamond     AssetKind = "core.diamond"
    AssetScratchCard AssetKind = "core.scratch_card"
)

func ResolveGift(activityID string, giftType int32) (AssetKind, bool) {
    switch giftType {
    case 1: // Gold
        return AssetGold, true
    case 2: // ShaveCard
        return AssetScratchCard, true
    case 5: // Diamond
        return AssetDiamond, true
    case 6: // EventCoin
        if activityID == "" {
            return "", false
        }
        return AssetKind("event." + activityID + ".coin"), true
    default:
        return "", false
    }
}
```

原则：

```text
core.gold / core.diamond
  -> PG 账本与同步结算

event.<activity>.<item>
  -> 活动模块 / DDB 文档

buff.<name>
  -> 活动/玩家状态，带过期时间，而不是永久累加数值
```

新增活动只需新增 `event.summer_2026.coin` 等命名空间资产以及对应 handler，不需要修改 Spin 的核心结算代码。

## 13. Go 代码中最容易踩的坑

### 13.1 不要在 Actor handler 中 `go func()` 改 Actor 字段

```go
// 错误：绕过 mailbox，直接与 Actor 的 Spin handler 并发写状态。
go func() {
    r.roomDataInfo.Version++
}()
```

后台 goroutine 可以处理不可变事件、数据库 IO、快照；要修改 Actor 状态，必须把消息重新投递给这个 Actor。

### 13.2 避免在同一 Actor 内 CallWait 自己

一个 Actor 处理 Spin 时，若 `CallWait` 同一个 Actor 的另一个方法，它会等待自己继续消费 mailbox，形成逻辑死锁或框架错误。当前 `ActorRoom.Spin` 调 `GetUserInfo` 会跨到 `player.<uid>`；但由于它实际运行在 player Actor 的 Local handler 中，应检查这个调用链是否会形成同 Actor 同步调用。更好的设计是将 `playerData` 作为明确输入传给 Slots 业务对象，避免不必要的自 RPC。

### 13.3 不要在 SQL 事务内做网络 RPC

```text
BEGIN -> 扣金币 -> 调活动服务 -> COMMIT
```

活动服务慢或不可用时会长期持有数据库锁，直接拖垮 Spin p99。事务内只做本 PG 数据库操作；跨进程工作用 Outbox。

### 13.4 不要吞掉异步失败

`SubmitTask` 返回 false、DDB 条件失败、Outbox 发布持续失败，都必须有指标和报警。

```text
outbox pending age
outbox retry count
jetstream consumer redelivery
ddb conditional conflict count
room snapshot enqueue failed
asset ledger / balance reconciliation delta
```

### 13.5 枚举和 protobuf 字段号只能追加

`GiftType` 以及 `RewardAttribute` 已有大量历史字段。字段号不能复用，也不要改变旧字段含义。新增字段只追加；废弃字段保留 reserved，保证旧客户端/旧消息可兼容。

### 13.6 对缓存余额保持怀疑

Actor 内存 `playerData.Money` 可以加速展示，但不能作为最后余额校验。同步结算返回的 PG 余额是权威值；将它回写 Actor 内存只是缓存刷新。

## 14. 建议的演进顺序

1. 修正 `RoomDataManager` 初始化，并明确它只被所属玩家 Actor 使用。
2. 给 Spin 增加 `request_id`，接入 `asset_operation` 的回放查询。
3. 确认 `SlotsUser.Money int64` 对应 PG 整数列，建立账本和 Outbox 表；先只改普通 Spin。
4. 把 SpinEngine 改为输出 `SpinPlan`，确保 PG 失败不污染真实 RoomDataInfo。
5. 接入 PG 事务；成功后更新 Actor 内存，失败丢弃 `next`。
6. 启用 NATS JetStream，部署 Outbox Relay。普通 NATS Actor RPC 继续用于即时通信，领域事件使用 JetStream。
7. 完成 DDB Room Snapshot Consumer，再把活动 Worker 接在同一个 `spin.completed` 主题上。
8. 将支付回调、GM 补偿、钻石等核心资产也接入相同账本命令。
9. 做对账任务：余额应等于期初余额加账本 delta 之和；发现差异立即报警，不自动悄悄修正。

## 15. 最终验收标准

```text
同 request_id 重发 100 次
  -> 只写一组账本，只返回同一个结果

PG 在 commit 前失败
  -> 不扣钱，不推进 Actor 房间状态

PG commit 后 Game 立刻崩溃
  -> 金币和 Spin 操作存在；Outbox 最终发布；房间状态/活动最终追平

JetStream 重复投递事件
  -> DDB 状态、活动币、奖励不会重复发

活动节点停机一小时
  -> Spin 仍快速返回；恢复后活动消费者补完积压

单玩家连续 Spin
  -> Actor 顺序一致；余额不可能低于零；Room version 单调递增
```

这套设计并不是把 Actor 替换成微服务，而是承认 Actor、PG 事务、DDB 文档与事件流各自擅长不同问题，并让它们只处理自己的边界。
