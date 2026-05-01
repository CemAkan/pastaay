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

// TestEvaluator_RaceCondition bombards the evaluator with thousands of concurrent
// requests to ensure the internal RNG Mutex prevents race conditions.
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
			// If our sync.Mutex logic is flawed, 'go test -race' will catch it here.
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
	b.ReportAllocs() // Instructs the benchmark to track memory allocations per operation.

	for i := 0; i < b.N; i++ {
		eval.Evaluate(ctx, msgCtx)
	}
}
