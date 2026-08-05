package outbox

import (
	"context"
	"fmt"
	"time"

	"github.com/cherry-game/examples/demo_cluster/internal/model"
	"github.com/nats-io/nats.go"
)

const (
	StreamName = "GAME_EVENTS"
)

type Publisher struct {
	js nats.JetStreamContext
}

func NewPublisher(nc *nats.Conn) (*Publisher, error) {
	if nc == nil {
		return nil, fmt.Errorf("outbox publisher: nil nats connection")
	}
	js, err := nc.JetStream()
	if err != nil {
		return nil, fmt.Errorf("outbox publisher: failed to get JetStream context: %w", err)
	}
	//保证 stream存在,就不创建了
	if _, err := js.StreamInfo(StreamName); err != nil {
		if _, addErr := js.AddStream(&nats.StreamConfig{
			Name:      StreamName,
			Subjects:  []string{"game.>"},
			Storage:   nats.FileStorage,
			Retention: nats.LimitsPolicy,
			MaxAge:    7 * 24 * time.Hour,
		}); addErr != nil {
			//并发可能导致创建失败,再尝试获取一次.double check if the stream exists after the add stream error
			if _, infoErr := js.StreamInfo(StreamName); infoErr != nil {
				return nil, fmt.Errorf("outbox publisher: failed to get stream info after add stream error: %w", addErr)
			}
		}
	}
	return &Publisher{js: js}, nil
}
func (p *Publisher) Publish(ctx context.Context, event model.DomainOutbox) error {
	if event.EventType == "" || event.EventID == "" {
		return fmt.Errorf("outbox publisher: invalid event")
	}
	msg := nats.NewMsg(event.EventType)
	msg.Data = []byte(event.Payload)
	// JetStream de-duplicates this message id for its configured duplicate
	// window.  Activity still has its own durable Inbox because that window
	// cannot prove exactly-once business execution.
	msg.Header.Set(nats.MsgIdHdr, event.EventID)
	msg.Header.Set("X-Event-ID", event.EventID)
	msg.Header.Set("X-Aggregate-ID", event.AggregateID)
	_, err := p.js.PublishMsg(msg, nats.Context(ctx))
	if err != nil {
		return fmt.Errorf("outbox publisher: failed to publish message: %w", err)
	}
	return nil
}
