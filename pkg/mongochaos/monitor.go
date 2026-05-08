package mongochaos

import (
	"context"
	"log"
	"math/rand/v2"
	"strings"
	"time"

	"github.com/CemAkan/pastaay/pkg/config"
	"github.com/CemAkan/pastaay/pkg/metrics"
	"github.com/CemAkan/pastaay/pkg/tracing"
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
						spanCtx, span := tracing.StartChaosSpan(ctx, "pastaay.mongo.latency", evt.CommandName, "latency")

						timer := time.NewTimer(p.LatencyDuration)
						select {
						case <-timer.C:
							timer.Stop()
							span.End()
						case <-spanCtx.Done():
							timer.Stop()
							span.End()
							return
						}
					}

					if p.ErrorChance > 0 && rand.Float64() < p.ErrorChance {
						metrics.InjectedFaultsTotal.WithLabelValues(metricTag, "error").Inc()
						_, span := tracing.StartChaosSpan(ctx, "pastaay.mongo.error", evt.CommandName, "error")
						span.End()

						log.Printf("[Pastaay-Mongo] Chaos: aborting command %s by blocking execution", evt.CommandName)

						select {
						case <-ctx.Done():
						case <-time.After(30 * time.Second):
						}
						return
					}
				}
			}
		},
	}
}
