package redis

import (
	"context"
	"sync"
	"time"

	cfacade "github.com/cherry-game/cherry/facade"
	clog "github.com/cherry-game/cherry/logger"
	cprofile "github.com/cherry-game/cherry/profile"
	"github.com/redis/go-redis/v9"
)

type RedisConfig struct {
	PrefixKey    string         `json:"prefix_key"`
	SubscribeKey string         `json:"subscribe_key"`
	UseCluster   bool           `json:"use_cluster"`
	Single       *SingleConfig  `json:"single"`
	Cluster      *ClusterConfig `json:"cluster"`
}

type SingleConfig struct {
	Address  string `json:"address"`
	Password string `json:"password"`
	DB       int    `json:"db"`
}
type ClusterConfig struct {
	Address  []string `json:"address"`
	Password string   `json:"password"`
	DB       int      `json:"db"`
}

func getRedisConfig() *RedisConfig {
	redisJsonConfig := cprofile.GetConfig("redis")
	redisConfig := &RedisConfig{}
	redisConfig.PrefixKey = redisJsonConfig.GetString("prefix_key")
	redisConfig.SubscribeKey = redisJsonConfig.GetString("subscribe_key")
	redisConfig.UseCluster = redisJsonConfig.GetBool("UseCluster")
	if redisConfig.UseCluster {
		redisConfig.Cluster = &ClusterConfig{}
		clusterJsonConfig := redisJsonConfig.GetConfig("cluster")
		addresss := clusterJsonConfig.GetConfig("address")
		for i := 0; i < addresss.Size(); i++ {
			redisConfig.Cluster.Address = append(redisConfig.Cluster.Address, addresss.GetString(i))
		}
		redisConfig.Cluster.Password = clusterJsonConfig.GetString("password")
		redisConfig.Cluster.DB = clusterJsonConfig.GetInt("db")
	} else {
		redisConfig.Single = &SingleConfig{}
		singleJsonConfig := redisJsonConfig.GetConfig("single")
		redisConfig.Single.Address = singleJsonConfig.GetString("address")
		redisConfig.Single.Password = singleJsonConfig.GetString("password")
		redisConfig.Single.DB = singleJsonConfig.GetInt("db")
	}
	return redisConfig
}

type RedisCompent struct {
	cfacade.Component
	redisClient redis.UniversalClient
	initOnce    sync.Once
}

var Name = "db_redis"

func NewRedisCompent() *RedisCompent {
	return &RedisCompent{}
}

func (r *RedisCompent) Name() string {
	return Name
}

func (r *RedisCompent) Init() {
	r.initOnce.Do(func() {
		cfg := getRedisConfig()
		// 设置连接超时控制，防止初始化时因网络问题导致进程永久卡死
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()

		if cfg.UseCluster {
			// 集群模式初始化
			if cfg.Cluster == nil || len(cfg.Cluster.Address) == 0 {
				clog.ErrorContext(ctx, "redis cluster configuration is empty")
				return
			}
			clusterCfg := cfg.Cluster
			r.redisClient = redis.NewClusterClient(&redis.ClusterOptions{
				Addrs:        clusterCfg.Address,
				Password:     clusterCfg.Password,
				DialTimeout:  5 * time.Second,
				ReadTimeout:  3 * time.Second,
				WriteTimeout: 3 * time.Second,
				PoolSize:     64, // 商业级配置：根据 CPU/并发设置合理的连接池大小
				MinIdleConns: 10, // 保持部分空闲连接，避免高并发时频繁创建连接
			})
		} else {
			// 单机模式初始化
			r.redisClient = redis.NewClient(&redis.Options{
				Addr:         cfg.Single.Address,
				Password:     cfg.Single.Password,
				DB:           cfg.Single.DB,
				DialTimeout:  5 * time.Second,
				ReadTimeout:  3 * time.Second,
				WriteTimeout: 3 * time.Second,
				PoolSize:     64,
				MinIdleConns: 10,
			})
		}

		// 商业级必备：初始化后立即进行 Ping 健康检查，确保配置和网络切实可用
		if err := r.redisClient.Ping(ctx).Err(); err != nil {
			_ = r.redisClient.Close() // 失败时清理已创建的连接池
			clog.Panic("failed to ping redis", err.Error())
			return
		}
	})
}

func (r *RedisCompent) GetDb() redis.UniversalClient {
	if r.redisClient == nil {
		clog.ErrorContext(context.Background(), "r.redisClient not initialized. Call InitDatabase first")
		clog.Panic(".redisClient not initialized. Call InitDatabase first")

	}
	return r.redisClient
}

func (r *RedisCompent) OnAfterInit() {
}

func (r *RedisCompent) OnBeforeStop() {
}

func (r *RedisCompent) OnStop() {
}
