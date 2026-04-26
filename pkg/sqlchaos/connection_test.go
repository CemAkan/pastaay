package sqlchaos

import (
	"context"
	"database/sql/driver"
	"testing"
	"time"

	"github.com/CemAkan/pastaay/pkg/config"
)

// MockConn is a fake database connection to test our Wrapper without needing a real SQL database.
type MockConn struct{}

// Dummy implementations of the driver.Conn interface
func (m *MockConn) Prepare(query string) (driver.Stmt, error) { return nil, nil }
func (m *MockConn) Close() error                              { return nil }
func (m *MockConn) Begin() (driver.Tx, error)                 { return nil, nil }

// Dummy implementation of driver.ExecerContext
func (m *MockConn) ExecContext(ctx context.Context, query string, args []driver.NamedValue) (driver.Result, error) {
	return nil, nil
}

// TestWrapperConn_InjectsLatency tests if the SQL wrapper successfully intercepts and delays a query.
func TestWrapperConn_InjectsLatency(t *testing.T) {
	// 1. Setup a test configuration: 100% chance to inject a 100ms delay for the "database" target.
	cfg := &config.PastaayConfig{
		Policies: []config.Policy{
			{Target: "database", Type: "sql", LatencyChance: 1.0, LatencyDuration: 100 * time.Millisecond},
		},
	}
	manager := config.NewManager(cfg)

	// 2. Wrap our fake connection with the Pastaay WrapperConn
	wrapper := &WrapperConn{
		originalConn: &MockConn{},
		cfgManager:   manager,
	}

	// 3. Start a timer and execute a fake database query
	start := time.Now()
	_, _ = wrapper.ExecContext(context.Background(), "INSERT INTO users (name) VALUES ('cem')", nil)
	elapsed := time.Since(start)

	// 4. Assert: The elapsed time MUST be greater than or equal to the 100ms we configured.
	if elapsed < 100*time.Millisecond {
		t.Errorf("Expected SQL query to be delayed by at least 100ms, but it only took %v", elapsed)
	}
}
