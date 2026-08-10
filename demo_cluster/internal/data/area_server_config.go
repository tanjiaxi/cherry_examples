package data

import (
	cherryError "github.com/cherry-game/cherry/error"
	cherryLogger "github.com/cherry-game/cherry/logger"
)

type (
	// AreaServerRow 逻辑服：选服页的「服」，创角/合服单位；绑定 Game 进程池
	AreaServerRow struct {
		ServerId   int32    `json:"serverId"` // 逻辑服ID（对玩家可见，≠ gameNodeId）
		ServerName string   `json:"serverName"`
		AreaId     int32    `json:"areaId"`    // 所属大区
		GameNodes  []string `json:"gameNodes"` // Game 节点ID列表（集群 node_id）
		Status     int32    `json:"status"`    // 1=开服 0=维护
	}

	areaServerConfig struct {
		maps map[int32]*AreaServerRow
	}
)

func (p *areaServerConfig) Name() string { return "areaServerConfig" }

func (p *areaServerConfig) Init() {
	p.maps = make(map[int32]*AreaServerRow)
}

func (p *areaServerConfig) OnLoad(maps interface{}, _ bool) (int, error) {
	list, ok := maps.([]interface{})
	if !ok {
		return 0, cherryError.Error("maps convert to []interface{} error.")
	}

	loadMaps := make(map[int32]*AreaServerRow)
	for index, raw := range list {
		row := &AreaServerRow{}
		if err := DecodeData(raw, row); err != nil {
			cherryLogger.Warnf("decode error. [row = %d, %v], err = %s", index+1, row, err)
			continue
		}
		loadMaps[row.ServerId] = row
	}
	p.maps = loadMaps
	return len(list), nil
}

func (p *areaServerConfig) OnAfterLoad(_ bool) {}

func (p *areaServerConfig) Get(pk int32) (*AreaServerRow, bool) {
	i, found := p.maps[pk]
	return i, found
}

func (p *areaServerConfig) Contain(pk int32) bool {
	_, found := p.Get(pk)
	return found
}

func (p *areaServerConfig) ListWithAreaId(areaId int32) []*AreaServerRow {
	var list []*AreaServerRow
	for _, row := range p.maps {
		if row.AreaId == areaId {
			list = append(list, row)
		}
	}
	return list
}

// ListOpenWithAreaId 仅返回开服中的逻辑服（Web 列表用）
func (p *areaServerConfig) ListOpenWithAreaId(areaId int32) []*AreaServerRow {
	var list []*AreaServerRow
	for _, row := range p.maps {
		if row.AreaId == areaId && row.Status == 1 {
			list = append(list, row)
		}
	}
	return list
}

// GameNodesOf 取逻辑服绑定的 Game 进程列表（Center 分配用）
func (p *areaServerConfig) GameNodesOf(serverId int32) ([]string, bool) {
	row, ok := p.Get(serverId)
	if !ok || len(row.GameNodes) == 0 {
		return nil, false
	}
	return row.GameNodes, true
}
