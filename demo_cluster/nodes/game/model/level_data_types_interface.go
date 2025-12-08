package model

type LevelDataInterface interface {
	GetUserId() (int32, error)
	GetRoomId() (int32, error)
	GetCurBetNum() (int64, error)
	GetSpeSpinBet() (int64, error)
	GetStage() (int, error)
	GetNextStage() (int, error)
	GetFreeSpinNum() (int, error)
	GetSpinNum() (int, error)
	GetSeedNormalCount() (int, error)
	GetSeedTmpCount() (int, error)
	GetUserReelLevel() (int, error)
	GetReelLevelType() (int, error)
	GetNewJackpotAcc() (int, error)
	GetCreatedAt() (int64, error)
	GetUpdatedAt() (int64, error)
	GetVersion() (int, error)
	GetIsDirty() (bool, error)
	GetRecommendBet() (int64, error)
}
