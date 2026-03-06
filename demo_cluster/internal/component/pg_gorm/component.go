package pggorm

import (
	"fmt"
	"time"

	cfacade "github.com/cherry-game/cherry/facade"
	clog "github.com/cherry-game/cherry/logger"
	cprofile "github.com/cherry-game/cherry/profile"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

const (
	Name = "pggorm_component"
	dsn  = "host=%s user=%s password=%s dbname=%s port=%d sslmode=disable TimeZone=Asia/Shanghai"
)

type (
	Component struct {
		cfacade.Component
		// key:groupID,value:{key:id,value:*gorm.Db}
		ormMap map[string]map[string]*gorm.DB
	}

	mySqlConfig struct {
		Enable         bool
		GroupID        string
		ID             string
		DbName         string
		Host           string
		UserName       string
		Password       string
		MaxIdleConnect int
		MaxOpenConnect int
		Port           int
		LogMode        bool
		DSN            string
		SlowThreshold  time.Duration
		MaxLifetime    time.Duration
		MaxIdleTime    time.Duration
	}

	// HashDb hash by group id
	HashDb func(dbMaps map[string]*gorm.DB) string
)

func NewComponent() *Component {
	return &Component{
		ormMap: make(map[string]map[string]*gorm.DB),
	}
}

func (s *Component) Name() string {
	return Name
}

func parseMysqlConfig(groupID string, item cfacade.ProfileJSON) *mySqlConfig {
	return &mySqlConfig{
		GroupID:        groupID,
		ID:             item.GetString("db_id"),
		DSN:            item.GetString("dsn", ""),
		DbName:         item.GetString("db_name"),
		Host:           item.GetString("host"),
		UserName:       item.GetString("user_name"),
		Password:       item.GetString("password"),
		MaxIdleConnect: item.GetInt("max_idle_connect", 4),
		MaxOpenConnect: item.GetInt("max_open_connect", 8),
		LogMode:        item.GetBool("log_mode", true),
		Enable:         item.GetBool("enable", true),
		Port:           item.GetInt("port", 5432),
		SlowThreshold:  item.GetDuration("slow_threshold_ms", 20) * time.Millisecond,
		MaxLifetime:    item.GetDuration("max_life_time_minute", 30) * time.Minute,
		MaxIdleTime:    item.GetDuration("max_idle_time_minute", 10) * time.Minute,
	}

}

func (s *Component) Init() {
	// load only the database contained in the `db_id_list`
	dbIDList := s.App().Settings().Get("db_id_list")
	if dbIDList.LastError() != nil || dbIDList.Size() < 1 {
		clog.Warnf("[nodeID = %s] `db_id_list` property not exists.", s.App().NodeID())
		return
	}

	dbConfig := cprofile.GetConfig("db")
	if dbConfig.LastError() != nil {
		clog.Panic("`db` property not exists in profile file.")
	}

	for _, groupID := range dbConfig.Keys() {
		s.ormMap[groupID] = make(map[string]*gorm.DB)

		dbGroup := dbConfig.GetConfig(groupID)
		for i := 0; i < dbGroup.Size(); i++ {
			item := dbGroup.GetConfig(i)
			mysqlConfig := parseMysqlConfig(groupID, item)

			for _, key := range dbIDList.Keys() {
				if dbIDList.Get(key).ToString() != mysqlConfig.ID {
					continue
				}

				if !mysqlConfig.Enable {
					clog.Panicf("[dbName = %s] is disabled!", mysqlConfig.DbName)
				}

				db, err := s.createORM(mysqlConfig)
				if err != nil {
					clog.Panicf("[dbName = %s] create orm fail. error = %s", mysqlConfig.DbName, err)
				}

				s.ormMap[groupID][mysqlConfig.ID] = db
				clog.Infof("[dbGroup =%s, dbName = %s] is connected.", mysqlConfig.GroupID, mysqlConfig.ID)
			}
		}
	}
}

func (s *Component) createORM(cfg *mySqlConfig) (*gorm.DB, error) {
	db, err := gorm.Open(postgres.Open(cfg.GetDSN()), &gorm.Config{
		Logger:                 getLogger(cfg),
		SkipDefaultTransaction: true, // 关闭默认事务
		PrepareStmt:            true, // 开启预编译缓存，提升速度
	})

	if err != nil {
		return nil, err
	}

	sqlDB, err := db.DB()
	if err != nil {
		return nil, err
	}
	// 2. 配置连接池参数
	// SetMaxIdleConns 用于设置连接池中空闲连接的最大数量。
	// 如果不设置，默认为2。
	sqlDB.SetMaxIdleConns(cfg.MaxIdleConnect)
	// SetMaxOpenConns 用于设置打开数据库连接的最大数量。
	// 如果不设置，默认为0，表示不限制。
	sqlDB.SetMaxOpenConns(cfg.MaxOpenConnect)
	// SetConnMaxIdleTime 用于设置连接在被关闭前可以处于空闲状态的最长时间。
	// 如果一个连接空闲时间超过这个值，它将被关闭。这比 SetMaxIdleConns 更灵活。
	sqlDB.SetConnMaxLifetime(30 * time.Minute)
	sqlDB.SetConnMaxIdleTime(10 * time.Minute)

	if cfg.LogMode {
		return db.Debug(), nil
	}

	return db, nil
}

func getLogger(cfg *mySqlConfig) logger.Interface {
	return logger.New(
		gormLogger{log: clog.DefaultLogger},
		logger.Config{
			SlowThreshold:             cfg.SlowThreshold,
			LogLevel:                  logger.Warn,
			Colorful:                  true,
			IgnoreRecordNotFoundError: true,
		},
	)
}

func (s *Component) GetDb(id string) *gorm.DB {
	for _, group := range s.ormMap {
		for k, v := range group {
			if k == id {
				return v
			}
		}
	}
	return nil
}

func (s *Component) GetHashDb(groupID string, hashFn HashDb) (*gorm.DB, bool) {
	dbGroup, found := s.GetDbMap(groupID)
	if !found {
		clog.Warnf("groupID = %s not found.", groupID)
		return nil, false
	}

	dbID := hashFn(dbGroup)
	db, found := dbGroup[dbID]
	return db, found
}

func (s *Component) GetDbMap(groupID string) (map[string]*gorm.DB, bool) {
	dbGroup, found := s.ormMap[groupID]
	return dbGroup, found
}

func (s *mySqlConfig) GetDSN() string {
	if s.DSN == "" {
		s.DSN = dsn
	}

	return fmt.Sprintf(s.DSN, s.Host, s.UserName, s.Password, s.DbName, s.Port)
}

// PrintPoolStats 打印指定数据库的连接池统计信息
func (s *Component) PrintPoolStats(dbID string) {
	db := s.GetDb(dbID)
	if db == nil {
		clog.Warnf("[PrintPoolStats] db not found: %s", dbID)
		return
	}

	sqlDB, err := db.DB()
	if err != nil {
		clog.Warnf("[PrintPoolStats] get sql.DB failed: %v", err)
		return
	}

	stats := sqlDB.Stats()
	clog.Infof("====== DB Pool Stats [%s] ======", dbID)
	clog.Infof("最大允许打开连接数 (MaxOpenConnections): %d", stats.MaxOpenConnections)
	clog.Infof("当前已打开连接数 (OpenConnections): %d", stats.OpenConnections)
	clog.Infof("正在使用中 (InUse): %d", stats.InUse)
	clog.Infof("空闲中 (Idle): %d", stats.Idle)
	clog.Infof("累计等待连接总耗时 (WaitDuration): %v", stats.WaitDuration)
	clog.Infof("累计等待连接总次数 (WaitCount): %d", stats.WaitCount)
	clog.Infof("===========================")
}

// PrintAllPoolStats 打印所有数据库的连接池统计信息
func (s *Component) PrintAllPoolStats() {
	for groupID, group := range s.ormMap {
		for dbID, db := range group {
			sqlDB, err := db.DB()
			if err != nil {
				clog.Warnf("[PrintAllPoolStats] get sql.DB failed for %s/%s: %v", groupID, dbID, err)
				continue
			}

			stats := sqlDB.Stats()
			clog.Warnf("====== DB Pool Stats [%s/%s] ======", groupID, dbID)
			clog.Warnf("最大允许打开连接数 (MaxOpenConnections): %d", stats.MaxOpenConnections)
			clog.Warnf("当前已打开连接数 (OpenConnections): %d", stats.OpenConnections)
			clog.Warnf("正在使用中 (InUse): %d", stats.InUse)
			clog.Warnf("空闲中 (Idle): %d", stats.Idle)
			clog.Warnf("累计等待连接总耗时 (WaitDuration): %v", stats.WaitDuration)
			clog.Warnf("累计等待连接总次数 (WaitCount): %d", stats.WaitCount)
			clog.Warnf("===========================")
		}
	}
}

// OnAfterInit 组件初始化完成后，启动定时打印连接池状态
func (s *Component) OnAfterInit() {
	// 每30秒打印一次连接池状态（可根据需要调整间隔）
	// go func() {
	// 	ticker := time.NewTicker(5 * time.Second)
	// 	defer ticker.Stop()
	// 	for range ticker.C {
	// 		s.PrintAllPoolStats()
	// 	}
	// }()
}
