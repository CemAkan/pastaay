package mongochaos

import (
	"context"
	"testing"
	"time"

	"github.com/CemAkan/pastaay/pkg/config"
	"go.mongodb.org/mongo-driver/v2/event"
)

// TestChaosDialer_DropConnection verifies that the dialer correctly drops connections when the DropConnection policy is active.
func TestChaosDialer_DropConnection(t *testing.T) {
	mgr := config.NewManager(&config.PastaayConfig{
		Version: 1,
		Policies: []config.Policy{
			{
				Type:           "mongo",
				Target:         "all",
				DropConnection: true,
				ErrorChance:    1.0,
			},
		},
	})

	dialer := &ChaosDialer{
		Manager: mgr,
	}

	_, err := dialer.DialContext(context.Background(), "tcp", "localhost:27017")
	if err == nil {
		t.Fatal("expected error from dropped connection, got nil")
	}

	expectedErr := "[Pastaay-Mongo] Chaos: connection forcefully dropped by policy"
	if err.Error() != expectedErr {
		t.Errorf("expected error %q, got %q", expectedErr, err.Error())
	}
}

// TestChaosMonitor_Latency verifies that the monitor correctly injects the specified sleep duration during command execution.
func TestChaosMonitor_Latency(t *testing.T) {
	latency := 100 * time.Millisecond
	mgr := config.NewManager(&config.PastaayConfig{
		Version: 1,
		Policies: []config.Policy{
			{
				Type:            "mongo",
				Target:          "all",
				LatencyDuration: latency,
				LatencyChance:   1.0,
			},
		},
	})

	monitor := NewChaosMonitor(mgr)
	start := time.Now()

	// Simulate a command start event
	monitor.Started(context.Background(), &event.CommandStartedEvent{
		CommandName: "ping",
	})

	elapsed := time.Since(start)
	if elapsed < latency {
		t.Errorf("expected at least %v latency, got %v", latency, elapsed)
	}
}
