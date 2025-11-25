package cherryETCD

import (
	"context"
	"fmt"
	"strings"
	"time"

	cfacade "github.com/cherry-game/cherry/facade"
	cherryFacade "github.com/cherry-game/cherry/facade"
	clog "github.com/cherry-game/cherry/logger"
	cprofile "github.com/cherry-game/cherry/profile"
	config_cache "github.com/cherry-game/examples/demo_cluster/internal/config_cache"
	jsoniter "github.com/json-iterator/go"
	"go.etcd.io/etcd/api/v3/mvccpb"
	clientv3 "go.etcd.io/etcd/client/v3"
	"go.etcd.io/etcd/client/v3/namespace"
)

// CheckConfigETCD etcd方式发现配置更新
type CheckConfigETCD struct {
	cherryFacade.Component
	prefix     string
	config     clientv3.Config
	ttl        int64
	cli        *clientv3.Client // etcd client
	leaseID    clientv3.LeaseID // get lease id
	dataCenter config_cache.IConfigCenter
	keyPrefix  string
}

func New(keyPrefix string, configDataCenter config_cache.IConfigCenter) *CheckConfigETCD {
	c := &CheckConfigETCD{
		keyPrefix:  keyPrefix,
		dataCenter: configDataCenter,
	}
	return c
}
func (c *CheckConfigETCD) Name() string {
	return "etcd"
}
func (c *CheckConfigETCD) register() {

	jsonString := "100"
	registerKeyFormat := c.keyPrefix + "%s"
	key := fmt.Sprintf(registerKeyFormat, c.App().NodeID())
	_, err := c.cli.Put(context.Background(), key, jsonString, clientv3.WithLease(c.leaseID))
	if err != nil {
		clog.Fatal(err)
		return
	}
}
func (c *CheckConfigETCD) Init() {
	c.ttl = 10

	clusterConfig := cprofile.GetConfig("cluster").GetConfig(c.Name())
	if clusterConfig.LastError() != nil {
		clog.Fatalf("etcd config not found. err = %v", clusterConfig.LastError())
		return
	}

	c.loadConfig(clusterConfig)
	c.cliInit()
	c.getLeaseID()
	c.watch()
	c.register()
	clog.Infof("[etcd] init complete! [endpoints = %v] [leaseID = %d]", c.config.Endpoints, c.leaseID)
}

func (c *CheckConfigETCD) OnStop() {
	registerKeyFormat := c.keyPrefix + "%s"
	key := fmt.Sprintf(registerKeyFormat, c.App().NodeID())
	_, err := c.cli.Delete(context.Background(), key)
	clog.Infof("CheckConfigETCD stopping! err = %v", err)

	err = c.cli.Close()
	if err != nil {
		clog.Warnf("CheckConfigETCD stopping error! err = %v", err)
	}
}

func getDialTimeout(config jsoniter.Any) time.Duration {
	t := time.Duration(config.Get("dial_timeout_second").ToInt64()) * time.Second
	if t < 1*time.Second {
		t = 3 * time.Second
	}

	return t
}

func getEndPoints(config jsoniter.Any) []string {
	return strings.Split(config.Get("end_points").ToString(), ",")
}

func (c *CheckConfigETCD) loadConfig(config cfacade.ProfileJSON) {
	c.config = clientv3.Config{
		Logger: clog.DefaultLogger.Desugar(),
	}

	c.config.Endpoints = getEndPoints(config)
	c.config.DialTimeout = getDialTimeout(config)
	c.config.Username = config.GetString("user")
	c.config.Password = config.GetString("password")
	c.ttl = config.GetInt64("ttl", 5)
	c.prefix = config.GetString("prefix", "cherry")
}

func (c *CheckConfigETCD) cliInit() {
	var err error
	c.cli, err = clientv3.New(c.config)
	if err != nil {
		clog.Fatalf("etcd connect fail. err = %v", err)
		return
	}

	// set namespace
	c.cli.KV = namespace.NewKV(c.cli.KV, c.prefix)
	c.cli.Watcher = namespace.NewWatcher(c.cli.Watcher, c.prefix)
	c.cli.Lease = namespace.NewLease(c.cli.Lease, c.prefix)
}

func (c *CheckConfigETCD) getLeaseID() {
	var err error
	//设置租约时间
	resp, err := c.cli.Grant(context.Background(), c.ttl)
	if err != nil {
		clog.Fatal(err)
		return
	}

	c.leaseID = resp.ID

	//设置续租 定期发送需求请求
	keepaliveChan, err := c.cli.KeepAlive(context.Background(), resp.ID)
	if err != nil {
		clog.Fatal(err)
		return
	}

	go func() {
		for {
			select {
			case <-keepaliveChan:
				{
				}
			case die := <-c.App().DieChan():
				{
					if die {
						return
					}
				}
			}
		}
	}()
}

func (c *CheckConfigETCD) watch() {
	_, err := c.cli.Get(context.Background(), c.keyPrefix, clientv3.WithPrefix())
	if err != nil {
		clog.Fatal(err)
		return
	}

	watchChan := c.cli.Watch(context.Background(), c.keyPrefix, clientv3.WithPrefix())
	go func() {
		for rsp := range watchChan {
			for _, ev := range rsp.Events {
				switch ev.Type {
				case mvccpb.PUT:
					{
						// 触发重新加载
						if err := c.dataCenter.Reload(); err != nil {
							clog.Errorf("reload config failed: %v", err)
						} else {
							clog.Infof("config reloaded successfully")
						}
					}
				}
			}
		}
	}()
}
