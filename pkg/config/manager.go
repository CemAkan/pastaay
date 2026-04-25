package config

import (
	"sync"
)

// Manager handles thread-safe access to the Pastaay configuration.
// It uses an RWMutex to allow multiple concurrent readers but only one writer.
type Manager struct {
	mu  sync.RWMutex
	cfg *PastaayConfig
}

// NewManager initializes a new safe configuration manager.
func NewManager(initialConfig *PastaayConfig) *Manager {
	return &Manager{
		cfg: initialConfig,
	}
}

// Get safely returns a pointer to the current configuration.
// Multiple goroutines can call this simultaneously without blocking each other.
func (m *Manager) Get() *PastaayConfig {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.cfg
}

// Update safely replaces the current configuration with a new one.
// This blocks all readers until the new configuration is fully written to memory.
func (m *Manager) Update(newCfg *PastaayConfig) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.cfg = newCfg
}
