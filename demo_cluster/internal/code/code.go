/*
 * @Author: t 921865806@qq.com
 * @Date: 2025-09-15 18:02:10
 * @LastEditors: t 921865806@qq.com
 * @LastEditTime: 2025-12-14 21:10:10
 * @FilePath: /examples/demo_cluster/internal/code/code.go
 * @Description: 这是默认设置,请设置`customMade`, 打开koroFileHeader查看配置 进行设置: https://github.com/OBKoro1/koro1FileHeader/wiki/%E9%85%8D%E7%BD%AE
 */
package code

var (
	OK                       int32 = 0   // is ok
	Error                    int32 = 1   // error
	ParamError               int32 = 2   // 参数错误
	PIDError                 int32 = 100 // pid错误
	LoginError               int32 = 201 // 登录异常
	AccountAuthFail          int32 = 202 // 帐号授权失败
	AccountBindFail          int32 = 203 // 帐号绑定失败
	AccountTokenValidateFail int32 = 204 // token验证失败
	AccountNameIsExist       int32 = 205 // 帐号已存在
	AccountRegisterError     int32 = 206 //
	AccountGetFail           int32 = 207 //
	PlayerDenyLogin          int32 = 301 // 玩家禁止登录
	PlayerDuplicateLogin     int32 = 302 // 玩家重复登录
	PlayerNameExist          int32 = 303 // 玩家角色名已存在
	PlayerCreateFail         int32 = 304 // 玩家创建角色失败
	PlayerNotLogin           int32 = 305 // 玩家未登录
	PlayerIDError            int32 = 306 // 玩家id错误
	ServerMaintenance        int32 = 307 //服务器错误
	NoAvailableGameServer    int32 = 308 //无法获取游戏服务器
	PlayerNoUserInfo         int32 = 309 //玩家不存在
	NoRoomConfig             int32 = 310 //无法获取房间配置
	NoRoomPlayerData         int32 = 311 //无法获取房间玩家数据
	UpdateRoomPlayerDataFial int32 = 312 //更新房间玩家数据失败
	GetRulstInfoError        int32 = 313 //获取房间信息失败

	// 节点分配相关错误码
	NoAvailableGate        int32 = 401 // 无可用Gate节点
	NoAvailableGame        int32 = 402 // 无可用Game节点
	AllocateNodeFail       int32 = 403 // 分配节点失败
	PlayerLocationNotFound int32 = 404 // 玩家位置未找到
	RemoveLocationFail     int32 = 405 // 移除位置失败
)

// 临时变量
var (
	CostCoinsM float64 = 1.5
	Schama     string  = "public"
)
