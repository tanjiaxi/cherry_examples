/*
 * @Author: t 921865806@qq.com
 * @Date: 2025-12-21 19:05:10
 * @LastEditors: t 921865806@qq.com
 * @LastEditTime: 2025-12-21 19:45:10
 * @FilePath: /examples/demo_cluster/internal/model/player_room_data.go
 * @Description: 这是默认设置,请设置`customMade`, 打开koroFileHeader查看配置 进行设置: https://github.com/OBKoro1/koro1FileHeader/wiki/%E9%85%8D%E7%BD%AE
 */
package model

// // PlayerRoomData 玩家房间数据（用于数据库持久化）
// type PlayerRoomData struct {
// 	ID        int64  `gorm:"primaryKey;autoIncrement" json:"id"`
// 	UserId    int32  `gorm:"index:idx_user_room,unique;not null" json:"user_id"`
// 	RoomId    int32  `gorm:"index:idx_user_room,unique;not null" json:"room_id"`
// 	Data      string `gorm:"type:text" json:"data"`           // JSON序列化的房间数据
// 	Version   int    `gorm:"default:1" json:"version"`        // 乐观锁版本号
// 	CreatedAt int64  `gorm:"autoCreateTime" json:"created_at"`
// 	UpdatedAt int64  `gorm:"autoUpdateTime" json:"updated_at"`
// }

// func (PlayerRoomData) TableName() string {
// 	return "player_room_data"
// }

// // RoomDataSnapshot 房间数据快照（用于JSON序列化）
// type RoomDataSnapshot struct {
// 	CurBetNum       int64 `json:"cur_bet_num"`
// 	SpeSpinBet      int64 `json:"spe_spin_bet"`
// 	Stage           int   `json:"stage"`
// 	NextStage       int   `json:"next_stage"`
// 	FreeSpinNum     int   `json:"free_spin_num"`
// 	SpinNum         int   `json:"spin_num"`
// 	SeedNormalCount int   `json:"seed_normal_count"`
// 	SeedTmpCount    int   `json:"seed_tmp_count"`
// 	UserReelLevel   int   `json:"user_reel_level"`
// 	ReelLevelType   int   `json:"reel_level_type"`
// 	NewJackpotAcc   int   `json:"new_jac
