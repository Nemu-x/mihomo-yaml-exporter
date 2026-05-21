package metrics

import (
	"strconv"
	"sync"
	"time"

	"github.com/nemu-x/mihomo-yaml-exporter/internal/subscription"
	"github.com/prometheus/client_golang/prometheus"
)

type Registry struct {
	proxyUp        *prometheus.GaugeVec
	proxyLatency   *prometheus.GaugeVec
	proxyCheckTS   *prometheus.GaugeVec
	subLastSuccess prometheus.Gauge
	subLoadSuccess prometheus.Gauge
	proxyTotal     prometheus.Gauge
	proxyOnline    prometheus.Gauge
	proxyOffline   prometheus.Gauge

	mu sync.Mutex
}

func NewRegistry() *Registry {
	r := &Registry{
		proxyUp: prometheus.NewGaugeVec(prometheus.GaugeOpts{
			Name: "mihomo_proxy_up",
			Help: "1 if TCP connect to proxy server:port succeeded, 0 otherwise",
		}, []string{"name", "type", "server", "port"}),
		proxyLatency: prometheus.NewGaugeVec(prometheus.GaugeOpts{
			Name: "mihomo_proxy_latency_ms",
			Help: "TCP connect latency in milliseconds",
		}, []string{"name", "type", "server", "port"}),
		proxyCheckTS: prometheus.NewGaugeVec(prometheus.GaugeOpts{
			Name: "mihomo_proxy_check_timestamp",
			Help: "Unix timestamp of the last check for this proxy",
		}, []string{"name", "type", "server", "port"}),
		subLastSuccess: prometheus.NewGauge(prometheus.GaugeOpts{
			Name: "mihomo_subscription_last_success_timestamp",
			Help: "Unix timestamp of the last successful subscription fetch",
		}),
		subLoadSuccess: prometheus.NewGauge(prometheus.GaugeOpts{
			Name: "mihomo_subscription_load_success",
			Help: "1 if the last subscription fetch succeeded, 0 otherwise",
		}),
		proxyTotal: prometheus.NewGauge(prometheus.GaugeOpts{
			Name: "mihomo_proxy_total",
			Help: "Total number of proxies being monitored",
		}),
		proxyOnline: prometheus.NewGauge(prometheus.GaugeOpts{
			Name: "mihomo_proxy_online_total",
			Help: "Number of proxies that passed the last TCP check",
		}),
		proxyOffline: prometheus.NewGauge(prometheus.GaugeOpts{
			Name: "mihomo_proxy_offline_total",
			Help: "Number of proxies that failed the last TCP check",
		}),
	}

	prometheus.MustRegister(
		r.proxyUp,
		r.proxyLatency,
		r.proxyCheckTS,
		r.subLastSuccess,
		r.subLoadSuccess,
		r.proxyTotal,
		r.proxyOnline,
		r.proxyOffline,
	)
	return r
}

func (r *Registry) Update(results []subscription.CheckResult, subLoadOK bool, subLastSuccess time.Time, checkTime time.Time) {
	r.mu.Lock()
	defer r.mu.Unlock()

	r.proxyUp.Reset()
	r.proxyLatency.Reset()
	r.proxyCheckTS.Reset()

	checkUnix := float64(checkTime.Unix())
	var online, offline int

	for _, res := range results {
		labels := prometheus.Labels{
			"name":   res.Proxy.Name,
			"type":   res.Proxy.Type,
			"server": res.Proxy.Server,
			"port":   portLabel(res.Proxy.Port),
		}
		up := 0.0
		if res.Up {
			up = 1
			online++
		} else {
			offline++
		}
		r.proxyUp.With(labels).Set(up)
		r.proxyLatency.With(labels).Set(res.LatencyMs)
		r.proxyCheckTS.With(labels).Set(checkUnix)
	}

	r.proxyTotal.Set(float64(len(results)))
	r.proxyOnline.Set(float64(online))
	r.proxyOffline.Set(float64(offline))

	if subLoadOK {
		r.subLoadSuccess.Set(1)
	} else {
		r.subLoadSuccess.Set(0)
	}
	if !subLastSuccess.IsZero() {
		r.subLastSuccess.Set(float64(subLastSuccess.Unix()))
	}
}

func portLabel(port int) string {
	return strconv.Itoa(port)
}
