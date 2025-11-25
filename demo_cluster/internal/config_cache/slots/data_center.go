/*
 * @Author: t 921865806@qq.com
 * @Date: 2025-11-20 23:45:18
 * @LastEditors: t 921865806@qq.com
 * @LastEditTime: 2025-11-24 16:44:59
 * @FilePath: /examples/demo_cluster/nodes/game/db/slots/data_center.go
 * @Description: 这是默认设置,请设置`customMade`, 打开koroFileHeader查看配置 进行设置: https://github.com/OBKoro1/koro1FileHeader/wiki/%E9%85%8D%E7%BD%AE
 */
package slots

import (
	"fmt"
	"sync"
	"sync/atomic"

	clog "github.com/cherry-game/cherry/logger"
	gameModel "github.com/cherry-game/examples/demo_cluster/nodes/game/model"
)

type DataCenter struct {
	//automic.value 储存配置快照,可以热更
	snapshotAuto atomic.Value // *ConfigSnapshot
	reloadMu     sync.Mutex
	// 数据加载器
	loader *DataLoader
}

var (
	instance *DataCenter
	once     sync.Once
)

func GetInstance() *DataCenter {
	once.Do(func() {
		instance = &DataCenter{}
		instance.loader = NewDataLoader()
	})
	return instance
}
func (dc *DataCenter) getSnapshot() *ConfigSnapshot {
	val := dc.snapshotAuto.Load()
	if val == nil {
		// 处理未初始化的情况，比如返回默认值或 nil
		return nil
	}
	return val.(*ConfigSnapshot)
}

// 更新配置
func (dc *DataCenter) Reload() error {
	dc.reloadMu.Lock()
	defer dc.reloadMu.Unlock()
	newonfigSnapshot, err := dc.loader.LoadAllConfig()
	if err != nil {
		return err
	}
	//原子替换
	dc.snapshotAuto.Store(newonfigSnapshot)
	return nil
}

// 获取card 配置
/*
roomID  规则房间ID 1，2，3
*/
func (dc *DataCenter) GetCardConfig(roomID int32) (map[int32]*gameModel.N2CfgCard, error) {
	n2CfgCard := make(map[int32]*gameModel.N2CfgCard)
	allN2CfgCard := dc.getSnapshot().N2CfgCard
	for _, v := range allN2CfgCard {
		if v.RoomID == roomID {
			n2CfgCard[v.Cardindex] = v
		}
	}
	if len(n2CfgCard) == 0 {
		clog.Panic("room %d no card config ", roomID)
		return nil, fmt.Errorf("room %d no card config ", roomID)
	}
	return n2CfgCard, nil
}

//获取room配置
/*
roomID  真实房间ID 1001 ，1002
*/
func (dc *DataCenter) GetRoomConfig(roomID int32) (*gameModel.N2CfgRoomlist, error) {
	allN2CfgRoomlist := dc.getSnapshot().N2CfgRoomlist
	var n2CfgRoomlist *gameModel.N2CfgRoomlist
	for _, v := range allN2CfgRoomlist {
		if v.RoomID == roomID {
			n2CfgRoomlist = v
			break
		}
	}
	if n2CfgRoomlist == nil {
		clog.Panic("room %d no room config ", roomID)
		return nil, fmt.Errorf("room %d no room config ", roomID)
	}
	return n2CfgRoomlist, nil
}
func (dc *DataCenter) GetN2CfgReelRoom(roomID int) (*gameModel.N2CfgReelRoom, error) {
	allN2CfgReelRoom := dc.getSnapshot().N2CfgReelRoom
	if allN2CfgReelRoom[roomID] == nil {
		clog.Panic("room %d no reel room config ", roomID)
		return nil, fmt.Errorf("room %d no reel room config ", roomID)
	}
	return allN2CfgReelRoom[roomID], nil
}
