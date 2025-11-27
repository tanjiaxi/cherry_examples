/*
 * @Author: t 921865806@qq.com
 * @Date: 2025-11-24 18:11:37
 * @LastEditors: t 921865806@qq.com
 * @LastEditTime: 2025-11-26 14:23:39
 * @FilePath: /examples/demo_cluster/nodes/game/server/ slots/core/session_mgr.go
 * @Description: 管理房间的数据，获取数据库数据，落地数据到数据库
 */
package spinmanage

type RoomDataManager struct {
	levelSessionDataMgr map[int]*RoomDataInfo //key 房间号， vaule 房间数据
}

func NewSessoinManager() *RoomDataManager {
	return &RoomDataManager{
		levelSessionDataMgr: make(map[int]*RoomDataInfo),
	}
}
func (s *RoomDataManager) NewLevelSessionData(userID, roomId int) *RoomDataInfo {
	return &RoomDataInfo{
		RoomId: roomId,
		UserId: userID,
	}
}
func (s *RoomDataManager) GetLevelSessionDataByRoomId(userID, roomId int) *RoomDataInfo {
	if _, ok := s.levelSessionDataMgr[roomId]; !ok {
		s.levelSessionDataMgr[roomId] = s.NewLevelSessionData(userID, roomId)
	}
	return s.levelSessionDataMgr[roomId]
}
