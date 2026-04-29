package redischaos

import (
	"context"
	"errors"
	"net"

	"github.com/CemAkan/pastaay/pkg/config"
)

// NewChaosDialer returns a dial function that respects Pastaay network drop policies.
func NewChaosDialer(mgr *config.Manager) func(ctx context.Context, network, addr string) (net.Conn, error) {
	return func(ctx context.Context, network, addr string) (net.Conn, error) {
		// Filter policies by 'redis' type
		policies := mgr.GetActivePolicies("redis")
		for _, p := range policies {
			if p.DropConnection {
				return nil, errors.New("[Pastaay-Redis] Chaos: TCP connection forcefully dropped")
			}
		}

		d := net.Dialer{}
		return d.DialContext(ctx, network, addr)
	}
}
