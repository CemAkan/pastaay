package sqlchaos

import (
	"context"
	"database/sql/driver"
	"log"
	"math/rand"
	"time"

	"github.com/CemAkan/pastaay/pkg/config"
)

// WrapperConn wraps a standard database connection to intercept queries.
type WrapperConn struct {
	originalConn driver.Conn
	cfgManager   *config.Manager
}

// injectChaos reads the configuration and applies latency if a database policy exists.
func (c *WrapperConn) injectChaos() {
	currentConfig := c.cfgManager.Get()

	// Search for SQL chaos policies
	for _, policy := range currentConfig.Policies {
		// Apply policies targeting "database"
		if policy.Type == "sql" && policy.Target == "database" {
			if rand.Float64() < policy.LatencyChance {
				log.Printf("Pastaay SQL Chaos: Injecting %v latency to database query", policy.LatencyDuration)
				time.Sleep(policy.LatencyDuration)
			}
			// Note: Error injection for SQL is more complex, currently simulating only latency.
			break
		}
	}
}

// ExecContext intercepts Exec operations (INSERT, UPDATE, DELETE).
func (c *WrapperConn) ExecContext(ctx context.Context, query string, args []driver.NamedValue) (driver.Result, error) {
	c.injectChaos() // Apply chaos injection

	// Check if the original connection supports ExecContext
	if execer, ok := c.originalConn.(driver.ExecerContext); ok {
		return execer.ExecContext(ctx, query, args)
	}
	return nil, driver.ErrSkip // Fallback to standard library if not supported
}

// QueryContext intercepts Query operations (SELECT).
func (c *WrapperConn) QueryContext(ctx context.Context, query string, args []driver.NamedValue) (driver.Rows, error) {
	c.injectChaos() // Apply chaos injection

	// Check if the original connection supports QueryContext
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
