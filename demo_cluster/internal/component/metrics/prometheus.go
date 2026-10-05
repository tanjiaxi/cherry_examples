package metrics

import (
	"github.com/prometheus/client_golang/prometheus"
)

type routeCollector struct {
	component *Component

	requests   *prometheus.Desc
	responses  *prometheus.Desc
	errors     *prometheus.Desc
	qps        *prometheus.Desc
	respQPS    *prometheus.Desc
	latencyP50 *prometheus.Desc
	latencyP90 *prometheus.Desc
	latencyP99 *prometheus.Desc
}

func newRouteCollector(c *Component) *routeCollector {
	return &routeCollector{
		component: c,
		requests: prometheus.NewDesc(
			"game_route_requests_total",
			"Total client/server requests recorded by route",
			[]string{"route"}, nil,
		),
		responses: prometheus.NewDesc(
			"game_route_responses_total",
			"Total responses recorded by route",
			[]string{"route"}, nil,
		),
		errors: prometheus.NewDesc(
			"game_route_errors_total",
			"Total errors recorded by route",
			[]string{"route"}, nil,
		),
		qps: prometheus.NewDesc(
			"game_route_qps",
			"Request QPS (10s average) by route",
			[]string{"route"}, nil,
		),
		respQPS: prometheus.NewDesc(
			"game_route_resp_qps",
			"Response QPS (10s average) by route",
			[]string{"route"}, nil,
		),
		latencyP50: prometheus.NewDesc(
			"game_route_latency_p50_seconds",
			"Request latency P50 in seconds by route",
			[]string{"route"}, nil,
		),
		latencyP90: prometheus.NewDesc(
			"game_route_latency_p90_seconds",
			"Request latency P90 in seconds by route",
			[]string{"route"}, nil,
		),
		latencyP99: prometheus.NewDesc(
			"game_route_latency_p99_seconds",
			"Request latency P99 in seconds by route",
			[]string{"route"}, nil,
		),
	}
}

func (c *routeCollector) Describe(ch chan<- *prometheus.Desc) {
	ch <- c.requests
	ch <- c.responses
	ch <- c.errors
	ch <- c.qps
	ch <- c.respQPS
	ch <- c.latencyP50
	ch <- c.latencyP90
	ch <- c.latencyP99
}

func (c *routeCollector) Collect(ch chan<- prometheus.Metric) {
	if c.component == nil {
		return
	}
	for _, st := range c.component.GetAllStats() {
		ch <- prometheus.MustNewConstMetric(c.requests, prometheus.CounterValue, float64(st.Requests), st.Route)
		ch <- prometheus.MustNewConstMetric(c.responses, prometheus.CounterValue, float64(st.Responses), st.Route)
		ch <- prometheus.MustNewConstMetric(c.errors, prometheus.CounterValue, float64(st.Errors), st.Route)
		ch <- prometheus.MustNewConstMetric(c.qps, prometheus.GaugeValue, st.Avg10sQPS, st.Route)
		ch <- prometheus.MustNewConstMetric(c.respQPS, prometheus.GaugeValue, st.Avg10sRespQPS, st.Route)
		ch <- prometheus.MustNewConstMetric(c.latencyP50, prometheus.GaugeValue, st.P50Ms/1000, st.Route)
		ch <- prometheus.MustNewConstMetric(c.latencyP90, prometheus.GaugeValue, st.P90Ms/1000, st.Route)
		ch <- prometheus.MustNewConstMetric(c.latencyP99, prometheus.GaugeValue, st.P99Ms/1000, st.Route)
	}
}

// RegisterPrometheus 把按 route 的 RED 指标挂到 runtime_monitor 的 /metrics 上
func RegisterPrometheus(reg prometheus.Registerer) error {
	if reg == nil {
		return nil
	}
	c := Global()
	if c == nil {
		return nil
	}
	return reg.Register(newRouteCollector(c))
}
