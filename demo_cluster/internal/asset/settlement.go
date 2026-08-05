package asset

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"reflect"
	"strconv"
	"time"

	clog "github.com/cherry-game/cherry/logger"
	"github.com/cherry-game/examples/demo_cluster/internal/model"
	gameModel "github.com/cherry-game/examples/demo_cluster/internal/model"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgconn"
	"gorm.io/datatypes"
	"gorm.io/gorm"
)

type SettleSpinCommand struct {
	OperationID string
	EventID     string
	UserID      int64
	RequestID   string
	Bet         int64
	Win         int64
	Response    []byte
	OutboxJSON  []byte
}
type SettleSpinResult struct {
	Balance  int64
	Replay   bool
	Response []byte
}
type SpinOperation struct {
	OperationID     string
	Status          string
	ResponsePayload []byte
}
type Repository struct {
	db *gorm.DB
}

// 以下错误由上层 Actor 映射为业务错误码；
// 不要用字符串比较 PostgreSQL 错误信息。
var (
	ErrInvalidCommand   = errors.New("invalid settlement command")
	ErrInsufficientGold = errors.New("insufficient gold")
	ErrInProgress       = errors.New("operation is processing")
)

func NewRepository(db *gorm.DB) (*Repository, error) {
	if db == nil {
		return nil, errors.New("settlement repository: nil db")
	}
	return &Repository{db: db}, nil
}
func (r *Repository) SettleSpin(
	ctx context.Context, cmd SettleSpinCommand,
) (SettleSpinResult, error) {
	if cmd.UserID <= 0 || cmd.RequestID == "" || !isJSONObject(cmd.OutboxJSON) {
		return SettleSpinResult{}, ErrInvalidCommand
	}
	clog.Warnf("settle spin outbox insert: %v", datatypes.JSON(cmd.OutboxJSON))
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
		op := gameModel.AssetOperation{
			OperationID: cmd.OperationID, UserID: cmd.UserID,
			OperationType: "spin", RequestID: cmd.RequestID, Status: "COMPLETED",
			ResponsePayload: cmd.Response, CreatedAt: completedAt, CompletedAt: completedAt,
		}
		if err := tx.Create(&op).Error; err != nil {
			return err
		}
		// 2) 原子条件扣款。不能先 SELECT 余额再 UPDATE，后者会产生竞态。
		var user gameModel.SlotsUser
		err := tx.Model(&gameModel.SlotsUser{}).Where("user_id = ? AND money >= ?", cmd.UserID, cmd.Bet).
			UpdateColumn("money", gorm.Expr("money - ?", cmd.Bet)).
			Scan(&user).Error // 使用 Scan 接收 RETURNING 的结果
		if err != nil {
			return err
		}
		// 2. 依然通过 RowsAffected 判断是否因余额不足导致更新失败
		// 注意：在 GORM 中，即使用了 Scan，tx.RowsAffected 依然有效
		if user.UserID == 0 {
			return ErrInsufficientGold
		}
		afterDebit := user.Money
		// 3) 下注账本。账本是 append-only，不允许 UPDATE/DELETE。
		if err := tx.Create(&gameModel.AssetLedger{
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
			if err := tx.Create(&model.AssetLedger{
				OperationID: cmd.OperationID, UserID: cmd.UserID, AssetKind: "core.gold",
				Delta: cmd.Win, BalanceAfter: finalBalance, Reason: "spin_win",
				SourceType: "spin", SourceID: cmd.OperationID,
			}).Error; err != nil {
				return err
			}
		}
		// 5) Outbox 先于 Commit 写入。不能先 Commit 再 Publish NATS。
		eventID := cmd.EventID
		if eventID == "" {
			eventID = uuid.NewString()
		}
		// Write an explicit jsonb expression.  That removes []byte / bytea
		// parameter inference from the driver path entirely.
		// outboxValues := map[string]any{
		// 	"event_id":       eventID,
		// 	"aggregate_type": "user",
		// 	"aggregate_id":   strconv.FormatInt(cmd.UserID, 10),
		// 	"event_type":     "game.spin.completed.v1",
		// 	"payload":        datatypes.JSON(cmd.OutboxJSON),
		// 	"status":         "PENDING",
		// }
		outboxValues := &gameModel.DomainOutbox{
			EventID:       eventID,
			AggregateType: "user",
			AggregateID:   strconv.FormatInt(cmd.UserID, 10),
			EventType:     "game.spin.completed.v1",
			Payload:       datatypes.JSON(cmd.OutboxJSON), // 此时结构体字段会完美触发 Valuer 接口
			Status:        "PENDING",
		}
		if err := tx.Create(&outboxValues).Error; err != nil {
			return err
		}
		// Detect a database trigger/rule/default that rewrites payload before the
		// transaction commits. JSONB normalizes insignificant whitespace, so
		// compare semantic JSON instead of raw bytes.
		// var storedJSONStr string
		// if err := tx.Raw("SELECT payload::text FROM newsz_2024.domain_outbox WHERE event_id = ?", eventID).Scan(&storedJSONStr).Error; err != nil {
		// 	return fmt.Errorf("read just-written outbox payload: %w", err)
		// }
		// if !jsonEqual(cmd.OutboxJSON, []byte(storedJSONStr)) {
		// 	return fmt.Errorf("outbox payload changed while writing event %s: input=%s stored=%s", eventID, cmd.OutboxJSON, storedJSONStr)
		// }
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
func (r *Repository) SettleActivity() {

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
		Response: op.ResponsePayload,
	}, nil
}

func (r *Repository) FindSpinOperation(
	ctx context.Context, userID int64, requestID string,
) (*SpinOperation, error) {
	var op SpinOperation

	err := r.db.WithContext(ctx).
		Table("newsz_2024.asset_operation").
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

// isUniqueViolation 只负责识别 PG 唯一索引冲突（SQLSTATE 23505）。
// 例如 (user_id, operation_type, request_id) 冲突，通常代表客户端重试。
func isUniqueViolation(err error) bool {
	var pgErr *pgconn.PgError
	return errors.As(err, &pgErr) && pgErr.Code == "23505"
}

func isJSONObject(raw []byte) bool {
	if len(bytes.TrimSpace(raw)) == 0 {
		return false
	}
	var value map[string]json.RawMessage
	return json.Unmarshal(raw, &value) == nil && value != nil
}

func jsonEqual(left, right []byte) bool {
	var a, b any
	return json.Unmarshal(left, &a) == nil &&
		json.Unmarshal(right, &b) == nil &&
		reflect.DeepEqual(a, b)
}

type SpinCompletedEvent struct {
	EventID     string `json:"eventId"`
	UserID      int64  `json:"userId"`
	RoomID      int32  `json:"roomId"`
	Bet         int64  `json:"bet"`
	Win         int64  `json:"win"`
	CompletedAt int64  `json:"completedAt"`
}

// buildSpinCompletedEvent is the public, versioned JSON contract consumed by
// Activity.  Keep it independent of protobuf response layout.
func BuildSpinCompletedEvent(eventID string, userID int64, roomID int32, bet, win int64) *SpinCompletedEvent {
	return &SpinCompletedEvent{
		EventID:     eventID,
		UserID:      userID,
		RoomID:      roomID,
		Bet:         bet,
		Win:         win,
		CompletedAt: time.Now().UnixMilli(),
	}
}
func (e SpinCompletedEvent) Validate() error {
	if e.EventID == "" || e.UserID <= 0 || e.RoomID <= 0 || e.Bet < 0 || e.Win < 0 {
		return fmt.Errorf("invalid spin completed event")
	}
	return nil
}
