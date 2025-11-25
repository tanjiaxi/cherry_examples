/**FileHeader
 * @Author: Your Name
 * @Date: 2025/11/21 16:06:16
 * @LastEditors: Your Name
 * @LastEditTime: 2025/11/21 17:48:54
 * @Description:
 * @Copyright: Copyright (©)}) 2025 Your Name. All rights reserved.
 * @Email: xxx@xxx.com
 */
package slots

import (
	"time"

	clog "github.com/cherry-game/cherry/logger"
	"github.com/cherry-game/examples/demo_cluster/nodes/game/db"
	gameModel "github.com/cherry-game/examples/demo_cluster/nodes/game/model"
)

type ConfigSnapshot struct {
	Version  int64 //版本号
	LoadTime int64 //加载时间

	//配置数据
	N2CfgCard     map[int]*gameModel.N2CfgCard
	N2CfgReelRoom map[int]*gameModel.N2CfgReelRoom
	N2CfgRoomlist map[int]*gameModel.N2CfgRoomlist
}
type DataLoader struct {
}

func NewDataLoader() *DataLoader {
	return &DataLoader{}
}

func (d *DataLoader) LoadAllConfig() (*ConfigSnapshot, error) {
	configSnapshot := ConfigSnapshot{
		Version:       time.Now().Unix(),
		LoadTime:      time.Now().Unix(),
		N2CfgCard:     make(map[int]*gameModel.N2CfgCard),
		N2CfgReelRoom: make(map[int]*gameModel.N2CfgReelRoom),
		N2CfgRoomlist: make(map[int]*gameModel.N2CfgRoomlist),
	}
	//加载配置
	if err := d.LoadCardConfig(&configSnapshot); err != nil {
		clog.Panic("load card config failed: %w", err)
	}
	if err := d.LoadRoomConfig(&configSnapshot); err != nil {
		clog.Panic("load room config failed: %w", err)
	}

	if err := d.LoadReelRoomConfig(&configSnapshot); err != nil {
		clog.Panic("load reel room config failed: %w", err)
	}
	return &configSnapshot, nil
}

/** card配置
 * @description:
 * @return {*}
 */
func (d *DataLoader) LoadCardConfig(configSnapshot *ConfigSnapshot) error {
	var cardConfig []*gameModel.N2CfgCard
	//从数据库查找
	result := db.GetDB().Find(&cardConfig)
	if result.Error != nil {
		return result.Error
	}
	//转换为镜像map
	for _, v := range cardConfig {
		configSnapshot.N2CfgCard[int(v.Kid)] = v
	}
	return nil
}

// room 配置
func (d *DataLoader) LoadRoomConfig(configSnapshot *ConfigSnapshot) error {
	var roomConfig []*gameModel.N2CfgRoomlist
	//从数据库查找
	result := db.GetDB().Find(&roomConfig)
	if result.Error != nil {
		return result.Error
	}
	//转换为镜像map
	for _, v := range roomConfig {
		configSnapshot.N2CfgRoomlist[int(v.Kid)] = v
	}
	return nil
}
func (d *DataLoader) LoadReelRoomConfig(configSnapshot *ConfigSnapshot) error {
	var reelRoomConfig []*gameModel.N2CfgReelRoom
	//从数据库查找
	result := db.GetDB().Find(&reelRoomConfig)
	if result.Error != nil {
		return result.Error
	}
	//转换为镜像map
	for _, v := range reelRoomConfig {
		configSnapshot.N2CfgReelRoom[int(v.RoomID)] = v
	}
	return nil
}
