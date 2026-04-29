package config

import "sync"

type Manager struct {
	mu  sync.RWMutex
	cfg *PastaayConfig
}

func NewManager(initialConfig *PastaayConfig) *Manager {
	return &Manager{cfg: initialConfig}
}

func (m *Manager) GetActivePolicies(policyType string) []Policy {
	m.mu.RLock()
	defer m.mu.RUnlock()

	var active []Policy
	if m.cfg == nil {
		return active
	}

	for _, p := range m.cfg.Policies {
		if p.Type == policyType {
			active = append(active, p)
		}
	}
	return active
}

func (m *Manager) Update(newCfg *PastaayConfig) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.cfg = newCfg
}
