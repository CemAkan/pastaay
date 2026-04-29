package sqlchaos

import (
	"context"
	"database/sql/driver"
	"errors"
	"log"
	"math/rand"
	"time"

	"github.com/CemAkan/pastaay/pkg/config"
)

type WrapperConn struct {
	originalConn driver.Conn
	cfgManager   *config.Manager
}

// injectChaos evaluates SQL policies for the current session.
func (c *WrapperConn) injectChaos() error {
	policies := c.cfgManager.GetActivePolicies("sql")
	for _, p := range policies {
		// Latency Injection
		if p.LatencyChance > 0 && rand.Float64() < p.LatencyChance {
			log.Printf("[Pastaay-SQL] Latency: delaying query by %v", p.LatencyDuration)
			time.Sleep(p.LatencyDuration)
		}

		// Synthetic Error Injection (e.g., Deadlocks, Connection Failures)
		if p.ErrorChance > 0 && rand.Float64() < p.ErrorChance {
			msg := p.ErrorBody
			if msg == "" {
				msg = "sql: database connection is closed"
			}
			return errors.New(msg)
		}
	}
	return nil
}

// Sadece ExecContext ve QueryContext metodlarını şu şekilde güncelle:

func (c *WrapperConn) QueryContext(ctx context.Context, query string, args []driver.NamedValue) (driver.Rows, error) {
	if !c.cfgManager.IsCommandIgnored("sql", query) {
		if err := c.injectChaos(); err != nil {
			return nil, err
		}
	}

	if q, ok := c.originalConn.(driver.QueryerContext); ok {
		return q.QueryContext(ctx, query, args)
	}
	return nil, driver.ErrSkip
}

func (c *WrapperConn) ExecContext(ctx context.Context, query string, args []driver.NamedValue) (driver.Result, error) {
	if !c.cfgManager.IsCommandIgnored("sql", query) {
		if err := c.injectChaos(); err != nil {
			return nil, err
		}
	}

	if e, ok := c.originalConn.(driver.ExecerContext); ok {
		return e.ExecContext(ctx, query, args)
	}
	return nil, driver.ErrSkip
}

// Ensure interface compliance
func (c *WrapperConn) Prepare(query string) (driver.Stmt, error) {
	return c.originalConn.Prepare(query)
}
func (c *WrapperConn) Close() error              { return c.originalConn.Close() }
func (c *WrapperConn) Begin() (driver.Tx, error) { return c.originalConn.Begin() }
