package activity

import (
	"context"
	"errors"
	"fmt"
	"strconv"
	"strings"
	"sync"
	"time"

	clog "github.com/cherry-game/cherry/logger"
	asset "github.com/cherry-game/examples/demo_cluster/internal/asset"
)

var ErrAlreadyApplied = errors.New("activity event already applied")

// SpinCoinStore is the Activity consumer's persistence boundary.  `log` mode
// is intentionally in-memory and is useful only for NATS/JetStream testing.
// `dynamodb` mode is the production implementation.
type SpinCoinStore interface {
	GrantSpinCoin(context.Context, asset.SpinCompletedEvent, string, int64) error
}

// Store is the DynamoDB implementation. The
// Inbox Put, counter change, and activity revision happen in one DDB
// transaction: a crash before ACK is harmless because the next delivery sees
// the Inbox key and becomes an acknowledged no-op.
type Store struct {
	ddb      *DynamoClient
	table    string
	inboxTTL time.Duration
}

func NewStore(ddb *DynamoClient, inboxTTL time.Duration) *Store {
	if inboxTTL <= 0 {
		inboxTTL = 90 * 24 * time.Hour
	}
	return &Store{ddb: ddb, table: ddb.cfg.Table, inboxTTL: inboxTTL}
}

func (s *Store) GrantSpinCoin(ctx context.Context, event asset.SpinCompletedEvent, activityID string, amount int64) error {
	if err := event.Validate(); err != nil {
		return err
	}
	if activityID == "" || amount == 0 {
		return fmt.Errorf("invalid activity coin command")
	}
	pk := "USER#" + strconv.FormatInt(event.UserID, 10)
	now := time.Now().Unix()
	body := map[string]any{
		"TransactItems": []any{
			map[string]any{"Put": map[string]any{
				"TableName": s.table,
				"Item": map[string]any{
					"pk": attributeS(pk), "sk": attributeS("EVENT#" + event.EventID),
					"activity_id": attributeS(activityID), "event_type": attributeS("game.spin.completed.v1"),
					"created_at": attributeN(now), "expires_at": attributeN(now + int64(s.inboxTTL/time.Second)),
				},
				"ConditionExpression": "attribute_not_exists(pk) AND attribute_not_exists(sk)",
			}},
			map[string]any{"Update": map[string]any{
				"TableName":                 s.table,
				"Key":                       map[string]any{"pk": attributeS(pk), "sk": attributeS("ASSET#" + activityID + "#coin")},
				"UpdateExpression":          "SET #updated_at = :now ADD #balance :delta",
				"ExpressionAttributeNames":  map[string]string{"#updated_at": "updated_at", "#balance": "balance"},
				"ExpressionAttributeValues": map[string]any{":now": attributeN(now), ":delta": attributeN(amount)},
			}},
			map[string]any{"Update": map[string]any{
				"TableName":                 s.table,
				"Key":                       map[string]any{"pk": attributeS(pk), "sk": attributeS("ACTIVITY#" + activityID)},
				"UpdateExpression":          "SET #revision = if_not_exists(#revision, :zero) + :one, #updated_at = :now",
				"ExpressionAttributeNames":  map[string]string{"#revision": "revision", "#updated_at": "updated_at"},
				"ExpressionAttributeValues": map[string]any{":zero": attributeN(0), ":one": attributeN(1), ":now": attributeN(now)},
			}},
		},
	}
	err := s.ddb.transactWrite(ctx, body)
	if err == nil {
		return nil
	}
	var ddbErr *DynamoError
	if errors.As(err, &ddbErr) && ddbErr.Code == "com.amazonaws.dynamodb.v20120810#TransactionCanceledException" {
		// TransactionConflict is also represented by TransactionCanceledException.
		// Treat it as retryable; only the Inbox condition failure proves that an
		// earlier delivery committed the business update.
		if strings.Contains(ddbErr.Raw, "ConditionalCheckFailed") {
			return ErrAlreadyApplied
		}
	}
	return err
}

// LogStore verifies the entire asynchronous path without DynamoDB. It logs
// each first application and remembers event IDs only until the process exits.
// Therefore it must never be used as a production persistence mode.
type LogStore struct {
	mu       sync.Mutex
	applied  map[string]struct{}
	balances map[string]int64
	logEvery int64
	count    int64
}

func NewLogStore(logEvery int64) *LogStore {
	if logEvery <= 0 {
		logEvery = 100
	}
	return &LogStore{
		applied:  make(map[string]struct{}),
		balances: make(map[string]int64),
		logEvery: logEvery,
	}
}

func (s *LogStore) GrantSpinCoin(_ context.Context, event asset.SpinCompletedEvent, activityID string, amount int64) error {
	if err := event.Validate(); err != nil {
		return err
	}
	if activityID == "" || amount == 0 {
		return fmt.Errorf("invalid activity coin command")
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	if _, exists := s.applied[event.EventID]; exists {
		clog.Warnf("activity log store duplicate event=%s user=%d activity=%s", event.EventID, event.UserID, activityID)
		return ErrAlreadyApplied
	}
	s.applied[event.EventID] = struct{}{}
	s.count++
	key := strconv.FormatInt(event.UserID, 10) + ":" + activityID + ":coin"
	s.balances[key] += amount
	// if s.count == 1 || s.count%s.logEvery == 0 {
	// 	clog.Warnf("activity log store progress applied=%d latest_event=%s user=%d activity=%s balance=%d",
	// 		s.count, event.EventID, event.UserID, activityID, s.balances[key])
	// }
	clog.Warnf("activity log store progress applied=%d latest_event=%s user=%d activity=%s balance=%d",
		s.count, event.EventID, event.UserID, activityID, s.balances[key])
	return nil
}
