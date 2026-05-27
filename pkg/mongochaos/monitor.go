package mongochaos

import (
	"context"
	"math/rand/v2"
	"strings"
	"time"

	"github.com/CemAkan/pastaay/pkg/config"
	"github.com/CemAkan/pastaay/pkg/metrics"
	"github.com/CemAkan/pastaay/pkg/telemetry"
	"github.com/CemAkan/pastaay/pkg/tracing"
	"go.mongodb.org/mongo-driver/v2/event"
)

// NewChaosMonitor returns a MongoDB command monitor that evaluates everyn// started command against active Pastaay policies, injecting latency or aborting the command when a policy matches.
func NewChaosMonitor(mgr *config.Manager) *event.CommandMonitor {
	return &event.CommandMonitor{
		Started: func(ctx context.Context, evt *event.CommandStartedEvent) {
			if mgr.IsCommandIgnored("mongo", evt.CommandName) {
				return
			}

			policies := mgr.GetActivePolicies("mongo")
			for _, p := range policies {
				if !(strings.EqualFold(p.Target, "all") || strings.EqualFold(p.Target, evt.CommandName)) {
					continue
				}

				metricTag := p.MetricTag

				if p.LatencyChance > 0 && rand.Float64() < p.LatencyChance {
					metrics.InjectedFaultsTotal.WithLabelValues(metricTag, "latency").Inc()
					spanCtx, span := tracing.StartChaosSpan(ctx, "pastaay.mongo.latency", evt.CommandName, "latency")

					telemetry.EmitInfo("mongo", "Mongo Latency Injected", map[string]interface{}{"duration": p.LatencyDuration.String(), "target": evt.CommandName}, span)

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

					telemetry.EmitError("mongo", evt.CommandName, "Mongo Fault Injected", "force_abort_not_supported", span)

					span.End()
					return
				}
			}
		},
	}
}
