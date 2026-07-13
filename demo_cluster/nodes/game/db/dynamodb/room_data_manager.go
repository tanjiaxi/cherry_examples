/*
 * @Author: t 921865806@qq.com
 * @Date: 2025-11-24 18:11:37
 * @LastEditors: t 921865806@qq.com
 * @LastEditTime: 2026-07-13 20:06:50
 * @FilePath: /examples/demo_cluster/nodes/game/server/ slots/core/session_mgr.go
 * @Description: 管理房间的数据，获取数据库数据，落地数据到数据库
 */
package dynamodb

import (
	"context"
	"encoding/json"
	"fmt"
	"strconv"

	cfacade "github.com/cherry-game/cherry/facade"
	dbQueue "github.com/cherry-game/examples/demo_cluster/internal/component/db_queue"
	dbqueue "github.com/cherry-game/examples/demo_cluster/internal/component/db_queue"
	cherryRedis "github.com/cherry-game/examples/demo_cluster/internal/component/redis"
	slotsModel "github.com/cherry-game/examples/demo_cluster/nodes/game/model"
)

var tableName = "classic_slots_user_room"

type RoomDataManager struct {
	// 后面可以换成任意的方式存储
	// cacheEntity        ifacade.DataManager[*slotsModel.RoomDataInfo]
	persistenceBackend dbQueue.PersistenceBackend
	dbQueueComp        *dbQueue.DBWriteQueueComponent // 保存组件引用
	roomDataInfo       *slotsModel.RoomDataInfo
}

func NewRoomDataManager(app cfacade.IApplication) *RoomDataManager {
	redisComp := app.Find(cherryRedis.Name).(*cherryRedis.RedisCompent)
	// 初始化 BigCache 的配置
	// bcConfig := bigcache.DefaultConfig(30 * time.Minute)
	// cacheEntity, error := cache.NewDataRepository[*slotsModel.RoomDataInfo](tableName, redisComp, bcConfig)
	redisBackend := dbQueue.NewRedisBackend(redisComp)
	// if error != nil {
	// 	clog.ErrorContext(context.Background(), "new cacheEntity error", zap.Any("error", error))
	// }
	comp := app.Find("db_write_queue")
	var dbQueueComp *dbqueue.DBWriteQueueComponent
	if comp != nil {
		dbQueueComp = comp.(*dbqueue.DBWriteQueueComponent)
	}

	return &RoomDataManager{
		// cacheEntity: cacheEntity,
		dbQueueComp:        dbQueueComp,
		persistenceBackend: redisBackend,
	}
}

func (s *RoomDataManager) NewPlayerRoomData(userID, roomId int32) *slotsModel.RoomDataInfo {
	return &slotsModel.RoomDataInfo{
		RoomId: roomId,
		UserId: userID,
	}
}

func (s *RoomDataManager) GetData(ctx context.Context, userID, roomId int32) *slotsModel.RoomDataInfo {
	if s.roomDataInfo != nil {
		return s.roomDataInfo
	}
	data, err := s.persistenceBackend.Load(ctx, tableName, strconv.FormatInt(int64(userID), 10), strconv.FormatInt(int64(roomId), 10))
	var roomInfo *slotsModel.RoomDataInfo
	if err == nil {
		err = json.Unmarshal(data, roomInfo)
		s.roomDataInfo = roomInfo
		return roomInfo
	}
	if err != nil {
		return s.NewPlayerRoomData(userID, roomId)
	}
	return nil
	// roomInfo, isSuccess := s.cacheEntity.GetData(ctx, func() *slotsModel.RoomDataInfo {
	// 	return s.NewPlayerRoomData(userID, roomId)
	// }, strconv.FormatInt(int64(userID), 10), strconv.FormatInt(int64(roomId), 10))
	// if isSuccess {
	// 	return roomInfo
	// }
	// return nil
}

func (s *RoomDataManager) SaveData(ctx context.Context) error {
	if s.roomDataInfo == nil {
		return fmt.Errorf("no roomDataInfo")
	}
	data, err := json.Marshal(s.roomDataInfo)
	if err == nil {
		s.dbQueueComp.SubmitTask(&dbqueue.DbWriteTask{
			Table:      tableName,
			ExtraKeyId: strconv.FormatInt(int64(s.roomDataInfo.RoomId), 10),
			PlayerID:   s.roomDataInfo.UserId,

			OpType: dbQueue.OpUpdate,
			Data:   data,
		})
		return nil
	}
	return err
	// s.persistenceBackend.Save(ctx, tableName, strconv.FormatInt(int64(roomDataInfo.UserId), 10), strconv.FormatInt(int64(roomDataInfo.RoomId), 10), roomDataInfo)
	// return s.cacheEntity.SaveData(ctx, roomDataInfo, strconv.FormatInt(int64(roomDataInfo.UserId), 10), strconv.FormatInt(int64(roomDataInfo.RoomId), 10))
}
