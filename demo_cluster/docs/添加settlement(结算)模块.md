不直接改工作区。下面按你现在的 `player.<uid>` Actor 执行 Spin 的方式，给出可手动落入项目的第一版代码骨架。

核心是：

```text
同一玩家 Spin：
Actor 串行
  → 用 RoomDataInfo 副本计算结果
  → PG 一个事务：金币 + 账本 + 幂等结果 + Outbox
  → 事务成功，才替换 Actor 内存状态
  → 立刻返回 SpinResponse
  → 后台可靠发布：DDB Room 快照、活动处理
```

`RoomDataInfo` 仍然以 `roomId` 为 key，属于玩家 Actor 私有内存。金币不走异步写后队列。

---

## 1. 先给 Spin 请求增加幂等 ID

修改 `demo_cluster/internal/protocol/slots.proto`：

```proto
message Spin {
  int32 id = 1;
  int64 curBet = 2;
  int64 curMoney = 3;
  int64 curCost = 4;
  int32 reelsVersion = 5;
  string testSeed = 6;
  int32 testReelLevel = 7;
  string token = 8;
  bool isCasinoQuest = 9;

  // 客户端生成 UUID；网络重试必须复用同一个 requestId。
  string requestId = 10;
}
```

重新生成 protobuf 后，Spin 处理入口必须校验：

```go
if req.GetRequestId() == "" {
    r.ResponseCode(session, code.InvalidRequest)
    return
}
```

建议新增状态码：

```go
// demo_cluster/internal/code/code.go
const (
    InvalidRequest      int32 = 316
    AssetSettleFailed   int32 = 317
    SpinInProgress      int32 = 318
)
```

---

## 2. 新增 PG 表

不要直接把 `SlotsUser.Money float64` 当结算余额。先加精确整数列，原字段保留兼容读取，直到迁移完成。

```sql
ALTER TABLE newsz_2024.slots_user
    ADD COLUMN IF NOT EXISTS money_units BIGINT NOT NULL DEFAULT 0;

-- 一次性数据迁移；比例按你们金币是否有小数确定。
UPDATE newsz_2024.slots_user
SET money_units = ROUND(money)::BIGINT
WHERE money_units = 0;
```

新增资产账本、Spin 幂等记录和 Outbox：

```sql
CREATE TABLE IF NOT EXISTS asset_operation (
    operation_id UUID PRIMARY KEY,
    player_id BIGINT NOT NULL,
    operation_type VARCHAR(32) NOT NULL,
    request_id VARCHAR(128) NOT NULL,
    status VARCHAR(16) NOT NULL,
    response_code INT NOT NULL DEFAULT 0,
    response_payload BYTEA,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    completed_at TIMESTAMPTZ,

    UNIQUE (player_id, operation_type, request_id)
);

CREATE TABLE IF NOT EXISTS asset_ledger (
    ledger_id BIGSERIAL PRIMARY KEY,
    operation_id UUID NOT NULL,
    player_id BIGINT NOT NULL,
    asset_kind VARCHAR(128) NOT NULL,
    delta BIGINT NOT NULL,
    balance_after BIGINT NOT NULL,
    reason VARCHAR(64) NOT NULL,
    source_type VARCHAR(64) NOT NULL,
    source_id VARCHAR(128) NOT NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),

    UNIQUE (operation_id, asset_kind, reason)
);

CREATE INDEX IF NOT EXISTS idx_asset_ledger_player_created
ON asset_ledger(player_id, created_at DESC);

CREATE TABLE IF NOT EXISTS domain_outbox (
    event_id UUID PRIMARY KEY,
    aggregate_type VARCHAR(64) NOT NULL,
    aggregate_id VARCHAR(128) NOT NULL,
    event_type VARCHAR(128) NOT NULL,
    payload JSONB NOT NULL,
    status VARCHAR(16) NOT NULL DEFAULT 'PENDING',
    retry_count INT NOT NULL DEFAULT 0,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    published_at TIMESTAMPTZ
);

CREATE INDEX IF NOT EXISTS idx_domain_outbox_pending
ON domain_outbox(status, created_at);
```

