package activity

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"sync"
	"time"

	clog "github.com/cherry-game/cherry/logger"
	asset "github.com/cherry-game/examples/demo_cluster/internal/asset"
	"github.com/cherry-game/examples/demo_cluster/internal/component/outbox"
	"github.com/nats-io/nats.go"
)

// SpinHandler contains activity-specific rule code.  It deliberately has no
// DynamoDB dependency; rules decide a delta while Store enforces durability.
type SpinHandler interface {
	ActivityID() string
	CoinDelta(asset.SpinCompletedEvent) (int64, error)
}

type FixedCoinHandler struct {
	ID          string
	CoinPerSpin int64
}

func (h FixedCoinHandler) ActivityID() string { return h.ID }
func (h FixedCoinHandler) CoinDelta(event asset.SpinCompletedEvent) (int64, error) {
	if h.CoinPerSpin <= 0 {
		return 0, fmt.Errorf("activity %s has non-positive coin_per_spin", h.ID)
	}
	return h.CoinPerSpin, nil
}

type ConsumerConfig struct {
	Durable       string
	AckWait       time.Duration
	MaxAckPending int
}

type Consumer struct {
	js      nats.JetStreamContext
	store   SpinCoinStore
	handler SpinHandler
	cfg     ConsumerConfig

	mu     sync.Mutex
	cancel context.CancelFunc
	done   chan struct{}
}

func NewConsumer(nc *nats.Conn, store SpinCoinStore, handler SpinHandler, cfg ConsumerConfig) (*Consumer, error) {
	if nc == nil || store == nil || handler == nil {
		return nil, fmt.Errorf("activity consumer: nil dependency")
	}
	if handler.ActivityID() == "" {
		return nil, fmt.Errorf("activity consumer: empty activity id")
	}
	if cfg.Durable == "" {
		cfg.Durable = "activity-" + handler.ActivityID() + "-v1"
	}
	if cfg.AckWait <= 0 {
		cfg.AckWait = 30 * time.Second
	}
	if cfg.MaxAckPending <= 0 {
		cfg.MaxAckPending = 256
	}
	js, err := nc.JetStream()
	if err != nil {
		return nil, fmt.Errorf("activity JetStream context: %w", err)
	}
	return &Consumer{js: js, store: store, handler: handler, cfg: cfg}, nil
}
func (c *Consumer) Start(parent context.Context) {
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.cancel != nil {
		return
	}
	ctx, cancel := context.WithCancel(parent)
	c.cancel, c.done = cancel, make(chan struct{})
	go c.run(ctx)
}

func (c *Consumer) Stop(timeout time.Duration) {
	c.mu.Lock()
	cancel, done := c.cancel, c.done
	c.cancel, c.done = nil, nil
	c.mu.Unlock()
	if cancel == nil {
		return
	}
	cancel()
	if timeout <= 0 {
		timeout = 10 * time.Second
	}
	select {
	case <-done:
	case <-time.After(timeout):
		clog.Warnf("activity consumer %s shutdown timed out", c.cfg.Durable)
	}
}

func (c *Consumer) run(ctx context.Context) {
	defer close(c.done)
	// The stream may not have been created yet when Activity deploys before
	// Game. Retry subscribe rather than taking the service down.
	for ctx.Err() == nil {
		sub, err := c.js.PullSubscribe("game.spin.completed.v1", c.cfg.Durable,
			nats.BindStream(outbox.StreamName), nats.ManualAck(), nats.AckExplicit(),
			nats.AckWait(c.cfg.AckWait), nats.MaxAckPending(c.cfg.MaxAckPending))
		if err != nil {
			clog.Errorf("activity consumer %s subscribe failed: %v", c.cfg.Durable, err)
			if !waitContext(ctx, 2*time.Second) {
				return
			}
			continue
		}
		c.consume(ctx, sub)
		_ = sub.Unsubscribe()
	}
}

