package server

import (
	cherryTime "github.com/cherry-game/cherry/extend/time"
	cherryLogger "github.com/cherry-game/cherry/logger"
	"github.com/cherry-game/examples/demo_cluster/nodes/center/db"
	"github.com/cherry-game/examples/demo_cluster/nodes/center/model"
)

func GetUID(pid int32, openId string) (int32, bool) {

	val, err := db.GetUserBind(pid, openId)

	if val == nil || err != nil {
		return 0, false
	}

	return val.UserID, true
}

// BindUID 绑定UID
func BindUID(sdkId, pid int32, openId string, userId int32) (int32, bool) {
	// TODO 根据 platformType的配置要求，决定查询UID的方式：
	// 条件1: platformType + openId查询，是否存在uid
	// 条件2: pid + openId查询，是否存在uid

	userId, ok := GetUID(pid, openId)
	if ok {
		return userId, true
	}
	userBind := &model.UserBind{
		UserID:   userId,
		SdkID:    sdkId,
		Pid:      pid,
		OpenID:   openId,
		BindTime: cherryTime.Now().Time,
		UpTime:   cherryTime.Now().Time,
	}

	err := db.CreateUserBind(userBind)
	if err != nil {
		cherryLogger.Warnf("userBind fial,err:", err)
		return 0, true
	}
	return userBind.UserID, true
}