这四张表的职责：

| 表 | 用途 |
|---|---|
| `slots_user.money_units` | 当前金币余额，用于快速扣款/读取 |
| `asset_operation` | Spin 请求的幂等记录与可重放响应 |
| `asset_ledger` | 不可变账本，真正用于对账 |
| `domain_outbox` | PG 成功后可靠投递 DDB/活动消息 |

---

## 3. 新增资产结算模块

新增 `demo_cluster/internal/asset/settlement.go`：

```go
package asset

import (
	"context"
	"encoding/json"
	"errors"
	"time"

	"github.com/google/uuid"
	"gorm.io/gorm"

	"github.com/cherry-game/examples/demo_cluster/internal/code"
)

const (
	AssetGold = "core.gold"

	OperationSpin = "spin"

	ReasonSpinBet = "spin_bet"
	ReasonSpinWin = "spin_win"

	OutboxSpinCompleted = "game.spin.completed.v1"
)

var (
	ErrNotEnoughGold = errors.New("not enough gold")
	ErrInProgress    = errors.New("operation in progress")
)

type Service struct {
	db *gorm.DB
}

func NewService(db *gorm.DB) *Service {
	return &Service{db: db}
}

type SettleSpinCommand struct {
	PlayerID  int64
	RequestID string
	RoomID    int32

	Bet int64
	Win int64

	// 已序列化的 SpinResponse；重复请求直接回放它。
	ResponsePayload []byte

	// 用于 DDB 恢复与活动异步处理。
	SpinEvent SpinCompletedEvent
}

type SettleSpinResult struct {
	OperationID string
	Balance     int64
	Replay      bool
	Response    []byte
}

type AssetOperation struct {
	OperationID     string     `gorm:"column:operation_id;primaryKey"`
	PlayerID        int64      `gorm:"column:player_id"`
	OperationType   string     `gorm:"column:operation_type"`
	RequestID       string     `gorm:"column:request_id"`
	Status          string     `gorm:"column:status"`
	ResponseCode    int32      `gorm:"column:response_code"`
	ResponsePayload []byte     `gorm:"column:response_payload"`
	CreatedAt       time.Time  `gorm:"column:created_at"`
	CompletedAt     *time.Time `gorm:"column:completed_at"`
}

func (AssetOperation) TableName() string {
	return "asset_operation"
}

type AssetLedger struct {
	LedgerID     int64     `gorm:"column:ledger_id;primaryKey"`
	OperationID  string    `gorm:"column:operation_id"`
	PlayerID     int64     `gorm:"column:player_id"`
	AssetKind    string    `gorm:"column:asset_kind"`
	Delta        int64     `gorm:"column:delta"`
	BalanceAfter int64     `gorm:"column:balance_after"`
	Reason       string    `gorm:"column:reason"`
	SourceType   string    `gorm:"column:source_type"`
	SourceID     string    `gorm:"column:source_id"`
	CreatedAt    time.Time `gorm:"column:created_at"`
}

func (AssetLedger) TableName() string {
	return "asset_ledger"
}

type OutboxEvent struct {
	EventID       string     `gorm:"column:event_id;primaryKey"`
	AggregateType string     `gorm:"column:aggregate_type"`
	AggregateID   string     `gorm:"column:aggregate_id"`
	EventType     string     `gorm:"column:event_type"`
	Payload       []byte     `gorm:"column:payload"`
	Status        string     `gorm:"column:status"`
	RetryCount    int32      `gorm:"column:retry_count"`
	CreatedAt     time.Time  `gorm:"column:created_at"`
	PublishedAt   *time.Time `gorm:"column:published_at"`
}

func (OutboxEvent) TableName() string {
	return "domain_outbox"
}

type SpinCompletedEvent struct {
	EventID       string          `json:"eventId"`
	OperationID   string          `json:"operationId"`
	PlayerID      int64           `json:"playerId"`
	RoomID        int32           `json:"roomId"`
	RequestID     string          `json:"requestId"`
	Bet           int64           `json:"bet"`
	Win           int64           `json:"win"`
	Balance       int64           `json:"balance"`
	RoomVersion   int             `json:"roomVersion"`
	ConfigVersion int32           `json:"configVersion"`
	State         json.RawMessage `json:"state"`
	Result        json.RawMessage `json:"result"`
	OccurredAt    time.Time       `json:"occurredAt"`
}

// GetCompletedSpin 在生成随机结果前调用。
// 如果是重试请求，直接拿到第一次的响应，绝不能重新跑随机。
func (s *Service) GetCompletedSpin(
	ctx context.Context,
	playerID int64,
	requestID string,
) (*SettleSpinResult, error) {
	var op AssetOperation

	err := s.db.WithContext(ctx).
		Where(
			"player_id = ? AND operation_type = ? AND request_id = ?",
			playerID,
			OperationSpin,
			requestID,
		).
		First(&op).Error

	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	if op.Status != "COMPLETED" {
		return nil, ErrInProgress
	}

	var balance int64
	if err := s.db.WithContext(ctx).
		Table("newsz_2024.slots_user").
		Select("money_units").
		Where("user_id = ?", playerID).
		Scan(&balance).Error; err != nil {
		return nil, err
	}

	return &SettleSpinResult{
		OperationID: op.OperationID,
		Balance:     balance,
		Replay:      true,
		Response:    op.ResponsePayload,
	}, nil
}

// SettleSpin 将扣投注、加中奖、账本、Spin 响应、Outbox 放在同一个 PG 事务。
func (s *Service) SettleSpin(
	ctx context.Context,
	cmd SettleSpinCommand,
) (*SettleSpinResult, error) {
	if cmd.PlayerID <= 0 || cmd.RequestID == "" || cmd.Bet < 0 || cmd.Win < 0 {
		return nil, errors.New("invalid settle spin command")
	}

	operationID := uuid.NewString()
	var result SettleSpinResult

	err := s.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		// 1. 同一 request_id 已处理时，直接重放。
		var exists AssetOperation
		err := tx.
			Where(
				"player_id = ? AND operation_type = ? AND request_id = ?",
				cmd.PlayerID,
				OperationSpin,
				cmd.RequestID,
			).
			First(&exists).Error

		if err == nil {
			if exists.Status == "COMPLETED" {
				result = SettleSpinResult{
					OperationID: exists.OperationID,
					Replay:      true,
					Response:    exists.ResponsePayload,
				}
				return nil
			}
			return ErrInProgress
		}
		if !errors.Is(err, gorm.ErrRecordNotFound) {
			return err
		}

		// 2. 先插入幂等主记录。
		now := time.Now()
		op := AssetOperation{
			OperationID:   operationID,
			PlayerID:      cmd.PlayerID,
			OperationType: OperationSpin,
			RequestID:     cmd.RequestID,
			Status:        "PROCESSING",
			CreatedAt:     now,
		}
		if err := tx.Create(&op).Error; err != nil {
			// 并发插入唯一键冲突时，外层重新查并回放即可。
			return err
		}

		// 3. 条件扣款并同时加中奖。
		// netChange = win - bet，但账本仍分别写 bet / win 两条。
		netChange := cmd.Win - cmd.Bet
		var balance int64

		row := tx.Table("newsz_2024.slots_user").
			Where("user_id = ? AND money_units >= ?", cmd.PlayerID, cmd.Bet).
			Clauses().
			UpdateColumn("money_units", gorm.Expr("money_units + ?", netChange))

		if row.Error != nil {
			return row.Error
		}
		if row.RowsAffected != 1 {
			return ErrNotEnoughGold
		}

		if err := tx.Table("newsz_2024.slots_user").
			Select("money_units").
			Where("user_id = ?", cmd.PlayerID).
			Scan(&balance).Error; err != nil {
			return err
		}

		// 4. 不可变账本。
		// balance_after 对于下注分录记录净结算后的余额；如需严格中间余额，
		// 可改为“先扣、再加”的两个 UPDATE。
		ledgers := []AssetLedger{
			{
				OperationID:  operationID,
				PlayerID:     cmd.PlayerID,
				AssetKind:    AssetGold,
				Delta:        -cmd.Bet,
				BalanceAfter: balance,
				Reason:       ReasonSpinBet,
				SourceType:   OperationSpin,
				SourceID:     operationID,
				CreatedAt:    now,
			},
		}
		if cmd.Win > 0 {
			ledgers = append(ledgers, AssetLedger{
				OperationID:  operationID,
				PlayerID:     cmd.PlayerID,
				AssetKind:    AssetGold,
				Delta:        cmd.Win,
				BalanceAfter: balance,
				Reason:       ReasonSpinWin,
				SourceType:   OperationSpin,
				SourceID:     operationID,
				CreatedAt:    now,
			})
		}
		if err := tx.Create(&ledgers).Error; err != nil {
			return err
		}

		// 5. 写入可靠事件。此时绝不能直接 Publish NATS。
		event := cmd.SpinEvent
		event.EventID = uuid.NewString()
		event.OperationID = operationID
		event.PlayerID = cmd.PlayerID
		event.RequestID = cmd.RequestID
		event.Bet = cmd.Bet
		event.Win = cmd.Win
		event.Balance = balance
		event.OccurredAt = now

		eventPayload, err := json.Marshal(event)
		if err != nil {
			return err
		}
		if err := tx.Create(&OutboxEvent{
			EventID:       event.EventID,
			AggregateType: "player",
			AggregateID:   stringPlayerID(cmd.PlayerID),
			EventType:     OutboxSpinCompleted,
			Payload:       eventPayload,
			Status:        "PENDING",
			CreatedAt:     now,
		}).Error; err != nil {
			return err
		}

		// 6. 事务完成后可重放响应。
		if err := tx.Model(&AssetOperation{}).
			Where("operation_id = ?", operationID).
			Updates(map[string]interface{}{
				"status":           "COMPLETED",
				"response_code":    code.OK,
				"response_payload": cmd.ResponsePayload,
				"completed_at":     now,
			}).Error; err != nil {
			return err
		}

		result = SettleSpinResult{
			OperationID: operationID,
			Balance:     balance,
			Response:    cmd.ResponsePayload,
		}
		return nil
	})

	if errors.Is(err, ErrNotEnoughGold) {
		return nil, ErrNotEnoughGold
	}
	if err != nil {
		return nil, err
	}
	return &result, nil
}

func stringPlayerID(playerID int64) string {
	return uuid.NewSHA1(uuid.NameSpaceOID, []byte(string(rune(playerID)))).String()
}
```

