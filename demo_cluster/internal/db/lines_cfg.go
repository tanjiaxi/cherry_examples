/*
 * @Author: t 921865806@qq.com
 * @Date: 2025-12-03 18:11:19
 * @LastEditors: t 921865806@qq.com
 * @LastEditTime: 2025-12-04 00:31:38
 * @FilePath: /examples/demo_cluster/internal/db/lines_cfg.go
 * @Description: 这是默认设置,请设置`customMade`, 打开koroFileHeader查看配置 进行设置: https://github.com/OBKoro1/koro1FileHeader/wiki/%E9%85%8D%E7%BD%AE
 */
package db

import (
	"reflect"
	"strings"

	"github.com/cherry-game/examples/demo_cluster/internal/component/db"
)

type line struct {
	x int
	y int
}
type CommonLines struct {
	ID       int
	LinesArr []line
}
type CommonLinesConfig struct {
	ID   int32
	Line string
}

func GetLinesConfig(lines []interface{}, x, y int) (commonLines []*CommonLines, err error) {
	//  var lines []linesMod
	result := db.GetDB().Find(&lines)
	if result.Error != nil {
		return nil, result.Error
	}
	for _, v := range lines {
		val := reflect.ValueOf(v).Elem() // 获取指针指向的值
		lines, err := FormatLinesConfig(x, y, val.FieldByName("Line").String())
		if err != nil {
			return nil, err
		}
		common := &CommonLines{
			ID:       int(val.Int()), // Int() 返回 int64，强转为 int32
			LinesArr: lines,
		}
		commonLines = append(commonLines, common)
	}
	return commonLines, nil
}
func FormatLinesConfig(x, y int, lines string) ([]line, error) {
	linesArr := strings.Split(strings.TrimSpace(lines), ",")
	formatLines := make([]line, 0, len(linesArr))
	for i := 0; i < y; i++ {
		for j := 0; j < x; j++ {
			if linesArr[j*y+i] == "1" {
				line := line{
					x: j,
					y: i,
				}
				formatLines = append(formatLines, line)
			}
		}
	}
	return formatLines, nil
}
