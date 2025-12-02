/*
 * @Author: t 921865806@qq.com
 * @Date: 2025-09-15 18:02:10
 * @LastEditors: t 921865806@qq.com
 * @LastEditTime: 2025-11-28 15:26:59
 * @FilePath: /examples/demo_cluster/nodes/game/db/component.go
 * @Description: 这是相当于组合所有所用数据库的中间件
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
	return "common_db_game_component"
}

// Init 组件初始化函数
// 为了简化部署的复杂性，本示例取消了数据库连接相关的逻辑
func (c *Component) Init() {
	//数据库初始化
	InitDatabase(c.App())
	//redis等
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