注意：上面最后的 `stringPlayerID` 不应该采用 UUID。手动替换为下面这个正常实现，并补 `strconv` import：

```go
func stringPlayerID(playerID int64) string {
	return strconv.FormatInt(playerID, 10)
}
```

`gorm.Clauses()` 那一行可直接删掉：

```go
row := tx.Table("newsz_2024.slots_user").
    Where("user_id = ? AND money_units >= ?", cmd.PlayerID, cmd.Bet).
    UpdateColumn("money_units", gorm.Expr("money_units + ?", netChange))
```

---

## 4. Spin 计算必须先用状态副本

修改 `Spin` 的核心逻辑时，不能直接把 Actor 内存中的 `roomDataInfo` 传进 Spin 引擎。

```go
// 错误：结算失败时内存状态已经改变。
// spinManager.ReadySPin(..., roomDataInfo, ...)

// 正确：先对 RoomState 副本计算。
nextRoomData := roomDataInfo.Clone()

spinResult, err := spinManager.ReadySPin(
    ctx,
    roomID,
    ruleID,
    false,
    int(curBet),
    roomConfig,
    nextRoomData,
    r.roomDataManager,
)
if err != nil {
    // ...
    return
}
```

`ReadySPin` 内的 `SpinAfter` 修改的是 `nextRoomData`，PG 提交失败时丢弃这个副本即可，原 Actor 内存完全不变。

