/*
 * @Author: t 921865806@qq.com
 * @Date: 2025-11-17 09:53:50
 * @LastEditors: t 921865806@qq.com
 * @LastEditTime: 2026-01-05 17:50:35
 * @FilePath: /examples/demo_cluster/internal/component/pg_gorm/gorm_logger.go
 * @Description: 这是默认设置,请设置`customMade`, 打开koroFileHeader查看配置 进行设置: https://github.com/OBKoro1/koro1FileHeader/wiki/%E9%85%8D%E7%BD%AE
 */
package pggorm

import (
	"strings"

	clog "github.com/cherry-game/cherry/logger"
)

type gormLogger struct {
	log *clog.CherryLogger
}

func (l gormLogger) Printf(s string, i ...interface{}) {
	l.log.Warnf(strings.ReplaceAll(s, "\n", ""), i...)
}
