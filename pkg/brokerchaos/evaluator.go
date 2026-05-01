package brokerchaos

import (
	"context"
	"errors"
	"math/rand/v2"
	"strings"
	"time"

	"github.com/CemAkan/pastaay/pkg/config"
)

// ConfigProvider abstracts the configuration manager.
type ConfigProvider interface {
	GetActivePolicies() []config.Policy
	IsCommandIgnored(protocol string, cmd string) bool
}

type defaultEvaluator struct {
	provider ConfigProvider
}

func NewEvaluator(provider ConfigProvider) Evaluator {
	return &defaultEvaluator{
		provider: provider,
	}
}

func (e *defaultEvaluator) Evaluate(ctx context.Context, msgCtx *MessageContext) (ChaosAction, time.Duration, error) {
	if msgCtx == nil {
		return ActionPass, 0, nil
	}

	if e.provider.IsCommandIgnored(string(msgCtx.Protocol), msgCtx.Topic) {
		return ActionPass, 0, nil
	}

	select {
	case <-ctx.Done():
		return ActionPass, 0, ctx.Err()
	default:
	}

	policies := e.provider.GetActivePolicies()
	if len(policies) == 0 {
		return ActionPass, 0, nil
	}

	for _, p := range policies {
		if !strings.EqualFold(p.Type, string(msgCtx.Protocol)) {
			continue
		}

		isGlobal := strings.EqualFold(p.Target, "all")
		isExactMatch := strings.EqualFold(p.Target, msgCtx.Topic)

		if !isGlobal && !isExactMatch {
			continue
		}

		if len(p.MatchHeaders) > 0 {
			headersMatch := true
			for k, v := range p.MatchHeaders {
				if msgCtx.Headers[k] != v {
					headersMatch = false
					break
				}
			}
			if !headersMatch {
				continue
			}
		}

		roll := rand.Float64()

		if p.ErrorChance > 0 && roll < p.ErrorChance {
			if p.DropConnection {
				return ActionDrop, 0, nil
			}

			errMsg := p.ErrorBody
			if errMsg == "" {
				errMsg = "pastaay synthetic broker fault: unrecoverable message error"
			}
			return ActionError, 0, errors.New(errMsg)
		}

		if p.LatencyDuration > 0 {
			return ActionDelay, p.LatencyDuration, nil
		}
	}

	return ActionPass, 0, nil
}