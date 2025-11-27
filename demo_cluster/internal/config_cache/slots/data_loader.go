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
	toolUtils "github.com/cherry-game/examples/demo_cluster/internal/common"
	gameModel "github.com/cherry-game/examples/demo_cluster/internal/model"
	logicGameModel "github.com/cherry-game/examples/demo_cluster/internal/model/logic_model"
	"github.com/cherry-game/examples/demo_cluster/nodes/game/db"
	"github.com/jinzhu/copier"
)

type ConfigSnapshot struct {
	Version  int64 //版本号
	LoadTime int64 //加载时间

	//配置数据
	N2CfgCard     map[int32]*gameModel.N2CfgCard
	N2CfgReelRoom map[int32]*logicGameModel.N2CfgReelRoom
	N2CfgRoomlist map[int32]*gameModel.N2CfgRoomlist
	N2CfgLevel    map[int32]*gameModel.N2CfgLevel //key是levelid
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
		N2CfgCard:     make(map[int32]*gameModel.N2CfgCard),
		N2CfgReelRoom: make(map[int32]*logicGameModel.N2CfgReelRoom),
		N2CfgRoomlist: make(map[int32]*gameModel.N2CfgRoomlist),
		N2CfgLevel:    make(map[int32]*gameModel.N2CfgLevel),
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

	if err := d.LoadLevelConfig(&configSnapshot); err != nil {
		clog.Panic("load level config failed: %w", err)
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
		configSnapshot.N2CfgCard[v.Kid] = v
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
		configSnapshot.N2CfgRoomlist[v.Kid] = v
	}
	return nil
}
func (d *DataLoader) LoadReelRoomConfig(configSnapshot *ConfigSnapshot) error {
	var reelRoomConfig []*gameModel.N2CfgReelRoom
	var logicReelRoomConfig []*logicGameModel.N2CfgReelRoom
	//从数据库查找
	result := db.GetDB().Find(&reelRoomConfig)
	if result.Error != nil {
		return result.Error
	}
	logicReelRoomConfig = make([]*logicGameModel.N2CfgReelRoom, len(reelRoomConfig))
	//利用反射复制两个结构中相同的值
	err := copier.Copy(&logicReelRoomConfig, &reelRoomConfig)
	if err != nil {
		clog.Panic("copy reelRoomConfig err: %v", err)
	}
	//转换为镜像map
	for i, v := range reelRoomConfig {
		reelsequencesByte, err := toolUtils.DecompressBase64Zlib(v.Reelsequences)
		if err != nil {
			clog.Panic("DecompressBase64Zlib reelRoomConfig err: %v", err)
		}
		logicReelRoomConfig[i].Reelsequences = reelsequencesByte
	}
	for _, v := range logicReelRoomConfig {
		configSnapshot.N2CfgReelRoom[v.RoomID] = v
	}
	return nil
}

func (d *DataLoader) LoadLevelConfig(configSnapshot *ConfigSnapshot) error {
	var levelConfig []*gameModel.N2CfgLevel
	//从数据库查找
	result := db.GetDB().Find(&levelConfig)
	if result.Error != nil {
		return result.Error
	}
	//转换为镜像map
	for _, v := range levelConfig {
		configSnapshot.N2CfgLevel[v.Levelid] = v
	}
	return nil
}
