package mongochaos

import (
	"context"
	"errors"
	"net"
	"strings"
	"time"

	"github.com/CemAkan/pastaay/pkg/config"
)

type ChaosDialer struct {
	DefaultDialer *net.Dialer
	Manager       *config.Manager
}

func (c *ChaosDialer) DialContext(ctx context.Context, network, address string) (net.Conn, error) {
	policies := c.Manager.GetActivePolicies("mongo")

	for _, p := range policies {
		if p.DropConnection && strings.EqualFold(p.Target, "all") {
			return nil, errors.New("[Pastaay-Mongo] Chaos: connection forcefully dropped by policy")
		}
	}

	dialer := c.DefaultDialer
	if dialer == nil {
		dialer = &net.Dialer{
			Timeout:   10 * time.Second,
			KeepAlive: 30 * time.Second,
		}
	}

	return dialer.DialContext(ctx, network, address)
}
