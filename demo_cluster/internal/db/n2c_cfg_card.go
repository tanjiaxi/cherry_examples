/*
 * @Author: t 921865806@qq.com
 * @Date: 2025-12-02 10:29:52
 * @LastEditors: t 921865806@qq.com
 * @LastEditTime: 2025-12-11 23:49:24
 * @FilePath: /examples/demo_cluster/internal/db/n2c_cfg_card.go
 * @Description: 这是默认设置,请设置`customMade`, 打开koroFileHeader查看配置 进行设置: https://github.com/OBKoro1/koro1FileHeader/wiki/%E9%85%8D%E7%BD%AE
 */
package db

import (
	"slices"

	until "github.com/cherry-game/examples/demo_cluster/internal/common"
	"github.com/cherry-game/examples/demo_cluster/internal/component/db"
	gameModel "github.com/cherry-game/examples/demo_cluster/internal/model"
)

type FormatCardConfig struct {
	gameModel.N2CfgCard
	MirrorImageIdList     []int         //镜像id
	Conditionoddsseq      []int         //不同中奖情况中奖金额 (数组号为中奖个数)
	Conlittlegameidseq    []int         //小游戏类型  (数组号为中奖个数)
	Confreespinsseq       []int         //免费转次数 (数组号为中奖个数)
	MixedOdds             map[int][]int //不同中奖情况中奖金额 (key为中奖个数，value为中奖金额)
	MixedGroupAll         []int         //混合组
	Conditionargumentseq  map[int]int   //不同中奖情况中奖金额 (key为中奖个数，value为中奖金额)
	ConlittlegameidseqAll map[int]int   //小游戏类型  (key为中奖个数，value为小游戏类型)
	MixedCondition        []string
	MixedGroup            int
}

func GetCardConfig() []*gameModel.N2CfgCard {
	var cardConfig []*gameModel.N2CfgCard
	result := db.GetDB().Find(&cardConfig)
	if result.Error != nil {
		return nil
	}
	return cardConfig
}
func FormatCardConfigData(cardConfig *gameModel.N2CfgCard) (*FormatCardConfig, error) {
	mirrorImageIDList, err := until.SplitNumber(cardConfig.MirrorImageID, ";")
	if err != nil {
		return nil, err
	}
	if cardConfig.RoomID == 86 {
		cardConfig.RoomID = 86
	}
	var formatCardConfig = &FormatCardConfig{
		N2CfgCard:         *cardConfig,
		MirrorImageIdList: mirrorImageIDList,
	}
	conditionAll, err := until.SplitNumber(cardConfig.Conditionargumentseq, ";")
	conditionOddsAll, err := until.SplitNumber(cardConfig.Conditionoddsseq, ";")
	conLittleGameIdAll, err := until.SplitNumber(cardConfig.Conlittlegameidseq, ";")
	conFreeSpinsAll, err := until.SplitNumber(cardConfig.Confreespinsseq, ";")
	if len(conditionAll) > 0 {
		formatCardConfig.Conditionoddsseq = make([]int, slices.Max(conditionAll)+1)
	} else {
		formatCardConfig.Conditionoddsseq = make([]int, 0)
	}
	if len(conLittleGameIdAll) > 0 {
		formatCardConfig.Conlittlegameidseq = make([]int, slices.Max(conditionAll)+1)
	} else {
		formatCardConfig.Conlittlegameidseq = make([]int, 0)
	}
	if len(conFreeSpinsAll) > 0 {
		formatCardConfig.Confreespinsseq = make([]int, slices.Max(conditionAll)+1)
	} else {
		formatCardConfig.Confreespinsseq = make([]int, 0)
	}
	formatCardConfig.Conditionargumentseq = make(map[int]int)
	for i := 0; i < len(conditionAll); i++ {
		if conditionAll[i] <= 0 {
			continue
		}
		formatCardConfig.Conditionargumentseq[conditionAll[i]] = 1
		if len(conditionOddsAll) > i {
			formatCardConfig.Conditionoddsseq[conditionAll[i]] = conditionOddsAll[i]
		}
		if len(conLittleGameIdAll) > i {
			formatCardConfig.Conlittlegameidseq[conditionAll[i]] = conLittleGameIdAll[i]
		}
		if len(conFreeSpinsAll) > i {
			formatCardConfig.Confreespinsseq[conditionAll[i]] = conFreeSpinsAll[i]
		}
	}
	mixedGroupAll := make([]int, 0)
	mixedOddsAll := make([]string, 0)
	mixedConditionAll := make([]string, 0)
	if cardConfig.MixedGroup != "" {
		mixedGroupAll, err = until.SplitNumber(cardConfig.MixedGroup, ";")
	} else {
		mixedGroupAll = append(mixedGroupAll, 0)
	}
	if cardConfig.MixedOdds != "" {
		mixedOddsAll, err = until.SplitString(cardConfig.MixedOdds, ";")
	} else {
		mixedOddsAll = append(mixedOddsAll, "0")
	}
	if cardConfig.MixedCondition != "" {
		mixedConditionAll, err = until.SplitString(cardConfig.MixedCondition, ";")
	} else {
		mixedConditionAll = append(mixedConditionAll, "0")
	}
	formatCardConfig.MixedGroupAll = make([]int, len(mixedGroupAll))
	formatCardConfig.MixedGroup = 0
	formatCardConfig.MixedOdds = make(map[int][]int)
	for i := 0; i < len(mixedGroupAll); i++ {
		if mixedGroupAll[i] <= 0 {
			continue
		}
		formatCardConfig.MixedGroupAll[i] = mixedGroupAll[i]

		mixedOdds, err := until.SplitNumber(mixedOddsAll[i], ",")

		mixedCondition, err := until.SplitNumber(mixedConditionAll[i], ",")
		if err != nil {
			return nil, err
		}
		for k := 0; k < len(mixedCondition); k++ {
			condition := mixedCondition[k]
			if formatCardConfig.MixedOdds[mixedGroupAll[i]] == nil {
				formatCardConfig.MixedOdds[mixedGroupAll[i]] = make([]int, slices.Max(mixedCondition)+1)
			}
			formatCardConfig.MixedOdds[mixedGroupAll[i]][condition] = mixedOdds[k]
		}
		formatCardConfig.MixedGroup = 1
		formatCardConfig.MixedCondition = mixedConditionAll
	}
	return formatCardConfig, nil
}
