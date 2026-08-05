package activity

import (
	"github.com/cherry-game/cherry"
	cdiscovery "github.com/cherry-game/cherry/net/discovery"
	cherryETCD "github.com/cherry-game/components/etcd"
)

// Run starts a stateless Activity worker.  Multiple processes with the same
// durable name cooperate on one JetStream consumer; they are not duplicates.
func Run(profileFilePath, nodeID string) {
	app := cherry.Configure(profileFilePath, nodeID, false, cherry.Cluster)
	cdiscovery.Register(cherryETCD.New())
	app.Register(NewComponent())
	app.Startup()
}
