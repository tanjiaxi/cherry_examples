/*
 * @Author: t 921865806@qq.com
 * @Date: 2025-09-15 18:02:10
 * @LastEditors: t 921865806@qq.com
 * @LastEditTime: 2025-12-01 22:08:23
 * @FilePath: /examples/demo_cluster/internal/rpc/game/game.go
 * @Description: 这是默认设置,请设置`customMade`, 打开koroFileHeader查看配置 进行设置: https://github.com/OBKoro1/koro1FileHeader/wiki/%E9%85%8D%E7%BD%AE
 */
package rpcGame

import (
	"fmt"

	cfacade "github.com/cherry-game/cherry/facade"
	clog "github.com/cherry-game/cherry/logger"
	cproto "github.com/cherry-game/cherry/net/proto"
	"github.com/cherry-game/examples/demo_cluster/internal/pb"
	sessionKey "github.com/cherry-game/examples/demo_cluster/internal/session_key"
)

const (
	playerActor = "player"
)

const (
	sessionClose  = "sessionClose"
	getPlayerData = "getPlayerData"
)
const (
// sourcePath = ".system"
)

// SessionClose 如果session已登录，则调用rpcGame.SessionClose() 告知游戏服
func SessionClose(app cfacade.IApplication, session *cproto.Session) {
	nodeID := session.GetString(sessionKey.ServerID)
	if nodeID == "" {
		clog.Warnf("Get server id fail. session = %s", session.Sid)
		return
	}

	targetPath := fmt.Sprintf("%s.%s.%s", nodeID, playerActor, session.Sid)
	app.ActorSystem().Call("", targetPath, sessionClose, &pb.Int64{
		Value: session.Uid,
	})

	//clog.Infof("send close session to game node. [node = %s, uid = %d]", nodeID, session.Uid)
}
func GetUserInfo(a cfacade.IActor, session *cproto.Session) *pb.GetUserInfoResponse {
	nodeID := session.GetString(sessionKey.ServerID)
	if nodeID == "" {
		clog.Warnf("Get server id fail. session = %s", session.Uid)
		return nil
	}
	userInfo := &pb.GetUserInfoResponse{}
	targetPath := cfacade.NewChildPath("", playerActor, session.Uid)
	a.CallWait(targetPath, getPlayerData, &pb.Int32{
		Value: int32(session.Uid),
	}, userInfo)
	// app.ActorSystem().CallWait(sourcePath, targetPath, getPlayerData, &pb.Int64{
	// 	Value: session.Uid,
	// }, userInfo)
	return userInfo
}