---

## 5. 从 SpinResult 读取中奖金额

建议增加一个统一方法，不让 `ActorRoom` 了解不同 Slots 结果结构。

新增 `demo_cluster/nodes/game/server/slots/spin_manager/settlement.go`：

```go
package spinmanage

import "github.com/cherry-game/examples/demo_cluster/internal/pb"

// TotalWin 应由所有关卡结果统一汇总。
// 当前 pb.SpinResult 若已经有 TotalWin 字段，直接返回。
// 若中奖数据只存在 WinInfo 中，在这里集中解析。
func TotalWin(result *pb.SpinResult) int64 {
	if result == nil {
		return 0
	}

	var total int64
	for _, win := range result.GetWinInfo() {
		total += int64(win.GetWinMoney())
	}
	return total
}
```

如果你现在 `SpinResult` 没有直接的 `WinInfo` 或 `WinMoney`，就应该在各 `GenResultXX` 生成结果时补一个统一字段，例如：

```proto
message SpinResult {
  // 原有字段...

  int64 totalWin = 100;
}
```

然后：

```go
func TotalWin(result *pb.SpinResult) int64 {
    return result.GetTotalWin()
}
```

这是更好的方式。金币结算不能靠后续再解析展示用的复杂嵌套结果。

---

## 6. 在 ActorRoom.Spin 中接入结算

