package config

import (
	"strings"
	"sync"
	"time"
)

type Manager struct {
	mu            sync.RWMutex
	cfg           *PastaayConfig
	typedPolicies map[string][]Policy
	startTime     time.Time
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

	// Warmup Protection
	if m.cfg != nil && time.Since(m.startTime) < m.cfg.WarmupDuration {
		return nil
	}

	return m.typedPolicies[policyType]
}

func (m *Manager) IsCommandIgnored(protocol string, cmd string) bool {
	m.mu.RLock()
	defer m.mu.RUnlock()
	if m.cfg == nil {
		return false
	}

	cleanCmd := strings.TrimSpace(cmd)
	if protocol == "sql" {
		
		for {
			prev := cleanCmd
			if strings.HasPrefix(cleanCmd, "/*") {
				if endIndex := strings.Index(cleanCmd, "*/"); endIndex != -1 {
					cleanCmd = strings.TrimSpace(cleanCmd[endIndex+2:])
				}
			}
			if strings.HasPrefix(cleanCmd, "--") {
				lines := strings.SplitN(cleanCmd, "\n", 2)
				if len(lines) > 1 {
					cleanCmd = strings.TrimSpace(lines[1])
				} else {
					cleanCmd = ""
				}
			}
			if cleanCmd == prev {
				break
			}
		}
	}

	cleanCmd = strings.ToUpper(cleanCmd)
	cleanCmd = strings.TrimPrefix(cleanCmd, "/")

	if m.cfg.EnableDefaultIgnored {
		if list, ok := DefaultProtectedCommands[protocol]; ok {
			for _, protected := range list {
				if strings.HasPrefix(cleanCmd, strings.ToUpper(protected)) {
					return true
				}
			}
		}
	}

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
