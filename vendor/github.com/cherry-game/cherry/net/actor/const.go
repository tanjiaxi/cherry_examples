/*
 * @Author: t 921865806@qq.com
 * @Date: 2025-12-15 22:06:17
 * @LastEditors: t 921865806@qq.com
 * @LastEditTime: 2025-12-23 15:40:14
 * @FilePath: /examples/vendor/github.com/cherry-game/cherry/net/actor/const.go
 * @Description: 这是默认设置,请设置`customMade`, 打开koroFileHeader查看配置 进行设置: https://github.com/OBKoro1/koro1FileHeader/wiki/%E9%85%8D%E7%BD%AE
 */
package cherryActor

import (
	cerror "github.com/cherry-game/cherry/error"
)

var (
	ErrForbiddenToCallSelf       = cerror.Errorf("SendActorID cannot be equal to TargetActorID")
	ErrForbiddenCreateChildActor = cerror.Errorf("Forbidden create child actor")
	ErrActorIDIsNil              = cerror.Error("actorID is nil.")
)

const (
	LocalName  = "local"
	RemoteName = "remote"
)