建议把 `asset.Service` 注入 `ActorRoom`。下面先展示核心 Spin 替换逻辑。

```go
func (r *ActorRoom) Spin(
    ctx context.Context,
    session *cproto.Session,
    req *pb.Spin,
) {
    done := metrics.TrackRequest("game.slots.spin")
    hasError := false
    defer func() { done(hasError) }()

    if req.GetRequestId() == "" {
        hasError = true
        r.ResponseCode(session, code.InvalidRequest)
        return
    }

    roomID := req.GetId()
    ruleID := roomID / 1000
    bet := req.GetCurCost()

    if bet <= 0 {
        hasError = true
        r.ResponseCode(session, code.InvalidRequest)
        return
    }

    traceID := ccontext.GetTraceId(ctx)
    userInfo, errCode := rpcGame.GetUserInfo(r.Actor, session, traceID)
    if code.IsFail(errCode) {
        hasError = true
        r.ResponseCode(session, errCode)
        return
    }

    settlement := r.settlement // 由构造函数注入

    // 1. 最先检查历史请求；命中时不重新计算随机结果。
    replay, err := settlement.GetCompletedSpin(
        ctx,
        int64(userInfo.GetUserId()),
        req.GetRequestId(),
    )
    if err != nil {
        if errors.Is(err, asset.ErrInProgress) {
            hasError = true
            r.ResponseCode(session, code.SpinInProgress)
            return
        }

        hasError = true
        r.ResponseCode(session, code.AssetSettleFailed)
        return
    }

    if replay != nil && replay.Replay {
        response := &pb.SpinResponse{}
        if err := proto.Unmarshal(replay.Response, response); err != nil {
            hasError = true
            r.ResponseCode(session, code.AssetSettleFailed)
            return
        }

        r.Response(session, response)
        return
    }

    // 2. 初步余额检查只用于快速失败。
    // 真正的余额检查在 PG 条件更新中完成。
    if userInfo.GetMoney() < bet {
        hasError = true
        r.ResponseCode(session, code.NotEnoughMoney)
        return
    }

    roomConfig, err := configCacheSlots.GetInstance().GetRoomConfig(roomID)
    if err != nil || roomConfig == nil {
        hasError = true
        r.ResponseCode(session, code.NoRoomConfig)
        return
    }

    roomData := r.roomDataManager.GetData(ctx, userInfo.GetUserId(), roomID)
    if roomData == nil {
        hasError = true
        r.ResponseCode(session, code.NoRoomPlayerData)
        return
    }

    // 3. 用副本计算，不可直接修改 Actor 当前状态。
    nextRoomData := roomData.Clone()
    nextRoomData.Version++
    nextRoomData.MarkDirty()

    spinResult, err := spinManager.ReadySPin(
        ctx,
        roomID,
        ruleID,
        false,
        int(bet),
        roomConfig,
        nextRoomData,
        r.roomDataManager,
    )
    if err != nil {
        hasError = true
        r.ResponseCode(session, code.GetRulstInfoError)
        return
    }

    win := spinManager.TotalWin(spinResult)

    response := &pb.SpinResponse{
        Id:         roomID,
        UserBet:    bet,
        SpinResult: spinResult,
        SpinUserInfo: &pb.SpinUserInfo{
            // 按现有 proto 填充。
        },
    }

    responsePayload, err := proto.Marshal(response)
    if err != nil {
        hasError = true
        r.ResponseCode(session, code.AssetSettleFailed)
        return
    }

    roomStateJSON, err := json.Marshal(nextRoomData)
    if err != nil {
        hasError = true
        r.ResponseCode(session, code.AssetSettleFailed)
        return
    }

    resultJSON, err := json.Marshal(spinResult)
    if err != nil {
        hasError = true
        r.ResponseCode(session, code.AssetSettleFailed)
        return
    }

    // 4. 同步 PG 结算：余额、账本、幂等、Outbox。
    settleResult, err := settlement.SettleSpin(ctx, asset.SettleSpinCommand{
        PlayerID:        int64(userInfo.GetUserId()),
        RequestID:       req.GetRequestId(),
        RoomID:          roomID,
        Bet:             bet,
        Win:             win,
        ResponsePayload: responsePayload,
        SpinEvent: asset.SpinCompletedEvent{
            PlayerID:      int64(userInfo.GetUserId()),
            RoomID:        roomID,
            RoomVersion:   nextRoomData.Version,
            ConfigVersion: roomConfig.GetVersion(),
            State:         roomStateJSON,
            Result:        resultJSON,
        },
    })
    if err != nil {
        if errors.Is(err, asset.ErrNotEnoughGold) {
            hasError = true
            r.ResponseCode(session, code.NotEnoughMoney)
            return
        }

        hasError = true
        r.ResponseCode(session, code.AssetSettleFailed)
        return
    }

    // 5. PG 成功后，才提交 Actor 内存状态。
    // RoomDataManager 需要提供 ReplaceData(roomID, nextRoomData)。
    r.roomDataManager.ReplaceData(roomID, nextRoomData)

    // 6. 只提交 Room Snapshot 的异步任务；不影响当前返回。
    // SubmitTask 返回 false 只记录告警，不能回滚已完成金币结算。
    if ok := r.roomDataManager.SaveData(ctx, roomID); !ok {
        clog.Warnf(
            "room snapshot enqueue failed, operation=%s player=%d room=%d",
            settleResult.OperationID,
            userInfo.GetUserId(),
            roomID,
        )
    }

    // 7. 返回准确余额，客户端下次无需依赖旧缓存。
    response.SpinUserInfo = buildSpinUserInfo(settleResult.Balance)
    r.Response(session, response)
}
```

