/**FileHeader
 * @Author: Your Name
 * @Date: 2025/11/21 16:06:16
 * @LastEditors: Your Name
 * @LastEditTime: 2025/11/21 17:48:54
 * @Description:
 * @Copyright: Copyright (©)}) 2025 Your Name. All rights reserved.
 * @Email: xxx@xxx.com
 */
package slots

import (
	"fmt"
	"reflect"
	"sync"
	"time"

	clog "github.com/cherry-game/cherry/logger"
	"github.com/cherry-game/examples/demo_cluster/internal/component/db" //结构
	dbData "github.com/cherry-game/examples/demo_cluster/internal/db"    //具体数据
	gameModel "github.com/cherry-game/examples/demo_cluster/internal/model"
	logicGameModel "github.com/cherry-game/examples/demo_cluster/internal/model/logic_model"
)

type ConfigSnapshot struct {
	Version  int64 //版本号
	LoadTime int64 //加载时间

	//配置数据
	N2CfgCard        map[int32]*dbData.FormatCardConfig
	RoomCardIndex    map[int32]map[int32]*dbData.FormatCardConfig
	N2CfgReelRoom    map[int32]*logicGameModel.N2CfgReelRoom
	N2CfgRoomlist    map[int32]*gameModel.N2CfgRoomlist
	FromatN2CfgLevel map[int32]*dbData.FormatLevelConfig     //key是levelid
	FromatLines      map[string]*dbData.CommonLines          //key是levelid
	FromatLineIds    map[string]*dbData.FormatLinesIdsConfig //key是levelid
}
type DataLoader struct {
}

func NewDataLoader() *DataLoader {
	return &DataLoader{}
}

// 后期的map key应该是id+schama 分配置
func (d *DataLoader) LoadAllConfig() (*ConfigSnapshot, error) {
	configSnapshot := ConfigSnapshot{
		Version:  time.Now().Unix(),
		LoadTime: time.Now().Unix(),

		N2CfgCard:     make(map[int32]*dbData.FormatCardConfig),
		RoomCardIndex: make(map[int32]map[int32]*dbData.FormatCardConfig),

		N2CfgReelRoom:    make(map[int32]*logicGameModel.N2CfgReelRoom),
		N2CfgRoomlist:    make(map[int32]*gameModel.N2CfgRoomlist),
		FromatN2CfgLevel: make(map[int32]*dbData.FormatLevelConfig),
		FromatLines:      make(map[string]*dbData.CommonLines),
		FromatLineIds:    make(map[string]*dbData.FormatLinesIdsConfig),
	}
	configWait := sync.WaitGroup{}
	configWait.Add(1)
	go func() {
		if err := d.LoadLevelConfig(&configSnapshot, "public"); err != nil {
			clog.Errorf("load level config failed: %v", err)
		}
		configWait.Done()
	}()

	//加载配置
	if err := d.LoadCardConfig(&configSnapshot, "public"); err != nil {
		clog.Panic("load card config failed: %w", err)
	}
	if err := d.LoadRoomConfig(&configSnapshot, "public"); err != nil {
		clog.Panic("load room config failed: %w", err)
	}

	if err := d.LoadReelRoomConfig(&configSnapshot, "public"); err != nil {
		clog.Panic("load reel room config failed: %w", err)
	}
	if err := LoadLinesConfig[gameModel.Lines3x3](&configSnapshot, 3, 3, "public"); err != nil {
		clog.Panic("load Lines3x3 config failed: %w", err)
	}
	if err := LoadLinesConfig[gameModel.Lines3x4](&configSnapshot, 3, 4, "public"); err != nil {
		clog.Panic("load Lines3x4 failed: %w", err)
	}
	if err := LoadLinesConfig[gameModel.Lines3x5](&configSnapshot, 3, 5, "public"); err != nil {
		clog.Panic("load Lines3x5 failed: %w", err)
	}
	if err := LoadLinesConfig[gameModel.Lines3x6](&configSnapshot, 3, 6, "public"); err != nil {
		clog.Panic("load Lines3x6 failed: %w", err)
	}
	if err := LoadLinesConfig[gameModel.Lines4x3](&configSnapshot, 4, 3, "public"); err != nil {
		clog.Panic("load Lines4x3 failed: %w", err)
	}
	if err := LoadLinesConfig[gameModel.Lines4x5](&configSnapshot, 4, 5, "public"); err != nil {
		clog.Panic("load Lines4x5 failed: %w", err)
	}
	if err := LoadLinesConfig[gameModel.Lines4x6](&configSnapshot, 4, 6, "public"); err != nil {
		clog.Panic("load Lines4x6 failed: %w", err)
	}
	if err := LoadLinesConfig[gameModel.Lines5x5](&configSnapshot, 5, 5, "public"); err != nil {
		clog.Panic("load Lines5x5 failed: %w", err)
	}
	if err := d.LoadLinesIdsConfig(&configSnapshot, "public"); err != nil {
		clog.Panic("load LinesIds config failed: %w", err)
	}
	configWait.Wait()
	return &configSnapshot, nil
}

/** card配置
 * @description:
 * @return {*}
 */
func (d *DataLoader) LoadCardConfig(configSnapshot *ConfigSnapshot, schema string) error {
	var cardConfig []*gameModel.N2CfgCard
	//从数据库查找
	cardConfig = dbData.GetCardConfig()
	if cardConfig == nil {
		return fmt.Errorf("no card config")
	}
	//转换为镜像map
	for _, v := range cardConfig {
		formatCardConfig, err := dbData.FormatCardConfigData(v)
		if err != nil {
			return err
		}
		if _, ok := configSnapshot.RoomCardIndex[v.RoomID]; !ok {
			configSnapshot.RoomCardIndex[v.RoomID] = make(map[int32]*dbData.FormatCardConfig)
		}
		configSnapshot.RoomCardIndex[v.RoomID][v.Kid] = formatCardConfig
		configSnapshot.N2CfgCard[v.Kid] = formatCardConfig
	}
	return nil
}

