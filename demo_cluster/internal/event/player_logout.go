package event

type PlayerLogout struct {
	ActorId string // actor id
	UserId  int64  // player id
}

func NewPlayerLogout(actorId string, playerId int64) PlayerLogout {
	event := PlayerLogout{
		ActorId: actorId,
		UserId:  playerId,
	}
	return event
}

func (PlayerLogout) Name() string {
	return PlayerLogoutKey
}

func (p PlayerLogout) UniqueID() int64 {
	return p.UserId
}
