package redischaos

import (
	"context"
	"errors"
	"net"
	"strings"
	"time"

	"github.com/CemAkan/pastaay/pkg/config"
)

func NewChaosDialer(mgr *config.Manager, baseDialer *net.Dialer) func(ctx context.Context, network, addr string) (net.Conn, error) {
	if baseDialer == nil {
		baseDialer = &net.Dialer{Timeout: 5 * time.Second}
	}
	return func(ctx context.Context, network, addr string) (net.Conn, error) {
		policies := mgr.GetActivePolicies("redis")
		for _, p := range policies {
			if p.DropConnection && strings.EqualFold(p.Target, "all") {
				return nil, errors.New("[Pastaay-Redis] Chaos: TCP connection forcefully dropped")
			}
		}
		return baseDialer.DialContext(ctx, network, addr)
	}
}
