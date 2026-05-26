package brokerchaos

import (
	"context"
	"time"

	"github.com/CemAkan/pastaay/pkg/metrics"
	"github.com/CemAkan/pastaay/pkg/telemetry"
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

	shouldDrop, delay, evalErr, latencyTag, errorTag := m.evaluator.Evaluate(ctx, msgCtx)

	if delay > 0 && latencyTag != "" {
		metrics.InjectedFaultsTotal.WithLabelValues(latencyTag, "latency").Inc()
		spanCtx, span := tracing.StartChaosSpan(ctx, "pastaay.kafka.latency", latencyTag, "latency")

		telemetry.EmitInfo("kafka", "Kafka Latency Injected", map[string]interface{}{"duration": delay.String(), "target": msg.Topic}, span)

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

	if shouldDrop && errorTag != "" {
		metrics.InjectedFaultsTotal.WithLabelValues(errorTag, "drop").Inc()
		_, span := tracing.StartChaosSpan(ctx, "pastaay.kafka.drop", errorTag, "drop")
		telemetry.EmitError("kafka", msg.Topic, "Message dropped securely", "Silent drop", span)
		span.End()
		return true, nil
	}
	if evalErr != nil && errorTag != "" {
		metrics.InjectedFaultsTotal.WithLabelValues(errorTag, "error").Inc()
		_, span := tracing.StartChaosSpan(ctx, "pastaay.kafka.error", errorTag, "error")

		telemetry.EmitError("kafka", errorTag, "Kafka Fault Injected", evalErr.Error(), span)

		span.End()
		return true, evalErr
	}

	return false, nil
}
