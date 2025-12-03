/*
 * @Author: t 921865806@qq.com
 * @Date: 2025-09-29 17:58:23
 * @LastEditors: t 921865806@qq.com
 * @LastEditTime: 2025-12-02 22:43:38
 * @FilePath: /examples/demo_cluster/nodes/game/server/slots/core/component.go
 * @Description: 这是默认设置,请设置`customMade`, 打开koroFileHeader查看配置 进行设置: https://github.com/OBKoro1/koro1FileHeader/wiki/%E9%85%8D%E7%BD%AE
 */
package db

import (
	cherryFacade "github.com/cherry-game/cherry/facade"

	"github.com/cherry-game/examples/demo_cluster/nodes/game/server/slots/spin_engine/bonus"
	"github.com/cherry-game/examples/demo_cluster/nodes/game/server/slots/spin_engine/collect"
	"github.com/cherry-game/examples/demo_cluster/nodes/game/server/slots/spin_engine/machine"
	"github.com/cherry-game/examples/demo_cluster/nodes/game/server/slots/spin_engine/result"
)

type Component struct {
	cherryFacade.Component
}

func (c *Component) Name() string {
	return "slots_level_component"
}

// Init 组件初始化函数
// 为了简化部署的复杂性，本示例取消了数据库连接相关的逻辑

func (c *Component) OnAfterInit() {
	//注册machine
	machine.RegisterMachineAll()
	result.RegisteAllGenReslt()
	bonus.RegisteAllBonus()
	collect.RegisteAllCollect()
}

func New() *Component {
	return &Component{}
}
