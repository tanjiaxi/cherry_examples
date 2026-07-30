/*
 * @Author: t 921865806@qq.com
 * @Date: 2025-09-15 18:02:10
 * @LastEditors: t 921865806@qq.com
 * @LastEditTime: 2026-06-23 16:26:58
 * @FilePath: /examples/demo_cluster/internal/rpc/game/game.go
 * @Description: 这是默认设置,请设置`customMade`, 打开koroFileHeader查看配置 进行设置: https://github.com/OBKoro1/koro1FileHeader/wiki/%E9%85%8D%E7%BD%AE
 */
package rpcGame

import (
	"fmt"

	cfacade "github.com/cherry-game/cherry/facade"
	clog "github.com/cherry-game/cherry/logger"
	cproto "github.com/cherry-game/cherry/net/proto"
	"github.com/cherry-game/examples/demo_cluster/internal/code"
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
func SessionClose(app cfacade.IApplication, session *cproto.Session, traceId string) {
	nodeID := session.GetString(sessionKey.ServerID)
	if nodeID == "" {
		clog.Warnf("Get server id fail. session = %s", session.Sid)
		return
	}

	targetPath := fmt.Sprintf("%s.%s.%s", nodeID, playerActor, session.Sid)
	app.ActorSystem().Call("", targetPath, sessionClose, traceId, &pb.Int64{
		Value: session.Uid,
	})

	//clog.Infof("send close session to game node. [node = %s, uid = %d]", nodeID, session.Uid)
}
func GetUserInfo(a cfacade.IActor, session *cproto.Session, traceId string) (*pb.GetUserInfoResponse, int32) {
	userInfo := &pb.GetUserInfoResponse{}
	//调用自己服务的actor
	targetPath := cfacade.NewChildPath(a.App().NodeID(), playerActor, session.Uid)
	errCode := a.CallWait(targetPath, getPlayerData, traceId, &pb.Int32{
		Value: int32(session.Uid),
	}, userInfo)
	if code.IsFail(errCode) {
		return nil, errCode
	}
	if userInfo.UserId <= 0 {
		return nil, code.PlayerNoUserInfo
	}
	return userInfo, code.OK
}
