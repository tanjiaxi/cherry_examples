/*
 * @Author: t 921865806@qq.com
 * @Date: 2025-12-02 17:49:48
 * @LastEditors: t 921865806@qq.com
 * @LastEditTime: 2025-12-02 21:10:45
 * @FilePath: /examples/demo_cluster/nodes/game/server/slots/spin_engine/result/gen_result1.go
 * @Description: 关卡基础类
 */
package result

type GenResultBase struct {
}

func NewGenResultBase() *GenResultBase {
	return &GenResultBase{}
}
func (g *GenResultBase) GenResult() error {
	return nil
}
func (g *GenResultBase) GetGameMap() float64 {
	return 0
}
func (g *GenResultBase) GetWinType() string {
	return ""
}
func (g *GenResultBase) GetWinLines() error {
	return nil
}
