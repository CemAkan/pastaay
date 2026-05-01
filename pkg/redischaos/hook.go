package redischaos

import (
	"context"
	"log"
	"math/rand/v2"
	"net"
	"strings"
	"time"

	"github.com/CemAkan/pastaay/pkg/config"
	"github.com/CemAkan/pastaay/pkg/metrics"
	"github.com/redis/go-redis/v9"
)

type ChaosHook struct {
	mgr *config.Manager
}

func NewChaosHook(mgr *config.Manager) *ChaosHook {
	return &ChaosHook{mgr: mgr}
}

func (h *ChaosHook) ProcessHook(next redis.ProcessHook) redis.ProcessHook {
	return func(ctx context.Context, cmd redis.Cmder) error {

		if h.mgr.IsCommandIgnored("redis", cmd.Name()) {
			return next(ctx, cmd)
		}

		policies := h.mgr.GetActivePolicies("redis")

		for _, p := range policies {
			if strings.EqualFold(p.Target, "all") || strings.EqualFold(p.Target, cmd.Name()) {
				if p.LatencyChance > 0 && rand.Float64() < p.LatencyChance {
					log.Printf("[Pastaay-Redis] Latency: injecting %v to %s", p.LatencyDuration, cmd.Name())
					metrics.InjectedFaultsTotal.WithLabelValues("redis", "latency").Inc()

					timer := time.NewTimer(p.LatencyDuration)
					select {
					case <-timer.C:
					case <-ctx.Done():
						timer.Stop()
						return ctx.Err()
					}
					timer.Stop()
				}

				if p.ErrorChance > 0 && rand.Float64() < p.ErrorChance {
					log.Printf("[Pastaay-Redis] Error: simulating cache miss for %s", cmd.Name())
					metrics.InjectedFaultsTotal.WithLabelValues("redis", "error").Inc()
					return redis.Nil
				}
			}
		}
		return next(ctx, cmd)
	}
}

func (h *ChaosHook) DialHook(next redis.DialHook) redis.DialHook {
	return func(ctx context.Context, network, addr string) (net.Conn, error) {
		return next(ctx, network, addr)
	}
}

func (h *ChaosHook) ProcessPipelineHook(next redis.ProcessPipelineHook) redis.ProcessPipelineHook {
	return func(ctx context.Context, cmds []redis.Cmder) error {
		policies := h.mgr.GetActivePolicies("redis")
		latencyApplied := false

		// Latency
		for i := range cmds {
			if h.mgr.IsCommandIgnored("redis", cmds[i].Name()) {
				continue
			}
			if latencyApplied {
				break
			}
			for _, p := range policies {
				if strings.EqualFold(p.Target, "all") || strings.EqualFold(p.Target, cmds[i].Name()) {
					if p.LatencyChance > 0 && rand.Float64() < p.LatencyChance {
						metrics.InjectedFaultsTotal.WithLabelValues("redis", "latency").Inc()
						timer := time.NewTimer(p.LatencyDuration)
						select {
						case <-timer.C:
						case <-ctx.Done():
							timer.Stop()
							return ctx.Err()
						}
						timer.Stop()
						latencyApplied = true
						break // Probability Distortion fix
					}
				}
			}
		}

		err := next(ctx, cmds)

		var injectedErr error

		// Error
		for i := range cmds {
			if h.mgr.IsCommandIgnored("redis", cmds[i].Name()) {
				continue
			}
			for _, p := range policies {
				if strings.EqualFold(p.Target, "all") || strings.EqualFold(p.Target, cmds[i].Name()) {
					if p.ErrorChance > 0 && rand.Float64() < p.ErrorChance {
						metrics.InjectedFaultsTotal.WithLabelValues("redis", "error").Inc()
						cmds[i].SetErr(redis.Nil)
						injectedErr = redis.Nil
						break // Probability Distortion fix
					}
				}
			}
		}

		if err == nil && injectedErr != nil {
			return injectedErr
		}

		return err
	}
}
