package redischaos

import (
	"context"
	"log"
	"math/rand"
	"net"
	"time"

	"github.com/CemAkan/pastaay/pkg/config"
	"github.com/CemAkan/pastaay/pkg/metrics"
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
		currentConfig := h.cfgManager.Get()
		var activePolicy *config.Policy

		for _, policy := range currentConfig.Policies {
			// Target can be a specific command like "get", "set", or "all"
			if policy.Type == "redis" && (policy.Target == cmd.Name() || policy.Target == "all") {
				p := policy
				activePolicy = &p
				break
			}
		}

		if activePolicy != nil {
			// Latency Injection
			if activePolicy.LatencyChance > 0 && rand.Float64() < activePolicy.LatencyChance {
				log.Printf("Pastaay Redis: Injecting %v latency to %s command", activePolicy.LatencyDuration, cmd.Name())
				metrics.InjectedFaultsTotal.WithLabelValues("redis_"+cmd.Name(), "latency").Inc()
				time.Sleep(activePolicy.LatencyDuration)
			}

			// Error (Cache Miss) Injection
			if activePolicy.ErrorChance > 0 && rand.Float64() < activePolicy.ErrorChance {
				log.Printf("Pastaay Redis: Simulating Cache Miss (redis.Nil) for %s command", cmd.Name())
				metrics.InjectedFaultsTotal.WithLabelValues("redis_"+cmd.Name(), "error").Inc()
				// Simulate "no data" to force a database hit (Cache Stampede simulation)
				return redis.Nil
			}
		}

		return next(ctx, cmd)
	}
}

// ProcessPipelineHook intercepts Redis pipeline commands.
func (h *ChaosHook) ProcessPipelineHook(next redis.ProcessPipelineHook) redis.ProcessPipelineHook {
	return func(ctx context.Context, cmds []redis.Cmder) error {
		return next(ctx, cmds)
	}
}
