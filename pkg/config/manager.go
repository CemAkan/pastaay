package config

import (
	"log"
	"math"
	"regexp"
	"sort"
	"strings"
	"sync"
	"sync/atomic"
	"time"
)

// unit of atomic publication
type snapshot struct {
	cfg           *PastaayConfig
	typedPolicies map[string][]Policy
}

type Manager struct {
	mu           sync.Mutex
	snap         atomic.Pointer[snapshot]
	startTime    time.Time
	sensorStatus sync.Map // map[string]string
}

func NewManager(initialConfig *PastaayConfig) *Manager {
	m := &Manager{startTime: time.Now()}
	// Seed with empty snapshot so readers never observe a nil snap.
	m.snap.Store(&snapshot{typedPolicies: make(map[string][]Policy)})
	m.Update(initialConfig)
	return m
}

func (m *Manager) SetSensorStatus(name, status string) {
	m.sensorStatus.Store(name, status)
}

func (m *Manager) GetSensorStatuses() map[string]string {
	res := make(map[string]string)
	m.sensorStatus.Range(func(k, v interface{}) bool {
		ks, kok := k.(string)
		vs, vok := v.(string)
		if kok && vok {
			res[ks] = vs
		}
		return true
	})
	return res
}

// canonicalFloat64Bits collapses ±0 and NaN.
func canonicalFloat64Bits(v float64) uint64 {
	if math.IsNaN(v) {
		return 0x7FF8000000000001 // canonical quiet NaN
	}
	if v == 0 {
		return 0 // collapse +0 and -0
	}
	return math.Float64bits(v)
}

func generateStableHash(p *Policy) uint64 {
	const prime uint64 = 1099511628211
	var h uint64 = 14695981039346656037
	sep := func() { h *= prime }

	for _, s := range []string{p.Name, p.Target, p.Type, p.ErrorBody, p.StreamRollMode} {
		for i := 0; i < len(s); i++ {
			h ^= uint64(s[i])
			h *= prime
		}
		sep()
	}

	if p.DropConnection {
		h ^= 1
	}
	h *= prime

	h ^= uint64(p.ThrottleThreshold)
	h *= prime
	h ^= uint64(p.RAMChunkMB)
	h *= prime
	h ^= uint64(p.RAMInterval)
	h *= prime
	h ^= uint64(p.LatencyDuration)
	h *= prime
	h ^= uint64(p.ErrorCode)
	h *= prime
	h ^= canonicalFloat64Bits(p.LatencyChance)
	h *= prime
	h ^= canonicalFloat64Bits(p.ErrorChance)
	h *= prime

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
				h *= prime
			}
			sep()
			for i := 0; i < len(v); i++ {
				h ^= uint64(v[i])
				h *= prime
			}
			sep()
		}
	}
	return h
}

// Update atomically swaps in a new configuration.
func (m *Manager) Update(newCfg *PastaayConfig) {
	m.mu.Lock()
	defer m.mu.Unlock()

	newSnap := &snapshot{typedPolicies: make(map[string][]Policy)}

	if newCfg != nil {
		if err := newCfg.Validate(); err != nil {
			log.Printf("[Pastaay-Config] Update rejected: %v", err)
			return
		}

		for i := range newCfg.Policies {
			p := &newCfg.Policies[i]

			tag := p.Type + ":" + p.Target
			if len(tag) > 64 {
				tag = tag[:61] + "..."
			}
			p.MetricTag = tag

			if strings.HasSuffix(p.Target, "*") && len(p.Target) > 1 {
				p.IsWildcard = true
				p.WildcardPrefix = strings.ToUpper(p.Target[:len(p.Target)-1])
			}

			if strings.EqualFold(p.Type, "sql") && !strings.EqualFold(p.Target, "ALL") && !strings.EqualFold(p.Target, "DATABASE") {
				targetPattern := p.Target

				isAlphaNum := true
				for _, char := range targetPattern {
					if !((char >= 'a' && char <= 'z') || (char >= 'A' && char <= 'Z') || (char >= '0' && char <= '9') || char == '_') {
						isAlphaNum = false
						break
					}
				}
				if isAlphaNum {
					targetPattern = `\b` + targetPattern + `\b`
				}

				re, err := regexp.Compile(`(?i)` + targetPattern)
				if err != nil {
					log.Printf("[Pastaay-Config] WARN: policy %q target regex compile failed: %v", p.Name, err)
				} else {
					p.CompiledRegex = re
				}
			}

			p.PolicyHash = generateStableHash(p)
		}

		newSnap.cfg = newCfg
		for _, p := range newCfg.Policies {
			key := strings.ToLower(p.Type)
			newSnap.typedPolicies[key] = append(newSnap.typedPolicies[key], p)
		}
	}

	// Single atomic publication
	m.snap.Store(newSnap)
}

func (m *Manager) GetActivePolicies(policyType string) []Policy {
	s := m.snap.Load()
	if s == nil {
		return nil
	}
	if s.cfg != nil && s.cfg.WarmupDuration > 0 && time.Since(m.startTime) < s.cfg.WarmupDuration {
		return nil
	}
	return s.typedPolicies[strings.ToLower(policyType)]
}

// CleanSQLCommand strips comments and uppercases for matching.
func CleanSQLCommand(cmd string) string {
	if cmd == "" {
		return ""
	}
	var result strings.Builder
	result.Grow(len(cmd))
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
	s := m.snap.Load()
	if s == nil || s.cfg == nil {
		return false
	}
	cfg := s.cfg
	if cfg.EnableDefaultIgnored {
		if list, ok := DefaultProtectedCommands[protocol]; ok {
			for _, protected := range list {
				p := strings.TrimLeft(strings.ToUpper(protected), "/")
				if p == "" {
					continue
				}
				if strings.HasPrefix(cleanCmd, p) {
					return true
				}
			}
		}
	}
	if cfg.IgnoredCommands != nil {
		if customList, ok := cfg.IgnoredCommands[protocol]; ok {
			for _, custom := range customList {
				c := strings.TrimLeft(strings.ToUpper(custom), "/")
				if c == "" {
					continue
				}
				if strings.HasPrefix(cleanCmd, c) {
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

func (m *Manager) GetRawConfig() *PastaayConfig {
	s := m.snap.Load()
	if s == nil {
		return nil
	}
	return s.cfg
}