这里需要的 imports：

```go
import (
    "encoding/json"
    "errors"

    "google.golang.org/protobuf/proto"

    "github.com/cherry-game/examples/demo_cluster/internal/asset"
)
```

`buildSpinUserInfo` 按你们已有的 `SpinUserInfo` 定义实现即可。

---

## 7. 修正 RoomDataManager

当前 `roomDataInfo` 必须初始化；另外，`SaveData` 建议返回入队是否成功。

```go
func NewRoomDataManager(app cfacade.IApplication) *RoomDataManager {
    comp := app.Find("db_write_queue")

    var dbQueueComp *dbQueue.DBWriteQueueComponent
    if comp != nil {
        dbQueueComp = comp.(*dbQueue.DBWriteQueueComponent)
    }

    return &RoomDataManager{
        dbQueueComp:  dbQueueComp,
        roomDataInfo: make(map[int32]*slotsModel.RoomDataInfo),
    }
}

func (s *RoomDataManager) ReplaceData(
    roomID int32,
    roomData *slotsModel.RoomDataInfo,
) {
    s.roomDataInfo[roomID] = roomData
}

func (s *RoomDataManager) SaveData(
    ctx context.Context,
    roomID int32,
) bool {
    roomData := s.roomDataInfo[roomID]
    if roomData == nil || s.dbQueueComp == nil {
        return false
    }

    data, err := json.Marshal(roomData)
    if err != nil {
        return false
    }

    return s.dbQueueComp.SubmitTask(&dbQueue.DbWriteTask{
        Table:      tableName,
        ExtraKeyId: strconv.FormatInt(int64(roomData.RoomId), 10),
        PlayerID:   roomData.UserId,
        OpType:     dbQueue.OpUpdate,
        Data:       data,
    })
}
```

