package config

import "sync"

// Manager handles thread-safe access to the Pastaay configuration.
type Manager struct {
	mu            sync.RWMutex
	cfg           *PastaayConfig
	typedPolicies map[string][]Policy // Cache map

}

// NewManager creates a new Manager instance and initializes the internal cache.
func NewManager(initialConfig *PastaayConfig) *Manager {
	m := &Manager{}
	m.Update(initialConfig) // Call Update to build the initial cache
	return m
}

// Update replaces the current configuration and rebuilds the policy cache.
// This is thread-safe and designed to be called by hot-reloading watchers.
func (m *Manager) Update(newCfg *PastaayConfig) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.cfg = newCfg

	//  loop runs only once per configuration reload, not per request.
	cache := make(map[string][]Policy)
	if newCfg != nil {
		for _, p := range newCfg.Policies {
			cache[p.Type] = append(cache[p.Type], p)
		}
	}
	m.typedPolicies = cache
}

// GetActivePolicies retrieves policies for a specific system type
func (m *Manager) GetActivePolicies(policyType string) []Policy {
	m.mu.RLock()
	defer m.mu.RUnlock()

	// Directly return the slice of policies from the cache map.
	return m.typedPolicies[policyType]
}
