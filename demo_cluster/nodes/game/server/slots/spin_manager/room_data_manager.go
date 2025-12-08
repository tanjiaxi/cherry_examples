/*
 * @Author: t 921865806@qq.com
 * @Date: 2025-11-24 18:11:37
 * @LastEditors: t 921865806@qq.com
 * @LastEditTime: 2025-12-08 14:51:27
 * @FilePath: /examples/demo_cluster/nodes/game/server/ slots/core/session_mgr.go
 * @Description: 管理房间的数据，获取数据库数据，落地数据到数据库
 */
package spinmanage

import (
	"fmt"
	"time"

	slotsModel "github.com/cherry-game/examples/demo_cluster/nodes/game/model"
)

type RoomDataManager struct {
	levelSessionDataMgr map[int32]*slotsModel.RoomDataInfo //key 房间号， vaule 房间数据
}

func NewSessoinManager() *RoomDataManager {
	return &RoomDataManager{
		levelSessionDataMgr: make(map[int32]*slotsModel.RoomDataInfo),
	}
}
func (s *RoomDataManager) NewLevelSessionData(userID, roomId int32) *slotsModel.RoomDataInfo {
	return &slotsModel.RoomDataInfo{
		RoomId: roomId,
		UserId: userID,
	}
}
func (s *RoomDataManager) GetLevelSessionDataByRoomId(userID, roomId int32) (*slotsModel.RoomDataInfo, error) {
	if _, ok := s.levelSessionDataMgr[roomId]; !ok {
		s.levelSessionDataMgr[roomId] = s.NewLevelSessionData(userID, roomId)
	}
	// return s.levelSessionDataMgr[roomId], nil
	//这里得到的是一份copy数据，保证了数据一致性
	copyData, err := s.levelSessionDataMgr[roomId].CopierToData()
	if err != nil {
		return nil, err
	}
	return copyData, nil
}
func (s *RoomDataManager) UpdateLevelSessionData(roomDataInfo *slotsModel.RoomDataInfo) error {
	if roomDataInfo == nil {
		return fmt.Errorf("roomDataInfo is nil")
	}
	// 1. 获取 Map 里当前的旧数据 (它即将被丢弃)
	oldData, exists := s.levelSessionDataMgr[roomDataInfo.RoomId]
	s.levelSessionDataMgr[roomDataInfo.RoomId] = roomDataInfo
	if exists && oldData != nil {
		// 必须 Reset 清空数据，防止内存泄漏或数据污染
		oldData.Reset()
		// 扔回池子，等待下一次被 Get 复用
		slotsModel.RoomDataPool.Put(oldData)
	}
	return nil
}

// 数据同步
func (s *RoomDataManager) SaveRoomDataInfo(roomDataInfo *slotsModel.RoomDataInfo) {
	if !roomDataInfo.IsDirty {
		return
	} else if roomDataInfo.IsDirty == true {
		//数据同步到数据库
		roomDataInfo.IsDirty = false
		return
	}

	roomDataInfo.UpdatedAt = time.Now().Unix()
}
