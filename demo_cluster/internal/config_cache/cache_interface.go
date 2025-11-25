/*
 * @Author: t 921865806@qq.com
 * @Date: 2025-11-21 17:44:08
 * @LastEditors: t 921865806@qq.com
 * @LastEditTime: 2025-11-24 14:24:33
 * @FilePath: /examples/demo_cluster/internal/config_cache/cache_interface.go
 * @Description: 这是默认设置,请设置`customMade`, 打开koroFileHeader查看配置 进行设置: https://github.com/OBKoro1/koro1FileHeader/wiki/%E9%85%8D%E7%BD%AE
 */
package configcache

type IConfigCenter interface {
	Reload() error
}
