package controller

import (
	"time"

	cherryString "github.com/cherry-game/cherry/extend/string"
	cherryLogger "github.com/cherry-game/cherry/logger"
	cherryGin "github.com/cherry-game/components/gin"
	"github.com/cherry-game/examples/demo_cluster/internal/code"
	"github.com/cherry-game/examples/demo_cluster/internal/component/metrics"
	"github.com/cherry-game/examples/demo_cluster/internal/data"
	rpcCenter "github.com/cherry-game/examples/demo_cluster/internal/rpc/center"
	"github.com/cherry-game/examples/demo_cluster/internal/token"
	"github.com/cherry-game/examples/demo_cluster/nodes/web/sdk"
)

type Controller struct {
	cherryGin.BaseController
}

func (p *Controller) Init() {
	group := p.Group("/")
	group.GET("/", p.index)
	group.GET("/hello", p.hello)
	group.GET("/register", p.register)
	group.GET("/login", p.login)
	group.GET("/server/list/:pid", p.serverList)
}

// index h5客户端
func (p *Controller) index(c *cherryGin.Context) {
	c.HTML200("index.html")
}

// hello 输出json示例
// http://127.0.0.1/hello
func (p *Controller) hello(c *cherryGin.Context) {
	// 输出json
	code.RenderResult(c, code.OK, map[string]string{
		"data": "hello",
	})
}

// register 开发模式帐号注册
// http://127.0.0.1/register?account=test11&password=test11
func (p *Controller) register(c *cherryGin.Context) {
	done := metrics.TrackRequest("/register")
	defer done(false)
	accountName := c.GetString("account", "", true)
	password := c.GetString("password", "", true)

	statusCode := rpcCenter.RegisterDevAccount(p.App, accountName, password, c.ClientIP())
	code.RenderResult(c, statusCode)
}

// login 根据pid获取sdkConfig，与第三方进行帐号登陆效验
// http://127.0.0.1/login?pid=2126001&account=test1&password=test1
func (p *Controller) login(c *cherryGin.Context) {
	done := metrics.TrackRequest("/login")
	// 平台id，产品 ID、项目 ID 或 包 ID
	pid := c.GetInt32("pid", 0, true)

	if pid < 1 {
		cherryLogger.Warnf("if pid < 1 {. params=%s", c.GetParams())
		code.RenderResult(c, code.PIDError)
		return
	}

	config := data.SdkConfig.Get(pid)
	if config == nil {
		cherryLogger.Warnf("if platformConfig == nil {. params=%s", c.GetParams())
		code.RenderResult(c, code.LoginError)
		return
	}

	sdkInvoke, err := sdk.GetInvoke(config.SdkId)
	if err != nil {
		cherryLogger.Warnf("[pid = %d] get invoke error. params=%s", pid, c.GetParams())
		code.RenderResult(c, code.PIDError)
		return
	}

	params := c.GetParams(true)
	params["pid"] = cherryString.ToString(pid)

	// invoke login
	sdkInvoke.Login(config, params, func(statusCode int32, result sdk.Params, error ...error) {
		if code.IsFail(statusCode) {
			cherryLogger.Warnf("login validate fail. code = %d, params = %s", statusCode, c.GetParams())
			if len(error) > 0 {
				cherryLogger.Warnf("code = %d, error = %s", statusCode, error[0])
			}

			code.RenderResult(c, statusCode)
			return
		}

		if result == nil {
			cherryLogger.Warnf("callback result map is nil. params= %s", c.GetParams())
			code.RenderResult(c, code.LoginError)
			return
		}

		openId, found := result.GetString("open_id")
		if found == false {
			cherryLogger.Warnf("callback result map not found `open_id`. result = %s", result)
			code.RenderResult(c, code.LoginError)
			return
		}

		base64Token := token.New(pid, openId, config.Salt).ToBase64()
		code.RenderResult(c, code.OK, base64Token)
		done(false)
	})
}

// severList 区服列表（带负载均衡）
// http://127.0.0.1/server/list/2126001
func (p *Controller) serverList(c *cherryGin.Context) {
	startTime := time.Now()
	pid := c.GetInt32("pid", 2126001)

	if pid < 1 {
		cherryLogger.Warnf("if pid < 1 {. params=%v", c.GetParams())
		code.RenderResult(c, code.PIDError)
		return
	}

	areaGroup, found := data.AreaGroupConfig.Get(pid)
	if found == false {
		code.RenderResult(c, code.PIDError)
		return
	}

	dataList := &struct {
		Areas          []*AreaResponse   `json:"areas"`
		Servers        []*ServerResponse `json:"servers"`
		UseLoadBalance bool              `json:"useLoadBalance"`
	}{
		UseLoadBalance: true,
	}
	// 先得到游戏渠道AreaIdList,再得到区,AreaConfig,
	// 再根据GateNodes获取最优Gate地址
	for _, areaId := range areaGroup.AreaIdList {
		areaRow, found := data.AreaConfig.Get(areaId)
		if found == false {
			continue
		}

		// 根据区的Gate节点列表获取最优Gate
		var bestGateAddr string
		if len(areaRow.GateNodes) > 0 {
			// 调用Center获取该区最优的Gate节点
			addr, errCode := rpcCenter.GetBestGateFromNodes(p.App, areaRow.GateNodes, "")
			if code.IsOK(errCode) && addr != "" {
				bestGateAddr = addr
			}
		}

		// 如果获取失败，使用默认地址
		if bestGateAddr == "" {
			if areaRow.DefaultTcpAddress != "" {
				bestGateAddr = areaRow.DefaultTcpAddress
			}
		}

		areaResp := &AreaResponse{
			AreaId:   areaRow.AreaId,
			AreaName: areaRow.AreaName,
			Gate:     bestGateAddr,
		}
		dataList.Areas = append(dataList.Areas, areaResp)

		// 获取该区的服务器列表，转换为响应结构（不暴露gameNodes）
		serverList := data.AreaServerConfig.ListWithAreaId(areaRow.AreaId)
		for _, server := range serverList {
			serverResp := &ServerResponse{
				ServerId:   server.ServerId,
				ServerName: server.ServerName,
				AreaId:     server.AreaId,
				Status:     server.Status,
			}
			dataList.Servers = append(dataList.Servers, serverResp)
		}
	}
	elapsed := time.Since(startTime)
	if elapsed.Milliseconds() > 10 {
		cherryLogger.Warnf("[serverList] time=%d ms", elapsed.Milliseconds())
	}
	code.RenderResult(c, code.OK, dataList)
}

// AreaResponse 区响应结构（返回给客户端）
type AreaResponse struct {
	AreaId   int32  `json:"areaId"`
	AreaName string `json:"areaName"`
	Gate     string `json:"gate"`
}

// ServerResponse 服响应结构（返回给客户端，不包含内部节点信息）
type ServerResponse struct {
	ServerId   int32  `json:"serverId"`
	ServerName string `json:"serverName"`
	AreaId     int32  `json:"areaId"`
	Status     int32  `json:"status"`
}
