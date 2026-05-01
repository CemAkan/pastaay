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

	var extractedHeaders map[string]string
	headersExtracted := false

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
			if msgCtx.ExtractHeaders == nil {
				continue
			}

			if !headersExtracted {
				extractedHeaders = msgCtx.ExtractHeaders()
				headersExtracted = true
			}

			headersMatch := true
			for k, v := range p.MatchHeaders {
				if extractedHeaders[k] != v {
					headersMatch = false
					break
				}
			}

			if !headersMatch {
				continue
			}
		}

		if p.ErrorChance > 0 && rand.Float64() < p.ErrorChance {
			if p.DropConnection {
				return ActionDrop, 0, nil
			}

			errMsg := p.ErrorBody
			if errMsg == "" {
				errMsg = "pastaay synthetic broker fault: unrecoverable message error"
			}
			return ActionError, 0, errors.New(errMsg)
		}

		if p.LatencyDuration > 0 && rand.Float64() < p.LatencyChance {
			return ActionDelay, p.LatencyDuration, nil
		}
	}

	return ActionPass, 0, nil
}
