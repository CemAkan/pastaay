package redischaos

import (
	"context"
	"testing"

	"github.com/CemAkan/pastaay/pkg/config"
	"github.com/redis/go-redis/v9"
)

// mockCmder implements a simple redis.Cmder for testing
type mockCmder struct {
	redis.Cmder
	name string
}

func (m *mockCmder) Name() string {
	return m.name
}

func TestRedisHook_ErrorInjection(t *testing.T) {
	// inject error for get commands
	cfgManager := config.NewManager(&config.PastaayConfig{
		Policies: []config.Policy{
			{
				Name:        "test-redis-error",
				Type:        "redis",
				Target:      "get",
				ErrorChance: 1.0, // %100 Cache Miss
			},
		},
	})

	hook := NewChaosHook(cfgManager)

	nextCalled := false
	next := func(ctx context.Context, cmd redis.Cmder) error {
		nextCalled = true
		return nil
	}

	processHook := hook.ProcessHook(next)

	cmd := &mockCmder{name: "get"}
	err := processHook(context.Background(), cmd)

	if err != redis.Nil {
		t.Errorf("Expected redis.Nil error, got %v", err)
	}
	if nextCalled {
		t.Errorf("Expected next handler NOT to be called when error is injected")
	}
}

func TestRedisHook_Bypass(t *testing.T) {
	// empty conf, no chaos
	cfgManager := config.NewManager(&config.PastaayConfig{})
	hook := NewChaosHook(cfgManager)

	nextCalled := false
	next := func(ctx context.Context, cmd redis.Cmder) error {
		nextCalled = true
		return nil
	}

	processHook := hook.ProcessHook(next)
	cmd := &mockCmder{name: "get"}
	err := processHook(context.Background(), cmd)

	if err != nil {
		t.Errorf("Expected no error, got %v", err)
	}
	if !nextCalled {
		t.Errorf("Expected next handler to be called")
	}
}
