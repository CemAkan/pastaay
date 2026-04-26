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
	conn, err := d.original.Open(name)
	if err != nil {
		return nil, err
	}

	// Wrap the original connection with our agent (WrapperConn)
	return &WrapperConn{
		originalConn: conn,
		cfgManager:   d.cfgManager,
	}, nil
}

// Register registers the Pastaay chaos driver with the Go sql package.
// Users will call this instead of directly registering their normal DB driver.
func Register(driverName string, originalDriver driver.Driver, cfgManager *config.Manager) {
	sql.Register(driverName, &WrapperDriver{
		original:   originalDriver,
		cfgManager: cfgManager,
	})
}
