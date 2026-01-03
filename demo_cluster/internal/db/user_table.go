/*
 * @Author: t 921865806@qq.com
 * @Date: 2025-11-28 14:43:28
 * @LastEditors: t 921865806@qq.com
 * @LastEditTime: 2025-12-30 16:21:58
 * @FilePath: /examples/demo_cluster/internal/db/user_table.go
 * @Description: 这是默认设置,请设置`customMade`, 打开koroFileHeader查看配置 进行设置: https://github.com/OBKoro1/koro1FileHeader/wiki/%E9%85%8D%E7%BD%AE
 */
package db

import (
	"github.com/cherry-game/examples/demo_cluster/internal/component/db"
	gameModel "github.com/cherry-game/examples/demo_cluster/internal/model"
)

func GetUserAllInfo(userId int32) *gameModel.SlotsUser {
	var user gameModel.SlotsUser
	result := db.GetDB().Where("user_id = ?", userId).Find(&user)
	if result.Error != nil {
		return nil
	}
	return &user
}
