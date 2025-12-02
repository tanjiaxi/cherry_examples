/*
 * @Author: t 921865806@qq.com
 * @Date: 2025-12-01 23:24:04
 * @LastEditors: t 921865806@qq.com
 * @LastEditTime: 2025-12-01 23:36:35
 * @FilePath: /examples/demo_cluster/nodes/game/server/slots/core/data_core.go
 * @Description: 这是默认设置,请设置`customMade`, 打开koroFileHeader查看配置 进行设置: https://github.com/OBKoro1/koro1FileHeader/wiki/%E9%85%8D%E7%BD%AE
 */
package core

func FormatUserBetArr(betArr []int) []int {
	var betArr2 []int
	for _, v := range betArr {
		if v > 0 {
			betArr2 = append(betArr2, v)
		}
	}
	return betArr2
}
