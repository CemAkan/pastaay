package config

import (
	"strings"
	"sync"
	"time"
)

// Manager handles thread-safe access to the Pastaay configuration.
type Manager struct {
	mu            sync.RWMutex
	cfg           *PastaayConfig
	typedPolicies map[string][]Policy
	startTime     time.Time // Added to track Warmup
}

func NewManager(initialConfig *PastaayConfig) *Manager {
	m := &Manager{
		startTime: time.Now(),
	}
	m.Update(initialConfig)
	return m
}

func (m *Manager) Update(newCfg *PastaayConfig) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.cfg = newCfg

	cache := make(map[string][]Policy)
	if newCfg != nil {
		for _, p := range newCfg.Policies {
			cache[p.Type] = append(cache[p.Type], p)
		}
	}
	m.typedPolicies = cache
}

func (m *Manager) GetActivePolicies(policyType string) []Policy {
	m.mu.RLock()
	defer m.mu.RUnlock()

	// 1. Warmup Protection: Return nothing if system is still warming up
	if m.cfg != nil && time.Since(m.startTime) < m.cfg.WarmupDuration {
		return nil
	}

	return m.typedPolicies[policyType]
}

// IsCommandIgnored checks if a specific query/command should bypass chaos injection.
func (m *Manager) IsCommandIgnored(protocol string, cmd string) bool {
	m.mu.RLock()
	defer m.mu.RUnlock()
	if m.cfg == nil {
		return false
	}

	cleanCmd := strings.ToUpper(strings.TrimSpace(cmd))

	// 1. Check Default Protections (if enabled)
	if m.cfg.EnableDefaultIgnored {
		if list, ok := DefaultProtectedCommands[protocol]; ok {
			for _, protected := range list {
				if strings.HasPrefix(cleanCmd, strings.ToUpper(protected)) {
					return true
				}
			}
		}
	}

	// 2. Check Custom User Protections
	if m.cfg.IgnoredCommands != nil {
		if customList, ok := m.cfg.IgnoredCommands[protocol]; ok {
			for _, custom := range customList {
				if strings.HasPrefix(cleanCmd, strings.ToUpper(custom)) {
					return true
				}
			}
		}
	}

	return false
}
