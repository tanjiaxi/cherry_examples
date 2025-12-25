/*
 * @Description: N2CfgRoomlist 只读配置接口
 * 业务层只能通过接口访问配置，无法直接修改
 */
package slots

import (
	gameModel "github.com/cherry-game/examples/demo_cluster/internal/model"
)

// IRoomListConfig 房间配置只读接口
// 业务层只能通过此接口访问配置数据，无法修改原始配置
type IRoomListConfig interface {
	// 基础信息
	GetKid() int32
	GetID() int32
	GetRoomID() int32
	GetRoomName() string
	GetRoomIndex() int32

	// 解锁条件
	GetLevelUnlock() int32
	GetUnlockType() int32
	GetVipFirst() int32

	// 显示相关
	GetRecommendIconFunlabel() int32
	GetRoomIconShowlable() int32
	GetRoomShowstate() int32
	GetIconResAddr() string
	GetGameResAddr() string

	// 棋盘配置
	GetAxisX() float64
	GetAxisY() float64
	GetLineMin() int32
	GetLineMax() int32
	GetLinesequence() string
	GetIslinevisible() int32

	// 下注配置
	GetBetbaseamount() int64
	GetBetDisplay() string
	GetBetDisplay2() string
	GetBetDisplayButtonType() int32
	GetBetUnlockCondition() string
	GetBetStakegear() int32
	GetCoinsbetRequied() string

	// 中奖阈值
	GetSfxMidwin() int32
	GetSfxBigwin() int32
	GetBigthreshold() int64
	GetMegathreshold() int64
	GetSuperthreshold() int64
	GetEpicthreshold() int64

	// FreeSpin 配置
	GetRoundawardspin() int64
	GetIfGetmorespin() int32
	GetFreespinTimesRange() int32
	GetFreespinMultiplier() int32

	// 收集配置
	GetCardCollectID() int32
	GetCollectnumrequied() int32
	GetCardsequence() string
	GetCollectAmount() string
	GetCoinsRequied() string
	GetCollectPricerate() string
	GetCollectConvertrate() string
	GetCollectTimeMultiplier() string
	GetCollectRequiedtime() string

	// 成就配置
	GetAcquiredSpinTimes() string
	GetAcquired5Ofkind() string
	GetAcquired33() string

	// 其他配置
	GetTournamentid() int32
	GetAccessTimeRange() string
	GetBillingID() string
	GetWildRange() string
	GetVersion() int32
	GetIsGlobalJackpot() string
	GetNote() string
	GetChooseNum() int16
}

// roomListConfigImpl 内部实现，包装原始配置
type roomListConfigImpl struct {
	data *gameModel.N2CfgRoomlist
}

// NewRoomListConfig 创建只读配置包装器
func NewRoomListConfig(data *gameModel.N2CfgRoomlist) IRoomListConfig {
	if data == nil {
		return nil
	}
	return &roomListConfigImpl{data: data}
}

// ========== 基础信息 ==========

func (r *roomListConfigImpl) GetKid() int32 {
	return r.data.Kid
}

func (r *roomListConfigImpl) GetID() int32 {
	return r.data.ID
}

func (r *roomListConfigImpl) GetRoomID() int32 {
	return r.data.RoomID
}

func (r *roomListConfigImpl) GetRoomName() string {
	return r.data.RoomName
}

func (r *roomListConfigImpl) GetRoomIndex() int32 {
	return r.data.RoomIndex
}

// ========== 解锁条件 ==========

func (r *roomListConfigImpl) GetLevelUnlock() int32 {
	return r.data.LevelUnlock
}

func (r *roomListConfigImpl) GetUnlockType() int32 {
	return r.data.UnlockType
}

func (r *roomListConfigImpl) GetVipFirst() int32 {
	return r.data.VipFirst
}

// ========== 显示相关 ==========

func (r *roomListConfigImpl) GetRecommendIconFunlabel() int32 {
	return r.data.RecommendIconFunlabel
}

func (r *roomListConfigImpl) GetRoomIconShowlable() int32 {
	return r.data.RoomIconShowlable
}

func (r *roomListConfigImpl) GetRoomShowstate() int32 {
	return r.data.RoomShowstate
}

func (r *roomListConfigImpl) GetIconResAddr() string {
	return r.data.IconResAddr
}

func (r *roomListConfigImpl) GetGameResAddr() string {
	return r.data.GameResAddr
}

// ========== 棋盘配置 ==========

func (r *roomListConfigImpl) GetAxisX() float64 {
	return r.data.Axisx
}

func (r *roomListConfigImpl) GetAxisY() float64 {
	return r.data.Axisy
}

func (r *roomListConfigImpl) GetLineMin() int32 {
	return r.data.LineMin
}

