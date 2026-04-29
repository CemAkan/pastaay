package mongochaos

import (
	"context"
	"time"

	"github.com/CemAkan/pastaay/pkg/config"
	"go.mongodb.org/mongo-driver/v2/event"
)

// NewChaosMonitor creates a monitor that injects latency based on dynamic policies.
func NewChaosMonitor(mgr *config.Manager) *event.CommandMonitor {
	return &event.CommandMonitor{
		Started: func(ctx context.Context, evt *event.CommandStartedEvent) {

			if mgr.IsCommandIgnored("mongo", evt.CommandName) {
				return // Bypass chaos
			}

			policies := mgr.GetActivePolicies("mongo")
			for _, p := range policies {
				if p.LatencyDuration > 0 {
					time.Sleep(p.LatencyDuration)
					break
				}
			}
		},
	}
}
