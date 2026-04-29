package redischaos

import (
	"context"
	"testing"

	"github.com/CemAkan/pastaay/pkg/config"
	"github.com/redis/go-redis/v9"
)

// mockCmder satisfies the redis.Cmder interface for testing.
type mockCmder struct {
	redis.Cmder
	name string
}

func (m *mockCmder) Name() string { return m.name }

func TestRedisHook_ErrorInjection(t *testing.T) {
	mgr := config.NewManager(&config.PastaayConfig{
		Policies: []config.Policy{
			{
				Type:        "redis",
				Target:      "get",
				ErrorChance: 1.0, // 100% chance to fail
			},
		},
	})

	hook := NewChaosHook(mgr)
	next := func(ctx context.Context, cmd redis.Cmder) error { return nil }
	processHook := hook.ProcessHook(next)

	err := processHook(context.Background(), &mockCmder{name: "get"})

	if err != redis.Nil {
		t.Errorf("expected redis.Nil error, got %v", err)
	}
}
