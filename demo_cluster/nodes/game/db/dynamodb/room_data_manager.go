/*
 * @Author: t 921865806@qq.com
 * @Date: 2025-11-24 18:11:37
 * @LastEditors: t 921865806@qq.com
 * @LastEditTime: 2026-07-04 21:23:47
 * @FilePath: /examples/demo_cluster/nodes/game/server/ slots/core/session_mgr.go
 * @Description: 管理房间的数据，获取数据库数据，落地数据到数据库
 */
package dynamodb

import (
	"context"
	"fmt"
	"strconv"
	"time"

	"github.com/allegro/bigcache/v3"
	cfacade "github.com/cherry-game/cherry/facade"
	clog "github.com/cherry-game/cherry/logger"
	"github.com/cherry-game/examples/demo_cluster/internal/cache"
	cherryRedis "github.com/cherry-game/examples/demo_cluster/internal/component/redis"
	ifacade "github.com/cherry-game/examples/demo_cluster/internal/facade"
	slotsModel "github.com/cherry-game/examples/demo_cluster/nodes/game/model"
	"go.uber.org/zap"
)

type RoomDataManager struct {
	// 后面可以换成任意的方式存储
	cacheEntity ifacade.DataManager[*slotsModel.RoomDataInfo]
}

func NewRoomDataManager(app cfacade.IApplication) *RoomDataManager {
	tableName := "classic_slots_user_room"
	redisComp := app.Find(cherryRedis.Name).(*cherryRedis.RedisCompent)
	// 初始化 BigCache 的配置
	bcConfig := bigcache.DefaultConfig(30 * time.Minute)
	cacheEntity, error := cache.NewDataRepository[*slotsModel.RoomDataInfo](tableName, redisComp, bcConfig)
	if error != nil {
		clog.ErrorContext(context.Background(), "new cacheEntity error", zap.Any("error", error))
	}
	return &RoomDataManager{
		cacheEntity: cacheEntity,
	}
}

func (s *RoomDataManager) NewPlayerRoomData(userID, roomId int32) *slotsModel.RoomDataInfo {
	return &slotsModel.RoomDataInfo{
		RoomId: roomId,
		UserId: userID,
	}
}

func (s *RoomDataManager) GetData(ctx context.Context, userID, roomId int32) *slotsModel.RoomDataInfo {
	roomInfo, isSuccess := s.cacheEntity.GetData(ctx, func() *slotsModel.RoomDataInfo {
		return s.NewPlayerRoomData(userID, roomId)
	}, strconv.FormatInt(int64(userID), 10), strconv.FormatInt(int64(roomId), 10))
	if isSuccess {
		return roomInfo
	}
	return nil
}

func (s *RoomDataManager) SaveData(ctx context.Context, roomDataInfo *slotsModel.RoomDataInfo) error {
	if roomDataInfo == nil {
		return fmt.Errorf("roomDataInfo is nil")
	}
	return s.cacheEntity.SaveData(ctx, roomDataInfo, strconv.FormatInt(int64(roomDataInfo.UserId), 10), strconv.FormatInt(int64(roomDataInfo.RoomId), 10))
}
