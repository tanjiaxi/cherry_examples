/*
 * @Author: t 921865806@qq.com
 * @Date: 2025-09-15 18:02:10
 * @LastEditors: t 921865806@qq.com
 * @LastEditTime: 2025-12-30 21:49:13
 * @FilePath: /examples/demo_cluster/nodes/game/module/online/online.go
 * @Description: 这是默认设置,请设置`customMade`, 打开koroFileHeader查看配置 进行设置: https://github.com/OBKoro1/koro1FileHeader/wiki/%E9%85%8D%E7%BD%AE
 */
package online

import (
	"sync"

	cherryFacade "github.com/cherry-game/cherry/facade"
	clog "github.com/cherry-game/cherry/logger"
)

var (
	currentServerId int32 = 0 // 当前游戏节点的serverId
)

var (
	lock        = &sync.RWMutex{}
	playerIdMap = make(map[int64]string) // key:playerId, value:agentActorPath
	uidMap      = make(map[int64]int64)  // key:UID, value:playerId
)

func BindPlayer(playerId int64, uid int64, agentActorPath string) {
	if playerId < 1 || uid < 1 || agentActorPath == "" {
		return
	}

	lock.Lock()
	defer lock.Unlock()

	playerIdMap[playerId] = agentActorPath
	uidMap[uid] = playerId
}

func UnBindPlayer(uid cherryFacade.UID) int64 {
	if uid < 1 {
		return 0
	}

	lock.Lock()
	defer lock.Unlock()

	playerId, found := uidMap[uid]
	if !found {
		return 0
	}

	delete(playerIdMap, playerId)
	delete(uidMap, uid)

	playerIdCount := len(playerIdMap)
	uidCount := len(uidMap)

	if playerIdCount == 0 || uidCount == 0 {
		clog.Infof("Unbind player uid = %d, playerIdCount = %d, uidCount = %d", uid, playerIdCount, uidCount)
	}

	return playerId
}

func GetPlayerId(uid cherryFacade.UID) int64 {
	if uid < 1 {
		return 0
	}

	lock.RLock()
	defer lock.RUnlock()

	if playerId, found := uidMap[uid]; found {
		return playerId
	}

	return 0
}

func Count() int {
	lock.Lock()
	defer lock.Unlock()

	count := len(playerIdMap)
	return count
}
