/*
 * @Author: t 921865806@qq.com
 * @Date: 2025-12-04 10:07:21
 * @LastEditors: t 921865806@qq.com
 * @LastEditTime: 2025-12-04 10:58:10
 * @FilePath: /examples/demo_cluster/internal/db/lines_ids_cfg.go
 * @Description: 这是默认设置,请设置`customMade`, 打开koroFileHeader查看配置 进行设置: https://github.com/OBKoro1/koro1FileHeader/wiki/%E9%85%8D%E7%BD%AE
 */
package db

import (
	"fmt"
	"strconv"
	"strings"

	"github.com/cherry-game/examples/demo_cluster/internal/component/db"
	gameModel "github.com/cherry-game/examples/demo_cluster/internal/model"
)

func GetLinesIdsConfig() ([]*FormatLinesIdsConfig, error) {
	var linesIdsConfig []*gameModel.LinesID
	result := db.GetDB().Find(&linesIdsConfig)
	if result.Error != nil {
		return nil, result.Error
	}
	formatLinesIds, err := formatConifg(linesIdsConfig)
	if err != nil {
		return nil, err
	}
	return formatLinesIds, nil
}

type FormatLinesIdsConfig struct {
	Xy  string
	Ids []int
}

func formatConifg(linesIdsConfig []*gameModel.LinesID) ([]*FormatLinesIdsConfig, error) {
	formatLinesIds := make([]*FormatLinesIdsConfig, 0, len(linesIdsConfig))
	for _, v := range linesIdsConfig {
		idStrArray := strings.Split(strings.TrimSpace(v.Ids), ",")
		idIntArray := make([]int, 0, len(idStrArray))
		for _, ids := range idStrArray {
			num, err := strconv.Atoi(ids)
			if err != nil {
				return nil, fmt.Errorf("转换错误: %w", err)
			}
			idIntArray = append(idIntArray, num)
		}
		formatLinesId := FormatLinesIdsConfig{
			Xy:  v.Xy,
			Ids: idIntArray,
		}
		formatLinesIds = append(formatLinesIds, &formatLinesId)
	}
	return formatLinesIds, nil
}
