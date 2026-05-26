package redischaos

import (
	"context"
	"errors"
	"math/rand/v2"
	"net"
	"strings"
	"time"

	"github.com/CemAkan/pastaay/pkg/config"
	"github.com/CemAkan/pastaay/pkg/metrics"
	"github.com/CemAkan/pastaay/pkg/telemetry"
	"github.com/CemAkan/pastaay/pkg/tracing"
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

				metricTag := p.MetricTag

				if p.LatencyChance > 0 && rand.Float64() < p.LatencyChance {
					metrics.InjectedFaultsTotal.WithLabelValues(metricTag, "latency").Inc()
					spanCtx, span := tracing.StartChaosSpan(ctx, "pastaay.redis.latency", cmd.Name(), "latency")

					telemetry.EmitInfo("redis", "Redis Latency Injected", map[string]interface{}{"duration": p.LatencyDuration.String(), "target": cmd.Name()}, span)

					timer := time.NewTimer(p.LatencyDuration)
					select {
					case <-timer.C:
						timer.Stop()
						span.End()
					case <-spanCtx.Done():
						timer.Stop()
						span.End()
						return spanCtx.Err()
					}
				}
				if p.ErrorChance > 0 && rand.Float64() < p.ErrorChance {
					metrics.InjectedFaultsTotal.WithLabelValues(metricTag, "error").Inc()
					_, span := tracing.StartChaosSpan(ctx, "pastaay.redis.error", cmd.Name(), "error")

					telemetry.EmitError("redis", cmd.Name(), "Redis Fault Injected", p.ErrorBody, span)

					span.End()
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
		var latencyTag string
		var errorTag string
		hasError := false

		protected := make([]bool, len(cmds))

		for i := range cmds {
			cmd := cmds[i]
			if h.mgr.IsCommandIgnored("redis", cmd.Name()) {
				protected[i] = true
				continue
			}
			for _, p := range policies {
				if strings.EqualFold(p.Target, "all") || strings.EqualFold(p.Target, cmd.Name()) {
					if p.LatencyChance > 0 && rand.Float64() < p.LatencyChance {
						if p.LatencyDuration > maxLatency {
							maxLatency = p.LatencyDuration
							latencyTag = p.MetricTag
						}
					}
					if p.ErrorChance > 0 && rand.Float64() < p.ErrorChance {
						hasError = true
						errorTag = p.MetricTag
					}
				}
			}
		}

		if maxLatency > 0 {
			metrics.InjectedFaultsTotal.WithLabelValues(latencyTag, "latency").Inc()
			spanCtx, span := tracing.StartChaosSpan(ctx, "pastaay.redis.pipeline_latency", "pipeline", "latency")
			telemetry.EmitInfo("redis", "Redis Pipeline Latency Injected", map[string]interface{}{"duration": maxLatency.String(), "target": "pipeline"}, span)

			timer := time.NewTimer(maxLatency)
			select {
			case <-timer.C:
				timer.Stop()
				span.End()
			case <-spanCtx.Done():
				timer.Stop()
				span.End()
				return spanCtx.Err()
			}
		}

		if hasError {
			allProtected := true
			for _, pr := range protected {
				if !pr {
					allProtected = false
					break
				}
			}
			if allProtected {
				return next(ctx, cmds)
			}

			targetTag := "redis:all"
			if errorTag != "" {
				targetTag = errorTag
			}
			metrics.InjectedFaultsTotal.WithLabelValues(targetTag, "error").Inc()
			_, span := tracing.StartChaosSpan(ctx, "pastaay.redis.pipeline_error", "pipeline", "error")

			chaosErr := errors.New("pastaay: redis pipeline batch aborted due to chaos policy")
			for i := range cmds {
				if protected[i] {
					continue
				}
				cmds[i].SetErr(chaosErr)
			}

			telemetry.EmitError("redis", "pipeline", "Redis Pipeline Batch Aborted", "batch aborted due to chaos policy", span)
			span.End()

			return chaosErr
		}
		return next(ctx, cmds)
	}
}

func (h *ChaosHook) DialHook(next redis.DialHook) redis.DialHook {
	return func(ctx context.Context, network, addr string) (net.Conn, error) { return next(ctx, network, addr) }
}
