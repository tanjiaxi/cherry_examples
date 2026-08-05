package activity

import (
	"context"
	"os"
	"strings"
	"time"

	cfacade "github.com/cherry-game/cherry/facade"
	clog "github.com/cherry-game/cherry/logger"
	cnats "github.com/cherry-game/cherry/net/nats"
	activitydomain "github.com/cherry-game/examples/demo_cluster/internal/activity"
)

const componentName = "activity_spin_consumer"

type Component struct {
	cfacade.Component
	consumer *activitydomain.Consumer
}

func NewComponent() *Component  { return &Component{} }
func (*Component) Name() string { return componentName }

func (c *Component) Init() {
	settings := c.App().Settings().GetConfig("activity")
	storageMode := strings.ToLower(settings.GetString("storage", "log"))
	handler := activitydomain.FixedCoinHandler{
		ID:          settings.GetString("activity_id"),
		CoinPerSpin: settings.GetInt64("coin_per_spin", 1),
	}
	var store activitydomain.SpinCoinStore
	switch storageMode {
	case "log":
		// Local integration mode: consumer ACKs after printing; duplicates are
		// remembered only in this process's memory.
		store = activitydomain.NewLogStore(settings.GetInt64("log_every", 100))
		clog.Warnf("activity storage=log: no activity state is durable")
	case "dynamodb":
		store = c.newDynamoStore(settings)
	default:
		clog.Panicf("unsupported activity storage %q; use log or dynamodb", storageMode)
	}

	consumer, err := activitydomain.NewConsumer(cnats.GetConnect().Conn, store, handler, activitydomain.ConsumerConfig{
		Durable:       settings.GetString("durable"),
		AckWait:       settings.GetDuration("ack_wait_seconds", 30) * time.Second,
		MaxAckPending: settings.GetInt("max_ack_pending", 256),
	})
	if err != nil {
		clog.Panicf("activity consumer init: %v", err)
	}
	c.consumer = consumer
}

func (c *Component) newDynamoStore(settings cfacade.ProfileJSON) activitydomain.SpinCoinStore {
	dynamoCfg := activitydomain.DynamoConfig{
		Endpoint:     settings.GetString("dynamodb_endpoint"),
		Region:       settings.GetString("dynamodb_region", "us-east-1"),
		AccessKey:    settings.GetString("dynamodb_access_key"),
		SecretKey:    settings.GetString("dynamodb_secret_key"),
		SessionToken: settings.GetString("dynamodb_session_token"),
		SignRequests: settings.GetBool("dynamodb_sign_requests", true),
		Table:        settings.GetString("dynamodb_table"),
		Timeout:      settings.GetDuration("dynamodb_timeout_ms", 3000) * time.Millisecond,
	}
	if dynamoCfg.AccessKey == "" {
		dynamoCfg.AccessKey = os.Getenv("AWS_ACCESS_KEY_ID")
	}
	if dynamoCfg.SecretKey == "" {
		dynamoCfg.SecretKey = os.Getenv("AWS_SECRET_ACCESS_KEY")
	}
	if dynamoCfg.SessionToken == "" {
		dynamoCfg.SessionToken = os.Getenv("AWS_SESSION_TOKEN")
	}
	ddb, err := activitydomain.NewDynamoClient(dynamoCfg)
	if err != nil {
		clog.Panicf("activity DynamoDB init: %v", err)
	}
	return activitydomain.NewStore(ddb, settings.GetDuration("inbox_ttl_hours", 2160)*time.Hour)
}

func (c *Component) OnAfterInit() { c.consumer.Start(context.Background()) }
func (c *Component) OnBeforeStop() {
	if c.consumer != nil {
		c.consumer.Stop(10 * time.Second)
	}
}
