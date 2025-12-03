/*
 * @Author: t 921865806@qq.com
 * @Date: 2025-12-02 21:33:45
 * @LastEditors: t 921865806@qq.com
 * @LastEditTime: 2025-12-02 21:48:47
 * @FilePath: /examples/demo_cluster/nodes/game/server/slots/spin_engine/collect/collect_interface.go
 * @Description: 这是默认设置,请设置`customMade`, 打开koroFileHeader查看配置 进行设置: https://github.com/OBKoro1/koro1FileHeader/wiki/%E9%85%8D%E7%BD%AE
 */
package collect

type CollectInterface interface {
	InitData() error
	DispatchFreeSpin() error
	DispatchBonus() error
	CollectFreeSpin() error
	CollectBonus() error
}