这段的前提是所有读写都发生在玩家 Actor 的串行 handler 内；如果你后续把 `RoomDataManager` 放到多个 goroutine 共用，必须改成带锁的共享 Repository。

---

## 8. Outbox Relay：负责把 PG 事件投递给 JetStream

新增 `demo_cluster/internal/asset/outbox_relay.go`：

```go
package asset

import (
	"context"
	"time"

	"github.com/nats-io/nats.go"
	"gorm.io/gorm"
)

type OutboxRelay struct {
	db *gorm.DB
	js nats.JetStreamContext
}

func NewOutboxRelay(db *gorm.DB, js nats.JetStreamContext) *OutboxRelay {
	return &OutboxRelay{
		db: db,
		js: js,
	}
}

func (r *OutboxRelay) Run(ctx context.Context) {
	ticker := time.NewTicker(100 * time.Millisecond)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			_ = r.publishBatch(ctx, 100)
		}
	}
}

func (r *OutboxRelay) publishBatch(ctx context.Context, limit int) error {
	var events []OutboxEvent

	err := r.db.WithContext(ctx).
		Where("status = ?", "PENDING").
		Order("created_at ASC").
		Limit(limit).
		Find(&events).Error
	if err != nil {
		return err
	}

	for _, event := range events {
		// Nats-Msg-Id 使 Relay 自身重试不会造成 JetStream 重复持久化。
		_, err := r.js.Publish(
			event.EventType,
			event.Payload,
			nats.MsgId(event.EventID),
		)
		if err != nil {
			r.db.WithContext(ctx).
				Model(&OutboxEvent{}).
				Where("event_id = ?", event.EventID).
				Updates(map[string]interface{}{
					"retry_count": gorm.Expr("retry_count + 1"),
				})
			continue
		}

		now := time.Now()
		r.db.WithContext(ctx).
			Model(&OutboxEvent{}).
			Where("event_id = ? AND status = ?", event.EventID, "PENDING").
			Updates(map[string]interface{}{
				"status":       "PUBLISHED",
				"published_at": now,
			})
	}

	return nil
}
```

需要在 NATS 配置中启用 JetStream：

```conf
jetstream {
  store_dir: "/data/nats-jetstream"
}
```

并创建 Stream：

