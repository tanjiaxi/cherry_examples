/*
 * @Author: t 921865806@qq.com
 * @Date: 2025-11-20 23:45:18
 * @LastEditors: t 921865806@qq.com
 * @LastEditTime: 2025-12-17 22:41:54
 * @FilePath: /examples/demo_cluster/nodes/game/db/slots/data_center.go
 * @Description: 这是默认设置,请设置`customMade`, 打开koroFileHeader查看配置 进行设置: https://github.com/OBKoro1/koro1FileHeader/wiki/%E9%85%8D%E7%BD%AE
 */
package slots

import (
	"fmt"
	"sync"
	"sync/atomic"

	"github.com/DmitriyVTitov/size"
	clog "github.com/cherry-game/cherry/logger"
	dbData "github.com/cherry-game/examples/demo_cluster/internal/db" //具体数据
	logicGameModel "github.com/cherry-game/examples/demo_cluster/internal/model/logic_model"
	"github.com/jinzhu/copier"
)

type DataCenter struct {
	//automic.value 储存配置快照,可以热更
	snapshotAuto atomic.Value // *ConfigSnapshot
	reloadMu     sync.Mutex
	// 数据加载器
	loader *DataLoader
}

var (
	instance *DataCenter
	once     sync.Once
)

func GetInstance() *DataCenter {
	once.Do(func() {
		instance = &DataCenter{}
	})
	return instance
}

// SetLoader 设置数据加载器（用于测试）
func (dc *DataCenter) SetLoader(loader *DataLoader) {
	dc.loader = loader
}
func (dc *DataCenter) Init() *ConfigSnapshot {
	return dc.snapshotAuto.Load().(*ConfigSnapshot)
}
func (dc *DataCenter) getSnapshot() *ConfigSnapshot {
	return dc.snapshotAuto.Load().(*ConfigSnapshot)
}

// 更新配置
func (dc *DataCenter) Reload() error {
	dc.reloadMu.Lock()
	defer dc.reloadMu.Unlock()
	newonfigSnapshot, err := dc.loader.LoadAllConfig()
	if err != nil {
		return err
	}
	//原子替换
	dc.snapshotAuto.Store(newonfigSnapshot)
	// 1. 获取字节数
	bytesSize := size.Of(newonfigSnapshot)

	// 2. 转换为 MB (注意要转为 float64 以保留小数)
	// 1 MB = 1024 KB = 1024 * 1024 Bytes
	mbSize := float64(bytesSize) / (1024 * 1024)

	// 3. 打印，保留2位小数
	fmt.Printf("Deep size: %.2f MB\n", mbSize)

	// 如果想看详细对比：
	fmt.Printf("Bytes: %d, MB: %.4f\n", bytesSize, mbSize)
	return nil
}

// 获取card 配置
/*
roomID  规则房间ID 1，2，3
*/
func (dc *DataCenter) GetCardConfig(ruleId int32) (map[int32]*dbData.FormatCardConfig, error) {
	n2CfgCard := make(map[int32]*dbData.FormatCardConfig)
	if cfg, ok := dc.getSnapshot().RoomCardIndex[ruleId]; ok {
		return cfg, nil
		err := copier.Copy(&n2CfgCard, &cfg)
		if err != nil {
			return nil, fmt.Errorf("Copier Data  failed")
		}
		return n2CfgCard, nil
	}
	return nil, fmt.Errorf("room %d no card config", ruleId)
}

//获取room配置
/*
roomID  真实房间ID 1001 ，1002
*/
func (dc *DataCenter) GetRoomConfig(roomID int32) (IRoomListConfig, error) {
	allN2CfgRoomlist := dc.getSnapshot().N2CfgRoomlist
	if n2CfgRoomlist, ok := allN2CfgRoomlist[roomID]; ok {
		return NewRoomListConfig(n2CfgRoomlist), nil
	}
	clog.Panic("room %d no room config ", roomID)
	return nil, fmt.Errorf("room %d no room config ", roomID)
}
func (dc *DataCenter) GetN2CfgReelRoom(ruleId int32) (*logicGameModel.N2CfgReelRoom, error) {
	allN2CfgReelRoom := dc.getSnapshot().N2CfgReelRoom
	if allN2CfgReelRoom[ruleId] == nil {
		clog.Panic("room %d no reel room config ", ruleId)
		return nil, fmt.Errorf("room %d no reel room config ", ruleId)
	}
	return allN2CfgReelRoom[ruleId], nil
}

func (dc *DataCenter) GetN2CLevel(levelid int32) (*dbData.FormatLevelConfig, error) {
	allN2CfgReel := dc.getSnapshot().FromatN2CfgLevel
	if allN2CfgReel[levelid] == nil {
		clog.Panic("levelConfig %d no reel  config ", levelid)
		return nil, fmt.Errorf("room %d no reel  config ", levelid)
	}
	return allN2CfgReel[levelid], nil
}
func (dc *DataCenter) GetFromatLines(x, y, id int) (*dbData.CommonLines, error) {
	fromatLines := dc.getSnapshot().FromatLines
	key := getLineKeyName(x, y, id)
	if fromatLines[key] == nil {
		clog.Panic("FromatLines %d no reel  config ", id)
		return nil, fmt.Errorf("FromatLines %d no  config ", id)
	}

	return fromatLines[key], nil
}
func (dc *DataCenter) GetFromatLineIds(x, y int) (*dbData.FormatLinesIdsConfig, error) {
	fromatLineIds := dc.getSnapshot().FromatLineIds
	key := getLineIdsKeyName(x, y)
	if fromatLineIds[key] == nil { //xy= 4*5
		clog.Panic("fromatLineIds %s no config ", key)
		return nil, fmt.Errorf("fromatLineIds %s no  config ", key)
	}

	return fromatLineIds[key], nil
}
