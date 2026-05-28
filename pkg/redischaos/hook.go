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
			if !(strings.EqualFold(p.Target, "all") || strings.EqualFold(p.Target, cmd.Name())) {
				continue
			}

			metricTag := p.MetricTag

			latencyHit := p.LatencyChance > 0 && rand.Float64() < p.LatencyChance
			errorHit := p.ErrorChance > 0 && rand.Float64() < p.ErrorChance
			if latencyHit && errorHit {
				latencyHit = false
			}

			if latencyHit {
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

			if errorHit {
				metrics.InjectedFaultsTotal.WithLabelValues(metricTag, "error").Inc()
				_, span := tracing.StartChaosSpan(ctx, "pastaay.redis.error", cmd.Name(), "error")
				telemetry.EmitError("redis", cmd.Name(), "Redis Fault Injected", p.ErrorBody, span)
				span.End()
				return errors.New("pastaay: synthetic redis fault")
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
		errorTags := make([]string, 0, 4)
		hasError := false

		protected := make([]bool, len(cmds))
		anyUnprotected := false
		for i := range cmds {
			if h.mgr.IsCommandIgnored("redis", cmds[i].Name()) {
				protected[i] = true
				continue
			}
			anyUnprotected = true
		}

		if anyUnprotected {
			seenTags := make(map[string]struct{}, len(policies))
			for _, p := range policies {
				matchesAny := false
				for i := range cmds {
					if protected[i] {
						continue
					}
					if strings.EqualFold(p.Target, "all") || strings.EqualFold(p.Target, cmds[i].Name()) {
						matchesAny = true
						break
					}
				}
				if !matchesAny {
					continue
				}
				latencyHit := p.LatencyChance > 0 && rand.Float64() < p.LatencyChance
				errorHit := p.ErrorChance > 0 && rand.Float64() < p.ErrorChance
				if latencyHit && errorHit {
					latencyHit = false
				}
				if latencyHit {
					if p.LatencyDuration > maxLatency {
						maxLatency = p.LatencyDuration
						latencyTag = p.MetricTag
					}
				}
				if errorHit {
					hasError = true
					if _, dup := seenTags[p.MetricTag]; !dup {
						seenTags[p.MetricTag] = struct{}{}
						errorTags = append(errorTags, p.MetricTag)
					}
				}
			}
		}

		if maxLatency > 0 {
			metrics.InjectedFaultsTotal.WithLabelValues(latencyTag, "latency").Inc()
			spanCtx, span := tracing.StartChaosSpan(ctx, "pastaay.redis.pipeline_latency", "pipeline", "latency")
			telemetry.EmitInfo("redis", "Redis Pipeline Latency Injected", map[string]interface{}{
				"duration": maxLatency.String(), "target": "pipeline",
			}, span)
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

			// Stable, multi-tag attribution rather than "last wins".
			tag := "redis:pipeline"
			if len(errorTags) > 0 {
				tag = errorTags[0]
				if len(errorTags) > 1 {
					tag += "+" + errorTags[len(errorTags)-1]
				}
			}
			metrics.InjectedFaultsTotal.WithLabelValues(tag, "error").Inc()
			_, span := tracing.StartChaosSpan(ctx, "pastaay.redis.pipeline_error", "pipeline", "error")
			defer span.End()

			chaosErr := errors.New("pastaay: redis pipeline batch aborted due to chaos policy")
			for i := range cmds {
				if protected[i] {
					continue
				}
				cmds[i].SetErr(chaosErr)
			}

			telemetry.EmitError("redis", "pipeline", "Redis Pipeline Batch Aborted", "batch aborted due to chaos policy", span)
			return chaosErr
		}
		return next(ctx, cmds)
	}
}

// DialHook drops connections when drop_connection policy matches.
func (h *ChaosHook) DialHook(next redis.DialHook) redis.DialHook {
	return func(ctx context.Context, network, addr string) (net.Conn, error) {
		if h == nil || h.mgr == nil {
			return next(ctx, network, addr)
		}
		policies := h.mgr.GetActivePolicies("redis")
		for _, p := range policies {
			if p.DropConnection && (strings.EqualFold(p.Target, "all") || strings.EqualFold(p.Target, "database")) {
				chance := p.ErrorChance
				if chance == 0 {
					chance = 1.0
				}
				if rand.Float64() < chance {
					metrics.InjectedFaultsTotal.WithLabelValues(p.MetricTag, "drop").Inc()
					return nil, errors.New("[Pastaay-Redis] Chaos: TCP connection forcefully dropped (hook)")
				}
			}
		}
		return next(ctx, network, addr)
	}
}
