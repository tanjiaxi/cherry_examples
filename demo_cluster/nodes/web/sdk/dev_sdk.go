/*
 * @Author: t 921865806@qq.com
 * @Date: 2025-11-25 15:57:48
 * @LastEditors: t 921865806@qq.com
 * @LastEditTime: 2025-11-27 15:51:58
 * @FilePath: /examples/demo_cluster/nodes/web/sdk/dev_sdk.go
 * @Description: 这是默认设置,请设置`customMade`, 打开koroFileHeader查看配置 进行设置: https://github.com/OBKoro1/koro1FileHeader/wiki/%E9%85%8D%E7%BD%AE
 */
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
	//这里就是deviname ，开发时期，open_id和deviname一样
	//正常登陆应该第三方，facebook，类的返回open_id。然后创建DevAccount和user表
	accountId := rpcCenter.GetDevAccount(p.app, accountName, password)
	if accountId == "" {
		callback(code.LoginError, nil)
		return
	}

	callback(code.OK, map[string]string{
		"open_id":     accountId,
		"device_name": accountName,
	})
}

func (devSdk) PayCallback(_ *data.SdkRow, _ *cherryGin.Context) {
}
