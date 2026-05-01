package sqlchaos

import (
	"context"
	"database/sql/driver"
	"errors"
	"log"
	"math/rand/v2"
	"strings"
	"time"

	"github.com/CemAkan/pastaay/pkg/config"
	"github.com/CemAkan/pastaay/pkg/metrics"
)

type WrapperConn struct {
	originalConn driver.Conn
	cfgManager   *config.Manager
}

type WrapperStmt struct {
	originalStmt driver.Stmt
	query        string
	cleanQuery   string
	cfgManager   *config.Manager
}

func applySQLChaos(ctx context.Context, mgr *config.Manager, cleanQuery string) error {
	if mgr.IsCleanCommandIgnored("sql", cleanQuery) {
		return nil
	}

	policies := mgr.GetActivePolicies("sql")
	for _, p := range policies {
		targetUpper := strings.ToUpper(p.Target)

		if targetUpper == "ALL" || targetUpper == "DATABASE" || (cleanQuery != "" && strings.HasPrefix(cleanQuery, targetUpper)) {
			if p.LatencyChance > 0 && rand.Float64() < p.LatencyChance {
				metrics.InjectedFaultsTotal.WithLabelValues("sql", "latency").Inc()
				log.Printf("[Pastaay-SQL] Latency: delaying query by %v", p.LatencyDuration)

				timer := time.NewTimer(p.LatencyDuration)
				select {
				case <-timer.C:
				case <-ctx.Done():
					timer.Stop()
					return ctx.Err()
				}
				timer.Stop()
			}

			if p.ErrorChance > 0 && rand.Float64() < p.ErrorChance {
				metrics.InjectedFaultsTotal.WithLabelValues("sql", "error").Inc()
				msg := p.ErrorBody
				if msg == "" {
					msg = "sql: database connection is closed"
				}
				return errors.New(msg)
			}
		}
	}
	return nil
}

// STATEMENT

func (s *WrapperStmt) Exec(args []driver.Value) (driver.Result, error) {
	if _, ok := s.originalStmt.(driver.StmtExecContext); !ok {
		ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
		defer cancel()
		// Önbellekten doğrudan cleanQuery paslanıyor!
		if err := applySQLChaos(ctx, s.cfgManager, s.cleanQuery); err != nil {
			return nil, err
		}
	}
	return s.originalStmt.Exec(args)
}

func (s *WrapperStmt) Query(args []driver.Value) (driver.Rows, error) {
	if _, ok := s.originalStmt.(driver.StmtQueryContext); !ok {
		ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
		defer cancel()
		if err := applySQLChaos(ctx, s.cfgManager, s.cleanQuery); err != nil {
			return nil, err
		}
	}
	return s.originalStmt.Query(args)
}

func (s *WrapperStmt) ExecContext(ctx context.Context, args []driver.NamedValue) (driver.Result, error) {
	if ec, ok := s.originalStmt.(driver.StmtExecContext); ok {
		if err := applySQLChaos(ctx, s.cfgManager, s.cleanQuery); err != nil {
			return nil, err
		}
		return ec.ExecContext(ctx, args)
	}
	return nil, driver.ErrSkip
}

func (s *WrapperStmt) QueryContext(ctx context.Context, args []driver.NamedValue) (driver.Rows, error) {
	if qc, ok := s.originalStmt.(driver.StmtQueryContext); ok {
		if err := applySQLChaos(ctx, s.cfgManager, s.cleanQuery); err != nil {
			return nil, err
		}
		return qc.QueryContext(ctx, args)
	}
	return nil, driver.ErrSkip
}

func (s *WrapperStmt) Close() error  { return s.originalStmt.Close() }
func (s *WrapperStmt) NumInput() int { return s.originalStmt.NumInput() }

// CONNECTION

func (c *WrapperConn) Prepare(query string) (driver.Stmt, error) {
	stmt, err := c.originalConn.Prepare(query)
	if err != nil {
		return nil, err
	}
	// HAZIRLIK AŞAMASI: Sadece 1 kez temizle ve WrapperStmt içine göm.
	cleanQuery := config.CleanSQLCommand(query)
	return &WrapperStmt{originalStmt: stmt, query: query, cleanQuery: cleanQuery, cfgManager: c.cfgManager}, nil
}

func (c *WrapperConn) PrepareContext(ctx context.Context, query string) (driver.Stmt, error) {
	if pc, ok := c.originalConn.(driver.ConnPrepareContext); ok {
		stmt, err := pc.PrepareContext(ctx, query)
		if err != nil {
			return nil, err
		}
		cleanQuery := config.CleanSQLCommand(query)
		return &WrapperStmt{originalStmt: stmt, query: query, cleanQuery: cleanQuery, cfgManager: c.cfgManager}, nil
	}
	return c.Prepare(query)
}

func (c *WrapperConn) QueryContext(ctx context.Context, query string, args []driver.NamedValue) (driver.Rows, error) {
	if q, ok := c.originalConn.(driver.QueryerContext); ok {
		if err := applySQLChaos(ctx, c.cfgManager, config.CleanSQLCommand(query)); err != nil {
			return nil, err
		}
		return q.QueryContext(ctx, query, args)
	}
	return nil, driver.ErrSkip
}

func (c *WrapperConn) ExecContext(ctx context.Context, query string, args []driver.NamedValue) (driver.Result, error) {
	if e, ok := c.originalConn.(driver.ExecerContext); ok {
		if err := applySQLChaos(ctx, c.cfgManager, config.CleanSQLCommand(query)); err != nil {
			return nil, err
		}
		return e.ExecContext(ctx, query, args)
	}
	return nil, driver.ErrSkip
}

func (c *WrapperConn) Close() error { return c.originalConn.Close() }

func (c *WrapperConn) Begin() (driver.Tx, error) {
	if err := applySQLChaos(context.Background(), c.cfgManager, ""); err != nil {
		return nil, err
	}
	return c.originalConn.Begin()
}

func (c *WrapperConn) BeginTx(ctx context.Context, opts driver.TxOptions) (driver.Tx, error) {
	if bt, ok := c.originalConn.(driver.ConnBeginTx); ok {
		if err := applySQLChaos(ctx, c.cfgManager, ""); err != nil {
			return nil, err
		}
		return bt.BeginTx(ctx, opts)
	}
	return c.Begin()
}
