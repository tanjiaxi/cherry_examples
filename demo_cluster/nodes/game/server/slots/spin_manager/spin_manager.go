/*
 * @Author: t 921865806@qq.com
 * @Date: 2025-11-20 23:46:24
 * @LastEditors: t 921865806@qq.com
 * @LastEditTime: 2025-12-02 21:29:45
 * @FilePath: /examples/demo_cluster/nodes/game/server/ slots/component/spin_manager.go
 * @Description: 这是进入spin，前，后的数据获取和处理。 （玩家赔率的控制，产生的数据，处理，管理关卡的数据转换提供给关卡逻辑）
 */
package spinmanage

// 组件层（全局单例）
type SpinManager struct {
}

func (s *SpinManager) Init() {

}

func SpinBefore() {}
func SpinAfter()  {}
func SpinEnd()    {}
func SpinStart()  {}
