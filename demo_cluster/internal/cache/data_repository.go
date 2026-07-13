package cache

import (
	"context"
	"encoding/json"
	"fmt"
	"reflect"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/allegro/bigcache/v3"
	clog "github.com/cherry-game/cherry/logger"
	cherryRedis "github.com/cherry-game/examples/demo_cluster/internal/component/redis"
	"github.com/jinzhu/copier"
	"go.uber.org/zap"
)

const (
	StateRunning  int32 = 0 // 正在运行
	StateShutting int32 = 1 // 正在关机
)

var count int32
var (
	globalBigCache *bigcache.BigCache
	once           sync.Once
)

// DirtyEntry 通用脏数据包装器
type DirtyEntry struct {
	TableName string
	Key       string
	Timestamp time.Time
}
type DataRepository[T any] struct {
	status    int32 // 💡 原子状态：0运行，1准备关闭
	tableName string
	mu        sync.RWMutex
	activeMap map[string]T // 玩家数据
	// 脏数据机制和hash对比优化
	lastHashes map[string]uint64 // 记录上一次保存的数据 Hash，防止重复同步相同数据
	dityData   map[string][]byte // 脏数据暂存区，防 BigCache 淘汰导致数据丢失

	bigCache  *bigcache.BigCache
	redisComp *cherryRedis.RedisCompent // 缓存组件引用
	dirtyChan chan DirtyEntry           // 队列里面只需要有需要更新的key
	closeChan chan struct{}
	wg        sync.WaitGroup
}

// 初始化全局 BigCache (应用启动时调用一次),不能一个玩家new 一个,内存会爆炸
func InitGlobalBigCache() error {
	var err error
	config := bigcache.Config{
		// 1. 分片数，必须是 2 的幂（推荐 1024 或 256），分片越多并发锁竞争越小
		Shards: 1024,

		// 2. 缓存数据的生命周期（例如 30 分钟不访问就过期）
		LifeWindow: 30 * time.Minute,

		// 3. 开启定时清理协程的间隔（例如每 1 分钟去清理一次过期数据）
		CleanWindow: 1 * time.Minute,

		// 4. 初始化预估：预计在这 30 分钟内，全服最多有多少条数据
		MaxEntriesInWindow: 1 * 10000, // 假设 1 万条

		// 5. 单条数据的预估大小（单位：字节 Bytes）
		MaxEntrySize: 1500, // 1000 字节

		// 6. 🔥 最核心的防 OOM 安全线（单位：MB）
		// 限制整个缓存不管怎么扩容，最多只能吃 2GB (2048MB) 内存。到达后会自动踢掉最老的数据
		HardMaxCacheSize: 2048,
	}
	once.Do(func() {
		globalBigCache, err = bigcache.New(context.Background(), config)
	})
	return err
}

func NewDataRepository[T any](tableName string, redisComp *cherryRedis.RedisCompent, bcConfig bigcache.Config) (*DataRepository[T], error) {
	InitGlobalBigCache()
	// 确保全局 BigCache 已初始化
	if globalBigCache == nil {
		return nil, fmt.Errorf("global BigCache not initialized, call InitGlobalBigCache first")
	}
	repo := &DataRepository[T]{
		tableName: tableName,
		activeMap: make(map[string]T),
		bigCache:  globalBigCache,
		redisComp: redisComp,
		dirtyChan: make(chan DirtyEntry, 5), // 脏队列缓冲区
	}
	// 这里只能使用1个go,不然数据会错乱
	for i := 0; i < 1; i++ {
		// 开启go 同步数据
		repo.wg.Add(1)
		go repo.saveDataWork()
	}
	return repo, nil
}

