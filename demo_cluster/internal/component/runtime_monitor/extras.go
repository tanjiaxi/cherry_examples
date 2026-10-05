package runtime_monitor

import (
	"strconv"
	"sync"

	cherryNats "github.com/cherry-game/cherry/net/nats"
	"github.com/cherry-game/cherry/net/parser/pomelo"
	pggorm "github.com/cherry-game/examples/demo_cluster/internal/component/pg_gorm"
	"github.com/prometheus/client_golang/prometheus"
)

// RedisPoolSnapshot Redis 连接池快照（避免 runtime_monitor 直接依赖 go-redis）
type RedisPoolSnapshot struct {
	Hits       uint64
	Misses     uint64
	Timeouts   uint64
	TotalConns uint64
	IdleConns  uint64
	StaleConns uint64
}

var (
	extrasMu          sync.RWMutex
	onlinePlayersFunc func() int
	redisStatsFunc    func() *RedisPoolSnapshot
)

// SetOnlinePlayersFunc 由 game 节点注入在线人数
func SetOnlinePlayersFunc(fn func() int) {
	extrasMu.Lock()
	onlinePlayersFunc = fn
	extrasMu.Unlock()
}

// SetRedisStatsFunc 由持有 Redis 客户端的节点注入连接池统计
func SetRedisStatsFunc(fn func() *RedisPoolSnapshot) {
	extrasMu.Lock()
	redisStatsFunc = fn
	extrasMu.Unlock()
}

func currentOnlinePlayers() float64 {
	extrasMu.RLock()
	fn := onlinePlayersFunc
	extrasMu.RUnlock()
	if fn == nil {
		return 0
	}
	return float64(fn())
}

func currentRedisStats() *RedisPoolSnapshot {
	extrasMu.RLock()
	fn := redisStatsFunc
	extrasMu.RUnlock()
	if fn == nil {
		return nil
	}
	return fn()
}

type extrasCollector struct {
	comp *Component

	gateConnections *prometheus.Desc
	onlinePlayers   *prometheus.Desc

	dbMaxOpen      *prometheus.Desc
	dbOpen         *prometheus.Desc
	dbInUse        *prometheus.Desc
	dbIdle         *prometheus.Desc
	dbWaitCount    *prometheus.Desc
	dbWaitDuration *prometheus.Desc

	redisHits      *prometheus.Desc
	redisMisses    *prometheus.Desc
	redisTimeouts  *prometheus.Desc
	redisTotalConn *prometheus.Desc
	redisIdleConn  *prometheus.Desc
	redisStaleConn *prometheus.Desc

	natsConnected  *prometheus.Desc
	natsInMsgs     *prometheus.Desc
	natsOutMsgs    *prometheus.Desc
	natsReconnects *prometheus.Desc
}

func newExtrasCollector(comp *Component) *extrasCollector {
	return &extrasCollector{
		comp: comp,
		gateConnections: prometheus.NewDesc(
			"game_gate_connections",
			"Current Pomelo agent (client connection) count on this node",
			nil, nil,
		),
		onlinePlayers: prometheus.NewDesc(
			"game_online_players",
			"Current online player count on this node",
			nil, nil,
		),
		dbMaxOpen: prometheus.NewDesc(
			"game_db_max_open_connections",
			"Database max open connections",
			[]string{"group", "db"}, nil,
		),
		dbOpen: prometheus.NewDesc(
			"game_db_open_connections",
			"Database currently open connections",
			[]string{"group", "db"}, nil,
		),
		dbInUse: prometheus.NewDesc(
			"game_db_in_use_connections",
			"Database connections currently in use",
			[]string{"group", "db"}, nil,
		),
		dbIdle: prometheus.NewDesc(
			"game_db_idle_connections",
			"Database idle connections",
			[]string{"group", "db"}, nil,
		),
		dbWaitCount: prometheus.NewDesc(
			"game_db_wait_count_total",
			"Total number of connections waited for from the pool",
			[]string{"group", "db"}, nil,
		),
		dbWaitDuration: prometheus.NewDesc(
			"game_db_wait_duration_seconds_total",
			"Total time blocked waiting for a new connection",
			[]string{"group", "db"}, nil,
		),
		redisHits: prometheus.NewDesc(
			"game_redis_pool_hits_total",
			"Redis pool hits",
			nil, nil,
		),
		redisMisses: prometheus.NewDesc(
			"game_redis_pool_misses_total",
			"Redis pool misses",
			nil, nil,
		),
		redisTimeouts: prometheus.NewDesc(
			"game_redis_pool_timeouts_total",
			"Redis pool timeouts",
			nil, nil,
		),
		redisTotalConn: prometheus.NewDesc(
			"game_redis_pool_total_conns",
			"Redis pool total connections",
			nil, nil,
		),
		redisIdleConn: prometheus.NewDesc(
			"game_redis_pool_idle_conns",
			"Redis pool idle connections",
			nil, nil,
		),
		redisStaleConn: prometheus.NewDesc(
			"game_redis_pool_stale_conns",
			"Redis pool stale connections",
			nil, nil,
		),
		natsConnected: prometheus.NewDesc(
			"game_nats_connected",
			"Whether this NATS connection is currently connected (1/0)",
			[]string{"conn_id"}, nil,
		),
		natsInMsgs: prometheus.NewDesc(
			"game_nats_in_msgs_total",
			"NATS inbound messages",
			[]string{"conn_id"}, nil,
		),
		natsOutMsgs: prometheus.NewDesc(
			"game_nats_out_msgs_total",
			"NATS outbound messages",
			[]string{"conn_id"}, nil,
		),
		natsReconnects: prometheus.NewDesc(
			"game_nats_reconnects_total",
			"NATS reconnect count",
			[]string{"conn_id"}, nil,
		),
	}
}

