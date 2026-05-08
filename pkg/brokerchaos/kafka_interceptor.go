package brokerchaos

import (
	"context"
	"time"

	"github.com/CemAkan/pastaay/pkg/metrics"
	"github.com/CemAkan/pastaay/pkg/tracing"
	"github.com/IBM/sarama"
)

type KafkaConsumerMiddleware struct {
	evaluator Evaluator
}

func NewKafkaConsumerMiddleware(eval Evaluator) *KafkaConsumerMiddleware {
	return &KafkaConsumerMiddleware{evaluator: eval}
}

func (m *KafkaConsumerMiddleware) Intercept(ctx context.Context, msg *sarama.ConsumerMessage) (drop bool, err error) {
	if msg == nil {
		return false, nil
	}

	msgCtx := &MessageContext{
		Topic:     msg.Topic,
		Protocol:  ProtocolKafka,
		Partition: msg.Partition,
		GetHeader: func(key string) (string, bool) {
			for _, h := range msg.Headers {
				if len(h.Key) == len(key) {
					match := true
					for i := 0; i < len(key); i++ {
						if h.Key[i] != key[i] {
							match = false
							break
						}
					}
					if match {
						return string(h.Value), true
					}
				}
			}
			return "", false
		},
	}

	shouldDrop, delay, evalErr := m.evaluator.Evaluate(ctx, msgCtx)
	metricTag := "kafka:" + msg.Topic

	if delay > 0 {
		metrics.InjectedFaultsTotal.WithLabelValues(metricTag, "latency").Inc()
		spanCtx, span := tracing.StartChaosSpan(ctx, "pastaay.kafka.latency", msg.Topic, "latency")

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
		_, span := tracing.StartChaosSpan(ctx, "pastaay.kafka.drop", msg.Topic, "drop")
		span.End()
		return true, nil
	}
	if evalErr != nil {
		metrics.InjectedFaultsTotal.WithLabelValues(metricTag, "error").Inc()
		_, span := tracing.StartChaosSpan(ctx, "pastaay.kafka.error", msg.Topic, "error")
		span.End()
		return true, evalErr
	}

	return false, nil
}
