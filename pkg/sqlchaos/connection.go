package sqlchaos

import (
	"context"
	"database/sql/driver"
	"errors"
	"math/rand/v2"
	"strings"
	"time"

	"github.com/CemAkan/pastaay/pkg/config"
	"github.com/CemAkan/pastaay/pkg/metrics"
	"github.com/CemAkan/pastaay/pkg/tracing"
)

type chaosKey struct{}

type WrapperConn struct {
	originalConn driver.Conn
	cfgManager   *config.Manager
}

type WrapperStmt struct {
	originalStmt driver.Stmt
	cleanQuery   string
	cfgManager   *config.Manager
}

type WrapperTx struct {
	originalTx driver.Tx
	cfgManager *config.Manager
}

func (t *WrapperTx) Commit() error   { return t.originalTx.Commit() }
func (t *WrapperTx) Rollback() error { return t.originalTx.Rollback() }

func applySQLChaos(ctx context.Context, mgr *config.Manager, cleanQuery string) (context.Context, error) {

	if ctx.Value(chaosKey{}) != nil || mgr.IsCleanCommandIgnored("sql", cleanQuery) {
		return ctx, nil
	}

	policies := mgr.GetActivePolicies("sql")
	currentCtx := ctx

	for _, p := range policies {
		if isTargetMatch(cleanQuery, &p) {
			evalCtx := context.WithValue(currentCtx, chaosKey{}, true)
			metricTag := "sql:" + p.Target

			if p.LatencyChance > 0 && rand.Float64() < p.LatencyChance {
				metrics.InjectedFaultsTotal.WithLabelValues(metricTag, "latency").Inc()
				spanCtx, span := tracing.StartChaosSpan(evalCtx, "pastaay.sql.latency", p.Target, "latency")

				timer := time.NewTimer(p.LatencyDuration)
				select {
				case <-timer.C:
					timer.Stop()
					span.End()
				case <-spanCtx.Done():
					timer.Stop()
					span.End()
					return spanCtx, spanCtx.Err()
				}
			}
			if p.ErrorChance > 0 && rand.Float64() < p.ErrorChance {
				metrics.InjectedFaultsTotal.WithLabelValues(metricTag, "error").Inc()
				_, span := tracing.StartChaosSpan(evalCtx, "pastaay.sql.error", p.Target, "error")
				span.End()
				msg := p.ErrorBody
				if msg == "" {
					msg = "sql: database connection is closed"
				}
				return evalCtx, errors.New(msg)
			}
		}
	}
	return currentCtx, nil
}

func isTargetMatch(query string, p *config.Policy) bool {
	target := strings.ToUpper(p.Target)
	if target == "ALL" || target == "DATABASE" {
		return true
	}
	if query == "" || p.CompiledRegex == nil {
		return false
	}
	return p.CompiledRegex.MatchString(query)
}

func (c *WrapperConn) QueryContext(ctx context.Context, query string, args []driver.NamedValue) (driver.Rows, error) {
	if q, ok := c.originalConn.(driver.QueryerContext); ok {
		newCtx, err := applySQLChaos(ctx, c.cfgManager, config.CleanSQLCommand(query))
		if err != nil {
			return nil, err
		}
		return q.QueryContext(newCtx, query, args)
	}
	return nil, driver.ErrSkip
}

func (c *WrapperConn) ExecContext(ctx context.Context, query string, args []driver.NamedValue) (driver.Result, error) {
	if e, ok := c.originalConn.(driver.ExecerContext); ok {
		newCtx, err := applySQLChaos(ctx, c.cfgManager, config.CleanSQLCommand(query))
		if err != nil {
			return nil, err
		}
		return e.ExecContext(newCtx, query, args)
	}
	return nil, driver.ErrSkip
}

func (c *WrapperConn) BeginTx(ctx context.Context, opts driver.TxOptions) (driver.Tx, error) {
	if _, err := applySQLChaos(ctx, c.cfgManager, "BEGIN"); err != nil {
		return nil, err
	}
	if bt, ok := c.originalConn.(driver.ConnBeginTx); ok {
		tx, err := bt.BeginTx(ctx, opts)
		if err != nil {
			return nil, err
		}
		return &WrapperTx{originalTx: tx, cfgManager: c.cfgManager}, nil
	}
	tx, err := c.originalConn.Begin()
	if err != nil {
		return nil, err
	}
	return &WrapperTx{originalTx: tx, cfgManager: c.cfgManager}, nil
}

func (c *WrapperConn) PrepareContext(ctx context.Context, query string) (driver.Stmt, error) {
	if pc, ok := c.originalConn.(driver.ConnPrepareContext); ok {
		stmt, err := pc.PrepareContext(ctx, query)
		if err != nil {
			return nil, err
		}
		return &WrapperStmt{originalStmt: stmt, cleanQuery: config.CleanSQLCommand(query), cfgManager: c.cfgManager}, nil
	}
	return nil, driver.ErrSkip
}

func (c *WrapperConn) Prepare(query string) (driver.Stmt, error) {
	stmt, err := c.originalConn.Prepare(query)
	if err != nil {
		return nil, err
	}
	return &WrapperStmt{originalStmt: stmt, cleanQuery: config.CleanSQLCommand(query), cfgManager: c.cfgManager}, nil
}

func (c *WrapperConn) Close() error { return c.originalConn.Close() }
func (c *WrapperConn) Begin() (driver.Tx, error) {
	return c.BeginTx(context.Background(), driver.TxOptions{})
}

func (s *WrapperStmt) ExecContext(ctx context.Context, args []driver.NamedValue) (driver.Result, error) {
	if ec, ok := s.originalStmt.(driver.StmtExecContext); ok {

		newCtx, err := applySQLChaos(ctx, s.cfgManager, s.cleanQuery)
		if err != nil {
			return nil, err
		}
		return ec.ExecContext(newCtx, args)
	}
	return nil, driver.ErrSkip
}

func (s *WrapperStmt) QueryContext(ctx context.Context, args []driver.NamedValue) (driver.Rows, error) {
	if qc, ok := s.originalStmt.(driver.StmtQueryContext); ok {
		newCtx, err := applySQLChaos(ctx, s.cfgManager, s.cleanQuery)
		if err != nil {
			return nil, err
		}
		return qc.QueryContext(newCtx, args)
	}
	return nil, driver.ErrSkip
}

func (s *WrapperStmt) Exec(args []driver.Value) (driver.Result, error) {
	if _, err := applySQLChaos(context.Background(), s.cfgManager, s.cleanQuery); err != nil {
		return nil, err
	}
	return s.originalStmt.Exec(args)
}

func (s *WrapperStmt) Query(args []driver.Value) (driver.Rows, error) {
	if _, err := applySQLChaos(context.Background(), s.cfgManager, s.cleanQuery); err != nil {
		return nil, err
	}
	return s.originalStmt.Query(args)
}

func (s *WrapperStmt) Close() error  { return s.originalStmt.Close() }
func (s *WrapperStmt) NumInput() int { return s.originalStmt.NumInput() }