func (r *roomListConfigImpl) GetLineMax() int32 {
	return r.data.LineMax
}

func (r *roomListConfigImpl) GetLinesequence() string {
	return r.data.Linesequence
}

func (r *roomListConfigImpl) GetIslinevisible() int32 {
	return r.data.Islinevisible
}

// ========== 下注配置 ==========

func (r *roomListConfigImpl) GetBetbaseamount() int64 {
	return r.data.Betbaseamount
}

func (r *roomListConfigImpl) GetBetDisplay() string {
	return r.data.BetDisplay
}

func (r *roomListConfigImpl) GetBetDisplay2() string {
	return r.data.BetDisplay2
}

func (r *roomListConfigImpl) GetBetDisplayButtonType() int32 {
	return r.data.BetDisplayButtonType
}

func (r *roomListConfigImpl) GetBetUnlockCondition() string {
	return r.data.BetUnlockCondition
}

func (r *roomListConfigImpl) GetBetStakegear() int32 {
	return r.data.BetStakegear
}

func (r *roomListConfigImpl) GetCoinsbetRequied() string {
	return r.data.CoinsbetRequied
}

// ========== 中奖阈值 ==========

func (r *roomListConfigImpl) GetSfxMidwin() int32 {
	return r.data.SfxMidwin
}

func (r *roomListConfigImpl) GetSfxBigwin() int32 {
	return r.data.SfxBigwin
}

func (r *roomListConfigImpl) GetBigthreshold() int64 {
	return r.data.Bigthreshold
}

func (r *roomListConfigImpl) GetMegathreshold() int64 {
	return r.data.Megathreshold
}

func (r *roomListConfigImpl) GetSuperthreshold() int64 {
	return r.data.Superthreshold
}

func (r *roomListConfigImpl) GetEpicthreshold() int64 {
	return r.data.Epicthreshold
}

// ========== FreeSpin 配置 ==========

func (r *roomListConfigImpl) GetRoundawardspin() int64 {
	return r.data.Roundawardspin
}

func (r *roomListConfigImpl) GetIfGetmorespin() int32 {
	return r.data.IfGetmorespin
}

func (r *roomListConfigImpl) GetFreespinTimesRange() int32 {
	return r.data.FreespinTimesRange
}

func (r *roomListConfigImpl) GetFreespinMultiplier() int32 {
	return r.data.FreespinMultiplier
}

// ========== 收集配置 ==========

func (r *roomListConfigImpl) GetCardCollectID() int32 {
	return r.data.CardCollectID
}

func (r *roomListConfigImpl) GetCollectnumrequied() int32 {
	return r.data.Collectnumrequied
}

func (r *roomListConfigImpl) GetCardsequence() string {
	return r.data.Cardsequence
}

func (r *roomListConfigImpl) GetCollectAmount() string {
	return r.data.CollectAmount
}

func (r *roomListConfigImpl) GetCoinsRequied() string {
	return r.data.CoinsRequied
}

func (r *roomListConfigImpl) GetCollectPricerate() string {
	return r.data.CollectPricerate
}

func (r *roomListConfigImpl) GetCollectConvertrate() string {
	return r.data.CollectConvertrate
}

func (r *roomListConfigImpl) GetCollectTimeMultiplier() string {
	return r.data.CollectTimeMultiplier
}

func (r *roomListConfigImpl) GetCollectRequiedtime() string {
	return r.data.CollectRequiedtime
}

// ========== 成就配置 ==========

func (r *roomListConfigImpl) GetAcquiredSpinTimes() string {
	return r.data.AcquiredSpinTimes
}

func (r *roomListConfigImpl) GetAcquired5Ofkind() string {
	return r.data.Acquired5Ofkind
}

func (r *roomListConfigImpl) GetAcquired33() string {
	return r.data.Acquired33
}

// ========== 其他配置 ==========

func (r *roomListConfigImpl) GetTournamentid() int32 {
	return r.data.Tournamentid
}

func (r *roomListConfigImpl) GetAccessTimeRange() string {
	return r.data.AccessTimeRange
}

func (r *roomListConfigImpl) GetBillingID() string {
	return r.data.BillingID
}

func (r *roomListConfigImpl) GetWildRange() string {
	return r.data.WildRange
}

func (r *roomListConfigImpl) GetVersion() int32 {
	return r.data.Version
}

func (r *roomListConfigImpl) GetIsGlobalJackpot() string {
	return r.data.IsGlobalJackpot
}

func (r *roomListConfigImpl) GetNote() string {
	return r.data.Note
}

func (r *roomListConfigImpl) GetChooseNum() int16 {
	return r.data.ChooseNum
}
