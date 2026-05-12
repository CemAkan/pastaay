package metrics

import (
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
)

var InjectedFaultsTotal = promauto.NewCounterVec(
	prometheus.CounterOpts{
		Name: "pastaay_injected_faults_total",
		Help: "The total number of injected faults",
	},
	[]string{"target", "fault_type"},
)

// SensorStatus tracks if remote config providers are healthy (1) or failing (0)
var SensorStatus = promauto.NewGaugeVec(
	prometheus.GaugeOpts{
		Name: "pastaay_remote_sensor_status",
		Help: "Status of remote control sensors (1 for healthy, 0 for error)",
	},
	[]string{"sensor"},
)
