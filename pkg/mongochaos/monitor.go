package mongochaos

import (
	"context"
	"log"
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
				if strings.EqualFold(p.Target, "all") || strings.EqualFold(p.Target, evt.CommandName) {
					metricTag := "mongo:" + p.Target

					if p.LatencyChance > 0 && rand.Float64() < p.LatencyChance {
						metrics.InjectedFaultsTotal.WithLabelValues(metricTag, "latency").Inc()
						timer := time.NewTimer(p.LatencyDuration)
						select {
						case <-timer.C:
							timer.Stop()
						case <-ctx.Done():
							timer.Stop()
							return
						}
					}

					if p.ErrorChance > 0 && rand.Float64() < p.ErrorChance {
						metrics.InjectedFaultsTotal.WithLabelValues(metricTag, "error").Inc()
						log.Printf("[Pastaay-Mongo] Chaos: aborting command %s by blocking execution", evt.CommandName)
						<-ctx.Done()
						return
					}
				}
			}
		},
	}
}