func (r *DataRepository[T]) GetData(ctx context.Context, factory func() T, key ...string) (T, bool) {
	keyField := r.GetField(key...)
	r.mu.RLock()
	mapValue, ok := r.activeMap[keyField]
	if ok {
		r.mu.RUnlock() // 💡 命中后立刻手动解锁，提高并发吞吐
		return mapValue, true
	}
	r.mu.RUnlock() // 💡 命中后立刻手动解锁，提高并发吞吐
	// 2. 【第二阶段：加写锁回填】
	r.mu.Lock()
	defer r.mu.Unlock() // 这里的写锁可以用 defer，因为后续没有锁嵌套了
	// 从缓存中获取数据
	var vaule T
	targetType := reflect.TypeOf(vaule).Elem()
	vaule = reflect.New(targetType).Interface().(T)
	if cachedData, err := r.bigCache.Get(keyField); err == nil {
		if err := json.Unmarshal(cachedData, vaule); err == nil {
			r.activeMap[keyField] = vaule
			return vaule, true
		}
	}
	// 从redis 中获取数据
	if stringCmd := r.redisComp.GetDb().HGet(ctx, r.GetHashKey(), keyField); stringCmd.Err() == nil && stringCmd.Val() != "null" {
		if err := json.Unmarshal([]byte(stringCmd.Val()), vaule); err == nil {
			// 这里先不同步到bigCache
			r.activeMap[keyField] = vaule
			return vaule, true
		}
	} else {
		// 如果数据库也没有，使用传入的工厂函数初始化一个新对象 (对应您原代码的 NewLevelSessionData)
		vaule = factory()
	}
	return vaule, true
}

func (r *DataRepository[T]) SaveData(ctx context.Context, data T, key ...string) error {
	defer func() {
		if err := recover(); err != nil {
			clog.ErrorContext(ctx, "SaveData err", zap.Any("error", err))
		}
	}()
	keyField := r.GetField(key...)
	r.mu.Lock()
	r.activeMap[keyField] = data
	r.mu.Unlock()
	// 1. 💡 先用原子操作检查状态。如果已经进入关机流程，直接拒绝写入通道！
	if atomic.LoadInt32(&r.status) == StateShutting {
		// 商业容灾：此时说明网关已经在踢人了，业务可以转为【同步双写DB】，或者直接打日志
		return nil
	}
	// 发送脏数据通知
	// 异步发送落盘信号，不阻塞当前游戏主逻辑
	atomic.AddInt32(&count, 1)
	r.dirtyChan <- DirtyEntry{
		TableName: r.tableName,
		Key:       keyField,
		Timestamp: time.Now(),
	}
	clog.DebugContext(ctx, "cache count", zap.Int32("count", count))
	return nil
}

func (r *DataRepository[T]) saveDataWork() {
	defer r.wg.Done()
	for entry := range r.dirtyChan {
		atomic.AddInt32(&count, -1)
		r.executeSave(entry.Key)
	}
}

func (r *DataRepository[T]) executeSave(keyField string) {
	r.mu.RLock()
	val, ok := r.activeMap[keyField]
	if !ok {
		r.mu.RUnlock()
		return
	}
	r.mu.RUnlock()
	var snapshot T
	// 2. 利用反射动态初始化指针内存
	// 💡 原理：拿到 T 的指针底层指向的实际结构体类型，并 new 一个新对象赋值给 snapshot
	targetType := reflect.TypeOf(snapshot).Elem()
	snapshot = reflect.New(targetType).Interface().(T)
	// 深拷贝,防止破环数据
	copier.Copy(snapshot, val)
	jsonData, _ := json.Marshal(snapshot)
	r.redisComp.GetDb().HSet(context.Background(), r.GetHashKey(), keyField, jsonData)
	// 保存到bigcache,因为这里是一张表一个对象,所以不需要包含tableName
	r.bigCache.Set(keyField, jsonData)
}

func (r *DataRepository[T]) GetHashKey() string {
	return fmt.Sprint("player_data", ":", r.tableName)
}

func (r *DataRepository[T]) GetField(key ...string) string {
	return strings.Join(key, ":")
}