// room 配置
func (d *DataLoader) LoadRoomConfig(configSnapshot *ConfigSnapshot, schema string) error {
	var roomConfig []*gameModel.N2CfgRoomlist
	//从数据库查找
	result := db.GetDB().Find(&roomConfig)
	if result.Error != nil {
		return result.Error
	}
	//转换为镜像map
	for _, v := range roomConfig {
		configSnapshot.N2CfgRoomlist[v.RoomID] = v
	}
	return nil
}
func (d *DataLoader) LoadReelRoomConfig(configSnapshot *ConfigSnapshot, schema string) error {
	//从数据库查找
	reelRoomConfig := dbData.GetReelRoomConfig()
	if reelRoomConfig == nil {
		return fmt.Errorf("no reel config")
	}
	logicReelRoomConfig := dbData.FromatReelRoomConfig(reelRoomConfig)
	if logicReelRoomConfig == nil {
		return fmt.Errorf("format error")
	}
	for _, v := range logicReelRoomConfig {
		configSnapshot.N2CfgReelRoom[v.RoomID] = v
	}
	return nil
}

func (d *DataLoader) LoadLevelConfig(configSnapshot *ConfigSnapshot, schema string) error {
	var levelConfig = dbData.GetLevelConfig()
	if levelConfig == nil {
		return fmt.Errorf("no level config")
	}
	//转换为镜像map
	for _, v := range levelConfig {
		formatConfig, err := dbData.FormatData(v)
		if err != nil {
			return err
		}
		configSnapshot.FromatN2CfgLevel[v.Levelid] = formatConfig
	}
	return nil
}
func (d *DataLoader) LoadLinesIdsConfig(configSnapshot *ConfigSnapshot, schema string) error {
	var linesIdsConfig, err = dbData.GetLinesIdsConfig()
	if err != nil {
		return err
	}
	//转换为镜像map
	for _, v := range linesIdsConfig {
		configSnapshot.FromatLineIds[v.Xy] = v
	}
	return nil
}
func LoadLinesConfig[T any](configSnapshot *ConfigSnapshot, x, y int, schema string) error {
	var lines []*T
	var commonLines []*dbData.CommonLines
	//为了，少写代码，必须利用反射，利用反射就必须把db，放在这里
	result := db.GetDB().Find(&lines)
	if result.Error != nil {
		return result.Error
	}
	for _, v := range lines {
		val := reflect.ValueOf(v).Elem() // 获取指针指向的值
		lines, err := dbData.FormatLinesConfig(x, y, val.FieldByName("Line").String())
		if err != nil {
			return err
		}
		common := &dbData.CommonLines{
			ID:       int(val.FieldByName("ID").Int()), // Int() 返回 int64，强转为 int32
			LinesArr: lines,
		}
		commonLines = append(commonLines, common)
	}
	storeConfigSnapshot(configSnapshot, x, y, commonLines)
	//当然也可以用下面的这种工厂模式的方式
	// switch linesMod.(type) {
	// case gameModel.Lines3x3:
	// 	data, err := dbData.GetLinesConfig(linesMod1, x, y)
	// 	if err != nil {
	// 		return err
	// 	}
	// 	storeConfigSnapshot(configSnapshot, x, y, data)
	// case gameModel.Lines3x4:
	// 	data, err := dbData.GetLinesConfig[gameModel.Lines3x4](x, y)
	// 	if err != nil {
	// 		return err
	// 	}
	// 	storeConfigSnapshot(configSnapshot, x, y, data)
	// case gameModel.Lines3x5:
	// 	data, err := dbData.GetLinesConfig[gameModel.Lines3x5](x, y)
	// 	if err != nil {
	// 		return err
	// 	}
	// 	storeConfigSnapshot(configSnapshot, x, y, data)

	// case gameModel.Lines3x6:
	// 	data, err := dbData.GetLinesConfig[gameModel.Lines3x6](x, y)
	// 	if err != nil {
	// 		return err
	// 	}
	// 	storeConfigSnapshot(configSnapshot, x, y, data)

	// case gameModel.Lines4x3:
	// 	data, err := dbData.GetLinesConfig[gameModel.Lines4x3](x, y)
	// 	if err != nil {
	// 		return err
	// 	}
	// 	storeConfigSnapshot(configSnapshot, x, y, data)

	// case gameModel.Lines4x5:
	// 	data, err := dbData.GetLinesConfig[gameModel.Lines4x5](x, y)
	// 	if err != nil {
	// 		return err
	// 	}
	// 	storeConfigSnapshot(configSnapshot, x, y, data)

	// case gameModel.Lines5x5:
	// 	data, err := dbData.GetLinesConfig[gameModel.Lines5x5](x, y)
	// 	if err != nil {
	// 		return err
	// 	}
	// 	storeConfigSnapshot(configSnapshot, x, y, data)
	// }
	return nil
}
func getLineKeyName(x, y, id int) string {
	return fmt.Sprintf("%dx%d_%d", x, y, id)
}
func getLineIdsKeyName(x, y int) string {
	return fmt.Sprintf("%d*%d", x, y)
}
func storeConfigSnapshot(configSnapshot *ConfigSnapshot, x, y int, data []*dbData.CommonLines) error {
	for _, v := range data {
		key := getLineKeyName(x, y, v.ID)
		configSnapshot.FromatLines[key] = v
	}
	return nil
}
