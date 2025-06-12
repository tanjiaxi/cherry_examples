package center

import (
	"github.com/cherry-game/cherry"
	cherryCron "github.com/cherry-game/cherry/components/cron"
	"github.com/cherry-game/examples/demo_cluster/internal/data"
	"github.com/cherry-game/examples/demo_cluster/nodes/center/db"
	"github.com/cherry-game/examples/demo_cluster/nodes/center/module/account"
	"github.com/cherry-game/examples/demo_cluster/nodes/center/module/ops"
)

func Run(profileFilePath, nodeID string) {
	app := cherry.Configure(
		profileFilePath,
		nodeID,
		false,
		cherry.Cluster,
	)

	app.Register(cherryCron.New())
	app.Register(data.New())
	app.Register(db.New())

	app.AddActors(
		&account.ActorAccount{},
		&ops.ActorOps{},
	)

	app.Startup()
}
