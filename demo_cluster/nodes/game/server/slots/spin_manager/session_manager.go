/*
 * @Author: t 921865806@qq.com
 * @Date: 2025-11-24 18:11:37
 * @LastEditors: t 921865806@qq.com
 * @LastEditTime: 2025-11-24 21:34:55
 * @FilePath: /examples/demo_cluster/nodes/game/server/ slots/core/session_mgr.go
 * @Description: 管理房间的数据，获取数据库数据，落地数据到数据库
 */
package spinmanage

type SessoinManager struct {
	levelSessionDataMgr map[int]*LevelSessionData
}

func NewSessoinManager() *SessoinManager {
	return &SessoinManager{
		levelSessionDataMgr: make(map[int]*LevelSessionData),
	}
}
func (s *SessoinManager) NewLevelSessionData(userID, roomId int) *LevelSessionData {
	return &LevelSessionData{
		RoomId: roomId,
		UserId: userID,
	}
}
func (s *SessoinManager) GetLevelSessionDataByRoomId(userID, roomId int) *LevelSessionData {
	if _, ok := s.levelSessionDataMgr[roomId]; !ok {
		s.levelSessionDataMgr[roomId] = s.NewLevelSessionData(userID, roomId)
	}
	return s.levelSessionDataMgr[roomId]
}
