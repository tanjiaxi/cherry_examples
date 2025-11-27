/*
 * @Author: t 921865806@qq.com
 * @Date: 2025-11-20 22:33:18
 * @LastEditors: t 921865806@qq.com
 * @LastEditTime: 2025-11-26 23:00:02
 * @FilePath: /examples/demo_cluster/nodes/game/server/ slots/component/level_data_types.go
 * @Description: 这是默认设置,请设置`customMade`, 打开koroFileHeader查看配置 进行设置: https://github.com/OBKoro1/koro1FileHeader/wiki/%E9%85%8D%E7%BD%AE
 */
package spinmanage

import "fmt"

// level数据结构定义
type RoomDataInfo struct {
	//基础信息
	UserId int `json:"user_id"` //用户id
	RoomId int `json:"room_id"` //关卡id
	//下注信息
	CurBetNum  int64 `json:"cur_bet_num"`  //当前下注数
	SpeSpinBet int64 `json:"spe_spin_bet"` // 特殊Spin下注额
	//游戏阶段
	Stage     int `json:"stage"`      //游戏阶段
	NextStage int `json:"next_stage"` //下一个阶段
	// FreeSpin相关
	FreeSpinNum int `json:"free_spin_num"` //剩余FreeSpin次数
	SpinNum     int `json:"spin_num"`      //总FreeSpin次数
	//随机数种子计数
	SeedNormalCount int `json:"seed_normal_count"` // 普通种子计数
	SeedTmpCount    int `json:"seed_tmp_count"`    // 临时种子计数
	// 卷轴等级
	UserReelLevel int `json:"user_reel_level"` // 用户卷轴等级
	ReelLevelType int `json:"reel_level_type"` // 卷轴等级类型

	// Jackpot相关
	NewJackpotAcc int `json:"new_jackpot_acc"` // 新玩家Jackpot标志

	// 元数据（不在原始数据中，但需要）
	CreatedAt    int64 `json:"created_at"`    // 创建时间
	UpdatedAt    int64 `json:"updated_at"`    // 更新时间
	Version      int   `json:"version"`       // 版本号（乐观锁）
	IsDirty      bool  `json:"-"`             // 脏数据标记（不序列化）
	RecommendBet int64 `json:"recommend_bet"` //推荐下注额
}
type SessionKey struct {
	UserID int `json:"user_id"`
	RoomID int `json:"room_id"`
}

func (k SessionKey) String() string {
	return fmt.Sprintf("%d:%d", k.UserID, k.RoomID)
}

type SpinContext struct {
	Session   *RoomDataInfo
	Bet       int64 //本次下注
	TimeStamp int64 //时间戳
}
