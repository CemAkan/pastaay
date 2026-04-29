package sqlchaos

import (
	"context"
	"database/sql/driver"
	"testing"

	"github.com/CemAkan/pastaay/pkg/config"
)

type MockConn struct{}

func (m *MockConn) Prepare(query string) (driver.Stmt, error) { return nil, nil }
func (m *MockConn) Close() error                              { return nil }
func (m *MockConn) Begin() (driver.Tx, error)                 { return nil, nil }
func (m *MockConn) ExecContext(ctx context.Context, query string, args []driver.NamedValue) (driver.Result, error) {
	return nil, nil
}

func TestWrapperConn_InjectsError(t *testing.T) {
	mgr := config.NewManager(&config.PastaayConfig{
		Policies: []config.Policy{
			{
				Type:        "sql",
				ErrorChance: 1.0,
				ErrorBody:   "deadlock detected",
			},
		},
	})

	wrapper := &WrapperConn{
		originalConn: &MockConn{},
		cfgManager:   mgr,
	}

	_, err := wrapper.ExecContext(context.Background(), "UPDATE users SET status = 1", nil)

	if err == nil || err.Error() != "deadlock detected" {
		t.Errorf("expected deadlock error, got %v", err)
	}
}
