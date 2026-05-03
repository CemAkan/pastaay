package metrics

import (
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
)

// InjectedFaultsTotal tracks the number of chaos events injected by Pastaay.
// Label "target" follows the "protocol:target" naming convention (e.g., "sql:all", "kafka:orders")
var InjectedFaultsTotal = promauto.NewCounterVec(
	prometheus.CounterOpts{
		Name: "pastaay_injected_faults_total",
		Help: "The total number of injected faults (latency, drop, or error)",
	},
	[]string{"target", "fault_type"},
)
