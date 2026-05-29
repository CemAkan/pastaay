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

// NewChaosMonitor returns a MongoDB command monitor that applies policies.
func NewChaosMonitor(mgr *config.Manager) *event.CommandMonitor {
	return &event.CommandMonitor{
		Started: func(ctx context.Context, evt *event.CommandStartedEvent) {
			if mgr == nil || evt == nil {
				return
			}
			if mgr.IsCommandIgnored("mongo", evt.CommandName) {
				return
			}

			policies := mgr.GetActivePolicies("mongo")
			for _, p := range policies {
				if !(strings.EqualFold(p.Target, "all") || strings.EqualFold(p.Target, evt.CommandName)) {
					continue
				}

				metricTag := p.MetricTag

				latencyHit := p.LatencyChance > 0 && rand.Float64() < p.LatencyChance
				errorHit := p.ErrorChance > 0 && rand.Float64() < p.ErrorChance
				if latencyHit && errorHit {
					latencyHit = false
				}

				if latencyHit {
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

				if errorHit {
					metrics.InjectedFaultsTotal.WithLabelValues(metricTag, "error").Inc()
					_, span := tracing.StartChaosSpan(ctx, "pastaay.mongo.error", evt.CommandName, "error")

					msg := p.ErrorBody
					if msg == "" {
						msg = "pastaay: synthetic mongo fault (observability-only, driver cannot abort started command)"
					}
					telemetry.EmitError("mongo", evt.CommandName, "Mongo Fault Injected", msg, span)

					span.End()
					return
				}
			}
		},
	}
}
