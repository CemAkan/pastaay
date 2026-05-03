package mongochaos

import (
	"github.com/CemAkan/pastaay/pkg/config"
	"go.mongodb.org/mongo-driver/v2/mongo/options"
)

// ApplyChaos integrates Pastaay fault injection into MongoDB ClientOptions.
func ApplyChaos(opts *options.ClientOptions, mgr *config.Manager) *options.ClientOptions {
	if opts == nil {
		opts = options.Client()
	}
	
	var existingDialer options.ContextDialer
	if opts.Dialer != nil {
		existingDialer = opts.Dialer
	}

	opts.SetMonitor(NewChaosMonitor(mgr))
	opts.SetDialer(&ChaosDialer{
		DefaultDialer: existingDialer,
		Manager:       mgr,
	})

	return opts
}
