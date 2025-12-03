/*
 * @Author: t 921865806@qq.com
 * @Date: 2025-12-02 17:49:48
 * @LastEditors: t 921865806@qq.com
 * @LastEditTime: 2025-12-02 21:04:24
 * @FilePath: /examples/demo_cluster/nodes/game/server/slots/spin_engine/result/gen_result1.go
 * @Description: 关卡抽象对象
 */
package result

type ResultInterface interface {
	GenResult() error
	GetGameMap() float64
	GetWinType() string
	GetWinLines() error
}
