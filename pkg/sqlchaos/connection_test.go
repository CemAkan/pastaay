package sqlchaos

import (
	"context"
	"database/sql/driver"
	"errors"
	"regexp"
	_ "strings"
	"testing"
	"time"

	"github.com/CemAkan/pastaay/pkg/config"
)

// Mock objects for high-fidelity testing
type MockConn struct{ driver.Conn }

func (m *MockConn) Prepare(query string) (driver.Stmt, error) { return &MockStmt{query: query}, nil }
func (m *MockConn) Close() error                              { return nil }
func (m *MockConn) Begin() (driver.Tx, error)                 { return nil, nil }

type MockStmt struct {
	driver.Stmt
	query string
}

func (m *MockStmt) Close() error                                    { return nil }
func (m *MockStmt) Exec(args []driver.Value) (driver.Result, error) { return nil, nil }
func (m *MockStmt) Query(args []driver.Value) (driver.Rows, error)  { return nil, nil }

// TestWordBoundaryMatch ensures that "UPDATE_LOGS" table doesn't trigger "UPDATE" policy
func TestWordBoundaryMatch(t *testing.T) {
	updateRegex := regexp.MustCompile(`(?i)\bUPDATE\b`)
	mgr := config.NewManager(&config.PastaayConfig{
		Policies: []config.Policy{
			{
				Type:          "sql",
				Target:        "UPDATE",
				ErrorChance:   1.0,
				CompiledRegex: updateRegex,
			},
		},
	})

	// 1. False Positive Check: Tablename contains "UPDATE"
	query1 := "SELECT * FROM user_updates"
	ctx1, err1 := applySQLChaos(context.Background(), mgr, query1)
	if err1 != nil {
		t.Errorf("Should NOT inject chaos into substring matches: %v", err1)
	}
	if ctx1.Value(chaosKey{}) != nil {
		t.Error("Context should NOT be marked for false positive matches")
	}

	// 2. True Positive Check: Real UPDATE command
	query2 := "UPDATE users SET status = 1"
	_, err2 := applySQLChaos(context.Background(), mgr, query2)
	if err2 == nil {
		t.Error("Should inject chaos for exact word match")
	}
}

// TestDoubleChaosPrevention verifies the Fallback Shield (chaosKey)
func TestDoubleChaosPrevention(t *testing.T) {
	mgr := config.NewManager(&config.PastaayConfig{
		Policies: []config.Policy{
			{
				Type:        "sql",
				Target:      "ALL",
				ErrorChance: 1.0,
			},
		},
	})

	// Initial chaos application
	ctx := context.Background()
	markedCtx, err := applySQLChaos(ctx, mgr, "SELECT 1")
	if err == nil {
		t.Fatal("Initial chaos should have been triggered")
	}

	// Verify that second application on the same marked context returns nil (shield works)
	// We simulate a fallback where the same context is reused
	shieldedCtx, err2 := applySQLChaos(markedCtx, mgr, "SELECT 1")
	if err2 != nil {
		t.Errorf("Double chaos prevention failed: %v", err2)
	}
	if shieldedCtx != markedCtx {
		t.Error("Shielded context should be identical to marked context")
	}
}

// TestLatencyContextAwareness ensures delays are cancelled when context is done
func TestLatencyContextAwareness(t *testing.T) {
	mgr := config.NewManager(&config.PastaayConfig{
		Policies: []config.Policy{
			{
				Type:            "sql",
				Target:          "all",
				LatencyChance:   1.0,
				LatencyDuration: 1 * time.Hour,
			},
		},
	})

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Millisecond)
	defer cancel()

	start := time.Now()
	_, err := applySQLChaos(ctx, mgr, "SELECT 1")
	elapsed := time.Since(start)

	if !errors.Is(err, context.DeadlineExceeded) {
		t.Errorf("Expected context timeout, got %v", err)
	}

	if elapsed > 100*time.Millisecond {
		t.Error("Chaos delay was not interrupted by context cancellation")
	}
}
