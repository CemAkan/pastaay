package config

import (
	"regexp"
	"time"
)

type Policy struct {
	Name              string            `yaml:"name"`
	Target            string            `yaml:"target"`
	Type              string            `yaml:"type"`
	LatencyChance     float64           `yaml:"latency_chance"`
	LatencyDuration   time.Duration     `yaml:"latency_duration"`
	ErrorChance       float64           `yaml:"error_chance"`
	ErrorCode         int               `yaml:"error_code,omitempty"`
	ErrorBody         string            `yaml:"error_body,omitempty"`
	MatchHeaders      map[string]string `yaml:"match_headers,omitempty"`
	DropConnection    bool              `yaml:"drop_connection,omitempty"`
	StreamRollMode    string            `yaml:"stream_roll_mode,omitempty"`
	ThrottleThreshold int               `yaml:"throttle_threshold,omitempty"`
	RAMChunkMB        int               `yaml:"ram_chunk_mb,omitempty"`
	RAMInterval       time.Duration     `yaml:"ram_interval,omitempty"`
	CompiledRegex     *regexp.Regexp    `yaml:"-"`
	PolicyHash        uint64            `yaml:"-"`
	IsWildcard        bool              `yaml:"-"`
	WildcardPrefix    string            `yaml:"-"`
}

type PastaayConfig struct {
	Version              int                 `yaml:"version"`
	WarmupDuration       time.Duration       `yaml:"warmup_duration"`
	EnableDefaultIgnored bool                `yaml:"enable_default_ignored"`
	IgnoredCommands      map[string][]string `yaml:"ignored_commands"`
	Policies             []Policy            `yaml:"policies"`
}

var DefaultProtectedCommands = map[string][]string{
	"sql":      {"CREATE", "ALTER", "DROP", "TRUNCATE"},
	"mongo":    {"create", "createIndexes", "drop", "collMod"},
	"redis":    {"PING", "INFO", "CONFIG"},
	"grpc":     {"grpc.health.v1.Health"},
	"kafka":    {"__consumer_offsets", "_schemas", "__transaction_state"},
	"rabbitmq": {"amq.", "reply_"},
}
