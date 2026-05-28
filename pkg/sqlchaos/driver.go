package sqlchaos

import (
	"context"
	"database/sql"
	"database/sql/driver"
	"errors"
	"math/rand/v2"
	"strings"

	"github.com/CemAkan/pastaay/pkg/config"
	"github.com/CemAkan/pastaay/pkg/metrics"
	"github.com/CemAkan/pastaay/pkg/telemetry"
	"github.com/CemAkan/pastaay/pkg/tracing"
)

type WrapperDriver struct {
	original   driver.Driver
	cfgManager *config.Manager
}
type WrapperConnector struct {
	original   driver.Connector
	cfgManager *config.Manager
}
type fallbackConnector struct {
	driver *WrapperDriver
	name   string
}

func (c *WrapperConnector) Connect(ctx context.Context) (driver.Conn, error) {
	if err := applyConnectionChaos(ctx, c.cfgManager); err != nil {
		return nil, err
	}
	conn, err := c.original.Connect(ctx)
	if err != nil {
		return nil, err
	}
	return &WrapperConn{originalConn: conn, cfgManager: c.cfgManager}, nil
}

func (c *WrapperConnector) Driver() driver.Driver { return c.original.Driver() }
func (c *fallbackConnector) Connect(ctx context.Context) (driver.Conn, error) {
	return c.driver.Open(c.name)
}
func (c *fallbackConnector) Driver() driver.Driver { return c.driver }

// OpenConnector returns a chaos-aware connector or falls back to a compat wrapper.
func (d *WrapperDriver) OpenConnector(name string) (driver.Connector, error) {
	if dc, ok := d.original.(driver.DriverContext); ok {
		connector, err := dc.OpenConnector(name)
		if err != nil {
			return nil, err
		}
		return &WrapperConnector{original: connector, cfgManager: d.cfgManager}, nil
	}
	return &fallbackConnector{driver: d, name: name}, nil
}

func (d *WrapperDriver) Open(name string) (driver.Conn, error) {
	if err := applyConnectionChaos(context.Background(), d.cfgManager); err != nil {
		return nil, err
	}
	conn, err := d.original.Open(name)
	if err != nil {
		return nil, err
	}
	return &WrapperConn{originalConn: conn, cfgManager: d.cfgManager}, nil
}

func applyConnectionChaos(ctx context.Context, mgr *config.Manager) error {
	if mgr == nil {
		return nil
	}
	policies := mgr.GetActivePolicies("sql")
	for _, p := range policies {
		if !p.DropConnection {
			continue
		}
		if !strings.EqualFold(p.Target, "all") && !strings.EqualFold(p.Target, "database") {
			continue
		}
		chance := p.ErrorChance
		if chance <= 0 {
			chance = 1.0
		}
		if rand.Float64() >= chance {
			continue
		}

		metrics.InjectedFaultsTotal.WithLabelValues(p.MetricTag, "drop").Inc()
		// Defer span.End so any panic in telemetry.EmitError still closes the
		// span and does not leak an active OTLP recording.
		_, span := tracing.StartChaosSpan(ctx, "pastaay.sql.drop", p.Target, "drop")
		defer span.End()
		telemetry.EmitError("sql", p.Target, "Connection force dropped", "TCP stream severed by DropConnection policy", span)
		return errors.New("[Pastaay-SQL] Chaos: TCP connection forcefully dropped")
	}
	return nil
}

func Register(driverName string, original driver.Driver, mgr *config.Manager) {
	sql.Register(driverName, &WrapperDriver{original: original, cfgManager: mgr})
}
