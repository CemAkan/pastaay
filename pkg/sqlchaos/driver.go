package sqlchaos

import (
	"database/sql"
	"database/sql/driver"
	"fmt"
	"strings" // Case-insensitive target kontrolü için

	"github.com/CemAkan/pastaay/pkg/config"
)

// WrapperDriver implements the sql.Driver interface to intercept connection attempts.
type WrapperDriver struct {
	original   driver.Driver
	cfgManager *config.Manager
}

// Open checks for DropConnection policies before establishing a physical DB connection.
func (d *WrapperDriver) Open(name string) (driver.Conn, error) {
	// Retrieve dynamic SQL policies
	policies := d.cfgManager.GetActivePolicies("sql")
	for _, p := range policies {

		isGlobal := strings.EqualFold(p.Target, "all") || strings.EqualFold(p.Target, "database")

		if p.DropConnection && isGlobal {
			return nil, fmt.Errorf("[Pastaay-SQL] Chaos: connection rejected by active nuclear policy")
		}
	}

	conn, err := d.original.Open(name)
	if err != nil {
		return nil, err
	}

	return &WrapperConn{
		originalConn: conn,
		cfgManager:   d.cfgManager,
	}, nil
}

// Register facilitates the zero-friction integration of the chaos driver.
func Register(driverName string, original driver.Driver, mgr *config.Manager) {
	sql.Register(driverName, &WrapperDriver{
		original:   original,
		cfgManager: mgr,
	})
}
