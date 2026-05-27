package brokerchaos

import (
	"context"
	"errors"
	"math/rand/v2"
	"strings"
	"time"

	"github.com/CemAkan/pastaay/pkg/config"
)

type ConfigProvider interface {
	GetActivePolicies() []config.Policy
	IsCommandIgnored(protocol string, cmd string) bool
}

type defaultEvaluator struct {
	provider ConfigProvider
}

// NewEvaluator panics fast on nil provider to surface misconfiguration at boot rather than per-message in production.
func NewEvaluator(provider ConfigProvider) Evaluator {
	if provider == nil {
		panic("brokerchaos: NewEvaluator received nil ConfigProvider")
	}
	return &defaultEvaluator{provider: provider}
}

func (e *defaultEvaluator) Evaluate(ctx context.Context, msgCtx *MessageContext) (bool, time.Duration, error, string, string) {
	if e == nil || e.provider == nil || msgCtx == nil {
		return false, 0, nil, "", ""
	}
	if e.provider.IsCommandIgnored(string(msgCtx.Protocol), msgCtx.Topic) {
		return false, 0, nil, "", ""
	}

	policies := e.provider.GetActivePolicies()
	var delayDuration time.Duration
	var shouldDrop bool
	var chaosErr error
	var latencyTag string
	var errorTag string

	for _, p := range policies {
		if !strings.EqualFold(p.Type, string(msgCtx.Protocol)) {
			continue
		}
		if !strings.EqualFold(p.Target, "all") && !strings.EqualFold(p.Target, msgCtx.Topic) {
			continue
		}

		if len(p.MatchHeaders) > 0 {
			if msgCtx.GetHeader == nil {
				continue
			}
			match := true
			for k, v := range p.MatchHeaders {
				val, exists := msgCtx.GetHeader(k)
				if !exists || val != v {
					match = false
					break
				}
			}
			if !match {
				continue
			}
		}

		if p.LatencyDuration > delayDuration && rand.Float64() < p.LatencyChance {
			delayDuration = p.LatencyDuration
			latencyTag = p.MetricTag
		}

		if p.ErrorChance > 0 && rand.Float64() < p.ErrorChance {
			errorTag = p.MetricTag
			if p.DropConnection {
				shouldDrop = true
			} else {
				msg := p.ErrorBody
				if msg == "" {
					msg = "pastaay: synthetic broker fault"
				}
				chaosErr = errors.New(msg)
			}
			break
		}
	}

	return shouldDrop, delayDuration, chaosErr, latencyTag, errorTag
}
