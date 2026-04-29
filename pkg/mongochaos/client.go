package mongochaos

import (
	"net"

	"github.com/CemAkan/pastaay/pkg/config"
	"go.mongodb.org/mongo-driver/v2/mongo/options"
)

// ApplyChaos integrates Pastaay fault injection into MongoDB ClientOptions.
func ApplyChaos(opts *options.ClientOptions, mgr *config.Manager) *options.ClientOptions {
	if opts == nil {
		opts = options.Client()
	}

	// Inject both the monitor and the dialer into the client configuration.
	opts.SetMonitor(NewChaosMonitor(mgr))
	opts.SetDialer(&ChaosDialer{
		DefaultDialer: &net.Dialer{},
		Manager:       mgr,
	})

	return opts
}
