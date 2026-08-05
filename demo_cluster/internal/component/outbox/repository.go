package outbox

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/cherry-game/examples/demo_cluster/internal/model"
	"gorm.io/gorm"
)

const (
	tableName = "newsz_2024.domain_outbox"
)

type Repository struct {
	// 添加仓库相关的字段和方法
	db       *gorm.DB
	workerID string
	lease    time.Duration
}

func NewRepository(db *gorm.DB, workerID string, lease time.Duration) (*Repository, error) {
	if db == nil {
		return nil, fmt.Errorf("outbox repository: nil db")
	}
	if workerID == "" {
		return nil, fmt.Errorf("outbox repository: empty workerID")
	}
	if lease <= 0 {
		return nil, fmt.Errorf("outbox repository: invalid lease duration")
	}
	return &Repository{
		db:       db,
		workerID: workerID,
		lease:    lease,
	}, nil
}

// Claim returns rows leased by this relay.  The CTE is deliberately a very
// short PG transaction: publishing to NATS happens only after this commits.
// SKIP LOCKED makes active game nodes competitors instead of blockers.
// Claim 操作返回由该中继（relay）租用的行。这里的 CTE 特意设计为一个极短的 PG 事务：
// 只有在事务提交后，才会向 NATS 发布消息。
// SKIP LOCKED 使得活跃的游戏节点之间形成竞争关系，而非相互阻塞。
func (r *Repository) Claim(ctx context.Context, limit int) ([]model.DomainOutbox, error) {
	if limit <= 0 {
		return nil, fmt.Errorf("outbox repository: invalid limit")
	}
	rows := make([]model.DomainOutbox, 0, limit)
	leaseSeconds := int64(r.lease / time.Second)
	if leaseSeconds < 1 {
		leaseSeconds = 1
	}
	// 使用事务来确保原子性
	err := r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		return tx.Raw(`
WITH candidates AS (
    SELECT event_id
    FROM newsz_2024.domain_outbox
    WHERE status = 'PENDING'
      AND (locked_until IS NULL OR locked_until <= now())
      AND (next_attempt_at IS NULL OR next_attempt_at <= now())
    ORDER BY created_at ASC
    FOR UPDATE SKIP LOCKED
    LIMIT ?
)
UPDATE newsz_2024.domain_outbox AS o
SET locked_by = ?,
    locked_until = now() + (? * interval '1 second')
FROM candidates AS c
WHERE o.event_id = c.event_id
RETURNING o.event_id, o.aggregate_type, o.aggregate_id, o.event_type,
          o.payload, o.status, o.retry_count, o.created_at, o.published_at,
          o.locked_by, o.locked_until, o.next_attempt_at, o.last_error
`, limit, r.workerID, leaseSeconds).Scan(&rows).Error
	})
	if err != nil {
		return nil, fmt.Errorf("claim outbox batch: %w", err)
	}
	return rows, nil
}
func (r *Repository) MarkPublished(ctx context.Context, eventID string) error {
	if eventID == "" {
		return fmt.Errorf("outbox repository: empty eventID")
	}
	result := r.db.WithContext(ctx).Table(tableName).
		Where("event_id = ? AND status = 'PENDING' AND locked_by = ?", eventID, r.workerID).
		Updates(map[string]interface{}{
			"status":          "PUBLISHED",
			"published_at":    gorm.Expr("now()"),
			"locked_by":       nil,
			"locked_until":    nil,
			"next_attempt_at": nil,
			"last_error":      nil,
		})
	if result.Error != nil {
		return fmt.Errorf("mark outbox published: %w", result.Error)
	}
	if result.RowsAffected != 1 {
		return fmt.Errorf("outbox lease lost before mark published: %s", eventID)
	}
	return nil
}

// ReleaseForRetry increments the delivery attempt and releases the lease.
// Retry is intentionally unbounded: PUBLISHED means accepted by JetStream,
// not merely that a best-effort call was attempted.  Backoff keeps an outage
// from turning into a hot SQL loop.
// ReleaseForRetry 会增加投递尝试次数并释放租约。
// 重试机制特意未设上限：状态为 PUBLISHED 意味着消息已被 JetStream 接收，
// 而不仅仅是进行了一次尽力而为的调用尝试。退避（Backoff）机制可防止
// 服务中断演变成高频 SQL 循环。
func (r *Repository) ReleaseForRetry(ctx context.Context, eventID string, cause error) error {
	errText := "unknown publish failure"
	if cause != nil {
		errText = cause.Error()
	}
	if len(errText) > 2000 {
		errText = errText[:2000]
	}
	errText = strings.TrimSpace(errText)
	result := r.db.WithContext(ctx).Table(tableName).
		Where("event_id = ? AND status = 'PENDING' AND locked_by = ?", eventID, r.workerID).
		Updates(map[string]interface{}{
			"retry_count": gorm.Expr("retry_count + 1"),
			// Retry after 2, 4, ... up to 300 seconds.
			"next_attempt_at": gorm.Expr("now() + (LEAST(300, POWER(2, LEAST(retry_count + 1, 8))) * interval '1 second')"),
			"locked_by":       nil,
			"locked_until":    nil,
			"last_error":      errText,
		})
	if result.Error != nil {
		return fmt.Errorf("release outbox for retry: %w", result.Error)
	}
	return nil
}
