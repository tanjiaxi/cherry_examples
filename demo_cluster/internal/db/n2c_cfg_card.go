/*
 * @Author: t 921865806@qq.com
 * @Date: 2025-12-02 10:29:52
 * @LastEditors: t 921865806@qq.com
 * @LastEditTime: 2025-12-02 11:04:07
 * @FilePath: /examples/demo_cluster/internal/db/n2c_cfg_card.go
 * @Description: 这是默认设置,请设置`customMade`, 打开koroFileHeader查看配置 进行设置: https://github.com/OBKoro1/koro1FileHeader/wiki/%E9%85%8D%E7%BD%AE
 */
package db

import (
	"strconv"

	until "github.com/cherry-game/examples/demo_cluster/internal/common"
	"github.com/cherry-game/examples/demo_cluster/internal/component/db"
	gameModel "github.com/cherry-game/examples/demo_cluster/internal/model"
)

type FormatLevelConfig struct {
	gameModel.N2CfgLevel
	Exp       int
	Stakegear []int
}

func GetLevelConfig() []*gameModel.N2CfgLevel {
	var levelConfig []*gameModel.N2CfgLevel
	result := db.GetDB().Find(&levelConfig)
	if result.Error != nil {
		return nil
	}
	return levelConfig
}
func FormatData(levelConfig *gameModel.N2CfgLevel) (*FormatLevelConfig, error) {
	exp, err := strconv.Atoi(levelConfig.Exp)
	if err != nil {
		return nil, err
	}
	stakegear, err := until.SplitNumber(levelConfig.Stakegear, ",")
	if err != nil {
		return nil, err
	}
	var formatLevelConfig = &FormatLevelConfig{
		N2CfgLevel: *levelConfig,
		Exp:        exp,
		Stakegear:  stakegear,
	}
	return formatLevelConfig, nil
}
