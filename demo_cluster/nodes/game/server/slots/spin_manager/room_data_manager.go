/*
 * @Author: t 921865806@qq.com
 * @Date: 2025-11-24 18:11:37
 * @LastEditors: t 921865806@qq.com
 * @LastEditTime: 2026-07-02 18:15:09
 * @FilePath: /examples/demo_cluster/nodes/game/server/ slots/core/session_mgr.go
 * @Description: 管理房间的数据，获取数据库数据，落地数据到数据库
 */
package spinmanage

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	cfacade "github.com/cherry-game/cherry/facade"
	clog "github.com/cherry-game/cherry/logger"
	cherryRedis "github.com/cherry-game/examples/demo_cluster/internal/component/redis"
	slotsModel "github.com/cherry-game/examples/demo_cluster/nodes/game/model"
	"go.uber.org/zap"
)

type RoomDataManager struct {
	levelSessionDataMgr map[string]*slotsModel.RoomDataInfo // key userId:roomId， vaule 房间数据
	redisComp           *cherryRedis.RedisCompent           // 缓存组件引用
}

func NewSessoinManager(app cfacade.IApplication) *RoomDataManager {
	return &RoomDataManager{
		levelSessionDataMgr: make(map[string]*slotsModel.RoomDataInfo),
		redisComp:           app.Find(cherryRedis.Name).(*cherryRedis.RedisCompent),
	}
}

func (s *RoomDataManager) GetKey(item ...string) string {
	// 1. 校验：如果没有传入任何参数，或者传入了空参数，直接返回空字符串
	if len(item) == 0 {
		return ""
	}

	// 2. 校验：检查是否有非法（空）的元素
	for _, val := range item {
		if val == "" || val == "0" { // 对应原逻辑中 <= 0 的边界处理
			return ""
		}
	}

	// 3. 拼接：使用 ":" 将所有传入的字符串片段连接起来
	return strings.Join(item, ":")
}

func (s *RoomDataManager) NewLevelSessionData(userID, roomId int32) *slotsModel.RoomDataInfo {
	return &slotsModel.RoomDataInfo{
		RoomId: roomId,
		UserId: userID,
	}
}

func (s *RoomDataManager) GetData(userID, roomId string) (*slotsModel.RoomDataInfo, error) {
	roomKey := s.GetKey(userID, roomId)
	if _, ok := s.levelSessionDataMgr[roomKey]; !ok {
		s.levelSessionDataMgr[roomKey] = s.NewLevelSessionData(StringToInt32(userID), roomId)
	}
	// return s.levelSessionDataMgr[roomId], nil
	// 这里得到的是一份copy数据，保证了数据一致性
	// copyData, err := s.levelSessionDataMgr[roomKey].CopierToData()
	// if err != nil {
	// 	return nil, err
	// }
	return s.levelSessionDataMgr[roomKey], nil
}

func (s *RoomDataManager) SaveData(roomDataInfo *slotsModel.RoomDataInfo) error {
	if roomDataInfo == nil {
		return fmt.Errorf("roomDataInfo is nil")
	}
	roomKey := s.getKey(roomDataInfo.UserId, roomDataInfo.RoomId)
	// 1. 获取 Map 里当前的旧数据 (它即将被丢弃)
	oldData, exists := s.levelSessionDataMgr[roomKey]
	s.levelSessionDataMgr[roomKey] = roomDataInfo
	if exists && oldData != nil {
		// 必须 Reset 清空数据，防止内存泄漏或数据污染
		oldData.Reset()
		// 扔回池子，等待下一次被 Get 复用
		slotsModel.RoomDataPool.Put(oldData)
	}
	return nil
}

// 数据同步
func (s *RoomDataManager) SaveRoomDataInfo(ctx context.Context, roomDataInfo *slotsModel.RoomDataInfo) {
	stringCmd := s.redisComp.GetDb().Get(ctx, "roomIn")
	var roomData slotsModel.RoomDataInfo

	// 3. 将 JSON 字符串反序列化回结构体
	json.Unmarshal([]byte(stringCmd.Val()), &roomData)

	clog.DebugContext(ctx, "stringCmd", zap.Any("stringVal", roomData))
	roomDataBytes, err := json.Marshal(roomDataInfo)
	if err != nil {
		clog.DebugContext(ctx, "SaveRoomDataInfo", zap.String("err", err.Error()))
		return
	}
	statusCmd := s.redisComp.GetDb().SetEx(ctx, "roomIn", string(roomDataBytes), 5*time.Minute)
	if statusCmd.Err() != nil {
		clog.DebugContext(ctx, "SaveRoomDataInfo", zap.String("statusCmd", statusCmd.Val()), zap.String("statusError", statusCmd.Err().Error()))
	}
	if !roomDataInfo.IsDirty {
		return
	} else if roomDataInfo.IsDirty == true {
		// 数据同步到数据库
		roomDataInfo.IsDirty = false
		return
	}

	roomDataInfo.UpdatedAt = time.Now().Unix()
}
