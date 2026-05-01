package brokerchaos

import (
	"context"
	"time"

	"github.com/IBM/sarama"
)

// KafkaConsumerMiddleware acts as a chaos-injector before your application processes a Kafka message.
type KafkaConsumerMiddleware struct {
	evaluator Evaluator
}

// NewKafkaConsumerMiddleware initializes the Kafka chaos interceptor.
func NewKafkaConsumerMiddleware(eval Evaluator) *KafkaConsumerMiddleware {
	return &KafkaConsumerMiddleware{evaluator: eval}
}

// Intercept evaluates a single Kafka message against active chaos policies.
func (m *KafkaConsumerMiddleware) Intercept(ctx context.Context, msg *sarama.ConsumerMessage) (drop bool, err error) {
	if msg == nil {
		return false, nil
	}

	msgCtx := &MessageContext{
		Topic:     msg.Topic,
		Protocol:  ProtocolKafka,
		Partition: msg.Partition,

		ExtractHeaders: func() map[string]string {
			headers := make(map[string]string, len(msg.Headers))
			for _, h := range msg.Headers {
				headers[string(h.Key)] = string(h.Value)
			}
			return headers
		},
	}

	// Policy Lookup
	action, delay, evalErr := m.evaluator.Evaluate(ctx, msgCtx)

	switch action {
	case ActionDrop:
		return true, nil
	case ActionError:
		return true, evalErr
	case ActionDelay:
		// Context-aware blocking.
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
