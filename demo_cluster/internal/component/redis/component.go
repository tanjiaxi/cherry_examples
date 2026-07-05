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

// 1. 定义我们的前缀钩子结构体
type RedisPrefixHook struct {
	Prefix string
}

// 2. 必须实现这两个方法（来自 go-redis.Hook 接口）
func (h *RedisPrefixHook) DialHook(next redis.DialHook) redis.DialHook { return next }

func (h *RedisPrefixHook) ProcessHook(next redis.ProcessHook) redis.ProcessHook {
	return func(ctx context.Context, cmd redis.Cmder) error {
		// 在命令发送给 Redis 之前，拦截并修改 Key
		args := cmd.Args()

		// Redis 命令通常格式为: [SET, key, value] 或 [GET, key]
		// 第 0 位是命令本身(SET)，第 1 位就是 Key
		if len(args) > 1 {
			if keyStr, ok := args[1].(string); ok {
				// 关键点：把原来的 key 改成 "前缀 + 原key"
				args[1] = h.Prefix + keyStr
			}
		}

		return next(ctx, cmd) // 继续执行原来的 Redis 操作
	}
}

// 3. 管道命令(Pipeline)也需要处理
func (h *RedisPrefixHook) ProcessPipelineHook(next redis.ProcessPipelineHook) redis.ProcessPipelineHook {
	return func(ctx context.Context, cmds []redis.Cmder) error {
		for _, cmd := range cmds {
			args := cmd.Args()
			if len(args) > 1 {
				if keyStr, ok := args[1].(string); ok {
					args[1] = h.Prefix + keyStr
				}
			}
		}
		return next(ctx, cmds)
	}
}

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
		// 使用钩子函数加前缀
		r.redisClient.AddHook(&RedisPrefixHook{Prefix: cfg.PrefixKey})
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
