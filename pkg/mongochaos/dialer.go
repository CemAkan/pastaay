package mongochaos

import (
	"context"
	"errors"
	"net"

	"github.com/CemAkan/pastaay/pkg/config"
)

// ChaosDialer intercepts MongoDB TCP connection attempts.
type ChaosDialer struct {
	DefaultDialer *net.Dialer
	Manager       *config.Manager
}

func (c *ChaosDialer) DialContext(ctx context.Context, network, address string) (net.Conn, error) {
	// Retrieve only 'mongo' type policies from the manager.
	policies := c.Manager.GetActivePolicies("mongo")

	for _, p := range policies {
		// If the policy requires dropping the connection, abort here.
		if p.DropConnection {
			return nil, errors.New("[Pastaay-Mongo] Chaos: connection forcefully dropped by policy")
		}
	}

	return c.DefaultDialer.DialContext(ctx, network, address)
}
