/*
 * @Author: t 921865806@qq.com
 * @Date: 2025-09-15 18:02:10
 * @LastEditors: t 921865806@qq.com
 * @LastEditTime: 2025-11-24 17:11:28
 * @FilePath: /examples/demo_cluster/nodes/game/db/component.go
 * @Description: 这是默认设置,请设置`customMade`, 打开koroFileHeader查看配置 进行设置: https://github.com/OBKoro1/koro1FileHeader/wiki/%E9%85%8D%E7%BD%AE
 */
package db

import (
	cherryUtils "github.com/cherry-game/cherry/extend/utils"
	cherryFacade "github.com/cherry-game/cherry/facade"
	cherryLogger "github.com/cherry-game/cherry/logger"
)

var (
	onLoadFuncList []func() // db初始化时加载函数列表
)

type Component struct {
	cherryFacade.Component
}

func (c *Component) Name() string {
	return "db_game_component"
}

// Init 组件初始化函数
// 为了简化部署的复杂性，本示例取消了数据库连接相关的逻辑
func (c *Component) Init() {
	InitDatabase(c.App())
}

func (c *Component) OnAfterInit() {
	for _, fn := range onLoadFuncList {
		cherryUtils.Try(fn, func(errString string) {
			cherryLogger.Warnf(errString)
		})
	}
}

func (*Component) OnStop() {
	//组件停止时触发逻辑
}

func New() *Component {
	return &Component{} // register db center
}

func addOnload(fn func()) {
	onLoadFuncList = append(onLoadFuncList, fn)
}