func (c *Consumer) consume(ctx context.Context, sub *nats.Subscription) {
	for ctx.Err() == nil {
		// PullSubscribe does not push messages into NextMsgWithContext.  A Pull
		// consumer must first issue a Fetch request; calling NextMsgWithContext
		// on it produces "nats: invalid subscription type".  Fetch one event
		// at a time so an unacknowledged failure is naturally redelivered after
		// AckWait and does not let this worker advance business state.
		messages, err := sub.Fetch(10, nats.MaxWait(time.Second))
		if err != nil {
			if ctx.Err() == nil && !errors.Is(err, context.Canceled) && !errors.Is(err, nats.ErrTimeout) {
				clog.Errorf("activity consumer %s receive failed: %v", c.cfg.Durable, err)
			}
			// A one-second timeout is normal while no Spin has completed.  Keep
			// the subscription alive; other errors cause run() to recreate it.
			if errors.Is(err, nats.ErrTimeout) {
				continue
			}
			return
		}
		for _, msg := range messages {
			c.handle(ctx, msg)
		}
	}
}

func (c *Consumer) handle(ctx context.Context, msg *nats.Msg) {
	var event asset.SpinCompletedEvent
	if err := json.Unmarshal(msg.Data, &event); err != nil {
		c.ackPoison(ctx, msg, fmt.Errorf("decode spin event: %w", err))
		return
	}
	if err := event.Validate(); err != nil {
		c.ackPoison(ctx, msg, err)
		return
	}

	// The NATS header comes from the immutable Outbox row.  It must agree with
	// payload eventId; otherwise a corrupted message cannot poison another
	// event's Inbox record.

	if headerID := msg.Header.Get("X-Event-ID"); headerID != "" && headerID != event.EventID {
		c.ackPoison(ctx, msg, fmt.Errorf("event id header/payload mismatch"))
		return
	}
	delta, err := c.handler.CoinDelta(event)
	if err != nil {
		// This is configuration/rule failure, not a successful event. Do not ACK:
		// fixing configuration causes JetStream to redeliver it.
		clog.Errorf("activity rule failed durable=%s event=%s: %v", c.cfg.Durable, event.EventID, err)
		return
	}
	err = c.store.GrantSpinCoin(ctx, event, c.handler.ActivityID(), delta)
	if errors.Is(err, ErrAlreadyApplied) {
		_ = msg.Ack()
		return
	}
	if err != nil {
		// Network outage, DynamoDB throttle and transaction conflicts all remain
		// unacknowledged. JetStream redelivery plus Inbox makes retry safe.
		clog.Errorf("activity apply failed durable=%s event=%s: %v", c.cfg.Durable, event.EventID, err)
		return
	}
	if err := msg.Ack(); err != nil {
		clog.Errorf("activity ack failed durable=%s event=%s: %v", c.cfg.Durable, event.EventID, err)
	}
}

func (c *Consumer) ackPoison(ctx context.Context, msg *nats.Msg, cause error) {
	hash := sha256.Sum256(msg.Data)
	dlq := nats.NewMsg("game.activity.dlq.v1")
	dlq.Data = msg.Data
	dlq.Header.Set(nats.MsgIdHdr, "activity-dlq-"+hex.EncodeToString(hash[:]))
	dlq.Header.Set("X-Activity-Durable", c.cfg.Durable)
	dlq.Header.Set("X-Error", cause.Error())
	if _, err := c.js.PublishMsg(dlq, nats.Context(ctx)); err != nil {
		clog.Errorf("activity DLQ publish failed durable=%s: %v", c.cfg.Durable, err)
		return
	}
	if err := msg.Ack(); err != nil {
		clog.Errorf("activity poison ack failed durable=%s: %v", c.cfg.Durable, err)
	}
}

func waitContext(ctx context.Context, duration time.Duration) bool {
	timer := time.NewTimer(duration)
	defer timer.Stop()
	select {
	case <-ctx.Done():
		return false
	case <-timer.C:
		return true
	}
}
