package sqlchaos

import (
	"context"
	"database/sql/driver"
	"errors"
	"log"
	"math/rand"
	"time"

	"github.com/CemAkan/pastaay/pkg/config"
	"github.com/CemAkan/pastaay/pkg/metrics"
)

// WrapperConn wraps a standard database connection to intercept queries.
type WrapperConn struct {
	originalConn driver.Conn
	cfgManager   *config.Manager
}

// injectChaos reads the configuration and applies latency or returns a synthetic error.
func (c *WrapperConn) injectChaos() error {
	currentConfig := c.cfgManager.Get()

	// Search for SQL chaos policies
	for _, policy := range currentConfig.Policies {
		if policy.Type == "sql" && policy.Target == "database" {

			// Latency Injection
			if policy.LatencyChance > 0 && rand.Float64() < policy.LatencyChance {
				log.Printf("Pastaay SQL Chaos: Injecting %v latency to database query", policy.LatencyDuration)
				metrics.InjectedFaultsTotal.WithLabelValues("database", "latency").Inc()
				time.Sleep(policy.LatencyDuration)
			}

			// Error Injection
			if policy.ErrorChance > 0 && rand.Float64() < policy.ErrorChance {
				errorMsg := policy.ErrorBody
				if errorMsg == "" {
					errorMsg = "Pastaay Chaos: Synthetic Database Connection Error"
				}

				log.Printf("Pastaay SQL Chaos: Injecting database error: %s", errorMsg)
				metrics.InjectedFaultsTotal.WithLabelValues("database", "error").Inc()

				// Return the synthetic error to abort the real query
				return errors.New(errorMsg)
			}
			break
		}
	}
	return nil
}

// ExecContext intercepts Exec operations (INSERT, UPDATE, DELETE).
func (c *WrapperConn) ExecContext(ctx context.Context, query string, args []driver.NamedValue) (driver.Result, error) {
	if err := c.injectChaos(); err != nil {
		return nil, err
	}
	if execer, ok := c.originalConn.(driver.ExecerContext); ok {
		return execer.ExecContext(ctx, query, args)
	}
	return nil, driver.ErrSkip
}

// QueryContext intercepts Query operations (SELECT).
func (c *WrapperConn) QueryContext(ctx context.Context, query string, args []driver.NamedValue) (driver.Rows, error) {
	if err := c.injectChaos(); err != nil {
		return nil, err
	}
	if queryer, ok := c.originalConn.(driver.QueryerContext); ok {
		return queryer.QueryContext(ctx, query, args)
	}
	return nil, driver.ErrSkip
}

// Prepare implements the driver.Conn interface.
func (c *WrapperConn) Prepare(query string) (driver.Stmt, error) {
	return c.originalConn.Prepare(query)
}

// Close implements the driver.Conn interface.
func (c *WrapperConn) Close() error {
	return c.originalConn.Close()
}

// Begin implements the driver.Conn interface.
func (c *WrapperConn) Begin() (driver.Tx, error) {
	return c.originalConn.Begin()
}
