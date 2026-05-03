package redischaos

import (
	"context"
	"math/rand/v2"
	"net"
	"strings"
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

func (h *ChaosHook) ProcessHook(next redis.ProcessHook) redis.ProcessHook {
	return func(ctx context.Context, cmd redis.Cmder) error {
		if h.mgr.IsCommandIgnored("redis", cmd.Name()) {
			return next(ctx, cmd)
		}

		policies := h.mgr.GetActivePolicies("redis")
		for _, p := range policies {
			if strings.EqualFold(p.Target, "all") || strings.EqualFold(p.Target, cmd.Name()) {
				metricTag := "redis:" + p.Target

				if p.LatencyChance > 0 && rand.Float64() < p.LatencyChance {
					metrics.InjectedFaultsTotal.WithLabelValues(metricTag, "latency").Inc()
					timer := time.NewTimer(p.LatencyDuration)
					select {
					case <-timer.C:
						timer.Stop()
					case <-ctx.Done():
						timer.Stop()
						return ctx.Err()
					}
				}

				if p.ErrorChance > 0 && rand.Float64() < p.ErrorChance {
					metrics.InjectedFaultsTotal.WithLabelValues(metricTag, "error").Inc()
					return redis.Nil
				}
			}
		}
		return next(ctx, cmd)
	}
}

func (h *ChaosHook) ProcessPipelineHook(next redis.ProcessPipelineHook) redis.ProcessPipelineHook {
	return func(ctx context.Context, cmds []redis.Cmder) error {
		policies := h.mgr.GetActivePolicies("redis")
		var maxLatency time.Duration
		var bestMetricTag string
		injectErrMap := make([]bool, len(cmds))
		hasError := false

		for i, cmd := range cmds {
			if h.mgr.IsCommandIgnored("redis", cmd.Name()) {
				continue
			}
			for _, p := range policies {
				if strings.EqualFold(p.Target, "all") || strings.EqualFold(p.Target, cmd.Name()) {
					if p.LatencyChance > 0 && rand.Float64() < p.LatencyChance {
						if p.LatencyDuration > maxLatency {
							maxLatency = p.LatencyDuration
							bestMetricTag = "redis:" + p.Target
						}
					}
					if p.ErrorChance > 0 && rand.Float64() < p.ErrorChance {
						injectErrMap[i] = true
						hasError = true
					}
				}
			}
		}

		if maxLatency > 0 {
			metrics.InjectedFaultsTotal.WithLabelValues(bestMetricTag, "latency").Inc()
			timer := time.NewTimer(maxLatency)
			select {
			case <-timer.C:
				timer.Stop()
			case <-ctx.Done():
				timer.Stop()
				return ctx.Err()
			}
		}

		err := next(ctx, cmds)
		if hasError {
			metrics.InjectedFaultsTotal.WithLabelValues("redis:all", "error").Inc()
			for i := range cmds {
				if injectErrMap[i] {
					cmds[i].SetErr(redis.Nil)
				}
			}
			if err == nil {
				return redis.Nil
			}
		}
		return err
	}
}

func (h *ChaosHook) DialHook(next redis.DialHook) redis.DialHook {
	return func(ctx context.Context, network, addr string) (net.Conn, error) { return next(ctx, network, addr) }
}
