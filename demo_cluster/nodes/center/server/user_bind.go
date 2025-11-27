/*
 * @Author: t 921865806@qq.com
 * @Date: 2025-09-15 18:02:10
 * @LastEditors: t 921865806@qq.com
 * @LastEditTime: 2025-11-27 15:55:43
 * @FilePath: /examples/demo_cluster/nodes/center/server/user_bind.go
 * @Description: 这是默认设置,请设置`customMade`, 打开koroFileHeader查看配置 进行设置: https://github.com/OBKoro1/koro1FileHeader/wiki/%E9%85%8D%E7%BD%AE
 */
package server

import (
	cherryTime "github.com/cherry-game/cherry/extend/time"
	cherryLogger "github.com/cherry-game/cherry/logger"
	"github.com/cherry-game/examples/demo_cluster/internal/model"
	"github.com/cherry-game/examples/demo_cluster/nodes/center/db"
)

func GetUID(pid int32, openId string) (int32, bool) {

	val, err := db.GetUserBind(pid, openId)

	if val == nil || err != nil {
		return 0, false
	}

	return val.UserID, true
}

// BindUID 绑定UID openId也可能是device
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
