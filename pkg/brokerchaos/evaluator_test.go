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

type fakeProvider struct {
	policies []config.Policy
	ignored  map[string]bool
}

func (f *fakeProvider) GetActivePolicies() []config.Policy { return f.policies }
func (f *fakeProvider) IsCommandIgnored(protocol, cmd string) bool {
	return f.ignored[protocol+":"+cmd]
}

func TestNewEvaluator_PanicsOnNil(t *testing.T) {
	defer func() {
		if recover() == nil {
			t.Fatal("expected panic on nil ConfigProvider")
		}
	}()
	NewEvaluator(nil)
}

func TestEvaluate_NilMessageContextSafe(t *testing.T) {
	e := NewEvaluator(&fakeProvider{})
	drop, dur, err, lat, et := e.Evaluate(context.Background(), nil)
	if drop || dur != 0 || err != nil || lat != "" || et != "" {
		t.Fatalf("nil msgCtx must yield zero-value: %v %v %v %v %v", drop, dur, err, lat, et)
	}
}

func TestEvaluate_IgnoredCommandShortCircuits(t *testing.T) {
	e := NewEvaluator(&fakeProvider{
		policies: []config.Policy{{Type: "kafka", Target: "metrics", ErrorChance: 1.0}},
		ignored:  map[string]bool{"kafka:metrics": true},
	})
	_, _, err, _, _ := e.Evaluate(context.Background(), &MessageContext{
		Protocol: ProtocolKafka,
		Topic:    "metrics",
	})
	if err != nil {
		t.Fatalf("ignored command must not trigger fault, got %v", err)
	}
}

func TestEvaluate_HeaderMatchGate(t *testing.T) {
	e := NewEvaluator(&fakeProvider{
		policies: []config.Policy{{
			Type: "kafka", Target: "logs", ErrorChance: 1.0,
			MatchHeaders: map[string]string{"tenant": "alpha"},
		}},
	})
	// Header mismatch, must not fire.
	_, _, err, _, _ := e.Evaluate(context.Background(), &MessageContext{
		Protocol: ProtocolKafka, Topic: "logs",
		GetHeader: func(k string) (string, bool) { return "beta", true },
	})
	if err != nil {
		t.Fatalf("header mismatch must skip policy, got %v", err)
	}
	// Header match, must fire.
	_, _, err, _, _ = e.Evaluate(context.Background(), &MessageContext{
		Protocol: ProtocolKafka, Topic: "logs",
		GetHeader: func(k string) (string, bool) { return "alpha", true },
	})
	if err == nil {
		t.Fatalf("header match must fire policy")
	}
}

func TestEvaluate_NoMatchingPolicy(t *testing.T) {
	e := NewEvaluator(&fakeProvider{
		policies: []config.Policy{{Type: "rabbitmq", Target: "queue.x", ErrorChance: 1.0}},
	})
	_, _, err, _, _ := e.Evaluate(context.Background(), &MessageContext{
		Protocol: ProtocolKafka, Topic: "queue.x",
	})
	if err != nil {
		t.Fatalf("protocol mismatch must not fire, got %v", err)
	}
}

func TestEvaluate_SinglePolicyTieBreaksLatencyVsError(t *testing.T) {
	e := NewEvaluator(&fakeProvider{
		policies: []config.Policy{{
			Type:            "kafka",
			Target:          "logs",
			MetricTag:       "kafka:logs",
			LatencyChance:   1.0,
			LatencyDuration: 5 * 1_000_000, // 5ms
			ErrorChance:     1.0,
		}},
	})

	for i := 0; i < 32; i++ {
		_, dur, errOut, latTag, errTag := e.Evaluate(context.Background(), &MessageContext{
			Protocol: ProtocolKafka, Topic: "logs",
		})
		if errOut == nil || errTag == "" {
			t.Fatalf("iter %d: tie-break must produce error, got err=%v errTag=%q", i, errOut, errTag)
		}
		if dur != 0 || latTag != "" {
			t.Fatalf("iter %d: tie-break must SUPPRESS latency, got dur=%v latTag=%q", i, dur, latTag)
		}
	}
}
