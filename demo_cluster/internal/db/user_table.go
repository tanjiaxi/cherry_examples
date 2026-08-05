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
	"context"

	"github.com/cherry-game/examples/demo_cluster/internal/component/db"
	gameModel "github.com/cherry-game/examples/demo_cluster/internal/model"
)

// GetUserAllInfo 查询用户完整信息。
//
// 使用 First 而不是 Find：First 在没有记录时会返回
// gorm.ErrRecordNotFound；Find 则会返回一个 UserID=0 的零值对象，
// 上层无法区分“用户不存在”和“查询成功但数据为空”。
// context 从 Actor handler 向下传递，使请求取消和超时能够终止 SQL。
func GetUserAllInfo(ctx context.Context, userID int32) (*gameModel.SlotsUser, error) {
	var user gameModel.SlotsUser
	result := db.GetDB().
		WithContext(ctx).
		Where("user_id = ?", userID).
		First(&user)
	if result.Error != nil {
		return nil, result.Error
	}

	return &user, nil
}
