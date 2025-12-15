/*
 * @Author: t 921865806@qq.com
 * @Date: 2025-11-20 22:33:18
 * @LastEditors: t 921865806@qq.com
 * @LastEditTime: 2025-12-09 22:05:45
 * @FilePath: /examples/demo_cluster/nodes/game/server/ slots/component/level_data_types.go
 * @Description: 这是默认设置,请设置`customMade`, 打开koroFileHeader查看配置 进行设置: https://github.com/OBKoro1/koro1FileHeader/wiki/%E9%85%8D%E7%BD%AE
 */
package model

import (
	"fmt"
	"sync"
	"time"

	"github.com/jinzhu/copier"
)

// level数据结构定义
type RoomDataInfo struct {
	//基础信息
	UserId int32 `json:"user_id"` //用户id
	RoomId int32 `json:"room_id"` //关卡id
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
	StageType     int `json:"stage_type"`      // 阶段类型,一般就是reSpoin使用
	LastReelLevel int `json:"last_reel_level"` // 上一次卷轴等级

	// 元数据（不在原始数据中，但需要）
	CreatedAt    int64 `json:"created_at"`    // 创建时间
	UpdatedAt    int64 `json:"updated_at"`    // 更新时间
	Version      int   `json:"version"`       // 版本号（乐观锁）
	IsDirty      bool  `json:"-"`             // 脏数据标记（不序列化）
	RecommendBet int64 `json:"recommend_bet"` //推荐下注额
}

const (
	_                = iota // 使用 _ 跳过 0
	NORMAL                  // 1
	COLLECTION_BONUS        // 2
	COLLECTION_FS           //3
	BONUS                   //4
	FREE_SPIN               //5
	RE_SPIN                 //6
)
const (
	RE_SPIN_NORMAL = iota //  0
	RE_SPIN_ING           //1
)

// 创建浅拷贝，如果字段都是int，string类的没有map，slice类可以
func (r *RoomDataInfo) Clone() *RoomDataInfo {
	if r == nil {
		return nil
	}
	clone := *r
	return &clone
}

// 复制数据，参拷贝，手动性能最好，但是需要手动写
func (r *RoomDataInfo) CopyFrom(otherData *RoomDataInfo) (*RoomDataInfo, error) {
	if r == nil {
		return nil, fmt.Errorf("RoomDataInfo Data is nil")
	}
	var clone RoomDataInfo
	clone.UserId = otherData.UserId
	clone.RoomId = otherData.RoomId
	clone.CurBetNum = otherData.CurBetNum
	clone.SpeSpinBet = otherData.SpeSpinBet

	clone.Stage = otherData.Stage
	clone.NextStage = otherData.NextStage
	clone.FreeSpinNum = otherData.FreeSpinNum
	clone.SpinNum = otherData.SpinNum
	//还没有写完
	return &clone, nil
}

// 使用copier库
func (r *RoomDataInfo) CopierToData() (*RoomDataInfo, error) {
	//内存池里面拿一个
	dst := RoomDataPool.Get().(*RoomDataInfo)
	err := copier.Copy(dst, r)
	if err != nil {
		return nil, fmt.Errorf("Copier Data  failed")
	}
	return dst, nil
}

// MarkDirty 标记为脏数据
func (r *RoomDataInfo) MarkDirty() {
	r.IsDirty = true
	r.UpdatedAt = time.Now().Unix()
}

// ClearDirty 清除脏标记
func (r *RoomDataInfo) ClearDirty() {
	r.IsDirty = false
}
func (r *RoomDataInfo) Reset() {
	r.RoomId = 0
	r.UserId = 0
	r.CurBetNum = 0
	//没写完
	// 如果有 Map，通常建议直接 make new map 或者清空 key
	// Go 1.21+ 可以使用 clear(r.MyMap)
	// r.MyMap = nil
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

// 1.这是一个全局的 定义全局或结构体级别的对象池
var RoomDataPool = sync.Pool{
	New: func() interface{} {
		// 创建一个新的，初始化内部的切片/Map以避免后续频繁扩容
		return &RoomDataInfo{}
	},
}
