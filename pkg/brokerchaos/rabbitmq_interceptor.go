package brokerchaos

import (
	"context"
	"fmt"
	"time"

	"github.com/CemAkan/pastaay/pkg/metrics"
	"github.com/CemAkan/pastaay/pkg/tracing"
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
		spanCtx, span := tracing.StartChaosSpan(ctx, "pastaay.rabbitmq.latency", delivery.RoutingKey, "latency")

		timer := time.NewTimer(delay)
		select {
		case <-timer.C:
			timer.Stop()
			span.End()
		case <-spanCtx.Done():
			timer.Stop()
			span.End()
			return false, spanCtx.Err()
		}
	}

	if shouldDrop {
		metrics.InjectedFaultsTotal.WithLabelValues(metricTag, "drop").Inc()
		_, span := tracing.StartChaosSpan(ctx, "pastaay.rabbitmq.drop", delivery.RoutingKey, "drop")
		span.End()
		return true, nil
	}
	if evalErr != nil {
		metrics.InjectedFaultsTotal.WithLabelValues(metricTag, "error").Inc()
		_, span := tracing.StartChaosSpan(ctx, "pastaay.rabbitmq.error", delivery.RoutingKey, "error")
		span.End()
		return true, evalErr
	}

	return false, nil
}
