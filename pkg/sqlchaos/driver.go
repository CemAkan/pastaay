package sqlchaos

import (
	"database/sql"
	"database/sql/driver"

	"github.com/CemAkan/pastaay/pkg/config"
)

// WrapperDriver wraps a standard sql.Driver with chaos injection capabilities.
type WrapperDriver struct {
	original   driver.Driver
	cfgManager *config.Manager
}

// Open implements the driver.Driver interface.
// It intercepts the connection creation to potentially inject faults.
func (d *WrapperDriver) Open(name string) (driver.Conn, error) {
	// 1. Open the actual database connection using the original driver
	conn, err := d.original.Open(name)
	if err != nil {
		return nil, err
	}

	// TODO: Wrap this connection to intercept Exec and Query calls.
	// For now, just return the original, untouched connection.
	return conn, nil
}

// Register registers the Pastaay chaos driver with the Go sql package.
// Users will call this instead of directly registering their normal DB driver.
func Register(driverName string, originalDriver driver.Driver, cfgManager *config.Manager) {
	sql.Register(driverName, &WrapperDriver{
		original:   originalDriver,
		cfgManager: cfgManager,
	})
}
