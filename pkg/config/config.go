package config

import "time"

type Policy struct {
	Name            string            `yaml:"name"`
	Target          string            `yaml:"target"`
	Type            string            `yaml:"type"`
	LatencyChance   float64           `yaml:"latency_chance"`
	LatencyDuration time.Duration     `yaml:"latency_duration"`
	ErrorChance     float64           `yaml:"error_chance"`
	ErrorCode       int               `yaml:"error_code,omitempty"`
	ErrorBody       string            `yaml:"error_body,omitempty"`
	MatchHeaders    map[string]string `yaml:"match_headers,omitempty"`
	DropConnection  bool              `yaml:"drop_connection,omitempty"`
}

type PastaayConfig struct {
	Version              int                 `yaml:"version"`
	WarmupDuration       time.Duration       `yaml:"warmup_duration"`
	EnableDefaultIgnored bool                `yaml:"enable_default_ignored"` // Global protection toggle
	IgnoredCommands      map[string][]string `yaml:"ignored_commands"`       // Custom user overrides
	Policies             []Policy            `yaml:"policies"`
}

// DefaultProtectedCommands contains critical infrastructure commands
// that Pastaay should not sabotage during startup or migrations.
var DefaultProtectedCommands = map[string][]string{
	"sql":   {"CREATE", "ALTER", "DROP", "TRUNCATE"},
	"mongo": {"create", "createIndexes", "drop", "collMod"},
	"redis": {"PING", "INFO", "CONFIG"},
	"grpc":  {"grpc.health.v1.Health"},
}