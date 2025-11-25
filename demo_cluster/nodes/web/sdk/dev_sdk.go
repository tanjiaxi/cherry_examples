/*
 * @Author: t 921865806@qq.com
 * @Date: 2025-11-25 15:57:48
 * @LastEditors: t 921865806@qq.com
 * @LastEditTime: 2025-11-25 16:28:41
 * @FilePath: /examples/demo_cluster/nodes/web/sdk/dev_sdk.go
 * @Description: 这是默认设置,请设置`customMade`, 打开koroFileHeader查看配置 进行设置: https://github.com/OBKoro1/koro1FileHeader/wiki/%E9%85%8D%E7%BD%AE
 */
package sdk

import (
	cherryError "github.com/cherry-game/cherry/error"
	cherryString "github.com/cherry-game/cherry/extend/string"
	cfacade "github.com/cherry-game/cherry/facade"
	cherryGin "github.com/cherry-game/components/gin"
	"github.com/cherry-game/examples/demo_cluster/internal/code"
	"github.com/cherry-game/examples/demo_cluster/internal/data"
	rpcCenter "github.com/cherry-game/examples/demo_cluster/internal/rpc/center"
)

type devSdk struct {
	app cfacade.IApplication
}

func (devSdk) SdkId() int32 {
	return DevMode
}

func (p devSdk) Login(_ *data.SdkRow, params Params, callback Callback) {
	accountName, _ := params.GetString("account")
	password, _ := params.GetString("password")

	if accountName == "" || password == "" {
		err := cherryError.Errorf("account or password params is empty.")
		callback(code.LoginError, nil, err)
		return
	}

	accountId := rpcCenter.GetDevAccount(p.app, accountName, password)
	if accountId < 1 {
		callback(code.LoginError, nil)
		return
	}

	callback(code.OK, map[string]string{
		"open_id":     cherryString.ToString(accountId),
		"device_name": accountName,
	})
}

func (devSdk) PayCallback(_ *data.SdkRow, _ *cherryGin.Context) {
}
