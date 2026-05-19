package metrics

import (
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
)

// Chaos Metrics
var InjectedFaultsTotal = promauto.NewCounterVec(
	prometheus.CounterOpts{
		Name: "pastaay_injected_faults_total",
		Help: "Total count of injected faults by type and target",
	},
	[]string{"target", "fault_type"},
)

var RequestLatency = promauto.NewHistogramVec(
	prometheus.HistogramOpts{
		Name:    "pastaay_request_latency_seconds",
		Help:    "Latency of requests tracked by Pastaay agents",
		Buckets: prometheus.DefBuckets,
	},
	[]string{"target", "method", "status"},
)

var SensorStatus = promauto.NewGaugeVec(
	prometheus.GaugeOpts{
		Name: "pastaay_remote_sensor_status",
		Help: "Status of remote control sensors (1 healthy, 0 error)",
	},
	[]string{"sensor"},
)

// Browser-sourced metrics
var BrowserApdex = promauto.NewGaugeVec(
	prometheus.GaugeOpts{
		Name: "pastaay_browser_apdex_score",
		Help: "Apdex score reported by client-side browser probers",
	},
	[]string{"target"},
)

var BrowserLatency = promauto.NewGaugeVec(
	prometheus.GaugeOpts{
		Name: "pastaay_browser_latency_ms",
		Help: "Latency (ms) reported by client-side browser probers",
	},
	[]string{"target"},
)
