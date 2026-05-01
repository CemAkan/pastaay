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

// CleanSQLCommand strips comments and whitespace, and converts to uppercase.
func CleanSQLCommand(cmd string) string {
	cleanCmd := strings.TrimSpace(cmd)
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
	return strings.ToUpper(cleanCmd)
}

// IsCleanCommandIgnored evaluates already-scrubbed commands to prevent double-allocation.
func (m *Manager) IsCleanCommandIgnored(protocol string, cleanCmd string) bool {
	m.mu.RLock()
	defer m.mu.RUnlock()
	if m.cfg == nil {
		return false
	}

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

// Retained for backward compatibility with other protocols (Redis, Mongo, HTTP, etc.)
func (m *Manager) IsCommandIgnored(protocol string, cmd string) bool {
	var cleanCmd string
	if protocol == "sql" {
		cleanCmd = CleanSQLCommand(cmd)
	} else {
		cleanCmd = strings.ToUpper(strings.TrimSpace(cmd))
		cleanCmd = strings.TrimPrefix(cleanCmd, "/")
	}
	return m.IsCleanCommandIgnored(protocol, cleanCmd)
}