func (e *extrasCollector) Describe(ch chan<- *prometheus.Desc) {
	ch <- e.gateConnections
	ch <- e.onlinePlayers
	ch <- e.dbMaxOpen
	ch <- e.dbOpen
	ch <- e.dbInUse
	ch <- e.dbIdle
	ch <- e.dbWaitCount
	ch <- e.dbWaitDuration
	ch <- e.redisHits
	ch <- e.redisMisses
	ch <- e.redisTimeouts
	ch <- e.redisTotalConn
	ch <- e.redisIdleConn
	ch <- e.redisStaleConn
	ch <- e.natsConnected
	ch <- e.natsInMsgs
	ch <- e.natsOutMsgs
	ch <- e.natsReconnects
}

func (e *extrasCollector) Collect(ch chan<- prometheus.Metric) {
	ch <- prometheus.MustNewConstMetric(e.gateConnections, prometheus.GaugeValue, float64(pomelo.Count()))
	ch <- prometheus.MustNewConstMetric(e.onlinePlayers, prometheus.GaugeValue, currentOnlinePlayers())

	e.collectDB(ch)
	e.collectRedis(ch)
	e.collectNATS(ch)
}

func (e *extrasCollector) collectDB(ch chan<- prometheus.Metric) {
	if e.comp == nil {
		return
	}
	app := e.comp.App()
	if app == nil {
		return
	}
	raw := app.Find(pggorm.Name)
	if raw == nil {
		return
	}
	dbc, ok := raw.(*pggorm.Component)
	if !ok {
		return
	}
	for _, st := range dbc.AllPoolStats() {
		labels := []string{st.GroupID, st.DBID}
		ch <- prometheus.MustNewConstMetric(e.dbMaxOpen, prometheus.GaugeValue, float64(st.MaxOpenConnections), labels...)
		ch <- prometheus.MustNewConstMetric(e.dbOpen, prometheus.GaugeValue, float64(st.OpenConnections), labels...)
		ch <- prometheus.MustNewConstMetric(e.dbInUse, prometheus.GaugeValue, float64(st.InUse), labels...)
		ch <- prometheus.MustNewConstMetric(e.dbIdle, prometheus.GaugeValue, float64(st.Idle), labels...)
		ch <- prometheus.MustNewConstMetric(e.dbWaitCount, prometheus.CounterValue, float64(st.WaitCount), labels...)
		ch <- prometheus.MustNewConstMetric(e.dbWaitDuration, prometheus.CounterValue, st.WaitDuration.Seconds(), labels...)
	}
}

func (e *extrasCollector) collectRedis(ch chan<- prometheus.Metric) {
	st := currentRedisStats()
	if st == nil {
		return
	}
	ch <- prometheus.MustNewConstMetric(e.redisHits, prometheus.CounterValue, float64(st.Hits))
	ch <- prometheus.MustNewConstMetric(e.redisMisses, prometheus.CounterValue, float64(st.Misses))
	ch <- prometheus.MustNewConstMetric(e.redisTimeouts, prometheus.CounterValue, float64(st.Timeouts))
	ch <- prometheus.MustNewConstMetric(e.redisTotalConn, prometheus.GaugeValue, float64(st.TotalConns))
	ch <- prometheus.MustNewConstMetric(e.redisIdleConn, prometheus.GaugeValue, float64(st.IdleConns))
	ch <- prometheus.MustNewConstMetric(e.redisStaleConn, prometheus.GaugeValue, float64(st.StaleConns))
}

func (e *extrasCollector) collectNATS(ch chan<- prometheus.Metric) {
	for _, conn := range cherryNats.GetPool() {
		if conn == nil {
			continue
		}
		id := strconv.Itoa(conn.GetID())
		connected := 0.0
		var inMsgs, outMsgs, reconnects float64
		if conn.Conn != nil {
			if conn.IsConnected() {
				connected = 1
			}
			st := conn.Stats()
			inMsgs = float64(st.InMsgs)
			outMsgs = float64(st.OutMsgs)
			reconnects = float64(st.Reconnects)
		}
		ch <- prometheus.MustNewConstMetric(e.natsConnected, prometheus.GaugeValue, connected, id)
		ch <- prometheus.MustNewConstMetric(e.natsInMsgs, prometheus.CounterValue, inMsgs, id)
		ch <- prometheus.MustNewConstMetric(e.natsOutMsgs, prometheus.CounterValue, outMsgs, id)
		ch <- prometheus.MustNewConstMetric(e.natsReconnects, prometheus.CounterValue, reconnects, id)
	}
}
