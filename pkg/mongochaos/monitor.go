package mongochaos

import (
	"context"
	"math/rand/v2"
	"strings"
	"time"

	"github.com/CemAkan/pastaay/pkg/config"
	"github.com/CemAkan/pastaay/pkg/metrics"
	"go.mongodb.org/mongo-driver/v2/event"
)

func NewChaosMonitor(mgr *config.Manager) *event.CommandMonitor {
	return &event.CommandMonitor{
		Started: func(ctx context.Context, evt *event.CommandStartedEvent) {
			if mgr.IsCommandIgnored("mongo", evt.CommandName) {
				return
			}

			policies := mgr.GetActivePolicies("mongo")
			for _, p := range policies {
				// Eşleşme Case-Insensitive
				if strings.EqualFold(p.Target, "all") || strings.EqualFold(p.Target, evt.CommandName) {
					if p.LatencyChance > 0 && rand.Float64() < p.LatencyChance {
						metrics.InjectedFaultsTotal.WithLabelValues("mongo", "latency").Inc()

						timer := time.NewTimer(p.LatencyDuration)
						select {
						case <-timer.C:
						case <-ctx.Done():
							timer.Stop()
							return
						}
						timer.Stop()
					}
				}
			}
		},
	}
}
