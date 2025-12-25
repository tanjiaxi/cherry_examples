/*
 * @Author: t 921865806@qq.com
 * @Date: 2025-09-15 18:02:10
 * @LastEditors: t 921865806@qq.com
 * @LastEditTime: 2025-12-21 21:42:28
 * @FilePath: /examples/demo_cluster/internal/data/area_config.go
 * @Description: 这是默认设置,请设置`customMade`, 打开koroFileHeader查看配置 进行设置: https://github.com/OBKoro1/koro1FileHeader/wiki/%E9%85%8D%E7%BD%AE
 */
package data

import (
	cherryError "github.com/cherry-game/cherry/error"
	cherryLogger "github.com/cherry-game/cherry/logger"
)

type (
	AreaRow struct {
		AreaId      int32    `json:"areaId"`      // 游戏区id
		AreaName    string   `json:"areaName"`    // 游戏区名称
		GateNodes   []string `json:"gateNodes"`   // 该区的Gate节点ID列表
		DefaultGate string   `json:"defaultGate"` // 默认Gate地址（后备）
		Gate        string   `json:"gate"`        // 兼容旧配置
	}

	// 游戏区
	areaConfig struct {
		maps map[int32]*AreaRow
	}
)

// Name 根据名称读取 ./config/data/areaConfig.json文件
func (p *areaConfig) Name() string {
	return "areaConfig"
}

func (p *areaConfig) Init() {
	p.maps = make(map[int32]*AreaRow)
}

func (p *areaConfig) OnLoad(maps interface{}, _ bool) (int, error) {
	list, ok := maps.([]interface{})
	if !ok {
		return 0, cherryError.Error("maps convert to []interface{} error.")
	}

	loadMaps := make(map[int32]*AreaRow)
	for index, data := range list {
		loadConfig := &AreaRow{}
		err := DecodeData(data, loadConfig)
		if err != nil {
			cherryLogger.Warnf("decode error. [row = %d, %v], err = %s", index+1, loadConfig, err)
			continue
		}

		loadMaps[loadConfig.AreaId] = loadConfig
	}

	p.maps = loadMaps

	return len(list), nil
}

func (p *areaConfig) OnAfterLoad(_ bool) {
}

func (p *areaConfig) Get(pk int32) (*AreaRow, bool) {
	i, found := p.maps[pk]
	return i, found
}

func (p *areaConfig) Contain(pk int32) bool {
	_, found := p.Get(pk)
	return found
}
