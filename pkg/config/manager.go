package config

import (
	"strings"
	"sync"
	"sync/atomic"
	"time"
)

type Manager struct {
	mu            sync.Mutex
	cfg           atomic.Pointer[PastaayConfig]
	typedPolicies atomic.Pointer[map[string][]Policy]
	startTime     time.Time
}

func NewManager(initialConfig *PastaayConfig) *Manager {
	m := &Manager{
		startTime: time.Now(),
	}

	emptyMap := make(map[string][]Policy)
	m.typedPolicies.Store(&emptyMap)

	m.Update(initialConfig)
	return m
}

func (m *Manager) Update(newCfg *PastaayConfig) {
	m.mu.Lock()
	defer m.mu.Unlock()

	if newCfg != nil {
		m.cfg.Store(newCfg)
	}

	cache := make(map[string][]Policy)
	if newCfg != nil {
		for _, p := range newCfg.Policies {
			cache[p.Type] = append(cache[p.Type], p)
		}
	}
	m.typedPolicies.Store(&cache)
}

func (m *Manager) GetActivePolicies(policyType string) []Policy {
	policiesPtr := m.typedPolicies.Load()
	if policiesPtr == nil {
		return nil
	}
	typed := *policiesPtr

	cfg := m.cfg.Load()
	if cfg != nil && time.Since(m.startTime) < cfg.WarmupDuration {
		return nil
	}

	return typed[policyType]
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

func (m *Manager) IsCleanCommandIgnored(protocol string, cleanCmd string) bool {
	cfg := m.cfg.Load()
	if cfg == nil {
		return false
	}

	if cfg.EnableDefaultIgnored {
		if list, ok := DefaultProtectedCommands[protocol]; ok {
			for _, protected := range list {
				if strings.HasPrefix(cleanCmd, strings.ToUpper(protected)) {
					return true
				}
			}
		}
	}

	if cfg.IgnoredCommands != nil {
		if customList, ok := cfg.IgnoredCommands[protocol]; ok {
			for _, custom := range customList {
				if strings.HasPrefix(cleanCmd, strings.ToUpper(custom)) {
					return true
				}
			}
		}
	}

	return false
}

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
