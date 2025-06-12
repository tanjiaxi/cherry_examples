package master

import (
	"github.com/cherry-game/cherry"
)

func Run(profileFilePath, nodeID string) {
	app := cherry.Configure(profileFilePath, nodeID, false, cherry.Cluster)
	app.Startup()
}
