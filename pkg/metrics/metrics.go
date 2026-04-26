package metrics

import (
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
)

// InjectedFaultsTotal tracks the number of chaos events injected by Pastaay.
var InjectedFaultsTotal = promauto.NewCounterVec(
	prometheus.CounterOpts{
		Name: "pastaay_injected_faults_total",
		Help: "The total number of injected faults (latency or error)",
	},
	[]string{"path", "fault_type"},
)
