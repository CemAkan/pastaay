package brokerchaos

import (
	"context"
	"sync"
	"testing"
	"time"

	"github.com/CemAkan/pastaay/pkg/config"
)

// mockConfigProvider supplies dummy policies to isolate the test from the file system.
type mockConfigProvider struct {
	policies []config.Policy
}

func (m *mockConfigProvider) GetActivePolicies() []config.Policy {
	return m.policies
}

func (m *mockConfigProvider) IsCommandIgnored(protocol string, cmd string) bool {
	return false
}

// TestEvaluator_RaceCondition bombards the evaluator with thousands of concurrent requests
func TestEvaluator_RaceCondition(t *testing.T) {
	provider := &mockConfigProvider{
		policies: []config.Policy{
			{Type: "kafka", Target: "critical_topic", ErrorChance: 0.5},
		},
	}
	eval := NewEvaluator(provider)
	ctx := context.Background()

	var wg sync.WaitGroup
	workers := 5000 // Simulate 5000 concurrent Kafka consumer threads

	wg.Add(workers)
	for i := 0; i < workers; i++ {
		go func() {
			defer wg.Done()
			msgCtx := &MessageContext{
				Topic:    "critical_topic",
				Protocol: ProtocolKafka,
			}
			// Validates lock-free concurrency under heavy load
			eval.Evaluate(ctx, msgCtx)
		}()
	}
	wg.Wait()
}

func BenchmarkEvaluator(b *testing.B) {
	provider := &mockConfigProvider{
		policies: []config.Policy{
			{Type: "rabbitmq", Target: "payment_queue", ErrorChance: 0.05, LatencyDuration: time.Millisecond * 10},
		},
	}
	eval := NewEvaluator(provider)
	ctx := context.Background()
	msgCtx := &MessageContext{
		Topic:    "payment_queue",
		Protocol: ProtocolRabbitMQ,
	}

	b.ResetTimer()
	b.ReportAllocs()

	for i := 0; i < b.N; i++ {
		eval.Evaluate(ctx, msgCtx)
	}
}
