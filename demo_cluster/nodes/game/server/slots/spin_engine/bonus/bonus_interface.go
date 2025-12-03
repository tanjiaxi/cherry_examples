/*
 * @Author: t 921865806@qq.com
 * @Date: 2025-12-02 21:33:46
 * @LastEditors: t 921865806@qq.com
 * @LastEditTime: 2025-12-02 22:08:33
 * @FilePath: /examples/demo_cluster/nodes/game/server/slots/spin_engine/bonus/bonus_interface.go
 * @Description: bonus interface
 */
package bonus

type BonusInterface interface {
	InitData()
	CovertResult()
	ConvertBonus()
}
