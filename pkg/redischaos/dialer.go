package redischaos

import (
	"context"
	"errors"
	"math/rand/v2"
	"net"
	"strings"
	"time"

	"github.com/CemAkan/pastaay/pkg/config"
	"github.com/CemAkan/pastaay/pkg/metrics"
)

type DialerFunc func(ctx context.Context, network, addr string) (net.Conn, error)

// NewChaosDialer returns a dialer that may refuse to establish a connection when an active policy has drop_connection enabled.
func NewChaosDialer(mgr *config.Manager, baseDialer DialerFunc) DialerFunc {
	if baseDialer == nil {
		d := &net.Dialer{Timeout: 5 * time.Second}
		baseDialer = d.DialContext
	}
	return func(ctx context.Context, network, addr string) (net.Conn, error) {
		policies := mgr.GetActivePolicies("redis")
		for _, p := range policies {
			if p.DropConnection && (strings.EqualFold(p.Target, "all") || strings.EqualFold(p.Target, "database")) {
				chance := p.ErrorChance
				if chance == 0 {
					chance = 1.0
				}
				if rand.Float64() < chance {
					metrics.InjectedFaultsTotal.WithLabelValues(p.MetricTag, "drop").Inc()
					return nil, errors.New("[Pastaay-Redis] Chaos: TCP connection forcefully dropped")
				}
			}
		}
		return baseDialer(ctx, network, addr)
	}
}
