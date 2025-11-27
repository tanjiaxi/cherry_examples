/*
 * @Author: t 921865806@qq.com
 * @Date: 2025-11-20 22:24:38
 * @LastEditors: t 921865806@qq.com
 * @LastEditTime: 2025-11-26 17:26:17
 * @FilePath: /examples/demo_cluster/nodes/game/module/slots/room/level_room.go
 * @Description: 这是默认设置,请设置`customMade`, 打开koroFileHeader查看配置 进行设置: https://github.com/OBKoro1/koro1FileHeader/wiki/%E9%85%8D%E7%BD%AE
 */
package slots

import (
	"sync"
	"time"

	clog "github.com/cherry-game/cherry/logger"
	"github.com/cherry-game/cherry/net/parser/pomelo"
	cproto "github.com/cherry-game/cherry/net/proto"
	configCacheSlots "github.com/cherry-game/examples/demo_cluster/internal/config_cache/slots"
	"github.com/cherry-game/examples/demo_cluster/internal/pb"
	spinManager "github.com/cherry-game/examples/demo_cluster/nodes/game/server/slots/spin_manager"
)

// 关卡房间 actor
// 一个玩家对应一个房间，
type (
	ActorRoom struct {
		pomelo.ActorBase
		curRoomId int32

		balance         int
		roomDataManager *spinManager.RoomDataManager
		levelMutex      *sync.RWMutex
		//同步控制
		syncTimer *time.Timer
		spinCount int
	}
)

func NewActorRoom() *ActorRoom {
	a := &ActorRoom{}
	a.roomDataManager = spinManager.NewSessoinManager()
	a.levelMutex = &sync.RWMutex{}
	return a
}
func (r *ActorRoom) OnInit() {
	clog.Debugf("[actorRoom] path = %s init!", r.PathString())
	//处理gate的节点actor消息
	r.Local().Register("entermachine", r.enterMachine) // 进入关卡
	r.Local().Register("machineinfo", r.machineinfo)   // 初始化关卡数据
	r.Local().Register("spin", r.spin)                 // 关卡spin
	r.Local().Register("bonus", r.bonus)               // 关卡bonus请求
	r.Local().Register("collect", r.collect)           // 关卡collect 请求
}
func (r *ActorRoom) enterMachine(session *cproto.Session, req *pb.EnterMachine) {
	roomId := req.Id
	n2CfgRoomlist, error := configCacheSlots.GetInstance().GetRoomConfig(roomId)
	response := &pb.EnterMachineResponse{
		Id:      roomId,
		Succeed: true,
	}
	if error != nil || n2CfgRoomlist == nil {
		response.Succeed = false
		r.Response(session, response)
	}
	r.Response(session, response)
}
func (r *ActorRoom) machineinfo(session *cproto.Session, req *pb.MachineInfo) {
	roomId := req.Id
	n2CfgRoomlist, error := configCacheSlots.GetInstance().GetRoomConfig(roomId)
	if error != nil || n2CfgRoomlist == nil {
		response := &pb.ErrorResponse{
			Code:    110004,
			Message: "没有关卡room 配置",
		}
		r.Response(session, response)
	}
	//初始化关卡数据
	// r.roomDataManager.InitSessionData(session, n2CfgRoomlist)
}

func (r *ActorRoom) spin(session *cproto.Session, _ *pb.Spin) {

}
func (r *ActorRoom) bonus(session *cproto.Session, _ *pb.Bonus) {

}
func (r *ActorRoom) collect(session *cproto.Session, _ *pb.CollectDone) {

}
