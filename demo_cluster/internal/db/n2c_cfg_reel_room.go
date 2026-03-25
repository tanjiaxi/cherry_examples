/*
 * @Author: t 921865806@qq.com
 * @Date: 2025-12-02 10:29:52
 * @LastEditors: t 921865806@qq.com
 * @LastEditTime: 2026-03-25 10:10:30
 * @FilePath: /examples/demo_cluster/internal/db/n2c_cfg_card.go
 * @Description: 这是默认设置,请设置`customMade`, 打开koroFileHeader查看配置 进行设置: https://github.com/OBKoro1/koro1FileHeader/wiki/%E9%85%8D%E7%BD%AE
 */
package db

import (
	"encoding/json"

	clog "github.com/cherry-game/cherry/logger"
	toolUtils "github.com/cherry-game/examples/demo_cluster/internal/common"
	"github.com/cherry-game/examples/demo_cluster/internal/component/db"
	gameModel "github.com/cherry-game/examples/demo_cluster/internal/model"
	logicGameModel "github.com/cherry-game/examples/demo_cluster/internal/model/logic_model"
	"github.com/jinzhu/copier"
)

func GetReelRoomConfig() []*gameModel.N2CfgReelRoom {
	var reelConfigConfig []*gameModel.N2CfgReelRoom
	result := db.GetDB().Find(&reelConfigConfig)
	if result.Error != nil {
		return nil
	}
	return reelConfigConfig
}

func FromatReelRoomConfig(reelRoomConfig []*gameModel.N2CfgReelRoom) []*logicGameModel.N2CfgReelRoom {
	logicReelRoomConfig := make([]*logicGameModel.N2CfgReelRoom, len(reelRoomConfig))
	//利用反射复制两个结构中相同的值
	err := copier.Copy(&logicReelRoomConfig, &reelRoomConfig)
	if err != nil {
		clog.Panic("copy reelRoomConfig err: %v", err)
		return nil
	}
	//转换为镜像map
	for i, v := range reelRoomConfig {
		reelsequencesByte, err := toolUtils.DecompressBase64Zlib(v.Reelsequences)
		if err != nil {
			clog.Panic("DecompressBase64Zlib reelRoomConfig err: %v", err)
			return nil
		}
		var data logicGameModel.SlotData
		err = json.Unmarshal(reelsequencesByte, &data)
		if err != nil {
			clog.Panic("解析 JSON 失败:", err)
			return nil
		}
		logicReelRoomConfig[i].Reelsequences = data
	}
	return logicReelRoomConfig
}
