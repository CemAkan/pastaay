package brokerchaos

import (
	"context"
	"time"

	amqp "github.com/rabbitmq/amqp091-go"
)

// RabbitMQMiddleware acts as a chaos-injector for AMQP 0.9.1 message deliveries.
type RabbitMQMiddleware struct {
	evaluator Evaluator
}

// NewRabbitMQMiddleware initializes the RabbitMQ chaos interceptor.
func NewRabbitMQMiddleware(eval Evaluator) *RabbitMQMiddleware {
	return &RabbitMQMiddleware{evaluator: eval}
}

// Intercept evaluates a single RabbitMQ delivery against active chaos policies.
func (m *RabbitMQMiddleware) Intercept(ctx context.Context, delivery *amqp.Delivery) (drop bool, err error) {
	// Defensive check: Guard against nil pointer dereference
	if delivery == nil {
		return false, nil
	}

	headers := make(map[string]string, len(delivery.Headers))
	for k, v := range delivery.Headers {
		if strVal, ok := v.(string); ok {
			headers[k] = strVal
		}
	}

	// RabbitMQ uses RoutingKey or Exchange as the concept of Topic.
	// We map RoutingKey to our generic Topic field.
	msgCtx := &MessageContext{
		Topic:    delivery.RoutingKey,
		Protocol: ProtocolRabbitMQ,
		Headers:  headers,
	}

	action, delay, evalErr := m.evaluator.Evaluate(ctx, msgCtx)

	switch action {
	case ActionDrop:
		return true, nil

	case ActionError:
		return true, evalErr

	case ActionDelay:
		// Context-aware blocking to survive Graceful Shutdowns.
		select {
		case <-time.After(delay):
			return false, nil
		case <-ctx.Done():
			return false, ctx.Err()
		}

	default:
		return false, nil
	}
}
