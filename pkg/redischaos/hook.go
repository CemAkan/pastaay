package redischaos

import (
	"context"
	"log"
	"math/rand"
	"net"
	"time"

	"github.com/CemAkan/pastaay/pkg/config"
	"github.com/redis/go-redis/v9"
)

type ChaosHook struct {
	mgr *config.Manager
}

func NewChaosHook(mgr *config.Manager) *ChaosHook {
	return &ChaosHook{mgr: mgr}
}

// ProcessHook intercepts every Redis command to inject latency or errors.
func (h *ChaosHook) ProcessHook(next redis.ProcessHook) redis.ProcessHook {
	return func(ctx context.Context, cmd redis.Cmder) error {
		policies := h.mgr.GetActivePolicies("redis")

		for _, p := range policies {
			// Check if target is 'all' or matches specific command name
			if p.Target == "all" || p.Target == cmd.Name() {

				// Latency Injection
				if p.LatencyChance > 0 && rand.Float64() < p.LatencyChance {
					log.Printf("[Pastaay-Redis] Latency: injecting %v to command %s", p.LatencyDuration, cmd.Name())
					time.Sleep(p.LatencyDuration)
				}

				// Error Injection (Forced Cache Miss)
				if p.ErrorChance > 0 && rand.Float64() < p.ErrorChance {
					return redis.Nil // Simulates a cache miss for resiliency testing
				}
			}
		}
		return next(ctx, cmd)
	}
}

func (h *ChaosHook) DialHook(next redis.DialHook) redis.DialHook {
	return func(ctx context.Context, network, addr string) (net.Conn, error) { return next(ctx, network, addr) }
}

func (h *ChaosHook) ProcessPipelineHook(next redis.ProcessPipelineHook) redis.ProcessPipelineHook {
	return func(ctx context.Context, cmds []redis.Cmder) error { return next(ctx, cmds) }
}
