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
}

// defaultEvaluator is the internal implementation of the Evaluator interface.
type defaultEvaluator struct {
	provider ConfigProvider
}

// NewEvaluator constructs a highly concurrent and memory-safe policy evaluator.
func NewEvaluator(provider ConfigProvider) Evaluator {
	return &defaultEvaluator{
		provider: provider,
	}
}

// Evaluate applies the active Pastaay policies to the incoming broker message.
func (e *defaultEvaluator) Evaluate(ctx context.Context, msgCtx *MessageContext) (ChaosAction, time.Duration, error) {

	if msgCtx == nil {
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
		// 1. Protocol Match
		if !strings.EqualFold(p.Type, string(msgCtx.Protocol)) {
			continue
		}

		// 2. Target Topic/Queue Match
		isGlobal := strings.EqualFold(p.Target, "all")
		isExactMatch := strings.EqualFold(p.Target, msgCtx.Topic)

		if !isGlobal && !isExactMatch {
			continue
		}

		// 3. Header Match (Blast Radius Protection)
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

		// 4. Probability Execution
		roll := rand.Float64()

		// Execute Fault Injection if probability hits
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

		// 5. Latency Injection Verification
		if p.LatencyDuration > 0 {
			return ActionDelay, p.LatencyDuration, nil
		}
	}

	return ActionPass, 0, nil
}
