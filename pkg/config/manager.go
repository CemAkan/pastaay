package config

import (
	"math"
	"regexp"
	"sort"
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
	m := &Manager{startTime: time.Now()}
	emptyMap := make(map[string][]Policy)
	m.typedPolicies.Store(&emptyMap)
	m.Update(initialConfig)
	return m
}

func generateStableHash(p *Policy) uint64 {
	var h uint64 = 14695981039346656037
	sep := func() { h ^= 0; h *= 1099511628211 }

	// Essential policy strings
	for _, s := range []string{p.Name, p.Target, p.Type, p.ErrorBody, p.StreamRollMode} {
		for i := 0; i < len(s); i++ {
			h ^= uint64(s[i])
			h *= 1099511628211
		}
		sep()
	}

	// Boolean state change must trigger re-roll
	if p.DropConnection {
		h ^= 1
	} else {
		h ^= 0
	}
	h *= 1099511628211

	// Numeric & Probability fields
	h ^= uint64(p.ThrottleThreshold)
	h *= 1099511628211
	h ^= uint64(p.RAMChunkMB)
	h *= 1099511628211
	h ^= uint64(p.RAMInterval)
	h *= 1099511628211
	h ^= uint64(p.LatencyDuration)
	h *= 1099511628211
	h ^= uint64(p.ErrorCode)
	h *= 1099511628211
	h ^= math.Float64bits(p.LatencyChance)
	h *= 1099511628211
	h ^= math.Float64bits(p.ErrorChance)
	h *= 1099511628211

	// Metadata headers
	if len(p.MatchHeaders) > 0 {
		keys := make([]string, 0, len(p.MatchHeaders))
		for k := range p.MatchHeaders {
			keys = append(keys, k)
		}
		sort.Strings(keys)
		for _, k := range keys {
			v := p.MatchHeaders[k]
			for i := 0; i < len(k); i++ {
				h ^= uint64(k[i])
				h *= 1099511628211
			}
			sep()
			for i := 0; i < len(v); i++ {
				h ^= uint64(v[i])
				h *= 1099511628211
			}
			sep()
		}
	}
	return h
}

func (m *Manager) Update(newCfg *PastaayConfig) {
	m.mu.Lock()
	defer m.mu.Unlock()
	if newCfg != nil {
		for i := range newCfg.Policies {
			p := &newCfg.Policies[i]
			if strings.EqualFold(p.Type, "sql") && !strings.EqualFold(p.Target, "ALL") && !strings.EqualFold(p.Target, "DATABASE") {
				re, err := regexp.Compile(`(?i)\b` + regexp.QuoteMeta(strings.ToUpper(p.Target)) + `\b`)
				if err == nil {
					p.CompiledRegex = re
				}
			}
			if strings.HasSuffix(p.Target, "*") {
				p.IsWildcard = true
				p.WildcardPrefix = strings.ToUpper(p.Target[:len(p.Target)-1])
			}
			p.PolicyHash = generateStableHash(p)
		}
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
	ptr := m.typedPolicies.Load()
	cfg := m.cfg.Load()
	if ptr == nil || (cfg != nil && time.Since(m.startTime) < cfg.WarmupDuration) {
		return nil
	}
	return (*ptr)[policyType]
}

func CleanSQLCommand(cmd string) string {
	if cmd == "" {
		return ""
	}
	var result strings.Builder
	inString := false
	var stringChar byte
	for i := 0; i < len(cmd); i++ {
		char := cmd[i]
		if char == '\'' || char == '"' {
			isEscaped := false
			for j := i - 1; j >= 0 && cmd[j] == '\\'; j-- {
				isEscaped = !isEscaped
			}
			if !isEscaped {
				if inString && char == stringChar {
					inString = false
				} else if !inString {
					inString = true
					stringChar = char
				}
			}
		}
		if !inString {
			if char == '-' && i+1 < len(cmd) && cmd[i+1] == '-' {
				for i < len(cmd) && cmd[i] != '\n' {
					i++
				}
				result.WriteByte(' ')
				continue
			}
			if char == '/' && i+1 < len(cmd) && cmd[i+1] == '*' {
				endIdx := strings.Index(cmd[i+2:], "*/")
				if endIdx != -1 {
					i += endIdx + 3
					result.WriteByte(' ')
					continue
				}
			}
		}
		result.WriteByte(char)
	}
	clean := strings.Trim(result.String(), " \r\n\t;()")
	return strings.ToUpper(clean)
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
	if protocol == "sql" {
		return m.IsCleanCommandIgnored(protocol, CleanSQLCommand(cmd))
	}
	cleanCmd := strings.ToUpper(strings.TrimSpace(cmd))
	cleanCmd = strings.TrimLeft(cleanCmd, "/")
	return m.IsCleanCommandIgnored(protocol, cleanCmd)
}
