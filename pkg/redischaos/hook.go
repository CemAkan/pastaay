package redischaos

import (
	"context"
	"net"

	"github.com/CemAkan/pastaay/pkg/config"
	"github.com/redis/go-redis/v9"
)

// ChaosHook implements redis.Hook to inject faults into Redis queries.
type ChaosHook struct {
	cfgManager *config.Manager
}

// NewChaosHook creates a new Redis hook driven by Pastaay policies.
func NewChaosHook(cfgManager *config.Manager) *ChaosHook {
	return &ChaosHook{
		cfgManager: cfgManager,
	}
}

// DialHook intercepts the connection dialer (required by redis.Hook interface).
func (h *ChaosHook) DialHook(next redis.DialHook) redis.DialHook {
	return func(ctx context.Context, network, addr string) (net.Conn, error) {
		return next(ctx, network, addr)
	}
}

// ProcessHook intercepts individual Redis commands (e.g., GET, SET).
func (h *ChaosHook) ProcessHook(next redis.ProcessHook) redis.ProcessHook {
	return func(ctx context.Context, cmd redis.Cmder) error {

		// TODO: Chaos logic will be implemented here

		return next(ctx, cmd)
	}
}

// ProcessPipelineHook intercepts Redis pipeline commands.
func (h *ChaosHook) ProcessPipelineHook(next redis.ProcessPipelineHook) redis.ProcessPipelineHook {
	return func(ctx context.Context, cmds []redis.Cmder) error {
		return next(ctx, cmds)
	}
}
