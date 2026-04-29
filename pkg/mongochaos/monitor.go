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
			policies := mgr.GetActivePolicies("mongo")
			for _, p := range policies {
				// If a latency duration is specified, sleep before allowing the command.
				if p.LatencyDuration > 0 {
					time.Sleep(p.LatencyDuration)
					break
				}
			}
		},
	}
}
