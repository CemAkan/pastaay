package mongochaos

import (
	"context"
	"errors"
	"math/rand/v2"
	"net"
	"strings"
	"time"

	"github.com/CemAkan/pastaay/pkg/config"
	"github.com/CemAkan/pastaay/pkg/metrics"
	"go.mongodb.org/mongo-driver/v2/mongo/options"
)

// ChaosDialer wraps the MongoDB dialer to optionally refuse connections based on active chaos policies.
type ChaosDialer struct {
	DefaultDialer options.ContextDialer
	Manager       *config.Manager
}

func (c *ChaosDialer) DialContext(ctx context.Context, network, address string) (net.Conn, error) {
	policies := c.Manager.GetActivePolicies("mongo")
	for _, p := range policies {
		if p.DropConnection && (strings.EqualFold(p.Target, "all") || strings.EqualFold(p.Target, "database")) {
			chance := p.ErrorChance
			if chance == 0 {
				chance = 1.0
			}
			if rand.Float64() < chance {
				metrics.InjectedFaultsTotal.WithLabelValues(p.MetricTag, "drop").Inc()
				return nil, errors.New("[Pastaay-Mongo] Chaos: connection forcefully dropped by policy")
			}
		}
	}

	if c.DefaultDialer != nil {
		return c.DefaultDialer.DialContext(ctx, network, address)
	}
	fallback := &net.Dialer{Timeout: 10 * time.Second, KeepAlive: 30 * time.Second}
	return fallback.DialContext(ctx, network, address)
}