```go
_, err := js.AddStream(&nats.StreamConfig{
    Name:      "GAME_EVENTS",
    Subjects:  []string{"game.>"},
    Storage:   nats.FileStorage,
    Retention: nats.LimitsPolicy,
    Replicas:  3, // 生产三节点 NATS 集群
})
```

---

## 9. DDB Room Snapshot 消费端的幂等写法

活动与 Room Snapshot 都必须以 `eventId` 做幂等，而不是相信 JetStream 不会重复投递。

伪代码如下：

```go
func applyRoomSnapshot(
    ctx context.Context,
    event asset.SpinCompletedEvent,
) error {
    _, err := dynamoClient.TransactWriteItems(ctx, &dynamodb.TransactWriteItemsInput{
        TransactItems: []types.TransactWriteItem{
            {
                Put: &types.Put{
                    TableName: aws.String("player_activity_state"),
                    Item: map[string]types.AttributeValue{
                        "pk":      &types.AttributeValueMemberS{Value: "PLAYER#" + strconv.FormatInt(event.PlayerID, 10)},
                        "sk":      &types.AttributeValueMemberS{Value: "ROOM#" + strconv.Itoa(int(event.RoomID))},
                        "version": &types.AttributeValueMemberN{Value: strconv.Itoa(event.RoomVersion)},
                        "state":   &types.AttributeValueMemberS{Value: string(event.State)},
                    },
                    ConditionExpression: aws.String(
                        "attribute_not_exists(#pk) OR #version < :version",
                    ),
                    ExpressionAttributeNames: map[string]string{
                        "#pk":      "pk",
                        "#version": "version",
                    },
                    ExpressionAttributeValues: map[string]types.AttributeValue{
                        ":version": &types.AttributeValueMemberN{
                            Value: strconv.Itoa(event.RoomVersion),
                        },
                    },
                },
            },
            {
                Put: &types.Put{
                    TableName: aws.String("processed_game_event"),
                    Item: map[string]types.AttributeValue{
                        "pk": &types.AttributeValueMemberS{
                            Value: "EVENT#" + event.EventID,
                        },
                        "ttl": &types.AttributeValueMemberN{
                            Value: strconv.FormatInt(time.Now().Add(30*24*time.Hour).Unix(), 10),
                        },
                    },
                    ConditionExpression: aws.String("attribute_not_exists(pk)"),
                },
            },
        },
    })
    return err
}
```

实际实现时要处理两种条件失败：

- `processed_game_event` 已存在：说明重复消息，直接 ACK。
- Room `version` 小于等于当前值：说明旧事件乱序到达，直接 ACK。

---

## 10. GiftType 的落地映射

保持现有 `GiftType` 给策划使用，但不要在业务里直接写字段。

```go
type AssetKind string

const (
    AssetGold        AssetKind = "core.gold"
    AssetDiamond     AssetKind = "core.diamond"
    AssetScratchCard AssetKind = "core.scratch_card"
)

func ResolveGiftType(
    activityID string,
    giftType int32,
) (AssetKind, bool) {
    switch giftType {
    case 1:
        return AssetGold, true
    case 2:
        return AssetScratchCard, true
    case 5:
        return AssetDiamond, true
    case 6:
        // 活动代币必须归属活动，不是全局 eventcoin。
        return AssetKind("event." + activityID + ".coin"), true
    case 30:
        return AssetKind("buff.free_coin"), true
    case 31:
        return AssetKind("buff.exp"), true
    }

    return "", false
}
```

规则是：

```text
Gold / Diamond / 充值相关资产
  → PG 资产账本 + 余额

活动进度 / 活动币发放
  → Spin 事件异步消费 → DynamoDB

活动币立即消费
  → DynamoDB 条件更新 + operation_id 幂等记录
```

这样新增活动只增加：

```text
event.<new_activity_id>.<asset_name>
```

无需修改金币结算主流程。