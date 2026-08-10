package sessionKey

const (
	// AreaServerID 逻辑服 ID（LoginRequest.serverId / areaServerConfig.serverId）
	AreaServerID = "area_server_id"
	// GameNodeID Game 进程节点 ID（路由用，来自 AllocateNodes）
	GameNodeID = "game_node_id"
	// ServerID 废弃别名：历史代码里被误写成 gameNodeId。
	// 新代码禁止再写入业务逻辑服；读路由时仅作兼容 fallback。
	ServerID = "server_id" // int32 游戏服务器ID
	OpenID   = "open_id"   // string 第三方登陆sdk的用户唯一标识
	PID      = "pid"       // int32 sdk包id
	PlayerID = "player_id" // int64 玩家id
)
