package brokerchaos

import (
	"context"
	"time"

	"github.com/CemAkan/pastaay/pkg/metrics"
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
