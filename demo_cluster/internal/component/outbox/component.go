package outbox

import (
	"context"
	"time"

	cfacade "github.com/cherry-game/cherry/facade"
	clog "github.com/cherry-game/cherry/logger"
	cnats "github.com/cherry-game/cherry/net/nats"
	commonDB "github.com/cherry-game/examples/demo_cluster/internal/component/db"
)

const ComponentName = "outbox"

type Component struct {
	// Embed the concrete base component.  Embedding IComponent leaves a nil
	// interface in a newly allocated Component, so c.App() would panic during
	// startup before the framework can inject the application.
	cfacade.Component
	relay *Relay
}

func NewComponent() *Component {
	return &Component{}
}

func (c *Component) Name() string {
	return ComponentName
}

func (c *Component) Init() {
	settings := c.App().Settings().GetConfig("outbox")
	batchSize := settings.GetInt("batch_size", 100)
	workers := settings.GetInt("publish_workers", 16)
	pollInterval := settings.GetDuration("poll_interval_ms", 1000) * time.Millisecond
	lease := settings.GetDuration("lease_seconds", 30) * time.Second
	repo, err := NewRepository(commonDB.GetDB(), "game:"+c.App().NodeID(), lease)
	if err != nil {
		clog.Panicf("failed to create outbox repository: %v", err)
	}
	conn := cnats.GetConnect()
	publisher, err := NewPublisher(conn.Conn)
	if err != nil {
		clog.Panicf("failed to create outbox publisher: %v", err)
	}

	c.relay = NewRelay(repo, publisher, batchSize, workers, pollInterval)
}

func (c *Component) OnAfterInit() {
	c.relay.Start(context.Background())
	c.relay.Wake()
}

func (c *Component) OnBeforeStop() {
	if c.relay != nil {
		c.relay.Stop(5 * time.Second)
	}
}
func (c *Component) Wake() {
	if c.relay != nil {
		c.relay.Wake()
	}
}
