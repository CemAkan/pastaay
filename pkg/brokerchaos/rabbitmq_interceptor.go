package brokerchaos

import (
	"context"
	"fmt"
	"time"

	"github.com/CemAkan/pastaay/pkg/metrics"
	amqp "github.com/rabbitmq/amqp091-go"
)

type RabbitMQMiddleware struct {
	evaluator Evaluator
}

func NewRabbitMQMiddleware(eval Evaluator) *RabbitMQMiddleware {
	return &RabbitMQMiddleware{evaluator: eval}
}

func (m *RabbitMQMiddleware) Intercept(ctx context.Context, delivery *amqp.Delivery) (drop bool, err error) {
	if delivery == nil {
		return false, nil
	}

	msgCtx := &MessageContext{
		Topic:    delivery.RoutingKey,
		Protocol: ProtocolRabbitMQ,
		GetHeader: func(key string) (string, bool) {
			if val, ok := delivery.Headers[key]; ok {
				if strVal, isStr := val.(string); isStr {
					return strVal, true
				}
				return fmt.Sprintf("%v", val), true
			}
			return "", false
		},
	}

	shouldDrop, delay, evalErr := m.evaluator.Evaluate(ctx, msgCtx)
	metricTag := "rabbitmq:" + delivery.RoutingKey

	if delay > 0 {
		metrics.InjectedFaultsTotal.WithLabelValues(metricTag, "latency").Inc()
		timer := time.NewTimer(delay)
		select {
		case <-timer.C:
			timer.Stop()
		case <-ctx.Done():
			timer.Stop()
			return false, ctx.Err()
		}
	}

	if shouldDrop {
		metrics.InjectedFaultsTotal.WithLabelValues(metricTag, "drop").Inc()
		return true, nil
	}
	if evalErr != nil {
		metrics.InjectedFaultsTotal.WithLabelValues(metricTag, "error").Inc()
		return true, evalErr
	}

	return false, nil
}
